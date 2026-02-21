"""
VibePilot Maintenance Agent

The ONLY agent with git write access.
Polls maintenance_commands table and executes git operations.
Also implements approved system improvements.

This agent:
- NEVER decides (only executes what Supervisor commands)
- Has vault access for git credentials
- Reports all results back to command queue
- NO self-initiated actions
"""

import os
import json
import time
import logging
import subprocess
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
logger = logging.getLogger("VibePilot.Maintenance")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")
SUPABASE_SERVICE_KEY = get_env_or_vault("SUPABASE_SERVICE_KEY")
REPO_PATH = os.getenv("VIBEPILOT_REPO_PATH", os.path.expanduser("~/vibepilot"))

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)
db_service = (
    create_client(SUPABASE_URL, SUPABASE_SERVICE_KEY) if SUPABASE_SERVICE_KEY else db
)


class MaintenanceAgent:
    """
    Maintenance Agent - Git Operator and System Implementer.

    This is the ONLY agent with:
    - Git write access (create, commit, merge, delete, tag)
    - File system write access
    - Vault access (for credentials)

    Responsibilities:
    1. Poll maintenance_commands table for pending commands
    2. Execute git operations per Supervisor commands
    3. Implement approved system improvements
    4. Report all results back to command queue

    NEVER acts without approval:
    - Task git operations: Supervisor approval via command queue
    - System improvements: Council + Supervisor + Human approval
    """

    def __init__(self, agent_id: str = "maintenance"):
        self.agent_id = agent_id
        self.repo_path = REPO_PATH
        self.running = False
        self.logger = logger

    def start(self, poll_interval: int = 5):
        """
        Start polling for commands.

        Args:
            poll_interval: Seconds between polls (default 5)
        """
        self.running = True
        self.logger.info(f"Maintenance Agent {self.agent_id} started")
        self.logger.info(f"Polling interval: {poll_interval}s")
        self.logger.info(f"Repo path: {self.repo_path}")

        while self.running:
            try:
                self._poll_and_execute()
            except Exception as e:
                self.logger.error(f"Error in poll cycle: {e}")

            time.sleep(poll_interval)

    def stop(self):
        """Stop polling."""
        self.running = False
        self.logger.info("Maintenance Agent stopped")

    def execute(self, command: Dict[str, Any]):
        """
        Execute a single maintenance command.

        Used by orchestrator to process commands inline instead of polling.

        Args:
            command: Dict with 'action' key and command-specific params

        Returns:
            AgentResult with success/failure
        """
        from agents.base import AgentResult

        command_type = command.get("action")
        if not command_type:
            return AgentResult(success=False, output=None, error="Missing action")

        payload = {k: v for k, v in command.items() if k != "action"}

        if command_type == "create_branch":
            result = self._execute_create_branch(payload)
        elif command_type == "commit_code":
            result = self._execute_commit_code(payload)
        elif command_type == "commit_changes":
            result = self._execute_commit_changes(payload)
        elif command_type == "merge_branch":
            result = self._execute_merge_branch(payload)
        elif command_type == "delete_branch":
            result = self._execute_delete_branch(payload)
        elif command_type == "tag_release":
            result = self._execute_tag_release(payload)
        else:
            result = {
                "success": False,
                "error": f"Unknown command type: {command_type}",
            }

        return AgentResult(
            success=result.get("success", False),
            output=result if result.get("success") else None,
            error=result.get("error") if not result.get("success") else None,
        )

    def _poll_and_execute(self):
        """Poll for next command and execute it."""
        try:
            result = db_service.rpc(
                "claim_next_command", {"p_agent_id": self.agent_id}
            ).execute()

            if not result.data:
                return  # No pending commands

            command = result.data[0]
            command_id = command["command_id"]
            command_type = command["command_type"]
            payload = command["payload"]

            self.logger.info(f"Executing {command_type} (id: {command_id[:8]})")

            # Execute based on command type
            if command_type == "create_branch":
                result = self._execute_create_branch(payload)
            elif command_type == "commit_code":
                result = self._execute_commit_code(payload)
            elif command_type == "commit_changes":
                result = self._execute_commit_changes(payload)
            elif command_type == "merge_branch":
                result = self._execute_merge_branch(payload)
            elif command_type == "delete_branch":
                result = self._execute_delete_branch(payload)
            elif command_type == "tag_release":
                result = self._execute_tag_release(payload)
            else:
                result = {
                    "success": False,
                    "error": f"Unknown command type: {command_type}",
                }

            # Report result
            self._report_result(command_id, result)

            if result["success"]:
                self.logger.info(f"✅ {command_type} succeeded")
            else:
                self.logger.warning(f"❌ {command_type} failed: {result.get('error')}")

        except Exception as e:
            self.logger.error(f"Error executing command: {e}")

    def _execute_create_branch(self, payload: Dict) -> Dict:
        """
        Create a new git branch.

        Payload:
            branch_name: str - Name of branch to create
            base_branch: str - Branch to create from (default: main)
        """
        branch_name = payload.get("branch_name")
        base_branch = payload.get("base_branch", "main")

        if not branch_name:
            return {"success": False, "error": "Missing branch_name"}

        try:
            # Validate branch name
            if not self._is_valid_branch_name(branch_name):
                return {
                    "success": False,
                    "error": f"Invalid branch name: {branch_name}",
                }

            # Checkout base branch
            result = self._git_command(["checkout", base_branch])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to checkout {base_branch}: {result.stderr}",
                }

            # Pull latest
            result = self._git_command(["pull", "origin", base_branch])
            if result.returncode != 0:
                self.logger.warning(f"Pull failed (may be up to date): {result.stderr}")

            # Create branch
            result = self._git_command(["checkout", "-b", branch_name])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to create branch: {result.stderr}",
                }

            # Push to origin
            result = self._git_command(["push", "-u", "origin", branch_name])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to push branch: {result.stderr}",
                }

            return {
                "success": True,
                "branch_name": branch_name,
                "base_branch": base_branch,
                "message": f"Created branch {branch_name} from {base_branch}",
            }

        except Exception as e:
            return {"success": False, "error": f"Exception: {str(e)}"}

    def _execute_commit_code(self, payload: Dict) -> Dict:
        """
        Commit code to a branch.

        Payload:
            branch: str - Branch to commit to
            files: List[{"path": str, "content": str}] - Files to create/modify
            message: str - Commit message
            author: Optional[str] - Author name/email
        """
        branch = payload.get("branch")
        files = payload.get("files", [])
        message = payload.get("message", "Auto-commit by VibePilot")

        if not branch or not files:
            return {"success": False, "error": "Missing branch or files"}

        try:
            # Checkout branch
            result = self._git_command(["checkout", branch])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to checkout {branch}: {result.stderr}",
                }

            # Write files
            files_written = []
            for file_info in files:
                path = file_info.get("path")
                content = file_info.get("content")

                if not path:
                    continue

                # Ensure directory exists
                full_path = os.path.join(self.repo_path, path)
                os.makedirs(os.path.dirname(full_path), exist_ok=True)

                # Write file
                with open(full_path, "w") as f:
                    f.write(content)

                files_written.append(path)

            # Add files
            for path in files_written:
                result = self._git_command(["add", path])
                if result.returncode != 0:
                    self.logger.warning(f"Failed to add {path}: {result.stderr}")

            # Commit
            result = self._git_command(["commit", "-m", message])
            if result.returncode != 0:
                return {"success": False, "error": f"Failed to commit: {result.stderr}"}

            # Push
            result = self._git_command(["push", "origin", branch])
            if result.returncode != 0:
                return {"success": False, "error": f"Failed to push: {result.stderr}"}

            # Get commit hash
            result = self._git_command(["rev-parse", "HEAD"])
            commit_hash = result.stdout.strip() if result.returncode == 0 else None

            return {
                "success": True,
                "branch": branch,
                "files_committed": files_written,
                "commit_hash": commit_hash,
                "message": message,
            }

        except Exception as e:
            return {"success": False, "error": f"Exception: {str(e)}"}

    def _execute_commit_changes(self, payload: Dict) -> Dict:
        """
        Commit all changed files in working directory (for runners that write directly).

        Payload:
            branch: str - Branch to commit to
            message: str - Commit message
            add_all: bool - Add all changed files (default True)
        """
        branch = payload.get("branch")
        message = payload.get("message", "Auto-commit by VibePilot")
        add_all = payload.get("add_all", True)

        if not branch:
            return {"success": False, "error": "Missing branch"}

        try:
            result = self._git_command(["checkout", branch])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to checkout {branch}: {result.stderr}",
                }

            if add_all:
                result = self._git_command(["add", "-A"])
                if result.returncode != 0:
                    self.logger.warning(f"git add -A warning: {result.stderr}")

            result = self._git_command(["status", "--porcelain"])
            if result.returncode == 0 and not result.stdout.strip():
                return {
                    "success": True,
                    "message": "No changes to commit",
                    "files_committed": [],
                }

            result = self._git_command(["commit", "-m", message])
            if result.returncode != 0:
                if (
                    "nothing to commit" in result.stderr
                    or "nothing to commit" in result.stdout
                ):
                    return {
                        "success": True,
                        "message": "No changes to commit",
                        "files_committed": [],
                    }
                return {"success": False, "error": f"Failed to commit: {result.stderr}"}

            result = self._git_command(["push", "origin", branch])
            if result.returncode != 0:
                return {"success": False, "error": f"Failed to push: {result.stderr}"}

            result = self._git_command(["rev-parse", "HEAD"])
            commit_hash = result.stdout.strip() if result.returncode == 0 else None

            result = self._git_command(["diff", "--name-only", "HEAD~1"])
            files_committed = (
                result.stdout.strip().split("\n") if result.returncode == 0 else []
            )

            return {
                "success": True,
                "branch": branch,
                "files_committed": [f for f in files_committed if f],
                "commit_hash": commit_hash,
                "message": message,
            }

        except Exception as e:
            return {"success": False, "error": f"Exception: {str(e)}"}

    def _execute_merge_branch(self, payload: Dict) -> Dict:
        """
        Merge source branch into target branch.

        Payload:
            source: str - Branch to merge from
            target: str - Branch to merge into
            delete_source: bool - Whether to delete source after merge
            create_target_if_missing: bool - Create target if doesn't exist
        """
        source = payload.get("source")
        target = payload.get("target")
        delete_source = payload.get("delete_source", False)
        create_target = payload.get("create_target_if_missing", False)

        if not source or not target:
            return {"success": False, "error": "Missing source or target"}

        # Check for protected branches
        if target in ["main", "master"]:
            return {
                "success": False,
                "error": f"Merge to {target} requires human approval",
                "requires_human_approval": True,
            }

        try:
            # Ensure target branch exists
            result = self._git_command(["branch", "--list", target])
            if target not in result.stdout:
                if create_target:
                    # Create target from main
                    result = self._git_command(["checkout", "main"])
                    if result.returncode != 0:
                        return {"success": False, "error": "Failed to checkout main"}

                    result = self._git_command(["checkout", "-b", target])
                    if result.returncode != 0:
                        return {"success": False, "error": f"Failed to create {target}"}

                    result = self._git_command(["push", "-u", "origin", target])
                    if result.returncode != 0:
                        return {"success": False, "error": f"Failed to push {target}"}
                else:
                    return {
                        "success": False,
                        "error": f"Target branch {target} does not exist",
                    }

            # Checkout target
            result = self._git_command(["checkout", target])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to checkout {target}: {result.stderr}",
                }

            # Pull latest
            result = self._git_command(["pull", "origin", target])
            if result.returncode != 0:
                self.logger.warning(f"Pull failed: {result.stderr}")

            # Merge source
            result = self._git_command(
                ["merge", "--no-ff", source, "-m", f"Merge {source} into {target}"]
            )
            if result.returncode != 0:
                return {"success": False, "error": f"Merge failed: {result.stderr}"}

            # Push
            result = self._git_command(["push", "origin", target])
            if result.returncode != 0:
                return {"success": False, "error": f"Push failed: {result.stderr}"}

            # Get commit hash
            result = self._git_command(["rev-parse", "HEAD"])
            commit_hash = result.stdout.strip() if result.returncode == 0 else None

            # Delete source if requested
            source_deleted = False
            if delete_source:
                result = self._git_command(["push", "origin", "--delete", source])
                if result.returncode == 0:
                    source_deleted = True
                else:
                    self.logger.warning(f"Failed to delete {source}: {result.stderr}")

            return {
                "success": True,
                "source": source,
                "target": target,
                "commit_hash": commit_hash,
                "source_deleted": source_deleted,
            }

        except Exception as e:
            return {"success": False, "error": f"Exception: {str(e)}"}

    def _execute_delete_branch(self, payload: Dict) -> Dict:
        """
        Delete a git branch.

        Payload:
            branch_name: str - Branch to delete
            force: bool - Force delete if not merged
        """
        branch_name = payload.get("branch_name")
        force = payload.get("force", False)

        if not branch_name:
            return {"success": False, "error": "Missing branch_name"}

        # Check for protected branches
        if branch_name in ["main", "master"]:
            return {
                "success": False,
                "error": f"Cannot delete protected branch {branch_name}",
            }

        try:
            # Delete remote branch
            result = self._git_command(["push", "origin", "--delete", branch_name])
            if result.returncode != 0:
                return {
                    "success": False,
                    "error": f"Failed to delete remote branch: {result.stderr}",
                }

            # Delete local branch
            flag = "-D" if force else "-d"
            result = self._git_command(["branch", flag, branch_name])
            if result.returncode != 0:
                self.logger.warning(f"Failed to delete local branch: {result.stderr}")

            return {
                "success": True,
                "branch_name": branch_name,
                "message": f"Deleted branch {branch_name}",
            }

        except Exception as e:
            return {"success": False, "error": f"Exception: {str(e)}"}

    def _execute_tag_release(self, payload: Dict) -> Dict:
        """
        Create a git tag.

        Payload:
            tag: str - Tag name
            target: str - Branch/commit to tag
            message: str - Tag message
            annotated: bool - Create annotated tag
        """
        tag = payload.get("tag")
        target = payload.get("target", "main")
        message = payload.get("message", f"Release {tag}")
        annotated = payload.get("annotated", True)

        if not tag:
            return {"success": False, "error": "Missing tag"}

        # Human approval required for tags
        return {
            "success": False,
            "error": "Tag creation requires human approval",
            "requires_human_approval": True,
        }

    def _git_command(self, args: List[str]) -> subprocess.CompletedProcess:
        """Execute a git command in the repo directory."""
        cmd = ["git"] + args
        return subprocess.run(cmd, cwd=self.repo_path, capture_output=True, text=True)

    def _is_valid_branch_name(self, name: str) -> bool:
        """Validate branch name format."""
        import re

        # Allow: task/T001-desc, module/user-auth, feature/name
        pattern = r"^[a-z0-9]+/[a-z0-9-]+$"
        return bool(re.match(pattern, name))

    def _report_result(self, command_id: str, result: Dict):
        """Report command execution result to database."""
        try:
            success = result.get("success", False)

            db_service.rpc(
                "complete_command",
                {
                    "p_command_id": command_id,
                    "p_success": success,
                    "p_result": result,
                    "p_error_message": result.get("error") if not success else None,
                },
            ).execute()

        except Exception as e:
            self.logger.error(f"Failed to report result: {e}")


if __name__ == "__main__":
    print("=== VibePilot Maintenance Agent ===\n")

    agent = MaintenanceAgent()

    # Run one poll cycle for testing
    print("Running single poll cycle...")
    agent._poll_and_execute()

    print("\n✅ Maintenance Agent ready")
    print("Run .start() to begin continuous polling")
