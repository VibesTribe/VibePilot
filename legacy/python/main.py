import os
import sys
import logging
import requests
from supabase import create_client
from dotenv import load_dotenv
from datetime import datetime

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)
logger = logging.getLogger("VibePilot")

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")
DS_KEY = os.getenv("DEEPSEEK_KEY")

if not all([SUPABASE_URL, SUPABASE_KEY, DS_KEY]):
    logger.error("Missing required environment variables")
    sys.exit(1)

client = create_client(str(SUPABASE_URL), str(SUPABASE_KEY))

def execute_task(task_desc: str) -> str:
    try:
        headers = {
            "Authorization": f"Bearer {DS_KEY}",
            "Content-Type": "application/json"
        }
        payload = {
            "model": "deepseek-chat",
            "messages": [{"role": "user", "content": task_desc}],
            "max_tokens": 1000
        }
        r = requests.post(
            "https://api.deepseek.com/v1/chat/completions",
            headers=headers,
            json=payload,
            timeout=30
        )
        if r.status_code == 200:
            return r.json()["choices"][0]["message"]["content"]
        else:
            return f"API Error [{r.status_code}]: {r.text[:200]}"
    except requests.Timeout:
        return "Error: API request timed out"
    except Exception as e:
        return f"Error: {str(e)}"

def process_one_task() -> bool:
    t_id = None
    try:
        res = client.table('task_backlog').select("*").eq('status', 'pending').limit(1).execute()
        tasks = res.data if res.data else []
        
        if not tasks:
            logger.info("No pending tasks found")
            return False
        
        task = tasks[0]
        t_id = task.get('id')
        t_desc = task.get('description', 'No description')
        
        logger.info(f"Dispatching task #{t_id}: {t_desc[:50]}...")
        
        client.table('task_backlog').update({
            "status": "processing",
            "started_at": datetime.utcnow().isoformat()
        }).eq('id', t_id).execute()
        
        result = execute_task(t_desc)
        
        client.table('task_backlog').update({
            "status": "completed",
            "result": result,
            "completed_at": datetime.utcnow().isoformat()
        }).eq('id', t_id).execute()
        
        logger.info(f"Task #{t_id} completed successfully")
        return True
        
    except Exception as e:
        logger.error(f"Task processing failed: {e}")
        if 't_id' in locals():
            try:
                client.table('task_backlog').update({
                    "status": "failed",
                    "result": f"Error: {str(e)}"
                }).eq('id', t_id).execute()
            except:
                pass
        return False

def main():
    logger.info("VibePilot Factory Started (One-Shot Mode)")
    
    try:
        task_processed = process_one_task()
        if task_processed:
            logger.info("Task processed. Exiting cleanly.")
        else:
            logger.info("No work to do. Exiting cleanly.")
    except KeyboardInterrupt:
        logger.info("Interrupted by user")
    except Exception as e:
        logger.error(f"Unexpected error: {e}")
        sys.exit(1)
    
    sys.exit(0)

if __name__ == "__main__":
    main()
