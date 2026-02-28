from .base import Agent, AgentResult
from typing import Dict, Any, List, Optional
from datetime import datetime


class DirectorAgent(Agent):
    name = "Director"
    role = "Orchestrator - Manages agent flow, budget, and task dispatch"
    
    def __init__(self):
        super().__init__()
        self.budget = 1000
        self.spent = 0
        self.completed_tasks = 0
        self.failed_tasks = 0
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        action = task.get("action", "status")
        
        if action == "status":
            return self._get_status()
        elif action == "dispatch":
            return self._dispatch_task(task)
        elif action == "report":
            return self._report_completion(task)
        else:
            return AgentResult(
                success=False,
                output=None,
                error=f"Unknown action: {action}"
            )
    
    def _get_status(self) -> AgentResult:
        return AgentResult(
            success=True,
            output={
                "budget_remaining": self.budget - self.spent,
                "budget_used": self.spent,
                "completed_tasks": self.completed_tasks,
                "failed_tasks": self.failed_tasks,
                "health": "healthy" if self.budget - self.spent > 100 else "low_budget"
            }
        )
    
    def _dispatch_task(self, task: Dict[str, Any]) -> AgentResult:
        agent_name = task.get("agent")
        cost = task.get("cost", 10)
        
        if self.spent + cost > self.budget:
            self.log(f"Budget exceeded: {self.spent + cost} > {self.budget}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error="Budget exceeded - cannot dispatch task"
            )
        
        self.spent += cost
        self.log(f"Dispatching to {agent_name} (cost: {cost})")
        
        return AgentResult(
            success=True,
            output={
                "dispatched_to": agent_name,
                "cost": cost,
                "remaining_budget": self.budget - self.spent
            },
            metadata={"action": "dispatch", "timestamp": datetime.utcnow().isoformat()}
        )
    
    def _report_completion(self, task: Dict[str, Any]) -> AgentResult:
        success = task.get("success", False)
        
        if success:
            self.completed_tasks += 1
            self.log(f"Task completed. Total: {self.completed_tasks}")
        else:
            self.failed_tasks += 1
            self.log(f"Task failed. Total: {self.failed_tasks}", level="warning")
        
        return AgentResult(
            success=True,
            output={
                "completed": self.completed_tasks,
                "failed": self.failed_tasks
            }
        )
