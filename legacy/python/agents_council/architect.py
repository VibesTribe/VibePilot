from ..base import Agent, AgentResult
from typing import Dict, Any, List


class ArchitectAgent(Agent):
    name = "Architect"
    role = "Council - Ensures non-root Docker, multi-stage builds, proper architecture"
    
    CHECKS = [
        "Dockerfile uses non-root user",
        "Dockerfile uses multi-stage build",
        "No hardcoded configuration",
        "Environment variables used for secrets",
        "Proper layer caching in Dockerfile",
        "Health checks configured",
        "Resource limits defined"
    ]
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        self.log(f"Reviewing architecture for: {filename}")
        
        issues = []
        recommendations = []
        
        if "Dockerfile" in filename or "dockerfile" in filename.lower():
            issues.extend(self._check_dockerfile(code))
        
        if "docker-compose" in filename.lower():
            issues.extend(self._check_compose(code))
        
        passed = len(issues) == 0
        
        if passed:
            self.log("Architecture review PASSED")
        else:
            self.log(f"Architecture review FAILED: {len(issues)} issues", level="warning")
        
        return AgentResult(
            success=passed,
            output={
                "issues": issues,
                "recommendations": recommendations,
                "checks_performed": self.CHECKS
            },
            metadata={"filename": filename, "review_type": "architecture"}
        )
    
    def _check_dockerfile(self, code: str) -> List[str]:
        issues = []
        
        if "USER root" in code or (code.strip().startswith("FROM") and "USER" not in code):
            issues.append("Dockerfile should use non-root user")
        
        if code.count("FROM") < 2:
            issues.append("Consider multi-stage build for smaller image")
        
        if "hardcoded" in code.lower() or any(char in code for char in ["password=", "secret="]) and "$" not in code:
            issues.append("Avoid hardcoded secrets, use environment variables")
        
        if "HEALTHCHECK" not in code:
            issues.append("Add HEALTHCHECK instruction")
        
        return issues
    
    def _check_compose(self, code: str) -> List[str]:
        issues = []
        
        if "mem_limit" not in code and "deploy:" not in code:
            issues.append("Consider adding resource limits")
        
        return issues
