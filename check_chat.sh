#!/bin/bash
# AGENT_CHAT.md Change Notifier
# Usage: ./check_chat.sh [--watch] [--once]
#
# --watch : Monitor continuously (Ctrl+C to stop)
# --once  : Check once and report (default)

CHAT_FILE="$(dirname "$0")/AGENT_CHAT.md"
STATE_FILE="$(dirname "$0")/.chat_last_read"

# Create state file if doesn't exist
if [ ! -f "$STATE_FILE" ]; then
    touch "$STATE_FILE"
fi

# Get current file modification time
get_mod_time() {
    stat -c %Y "$CHAT_FILE" 2>/dev/null || echo "0"
}

# Get last read time
get_last_read() {
    cat "$STATE_FILE" 2>/dev/null || echo "0"
}

# Check for updates
check_once() {
    if [ ! -f "$CHAT_FILE" ]; then
        echo "❌ AGENT_CHAT.md not found"
        return 1
    fi
    
    CURRENT_MOD=$(get_mod_time)
    LAST_READ=$(get_last_read)
    
    if [ "$CURRENT_MOD" -gt "$LAST_READ" ]; then
        # Calculate how long ago
        NOW=$(date +%s)
        AGO=$((NOW - CURRENT_MOD))
        
        if [ $AGO -lt 60 ]; then
            TIME_AGO="${AGO}s ago"
        elif [ $AGO -lt 3600 ]; then
            TIME_AGO="$((AGO / 60))m ago"
        else
            TIME_AGO="$((AGO / 3600))h ago"
        fi
        
        echo "🔔 AGENT_CHAT.md updated ${TIME_AGO}"
        echo ""
        
        # Show last few lines to preview
        echo "--- Latest activity ---"
        tail -20 "$CHAT_FILE" | grep -E "^### |^\*\*From |^\*\*To " | tail -5
        echo ""
        echo "Run: cat AGENT_CHAT.md | tail -50"
        
        return 0
    else
        echo "✓ No new messages"
        return 1
    fi
}

# Mark as read
mark_read() {
    get_mod_time > "$STATE_FILE"
    echo "✓ Marked as read"
}

# Watch mode
watch_mode() {
    echo "👀 Watching AGENT_CHAT.md for changes (Ctrl+C to stop)..."
    echo ""
    
    while true; do
        if check_once; then
            echo ""
            read -p "Mark as read? [Y/n]: " -n 1 -r
            echo ""
            if [[ ! $REPLY =~ ^[Nn]$ ]]; then
                mark_read
            fi
            echo "--- Continuing to watch ---"
            echo ""
        fi
        sleep 30
    done
}

# Main
if [ "$1" == "--watch" ]; then
    watch_mode
elif [ "$1" == "--once" ]; then
    check_once
    if [ $? -eq 0 ]; then
        mark_read
    fi
elif [ "$1" == "--mark-read" ]; then
    mark_read
else
    # Default: check once, don't mark read
    check_once
fi
