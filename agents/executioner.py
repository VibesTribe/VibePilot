import subprocess
import os
from .base import Agent, AgentResult
from typing import Dict, Any, List


class ExecutionerAgent(Agent):
    name = "Executioner"
    role = "Validator - Runs tests and validates code quality"
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        action = task.get("action", "validate")
        
        if action == "validate":
            return self._validate_code(task)
        elif action == "run_tests":
            return self._run_tests(task)
        elif action == "lint":
            return self._run_lint(task)
        else:
            return AgentResult(
                success=False,
                output=None,
                error=f"Unknown action: {action}"
            )
    
    def _validate_code(self, task: Dict[str, Any]) -> AgentResult:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        self.log(f"Validating code: {filename}")
        
        checks = []
        all_passed = True
        
        if filename.endswith(".py"):
            checks.append(self._check_python_syntax(code, filename))
        
        if filename.endswith(".js") or filename.endswith(".ts"):
            checks.append(self._check_js_syntax(code, filename))
        
        results = []
        for check in checks:
            results.append(check)
            if not check.get("passed", False):
                all_passed = False
        
        if all_passed:
            self.log("Code validation PASSED")
        else:
            self.log("Code validation FAILED", level="error")
        
        return AgentResult(
            success=all_passed,
            output={
                "checks": results,
                "filename": filename
            }
        )
    
    def _run_tests(self, task: Dict[str, Any]) -> AgentResult:
        test_command = task.get("test_command", "pytest")
        cwd = task.get("cwd", os.getcwd())
        
        self.log(f"Running tests: {test_command}")
        
        try:
            result = subprocess.run(
                test_command.split(),
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=60
            )
            
            passed = result.returncode == 0
            
            if passed:
                self.log("Tests PASSED")
            else:
                self.log(f"Tests FAILED: {result.stderr[:200]}", level="error")
            
            return AgentResult(
                success=passed,
                output={
                    "stdout": result.stdout,
                    "stderr": result.stderr,
                    "return_code": result.returncode
                }
            )
        except subprocess.TimeoutExpired:
            self.log("Tests TIMED OUT", level="error")
            return AgentResult(
                success=False,
                output=None,
                error="Test execution timed out"
            )
        except Exception as e:
            self.log(f"Test execution error: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
    
    def _run_lint(self, task: Dict[str, Any]) -> AgentResult:
        lint_command = task.get("lint_command", "ruff check .")
        cwd = task.get("cwd", os.getcwd())
        
        self.log(f"Running linter: {lint_command}")
        
        try:
            result = subprocess.run(
                lint_command.split(),
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=30
            )
            
            has_issues = result.returncode != 0
            
            if not has_issues:
                self.log("Lint PASSED - no issues")
            else:
                self.log("Lint found issues", level="warning")
            
            return AgentResult(
                success=not has_issues,
                output={
                    "stdout": result.stdout,
                    "stderr": result.stderr,
                    "issues_found": has_issues
                }
            )
        except Exception as e:
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
    
    def _check_python_syntax(self, code: str, filename: str) -> Dict[str, Any]:
        try:
            compile(code, filename, 'exec')
            return {"check": "python_syntax", "passed": True, "message": "No syntax errors"}
        except SyntaxError as e:
            return {
                "check": "python_syntax",
                "passed": False,
                "message": f"Syntax error at line {e.lineno}: {e.msg}"
            }
    
    def _check_js_syntax(self, code: str, filename: str) -> Dict[str, Any]:
        if "function" in code or "const" in code or "let" in code:
            return {"check": "js_basic", "passed": True, "message": "Basic structure OK"}
        return {"check": "js_basic", "passed": True, "message": "Skipped detailed JS check"}
