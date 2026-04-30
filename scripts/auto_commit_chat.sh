#!/bin/bash
# Auto-commit AGENT_CHAT.md to prevent data loss on crash
# Run via cron every 2 minutes
# Logs to ~/.vibepilot_chat_commits.log

REPO_DIR="$(dirname "$0")/.."
CHAT_FILE="$REPO_DIR/AGENT_CHAT.md"
LOG_FILE="$HOME/.vibepilot_chat_commits.log"
LOCK_FILE="/tmp/vibepilot_chat_commit.lock"

# Prevent concurrent runs
if [ -f "$LOCK_FILE" ]; then
    PID=$(cat "$LOCK_FILE")
    if ps -p "$PID" > /dev/null 2>&1; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Another instance running, skipping" >> "$LOG_FILE"
        exit 0
    fi
fi
echo $$ > "$LOCK_FILE"

# Cleanup lock on exit
trap "rm -f $LOCK_FILE" EXIT

cd "$REPO_DIR" || exit 1

# Check if there are uncommitted changes to AGENT_CHAT.md
if git diff --quiet HEAD -- AGENT_CHAT.md 2>/dev/null; then
    # No changes
    exit 0
fi

# There are changes - commit them
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
SHORT_MSG=$(git diff AGENT_CHAT.md | grep -E "^\+### " | head -1 | sed 's/^+### //' || echo "Update")

git add AGENT_CHAT.md
git commit -m "Auto-commit chat: $SHORT_MSG [$TIMESTAMP]" >> "$LOG_FILE" 2>&1

if [ $? -eq 0 ]; then
    git push origin main >> "$LOG_FILE" 2>&1
    if [ $? -eq 0 ]; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Committed and pushed: $SHORT_MSG" >> "$LOG_FILE"
    else
        echo "$(date '+%Y-%m-%d %H:%M:%S') - ERROR: Commit succeeded but push failed" >> "$LOG_FILE"
    fi
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - ERROR: Commit failed" >> "$LOG_FILE"
fi
