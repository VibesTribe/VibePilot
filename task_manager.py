import os
import json
import logging
from typing import Optional, Dict, Any, List
from datetime import datetime
from supabase import create_client
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s | %(levelname)s | %(message)s"
)
logger = logging.getLogger("VibePilot.TaskManager")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)


class TaskManager:
    def __init__(
        self,
        model_id: str = "deepseek-chat",
        courier: str = "opencode",
        platform: str = "deepseek-api",
    ):
        self.model_id = model_id
        self.courier = courier
        self.platform = platform
        self.logger = logger

    def get_active_models(self) -> List[Dict]:
        res = db.table("models").select("*").eq("status", "active").execute()
        return res.data if res.data else []

    def get_model_status(self, model_id: str = None) -> Optional[Dict]:
        mid = model_id or self.model_id
        res = db.table("models").select("*").eq("id", mid).execute()
        return res.data[0] if res.data else None

    def check_model_limits(self, model_id: str = None) -> Dict:
        model = self.get_model_status(model_id)
        if not model:
            return {"ok": False, "reason": "Model not found"}

        if model["status"] != "active":
            return {
                "ok": False,
                "reason": model.get("status_reason", "Model not active"),
            }

        req_pct = (
            (model.get("request_used", 0) / model.get("request_limit", 1)) * 100
            if model.get("request_limit")
            else 0
        )
        tok_pct = (
            (model.get("token_used", 0) / model.get("token_limit", 1)) * 100
            if model.get("token_limit")
            else 0
        )

        if req_pct >= 80 or tok_pct >= 80:
            return {
                "ok": False,
                "reason": f"Approaching limit (req:{req_pct:.0f}%, tok:{tok_pct:.0f}%)",
            }

        return {"ok": True, "request_used": req_pct, "token_used": tok_pct}

    def claim_next_task(self) -> Optional[Dict]:
        try:
            result = db.rpc(
                "claim_next_task",
                {
                    "p_courier": self.courier,
                    "p_platform": self.platform,
                    "p_model_id": self.model_id,
                },
            ).execute()

            if result.data:
                task_id = result.data
                task = self.get_task(task_id)
                self.logger.info(f"Claimed task {task_id}")
                return task
            return None
        except Exception as e:
            self.logger.error(f"Failed to claim task: {e}")
            return None

    def get_available_tasks(self) -> List[Dict]:
        try:
            result = db.rpc("get_available_tasks").execute()
            return result.data if result.data else []
        except Exception as e:
            self.logger.error(f"Failed to get available tasks: {e}")
            return []

    def get_task(self, task_id: str) -> Optional[Dict]:
        res = db.table("tasks").select("*").eq("id", task_id).execute()
        return res.data[0] if res.data else None

    def get_task_packet(self, task_id: str, version: int = None) -> Optional[Dict]:
        query = db.table("task_packets").select("*").eq("task_id", task_id)
        if version:
            query = query.eq("version", version)
        res = query.order("version", desc=True).limit(1).execute()
        return res.data[0] if res.data else None

    def create_task(
        self,
        title: str,
        task_type: str,
        priority: int = 5,
        dependencies: List[str] = None,
    ) -> str:
        deps = dependencies or []
        res = (
            db.table("tasks")
            .insert(
                {
                    "title": title,
                    "type": task_type,
                    "priority": priority,
                    "dependencies": deps,
                    "status": "available" if not deps else "pending",
                }
            )
            .execute()
        )
        task_id = res.data[0]["id"]
        self.logger.info(f"Created task {task_id}: {title}")
        return task_id

    def create_task_packet(
        self,
        task_id: str,
        prompt: str,
        tech_spec: Dict = None,
        expected_output: str = None,
        context: Dict = None,
    ) -> str:
        existing = (
            db.table("task_packets")
            .select("version")
            .eq("task_id", task_id)
            .order("version", desc=True)
            .limit(1)
            .execute()
        )
        next_version = (existing.data[0]["version"] + 1) if existing.data else 1

        res = (
            db.table("task_packets")
            .insert(
                {
                    "task_id": task_id,
                    "prompt": prompt,
                    "tech_spec": tech_spec or {},
                    "expected_output": expected_output,
                    "context": context or {},
                    "version": next_version,
                }
            )
            .execute()
        )
        return res.data[0]["id"]

    def start_run(self, task_id: str, chat_url: str = None) -> str:
        res = (
            db.table("task_runs")
            .insert(
                {
                    "task_id": task_id,
                    "courier": self.courier,
                    "platform": self.platform,
                    "model_id": self.model_id,
                    "chat_url": chat_url,
                    "status": "running",
                }
            )
            .execute()
        )
        return res.data[0]["id"]

    def complete_run(
        self,
        run_id: str,
        success: bool,
        result: Dict = None,
        tokens_used: int = 0,
        error: str = None,
    ):
        status = "success" if success else "failed"
        db.table("task_runs").update(
            {
                "status": status,
                "result": result,
                "tokens_used": tokens_used,
                "error": error,
                "completed_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", run_id).execute()

        if tokens_used > 0:
            self._update_model_usage(tokens_used)

    def update_task_status(
        self, task_id: str, status: str, result: Dict = None, failure_notes: str = None
    ):
        update_data = {"status": status, "updated_at": datetime.utcnow().isoformat()}

        if result:
            update_data["result"] = result
        if failure_notes:
            update_data["failure_notes"] = failure_notes
        if status == "merged":
            update_data["completed_at"] = datetime.utcnow().isoformat()

        db.table("tasks").update(update_data).eq("id", task_id).execute()
        self.logger.info(f"Task {task_id} status → {status}")

        if status == "merged":
            self._check_dependent_tasks(task_id)

    def add_review(
        self, task_id: str, passed: bool, notes: str, reviewer: str = "system"
    ):
        task = self.get_task(task_id)
        review = task.get("review") or {}
        review = {
            "passed": passed,
            "notes": notes,
            "reviewer": reviewer,
            "reviewed_at": datetime.utcnow().isoformat(),
        }

        if passed:
            self.update_task_status(task_id, "testing", {"review": review})
        else:
            db.table("tasks").update(
                {
                    "status": "in_progress",
                    "review": review,
                    "failure_notes": notes,
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()

    def add_test_results(
        self, task_id: str, passed: bool, results: List[Dict], coverage: int = None
    ):
        task = self.get_task(task_id)
        tests = {
            "passed": passed,
            "results": results,
            "coverage": coverage,
            "tested_at": datetime.utcnow().isoformat(),
        }

        if passed:
            self.update_task_status(task_id, "approval", {"tests": tests})
        else:
            db.table("tasks").update(
                {
                    "status": "in_progress",
                    "tests": tests,
                    "failure_notes": f"Tests failed: {len([r for r in results if not r.get('passed')])} failures",
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()

    def approve_and_merge(
        self, task_id: str, merged_by: str = "maintenance", branch_name: str = None
    ):
        approval = {
            "passed": True,
            "merged_by": merged_by,
            "merged_at": datetime.utcnow().isoformat(),
        }

        update_data = {
            "status": "merged",
            "approval": approval,
            "completed_at": datetime.utcnow().isoformat(),
            "updated_at": datetime.utcnow().isoformat(),
        }

        if branch_name:
            update_data["branch_name"] = branch_name

        db.table("tasks").update(update_data).eq("id", task_id).execute()
        self.logger.info(f"Task {task_id} MERGED by {merged_by}")

        self._check_dependent_tasks(task_id)

    def _update_model_usage(self, tokens: int):
        model = self.get_model_status()
        if model:
            db.table("models").update(
                {
                    "request_used": (model.get("request_used", 0) + 1),
                    "token_used": (model.get("token_used", 0) + tokens),
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", self.model_id).execute()

    def _check_dependent_tasks(self, completed_task_id: str):
        try:
            result = db.rpc(
                "make_task_available", {"p_task_id": completed_task_id}
            ).execute()
            self.logger.info(f"Checked dependents for {completed_task_id}")
        except Exception as e:
            self.logger.error(f"Failed to check dependents: {e}")

    def handle_failure(
        self,
        task_id: str,
        reason: str,
        error_details: str = None,
        error_code: str = None,
    ):
        task = self.get_task(task_id)
        attempts = task.get("attempts", 0)
        max_attempts = task.get("max_attempts", 3)
        new_attempts = attempts + 1

        notes = f"[{error_code or 'UNKNOWN'}] Attempt {new_attempts}/{max_attempts}: {reason}"
        if error_details:
            notes += f"\nDetails: {error_details}"

        if new_attempts >= max_attempts:
            db.table("tasks").update(
                {
                    "status": "escalated",
                    "attempts": new_attempts,
                    "assigned_to": None,
                    "failure_notes": notes,
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()
            self.logger.warning(
                f"Task {task_id} ESCALATED after {new_attempts} attempts"
            )

            self._trigger_escalation_flow(task_id, notes)

            return {
                "escalated": True,
                "reason": "Max attempts reached",
                "flow": "supervisor_review",
            }
        else:
            db.table("tasks").update(
                {
                    "status": "available",
                    "attempts": new_attempts,
                    "assigned_to": None,
                    "failure_notes": notes,
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()
            self.logger.info(
                f"Task {task_id} returned to queue (attempt {new_attempts}/{max_attempts})"
            )
            return {
                "escalated": False,
                "remaining_attempts": max_attempts - new_attempts,
            }

    def _trigger_escalation_flow(self, task_id: str, notes: str):
        """
        Escalation flow for failed tasks:
        1. Supervisor reviews failure
        2. Supervisor calls Planner for reassignment options
        3. If architecture issue → Council review
        4. Options: Reassign, Split, Refine prompt, Council review
        """
        self.logger.info(f"Escalation flow triggered for task {task_id}")

        task = self.get_task(task_id)
        existing_result = task.get("result") or {}

        escalation_event = {
            "task_id": task_id,
            "triggered_at": datetime.utcnow().isoformat(),
            "status": "awaiting_supervisor_review",
            "notes": notes,
            "options": ["reassign", "split", "refine_prompt", "council_review"],
        }

        existing_result["escalation"] = escalation_event

        db.table("tasks").update({"result": existing_result}).eq(
            "id", task_id
        ).execute()

    def supervisor_process_escalation(self, task_id: str, action: str, **kwargs):
        """
        Supervisor processes escalated task.

        Actions:
        - 'reassign': Assign to different model/platform
        - 'split': Break into subtasks
        - 'refine': Update prompt and retry
        - 'council': Send to Council for review
        """
        if action == "reassign":
            return self.reassign_task(
                task_id,
                new_model_id=kwargs.get("model_id"),
                refined_prompt=kwargs.get("refined_prompt"),
            )
        elif action == "split":
            return self.split_task(task_id, kwargs.get("subtasks", []))
        elif action == "refine":
            return self.reassign_task(task_id, refined_prompt=kwargs.get("prompt"))
        elif action == "council":
            return self._send_to_council(
                task_id, kwargs.get("council_type", "plan_review")
            )
        else:
            return {"success": False, "error": f"Unknown action: {action}"}

    def _send_to_council(self, task_id: str, council_type: str):
        """Send task to Council for review."""
        task = self.get_task(task_id)

        council_request = {
            "task_id": task_id,
            "council_type": council_type,
            "triggered_at": datetime.utcnow().isoformat(),
            "task_data": task,
            "status": "pending_council_review",
        }

        # Council reviews independently (no chat between agents)
        # Each member receives: system summary + task + role prompt

        self.logger.info(f"Task {task_id} sent to Council for {council_type}")
        return {"success": True, "council_request": council_request}

    def get_escalated_tasks(self) -> List[Dict]:
        res = db.table("tasks").select("*").eq("status", "escalated").execute()
        return res.data if res.data else []

    def reassign_task(
        self, task_id: str, new_model_id: str = None, refined_prompt: str = None
    ):
        task = self.get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        update_data = {
            "status": "available",
            "attempts": 0,
            "assigned_to": None,
            "updated_at": datetime.utcnow().isoformat(),
        }

        db.table("tasks").update(update_data).eq("id", task_id).execute()

        if refined_prompt:
            self.create_task_packet(
                task_id,
                refined_prompt,
                task.get("result", {}).get("tech_spec"),
                task.get("result", {}).get("expected_output"),
            )
            self.logger.info(f"Task {task_id} reassigned with refined prompt")

        self.logger.info(
            f"Task {task_id} reassigned to {new_model_id or 'any available model'}"
        )
        return {"success": True}

    def split_task(self, task_id: str, subtasks: List[Dict]) -> List[str]:
        task = self.get_task(task_id)
        if not task:
            return []

        subtask_ids = []
        for i, subtask in enumerate(subtasks):
            sub_id = self.create_task(
                title=f"{task.get('title', 'Task')} - Part {i + 1}",
                task_type=task.get("type", "feature"),
                priority=task.get("priority", 5),
                dependencies=subtask.get("dependencies", []),
            )
            self.create_task_packet(
                sub_id,
                subtask.get("prompt"),
                subtask.get("tech_spec"),
                subtask.get("expected_output"),
            )
            subtask_ids.append(sub_id)

        db.table("tasks").update(
            {
                "status": "merged",
                "failure_notes": f"Split into {len(subtask_ids)} subtasks: {', '.join(subtask_ids)}",
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} split into {len(subtask_ids)} subtasks")
        return subtask_ids

    def needs_human_approval(self, task_id: str) -> bool:
        task = self.get_task(task_id)
        task_type = task.get("type", "")
        return task_type == "ui_ux"

    def approve_ui_ux(self, task_id: str, approved_by: str, notes: str = None):
        task = self.get_task(task_id)
        approval = task.get("approval") or {}
        approval["human_approved"] = True
        approval["approved_by"] = approved_by
        approval["human_notes"] = notes
        approval["human_approved_at"] = datetime.utcnow().isoformat()

        self.approve_and_merge(task_id, merged_by=f"human:{approved_by}")
        self.logger.info(f"UI/UX task {task_id} approved by human: {approved_by}")


if __name__ == "__main__":
    tm = TaskManager()

    print("=== Active Models ===")
    for m in tm.get_active_models():
        print(
            f"  {m['id']}: {m['status']} (requests: {m.get('request_used', 0)}/{m.get('request_limit', '?')})"
        )

    print("\n=== Available Tasks ===")
    for t in tm.get_available_tasks():
        print(f"  [{t['priority']}] {t['id'][:8]}... {t.get('title', 'No title')}")

    print("\n=== Model Limits Check ===")
    check = tm.check_model_limits()
    print(f"  OK: {check['ok']}")
    if not check["ok"]:
        print(f"  Reason: {check['reason']}")
