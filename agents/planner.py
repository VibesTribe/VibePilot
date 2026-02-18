"""
VibePilot Planner Agent

Takes a zero-ambiguity PRD and creates a modular, isolated execution plan.
Uses vertical slicing to ensure tasks are atomic and isolated.

Output goes to Supabase tasks table for Orchestrator to pick up.

See config/prompts/planner.md for full behavior specification.
"""

import os
import json
import logging
from typing import Dict, Any, List, Optional
from supabase import create_client
from dotenv import load_dotenv
from datetime import datetime

from .base import Agent, AgentResult
from runners.kimi_runner import KimiRunner

load_dotenv()

logger = logging.getLogger("VibePilot.Planner")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    raise ValueError("Missing SUPABASE_URL or SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)

# Planner uses Kimi CLI (internal agent, per PRD routing rules)
kimi_runner = KimiRunner()


class PlannerAgent(Agent):
    name = "Planner"
    role = "Breaks PRDs into atomic tasks using vertical slicing"
    
    def __init__(self):
        super().__init__()
        self.planner_prompt = self._load_planner_prompt()
    
    def _load_planner_prompt(self) -> str:
        """Load the full planner prompt from config."""
        prompt_path = os.path.join(
            os.path.dirname(__file__), 
            "..", "config", "prompts", "planner.md"
        )
        try:
            with open(prompt_path, "r") as f:
                return f.read()
        except Exception as e:
            self.logger.warning(f"Could not load planner prompt from file: {e}")
            return self._get_fallback_prompt()
    
    def _get_fallback_prompt(self) -> str:
        """Fallback prompt if file not found."""
        return """You are the Planner agent. Create a modular execution plan.

Break the PRD into:
1. Slices (isolated functional areas)
2. Phases within slices (P1, P2, P3)
3. Tasks within phases (atomic, 95%+ confidence)

Each task needs:
- task_id: SLICE-PHASE-TNNN format (e.g., RES-P1-T001)
- slice_id: Which slice it belongs to
- phase: P1, P2, or P3
- title: Clear task title
- routing_flag: "internal" or "web"
- routing_flag_reason: Why this flag
- dependencies: Array of dependent task_ids
- confidence: 0.95-1.0
- prompt_packet: Full instructions for the task

Output valid JSON following the format in config/prompts/planner.md."""
    
    def _call_planner_llm(self, prompt: str, max_tokens: int = 8000) -> str:
        """
        Call LLM for planning. Uses Kimi CLI (internal agent routing).
        
        Per PRD: Internal agents (Planner, Supervisor, Council) use CLI subscriptions,
        not API or web platforms.
        """
        result = kimi_runner.execute_task(
            prompt=prompt,
            timeout=300,
            auto_approve=True
        )
        
        if result["success"]:
            return result["output"]
        else:
            raise Exception(f"Kimi planning failed: {result.get('error', 'Unknown error')}")
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        """
        Execute planning on a PRD.
        
        Input:
            task = {
                "prd": "Full PRD text...",
                "project_id": "optional project reference"
            }
        
        Output:
            AgentResult with plan and Supabase write results
        """
        prd = task.get("prd", "")
        project_id = task.get("project_id")
        
        if not prd:
            return AgentResult(
                success=False,
                output=None,
                error="No PRD provided for planning"
            )
        
        self.log("Breaking down PRD into atomic tasks...")
        
        # Build the full prompt
        full_prompt = f"""{self.planner_prompt}

---

# NOW PLAN THIS PRD:

{prd}

---

Remember:
1. Identify natural slice boundaries (things that won't affect each other)
2. Group features into isolated slices
3. Phase work within each slice (P1=Foundation, P2=Features, P3=Polish)
4. Create atomic tasks with full prompt_packets
5. Flag routing correctly (internal vs web)
6. Map dependencies explicitly
7. Ensure 95%+ confidence on every task

Return ONLY valid JSON following the output format specified above."""
        
        try:
            # Call Kimi for planning
            response = self._call_planner_llm(full_prompt)
            
            # Parse JSON from response
            plan = self._parse_plan(response)
            
            if not plan:
                return AgentResult(
                    success=False,
                    output=None,
                    error="Could not parse plan from LLM response"
                )
            
            # Validate plan structure
            validation = self._validate_plan(plan)
            if not validation["valid"]:
                return AgentResult(
                    success=False,
                    output=plan,
                    error=f"Plan validation failed: {validation['errors']}"
                )
            
            # Extract all tasks from plan
            all_tasks = self._extract_tasks(plan)
            self.log(f"Generated {len(all_tasks)} atomic tasks across {len(plan.get('slices', []))} slices")
            
            # Write to Supabase
            write_result = self._write_tasks_to_supabase(all_tasks, project_id)
            
            if write_result["errors"]:
                self.log(f"Some tasks failed to write: {write_result['errors']}", level="warning")
            
            return AgentResult(
                success=True,
                output={
                    "plan": plan,
                    "tasks_written": write_result["written"],
                    "task_ids": write_result["task_ids"],
                    "write_errors": write_result["errors"]
                },
                metadata={
                    "task_count": len(all_tasks),
                    "slice_count": len(plan.get("slices", [])),
                    "source": "planner"
                }
            )
            
        except Exception as e:
            self.log(f"Failed to create plan: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
    
    def _parse_plan(self, response: str) -> Optional[Dict]:
        """Parse plan JSON from LLM response."""
        try:
            return json.loads(response)
        except json.JSONDecodeError:
            # Try to extract JSON from response
            start = response.find("{")
            end = response.rfind("}") + 1
            if start != -1 and end > start:
                try:
                    return json.loads(response[start:end])
                except json.JSONDecodeError:
                    pass
            
            # Try to find JSON array
            start = response.find("[")
            end = response.rfind("]") + 1
            if start != -1 and end > start:
                try:
                    tasks = json.loads(response[start:end])
                    return {"slices": [{"slice_id": "default", "phases": [{"phase_id": "P1", "tasks": tasks}]}]}
                except json.JSONDecodeError:
                    pass
            
            return None
    
    def _validate_plan(self, plan: Dict) -> Dict:
        """Validate plan has required structure."""
        errors = []
        
        if "slices" not in plan:
            errors.append("Missing 'slices' in plan")
            return {"valid": False, "errors": errors}
        
        for slice_data in plan.get("slices", []):
            if "slice_id" not in slice_data:
                errors.append(f"Slice missing 'slice_id': {slice_data}")
            
            for phase in slice_data.get("phases", []):
                for task in phase.get("tasks", []):
                    # Check required task fields
                    required = ["task_id", "slice_id", "title", "routing_flag"]
                    for field in required:
                        if field not in task:
                            errors.append(f"Task missing '{field}': {task.get('title', 'unknown')}")
        
        return {"valid": len(errors) == 0, "errors": errors}
    
    def _extract_tasks(self, plan: Dict) -> List[Dict]:
        """Extract all tasks from plan structure."""
        all_tasks = []
        
        for slice_data in plan.get("slices", []):
            slice_id = slice_data.get("slice_id", "unknown")
            
            for phase in slice_data.get("phases", []):
                phase_id = phase.get("phase_id", "P1")
                
                for task in phase.get("tasks", []):
                    # Ensure task has slice_id and phase
                    task["slice_id"] = task.get("slice_id", slice_id)
                    task["phase"] = task.get("phase", phase_id)
                    all_tasks.append(task)
        
        return all_tasks
    
    def _write_tasks_to_supabase(self, tasks: List[Dict], project_id: str = None) -> Dict:
        """
        Write tasks to Supabase tasks table.
        
        Each task gets:
        - task_number: Human-readable ID (RES-P1-T001)
        - slice_id, phase, routing_flag from planner
        - status: "pending"
        - result: Contains prompt_packet and other planning data
        """
        written = 0
        task_ids = []
        errors = []
        
        # First pass: Insert all tasks to get UUIDs
        task_uuid_map = {}  # task_number -> uuid
        
        for task in tasks:
            task_number = task.get("task_id", f"TASK-{written}")
            
            task_record = {
                "task_number": task_number,
                "title": task.get("title", "Untitled task"),
                "type": task.get("type", "feature"),
                "priority": task.get("priority", 5),
                "slice_id": task.get("slice_id"),
                "phase": task.get("phase"),
                "routing_flag": task.get("routing_flag", "web"),
                "routing_flag_reason": task.get("routing_flag_reason"),
                "status": "pending",
                "result": {
                    "prompt_packet": task.get("prompt_packet", ""),
                    "expected_output": task.get("expected_output"),
                    "confidence": task.get("confidence"),
                    "confidence_reasoning": task.get("confidence_reasoning"),
                    "objectives": task.get("objectives"),
                    "deliverables": task.get("deliverables"),
                    "suggested_agent": task.get("suggested_agent"),
                }
            }
            
            if project_id:
                task_record["project_id"] = project_id
            
            try:
                result = db.table("tasks").insert(task_record).execute()
                if result.data:
                    uuid = result.data[0].get("id")
                    task_uuid_map[task_number] = uuid
                    task_ids.append(task_number)
                    written += 1
                    self.log(f"Wrote task {task_number} (UUID: {uuid})")
            except Exception as e:
                errors.append({"task_id": task_number, "error": str(e)})
                self.log(f"Failed to write task {task_number}: {e}", level="error")
        
        # Second pass: Update dependencies (now we have UUIDs)
        for task in tasks:
            task_number = task.get("task_id")
            deps = task.get("dependencies", [])
            
            if deps and task_number in task_uuid_map:
                # Convert dependency task_numbers to UUIDs
                dep_uuids = []
                for dep in deps:
                    if isinstance(dep, dict):
                        dep_id = dep.get("task_id")
                    else:
                        dep_id = dep
                    
                    if dep_id in task_uuid_map:
                        dep_uuids.append(task_uuid_map[dep_id])
                
                if dep_uuids:
                    try:
                        db.table("tasks").update({
                            "dependencies": dep_uuids
                        }).eq("id", task_uuid_map[task_number]).execute()
                        self.log(f"Updated dependencies for {task_number}: {len(dep_uuids)} deps")
                    except Exception as e:
                        self.log(f"Failed to update dependencies for {task_number}: {e}", level="warning")
        
        return {
            "written": written,
            "task_ids": task_ids,
            "errors": errors,
            "uuid_map": task_uuid_map
        }


def create_research_module_plan() -> AgentResult:
    """
    Helper function to create a Research module plan.
    Used for testing the planner infrastructure.
    """
    research_prd = """# Research Module PRD

## Overview
VibePilot needs a research capability for autonomous improvement.

## Requirements

### 1. Daily Research (Scheduled)
- Automatically scan AI development sources daily
- Sources: r/LocalLLaMA, Latent Space, Simon Willison's blog, Hugging Face trending
- Extract findings relevant to VibePilot improvement
- Tag findings as SIMPLE (direct add) or VET (needs Council review)
- Output to research-considerations branch

### 2. Inquiry Research (On-Demand)
- Accept research requests from human via Vibes
- Research specific topics, videos, articles
- Apply same output format as daily research
- Include token/cost tracking

## Technical Constraints
- Uses Kimi CLI for execution
- No codebase access needed (research is standalone)
- Output goes to research-considerations branch
- All tasks are web-flagged (can run anywhere)

## Success Criteria
- Daily research runs automatically
- Inquiry research responds to requests
- Findings are structured with SIMPLE/VET tags
- Token usage is tracked for ROI
"""
    
    planner = PlannerAgent()
    result = planner.execute({"prd": research_prd, "project_id": "550e8400-e29b-41d4-a716-446655440001"})
    
    return result


if __name__ == "__main__":
    print("=" * 60)
    print("VIBEPILOT PLANNER AGENT")
    print("=" * 60)
    
    # Test with Research module
    print("\nCreating Research Module plan...")
    result = create_research_module_plan()
    
    if result.success:
        print(f"\n[OK] Plan created successfully")
        print(f"  Tasks written: {result.output.get('tasks_written', 0)}")
        print(f"  Task IDs: {result.output.get('task_ids', [])}")
        if result.output.get('write_errors'):
            print(f"  Errors: {result.output['write_errors']}")
    else:
        print(f"\n[FAIL] Plan creation failed: {result.error}")
    
    print("\n" + "=" * 60)
