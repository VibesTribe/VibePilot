import os
from supabase import create_client
from dotenv import load_dotenv
load_dotenv()
url = os.getenv('SUPABASE_URL')
key = os.getenv('SUPABASE_KEY')
print('Checking Supabase...')
if not url or not key: print('Missing .env')
else:
 c = create_client(url, key)
 tabs = ['task_backlog', 'secrets_vault', 'lessons_learned', 'agent_tasks']
 for t in tabs:
  try:
   c.table(t).select('*').limit(1).execute()
   print(f'Found: {t}')
  except: print(f'Missing: {t}')
