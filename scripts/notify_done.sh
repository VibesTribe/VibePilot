#!/bin/bash
# Notify other agent that work is complete
# Usage: ./scripts/notify_done.sh [from_agent] [task_summary]
# Example: ./scripts/notify_done.sh glm-5 "Fixed orchestrator wiring"

FROM_AGENT="${1:-glm-5}"
TO_AGENT="kimi"
if [ "$FROM_AGENT" = "kimi" ]; then
    TO_AGENT="glm-5"
fi

TASK_SUMMARY="${2:-Task completed}"

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"

if [ -d "venv" ]; then
    source venv/bin/activate
fi

# Get latest commit for context
LATEST_COMMIT=$(git log --oneline -1 2>/dev/null || echo "N/A")

python3 -c "
from core.agent_comm import AgentComm
comm = AgentComm('$FROM_AGENT')
comm.send('$TO_AGENT', {
    'text': '''Task completed by $FROM_AGENT

Summary: $TASK_SUMMARY
Commit: $LATEST_COMMIT

Check CURRENT_STATE.md for updates.''',
    'action': 'task_done'
}, message_type='task')
print('✅ Notified $TO_AGENT')
"
