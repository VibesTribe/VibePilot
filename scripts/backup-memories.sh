#!/bin/bash
# Auto-backup Hermes memories to GitHub
# Called by cron after Hermes memory changes
# Backs up to: agent/HERMES_MEMORIES.md

REPO="$HOME/VibePilot"
HERMES_HOME="$HOME/.hermes"

cd "$REPO" || exit 1

# Read Hermes memory files
MEM_FILE="$HERMES_HOME/memory/memory.md"
USER_FILE="$HERMES_HOME/memory/user.md"
OUT_FILE="$REPO/agent/HERMES_MEMORIES.md"

if [ ! -f "$MEM_FILE" ] && [ ! -f "$USER_FILE" ]; then
    exit 0
fi

mkdir -p "$REPO/agent"

{
    echo "# Hermes Agent Memories"
    echo "# Auto-backed up from Hermes memory store. Do not edit manually."
    echo "# Last updated: $(date -u '+%Y-%m-%d (%H:%M UTC)')"
    echo ""
    echo "## MEMORY"
    echo ""
    if [ -f "$MEM_FILE" ]; then
        cat "$MEM_FILE"
    fi
    echo ""
    echo "## USER PROFILE"
    echo ""
    if [ -f "$USER_FILE" ]; then
        cat "$USER_FILE"
    fi
} > "$OUT_FILE"

# Check if anything changed
CHANGED=$(git diff --name-only "$OUT_FILE" 2>/dev/null)
if [ -n "$CHANGED" ]; then
    git add "$OUT_FILE"
    git commit -m "chore: auto-backup Hermes memories to GitHub" --quiet
    git push --quiet
fi
