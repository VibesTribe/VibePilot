import os
import time
import subprocess
from supabase import create_client
from dotenv import load_dotenv
from vault_manager import VaultManager

load_dotenv()
client = create_client(os.getenv("SUPABASE_URL"), os.getenv("SUPABASE_KEY"))
vault = VaultManager()

# --- 1. THE SPLITTER LOGIC ---
def execute_split_task(parent_task, model):
    parent_id = parent_task['task_id']
    # Create Sub-Task A (Reasoning)
    sub_a = {
        "task_id": f"{parent_id}-A-brain",
        "name": f"Plan for {parent_task['name']}",
        "status": "ready",
        "technical_spec": {
            "prompt": f"Plan this task: {parent_task['technical_spec']['prompt']}",
            "skill_required": "reasoning",
            "estimated_tokens": 2000
        },
        "parent_id": parent_id
    }
    # Update Parent to wait for A
    parent_task['dependencies'] = [sub_a['task_id']]
    parent_task['technical_spec']['prompt'] = "Execute the plan from Task A."
    parent_task['technical_spec']['estimated_tokens'] = 2000
    
    client.table('task_backlog').insert(sub_a).execute()
    client.table('task_backlog').update({"dependencies": parent_task['dependencies']}).eq("task_id", parent_id).execute()
    print("✂️ Task Split.")
    return "Split"

# --- 2. THE SMART ROUTER ---
def smart_router(task):
    candidates = client.table('model_registry').select("*").eq('status', 'active').execute().data
    if not candidates: return handle_shortage(task)
    
    # Sort by Grade
    best_fit = sorted(candidates, key=lambda x: x['grade_score'], reverse=True)[0]
    estimated = task.get('predicted_context', 5000)
    
    # Glass Ceiling Check
    if estimated > best_fit['context_window_max']:
        print(f"⚠️ Splitting task (Est: {estimated}, Limit: {best_fit['context_window_max']})")
        return execute_split_task(task, best_fit)
    
    return best_fit

# --- 3. CORPORATE BRAIN CHECK ---
def check_lessons_learned(task_name):
    # Simple mockup of vector search (In prod, use supabase.rpc match_lessons)
    # For now, we just check if we have any lessons stored.
    lessons = client.table('lessons_learned').select("*").limit(5).execute().data
    if lessons:
        print(f"🧠 Warning: {len(lessons)} historical lessons found. Reviewing...")
        # In v1.2, we would match embeddings here.

# --- 4. LANE LOCKING ---
def acquire_lock(task):
    # Assuming task has a 'target_file' in spec or metadata
    resource = task['technical_spec'].get('target_file', 'global')
    existing = client.table('lane_locks').select("*").eq('resource_path', resource).eq('status', 'active').execute().data
    if existing:
        print(f"🚧 Lane {resource} locked by Task {existing[0]['task_id']}. Waiting...")
        return False
    client.table('lane_locks').insert({"resource_path": resource, "task_id": task['id']}).execute()
    return True

def release_lock(task):
    resource = task['technical_spec'].get('target_file', 'global')
    client.table('lane_locks').update({"status": 'released'}).eq('task_id', task['id']).execute()

# --- 5. MAIN HEARTBEAT ---
def heartbeat():
    print("🏛️ VibePilot v1.2 Sovereign Heartbeat...")
    while True:
        # 1. Get Tasks
        tasks = client.table('task_backlog').select("*").eq('status', 'ready').execute().data
        if tasks:
            task = tasks[0]
            print(f"⚡ Task: {task['name']}")
            
            # 2. Check Lessons
            check_lessons_learned(task['name'])
            
            # 3. Lane Lock
            if not acquire_lock(task): 
                time.sleep(5); continue
            
            # 4. Route
            model = smart_router(task)
            print(f"🚀 Dispatching to {model['model_id']}...")
            
            # 5. Simulate Execution (In prod, this calls the LLM/CLI)
            # time.sleep(2)
            
            # 6. Executioner Test (Vibe Tensor style)
            test_cmd = task.get('test_command')
            if test_cmd:
                print(f"🧪 Running Test: {test_cmd}")
                # subprocess.run(test_cmd, shell=True) # Uncomment in prod
            
            # 7. Complete & Release
            client.table('task_backlog').update({"status": "completed"}).eq("task_id", task['task_id']).execute()
            release_lock(task)
            print("✅ Task Complete.")
        else:
            # Chat Queue Check
            chats = client.table('chat_queue').select("*").eq('role', 'user').execute().data
            if chats:
                msg = chats[0]['content']
                print(f"💬 User said: {msg}")
                # Logic to reply goes here
                client.table('chat_queue').update({"role": 'assistant', "content": "I heard you."}).eq("id", chats[0]['id']).execute()
            else:
                print("...heartbeat...")
        
        time.sleep(5)

if __name__ == "__main__":
    try: heartbeat()
    except Exception as e: print(f"❌ Critical Failure: {e}")
