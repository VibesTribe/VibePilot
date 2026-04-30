#!/bin/bash
# Auto-rebuild and commit .context/ to GitHub
# Run via cron or manually
# This ensures .context/ stays fresh even if you forget to rebuild

REPO_ROOT="$HOME/VibePilot"
cd "$REPO_ROOT" || exit 1

# Rebuild
bash .context/build.sh 2>&1 | grep -E "^\[\.context\]"

# Check if anything changed
CHANGED=$(git diff --name-only .context/boot.md .context/map.md .context/meta.yaml 2>/dev/null)
if [ -n "$CHANGED" ]; then
    COMMIT=$(git rev-parse --short HEAD)
    git add .context/boot.md .context/map.md .context/meta.yaml
    git commit -m "chore: auto-update .context knowledge layer (from $COMMIT)" --quiet
    git push --quiet
    echo "[.context-sync] Pushed updates to GitHub"
else
    echo "[.context-sync] No changes to push"
fi
