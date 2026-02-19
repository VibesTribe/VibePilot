#!/bin/bash
# VibePilot Agent Session Startup
# REQUIRED: Run this at the start of EVERY session

echo "═══════════════════════════════════════════════════"
echo "  VIBEPILOT AGENT SESSION STARTUP"
echo "═══════════════════════════════════════════════════"
echo ""

cd "$(dirname "$0")"

# 1. Git sync
echo "📥 Syncing with main..."
git checkout main 2>/dev/null
git pull origin main
if [ $? -ne 0 ]; then
    echo "❌ Git pull failed! Check manually."
    exit 1
fi
echo "✅ Synced"
echo ""

# 2. Check protocol (in case it changed)
echo "📋 Checking protocol..."
if [ -f AGENT_PROTOCOL.md ]; then
    echo "✅ AGENT_PROTOCOL.md present"
else
    echo "⚠️  AGENT_PROTOCOL.md missing (shouldn't happen)"
fi
echo ""

# 3. PRIMARY: Check AGENT_CHAT.md
echo "💬 Checking AGENT_CHAT.md (PRIMARY)..."
./check_chat.sh
CHAT_STATUS=$?

if [ $CHAT_STATUS -eq 0 ]; then
    echo ""
    echo "🔔 NEW MESSAGES - READ THEM!"
    echo "Run: cat AGENT_CHAT.md | tail -100"
    echo ""
    read -p "Press Enter after reading..."
    ./check_chat.sh --once  # Mark as read
else
    echo "✅ No new messages"
fi
echo ""

# 4. SECONDARY: Check inbox
echo "📨 Checking inbox/kimi/ (if exists)..."
if [ -d inbox/kimi ] && [ "$(ls -A inbox/kimi 2>/dev/null)" ]; then
    echo "🔔 INBOX ITEMS FOUND:"
    ls -la inbox/kimi/
    echo ""
    echo "Read them:"
    for f in inbox/kimi/*; do
        echo "--- $f ---"
        head -20 "$f"
        echo ""
    done
    read -p "Press Enter after reading inbox..."
else
    echo "✅ No inbox items"
fi
echo ""

# 5. TERTIARY: Check handoff
echo "📄 Checking .handoff-to-kimi.md (if exists and new)..."
if [ -f .handoff-to-kimi.md ]; then
    echo "📝 Handoff file found:"
    stat -c "%y" .handoff-to-kimi.md
    echo ""
    read -p "Read handoff file? [Y/n]: " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        cat .handoff-to-kimi.md
        echo ""
        read -p "Press Enter to continue..."
    fi
else
    echo "✅ No handoff file"
fi
echo ""

# Done
echo "═══════════════════════════════════════════════════"
echo "  SESSION STARTUP COMPLETE"
echo "═══════════════════════════════════════════════════"
echo ""
echo "You are now synced and caught up."
echo "Remember: Check AGENT_CHAT.md regularly during session."
echo ""

# Show current status
echo "--- Current Session Status ---"
git log --oneline -3
echo ""
echo "Active agents:"
cat ACTIVE_SESSIONS.md | grep -E "^\| (kimi|glm)" || echo "(Check ACTIVE_SESSIONS.md)"
