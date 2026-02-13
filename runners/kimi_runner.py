import subprocess
import json
import os
import logging
from typing import Optional, Dict, Any

logger = logging.getLogger("VibePilot.KimiRunner")

class KimiRunner:
    def __init__(self):
        self.kimi_path = self._find_kimi()
        self.logger = logger
        
    def _find_kimi(self) -> str:
        result = subprocess.run(
            ["which", "kimi"],
            capture_output=True,
            text=True
        )
        if result.returncode == 0:
            return result.stdout.strip()
        return os.path.expanduser("~/.local/bin/kimi")
    
    def is_available(self) -> bool:
        try:
            result = subprocess.run(
                [self.kimi_path, "--version"],
                capture_output=True,
                text=True,
                timeout=5
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
        auto_approve: bool = True
    ) -> Dict[str, Any]:
        cmd = [self.kimi_path]
        
        if auto_approve:
            cmd.append("--yolo")
        
        cmd.extend([
            "--print",
            "--output-format", "text",
            "--final-message-only"
        ])
        
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
                cwd=work_dir or os.getcwd()
            )
            
            if result.returncode == 0:
                output = result.stdout.strip()
                self.logger.info("Kimi task completed successfully")
                return {
                    "success": True,
                    "output": output,
                    "model": "kimi-k2.5",
                    "error": None
                }
            else:
                error = result.stderr.strip() or "Unknown error"
                self.logger.error(f"Kimi task failed: {error}")
                return {
                    "success": False,
                    "output": None,
                    "model": "kimi-k2.5",
                    "error": error
                }
                
        except subprocess.TimeoutExpired:
            self.logger.error("Kimi task timed out")
            return {
                "success": False,
                "output": None,
                "model": "kimi-k2.5",
                "error": f"Timeout after {timeout}s"
            }
        except Exception as e:
            self.logger.error(f"Kimi execution error: {e}")
            return {
                "success": False,
                "output": None,
                "model": "kimi-k2.5",
                "error": str(e)
            }
    
    def execute_code_task(
        self,
        description: str,
        language: str = "python",
        work_dir: str = None
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
    
    def execute_review_task(
        self,
        code: str,
        filename: str = "code"
    ) -> Dict[str, Any]:
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
    if result['success']:
        print(f"   Code:\n{result['output'][:200]}...\n")
    
    print("=== Test Complete ===")
