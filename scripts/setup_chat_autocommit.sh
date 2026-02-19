#!/bin/bash
# Setup auto-commit cron job for AGENT_CHAT.md
# Prevents data loss if terminal crashes mid-conversation

echo "Setting up auto-commit for AGENT_CHAT.md..."

# Get the full path to the auto-commit script
SCRIPT_PATH="$(cd "$(dirname "$0")" && pwd)/auto_commit_chat.sh"

# Check if already in crontab
if crontab -l 2>/dev/null | grep -q "auto_commit_chat.sh"; then
    echo "✓ Auto-commit already configured"
    echo ""
    echo "Current schedule:"
    crontab -l | grep "auto_commit_chat.sh"
    exit 0
fi

# Add to crontab - run every 2 minutes
(crontab -l 2>/dev/null; echo "*/2 * * * * $SCRIPT_PATH >> /tmp/vibepilot_cron.log 2>&1") | crontab -

if [ $? -eq 0 ]; then
    echo "✓ Auto-commit configured successfully!"
    echo ""
    echo "Schedule: Every 2 minutes"
    echo "Script: $SCRIPT_PATH"
    echo "Log: ~/.vibepilot_chat_commits.log"
    echo ""
    echo "To verify: crontab -l"
    echo "To remove: crontab -e (delete the line)"
else
    echo "❌ Failed to configure cron job"
    echo "You may need to set it up manually:"
    echo "  crontab -e"
    echo "  Add: */2 * * * * $SCRIPT_PATH"
fi
