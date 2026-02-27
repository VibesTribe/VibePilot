import os
import json
import logging
from typing import Dict, Any, Optional, List
from supabase import create_client
from dotenv import load_dotenv

from agents import (
    ConsultantAgent,
    PlannerAgent,
    ArchitectAgent,
    SecurityAgent,
    MaintenanceAgent,
    DirectorAgent,
    ExecutionerAgent,
    CodeHandAgent,
)
from runners.kimi_runner import KimiRunner

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)
logger = logging.getLogger("VibePilot")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not all([SUPABASE_URL, SUPABASE_KEY]):
    logger.error("Missing required environment variables")
    exit(1)

db = create_client(str(SUPABASE_URL), str(SUPABASE_KEY))


class ModelRouter:
    def __init__(self):
        self.kimi = KimiRunner()
        self.logger = logging.getLogger("VibePilot.ModelRouter")
    
    def get_available_models(self) -> List[Dict]:
        res = db.table("models").select("*").eq("status", "active").execute()
        return res.data if res.data else []
    
    def select_model(self, task: Dict) -> str:
        task_type = task.get("type", "feature")
        priority = task.get("priority", 5)
        dependencies = task.get("dependencies", [])
        
        models = self.get_available_models()
        if not models:
            return None
        
        if len(dependencies) > 2 or priority <= 2:
            return "glm-5"
        
        if task_type in ["research", "analysis", "docs"]:
            if self.kimi.is_available():
                return "kimi-k2.5"
        
        if task_type == "feature" and len(dependencies) == 0:
            if self.kimi.is_available():
                return "kimi-k2.5"
        
        return "glm-5"
    
    def dispatch_to_kimi(self, prompt: str, task_type: str = "code") -> Dict:
        self.logger.info("Dispatching to Kimi...")
        
        if task_type == "code":
            return self.kimi.execute_code_task(prompt)
        else:
            return self.kimi.execute_task(prompt)
    
    def dispatch_to_glm(self, prompt: str, context: Dict = None) -> Dict:
        self.logger.info("Dispatching to GLM-5 (OpenCode)...")
        
        return {
            "success": True,
            "output": f"[GLM-5 would execute: {prompt[:50]}...]",
            "model": "glm-5",
            "note": "GLM-5 is the current session - execute directly"
        }


class DualModelOrchestrator:
    def __init__(self):
        self.consultant = ConsultantAgent()
        self.planner = PlannerAgent()
        self.council = {
            "architect": ArchitectAgent(),
            "security": SecurityAgent(),
            "maintenance": MaintenanceAgent(),
        }
        self.director = DirectorAgent()
        self.executioner = ExecutionerAgent()
        self.code_hand = CodeHandAgent()
        self.router = ModelRouter()
        
        self.logger = logging.getLogger("VibePilot.Orchestrator")
    
    def process_task(self, task: Dict) -> Dict[str, Any]:
        task_type = task.get("type", "idea")
        
        self.logger.info(f"Processing task type: {task_type}")
        
        if task_type == "idea":
            return self._process_idea(task)
        elif task_type == "code":
            return self._process_code_task(task)
        elif task_type == "review":
            return self._process_code_review(task)
        else:
            return {"success": False, "error": f"Unknown task type: {task_type}"}
    
    def _process_idea(self, task: Dict) -> Dict:
        idea = task.get("description", "")
        
        prd_result = self.consultant.execute({"description": idea})
        if not prd_result.success:
            return {"success": False, "error": "Consultant failed", "details": prd_result.error}
        
        prd = prd_result.output
        self.logger.info("PRD generated")
        
        plan_result = self.planner.execute({"prd": prd})
        if not plan_result.success:
            return {"success": False, "error": "Planner failed", "details": plan_result.error}
        
        tasks = plan_result.output
        
        return {
            "success": True,
            "prd": prd,
            "tasks": tasks,
            "task_count": len(tasks)
        }
    
    def _process_code_task(self, task: Dict) -> Dict:
        prompt = task.get("description", "")
        task_id = task.get("id")
        
        selected_model = self.router.select_model(task)
        self.logger.info(f"Selected model: {selected_model}")
        
        if selected_model == "kimi-k2.5":
            result = self.router.dispatch_to_kimi(prompt, "code")
        else:
            result = self.router.dispatch_to_glm(prompt)
        
        if task_id and result.get("success"):
            db.table("task_runs").insert({
                "task_id": task_id,
                "courier": "orchestrator",
                "platform": "kimi-cli" if selected_model == "kimi-k2.5" else "opencode",
                "model_id": selected_model,
                "status": "success",
                "result": {"output": result.get("output")},
                "tokens_used": result.get("tokens", 200)
            }).execute()
        
        return result
    
    def _process_code_review(self, task: Dict) -> Dict:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        results = {}
        
        arch_result = self.council["architect"].execute({"code": code, "filename": filename})
        results["architecture"] = {"passed": arch_result.success, "issues": arch_result.output.get("issues", [])}
        
        sec_result = self.council["security"].execute({"code": code, "filename": filename})
        results["security"] = {"passed": sec_result.success, "issues": sec_result.output.get("issues", [])}
        
        maint_result = self.council["maintenance"].execute({"code": code, "filename": filename})
        results["maintenance"] = {
            "passed": maint_result.success,
            "score": maint_result.output.get("score", 0),
            "warnings": maint_result.output.get("warnings", [])
        }
        
        all_passed = all(r["passed"] for r in results.values())
        
        return {
            "success": all_passed,
            "reviews": results,
            "filename": filename
        }
    
    def run_dispatch_loop(self, max_tasks: int = 5):
        """Process up to max_tasks from the queue."""
        processed = 0
        
        while processed < max_tasks:
            tasks = db.table("tasks").select("*").eq("status", "available").limit(1).execute()
            
            if not tasks.data:
                self.logger.info("No available tasks")
                break
            
            task = tasks.data[0]
            task_id = task["id"]
            
            db.table("tasks").update({"status": "in_progress"}).eq("id", task_id).execute()
            
            result = self.process_task(task)
            
            status = "review" if result.get("success") else "failed"
            db.table("tasks").update({
                "status": status,
                "result": result
            }).eq("id", task_id).execute()
            
            processed += 1
            self.logger.info(f"Task {task_id} → {status}")
        
        return processed


if __name__ == "__main__":
    orch = DualModelOrchestrator()
    
    print("=" * 60)
    print("VIBEPILOT DUAL-MODEL ORCHESTRATOR")
    print("=" * 60)
    
    models = orch.router.get_available_models()
    print(f"\nActive Models: {len(models)}")
    for m in models:
        print(f"  - {m['id']} ({m['platform']})")
    
    print("\n" + "=" * 60)
    
    test_task = {
        "type": "code",
        "description": "Write a Python function that checks if a string is a palindrome",
        "priority": 5,
        "dependencies": []
    }
    
    print("Test: Dispatching code task...")
    selected = orch.router.select_model(test_task)
    print(f"Selected: {selected}")
    
    result = orch.process_task(test_task)
    
    if result.get("success"):
        print("\n✅ Success!")
        print(f"Model: {result.get('model', 'unknown')}")
        print(f"\nOutput:\n{result.get('output', 'N/A')[:300]}...")
    else:
        print(f"\n❌ Failed: {result.get('error')}")
    
    print("\n" + "=" * 60)
