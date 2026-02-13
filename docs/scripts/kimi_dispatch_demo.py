import os
import sys
from supabase import create_client
from dotenv import load_dotenv
from runners.kimi_runner import KimiRunner

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)

def test_kimi_dispatch():
    print("=" * 60)
    print("KIMI DISPATCH TEST")
    print("=" * 60)
    
    # Get Kimi model info
    model = db.table("models").select("*").eq("id", "kimi-k2.5").execute()
    if not model.data:
        print("ERROR: Kimi not in models table")
        return
    
    print(f"\nModel: {model.data[0]['id']}")
    print(f"Status: {model.data[0]['status']}")
    print(f"Context: {model.data[0]['context_limit']} tokens")
    
    # Create test task
    print("\n[1] Creating task...")
    task_res = db.table("tasks").insert({
        "title": "Kimi Test: Fibonacci function",
        "type": "feature",
        "priority": 5,
        "status": "in_progress",
        "assigned_to": "kimi-k2.5"
    }).execute()
    task_id = task_res.data[0]["id"]
    print(f"    Task ID: {task_id}")
    
    # Run via Kimi
    print("\n[2] Dispatching to Kimi...")
    runner = KimiRunner()
    result = runner.execute_code_task(
        "Write a Python function that calculates the nth Fibonacci number",
        language="python"
    )
    
    if result["success"]:
        print("    Success!")
        print(f"\n[3] Code Generated:")
        print("-" * 40)
        print(result["output"][:500])
        print("-" * 40)
        
        # Update task
        db.table("tasks").update({
            "status": "review",
            "result": {"code": result["output"], "success": True}
        }).eq("id", task_id).execute()
        
        # Record run
        run_res = db.table("task_runs").insert({
            "task_id": task_id,
            "courier": "opencode",
            "platform": "kimi-cli",
            "model_id": "kimi-k2.5",
            "status": "success",
            "result": {"output": result["output"]},
            "tokens_used": 200
        }).execute()
        
        print(f"\n[4] Run recorded: {run_res.data[0]['id']}")
        print("\n✅ Kimi dispatch test complete!")
        
    else:
        print(f"    Failed: {result.get('error')}")
        db.table("tasks").update({
            "status": "failed",
            "failure_notes": result.get("error")
        }).eq("id", task_id).execute()
    
    print("=" * 60)

if __name__ == "__main__":
    test_kimi_dispatch()
