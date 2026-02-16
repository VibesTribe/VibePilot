"""
Base Runner Interface

Every runner MUST inherit from BaseRunner and implement:
- execute(task_packet) -> result dict
- probe() -> bool (health check)

The contract is enforced via stdin/stdout:
- Input: JSON via stdin or --task flag
- Output: JSON via stdout
- Exit: 0 = success, 1 = failure
"""

import sys
import json
import time
import argparse
import logging
from abc import ABC, abstractmethod
from typing import Dict, Any, Optional
from pathlib import Path

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s | %(levelname)s | %(name)s | %(message)s"
)
logger = logging.getLogger("VibePilot.BaseRunner")


class BaseRunner(ABC):
    """
    Abstract base class for all VibePilot runners.

    Enforces the RUNNER_INTERFACE contract:
    - Input: JSON task packet via stdin or --task flag
    - Output: JSON result via stdout
    - Exit codes: 0=success, 1=failure
    - --probe: health check, prints "OK" if healthy
    """

    VERSION = "1.0.0"
    RUNNER_TYPE = "base"

    def __init__(self, runner_id: str = None):
        self.runner_id = runner_id or self.RUNNER_TYPE
        self.logger = logging.getLogger(f"VibePilot.Runner.{self.runner_id}")

    @abstractmethod
    def execute(self, task_packet: Dict[str, Any]) -> Dict[str, Any]:
        """
        Execute a task packet.

        Args:
            task_packet: Dict with keys:
                - task_id: str
                - title: str
                - objectives: list
                - deliverables: list
                - prompt: str
                - context: str
                - output_format: dict
                - constraints: dict
                - runner_context: dict (platform, model, attempt_number, previous_failures)
                - codebase_files: dict (internal runners only)

        Returns:
            Dict with keys:
                - task_id: str
                - status: "success"|"failed"|"partial"|"timeout"
                - output: str or None
                - artifacts: list of file paths
                - errors: list of {code, message}
                - metadata: dict with model, platform, tokens_in, tokens_out, duration_seconds, etc.
                - feedback: dict with output_matched_prompt, suggested_next_step, failure_reason, virtual_cost
        """
        pass

    @abstractmethod
    def probe(self) -> tuple[bool, str]:
        """
        Health check for this runner.

        Returns:
            (success: bool, message: str)
            - (True, "OK") if healthy
            - (False, "PROBE_FAILED: reason") if unhealthy
        """
        pass

    def validate_input(self, task_packet: Dict[str, Any]) -> tuple[bool, Optional[str]]:
        """
        Validate task packet has required fields.

        Returns:
            (is_valid, error_message)
        """
        required = ["task_id", "prompt"]
        for field in required:
            if field not in task_packet:
                return False, f"Missing required field: {field}"
        return True, None

    def build_success_result(
        self,
        task_id: str,
        output: str,
        artifacts: list = None,
        tokens_in: int = 0,
        tokens_out: int = 0,
        duration_seconds: float = 0,
        chat_url: str = None,
        **extra_metadata,
    ) -> Dict[str, Any]:
        """Build a success result dict."""
        return {
            "task_id": task_id,
            "status": "success",
            "output": output,
            "artifacts": artifacts or [],
            "errors": [],
            "metadata": {
                "model": self.runner_id,
                "platform": self.RUNNER_TYPE,
                "tokens_in": tokens_in,
                "tokens_out": tokens_out,
                "duration_seconds": round(duration_seconds, 2),
                "runner_version": self.VERSION,
                **({"chat_url": chat_url} if chat_url else {}),
                **extra_metadata,
            },
            "feedback": {
                "output_matched_prompt": True,
                "suggested_next_step": "complete",
                "failure_reason": None,
                "virtual_cost": self._calculate_virtual_cost(tokens_in, tokens_out),
            },
        }

    def build_failure_result(
        self,
        task_id: str,
        error_code: str,
        error_message: str,
        suggested_next_step: str = "retry",
        tokens_in: int = 0,
        tokens_out: int = 0,
        **extra,
    ) -> Dict[str, Any]:
        """Build a failure result dict."""
        return {
            "task_id": task_id,
            "status": "failed",
            "output": None,
            "artifacts": [],
            "errors": [{"code": error_code, "message": error_message}],
            "metadata": {
                "model": self.runner_id,
                "platform": self.RUNNER_TYPE,
                "tokens_in": tokens_in,
                "tokens_out": tokens_out,
                "duration_seconds": 0,
                "runner_version": self.VERSION,
                **extra,
            },
            "feedback": {
                "output_matched_prompt": False,
                "suggested_next_step": suggested_next_step,
                "failure_reason": error_code.lower(),
                "virtual_cost": self._calculate_virtual_cost(tokens_in, tokens_out),
            },
        }

    def _calculate_virtual_cost(self, tokens_in: int, tokens_out: int) -> float:
        """
        Calculate virtual cost (what this would cost via API).
        Override in subclasses with actual rates.
        """
        return 0.0

    def run_from_stdin(self) -> int:
        """
        Read task packet from stdin, execute, print result to stdout.
        Returns exit code (0=success, 1=failure).
        """
        try:
            input_data = sys.stdin.read()
            task_packet = json.loads(input_data)
        except json.JSONDecodeError as e:
            result = self.build_failure_result(
                task_id=None,
                error_code="INVALID_INPUT",
                error_message=f"Invalid JSON: {e}",
            )
            print(json.dumps(result))
            return 1

        return self.run_with_packet(task_packet)

    def run_with_packet(self, task_packet: Dict[str, Any]) -> int:
        """
        Execute with a task packet, print result to stdout.
        Returns exit code (0=success, 1=failure).
        """
        is_valid, error = self.validate_input(task_packet)
        if not is_valid:
            result = self.build_failure_result(
                task_id=task_packet.get("task_id"),
                error_code="INVALID_INPUT",
                error_message=error,
            )
            print(json.dumps(result))
            return 1

        task_id = task_packet.get("task_id")
        self.logger.info(f"Executing task: {task_id}")

        start_time = time.time()
        try:
            result = self.execute(task_packet)
            duration = time.time() - start_time

            if "metadata" in result:
                result["metadata"]["duration_seconds"] = round(duration, 2)

            print(json.dumps(result))

            status = result.get("status", "failed")
            self.logger.info(f"Task {task_id}: {status} in {duration:.2f}s")

            return 0 if status == "success" else 1

        except Exception as e:
            self.logger.error(f"Task {task_id} crashed: {e}")
            result = self.build_failure_result(
                task_id=task_id,
                error_code="RUNNER_ERROR",
                error_message=str(e),
                suggested_next_step="retry",
            )
            print(json.dumps(result))
            return 1

    def run_probe(self) -> int:
        """
        Run health check, print result to stdout.
        Returns exit code (0=healthy, 1=unhealthy).
        """
        success, message = self.probe()
        print(message)
        return 0 if success else 1

    def main(self):
        """
        Main entry point for command-line usage.
        Parses args and routes to appropriate method.
        """
        parser = argparse.ArgumentParser(
            description=f"VibePilot {self.runner_id} Runner"
        )
        parser.add_argument("--probe", action="store_true", help="Run health check")
        parser.add_argument("--task", type=str, help="Path to task packet JSON file")
        parser.add_argument("--output", type=str, help="Path to write result JSON file")

        args = parser.parse_args()

        if args.probe:
            exit_code = self.run_probe()
            sys.exit(exit_code)

        if args.task:
            task_path = Path(args.task)
            if not task_path.exists():
                print(
                    json.dumps(
                        self.build_failure_result(
                            task_id=None,
                            error_code="FILE_NOT_FOUND",
                            error_message=f"Task file not found: {args.task}",
                        )
                    )
                )
                sys.exit(1)

            with open(task_path) as f:
                task_packet = json.load(f)

            exit_code = self.run_with_packet(task_packet)

            if args.output:
                result = (
                    json.loads(sys.stdout.getvalue())
                    if hasattr(sys.stdout, "getvalue")
                    else {}
                )
                with open(args.output, "w") as f:
                    json.dump(result, f, indent=2)

            sys.exit(exit_code)

        exit_code = self.run_from_stdin()
        sys.exit(exit_code)


def run_runner(runner_class):
    """
    Convenience function to run a runner from command line.

    Usage in runner file:
        if __name__ == "__main__":
            run_runner(MyRunner)
    """
    runner = runner_class()
    runner.main()
