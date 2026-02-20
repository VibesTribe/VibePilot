#!/bin/bash
# VibePilot Agent Chat Alert
# Add to ~/.bashrc: source ~/vibepilot/check_chat_alert.sh

VIBEPILOT_DIR="${HOME}/vibepilot"
CHAT_FILE="${VIBEPILOT_DIR}/AGENT_CHAT.md"
LAST_CHECK_FILE="${HOME}/.vibepilot_last_chat_check"
LAST_CHECKSUM_FILE="${HOME}/.vibepilot_last_chat_checksum"

# Function to check for new messages
check_agent_chat() {
    if [ ! -f "$CHAT_FILE" ]; then
        return 0
    fi
    
    # Calculate current checksum
    local current_checksum=$(md5sum "$CHAT_FILE" | awk '{print $1}')
    
    # If no previous checksum, store and return
    if [ ! -f "$LAST_CHECKSUM_FILE" ]; then
        echo "$current_checksum" > "$LAST_CHECKSUM_FILE"
        date +%s > "$LAST_CHECK_FILE_FILE"
        return 0
    fi
    
    local last_checksum=$(cat "$LAST_CHECKSUM_FILE" 2>/dev/null || echo "")
    
    # If changed, show alert
    if [ "$current_checksum" != "$last_checksum" ]; then
        echo ""
        echo -e "\033[1;33m╔════════════════════════════════════════════════╗\033[0m"
        echo -e "\033[1;33m║  📨 NEW MESSAGE IN AGENT_CHAT.md               ║\033[0m"
        echo -e "\033[1;33m╚════════════════════════════════════════════════╝\033[0m"
        echo ""
        
        # Show last 3 message headers
        local new_messages=$(grep -E "^### (Kimi|GLM-5|HUMAN)" "$CHAT_FILE" | tail -3)
        if [ -n "$new_messages" ]; then
            echo -e "\033[36mRecent activity:\033[0m"
            echo "$new_messages" | while read line; do
                echo -e "  \033[90m${line:0:60}...\033[0m"
            done
            echo ""
        fi
        
        echo -e "\033[33m  Read: tail -100 ${VIBEPILOT_DIR}/AGENT_CHAT.md\033[0m"
        echo -e "\033[33m  Or:   cd ${VIBEPILOT_DIR} && git pull && tail -50 AGENT_CHAT.md\033[0m"
        echo ""
        
        # Update stored checksum
        echo "$current_checksum" > "$LAST_CHECKSUM_FILE"
        date +%s > "$LAST_CHECK_FILE"
        
        # Optional: bell sound
        printf '\a'
    fi
}

# Run check on shell startup if interactive
if [[ $- == *i* ]]; then
    check_agent_chat
fi

# Also provide manual check command
alias checkchat='check_agent_chat'
alias chat='tail -100 "${VIBEPILOT_DIR}/AGENT_CHAT.md"'
alias chatnew='cd "${VIBEPILOT_DIR}" && git pull && check_agent_chat && tail -50 AGENT_CHAT.md'
