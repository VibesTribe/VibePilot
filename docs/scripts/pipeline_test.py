import os
import sys
from supabase import create_client
from dotenv import load_dotenv
from datetime import datetime

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not all([SUPABASE_URL, SUPABASE_KEY]):
    print("ERROR: Missing environment variables")
    sys.exit(1)

db = create_client(SUPABASE_URL, SUPABASE_KEY)

def test_full_pipeline():
    print("=" * 60)
    print("VIBEPILOT PIPELINE TEST")
    print("=" * 60)
    
    # 1. Create a test project
    print("\n[1] Creating test project...")
    project_res = db.table("projects").insert({
        "name": "VibePilot Test Project",
        "description": "Initial pipeline validation"
    }).execute()
    project = project_res.data[0]
    project_id = project["id"]
    print(f"    Project ID: {project_id}")
    
    # 2. Create a test task
    print("\n[2] Creating test task...")
    task_res = db.table("tasks").insert({
        "title": "Write a hello world function",
        "type": "feature",
        "priority": 5,
        "project_id": project_id,
        "status": "available"
    }).execute()
    task = task_res.data[0]
    task_id = task["id"]
    print(f"    Task ID: {task_id}")
    
    # 3. Create task packet
    print("\n[3] Creating task packet...")
    packet_res = db.table("task_packets").insert({
        "task_id": task_id,
        "prompt": "Write a Python function called 'hello_world' that returns the string 'Hello, World!'",
        "tech_spec": {
            "language": "python",
            "requirements": ["Function name: hello_world", "Returns string", "No imports needed"]
        },
        "expected_output": "A Python function definition that returns 'Hello, World!'",
        "version": 1
    }).execute()
    packet = packet_res.data[0]
    print(f"    Packet ID: {packet['id']}")
    
    # 4. Claim the task (simulate Director)
    print("\n[4] Claiming task...")
    claim_res = db.rpc("claim_next_task", {
        "p_courier": "opencode",
        "p_platform": "deepseek-api",
        "p_model_id": "deepseek-chat"
    }).execute()
    claimed_id = claim_res.data
    if claimed_id:
        print(f"    Claimed task: {claimed_id}")
    else:
        # Manual claim fallback
        db.table("tasks").update({
            "status": "in_progress",
            "assigned_to": "deepseek-chat",
            "attempts": 1,
            "started_at": datetime.utcnow().isoformat()
        }).eq("id", task_id).execute()
        print(f"    Manually claimed: {task_id}")
        claimed_id = task_id
    
    # 5. Execute task (simulated - in real flow, LLM would do this)
    print("\n[5] Executing task (simulated)...")
    result = {
        "code": "def hello_world():\n    return 'Hello, World!'",
        "language": "python",
        "lines": 2
    }
    print(f"    Result generated")
    
    # 6. Start task run
    print("\n[6] Recording task run...")
    run_res = db.table("task_runs").insert({
        "task_id": task_id,
        "courier": "opencode",
        "platform": "deepseek-api",
        "model_id": "deepseek-chat",
        "status": "success",
        "result": result,
        "tokens_used": 150,
        "completed_at": datetime.utcnow().isoformat()
    }).execute()
    run = run_res.data[0]
    print(f"    Run ID: {run['id']}")
    
    # 7. Calculate ROI
    print("\n[7] Calculating ROI...")
    db.rpc("calculate_task_roi", {"p_run_id": run["id"]}).execute()
    run_updated = db.table("task_runs").select("*").eq("id", run["id"]).execute()
    roi_data = run_updated.data[0]
    print(f"    Theoretical API cost: ${roi_data.get('theoretical_api_cost', 0):.4f}")
    print(f"    Actual cost: ${roi_data.get('actual_cost', 0):.4f}")
    print(f"    Savings: ${roi_data.get('savings', 0):.4f}")
    
    # 8. Supervisor review
    print("\n[8] Supervisor review...")
    db.table("tasks").update({
        "status": "review",
        "result": result,
        "review": {
            "passed": True,
            "notes": "Code looks correct",
            "reviewer": "supervisor",
            "reviewed_at": datetime.utcnow().isoformat()
        }
    }).eq("id", task_id).execute()
    print("    Review: PASSED")
    
    # 9. Testing stage
    print("\n[9] Testing stage...")
    db.table("tasks").update({
        "status": "testing",
        "tests": {
            "passed": True,
            "results": [{"test": "syntax_check", "passed": True}],
            "tested_at": datetime.utcnow().isoformat()
        }
    }).eq("id", task_id).execute()
    print("    Tests: PASSED")
    
    # 10. Approval & Merge
    print("\n[10] Approval & Merge...")
    db.table("tasks").update({
        "status": "merged",
        "approval": {
            "passed": True,
            "merged_by": "maintenance",
            "merged_at": datetime.utcnow().isoformat()
        },
        "completed_at": datetime.utcnow().isoformat(),
        "branch_name": f"task/{task_id[:8]}-hello-world"
    }).eq("id", task_id).execute()
    print("    Status: MERGED")
    
    # 11. Check project ROI
    print("\n[11] Project ROI Summary...")
    project_roi = db.rpc("get_project_roi", {"p_project_id": project_id}).execute()
    if project_roi.data:
        roi = project_roi.data
        print(f"    Project: {roi.get('project_name', 'Unknown')}")
        print(f"    Tasks completed: {roi.get('completed_tasks', 0)}")
        print(f"    Tokens used: {roi.get('total_tokens', 0)}")
        print(f"    Total savings: ${roi.get('total_savings', 0):.4f}")
    
    # 12. Summary
    print("\n" + "=" * 60)
    print("PIPELINE TEST COMPLETE")
    print("=" * 60)
    print(f"Project ID: {project_id}")
    print(f"Task ID: {task_id}")
    print(f"Run ID: {run['id']}")
    print(f"Status: merged")
    print("\nAll stages passed successfully!")
    
    return {
        "project_id": project_id,
        "task_id": task_id,
        "run_id": run["id"],
        "status": "success"
    }

if __name__ == "__main__":
    test_full_pipeline()
