"""
VibePilot Supervisor Agent

Reviews task outputs, coordinates testing, performs final merge approval.
The gatekeeper between execution and production.

See prompts/supervisor.md for full behavior specification.
"""

import os
import json
import logging
from typing import Dict, Any, Optional, List
from datetime import datetime
from supabase import create_client
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s | %(levelname)s | %(message)s"
)
logger = logging.getLogger("VibePilot.Supervisor")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)


class SupervisorAgent:
    """
    Supervisor Agent Implementation.

    Responsibilities:
    - Review task outputs against specifications
    - Approve/reject based on quality criteria
    - Coordinate testing (code + visual)
    - Perform final merge approval
    - Unlock dependent tasks
    - Update model performance ratings
    """

    def __init__(self):
        self.logger = logger

    def review_task_output(self, task_id: str, output: Dict, expected: Dict) -> Dict:
        """
        Review a completed task's output.

        Returns:
            {
                "approved": bool,
                "issues": [...],
                "warnings": [...],
                "next_action": "approve" | "reject" | "test" | "human_review"
            }
        """
        issues = []
        warnings = []

        files_created = output.get("files_created", [])
        files_expected = expected.get("files_created", [])

        missing_files = set(files_expected) - set(files_created)
        if missing_files:
            issues.append(f"Missing files: {missing_files}")

        extra_files = set(files_created) - set(files_expected)
        if extra_files:
            warnings.append(f"Extra files created: {extra_files}")

        api_endpoints = output.get("api_endpoints", [])
        api_expected = expected.get("api_endpoints", [])
        if api_expected:
            missing_api = [e for e in api_expected if e not in api_endpoints]
            if missing_api:
                issues.append(f"Missing API endpoints: {missing_api}")

        tests = output.get("tests", [])
        tests_expected = expected.get("tests_required", [])
        if tests_expected:
            missing_tests = set(tests_expected) - set(t.get("name", "") for t in tests)
            if missing_tests:
                issues.append(f"Missing tests: {missing_tests}")

        code_quality = self._check_code_quality(output)
        issues.extend(code_quality.get("issues", []))
        warnings.extend(code_quality.get("warnings", []))

        has_visual = expected.get("visual_change", False)
        has_security = expected.get("security_impact", False)

        if issues:
            return {
                "approved": False,
                "issues": issues,
                "warnings": warnings,
                "next_action": "reject",
            }

        if has_visual:
            return {
                "approved": False,
                "issues": [],
                "warnings": warnings,
                "next_action": "human_review",
                "reason": "Visual changes require human approval",
            }

        if has_security:
            warnings.append("Security-impacting change - ensure security review")

        return {
            "approved": True,
            "issues": [],
            "warnings": warnings,
            "next_action": "test",
        }

    def _check_code_quality(self, output: Dict) -> Dict:
        """Basic code quality checks."""
        issues = []
        warnings = []

        code = output.get("code", "")

        if "TODO" in code or "FIXME" in code:
            warnings.append("Code contains TODO/FIXME comments")

        if "print(" in code and "def " in code:
            warnings.append("Code contains print statements - consider logging")

        if "password" in code.lower() and "hardcoded" not in code.lower():
            pass

        return {"issues": issues, "warnings": warnings}

    def approve_task(self, task_id: str, reviewer: str = "supervisor") -> Dict:
        """Approve a task and unlock dependents."""
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        db.table("tasks").update(
            {
                "status": "approved",
                "approval": {
                    "approved_by": reviewer,
                    "approved_at": datetime.utcnow().isoformat(),
                },
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} APPROVED by {reviewer}")

        self._unlock_dependents(task_id)

        return {"success": True, "task_id": task_id, "status": "approved"}

    def reject_task(self, task_id: str, reason: str, reassign: bool = True) -> Dict:
        """Reject a task and optionally reassign."""
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        attempts = task.get("attempts", 0)
        max_attempts = task.get("max_attempts", 3)

        if attempts >= max_attempts:
            db.table("tasks").update(
                {
                    "status": "escalated",
                    "failure_notes": reason,
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()

            self.logger.warning(f"Task {task_id} ESCALATED: {reason}")
            return {"success": True, "escalated": True}

        if reassign:
            db.table("tasks").update(
                {
                    "status": "available",
                    "attempts": attempts + 1,
                    "failure_notes": reason,
                    "assigned_to": None,
                    "updated_at": datetime.utcnow().isoformat(),
                }
            ).eq("id", task_id).execute()

            self.logger.info(f"Task {task_id} REJECTED and reassigned: {reason}")
            return {"success": True, "reassigned": True, "attempts": attempts + 1}

        db.table("tasks").update(
            {
                "status": "in_progress",
                "failure_notes": reason,
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} REJECTED (same agent): {reason}")
        return {"success": True, "reassigned": False}

    def route_to_testing(self, task_id: str, test_type: str = "code") -> Dict:
        """Route task to appropriate tester."""
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        db.table("tasks").update(
            {
                "status": "testing",
                
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} routed to {test_type} testing")

        return {
            "success": True,
            "task_id": task_id,
            
            "next": f"tester_{test_type}",
        }

    def process_test_results(self, task_id: str, results: Dict) -> Dict:
        """Process test results and decide next action."""
        passed = results.get("passed", False)
        test_type = results.get("test_type", "code")

        if passed:
            if test_type == "visual":
                return self.route_to_human_review(
                    task_id, "Visual tests passed, needs human approval"
                )
            else:
                return self.approve_task(task_id, f"tester_{test_type}")
        else:
            failures = results.get("failures", [])
            reason = f"Tests failed: {len(failures)} failures - {failures[:3]}"
            return self.reject_task(task_id, reason, reassign=False)

    def route_to_human_review(self, task_id: str, reason: str) -> Dict:
        """Route task for human visual review."""
        db.table("tasks").update(
            {
                "status": "awaiting_human",
                "notes": reason,
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} awaiting human review: {reason}")

        return {
            "success": True,
            "task_id": task_id,
            "status": "awaiting_human",
            "reason": reason,
        }

    def human_approve(self, task_id: str, approved_by: str, notes: str = None) -> Dict:
        """Process human approval for visual tasks."""
        return self.approve_task(task_id, f"human:{approved_by}")

    def final_merge(self, task_id: str, branch_name: str = None) -> Dict:
        """
        Final merge to main.

        This is the last gate before production.
        """
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        if task.get("status") != "approved":
            return {
                "success": False,
                "error": f"Task status is {task.get('status')}, not approved",
            }

        db.table("tasks").update(
            {
                "status": "merged",
                "branch_name": branch_name,
                "completed_at": datetime.utcnow().isoformat(),
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} MERGED to main")

        self._unlock_dependents(task_id)

        self._update_model_rating(task)

        return {
            "success": True,
            "task_id": task_id,
            "status": "merged",
            "branch": branch_name,
        }

    def _get_task(self, task_id: str) -> Optional[Dict]:
        """Get task from database."""
        res = db.table("tasks").select("*").eq("id", task_id).execute()
        return res.data[0] if res.data else None

    def _unlock_dependents(self, completed_task_id: str):
        """Unlock tasks that were waiting on this task."""
        try:
            result = db.rpc(
                "unlock_dependent_tasks", {"p_completed_task_id": completed_task_id}
            ).execute()

            if result.data:
                self.logger.info(f"Unlocked {len(result.data)} dependent tasks")
        except Exception as e:
            self.logger.warning(f"Could not unlock dependents: {e}")

    def _update_model_rating(self, task: Dict):
        """Update model performance rating based on task outcome."""
        model_id = task.get("assigned_to")
        if not model_id:
            return

        task_type = task.get("type", "unknown")
        success = task.get("status") == "merged"
        tokens = task.get("tokens_used", 0)

        try:
            model = (
                db.table("models").select("task_ratings").eq("id", model_id).execute()
            )
            if not model.data:
                return

            ratings = model.data[0].get("task_ratings", {})

            if task_type not in ratings:
                ratings[task_type] = {
                    "success": 0,
                    "fail": 0,
                    "avg_tokens": 0,
                    "count": 0,
                }

            r = ratings[task_type]
            if success:
                r["success"] += 1
            else:
                r["fail"] += 1
            r["avg_tokens"] = ((r["avg_tokens"] * r["count"]) + tokens) / (
                r["count"] + 1
            )
            r["count"] += 1
            ratings[task_type] = r

            db.table("models").update(
                {"task_ratings": ratings, "updated_at": datetime.utcnow().isoformat()}
            ).eq("id", model_id).execute()

            self.logger.debug(
                f"Updated rating for {model_id}: {task_type} success={success}"
            )
        except Exception as e:
            self.logger.warning(f"Could not update model rating: {e}")

    def get_pending_reviews(self) -> List[Dict]:
        """Get all tasks awaiting supervisor review."""
        res = db.table("tasks").select("*").eq("status", "review").execute()
        return res.data if res.data else []

    def get_pending_approvals(self) -> List[Dict]:
        """Get all tasks awaiting final approval."""
        res = db.table("tasks").select("*").eq("status", "approved").execute()
        return res.data if res.data else []

    def get_awaiting_human(self) -> List[Dict]:
        """Get all tasks awaiting human review."""
        res = db.table("tasks").select("*").eq("status", "awaiting_human").execute()
        return res.data if res.data else []

    def process_review_queue(self, max_tasks: int = 10) -> Dict:
        """Process all pending reviews."""
        reviews = self.get_pending_reviews()
        processed = 0
        approved = 0
        rejected = 0

        for task in reviews[:max_tasks]:
            output = task.get("result", {})
            expected = task.get("expected_output", {})

            review = self.review_task_output(task["id"], output, expected)

            if review["next_action"] == "reject":
                self.reject_task(task["id"], "; ".join(review["issues"]))
                rejected += 1
            elif review["next_action"] == "test":
                self.route_to_testing(task["id"])
                approved += 1
            elif review["next_action"] == "human_review":
                self.route_to_human_review(
                    task["id"], review.get("reason", "Needs human review")
                )
                approved += 1
            else:
                self.approve_task(task["id"])
                approved += 1

            processed += 1

        return {"processed": processed, "approved": approved, "rejected": rejected}


if __name__ == "__main__":
    print("=== VibePilot Supervisor Agent ===\n")

    supervisor = SupervisorAgent()

    print("Pending reviews:")
    reviews = supervisor.get_pending_reviews()
    print(f"  {len(reviews)} tasks awaiting review")

    print("\nPending approvals:")
    approvals = supervisor.get_pending_approvals()
    print(f"  {len(approvals)} tasks awaiting final approval")

    print("\nAwaiting human:")
    human = supervisor.get_awaiting_human()
    print(f"  {len(human)} tasks awaiting human review")

    print("\n✅ Supervisor agent ready")
