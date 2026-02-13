import json
from .base import Agent, AgentResult
from typing import Dict, Any, List


class PlannerAgent(Agent):
    name = "Planner"
    role = "Breaks PRDs into atomic tasks using vertical slicing"
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        prd = task.get("prd", "")
        
        if not prd:
            return AgentResult(
                success=False,
                output=None,
                error="No PRD provided for planning"
            )
        
        self.log("Breaking down PRD into atomic tasks...")
        
        prompt = f"""You are The Planner. Your job is to break down a PRD into atomic, executable tasks using VERTICAL SLICING.

PRD:
{prd}

Generate a JSON array of tasks. Each task should be:
- Atomic (single responsibility)
- Testable (clear success criteria)
- Estimated (complexity: low/medium/high)

Return ONLY valid JSON in this format:
{{
  "tasks": [
    {{
      "id": 1,
      "title": "Task title",
      "description": "Detailed description",
      "type": "setup|feature|test|docs",
      "complexity": "low|medium|high",
      "dependencies": [],
      "acceptance_criteria": ["criteria 1", "criteria 2"]
    }}
  ]
}}

Use vertical slicing: each task should deliver a complete slice of functionality."""

        try:
            response = self.call_llm(prompt, max_tokens=3000)
            
            try:
                plan = json.loads(response)
            except json.JSONDecodeError:
                start = response.find("{")
                end = response.rfind("}") + 1
                if start != -1 and end > start:
                    plan = json.loads(response[start:end])
                else:
                    raise ValueError("Could not parse JSON from LLM response")
            
            tasks = plan.get("tasks", [])
            self.log(f"Generated {len(tasks)} atomic tasks")
            
            return AgentResult(
                success=True,
                output=tasks,
                metadata={"task_count": len(tasks), "source": "planner"}
            )
        except Exception as e:
            self.log(f"Failed to create plan: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
