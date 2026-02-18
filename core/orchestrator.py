"""
VibePilot Concurrent Orchestrator

Multi-agent task dispatcher with:
- Concurrent task execution
- Dependency-aware scheduling
- Runner pool management
- ROI tracking
- Model performance learning
- Usage tracking with 80% cooldown

This replaces the single-threaded orchestrator.py
"""

import os
import json
import time
import logging
import threading
import yaml
from typing import Dict, Any, Optional, List
from datetime import datetime, timedelta
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

# Default cooldown duration when hitting 80%
COOLDOWN_MINUTES = 60


class CooldownManager:
    """
    Tracks cooldowns for runners based on usage limits.
    When a runner hits 80% of its limits, it enters cooldown.
    """

    def __init__(self):
        self.cooldowns: Dict[str, datetime] = {}  # runner_id -> cooldown_expires_at
        self.lock = threading.Lock()
        self.logger = logging.getLogger("VibePilot.Cooldown")

    def is_in_cooldown(self, runner_id: str) -> bool:
        """Check if runner is in cooldown."""
        with self.lock:
            if runner_id not in self.cooldowns:
                return False

            expires_at = self.cooldowns[runner_id]
            if datetime.utcnow() >= expires_at:
                # Cooldown expired
                del self.cooldowns[runner_id]
                self.logger.info(f"Cooldown expired for {runner_id}")
                return False

            return True

    def get_cooldown_remaining(self, runner_id: str) -> Optional[int]:
        """Get seconds remaining in cooldown, or None if not in cooldown."""
        with self.lock:
            if runner_id not in self.cooldowns:
                return None

            expires_at = self.cooldowns[runner_id]
            remaining = (expires_at - datetime.utcnow()).total_seconds()
            if remaining <= 0:
                del self.cooldowns[runner_id]
                return None

            return int(remaining)

    def set_cooldown(
        self, runner_id: str, minutes: int = COOLDOWN_MINUTES, reason: str = ""
    ):
        """Put runner in cooldown."""
        with self.lock:
            expires_at = datetime.utcnow() + timedelta(minutes=minutes)
            self.cooldowns[runner_id] = expires_at

            # Also update database for dashboard visibility
            try:
                db.table("models").update(
                    {
                        "status": "paused",
                        "status_reason": f"Cooldown: {reason}",
                        "cooldown_expires_at": expires_at.isoformat(),
                        "updated_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", runner_id).execute()
            except Exception as e:
                self.logger.warning(f"Could not update cooldown in DB: {e}")

            self.logger.info(
                f"Runner {runner_id} in cooldown for {minutes}min: {reason}"
            )

    def clear_cooldown(self, runner_id: str):
        """Clear cooldown for a runner."""
        with self.lock:
            if runner_id in self.cooldowns:
                del self.cooldowns[runner_id]

            try:
                db.table("models").update(
                    {
                        "status": "active",
                        "status_reason": None,
                        "cooldown_expires_at": None,
                        "updated_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", runner_id).execute()
            except Exception as e:
                self.logger.warning(f"Could not clear cooldown in DB: {e}")


class UsageTracker:
    """
    Tracks usage per runner and triggers cooldown at 80%.
    Reads limits from Supabase models table (config JSONB).
    """

    PAUSE_THRESHOLD = 0.80  # 80%

    def __init__(self, cooldown_manager: CooldownManager):
        self.cooldown_manager = cooldown_manager
        self.usage: Dict[str, Dict] = {}  # runner_id -> {requests, tokens}
        self.lock = threading.Lock()
        self.logger = logging.getLogger("VibePilot.Usage")

    def get_limits(self, runner_id: str) -> Dict:
        """Get limits for a runner from Supabase."""
        try:
            result = (
                db.table("models")
                .select("config, request_limit, token_limit")
                .eq("id", runner_id)
                .execute()
            )
            if result.data:
                row = result.data[0]
                config = row.get("config", {})
                return {
                    "request_limit": row.get("request_limit")
                    or config.get("request_limit"),
                    "token_limit": row.get("token_limit") or config.get("token_limit"),
                    "rate_limits": config.get("rate_limits", {}),
                }
        except Exception as e:
            self.logger.warning(f"Could not get limits for {runner_id}: {e}")

        return {}

    def record_usage(self, runner_id: str, requests: int = 1, tokens: int = 0):
        """Record usage and check if cooldown needed."""
        with self.lock:
            if runner_id not in self.usage:
                self.usage[runner_id] = {"requests": 0, "tokens": 0}

            self.usage[runner_id]["requests"] += requests
            self.usage[runner_id]["tokens"] += tokens

        # Check limits
        limits = self.get_limits(runner_id)
        self._check_threshold(runner_id, limits)

    def _check_threshold(self, runner_id: str, limits: Dict):
        """Check if usage hit 80% threshold."""
        request_limit = limits.get("request_limit")
        token_limit = limits.get("token_limit")

        with self.lock:
            usage = self.usage.get(runner_id, {"requests": 0, "tokens": 0})

        # Check request limit
        if request_limit:
            request_pct = usage["requests"] / request_limit
            if request_pct >= self.PAUSE_THRESHOLD:
                pct = int(request_pct * 100)
                self.cooldown_manager.set_cooldown(
                    runner_id,
                    reason=f"Request usage at {pct}% ({usage['requests']}/{request_limit})",
                )
                return

        # Check token limit
        if token_limit:
            token_pct = usage["tokens"] / token_limit
            if token_pct >= self.PAUSE_THRESHOLD:
                pct = int(token_pct * 100)
                self.cooldown_manager.set_cooldown(
                    runner_id,
                    reason=f"Token usage at {pct}% ({usage['tokens']}/{token_limit})",
                )
                return


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
    Tracks which are busy, which have capacity, and cooldown status.

    Loads from config/models.json (primary) with database fallback.
    """

    def __init__(
        self, use_config: bool = True, cooldown_manager: CooldownManager = None
    ):
        self.runners: Dict[str, Dict] = {}
        self.busy: Dict[str, str] = {}  # runner_id -> task_id
        self.lock = threading.Lock()
        self.cooldown_manager = cooldown_manager
        self._load_runners(use_config)

    def _load_runners(self, use_config: bool = True):
        """Load active models/runners from config or database."""
        if use_config:
            self._load_from_config()
        else:
            self._load_from_database()

        logger.info(f"Loaded {len(self.runners)} runners")

    def _load_from_config(self):
        """
        Load runners from config/models.json merged with database status.

        Database is source of truth for: status, cooldown, pause reason
        Config is source of truth for: capabilities, routing, cost type
        """
        from core.config_loader import get_config_loader

        config = get_config_loader()
        models = config.get_models()

        # Get database status for all models
        db_status = {}
        try:
            res = (
                db.table("models")
                .select("id, status, status_reason, cooldown_expires_at, strengths")
                .execute()
            )
            for m in res.data or []:
                db_status[m["id"]] = m
        except Exception as e:
            logger.warning(f"Could not load model status from database: {e}")

        for model in models:
            model_id = model.get("id")

            # Check database status first (overrides config)
            db_info = db_status.get(model_id, {})
            db_status_val = db_info.get("status", "active")

            # Skip if database says paused/benched
            if db_status_val in ("paused", "benched", "offline"):
                reason = db_info.get("status_reason", "unknown")
                logger.info(
                    f"Skipping {model_id}: database status={db_status_val}, reason={reason}"
                )
                continue

            access_type = model.get("access_type", "api")
            capabilities = model.get("capabilities", [])

            # Determine routing capabilities
            # CLI/API runners have codebase access → can handle internal
            # Some internal runners ALSO have browser_control (Kimi) → can handle web
            has_browser = "browser_control" in capabilities or "vision" in capabilities
            has_codebase = access_type in ("cli", "api", "cli_subscription")

            if has_codebase and has_browser:
                routing_capability = ["internal", "web", "mcp"]
            elif has_codebase:
                routing_capability = ["internal", "mcp"]
            else:
                routing_capability = ["web"]

            # Determine cost category for scoring
            # Subscription (sunk cost) > Free API > Paid API
            if access_type == "cli_subscription":
                cost_priority = 0  # Best - already paid, unlimited
            elif model.get("cost_input_per_1k_usd", 0) == 0:
                cost_priority = 1  # Free API
            else:
                cost_priority = 2  # Paid API - use sparingly

            self.runners[model_id] = {
                "id": model_id,
                "name": model.get("name", model_id),
                "platform": model.get("platform", model.get("provider", "unknown")),
                "type": access_type,
                "context_limit": model.get("context_limit", 32000),
                "capabilities": capabilities,
                "has_browser": has_browser,
                "cost_priority": cost_priority,
                "task_ratings": {},
                "status": "active",
                "routing_capability": routing_capability,
                "config": model,
                "strengths": db_info.get("strengths", model.get("strengths", [])),
            }

            logger.debug(
                f"Loaded {model_id}: routing={routing_capability}, browser={has_browser}, cost_priority={cost_priority}"
            )

    def _load_from_database(self):
        """Load active models/runners from database (fallback)."""
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

    def is_available(self, runner_id: str) -> bool:
        """Check if runner is available (not busy and not in cooldown)."""
        with self.lock:
            if runner_id not in self.runners:
                return False
            if runner_id in self.busy:
                return False

        # Check cooldown
        if self.cooldown_manager and self.cooldown_manager.is_in_cooldown(runner_id):
            return False

        return True

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

        First filters by routing capability:
        - internal (Q): Only CLI/API runners with codebase access
        - web (W): Any runner
        - mcp (M): Only MCP-capable runners

        Then scores by performance and task type fit.
        """
        task_type = task.get("type", "feature")
        priority = task.get("priority", 5)
        dependencies = task.get("dependencies", [])
        routing_flag = task.get("routing_flag", "web")  # Default to web-capable

        available = self.get_available()
        if not available:
            return None

        # Filter by routing capability
        capable_runners = []
        for runner_id in available:
            runner = self.runners.get(runner_id, {})
            capability = runner.get("routing_capability", ["web"])
            if routing_flag in capability:
                capable_runners.append(runner_id)

        if not capable_runners:
            self.logger.warning(
                f"No runners with routing_capability={routing_flag} for task"
            )
            return None

        # Score remaining runners
        # Priority: browser capability (for web) > cost priority > success rate > strengths
        scored = []
        for runner_id in capable_runners:
            runner = self.runners[runner_id]

            ratings = runner.get("task_ratings", {}).get(task_type, {})
            success_rate = 0.5
            if ratings.get("count", 0) > 0:
                total = ratings["success"] + ratings["fail"]
                success_rate = ratings["success"] / total if total > 0 else 0.5

            # Cost priority: 0=subscription (best), 1=free API, 2=paid API
            cost_priority = runner.get("cost_priority", 2)
            cost_score = 1.0 - (cost_priority * 0.3)  # 1.0, 0.7, 0.4

            # Browser capability bonus for web tasks
            browser_bonus = 0.0
            if routing_flag == "web" and runner.get("has_browser"):
                browser_bonus = 0.5

            # Strengths match
            strengths = runner.get("strengths", [])
            strength_score = 1.0 if task_type in strengths else 0.5

            # Final score: weighted combination
            # Browser capability for web > cost > success rate > strengths
            w1, w2, w3, w4, w5 = 0.1, 0.3, 0.25, 0.15, 0.2
            score = (
                w1 * (priority / 10)
                + w2 * cost_score
                + w3 * success_rate
                + w4 * strength_score
                + w5 * (1.0 + browser_bonus)
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
    - Usage tracking with 80% cooldown

    Concurrency is dynamic - scales based on available runners and tasks.
    """

    def __init__(self, max_workers: int = None, config_path: str = None):
        """
        Initialize orchestrator.

        Args:
            max_workers: Maximum concurrent tasks. None = from config or auto
            config_path: Path to config file for settings
        """
        self.cooldown_manager = CooldownManager()
        self.usage_tracker = UsageTracker(self.cooldown_manager)
        self.runner_pool = RunnerPool(cooldown_manager=self.cooldown_manager)
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
        """
        Get available tasks with satisfied dependencies.

        Uses the database function get_available_for_routing which
        respects the current runner capabilities.

        Falls back to simple query if function not available.
        """
        # Determine what routing flags our runners can handle
        can_web = False
        can_internal = False
        can_mcp = False

        for runner_id, runner in self.runner_pool.runners.items():
            capability = runner.get("routing_capability", ["web"])
            if "web" in capability:
                can_web = True
            if "internal" in capability:
                can_internal = True
            if "mcp" in capability:
                can_mcp = True

        self.logger.debug(
            f"Runner capabilities: web={can_web}, internal={can_internal}, mcp={can_mcp}"
        )
        self.logger.debug(f"Active runners: {list(self.runner_pool.runners.keys())}")

        try:
            # Try using the RPC function first
            res = db.rpc(
                "get_available_for_routing",
                {
                    "p_can_web": can_web,
                    "p_can_internal": can_internal,
                    "p_can_mcp": can_mcp,
                },
            ).execute()

            if res.data:
                return res.data[:limit]
        except Exception as e:
            self.logger.debug(f"RPC get_available_for_routing not available: {e}")

        # Fallback to simple query
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
                routing_flag = task.get("routing_flag", "web")

                if routing_flag == "web" and can_web:
                    tasks.append(task)
                elif routing_flag == "internal" and can_internal:
                    tasks.append(task)
                elif routing_flag == "mcp" and can_mcp:
                    tasks.append(task)
                elif routing_flag == "web" and can_internal:
                    # FALLBACK: No courier available, use internal with browser capability
                    # Kimi (subscription, multimodal) should pick up web tasks
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

                runner_info = self.runner_pool.runners.get(runner_id, {})
                runner_type = runner_info.get("type", "api")

                run_insert = (
                    db.table("task_runs")
                    .insert(
                        {
                            "task_id": task_id,
                            "model_id": runner_id,
                            "courier": runner_id.split("-")[0]
                            if "-" in runner_id
                            else runner_id,
                            "platform": runner_info.get("platform", "unknown"),
                            "status": "success" if result.get("success") else "failed",
                            "result": result,
                            "tokens_in": result.get("prompt_tokens", 0),
                            "tokens_out": result.get("completion_tokens", 0),
                            "tokens_used": result.get("tokens", 0),
                        }
                    )
                    .execute()
                )

                # Calculate ROI for this run
                if run_insert.data:
                    run_id = run_insert.data[0].get("id")
                    if run_id:
                        try:
                            db.rpc(
                                "calculate_enhanced_task_roi", {"p_run_id": run_id}
                            ).execute()
                        except Exception as roi_err:
                            self.logger.debug(f"ROI calculation skipped: {roi_err}")

                if result.get("success"):
                    db.table("tasks").update(
                        {
                            "status": "review",
                            "result": result,
                            "updated_at": datetime.utcnow().isoformat(),
                        }
                    ).eq("id", task_id).execute()
                else:
                    error_code = result.get("error_code")
                    if error_code in ["QUOTA_EXHAUSTED", "CREDIT_NEEDED"]:
                        self._handle_runner_error(
                            runner_id,
                            error_code,
                            retry_after_seconds=result.get("retry_after_seconds"),
                        )
                    self.task_manager.handle_failure(
                        task_id,
                        result.get("error", "Unknown error"),
                        error_code=error_code,
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
        """Call the appropriate runner for the task using contract runners."""
        from runners.contract_runners import get_runner

        task_packet = {
            "task_id": task.get("id"),
            "prompt": prompt,
            "title": task.get("title", ""),
            "constraints": {
                "max_tokens": 4000,
                "timeout_seconds": 300,
            },
            "runner_context": {
                "work_dir": task.get("work_dir", os.getcwd()),
            },
        }

        runner_type = runner_id
        if "kimi" in runner_id.lower():
            runner_type = "kimi"
        elif "deepseek" in runner_id.lower():
            runner_type = "deepseek"
        elif "gemini" in runner_id.lower() and "api" in runner_id.lower():
            runner_type = "gemini"
        elif "gemini" in runner_id.lower():
            runner_type = "gemini"

        try:
            runner = get_runner(runner_type)
            result = runner.execute(task_packet)
            errors = result.get("errors", [])
            error_info = errors[0] if errors else {}
            return {
                "success": result.get("status") == "success",
                "output": result.get("output"),
                "error": error_info.get("message") if errors else None,
                "error_code": error_info.get("code") if errors else None,
                "retry_after_seconds": result.get("metadata", {}).get(
                    "retry_after_seconds"
                ),
                "tokens": result.get("metadata", {}).get("tokens_in", 0)
                + result.get("metadata", {}).get("tokens_out", 0),
                "prompt_tokens": result.get("metadata", {}).get("tokens_in", 0),
                "completion_tokens": result.get("metadata", {}).get("tokens_out", 0),
            }
        except Exception as e:
            return {"success": False, "error": str(e), "error_code": "RUNNER_EXCEPTION"}

    def _handle_runner_error(
        self, runner_id: str, error_code: str, retry_after_seconds: int = None
    ):
        """
        Handle runner errors that require status changes.

        - QUOTA_EXHAUSTED (429): Set cooldown with countdown timer
        - CREDIT_NEEDED (402): Pause and flag for review ($ icon)
        """
        if error_code == "QUOTA_EXHAUSTED":
            retry_minutes = (retry_after_seconds or 3600) // 60
            reason = "quota_exhausted"

            if self.cooldown_manager:
                self.cooldown_manager.set_cooldown(
                    runner_id, minutes=retry_minutes, reason=reason
                )

            try:
                expires_at = datetime.utcnow() + timedelta(minutes=retry_minutes)
                db.table("models").update(
                    {
                        "status": "paused",
                        "status_reason": reason,
                        "cooldown_expires_at": expires_at.isoformat(),
                        "updated_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", runner_id).execute()
                self.logger.info(
                    f"Model {runner_id} paused for quota exhaustion, resumes in {retry_minutes}m"
                )
            except Exception as e:
                self.logger.error(f"Failed to update model status: {e}")

        elif error_code == "CREDIT_NEEDED":
            reason = "credit_needed"

            try:
                db.table("models").update(
                    {
                        "status": "paused",
                        "status_reason": reason,
                        "cooldown_expires_at": None,
                        "updated_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", runner_id).execute()
                self.logger.warning(
                    f"Model {runner_id} paused - CREDIT NEEDED (flagged for review)"
                )
            except Exception as e:
                self.logger.error(f"Failed to update model status: {e}")

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
            "max_workers": self._get_max_workers(),
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
