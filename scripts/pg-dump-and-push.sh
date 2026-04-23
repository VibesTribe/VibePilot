#!/bin/bash
# VibePilot PG Dump & Push to GitHub
# Run via cron: every hour (0 * * * *)
set -e

REPO_DIR="/home/vibes/data/vibepilot-data"
DB_HOST="localhost"
DB_USER="vibes"
DB_NAME="vibepilot"
DB_PASS="vibepilot"
BRANCH="main"

# Clone or pull
if [ -d "$REPO_DIR" ]; then
    cd "$REPO_DIR"
    git pull --rebase origin "$BRANCH" 2>/dev/null || true
else
    git clone https://github.com/VibesTribe/knowledgebase.git "$REPO_DIR"
    cd "$REPO_DIR"
fi

# Dump (SQL only — human-readable, git-diffable, sufficient for restore)
mkdir -p pg-dump
PGPASSWORD="$DB_PASS" pg_dump -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" --no-owner --no-privileges | grep -v '^-- Dump completed on' | grep -v '^\\restrict' | grep -v '^\\unrestrict' > "pg-dump/vibepilot.sql.tmp"
mv "pg-dump/vibepilot.sql.tmp" "pg-dump/vibepilot.sql"

# Remove binary dump if it exists from old runs
rm -f pg-dump/vibepilot.dump

# Commit and push (only if data changed)
git add pg-dump/
TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M UTC")
if git diff --cached --quiet; then
    echo "[$(date)] No changes since last dump"
else
    git commit -m "PG backup: $TIMESTAMP"
    git push origin "$BRANCH"
    echo "[$(date)] PG dump pushed to GitHub"
fi
