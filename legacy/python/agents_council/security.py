from ..base import Agent, AgentResult
from typing import Dict, Any, List
import re


class SecurityAgent(Agent):
    name = "Security"
    role = "Council - Ensures no secrets in code, proper security practices"
    
    SECRET_PATTERNS = [
        (r'password\s*=\s*["\'][^"\']+["\']', "Hardcoded password"),
        (r'api_key\s*=\s*["\'][^"\']+["\']', "Hardcoded API key"),
        (r'secret\s*=\s*["\'][^"\']+["\']', "Hardcoded secret"),
        (r'token\s*=\s*["\'][^"\']+["\']', "Hardcoded token"),
        (r'aws_access_key_id\s*=\s*["\'][^"\']+["\']', "Hardcoded AWS key"),
        (r'private_key\s*=\s*["\']-----BEGIN', "Hardcoded private key"),
    ]
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        self.log(f"Scanning for security issues: {filename}")
        
        issues = []
        
        for pattern, description in self.SECRET_PATTERNS:
            matches = re.findall(pattern, code, re.IGNORECASE)
            if matches:
                issues.append(f"{description} found in {filename}")
        
        if ".env" in filename and "production" not in filename:
            issues.append(".env file should not be committed (use .env.example)")
        
        if "eval(" in code and "ast.literal_eval" not in code:
            issues.append("Use of eval() is dangerous - consider ast.literal_eval()")
        
        if "pickle.loads" in code:
            issues.append("pickle.loads() can execute arbitrary code - use caution")
        
        passed = len(issues) == 0
        
        if passed:
            self.log("Security scan PASSED")
        else:
            self.log(f"Security scan FAILED: {len(issues)} issues", level="error")
        
        return AgentResult(
            success=passed,
            output={
                "issues": issues,
                "vulnerabilities_found": len(issues)
            },
            metadata={"filename": filename, "scan_type": "security"}
        )
