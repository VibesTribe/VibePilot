#!/bin/bash
# Manage AI agent tmux sessions
# Usage: ./agent_sessions.sh [status|attach kimi|attach opencode|kill-all]

ACTION=${1:-status}
TARGET=${2:-}

case $ACTION in
    status|ls|list)
        echo "=== Active VibePilot Agent Sessions ==="
        tmux list-sessions 2>/dev/null | grep vibepilot || echo "No active sessions"
        echo ""
        echo "=== System Resources ==="
        free -h | grep -E "(Mem|Swap)"
        echo ""
        echo "=== AI Process Count ==="
        echo "opencode: $(ps aux | grep opencode | grep -v grep | wc -l) processes"
        echo "kimi: $(ps aux | grep 'kimi' | grep -v grep | grep -v grep | wc -l) processes"
        ;;
    
    attach|a)
        if [ -z "$TARGET" ]; then
            echo "Usage: $0 attach [kimi|opencode]"
            echo "Available sessions:"
            tmux list-sessions 2>/dev/null | grep vibepilot || echo "None"
            exit 1
        fi
        SESSION="vibepilot-${TARGET}"
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            echo "Attaching to $SESSION..."
            echo "Detach with: Ctrl+B, then D"
            tmux attach -t "$SESSION"
        else
            echo "Session '$SESSION' not found."
            echo "Start with: ./start_agent_session.sh $TARGET"
        fi
        ;;
    
    kill-all|cleanup)
        echo "Cleaning up all zombie processes..."
        ~/vibepilot/scripts/cleanup_zombies.sh
        echo ""
        echo "Killing orphaned tmux sessions..."
        tmux list-sessions 2>/dev/null | grep vibepilot | cut -d: -f1 | while read session; do
            # Check if anyone is attached
            attached=$(tmux list-clients -t "$session" 2>/dev/null | wc -l)
            if [ "$attached" -eq 0 ]; then
                echo "Killing detached session: $session"
                tmux kill-session -t "$session" 2>/dev/null
            fi
        done
        echo "Done."
        ;;
    
    new|start)
        if [ -z "$TARGET" ]; then
            echo "Usage: $0 start [kimi|opencode]"
            exit 1
        fi
        ~/vibepilot/scripts/start_agent_session.sh "$TARGET"
        ;;
    
    *)
        echo "VibePilot Agent Session Manager"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  status              Show all sessions and resources"
        echo "  attach [kimi|opencode]  Attach to agent session"
        echo "  start [kimi|opencode]   Start new agent session"
        echo "  kill-all            Cleanup zombies and detached sessions"
        echo ""
        echo "Quick start:"
        echo "  $0 start kimi       # Start Kimi in tmux"
        echo "  $0 attach opencode  # Attach to OpenCode (GLM)"
        echo ""
        echo "tmux shortcuts (once attached):"
        echo "  Ctrl+B D            Detach (session keeps running)"
        echo "  Ctrl+B C            New window"
        echo "  Ctrl+B N            Next window"
        echo "  Ctrl+B [            Scroll mode (q to exit)"
        ;;
esac
