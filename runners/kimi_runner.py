import subprocess
import json
import os
import logging
import threading
from typing import Optional, Dict, Any, List
from concurrent.futures import ThreadPoolExecutor, as_completed

logger = logging.getLogger("VibePilot.KimiRunner")


class KimiRunner:
    def __init__(self):
        self.kimi_path = self._find_kimi()
        self.logger = logger

    def _find_kimi(self) -> str:
        result = subprocess.run(["which", "kimi"], capture_output=True, text=True)
        if result.returncode == 0:
            return result.stdout.strip()
        return os.path.expanduser("~/.local/bin/kimi")

    def is_available(self) -> bool:
        try:
            result = subprocess.run(
                [self.kimi_path, "--version"], capture_output=True, text=True, timeout=5
            )
            return result.returncode == 0
        except Exception as e:
            self.logger.error(f"Kimi not available: {e}")
            return False

    def execute_task(
        self,
        prompt: str,
        work_dir: str = None,
        timeout: int = 300,
        auto_approve: bool = True,
    ) -> Dict[str, Any]:
        cmd = [self.kimi_path]

        if auto_approve:
            cmd.append("--yolo")

        cmd.extend(["--print", "--output-format", "text", "--final-message-only"])

        if work_dir:
            cmd.extend(["--work-dir", work_dir])

        cmd.extend(["--prompt", prompt])

        self.logger.info(f"Dispatching task to Kimi: {prompt[:50]}...")

        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=timeout,
                cwd=work_dir or os.getcwd(),
            )

            if result.returncode == 0:
                output = result.stdout.strip()
                self.logger.info("Kimi task completed successfully")
                return {
                    "success": True,
                    "output": output,
                    "model": "kimi-k2.5",
                    "error": None,
                }
            else:
                error = result.stderr.strip() or "Unknown error"
                self.logger.error(f"Kimi task failed: {error}")
                return {
                    "success": False,
                    "output": None,
                    "model": "kimi-k2.5",
                    "error": error,
                }

        except subprocess.TimeoutExpired:
            self.logger.error("Kimi task timed out")
            return {
                "success": False,
                "output": None,
                "model": "kimi-k2.5",
                "error": f"Timeout after {timeout}s",
            }
        except Exception as e:
            self.logger.error(f"Kimi execution error: {e}")
            return {
                "success": False,
                "output": None,
                "model": "kimi-k2.5",
                "error": str(e),
            }

    def execute_code_task(
        self, description: str, language: str = "python", work_dir: str = None
    ) -> Dict[str, Any]:
        prompt = f"""You are a code generation agent. Generate clean, production-ready code.

Task: {description}
Language: {language}

Requirements:
1. Write clean, readable code
2. Include error handling
3. Follow best practices for {language}
4. No comments unless complex logic
5. Return ONLY the code, no explanations

Generate the code now:"""

        return self.execute_task(prompt, work_dir=work_dir)

    def execute_review_task(self, code: str, filename: str = "code") -> Dict[str, Any]:
        prompt = f"""You are a code reviewer. Review this code for issues.

File: {filename}

Code:
```
{code}
```

Check for:
1. Security vulnerabilities
2. Logic errors
3. Performance issues
4. Best practice violations

Respond with JSON:
{{"passed": true/false, "issues": ["issue1", ...], "suggestions": ["suggestion1", ...]}}"""

        result = self.execute_task(prompt)

        if result["success"] and result["output"]:
            try:
                json_start = result["output"].find("{")
                json_end = result["output"].rfind("}") + 1
                if json_start >= 0 and json_end > json_start:
                    review = json.loads(result["output"][json_start:json_end])
                    result["review"] = review
            except json.JSONDecodeError:
                pass

        return result

    def execute_swarm(
        self, tasks: List[Dict[str, Any]], max_workers: int = 10
    ) -> Dict[str, Any]:
        """
        Execute multiple tasks in parallel using Kimi swarm.

        Kimi can spawn up to 100 parallel agents. Use for "wide" tasks:
        - Repo-wide audits
        - Parallel refactoring across files
        - Bulk code generation

        Args:
            tasks: List of task dicts with 'prompt' and optional 'work_dir'
            max_workers: Max concurrent tasks (default 10, max 100)

        Returns:
            {
                "success": bool,
                "total": int,
                "completed": int,
                "failed": int,
                "results": [...]
            }
        """
        if not tasks:
            return {
                "success": True,
                "total": 0,
                "completed": 0,
                "failed": 0,
                "results": [],
            }

        max_workers = min(max_workers, 100)  # Kimi limit
        results = []
        completed = 0
        failed = 0

        self.logger.info(
            f"Starting Kimi swarm: {len(tasks)} tasks, {max_workers} workers"
        )

        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = {}

            for i, task in enumerate(tasks):
                prompt = task.get("prompt", "")
                work_dir = task.get("work_dir")
                task_id = task.get("id", f"swarm-{i}")

                future = executor.submit(
                    self.execute_task,
                    prompt,
                    work_dir,
                    task.get("timeout", 300),
                    task.get("auto_approve", True),
                )
                futures[future] = task_id

            for future in as_completed(futures):
                task_id = futures[future]
                try:
                    result = future.result()
                    result["task_id"] = task_id
                    results.append(result)

                    if result.get("success"):
                        completed += 1
                    else:
                        failed += 1

                except Exception as e:
                    self.logger.error(f"Swarm task {task_id} crashed: {e}")
                    results.append(
                        {"task_id": task_id, "success": False, "error": str(e)}
                    )
                    failed += 1

        success = failed == 0

        self.logger.info(f"Swarm complete: {completed}/{len(tasks)} succeeded")

        return {
            "success": success,
            "total": len(tasks),
            "completed": completed,
            "failed": failed,
            "results": results,
        }

    def execute_repo_audit(
        self, repo_path: str, checks: List[str] = None
    ) -> Dict[str, Any]:
        """
        Run parallel audits across a repository.

        Args:
            repo_path: Path to repository
            checks: List of check types. Default: security, performance, best-practices

        Returns:
            Aggregated audit results
        """
        checks = checks or [
            "security",
            "performance",
            "best-practices",
            "documentation",
        ]

        tasks = []
        for check in checks:
            tasks.append(
                {
                    "id": f"audit-{check}",
                    "prompt": f"""Audit this repository for {check}.

Repository path: {repo_path}

Check type: {check}

Instructions:
1. Explore the repository structure
2. Identify files relevant to {check}
3. Analyze for issues
4. Return JSON: {{"issues": [{{"file": "...", "line": N, "issue": "...", "severity": "high|medium|low"}}], "summary": "..."}}""",
                    "work_dir": repo_path,
                }
            )

        return self.execute_swarm(tasks)

    def execute_parallel_refactor(
        self, files: List[str], refactoring: str, work_dir: str = None
    ) -> Dict[str, Any]:
        """
        Apply same refactoring across multiple files in parallel.

        Args:
            files: List of file paths to refactor
            refactoring: Description of refactoring to apply
            work_dir: Working directory

        Returns:
            Results from all files
        """
        tasks = []
        for filepath in files:
            tasks.append(
                {
                    "id": f"refactor-{os.path.basename(filepath)}",
                    "prompt": f"""Apply this refactoring to the file.

File: {filepath}
Refactoring: {refactoring}

Instructions:
1. Read the current file
2. Apply the refactoring
3. Ensure code still works
4. Return the complete refactored file""",
                    "work_dir": work_dir or os.path.dirname(filepath),
                }
            )

        return self.execute_swarm(tasks)


if __name__ == "__main__":
    runner = KimiRunner()

    print("=== Kimi Runner Test ===\n")

    print("1. Checking availability...")
    if runner.is_available():
        print("   Kimi is available\n")
    else:
        print("   Kimi is NOT available\n")
        exit(1)

    print("2. Testing simple task...")
    result = runner.execute_task("Say 'Hello from Kimi!' and nothing else.")
    print(f"   Success: {result['success']}")
    print(f"   Output: {result.get('output', 'N/A')[:100]}\n")

    print("3. Testing code task...")
    result = runner.execute_code_task("Write a function that adds two numbers")
    print(f"   Success: {result['success']}")
    if result["success"]:
        print(f"   Code:\n{result['output'][:200]}...\n")

    print("=== Test Complete ===")
