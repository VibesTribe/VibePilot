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


class Orchestrator:
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
        
        self.logger = logging.getLogger("VibePilot.Orchestrator")
    
    def process_task(self, task: Dict[str, Any]) -> Dict[str, Any]:
        task_type = task.get("type", "idea")
        
        self.logger.info(f"Processing task type: {task_type}")
        
        if task_type == "idea":
            return self._process_idea(task)
        elif task_type == "code_review":
            return self._process_code_review(task)
        elif task_type == "generate_code":
            return self._process_code_generation(task)
        else:
            return {"success": False, "error": f"Unknown task type: {task_type}"}
    
    def _process_idea(self, task: Dict[str, Any]) -> Dict[str, Any]:
        idea = task.get("description", "")
        
        self.logger.info("Phase 1: Consulting on idea...")
        prd_result = self.consultant.execute({"description": idea})
        
        if not prd_result.success:
            return {"success": False, "error": "Consultant failed", "details": prd_result.error}
        
        prd = prd_result.output
        self.logger.info("Phase 2: Planning tasks...")
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
    
    def _process_code_review(self, task: Dict[str, Any]) -> Dict[str, Any]:
        code = task.get("code", "")
        filename = task.get("filename", "unknown")
        
        self.logger.info(f"Running Council review on {filename}...")
        
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
    
    def _process_code_generation(self, task: Dict[str, Any]) -> Dict[str, Any]:
        description = task.get("description", "")
        language = task.get("language", "python")
        
        self.logger.info(f"Generating {language} code...")
        
        gen_result = self.code_hand.execute({
            "type": "generate",
            "description": description,
            "language": language
        })
        
        if not gen_result.success:
            return {"success": False, "error": gen_result.error}
        
        code = gen_result.output
        
        review_result = self._process_code_review({"code": code, "filename": f"generated.{language}"})
        
        return {
            "success": True,
            "code": code,
            "review": review_result
        }
    
    def run_from_backlog(self) -> Optional[Dict[str, Any]]:
        try:
            res = db.table("task_backlog").select("*").eq("status", "pending").limit(1).execute()
            tasks = res.data if res.data else []
            
            if not tasks:
                self.logger.info("No pending tasks")
                return None
            
            task = tasks[0]
            task_id = task.get("id")
            
            db.table("task_backlog").update({"status": "processing"}).eq("id", task_id).execute()
            
            result = self.process_task(task)
            
            status = "completed" if result.get("success") else "failed"
            db.table("task_backlog").update({
                "status": status,
                "result": result
            }).eq("id", task_id).execute()
            
            self.logger.info(f"Task {task_id} {status}")
            return result
            
        except Exception as e:
            self.logger.error(f"Error processing task: {e}")
            return {"success": False, "error": str(e)}


if __name__ == "__main__":
    orch = Orchestrator()
    result = orch.run_from_backlog()
    if result:
        print(json.dumps(result, indent=2, default=str))
