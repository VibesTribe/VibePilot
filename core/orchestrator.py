"""
VibePilot Concurrent Orchestrator

Multi-agent task dispatcher with:
- Concurrent task execution
- Dependency-aware scheduling
- Runner pool management
- ROI tracking
- Model performance learning

This replaces the single-threaded orchestrator.py
"""

import os
import json
import time
import logging
import threading
import yaml
from typing import Dict, Any, Optional, List
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, Future, as_completed
from supabase import create_client
from dotenv import load_dotenv

from task_manager import TaskManager
from agents.supervisor import SupervisorAgent
from core.telemetry import get_telemetry, traced

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(threadName)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("VibePilot.Orchestrator")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)


def load_config() -> Dict:
    """Load orchestrator config from vibepilot.yaml."""
    config_path = os.path.join(
        os.path.dirname(__file__), "..", "config", "vibepilot.yaml"
    )

    try:
        with open(config_path, "r") as f:
            config = yaml.safe_load(f)
        return config.get("orchestrator", {})
    except Exception as e:
        logger.warning(f"Could not load config: {e}, using defaults")
        return {}


class RunnerPool:
    """
    Manages available runners for task execution.
    Tracks which are busy, which have capacity.
    """

    def __init__(self):
        self.runners: Dict[str, Dict] = {}
        self.busy: Dict[str, str] = {}  # runner_id -> task_id
        self.lock = threading.Lock()
        self._load_runners()

    def _load_runners(self):
        """Load active models/runners from database."""
        res = db.table("models").select("*").eq("status", "active").execute()

        for model in res.data or []:
            self.runners[model["id"]] = {
                "id": model["id"],
                "platform": model.get("platform", "unknown"),
                "type": model.get("type", "runner"),
                "context_limit": model.get("context_limit", 32000),
                "strengths": model.get("strengths", []),
                "task_ratings": model.get("task_ratings", {}),
                "status": model.get("status", "active"),
            }

        logger.info(f"Loaded {len(self.runners)} runners")

    def is_available(self, runner_id: str) -> bool:
        """Check if runner is available."""
        with self.lock:
            return runner_id in self.runners and runner_id not in self.busy

    def acquire(self, runner_id: str, task_id: str) -> bool:
        """Acquire a runner for a task."""
        with self.lock:
            if runner_id not in self.runners:
                return False
            if runner_id in self.busy:
                return False
            self.busy[runner_id] = task_id
            return True

    def release(self, runner_id: str):
        """Release a runner after task completion."""
        with self.lock:
            if runner_id in self.busy:
                del self.busy[runner_id]

    def get_available(self) -> List[str]:
        """Get list of available runner IDs."""
        with self.lock:
            return [r for r in self.runners if r not in self.busy]

    def get_best_for_task(self, task: Dict) -> Optional[str]:
        """
        Select best available runner for task.
        Uses router scoring formula from Vibeflow.
        """
        task_type = task.get("type", "feature")
        priority = task.get("priority", 5)
        dependencies = task.get("dependencies", [])

        available = self.get_available()
        if not available:
            return None

        scored = []
        for runner_id in available:
            runner = self.runners[runner_id]

            ratings = runner.get("task_ratings", {}).get(task_type, {})
            success_rate = 0.5
            if ratings.get("count", 0) > 0:
                total = ratings["success"] + ratings["fail"]
                success_rate = ratings["success"] / total if total > 0 else 0.5

            w1, w2, w3 = 0.3, 0.3, 0.4
            score = (
                w1 * (priority / 10)
                + w2 * success_rate
                + w3 * (1 if task_type in runner.get("strengths", []) else 0.5)
            )

            scored.append((runner_id, score))

        scored.sort(key=lambda x: x[1], reverse=True)
        return scored[0][0] if scored else None


class DependencyManager:
    """
    Manages task dependencies and unlock logic.
    """

    def __init__(self):
        self.lock = threading.Lock()

    def can_run(self, task: Dict) -> bool:
        """Check if all dependencies are satisfied."""
        deps = task.get("dependencies", [])
        if not deps:
            return True

        dep_ids = [d.get("task_id") if isinstance(d, dict) else d for d in deps]

        res = db.table("tasks").select("id, status").in_("id", dep_ids).execute()

        for t in res.data or []:
            if t.get("status") != "merged":
                return False

        return True

    def get_blocked_tasks(self) -> List[str]:
        """Get tasks that are blocked by incomplete dependencies."""
        res = (
            db.table("tasks")
            .select("id, dependencies, status")
            .eq("status", "locked")
            .execute()
        )

        blocked = []
        for task in res.data or []:
            deps = task.get("dependencies", [])
            if deps:
                blocked.append(task["id"])

        return blocked

    def unlock_ready_tasks(self) -> List[str]:
        """Find and unlock tasks whose dependencies are now complete."""
        unlocked = []

        res = (
            db.table("tasks")
            .select("id, dependencies, status")
            .eq("status", "locked")
            .execute()
        )

        for task in res.data or []:
            if self.can_run(task):
                db.table("tasks").update(
                    {"status": "available", "updated_at": datetime.utcnow().isoformat()}
                ).eq("id", task["id"]).execute()

                unlocked.append(task["id"])
                logger.info(f"Unlocked task {task['id'][:8]}...")

        return unlocked


class ConcurrentOrchestrator:
    """
    VibePilot Concurrent Orchestrator.

    Coordinates parallel task execution with:
    - Runner pool management
    - Dependency-aware scheduling
    - Supervisor integration
    - Telemetry
    - ROI tracking

    Concurrency is dynamic - scales based on available runners and tasks.
    """

    def __init__(self, max_workers: int = None, config_path: str = None):
        """
        Initialize orchestrator.

        Args:
            max_workers: Maximum concurrent tasks. None = from config or auto
            config_path: Path to config file for settings
        """
        self.runner_pool = RunnerPool()
        self.dependency_manager = DependencyManager()
        self.supervisor = SupervisorAgent()
        self.task_manager = TaskManager()
        self.telemetry = get_telemetry()

        self.config = load_config()
        self._explicit_max_workers = max_workers

        self.executor: Optional[ThreadPoolExecutor] = None
        self.active_tasks: Dict[str, Future] = {}
        self.running = False
        self.lock = threading.Lock()

        self.tick_interval = self.config.get("tick_interval", 2)
        self.dynamic_scaling = self.config.get("dynamic_scaling", True)
        self.min_workers = self.config.get("min_workers", 1)
        self.max_workers_cap = self.config.get("max_workers_cap", 50)

        swarm_config = self.config.get("swarm", {})
        self.swarm_enabled = swarm_config.get("enabled", True)
        self.swarm_max_workers = swarm_config.get("max_workers", 100)
        self.swarm_default_workers = swarm_config.get("default_workers", 10)
        self.swarm_task_types = swarm_config.get(
            "task_types",
            ["repo_audit", "bulk_refactor", "parallel_test", "wide_search"],
        )

        self.logger = logger

    def _should_use_swarm(self, task: Dict) -> bool:
        """Determine if task should use Kimi swarm mode."""
        if not self.swarm_enabled:
            return False

        task_type = task.get("type", "")
        if task_type in self.swarm_task_types:
            return True

        subtasks = task.get("subtasks", [])
        if len(subtasks) >= 3:
            return True

        return False

    def _dispatch_swarm(self, task: Dict) -> Dict:
        """Dispatch task to Kimi swarm."""
        from runners.kimi_runner import KimiRunner

        runner = KimiRunner()
        subtasks = task.get("subtasks", [])

        if subtasks:
            swarm_result = runner.execute_swarm(
                tasks=subtasks, max_workers=min(len(subtasks), self.swarm_max_workers)
            )
        else:
            task_type = task.get("type", "")

            if task_type == "repo_audit":
                swarm_result = runner.execute_repo_audit(
                    repo_path=task.get("repo_path", os.getcwd()),
                    checks=task.get("checks"),
                )
            elif task_type == "bulk_refactor":
                swarm_result = runner.execute_parallel_refactor(
                    files=task.get("files", []),
                    refactoring=task.get("refactoring", ""),
                    work_dir=task.get("work_dir"),
                )
            else:
                swarm_result = runner.execute_swarm(
                    tasks=[{"prompt": task.get("prompt", ""), "id": task.get("id")}],
                    max_workers=self.swarm_default_workers,
                )

        return swarm_result

    def _get_max_workers(self) -> int:
        """
        Determine max concurrent workers.

        Priority:
        1. Explicit constructor arg
        2. Config file max_workers
        3. Number of available runners (dynamic)
        4. min_workers as floor, max_workers_cap as ceiling
        """
        if self._explicit_max_workers is not None:
            return min(self._explicit_max_workers, self.max_workers_cap)

        config_max = self.config.get("max_workers")
        if config_max is not None:
            return min(config_max, self.max_workers_cap)

        available_runners = len(self.runner_pool.get_available())

        workers = max(self.min_workers, available_runners)
        workers = min(workers, self.max_workers_cap)

        return workers

    def start(self):
        """Start the orchestrator loop."""
        self.running = True

        max_w = self._get_max_workers()
        self.executor = ThreadPoolExecutor(max_workers=max_w)

        self.logger.info(f"Orchestrator started with up to {max_w} workers (dynamic)")
        self.logger.info(f"Available runners: {len(self.runner_pool.get_available())}")

        try:
            while self.running:
                self._tick()
                time.sleep(self.tick_interval)
        except KeyboardInterrupt:
            self.logger.info("Shutdown requested")
        finally:
            self.stop()

    def scale_workers(self):
        """
        Dynamically scale thread pool based on available runners.
        Call periodically or when runner pool changes significantly.
        """
        if self.executor is None:
            return

        current_max = self.executor._max_workers
        new_max = self._get_max_workers()

        if new_max != current_max:
            self.logger.info(f"Scaling workers: {current_max} → {new_max}")
            self.executor._max_workers = new_max

    def stop(self):
        """Stop the orchestrator."""
        self.running = False
        if self.executor:
            self.executor.shutdown(wait=True)
        self.logger.info("Orchestrator stopped")

    def _tick(self):
        """Main orchestrator tick - dispatch tasks and check results."""
        self._check_completed_futures()

        unlocked = self.dependency_manager.unlock_ready_tasks()
        if unlocked:
            self.logger.info(f"Unlocked {len(unlocked)} tasks")

        pending_reviews = len(self.supervisor.get_pending_reviews())
        if pending_reviews > 0:
            self._process_reviews()

        available = self.runner_pool.get_available()
        if not available:
            return

        tasks = self._get_available_tasks(len(available))

        for task in tasks:
            self._dispatch_task(task)

    def _get_available_tasks(self, limit: int) -> List[Dict]:
        """Get available tasks with satisfied dependencies."""
        res = (
            db.table("tasks")
            .select("*")
            .eq("status", "available")
            .order("priority")
            .limit(limit)
            .execute()
        )

        tasks = []
        for task in res.data or []:
            if self.dependency_manager.can_run(task):
                tasks.append(task)

        return tasks

    @traced("task_dispatch")
    def _dispatch_task(self, task: Dict) -> bool:
        """Dispatch a task to an available runner."""
        task_id = task["id"]

        runner_id = self.runner_pool.get_best_for_task(task)
        if not runner_id:
            self.logger.warning(f"No runner available for task {task_id[:8]}")
            return False

        if not self.runner_pool.acquire(runner_id, task_id):
            return False

        db.table("tasks").update(
            {
                "status": "in_progress",
                "assigned_to": runner_id,
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        future = self.executor.submit(self._execute_task, task, runner_id)

        with self.lock:
            self.active_tasks[task_id] = future

        self.logger.info(f"Dispatched {task_id[:8]}... to {runner_id}")
        return True

    def _execute_task(self, task: Dict, runner_id: str) -> Dict:
        """Execute a task on a runner. Runs in thread pool."""
        task_id = task["id"]
        start_time = time.time()

        try:
            with self.telemetry.span(
                "task_execution",
                {
                    "task_id": task_id,
                    "task_type": task.get("type"),
                    "runner": runner_id,
                },
            ) as span:
                packet = self.task_manager.get_task_packet(task_id)
                prompt = (
                    packet.get("prompt") if packet else task.get("prompt_packet", "")
                )

                result = self._call_runner(runner_id, prompt, task)

                duration_ms = (time.time() - start_time) * 1000

                self.telemetry.record_task_execution(
                    task_id=task_id,
                    task_type=task.get("type", "unknown"),
                    model=runner_id,
                    duration_ms=duration_ms,
                    success=result.get("success", False),
                    tokens=result.get("tokens", 0),
                )

                db.table("task_runs").insert(
                    {
                        "task_id": task_id,
                        "model_id": runner_id,
                        "platform": self.runner_pool.runners.get(runner_id, {}).get(
                            "platform"
                        ),
                        "status": "success" if result.get("success") else "failed",
                        "result": result,
                        "tokens_in": result.get("prompt_tokens", 0),
                        "tokens_out": result.get("completion_tokens", 0),
                        "tokens_total": result.get("tokens", 0),
                        "duration_seconds": int(duration_ms / 1000),
                    }
                ).execute()

                if result.get("success"):
                    db.table("tasks").update(
                        {
                            "status": "review",
                            "result": result,
                            "tokens_used": result.get("tokens", 0),
                            "updated_at": datetime.utcnow().isoformat(),
                        }
                    ).eq("id", task_id).execute()
                else:
                    self.task_manager.handle_failure(
                        task_id,
                        result.get("error", "Unknown error"),
                        error_code=result.get("error_code"),
                    )

                return result

        except Exception as e:
            self.logger.error(f"Task {task_id[:8]} failed: {e}")

            self.task_manager.handle_failure(
                task_id, str(e), error_code="ORCHESTRATOR_ERROR"
            )

            return {"success": False, "error": str(e)}

        finally:
            self.runner_pool.release(runner_id)

    def _call_runner(self, runner_id: str, prompt: str, task: Dict) -> Dict:
        """Call the appropriate runner for the task."""
        runner_info = self.runner_pool.runners.get(runner_id, {})
        platform = runner_info.get("platform", "")

        if "kimi" in platform.lower():
            return self._call_kimi(prompt, task)
        elif "deepseek" in platform.lower():
            return self._call_deepseek(prompt, task)
        elif "gemini" in platform.lower():
            return self._call_gemini(prompt, task)
        else:
            return {"success": False, "error": f"Unknown platform: {platform}"}

    def _call_kimi(self, prompt: str, task: Dict) -> Dict:
        """Call Kimi CLI runner."""
        try:
            from runners.kimi_runner import KimiRunner

            runner = KimiRunner()
            return runner.execute_code_task(prompt)
        except Exception as e:
            return {"success": False, "error": str(e)}

    def _call_deepseek(self, prompt: str, task: Dict) -> Dict:
        """Call DeepSeek API runner."""
        try:
            from runners.api_runner import DeepSeekRunner

            runner = DeepSeekRunner()
            return runner.execute(prompt)
        except Exception as e:
            return {"success": False, "error": str(e)}

    def _call_gemini(self, prompt: str, task: Dict) -> Dict:
        """Call Gemini API runner."""
        try:
            from runners.api_runner import GeminiRunner

            runner = GeminiRunner()
            return runner.execute(prompt)
        except Exception as e:
            return {"success": False, "error": str(e)}

    def _check_completed_futures(self):
        """Check for completed task futures."""
        with self.lock:
            completed = []
            for task_id, future in self.active_tasks.items():
                if future.done():
                    completed.append(task_id)

            for task_id in completed:
                del self.active_tasks[task_id]

    def _process_reviews(self):
        """Process pending supervisor reviews."""
        result = self.supervisor.process_review_queue(max_tasks=5)
        if result["processed"] > 0:
            self.logger.info(
                f"Processed {result['processed']} reviews: "
                f"{result['approved']} approved, {result['rejected']} rejected"
            )

    def get_status(self) -> Dict:
        """Get current orchestrator status."""
        return {
            "running": self.running,
            "max_workers": self.max_workers,
            "active_tasks": len(self.active_tasks),
            "available_runners": len(self.runner_pool.get_available()),
            "total_runners": len(self.runner_pool.runners),
            "pending_reviews": len(self.supervisor.get_pending_reviews()),
        }

    def get_roi_report(self, period: str = "today") -> Dict:
        """Generate ROI report."""
        try:
            res = db.table("task_runs").select("*").execute()

            runs = res.data or []

            total_tokens = sum(r.get("tokens_total", 0) for r in runs)
            successful = [r for r in runs if r.get("status") == "success"]
            failed = [r for r in runs if r.get("status") == "failed"]

            theoretical_cost = total_tokens * 0.00001  # $0.01 per 1M tokens
            actual_cost = 0  # Using free tiers

            by_model = {}
            for run in runs:
                model = run.get("model_id", "unknown")
                if model not in by_model:
                    by_model[model] = {"tasks": 0, "success": 0, "tokens": 0}
                by_model[model]["tasks"] += 1
                if run.get("status") == "success":
                    by_model[model]["success"] += 1
                by_model[model]["tokens"] += run.get("tokens_total", 0)

            recommendations = []
            for model, stats in by_model.items():
                success_rate = (
                    stats["success"] / stats["tasks"] if stats["tasks"] > 0 else 0
                )
                if success_rate >= 0.9:
                    recommendations.append(
                        f"{model}: {success_rate:.0%} success - Recommend keeping"
                    )
                elif success_rate < 0.7:
                    recommendations.append(
                        f"{model}: {success_rate:.0%} success - Consider dropping"
                    )

            return {
                "period": period,
                "summary": {
                    "tasks_completed": len(successful),
                    "tasks_failed": len(failed),
                    "total_tokens": total_tokens,
                    "theoretical_cost": theoretical_cost,
                    "actual_cost": actual_cost,
                    "savings": theoretical_cost - actual_cost,
                },
                "by_model": by_model,
                "recommendations": recommendations,
            }
        except Exception as e:
            self.logger.error(f"ROI report failed: {e}")
            return {"error": str(e)}


if __name__ == "__main__":
    print("=" * 60)
    print("VIBEPILOT CONCURRENT ORCHESTRATOR")
    print("=" * 60)

    orch = ConcurrentOrchestrator(max_workers=5)

    status = orch.get_status()
    print(f"\nStatus:")
    print(f"  Max workers: {status['max_workers']}")
    print(
        f"  Available runners: {status['available_runners']}/{status['total_runners']}"
    )
    print(f"  Pending reviews: {status['pending_reviews']}")

    roi = orch.get_roi_report()
    if "summary" in roi:
        print(f"\nROI Report:")
        print(f"  Tasks completed: {roi['summary']['tasks_completed']}")
        print(f"  Total tokens: {roi['summary']['total_tokens']:,}")
        print(f"  Savings: ${roi['summary']['savings']:.2f}")

    print("\n" + "=" * 60)
    print("Ready to start. Run orch.start() to begin dispatch loop.")
    print("=" * 60)
