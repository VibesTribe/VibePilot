"""
VibePilot Supervisor Agent

Reviews task outputs, coordinates testing, performs final merge approval.
The gatekeeper between execution and production.

See prompts/supervisor.md for full behavior specification.
"""

import os
import json
import time
import logging
from typing import Dict, Any, Optional, List
from datetime import datetime
from supabase import create_client
from dotenv import load_dotenv
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from vault_manager import get_env_or_vault

load_dotenv()

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s | %(levelname)s | %(message)s"
)
logger = logging.getLogger("VibePilot.Supervisor")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")
SUPABASE_SERVICE_KEY = get_env_or_vault("SUPABASE_SERVICE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)
db_service = (
    create_client(SUPABASE_URL, SUPABASE_SERVICE_KEY) if SUPABASE_SERVICE_KEY else db
)


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

    def __init__(self, council_callback=None):
        self.logger = logger
        self.council_callback = council_callback

    def set_council_callback(self, callback):
        """Set the Council review callback (injected by orchestrator)."""
        self.council_callback = callback

    def set_orchestrator(self, orchestrator):
        """Set the orchestrator reference for LLM routing."""
        self.orchestrator = orchestrator

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
        """Approve a task, execute git operations, and unlock dependents."""
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        branch_name = task.get("branch_name") or f"task/{task_id[:8]}"
        module = task.get("module", "general")
        module_branch = f"module/{module}"
        task_title = task.get("title", "Automated commit")

        db.table("tasks").update(
            {
                "status": "approved",
                "approval": {
                    "approved_by": reviewer,
                    "approved_at": datetime.utcnow().isoformat(),
                },
                "branch_name": branch_name,
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} APPROVED by {reviewer}")

        create_result = self.command_create_branch(
            task_id=task_id, branch_name=branch_name, base_branch="main"
        )

        if not create_result.get("success"):
            self.logger.warning(
                f"Branch creation queued but may have issues: {create_result}"
            )

        commit_result = self.command_commit_changes(
            task_id=task_id,
            branch=branch_name,
            message=f"Task {task_id[:8]}: {task_title}",
        )

        if not commit_result.get("success"):
            self.logger.warning(f"Commit queued but may have issues: {commit_result}")

        self._unlock_dependents(task_id)

        return {
            "success": True,
            "task_id": task_id,
            "status": "approved",
            "branch": branch_name,
            "git_commands_queued": {
                "create_branch": create_result.get("success", False),
                "commit_changes": commit_result.get("success", False),
            },
        }

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

    def final_merge(
        self, task_id: str, branch_name: str = None, target: str = None
    ) -> Dict:
        """
        Final merge from task branch to module or main.

        Args:
            task_id: Task ID
            branch_name: Source branch (task branch)
            target: Target branch (module/{name} or main). If None, uses module branch.
        """
        task = self._get_task(task_id)
        if not task:
            return {"success": False, "error": "Task not found"}

        if task.get("status") != "approved":
            return {
                "success": False,
                "error": f"Task status is {task.get('status')}, not approved",
            }

        source_branch = branch_name or task.get("branch_name") or f"task/{task_id[:8]}"
        module = task.get("module", "general")
        target_branch = target or f"module/{module}"

        merge_result = self.command_merge_branch(
            task_id=task_id,
            source=source_branch,
            target=target_branch,
            delete_source=True,
        )

        if not merge_result.get("success"):
            self.logger.warning(f"Merge command queued with issues: {merge_result}")

        db.table("tasks").update(
            {
                "status": "merged",
                "branch_name": source_branch,
                "completed_at": datetime.utcnow().isoformat(),
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()

        self.logger.info(f"Task {task_id} MERGED to {target_branch}")

        self._unlock_dependents(task_id)

        self._update_model_rating(task)

        return {
            "success": True,
            "task_id": task_id,
            "status": "merged",
            "source_branch": source_branch,
            "target_branch": target_branch,
            "merge_command_queued": merge_result.get("success", False),
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

    def get_pending_plans(self) -> List[Dict]:
        """Get all tasks awaiting plan approval (pre-execution)."""
        res = db.table("tasks").select("*").eq("status", "pending").execute()
        return res.data if res.data else []

    def review_plan(self, project_id: str = None) -> Dict:
        """
        Review pending plan tasks before execution.

        This is the pre-execution gate. Supervisor checks:
        - Tasks have complete prompt packets
        - Dependencies are valid
        - No obvious issues

        Returns:
            {"approved": bool, "issues": [...], "task_count": int}
        """
        query = db.table("tasks").select("*").eq("status", "pending")
        if project_id:
            query = query.eq("project_id", project_id)

        res = query.execute()
        pending_tasks = res.data or []

        if not pending_tasks:
            return {
                "approved": True,
                "issues": [],
                "task_count": 0,
                "message": "No pending tasks",
            }

        issues = []
        warnings = []

        for task in pending_tasks:
            task_id = task["id"][:8]
            title = task.get("title", "")

            if not task.get("routing_flag"):
                issues.append(f"Task {task_id} missing routing_flag")

            if not task.get("title"):
                warnings.append(f"Task {task_id} has no title")

            deps = task.get("dependencies") or []
            if deps:
                for dep_id in deps:
                    dep_check = (
                        db.table("tasks").select("id").eq("id", dep_id).execute()
                    )
                    if not dep_check.data:
                        issues.append(
                            f"Task {task_id} has invalid dependency: {dep_id[:8]}"
                        )

        if issues:
            self.logger.warning(f"Plan review found {len(issues)} issues")
            return {
                "approved": False,
                "issues": issues,
                "warnings": warnings,
                "task_count": len(pending_tasks),
            }

        self.logger.info(f"Plan review passed for {len(pending_tasks)} tasks")
        return {
            "approved": True,
            "issues": [],
            "warnings": warnings,
            "task_count": len(pending_tasks),
        }

    def call_council(
        self, project_id: str, plan_path: str = None, plan_summary: str = None
    ) -> Dict:
        """
        Call Council to review the plan.

        If council_callback is set (by orchestrator), uses real multi-model review.
        Otherwise, falls back to simplified placeholder check.

        Args:
            project_id: Project identifier
            plan_path: Path to Plan file in GitHub (e.g., docs/plans/{slug}-plan.md).
                      If not provided, will attempt to look up from vibes_ideas table.
            plan_summary: Optional summary of the plan for context

        Returns:
            {"approved": bool, "concerns": [...], "rounds": int}
        """
        pending = self.get_pending_plans()

        if not pending:
            return {
                "approved": True,
                "concerns": [],
                "message": "No pending tasks to review",
            }

        # Determine actual plan path
        actual_plan_path = plan_path
        if not actual_plan_path and project_id:
            # Try to look up plan_path from vibes_ideas table
            try:
                result = (
                    db.table("vibes_ideas")
                    .select("plan_path")
                    .eq("project_id", project_id)
                    .execute()
                )
                if result.data and result.data[0].get("plan_path"):
                    actual_plan_path = result.data[0]["plan_path"]
                    self.logger.info(
                        f"Found plan_path from vibes_ideas: {actual_plan_path}"
                    )
            except Exception as e:
                self.logger.warning(f"Could not look up plan_path: {e}")

        # Fallback to constructed path if still not found
        if not actual_plan_path:
            actual_plan_path = f"docs/plans/{project_id}-plan.md"
            self.logger.warning(f"Using constructed plan_path: {actual_plan_path}")

        if self.council_callback:
            try:
                self.logger.info(
                    f"Calling real Council review for {len(pending)} tasks using plan: {actual_plan_path}"
                )
                return self.council_callback(
                    doc_path=actual_plan_path,
                    lenses=["user_alignment", "architecture", "feasibility"],
                    context_type="plan",
                )
            except Exception as e:
                self.logger.error(f"Council callback failed: {e}, using fallback")

        concerns = []

        task_types = [t.get("type", "unknown") for t in pending]
        has_security = "security" in task_types or any(
            "auth" in (t.get("title") or "").lower() for t in pending
        )
        has_db_change = any(
            "database" in (t.get("title") or "").lower()
            or "schema" in (t.get("title") or "").lower()
            for t in pending
        )

        if has_security:
            concerns.append(
                "Plan includes security-related tasks - ensure security review"
            )

        if has_db_change:
            concerns.append("Plan includes database changes - ensure migration safety")

        high_priority = [t for t in pending if t.get("priority", 5) <= 2]
        if len(high_priority) > 5:
            concerns.append(
                f"Many high-priority tasks ({len(high_priority)}) - check resource allocation"
            )

        if len(concerns) == 0:
            self.logger.info(f"Council review passed for {len(pending)} tasks")
            return {
                "approved": True,
                "concerns": [],
                "rounds": 1,
                "task_count": len(pending),
            }

        self.logger.info(
            f"Council review raised {len(concerns)} concerns (auto-approved for now)"
        )
        return {
            "approved": True,
            "concerns": concerns,
            "rounds": 1,
            "task_count": len(pending),
        }

    def approve_plan(self, project_id: str = None) -> Dict:
        """
        Approve pending plan and transition tasks based on dependencies.

        This is called after:
        1. Supervisor reviews plan
        2. Council vets plan (if needed)

        Transitions:
        - status "pending" → "available" (no dependencies)
        - status "pending" → "locked" (has dependencies, waiting on parents)
        """
        review = self.review_plan(project_id)
        if not review["approved"]:
            return {
                "success": False,
                "error": "Plan has issues",
                "issues": review["issues"],
            }

        if review["task_count"] == 0:
            return {"success": True, "approved_count": 0, "message": "No pending tasks"}

        pending_tasks = self.get_pending_plans()
        if not pending_tasks:
            return {"success": True, "approved_count": 0, "message": "No pending tasks"}

        available_count = 0
        locked_count = 0

        for task in pending_tasks:
            deps = task.get("dependencies")
            has_deps = deps and len(deps) > 0

            if has_deps:
                new_status = "locked"
                locked_count += 1
            else:
                new_status = "available"
                available_count += 1

            update_data = {
                "status": new_status,
                "updated_at": datetime.utcnow().isoformat(),
            }

            db.table("tasks").update(update_data).eq("id", task["id"]).execute()

        self.logger.info(
            f"Approved plan: {available_count} available, {locked_count} locked (awaiting deps)"
        )

        return {
            "success": True,
            "available_count": available_count,
            "locked_count": locked_count,
            "total_count": available_count + locked_count,
        }

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

    # MAINTENANCE COMMAND METHODS (NEW - Phase B)
    # These methods insert commands to maintenance_commands table
    # Maintenance agent polls and executes them
    # =========================================================================

    def command_create_branch(
        self, task_id: str, branch_name: str, base_branch: str = "main"
    ) -> Dict:
        """
        Command Maintenance to create a new branch.

        Args:
            task_id: Task ID for tracking
            branch_name: Name of branch to create
            base_branch: Branch to create from (default: main)

        Returns:
            {"success": bool, "command_id": str, "message": str}
        """
        try:
            idempotency_key = f"create-branch-{task_id}-{int(time.time())}"

            result = (
                db_service.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "create_branch",
                        "payload": {
                            "branch_name": branch_name,
                            "base_branch": base_branch,
                        },
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "supervisor",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Commanded create branch: {branch_name}")

            return {
                "success": True,
                "command_id": command_id,
                "branch_name": branch_name,
                "message": f"Command queued: create {branch_name}",
            }

        except Exception as e:
            self.logger.error(f"Failed to queue create_branch command: {e}")
            return {"success": False, "error": str(e)}

    def command_commit_code(
        self, task_id: str, branch: str, files: List[Dict], message: str = None
    ) -> Dict:
        """
        Command Maintenance to commit code to a branch.

        Args:
            task_id: Task ID for tracking
            branch: Branch to commit to
            files: List of {"path": str, "content": str}
            message: Commit message (auto-generated if None)

        Returns:
            {"success": bool, "command_id": str, "message": str}
        """
        try:
            if not message:
                message = f"Task {task_id[:8]}: Automated commit"

            idempotency_key = f"commit-{task_id}-{int(time.time())}"

            result = (
                db_service.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "commit_code",
                        "payload": {
                            "branch": branch,
                            "files": files,
                            "message": message,
                        },
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "supervisor",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Commanded commit to {branch}: {len(files)} files")

            return {
                "success": True,
                "command_id": command_id,
                "branch": branch,
                "files_count": len(files),
                "message": f"Command queued: commit {len(files)} files to {branch}",
            }

        except Exception as e:
            self.logger.error(f"Failed to queue commit_code command: {e}")
            return {"success": False, "error": str(e)}

    def command_merge_branch(
        self,
        task_id: str,
        source: str,
        target: str,
        delete_source: bool = True,
        create_target_if_missing: bool = True,
    ) -> Dict:
        """
        Command Maintenance to merge source branch into target.

        Args:
            task_id: Task ID for tracking
            source: Source branch to merge from
            target: Target branch to merge into
            delete_source: Whether to delete source after merge
            create_target_if_missing: Create target if doesn't exist

        Returns:
            {"success": bool, "command_id": str, "message": str}
        """
        try:
            # Check if merge to main requires human approval
            if target in ["main", "master"]:
                return {
                    "success": False,
                    "error": f"Merge to {target} requires human approval",
                    "requires_human_approval": True,
                }

            idempotency_key = f"merge-{task_id}-{source}-{target}-{int(time.time())}"

            result = (
                db_service.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "merge_branch",
                        "payload": {
                            "source": source,
                            "target": target,
                            "delete_source": delete_source,
                            "create_target_if_missing": create_target_if_missing,
                        },
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "supervisor",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Commanded merge: {source} → {target}")

            return {
                "success": True,
                "command_id": command_id,
                "source": source,
                "target": target,
                "message": f"Command queued: merge {source} into {target}",
            }

        except Exception as e:
            self.logger.error(f"Failed to queue merge_branch command: {e}")
            return {"success": False, "error": str(e)}

    def command_delete_branch(self, task_id: str, branch_name: str) -> Dict:
        """
        Command Maintenance to delete a branch.

        Args:
            task_id: Task ID for tracking
            branch_name: Branch to delete

        Returns:
            {"success": bool, "command_id": str, "message": str}
        """
        try:
            # Check for protected branches
            if branch_name in ["main", "master"]:
                return {
                    "success": False,
                    "error": f"Cannot delete protected branch {branch_name}",
                }

            idempotency_key = f"delete-{task_id}-{branch_name}-{int(time.time())}"

            result = (
                db_service.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "delete_branch",
                        "payload": {"branch_name": branch_name},
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "supervisor",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Commanded delete branch: {branch_name}")

            return {
                "success": True,
                "command_id": command_id,
                "branch_name": branch_name,
                "message": f"Command queued: delete {branch_name}",
            }

        except Exception as e:
            self.logger.error(f"Failed to queue delete_branch command: {e}")
            return {"success": False, "error": str(e)}

    def command_commit_changes(
        self, task_id: str, branch: str, message: str = None
    ) -> Dict:
        """
        Command Maintenance to commit all changed files (for runners that write directly).

        Args:
            task_id: Task ID for tracking
            branch: Branch to commit to
            message: Commit message (optional)

        Returns:
            {"success": bool, "command_id": str, "message": str}
        """
        try:
            idempotency_key = f"commit-changes-{task_id}-{int(time.time())}"

            commit_message = message or f"Task {task_id[:8]}: Automated commit"

            result = (
                db_service.table("maintenance_commands")
                .insert(
                    {
                        "command_type": "commit_changes",
                        "payload": {
                            "branch": branch,
                            "message": commit_message,
                            "add_all": True,
                        },
                        "status": "pending",
                        "idempotency_key": idempotency_key,
                        "approved_by": "supervisor",
                        "created_at": datetime.utcnow().isoformat(),
                    }
                )
                .execute()
            )

            command_id = result.data[0]["id"] if result.data else None

            self.logger.info(f"Commanded commit_changes on branch: {branch}")

            return {
                "success": True,
                "command_id": command_id,
                "branch": branch,
                "message": f"Command queued: commit changes to {branch}",
            }

        except Exception as e:
            self.logger.error(f"Failed to queue commit_changes command: {e}")
            return {"success": False, "error": str(e)}

    def get_command_status(self, command_id: str) -> Optional[Dict]:
        """
        Get status of a queued command.

        Args:
            command_id: UUID of the command

        Returns:
            Command status dict or None
        """
        try:
            result = (
                db_service.table("maintenance_commands")
                .select("*")
                .eq("id", command_id)
                .execute()
            )
            return result.data[0] if result.data else None
        except Exception as e:
            self.logger.error(f"Failed to get command status: {e}")
            return None

    def wait_for_command(self, command_id: str, timeout: int = 60) -> Optional[Dict]:
        """
        Wait for a command to complete (blocking).

        Args:
            command_id: UUID of the command
            timeout: Maximum seconds to wait

        Returns:
            Final command status or None if timeout
        """
        start_time = time.time()

        while time.time() - start_time < timeout:
            status = self.get_command_status(command_id)

            if not status:
                return None

            if status.get("status") in ["completed", "failed"]:
                return status

            time.sleep(1)

        return None  # Timeout


if __name__ == "__main__":
    print("SupervisorAgent loaded. Use via import.")
