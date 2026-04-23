#!/bin/bash
# Hermes Memory Sync — dumps Hermes agent memory to vibes-agent-context repo
# Run via Hermes cron job (not system cron) so it can access the memory tool
# 
# This script does the git push AFTER Hermes has written the files.
# The actual memory dump is done by the Hermes cron prompt below.

set -e

REPO_DIR="$HOME/data/vibes-agent-context"
LOG="/tmp/hermes-memory-sync.log"

echo "$(date -Iseconds) Starting memory sync" >> "$LOG"

cd "$REPO_DIR"

# Pull any remote changes first
git pull --rebase origin main >> "$LOG" 2>&1 || true

# Check if there are changes
if git diff --quiet && git diff --cached --quiet && [ -z "$(git ls-files --others --exclude-standard)" ]; then
    echo "$(date -Iseconds) No changes to sync" >> "$LOG"
    exit 0
fi

# Stage, commit, push
git add -A >> "$LOG" 2>&1
git commit -m "memory sync $(date -u +%Y-%m-%dT%H:%MZ)" >> "$LOG" 2>&1
git push origin main >> "$LOG" 2>&1

echo "$(date -Iseconds) Memory sync complete" >> "$LOG"
