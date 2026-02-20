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
from agents.executioner import ExecutionerAgent
from agents.consultant import ConsultantAgent
from agents.planner import PlannerAgent
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
        self.logger = logging.getLogger("VibePilot.RunnerPool")
        self._load_runners(use_config)

    def _load_runners(self, use_config: bool = True):
        """Load active runners - try new access table first, then fallback."""
        self._load_from_database()

        if not self.runners:
            logger.info("No runners from access table, trying config...")
            if use_config:
                self._load_from_config()
            else:
                self._load_from_database_legacy()

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
        """Load active access methods from new access table."""
        try:
            res = (
                db.table("access")
                .select(
                    "id, model_id, tool_id, platform_id, method, priority, status, "
                    "requests_per_minute, requests_per_day, tokens_per_day, "
                    "requests_today, tokens_today, total_tasks, successful_tasks, "
                    "models_new(name, provider, capabilities, context_limit, cost_input_per_1k_usd, cost_output_per_1k_usd), "
                    "tools(name, type, has_codebase_access, has_browser_control, runner_class)"
                )
                .eq("status", "active")
                .order("priority")
                .execute()
            )

            for access in res.data or []:
                access_id = access["id"]
                model = access.get("models_new") or {}
                tool = access.get("tools") or {}

                model_id = access["model_id"]
                tool_id = access["tool_id"]
                method = access["method"]
                priority = access["priority"]

                capabilities = model.get("capabilities", []) or []
                has_browser = tool.get("has_browser_control", False) or False
                has_codebase = tool.get("has_codebase_access", False) or False

                if has_codebase and has_browser:
                    routing_capability = ["internal", "web", "mcp"]
                elif has_codebase:
                    routing_capability = ["internal", "mcp"]
                else:
                    routing_capability = ["web"]

                total = access.get("total_tasks", 0) or 0
                successful = access.get("successful_tasks", 0) or 0
                success_rate = successful / total if total > 0 else 0.5

                task_ratings = {}
                if total > 0:
                    task_ratings["default"] = {
                        "success": successful,
                        "fail": total - successful,
                        "count": total,
                    }

                runner_key = f"{model_id}:{tool_id}"

                self.runners[runner_key] = {
                    "id": runner_key,
                    "access_id": access_id,
                    "model_id": model_id,
                    "tool_id": tool_id,
                    "platform_id": access.get("platform_id"),
                    "name": model.get("name", model_id),
                    "provider": model.get("provider", "unknown"),
                    "type": tool.get("type", "unknown"),
                    "method": method,
                    "context_limit": model.get("context_limit", 32000),
                    "capabilities": capabilities,
                    "has_browser": has_browser,
                    "has_codebase": has_codebase,
                    "cost_priority": priority,
                    "routing_capability": routing_capability,
                    "task_ratings": task_ratings,
                    "success_rate": success_rate,
                    "status": "active",
                    "rate_limits": {
                        "rpm": access.get("requests_per_minute"),
                        "rpd": access.get("requests_per_day"),
                        "tpd": access.get("tokens_per_day"),
                    },
                    "current_usage": {
                        "requests_today": access.get("requests_today", 0) or 0,
                        "tokens_today": access.get("tokens_today", 0) or 0,
                    },
                    "runner_class": tool.get("runner_class"),
                }

                logger.debug(
                    f"Loaded {runner_key}: method={method}, pri={priority}, routing={routing_capability}"
                )

            logger.info(
                f"Loaded {len(self.runners)} active access methods from new schema"
            )

        except Exception as e:
            logger.warning(
                f"Could not load from access table: {e}, falling back to old models table"
            )
            self._load_from_database_legacy()

    def _load_from_database_legacy(self):
        """Legacy fallback: Load active models/runners from old models table."""
        res = db.table("models").select("*").eq("status", "active").execute()

        for model in res.data or []:
            model_id = model["id"]
            self.runners[model_id] = {
                "id": model_id,
                "model_id": model_id,
                "tool_id": model_id,
                "platform": model.get("platform", "unknown"),
                "type": model.get("type", "runner"),
                "context_limit": model.get("context_limit", 32000),
                "strengths": model.get("strengths", []),
                "task_ratings": model.get("task_ratings", {}),
                "status": model.get("status", "active"),
                "routing_capability": ["web"],
                "cost_priority": 2,
            }

    def is_available(self, runner_id: str) -> bool:
        """Check if runner is available (not busy, not in cooldown, under rate limits)."""
        with self.lock:
            if runner_id not in self.runners:
                return False
            if runner_id in self.busy:
                return False

        if self.cooldown_manager and self.cooldown_manager.is_in_cooldown(runner_id):
            return False

        if not self._check_rate_limits(runner_id):
            return False

        return True

    def _check_rate_limits(self, runner_id: str) -> bool:
        """Check if runner is under 80% of rate limits."""
        runner = self.runners.get(runner_id, {})
        rate_limits = runner.get("rate_limits", {})
        current_usage = runner.get("current_usage", {})

        if not rate_limits:
            return True

        rpd = rate_limits.get("rpd")
        tpd = rate_limits.get("tpd")

        if rpd:
            requests_today = current_usage.get("requests_today", 0)
            if requests_today >= rpd * 0.8:
                self.logger.debug(
                    f"{runner_id} at {requests_today}/{rpd} requests (80% threshold)"
                )
                return False

        if tpd:
            tokens_today = current_usage.get("tokens_today", 0)
            if tokens_today >= tpd * 0.8:
                self.logger.debug(
                    f"{runner_id} at {tokens_today}/{tpd} tokens (80% threshold)"
                )
                return False

        return True

    def record_usage(self, runner_id: str, tokens_in: int, tokens_out: int):
        """Record token usage for a runner."""
        runner = self.runners.get(runner_id)
        if not runner:
            return

        access_id = runner.get("access_id")
        if not access_id:
            return

        total_tokens = (tokens_in or 0) + (tokens_out or 0)

        try:
            db.rpc(
                "increment_access_usage",
                {"p_access_id": access_id, "p_tokens": total_tokens, "p_success": True},
            ).execute()

            current = runner.get("current_usage", {})
            current["requests_today"] = current.get("requests_today", 0) + 1
            current["tokens_today"] = current.get("tokens_today", 0) + total_tokens
            runner["current_usage"] = current

            self.logger.debug(f"Recorded {total_tokens} tokens for {runner_id}")
        except Exception as e:
            self.logger.warning(f"Could not record usage: {e}")

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
        self.supervisor.set_council_callback(self.route_council_review)
        self.task_manager = TaskManager()
        self.telemetry = get_telemetry()

        self.consultant = ConsultantAgent()
        self.planner = PlannerAgent()

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

        self._process_pending_ideas()

        unlocked = self.dependency_manager.unlock_ready_tasks()
        if unlocked:
            self.logger.info(f"Unlocked {len(unlocked)} tasks")

        pending_plans = self.supervisor.get_pending_plans()
        if pending_plans:
            self._process_pending_plans()

        pending_reviews = len(self.supervisor.get_pending_reviews())
        if pending_reviews > 0:
            self._process_reviews()

        testing_tasks = self._get_testing_tasks()
        if testing_tasks:
            self._process_testing_tasks(testing_tasks)

        available = self.runner_pool.get_available()
        if not available:
            return

        tasks = self._get_available_tasks(len(available))

        for task in tasks:
            self._dispatch_task(task)

    def _process_pending_ideas(self):
        """Process pending ideas from Vibes panel."""
        try:
            result = (
                db.table("vibes_ideas")
                .select("*")
                .eq("status", "pending")
                .limit(1)
                .execute()
            )

            if not result.data:
                return

            idea = result.data[0]
            idea_id = idea["id"]
            idea_text = idea["idea_text"]
            project_id = idea.get("project_id")

            self.logger.info(f"Processing idea: {idea_text[:50]}...")

            process_result = self.process_idea(
                idea_text, project_id, save_to_github=True
            )

            if process_result.get("success"):
                review_result = self.review_and_approve_plan(
                    process_result.get("plan_path"), project_id
                )

                db.table("vibes_ideas").update(
                    {
                        "status": "processed",
                        "prd_path": process_result.get("prd_path"),
                        "plan_path": process_result.get("plan_path"),
                        "processed_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", idea_id).execute()

                self.logger.info(
                    f"Idea processed: {review_result.get('tasks_created', 0)} tasks created"
                )
            else:
                db.table("vibes_ideas").update(
                    {"status": "failed", "processed_at": datetime.utcnow().isoformat()}
                ).eq("id", idea_id).execute()

                self.logger.error(
                    f"Idea processing failed: {process_result.get('error')}"
                )

        except Exception as e:
            self.logger.error(f"Error processing ideas: {e}")

    def _process_pending_plans(self):
        """Process pending plan tasks - review and approve."""
        pending = self.supervisor.get_pending_plans()
        if not pending:
            return

        self.logger.info(f"Processing {len(pending)} pending plan tasks")

        review = self.supervisor.review_plan()

        if not review["approved"]:
            self.logger.warning(f"Plan review failed: {review['issues']}")
            return

        if review.get("warnings"):
            for warning in review["warnings"]:
                self.logger.info(f"Plan warning: {warning}")

        if review["task_count"] > 0:
            council_result = self.supervisor.call_council(
                project_id=pending[0].get("project_id")
            )

            if council_result.get("concerns"):
                for concern in council_result["concerns"]:
                    self.logger.info(f"Council concern: {concern}")

        result = self.supervisor.approve_plan()

        if result.get("success"):
            self.logger.info(
                f"Approved {result.get('approved_count', 0)} tasks for execution"
            )
        else:
            self.logger.error(f"Plan approval failed: {result.get('error')}")

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

        attempts = task.get("attempts", 0)
        max_attempts = task.get("max_attempts", 3)
        if attempts >= max_attempts:
            self.logger.warning(
                f"Task {task_id[:8]} at max attempts ({attempts}/{max_attempts}), skipping dispatch"
            )
            if task.get("status") != "escalated":
                db.table("tasks").update(
                    {
                        "status": "escalated",
                        "assigned_to": None,
                        "updated_at": datetime.utcnow().isoformat(),
                    }
                ).eq("id", task_id).execute()
            return False

        runner_id = self.runner_pool.get_best_for_task(task)
        if not runner_id:
            self.logger.warning(f"No runner available for task {task_id[:8]}")
            return False

        if not self.runner_pool.acquire(runner_id, task_id):
            return False

        runner_info = self.runner_pool.runners.get(runner_id, {})
        model_id = runner_info.get(
            "model_id", runner_id.split(":")[0] if ":" in runner_id else runner_id
        )

        db.table("tasks").update(
            {
                "status": "in_progress",
                "assigned_to": model_id,
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

                self.runner_pool.record_usage(
                    runner_id,
                    result.get("prompt_tokens", 0),
                    result.get("completion_tokens", 0),
                )

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
                model_id = runner_info.get(
                    "model_id",
                    runner_id.split(":")[0] if ":" in runner_id else runner_id,
                )
                access_id = runner_info.get("access_id")

                run_insert = (
                    db.table("task_runs")
                    .insert(
                        {
                            "task_id": task_id,
                            "model_id": model_id,
                            "courier": runner_info.get("tool_id", "unknown"),
                            "platform": runner_info.get("platform_id")
                            or runner_info.get("provider", "unknown"),
                            "status": "success" if result.get("success") else "failed",
                            "result": result,
                            "tokens_in": result.get("prompt_tokens", 0),
                            "tokens_out": result.get("completion_tokens", 0),
                            "tokens_used": result.get("tokens", 0),
                        }
                    )
                    .execute()
                )

                if access_id:
                    try:
                        db.table("task_history").insert(
                            {
                                "task_id": task_id,
                                "access_id": access_id,
                                "task_type": task.get("type", "unknown"),
                                "actual_tokens_in": result.get("prompt_tokens", 0),
                                "actual_tokens_out": result.get("completion_tokens", 0),
                                "actual_requests": 1,
                                "success": result.get("success", False),
                                "failure_reason": result.get("error")
                                if not result.get("success")
                                else None,
                                "failure_code": result.get("error_code")
                                if not result.get("success")
                                else None,
                                "duration_ms": int(duration_ms),
                            }
                        ).execute()
                    except Exception as hist_err:
                        self.logger.debug(f"Task history log skipped: {hist_err}")

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

        runner_info = self.runner_pool.runners.get(runner_id, {})
        tool_id = runner_info.get("tool_id", runner_id)
        model_id = runner_info.get("model_id", runner_id)

        if tool_id == "kimi-cli" or "kimi" in tool_id.lower():
            runner_type = "kimi"
        elif tool_id == "opencode":
            runner_type = "kimi"
        elif tool_id == "direct-api":
            if "deepseek" in model_id.lower():
                runner_type = "deepseek"
            elif "gemini" in model_id.lower():
                runner_type = "gemini"
            else:
                runner_type = model_id
        elif tool_id == "courier":
            platform_id = runner_info.get("platform_id", "chatgpt")
            runner_type = f"courier-{platform_id}"
        else:
            runner_type = tool_id

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
                "model_id": model_id,
                "tool_id": tool_id,
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

    def _get_testing_tasks(self, limit: int = 5) -> List[Dict]:
        """Get tasks in testing status."""
        try:
            res = (
                db.table("tasks")
                .select("*")
                .eq("status", "testing")
                .limit(limit)
                .execute()
            )
            return res.data or []
        except Exception as e:
            self.logger.error(f"Failed to get testing tasks: {e}")
            return []

    def _process_testing_tasks(self, tasks: List[Dict]):
        """Run Executioner on testing tasks."""
        executioner = ExecutionerAgent()

        for task in tasks:
            task_id = task.get("id")
            work_dir = task.get("work_dir", os.getcwd())
            test_command = task.get("test_command", "pytest")

            self.logger.info(f"Running tests for task {task_id[:8]}")

            result = executioner.execute(
                {"action": "run_tests", "test_command": test_command, "cwd": work_dir}
            )

            if result.success:
                self.logger.info(f"Tests PASSED for task {task_id[:8]}")
                self.supervisor.process_test_results(
                    task_id,
                    {"passed": True, "test_type": "code", "output": result.output},
                )
            else:
                self.logger.warning(
                    f"Tests FAILED for task {task_id[:8]}: {result.error}"
                )
                self.supervisor.process_test_results(
                    task_id,
                    {
                        "passed": False,
                        "test_type": "code",
                        "failures": [result.error]
                        if result.error
                        else ["Unknown error"],
                    },
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

    # IDEA PROCESSING (Entry Layer)
    # Turns human ideas into PRD → Tasks → Queue
    # =========================================================================

    def process_idea(
        self, idea: str, project_id: str = None, save_to_github: bool = True
    ) -> Dict:
        """
        Process a human idea through Consultant → Planner → GitHub.

        This is the entry point for "Hey Vibes, I want X" requests.

        Flow:
        1. Consultant creates PRD
        2. Save PRD to GitHub: docs/prd/{slug}.md
        3. Planner creates Plan from PRD
        4. Save Plan to GitHub: docs/plans/{slug}-plan.md
        5. Return PRD path, Plan path, and task definitions

        After this, Council reviews the Plan from GitHub.
        If approved, call create_tasks_from_plan() to write tasks to Supabase.

        Args:
            idea: Human's description of what they want
            project_id: Optional project to associate tasks with
            save_to_github: If True, save PRD and Plan to GitHub

        Returns:
            {
                "success": bool,
                "prd_path": str,  # GitHub path to PRD
                "plan_path": str,  # GitHub path to Plan
                "tasks": List[Dict],  # Task definitions (not yet in DB)
                "task_count": int
            }
        """
        import re
        from datetime import datetime

        self.logger.info(f"Processing idea: {idea[:100]}...")

        slug = re.sub(r"[^a-z0-9]+", "-", idea.lower()[:30]).strip("-")
        if not slug:
            slug = f"idea-{datetime.now().strftime('%Y%m%d-%H%M%S')}"

        prd_result = self.consultant.execute({"description": idea})
        if not prd_result.success:
            self.logger.error(f"Consultant failed: {prd_result.error}")
            return {
                "success": False,
                "error": "Consultant failed",
                "details": prd_result.error,
            }

        prd = prd_result.output
        self.logger.info("PRD generated successfully")

        plan_result = self.planner.execute(
            {"prd": prd, "project_id": project_id}, write_to_db=False
        )
        if not plan_result.success:
            self.logger.error(f"Planner failed: {plan_result.error}")
            return {
                "success": False,
                "error": "Planner failed",
                "details": plan_result.error,
            }

        plan = plan_result.output.get("plan", {})
        tasks = plan_result.output.get("tasks", [])

        prd_path = f"docs/prd/{slug}.md"
        plan_path = f"docs/plans/{slug}-plan.md"

        if save_to_github:
            prd_commit = self._save_to_github(
                prd_path, prd, f"docs: Add PRD for {slug}"
            )
            if not prd_commit.get("success"):
                self.logger.warning(
                    f"Failed to save PRD to GitHub: {prd_commit.get('error')}"
                )

            plan_content = json.dumps(plan, indent=2)
            plan_commit = self._save_to_github(
                plan_path, plan_content, f"docs: Add plan for {slug}"
            )
            if not plan_commit.get("success"):
                self.logger.warning(
                    f"Failed to save Plan to GitHub: {plan_commit.get('error')}"
                )

        self.logger.info(f"Plan created with {len(tasks)} tasks")

        return {
            "success": True,
            "prd": prd,
            "prd_path": prd_path,
            "plan": plan,
            "plan_path": plan_path,
            "tasks": tasks,
            "task_count": len(tasks),
        }

    def _save_to_github(self, path: str, content: str, message: str) -> Dict:
        """
        Save a file to GitHub via Maintenance command queue.

        Args:
            path: File path (relative to repo root)
            content: File content
            message: Commit message

        Returns:
            {"success": bool, "command_id": str, "error": str}
        """
        import time
        from datetime import datetime

        try:
            idempotency_key = f"docs-{path.replace('/', '-')}-{int(time.time())}"

            result = (
                db.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "commit_code",
                        "payload": {
                            "branch": "main",
                            "files": [{"path": path, "content": content}],
                            "message": message,
                        },
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "orchestrator",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Queued save to GitHub: {path}")

            return {"success": True, "command_id": command_id, "path": path}

        except Exception as e:
            self.logger.error(f"Failed to queue save to GitHub: {e}")
            return {"success": False, "error": str(e)}

    def create_tasks_from_plan(self, plan_path: str, project_id: str = None) -> Dict:
        """
        Create tasks in Supabase from an approved Plan.

        Called after Council approves a Plan. Reads Plan from GitHub,
        extracts tasks, and writes them to Supabase with status 'pending'.

        Args:
            plan_path: Path to Plan file in GitHub (docs/plans/{slug}-plan.md)
            project_id: Optional project to associate tasks with

        Returns:
            {
                "success": bool,
                "tasks_written": int,
                "task_ids": List[str],
                "errors": List
            }
        """
        import os

        self.logger.info(f"Creating tasks from plan: {plan_path}")

        full_path = os.path.join(os.getcwd(), plan_path)

        try:
            with open(full_path, "r") as f:
                plan_content = f.read()

            plan = json.loads(plan_content)
        except FileNotFoundError:
            return {"success": False, "error": f"Plan file not found: {plan_path}"}
        except json.JSONDecodeError as e:
            return {"success": False, "error": f"Invalid JSON in plan: {e}"}

        all_tasks = []
        for slice_data in plan.get("slices", []):
            slice_id = slice_data.get("slice_id", "unknown")

            for phase in slice_data.get("phases", []):
                phase_id = phase.get("phase_id", "P1")

                for task in phase.get("tasks", []):
                    task["slice_id"] = task.get("slice_id", slice_id)
                    task["phase"] = task.get("phase", phase_id)
                    all_tasks.append(task)

        if not all_tasks:
            return {"success": False, "error": "No tasks found in plan"}

        written = 0
        task_ids = []
        errors = []

        for task in all_tasks:
            task_number = task.get("task_id", f"TASK-{written}")
            prompt_packet = task.get("prompt_packet", "")

            task_record = {
                "task_number": task_number,
                "title": task.get("title", "Untitled task"),
                "type": task.get("type", "feature"),
                "priority": task.get("priority", 5),
                "slice_id": task.get("slice_id"),
                "phase": task.get("phase"),
                "routing_flag": task.get("routing_flag", "web"),
                "routing_flag_reason": task.get("routing_flag_reason"),
                "status": "pending",
                "result": {
                    "prompt_packet": prompt_packet,
                    "expected_output": task.get("expected_output"),
                    "confidence": task.get("confidence"),
                },
            }

            if project_id:
                task_record["project_id"] = project_id

            try:
                result = db.table("tasks").insert(task_record).execute()
                if result.data:
                    task_ids.append(task_number)
                    written += 1
                    self.logger.info(f"Created task {task_number}")
            except Exception as e:
                errors.append({"task_id": task_number, "error": str(e)})
                self.logger.error(f"Failed to create task {task_number}: {e}")

        self.logger.info(f"Created {written} tasks from plan")

        return {
            "success": written > 0,
            "tasks_written": written,
            "task_ids": task_ids,
            "errors": errors,
        }

    # COUNCIL ROUTING (Phase C)
    # Routes council reviews to available models
    # =========================================================================

    def route_council_review(
        self,
        doc_path: str,
        lenses: List[str] = None,
        context_type: str = "project",
        timeout: int = 300,
        max_rounds: int = 4,
    ) -> Dict:
        """
        Route council review to available models with iterative consensus.

        Implements iterative deliberation: up to 4 rounds where models see
        each other's feedback and can revise their positions.

        Args:
            doc_path: Path to document to review (PRD or system doc)
            lenses: List of lenses to use ["user_alignment", "architecture", "feasibility"]
                   If None, uses default based on context_type
            context_type: "project" (PRD+Plan) or "system" (full context)
            timeout: Maximum seconds to wait for all reviews per round
            max_rounds: Maximum deliberation rounds (default 4)

        Returns:
            {
                "approved": bool,
                "consensus": str,  # "unanimous", "majority", "split"
                "reviews": Dict[str, Dict],
                "concerns": List[str],
                "recommendations": List[str],
                "rounds": int,
                "deliberation_history": List[Dict]
            }
        """
        if lenses is None:
            if context_type == "project":
                lenses = ["user_alignment", "architecture", "feasibility"]
            else:  # system
                lenses = ["architecture", "security", "integration", "reversibility"]

        self.logger.info(
            f"Routing council review: {len(lenses)} lenses, context={context_type}, max_rounds={max_rounds}"
        )

        # Get available models for council review
        available_models = self._get_council_models()

        if len(available_models) < len(lenses):
            self.logger.warning(
                f"Not enough models ({len(available_models)}) for {len(lenses)} lenses. "
                "Will reuse models for multiple lenses."
            )

        # Distribute lenses across models
        model_assignments = self._assign_lenses_to_models(lenses, available_models)

        # Iterative deliberation
        all_reviews = {}  # model_id -> List[review_dict] per round
        deliberation_history = []
        
        for round_num in range(1, max_rounds + 1):
            self.logger.info(f"Council deliberation round {round_num}/{max_rounds}")
            
            # Build context with previous rounds' feedback (for rounds > 1)
            previous_feedback = None
            if round_num > 1:
                previous_feedback = self._compile_deliberation_feedback(
                    all_reviews, round_num - 1
                )
            
            # Execute reviews in parallel for this round
            round_reviews = {}
            with ThreadPoolExecutor(max_workers=len(model_assignments)) as executor:
                futures = {}
                for model_id, assigned_lenses in model_assignments.items():
                    future = executor.submit(
                        self._execute_council_review,
                        model_id,
                        doc_path,
                        assigned_lenses,
                        context_type,
                        previous_feedback,
                        round_num,
                    )
                    futures[future] = model_id

                # Collect results with timeout
                for future in as_completed(futures, timeout=timeout):
                    model_id = futures[future]
                    try:
                        result = future.result()
                        round_reviews[model_id] = result
                    except Exception as e:
                        self.logger.error(f"Council review failed for {model_id} round {round_num}: {e}")
                        round_reviews[model_id] = {
                            "error": str(e),
                            "vote": "abstain",
                            "lenses": model_assignments[model_id],
                            "round": round_num,
                        }

            # Store reviews for this round
            for model_id, review in round_reviews.items():
                if model_id not in all_reviews:
                    all_reviews[model_id] = []
                all_reviews[model_id].append(review)

            # Aggregate results for this round
            aggregation = self._aggregate_council_reviews(round_reviews, lenses)
            aggregation["round"] = round_num
            deliberation_history.append(aggregation)
            
            self.logger.info(
                f"Round {round_num} result: {aggregation['consensus']} "
                f"({aggregation['votes']['approve']} approve, "
                f"{aggregation['votes']['reject']} reject, "
                f"{aggregation['votes']['needs_changes']} needs_changes)"
            )

            # Check if we have strong consensus
            if aggregation["consensus"] == "unanimous":
                self.logger.info(f"Unanimous consensus reached in round {round_num}")
                break
            elif aggregation["consensus"] == "majority" and round_num >= 2:
                # Strong majority after 2+ rounds is sufficient
                self.logger.info(f"Strong majority consensus reached in round {round_num}")
                break
            elif round_num == max_rounds:
                # Final round, use whatever we have
                self.logger.info(f"Max rounds reached ({max_rounds}), using final aggregation")
                break
            else:
                # Continue to next round with feedback
                self.logger.info(f"No strong consensus yet, continuing to round {round_num + 1}")

        # Final aggregation using last round's reviews
        final_reviews = {model_id: reviews[-1] for model_id, reviews in all_reviews.items()}
        final_result = self._aggregate_council_reviews(final_reviews, lenses)
        final_result["rounds"] = len(deliberation_history)
        final_result["deliberation_history"] = deliberation_history
        final_result["all_reviews"] = all_reviews
        
        return final_result

    def _compile_deliberation_feedback(self, all_reviews: Dict, last_round: int) -> str:
        """Compile feedback from previous rounds for iterative deliberation."""
        feedback = f"""
--- PREVIOUS DELIBERATION (Round {last_round}) ---

Summary of previous round reviews:
"""
        for model_id, reviews in all_reviews.items():
            if len(reviews) >= last_round:
                review = reviews[last_round - 1]
                vote = review.get("vote", "abstain")
                concerns = review.get("concerns", [])
                recommendations = review.get("recommendations", [])
                
                feedback += f"\n{model_id}:"
                feedback += f"\n  Vote: {vote}"
                if concerns:
                    feedback += f"\n  Concerns: {'; '.join(concerns[:3])}"
                if recommendations:
                    feedback += f"\n  Recommendations: {'; '.join(recommendations[:3])}"
                feedback += "\n"
        
        feedback += """
--- YOUR TASK ---

Consider the feedback above. You may:
1. Maintain your position with additional justification
2. Revise your vote based on others' perspectives
3. Address concerns raised by other reviewers

Provide your updated review below.
"""
        return feedback

    def _get_council_models(self) -> List[str]:
        """Get models available for council review."""
        available = []
        for runner_id, runner in self.runner_pool.runners.items():
            if self.runner_pool.is_available(runner_id):
                # Prefer models with good track record
                ratings = runner.get("task_ratings", {})
                council_rating = ratings.get("council_review", {})
                success_rate = council_rating.get("success", 0) / max(
                    council_rating.get("count", 1), 1
                )

                if success_rate >= 0.7 or council_rating.get("count", 0) < 5:
                    available.append(runner_id)

        # If no specific council experience, use any available
        if not available:
            available = self.runner_pool.get_available()

        return available[:3]  # Max 3 models for council

    def _assign_lenses_to_models(
        self, lenses: List[str], models: List[str]
    ) -> Dict[str, List[str]]:
        """Distribute lenses across available models."""
        if not models:
            # Use single model for all lenses if none available
            return {"kimi-cli": lenses}

        assignments = {}

        # If we have enough models, assign one lens per model
        if len(models) >= len(lenses):
            for i, lens in enumerate(lenses):
                model = models[i]
                assignments[model] = [lens]
        else:
            # Distribute lenses across available models
            for i, lens in enumerate(lenses):
                model = models[i % len(models)]
                if model not in assignments:
                    assignments[model] = []
                assignments[model].append(lens)

        return assignments

    def _execute_council_review(
        self, model_id: str, doc_path: str, lenses: List[str], context_type: str,
        previous_feedback: str = None, round_num: int = 1
    ) -> Dict:
        """Execute council review with a specific model."""
        # Read the document
        try:
            with open(doc_path, "r") as f:
                doc_content = f.read()
        except Exception as e:
            return {
                "error": f"Failed to read document: {e}",
                "vote": "abstain",
                "lenses": lenses,
            }

        # Build context based on type
        if context_type == "project":
            context = f"""
You are reviewing a PROJECT PLAN for VibePilot.

Document to review:
{doc_content[:5000]}...

Your assigned lenses: {", ".join(lenses)}

Review from your assigned perspective(s) and provide:
1. Your assessment of the plan
2. Specific concerns (if any)
3. Vote: approve / needs_changes / reject
4. Recommendations for improvement

Be thorough but concise.
"""
        else:  # system
            context = f"""
You are reviewing a SYSTEM IMPROVEMENT for VibePilot.

Document to review:
{doc_content[:5000]}...

Your assigned lenses: {", ".join(lenses)}

Review from your assigned perspective(s) and provide:
1. Your assessment of the improvement
2. Specific concerns (architecture, security, integration)
3. Vote: approve / needs_changes / reject
4. Recommendations for improvement

Consider VibePilot principles: zero vendor lock-in, modular, reversible, exit-ready.
"""

        # Add round information and previous feedback for iterative deliberation
        if round_num > 1:
            context = f"""{context}

--- ROUND {round_num} ---
This is deliberation round {round_num}. Previous rounds have occurred.
"""
            if previous_feedback:
                context = f"""{context}

{previous_feedback}
"""
        else:
            context = f"""{context}

--- ROUND {round_num} ---
This is the first round of deliberation.
"""

        # Dispatch to model via runner
        self.logger.info(f"Dispatching council review to {model_id} for lenses: {lenses}")

        try:
            # Get runner info from pool
            runner_info = self.runner_pool.runners.get(model_id, {})
            if not runner_info:
                # Try to find by model_id substring match
                for rid, info in self.runner_pool.runners.items():
                    if model_id in rid or model_id in info.get("model_id", ""):
                        runner_info = info
                        model_id = rid
                        break

            # Build task packet for council review
            task_packet = {
                "task_id": f"council-review-{datetime.utcnow().strftime('%Y%m%d%H%M%S')}",
                "prompt": context,
                "title": f"Council Review - {context_type}",
                "constraints": {
                    "max_tokens": 2000,
                    "timeout_seconds": 120,
                },
                "runner_context": {
                    "work_dir": os.getcwd(),
                },
            }

            # Determine runner type
            tool_id = runner_info.get("tool_id", model_id)
            runner_model_id = runner_info.get("model_id", model_id)

            if tool_id == "kimi-cli" or "kimi" in tool_id.lower():
                runner_type = "kimi"
            elif tool_id == "opencode":
                runner_type = "kimi"
            elif tool_id == "direct-api":
                if "deepseek" in runner_model_id.lower():
                    runner_type = "deepseek"
                elif "gemini" in runner_model_id.lower():
                    runner_type = "gemini"
                else:
                    runner_type = runner_model_id
            else:
                runner_type = tool_id

            # Execute via contract runner
            from runners.contract_runners import get_runner
            runner = get_runner(runner_type)
            result = runner.execute(task_packet)

            if result.get("status") != "success":
                errors = result.get("errors", [])
                error_msg = errors[0].get("message", "Unknown error") if errors else "Runner failed"
                self.logger.error(f"Council review failed for {model_id}: {error_msg}")
                return {
                    "model_id": model_id,
                    "lenses": lenses,
                    "vote": "abstain",
                    "concerns": [f"Review execution failed: {error_msg}"],
                    "recommendations": [],
                    "round": round_num,
                    "timestamp": datetime.utcnow().isoformat(),
                }

            # Parse the response to extract vote, concerns, recommendations
            output = result.get("output", "")
            parsed = self._parse_council_response(output, lenses)

            self.logger.info(f"Council review from {model_id}: {parsed['vote']}")

            return {
                "model_id": model_id,
                "lenses": lenses,
                "vote": parsed["vote"],
                "concerns": parsed["concerns"],
                "recommendations": parsed["recommendations"],
                "round": round_num,
                "raw_response": output[:1000],  # Truncated for logging
                "timestamp": datetime.utcnow().isoformat(),
            }

        except Exception as e:
            self.logger.error(f"Council review error for {model_id}: {e}")
            return {
                "model_id": model_id,
                "lenses": lenses,
                "vote": "abstain",
                "concerns": [f"Review execution error: {str(e)}"],
                "recommendations": [],
                "round": round_num,
                "timestamp": datetime.utcnow().isoformat(),
            }

    def _parse_council_response(self, response: str, lenses: List[str]) -> Dict:
        """Parse council review response to extract structured data.
        
        Expected format in response:
        - Vote: approve / needs_changes / reject
        - Concerns: list of concerns
        - Recommendations: list of recommendations
        """
        response_lower = response.lower()
        
        # Determine vote
        vote = "abstain"
        if "vote: approve" in response_lower or "vote:approve" in response_lower:
            vote = "approve"
        elif "vote: reject" in response_lower or "vote:reject" in response_lower:
            vote = "reject"
        elif "vote: needs_changes" in response_lower or "vote:needs_changes" in response_lower:
            vote = "needs_changes"
        elif "approve" in response_lower and "reject" not in response_lower:
            vote = "approve"
        elif "reject" in response_lower:
            vote = "reject"
        elif "needs changes" in response_lower or "needs_changes" in response_lower:
            vote = "needs_changes"
        
        # Extract concerns and recommendations
        concerns = []
        recommendations = []
        
        lines = response.split('\n')
        current_section = None
        
        for line in lines:
            line_stripped = line.strip()
            line_lower = line_stripped.lower()
            
            # Section detection
            if any(x in line_lower for x in ['concern:', 'concerns:', 'issues:', 'problems:']):
                current_section = 'concerns'
                # Check if content on same line
                content = line_stripped.split(':', 1)[1].strip()
                if content and not content.startswith('http'):
                    concerns.append(content)
                continue
            elif any(x in line_lower for x in ['recommendation:', 'recommendations:', 'suggestions:']):
                current_section = 'recommendations'
                content = line_stripped.split(':', 1)[1].strip()
                if content and not content.startswith('http'):
                    recommendations.append(content)
                continue
            
            # Content collection
            if line_stripped.startswith('- ') or line_stripped.startswith('* '):
                content = line_stripped[2:].strip()
                if content and not content.lower().startswith('vote:'):
                    if current_section == 'concerns':
                        concerns.append(content)
                    elif current_section == 'recommendations':
                        recommendations.append(content)
            elif line_stripped and current_section and not line_stripped.lower().startswith('vote:'):
                # Continuation or new item without bullet
                if current_section == 'concerns':
                    concerns.append(line_stripped)
                elif current_section == 'recommendations':
                    recommendations.append(line_stripped)
        
        # If no explicit sections found, do basic extraction
        if not concerns and not recommendations:
            sentences = response.split('.')
            for sent in sentences:
                sent = sent.strip()
                if any(word in sent.lower() for word in ['concern', 'issue', 'problem', 'risk', 'warning']):
                    concerns.append(sent)
                elif any(word in sent.lower() for word in ['recommend', 'suggest', 'improve', 'consider']):
                    recommendations.append(sent)
        
        return {
            "vote": vote,
            "concerns": concerns[:5],  # Limit to top 5
            "recommendations": recommendations[:5],
        }

    def _aggregate_council_reviews(
        self, reviews: Dict[str, Dict], lenses: List[str]
    ) -> Dict:
        """Aggregate council reviews into consensus decision."""
        votes = []
        all_concerns = []
        all_recommendations = []

        for model_id, review in reviews.items():
            vote = review.get("vote", "abstain")
            votes.append(vote)
            all_concerns.extend(review.get("concerns", []))
            all_recommendations.extend(review.get("recommendations", []))

        # Determine consensus
        approve_count = votes.count("approve")
        reject_count = votes.count("reject")
        needs_changes_count = votes.count("needs_changes")
        total_votes = len([v for v in votes if v != "abstain"])

        if total_votes == 0:
            consensus = "no_quorum"
            approved = False
        elif approve_count == total_votes:
            consensus = "unanimous"
            approved = True
        elif approve_count > total_votes / 2:
            consensus = "majority"
            approved = True
        else:
            consensus = "split"
            approved = False

        return {
            "approved": approved,
            "consensus": consensus,
            "votes": {
                "approve": approve_count,
                "reject": reject_count,
                "needs_changes": needs_changes_count,
                "total": total_votes,
            },
            "reviews": reviews,
            "concerns": list(set(all_concerns)),
            "recommendations": list(set(all_recommendations)),
        }

    # =========================================================================
    # RATE LIMIT COUNTDOWN (Phase C)
    # Shows time until platforms are available again
    # =========================================================================

    def get_rate_limit_status(self) -> Dict[str, Dict]:
        """
        Get rate limit status for all platforms with countdowns.

        Returns:
            {
                "platform_id": {
                    "status": "available" | "cooldown" | "paused",
                    "available_in_seconds": int | None,
                    "available_in_human": str | None,  # "4h 23m"
                    "daily_remaining": int,
                    "daily_limit": int,
                    "usage_percent": float  # 0.0 to 100.0
                }
            }
        """
        status = {}

        for runner_id, runner in self.runner_pool.runners.items():
            platform_id = runner.get("platform_id", runner_id)

            # Check cooldown
            cooldown_remaining = None
            if self.runner_pool.cooldown_manager:
                cooldown_remaining = (
                    self.runner_pool.cooldown_manager.get_cooldown_remaining(runner_id)
                )

            # Get rate limits
            rate_limits = runner.get("rate_limits", {})
            current_usage = runner.get("current_usage", {})

            rpd = rate_limits.get("rpd")
            requests_today = current_usage.get("requests_today", 0)

            # Calculate status
            if cooldown_remaining:
                platform_status = "cooldown"
                available_in = cooldown_remaining
            elif rpd and requests_today >= rpd * 0.8:
                platform_status = "paused"
                # Estimate reset time (midnight UTC or rolling)
                available_in = self._estimate_reset_time(platform_id)
            else:
                platform_status = "available"
                available_in = None

            # Calculate percentage
            usage_percent = (requests_today / rpd * 100) if rpd else 0

            status[platform_id] = {
                "status": platform_status,
                "available_in_seconds": available_in,
                "available_in_human": self._format_duration(available_in)
                if available_in
                else None,
                "daily_remaining": max(0, (rpd or 0) - requests_today),
                "daily_limit": rpd,
                "usage_percent": round(usage_percent, 1),
            }

        return status

    def _estimate_reset_time(self, platform_id: str) -> Optional[int]:
        """Estimate seconds until rate limit resets."""
        # Platform-specific reset logic
        reset_times = {
            "chatgpt": 5 * 60 * 60,  # 5 hours rolling
            "claude": 12 * 60 * 60,  # 12 hours
            "gemini": 24 * 60 * 60,  # 24 hours
            "deepseek": 24 * 60 * 60,  # 24 hours
        }

        for key, seconds in reset_times.items():
            if key in platform_id.lower():
                return seconds

        return 6 * 60 * 60  # Default 6 hours

    def _format_duration(self, seconds: int) -> str:
        """Format seconds as human-readable duration."""
        hours = seconds // 3600
        minutes = (seconds % 3600) // 60

        if hours > 0:
            return f"{hours}h {minutes}m"
        else:
            return f"{minutes}m"

    def log_rate_limit_status(self):
        """Log current rate limit status for all platforms."""
        status = self.get_rate_limit_status()

        self.logger.info("Rate Limit Status:")
        for platform, info in status.items():
            if info["status"] == "available":
                self.logger.info(
                    f"  ✅ {platform}: {info['daily_remaining']}/{info['daily_limit']} remaining"
                )
            elif info["status"] == "cooldown":
                self.logger.info(
                    f"  ⏲️  {platform}: Cooldown - available in {info['available_in_human']}"
                )
            else:
                self.logger.info(
                    f"  ⏸️  {platform}: Paused at 80% - available in {info['available_in_human']}"
                )

    def review_and_approve_plan(self, plan_path: str, project_id: str = None) -> Dict:
        """
        Review a Plan from GitHub via Council, and if approved, create tasks.

        This is the complete flow after process_idea():
        1. process_idea() creates PRD and Plan in GitHub
        2. review_and_approve_plan() reviews Plan via Council
        3. If approved, creates tasks in Supabase
        4. Tasks then flow through normal pipeline (pending → available → execution)

        Args:
            plan_path: Path to Plan file in GitHub (docs/plans/{slug}-plan.md)
            project_id: Optional project to associate tasks with

        Returns:
            {
                "success": bool,
                "approved": bool,
                "council_result": Dict,
                "tasks_created": int,
                "task_ids": List[str]
            }
        """
        self.logger.info(f"Reviewing plan: {plan_path}")

        council_result = self.route_council_review(
            doc_path=plan_path,
            lenses=["user_alignment", "architecture", "feasibility"],
            context_type="project",
        )

        if not council_result.get("approved"):
            self.logger.warning(f"Plan NOT approved by Council")
            return {
                "success": False,
                "approved": False,
                "council_result": council_result,
                "tasks_created": 0,
                "task_ids": [],
            }

        self.logger.info("Plan approved by Council, creating tasks...")

        create_result = self.create_tasks_from_plan(plan_path, project_id)

        return {
            "success": create_result.get("success", False),
            "approved": True,
            "council_result": council_result,
            "tasks_created": create_result.get("tasks_written", 0),
            "task_ids": create_result.get("task_ids", []),
            "errors": create_result.get("errors", []),
        }


if __name__ == "__main__":
    print("ConcurrentOrchestrator loaded. Use via import.")
