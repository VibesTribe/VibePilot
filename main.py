import os
import time
import requests
from supabase import create_client
from dotenv import load_dotenv

load_dotenv()

client = create_client(os.getenv("SUPABASE_URL"), os.getenv("SUPABASE_KEY"))
DS_KEY = os.getenv("DEEPSEEK_KEY")

print("🏭 VibePilot Factory Online.")

def execute_task(task_desc):
    try:
        headers = {"Authorization": f"Bearer {DS_KEY}", "Content-Type": "application/json"}
        payload = {
            "model": "deepseek-chat",
            "messages": [{"role": "user", "content": task_desc}],
            "max_tokens": 1000
        }
        r = requests.post("https://api.deepseek.com/v1/chat/completions", headers=headers, json=payload, timeout=30)
        if r.status_code == 200:
            return r.json()["choices"][0]["message"]["content"]
        else:
            return f"API Error: {r.text}"
    except Exception as e:
        return f"Error: {e}"

def main_loop():
    while True:
        try:
            res = client.table('task_backlog').select("*").eq('status', 'pending').execute()
            tasks = res.data
            if tasks:
                for task in tasks:
                    t_id = task['id']
                    t_desc = task['description']
                    print(f"🚀 Dispatching: {t_desc}")
                    result = execute_task(t_desc)
                    client.table('task_backlog').update({
                        "status": "completed",
                        "result": result
                    }).eq('id', t_id).execute()
                    print(f"✅ Task Done.")
            else:
                print("...Idle...")
            time.sleep(5)
        except Exception as e:
            print(f"Error: {e}")
            time.sleep(5)

if __name__ == "__main__":
    main_loop()
