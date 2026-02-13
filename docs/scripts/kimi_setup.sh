# Kimi CLI Setup Commands

# 1. Add Kimi to PATH (permanent)
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# 2. Authenticate Kimi
kimi auth login

# 3. Test Kimi
kimi "Say hello"

# 4. Add to VibePilot (run after auth confirmed)
cd /home/mjlockboxsocial/vibepilot
source venv/bin/activate
python -c "
from supabase import create_client
from dotenv import load_dotenv
import os
load_dotenv()
db = create_client(os.getenv('SUPABASE_URL'), os.getenv('SUPABASE_KEY'))
db.table('models').insert({
    'id': 'kimi-k2.5',
    'platform': 'kimi-cli',
    'courier': 'kimi',
    'context_limit': 128000,
    'strengths': ['code', 'reasoning', 'large-context'],
    'status': 'active',
    'request_limit': 1000,
    'api_cost_per_1k_tokens': 0
}).execute()
print('Kimi added to models table')
"
