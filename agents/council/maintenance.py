from ..base import Agent, AgentResult
from typing import Dict, Any, List
import re


class MaintenanceAgent(Agent):
    name = "Maintenance"
    role = "Council - Checks dependencies, code quality, and maintainability"
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        self.log(f"Checking maintainability: {filename}")
        
        issues = []
        warnings = []
        
        lines = code.split("\n")
        
        for i, line in enumerate(lines, 1):
            if len(line) > 120:
                warnings.append(f"Line {i} exceeds 120 characters ({len(line)} chars)")
        
        todos = len(re.findall(r'# TODO|TODO:|FIXME', code, re.IGNORECASE))
        if todos > 5:
            warnings.append(f"High TODO/FIXME count: {todos}")
        
        if filename.endswith(".py"):
            issues.extend(self._check_python(code, filename))
        
        if "requirements.txt" in filename:
            issues.extend(self._check_requirements(code))
        
        score = self._calculate_score(len(issues), len(warnings), len(lines))
        
        passed = len(issues) == 0 and score >= 70
        
        self.log(f"Maintenance score: {score}/100")
        
        return AgentResult(
            success=passed,
            output={
                "score": score,
                "issues": issues,
                "warnings": warnings,
                "line_count": len(lines),
                "todo_count": todos
            },
            metadata={"filename": filename}
        )
    
    def _check_python(self, code: str, filename: str) -> List[str]:
        issues = []
        
        if "import *" in code:
            issues.append("Avoid 'import *' - use explicit imports")
        
        bare_except = len(re.findall(r'except\s*:', code))
        if bare_except > 0:
            issues.append(f"Found {bare_except} bare except clauses - be specific")
        
        if "print(" in code and "test" not in filename.lower():
            issues.append("Use logging instead of print() in production code")
        
        return issues
    
    def _check_requirements(self, code: str) -> List[str]:
        issues = []
        
        lines = [l.strip() for l in code.split("\n") if l.strip() and not l.startswith("#")]
        
        unpinned = [l for l in lines if "==" not in l and not l.startswith("-")]
        if unpinned:
            issues.append(f"Unpinned dependencies: {', '.join(unpinned[:3])}")
        
        return issues
    
    def _calculate_score(self, issues: int, warnings: int, lines: int) -> int:
        base = 100
        base -= issues * 15
        base -= warnings * 3
        if lines > 500:
            base -= 5
        return max(0, min(100, base))
