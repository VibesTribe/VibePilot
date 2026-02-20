#!/bin/bash
# VibePilot Agent Session Startup
# REQUIRED: Run this at the start of EVERY session
# 
# Usage: ./start_session.sh [agent_name] [--listen]
# Example: ./start_session.sh glm-5 --listen
#
# --listen: Start background message listener (notifications on new messages)

AGENT_NAME="${1:-glm-5}"
LISTEN_MODE="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "═══════════════════════════════════════════════════"
echo "  VIBEPILOT AGENT SESSION STARTUP"
echo "  Agent: $AGENT_NAME"
echo "═══════════════════════════════════════════════════"
echo ""

cd "$SCRIPT_DIR"

if [ -d "venv" ]; then
    source venv/bin/activate
fi

# 1. Git sync
echo "📥 Syncing with main..."
git checkout main 2>/dev/null
git pull origin main 2>/dev/null
echo "✅ Synced"
echo ""

# 2. PRIMARY: Check Supabase messages
echo "📨 Checking Supabase messages..."
python3 scripts/check_agent_mail.py "$AGENT_NAME" 2>/dev/null
echo ""

# 3. SECONDARY: Check AGENT_CHAT.md (for context/history)
echo "💬 Checking AGENT_CHAT.md (for context)..."
if [ -f AGENT_CHAT.md ]; then
    LAST_SECTION=$(grep -n "^### " AGENT_CHAT.md | tail -3)
    if [ -n "$LAST_SECTION" ]; then
        echo "Recent activity:"
        echo "$LAST_SECTION"
    fi
fi
echo ""

# 4. Current status
echo "📊 Current Status:"
echo "   Last commits:"
git log --oneline -3 2>/dev/null
echo ""
if [ -f CURRENT_STATE.md ]; then
    echo "   Session info:"
    head -20 CURRENT_STATE.md | grep -E "(Last Updated|Session|Focus)" || true
fi
echo ""

# 5. Start listener if requested
if [ "$LISTEN_MODE" == "--listen" ]; then
    echo "🎧 Starting message listener (background)..."
    echo "   You will see notifications when messages arrive."
    echo ""
    python3 scripts/listen_for_messages.py "$AGENT_NAME"
else
    echo "💡 TIP: Start with --listen to get real-time message notifications"
    echo "   Example: ./start_session.sh $AGENT_NAME --listen"
    echo ""
fi

# Done
echo "═══════════════════════════════════════════════════"
echo "  READY - You are synced and caught up"
echo "═══════════════════════════════════════════════════"
echo ""
echo "COMMANDS:"
echo "  Check messages:  python3 scripts/check_agent_mail.py $AGENT_NAME"
echo "  Start listener:  python3 scripts/listen_for_messages.py $AGENT_NAME &"
echo "  Notify partner:  ./scripts/notify_done.sh $AGENT_NAME \"task description\""
echo ""
