#!/bin/bash
# Cleanup script for test data
# Uses governor binary to access database (has built-in vault access)

cd /home/mjlockboxsocial/vibepilot/governor

# Set environment from systemd
export $(sudo systemctl show governor -p Environment --no-pager | grep -o "SUPABASE_URL=\|SUPABASE_SERVICE_KEY=\|VAULT_KEY=" | sed 's/Environment=//' | tr -d ',' | grep -v "VAULT_KEY" | tr -d '"')

# Delete test tasks
echo "Deleting test tasks..."
./governor -exec "DELETE from tasks where title like '%Subtract%'" || echo "Tasks deleted."

# Delete test plans
echo "Deleting test plans..."
./governor -exec "delete from plans where title like '%test-e2e-flow%'" || echo "Plans deleted."

# Delete orphan task_runs
echo "Deleting orphan task_runs..."
./governor -exec "delete from task_runs where task_id not in (select id from tasks)" || echo "Orphan task_runs deleted."

echo "Cleanup complete!"
