#!/bin/bash
# Start AI agent in persistent tmux session
# Usage: ./start_agent_session.sh [kimi|opencode|glm]

AGENT=${1:-kimi}
SESSION_NAME="vibepilot-${AGENT}"

case $AGENT in
    kimi)
        CMD="kimi"
        ;;
    opencode|glm)
        CMD="opencode"
        ;;
    *)
        echo "Usage: $0 [kimi|opencode|glm]"
        exit 1
        ;;
esac

# Check if session exists
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo "Session '$SESSION_NAME' already exists."
    echo "Attach with: tmux attach -t $SESSION_NAME"
    echo "Or force new: tmux new-session -t $SESSION_NAME"
    
    # Auto-attach
    read -p "Attach to existing session? [Y/n] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        tmux attach -t "$SESSION_NAME"
    fi
else
    echo "Creating new tmux session: $SESSION_NAME"
    cd ~/vibepilot
    
    # Create session with the command
    tmux new-session -d -s "$SESSION_NAME" -n "main"
    
    # Setup environment
    tmux send-keys -t "$SESSION_NAME" "cd ~/vibepilot && source venv/bin/activate 2>/dev/null || true" Enter
    tmux send-keys -t "$SESSION_NAME" "export PS1='[${AGENT}] \w\$ '" Enter
    tmux send-keys -t "$SESSION_NAME" "$CMD" Enter
    
    # Attach to it
    tmux attach -t "$SESSION_NAME"
fi
