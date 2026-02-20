#!/bin/bash
# Poll for GLM-5 messages in AGENT_CHAT.md
# Usage: ./poll_for_glm.sh [interval_seconds]
# Default: check every 30 seconds

INTERVAL=${1:-30}
CHAT_FILE="/home/mjlockboxsocial/vibepilot/AGENT_CHAT.md"
LAST_CHECK_FILE="/tmp/last_glm_check"
LOG_FILE="/tmp/glm_poll.log"

echo "$(date): Starting GLM-5 message polling (every ${INTERVAL}s)" | tee -a "$LOG_FILE"
echo "Press Ctrl+C to stop"
echo ""

# Initialize last check time
if [ ! -f "$LAST_CHECK_FILE" ]; then
    touch "$LAST_CHECK_FILE"
fi

# Function to check for new GLM messages
check_for_glm() {
    local last_check=$(stat -c %Y "$LAST_CHECK_FILE" 2>/dev/null || echo "0")
    local chat_mtime=$(stat -c %Y "$CHAT_FILE" 2>/dev/null || echo "0")
    
    if [ "$chat_mtime" -gt "$last_check" ]; then
        # File changed, check for GLM-5 messages
        local new_messages=$(tail -100 "$CHAT_FILE" | grep -E "^### GLM-5\s*\[" | tail -5)
        
        if [ -n "$new_messages" ]; then
            echo ""
            echo "╔══════════════════════════════════════════════════════════════╗"
            echo "║  🔔 NEW GLM-5 MESSAGE DETECTED!                              ║"
            echo "╠══════════════════════════════════════════════════════════════╣"
            echo "$new_messages" | while read line; do
                echo "║  $line"
            done
            echo "╚══════════════════════════════════════════════════════════════╝"
            echo ""
            echo "Last 3 messages from GLM-5:"
            echo "---"
            tail -100 "$CHAT_FILE" | grep -A 50 "^### GLM-5" | tail -80 | head -60
            echo "---"
            echo ""
            echo "$(date): NEW GLM-5 message found!" | tee -a "$LOG_FILE"
            
            # Update last check
            touch "$LAST_CHECK_FILE"
            
            return 0
        fi
    fi
    
    return 1
}

# Function to show recent activity
show_status() {
    echo "$(date): Checking... (press Ctrl+C to stop)" | tee -a "$LOG_FILE"
    
    # Show current git status
    cd /home/mjlockboxsocial/vibepilot
    local last_commit=$(git log --oneline -1 | cut -d' ' -f1)
    local uncommitted=$(git status --porcelain | wc -l)
    
    echo "  Last commit: $last_commit | Uncommitted: $uncommitted"
}

# Main polling loop
counter=0
while true; do
    if check_for_glm; then
        # New message found - keep alerting every interval until acknowledged
        echo "⚠️  Waiting for you to acknowledge (GLM-5 message waiting)..."
    fi
    
    # Show status every 10 checks (5 minutes if 30s interval)
    counter=$((counter + 1))
    if [ $counter -ge 10 ]; then
        show_status
        counter=0
    fi
    
    sleep "$INTERVAL"
done
