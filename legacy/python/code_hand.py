from .base import Agent, AgentResult
from typing import Dict, Any


class CodeHandAgent(Agent):
    name = "CodeHand"
    role = "Builder - Writes code following the Architect's plan"
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        task_type = task.get("type", "generate")
        
        if task_type == "generate":
            return self._generate_code(task)
        elif task_type == "modify":
            return self._modify_code(task)
        elif task_type == "refactor":
            return self._refactor_code(task)
        else:
            return AgentResult(
                success=False,
                output=None,
                error=f"Unknown task type: {task_type}"
            )
    
    def _generate_code(self, task: Dict[str, Any]) -> AgentResult:
        description = task.get("description", "")
        language = task.get("language", "python")
        architecture = task.get("architecture", "")
        
        if not description:
            return AgentResult(
                success=False,
                output=None,
                error="No description provided for code generation"
            )
        
        self.log(f"Generating {language} code: {description[:50]}...")
        
        prompt = f"""You are The Code Hand - an expert programmer. Generate clean, production-ready code.

LANGUAGE: {language}
TASK: {description}
{"ARCHITECTURE GUIDELINES:" + chr(10) + architecture if architecture else ""}

Generate the code following best practices:
1. Clean, readable code
2. Proper error handling
3. Clear variable names
4. Comments for complex logic only
5. Follow the architecture guidelines if provided

Return ONLY the code, no explanations."""

        try:
            code = self.call_llm(prompt, max_tokens=3000)
            self.log("Code generated successfully")
            
            return AgentResult(
                success=True,
                output=code,
                metadata={
                    "language": language,
                    "lines": len(code.split("\n")),
                    "type": "generated"
                }
            )
        except Exception as e:
            self.log(f"Code generation failed: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
    
    def _modify_code(self, task: Dict[str, Any]) -> AgentResult:
        existing_code = task.get("code", "")
        changes = task.get("changes", "")
        
        if not existing_code or not changes:
            return AgentResult(
                success=False,
                output=None,
                error="Missing existing code or changes description"
            )
        
        self.log("Modifying existing code...")
        
        prompt = f"""You are The Code Hand. Modify the following code according to the requested changes.

EXISTING CODE:
```
{existing_code}
```

CHANGES REQUESTED:
{changes}

Return the complete modified code. Preserve all functionality not mentioned in changes."""

        try:
            modified_code = self.call_llm(prompt, max_tokens=3000)
            self.log("Code modified successfully")
            
            return AgentResult(
                success=True,
                output=modified_code,
                metadata={"type": "modified"}
            )
        except Exception as e:
            self.log(f"Code modification failed: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
    
    def _refactor_code(self, task: Dict[str, Any]) -> AgentResult:
        code = task.get("code", "")
        goals = task.get("goals", "improve readability and performance")
        
        if not code:
            return AgentResult(
                success=False,
                output=None,
                error="No code provided for refactoring"
            )
        
        self.log("Refactoring code...")
        
        prompt = f"""You are The Code Hand. Refactor the following code.

CODE:
```
{code}
```

REFACTORING GOALS:
{goals}

Refactor while:
1. Preserving exact functionality
2. Improving code quality
3. Following DRY principle
4. Improving naming
5. Reducing complexity

Return the refactored code only."""

        try:
            refactored = self.call_llm(prompt, max_tokens=3000)
            self.log("Code refactored successfully")
            
            return AgentResult(
                success=True,
                output=refactored,
                metadata={"type": "refactored"}
            )
        except Exception as e:
            self.log(f"Refactoring failed: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
