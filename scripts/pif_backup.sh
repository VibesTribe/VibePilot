#!/bin/bash
# ============================================================================
# pif_backup.sh — PIF Phase F: Automated per-project backup
# ============================================================================
# Git-commits the entire project directory to its private backup repo.
# Runs on a schedule (cron). Each project has its own backup repo.
#
# Usage:
#   ./pif_backup.sh sealed           # backup a specific project
#   ./pif_backup.sh --all            # backup all projects
#
# Spec: vibepilot/docs/PROJECT_ISOLATION_FRAMEWORK.md, Section 3.4
# Guardrails 5, 6, 9, 11
# ============================================================================

set -euo pipefail

PROJECTS_BASE="$HOME/projects"
BACKUP_CLONE_BASE="$HOME/.pif-backups"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

backup_project() {
    local slug="$1"
    local project_dir="$PROJECTS_BASE/$slug"

    if [ ! -d "$project_dir" ]; then
        echo -e "${RED}ERROR:${NC} Project directory not found: $project_dir"
        return 1
    fi

    # Read backup repo from vibepilot.toml
    local backup_repo=""
    if [ -f "$project_dir/vibepilot.toml" ]; then
        backup_repo=$(grep -A5 '\[backup\]' "$project_dir/vibepilot.toml" | grep 'repo' | head -1 | sed 's/.*= *"\(.*\)".*/\1/')
    fi

    if [ -z "$backup_repo" ]; then
        echo -e "${YELLOW}SKIP:${NC} No backup repo configured for $slug"
        return 0
    fi

    local backup_url="https://github.com/$backup_repo.git"
    local clone_dir="$BACKUP_CLONE_BASE/$slug-backup"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

    echo -e "${GREEN}==> Backing up:$NC $slug → $backup_repo"

    # Clone or pull the backup repo
    mkdir -p "$BACKUP_CLONE_BASE"
    if [ -d "$clone_dir/.git" ]; then
        cd "$clone_dir"
        git pull --quiet origin main 2>/dev/null || git pull --quiet origin master 2>/dev/null || true
    else
        git clone --quiet "$backup_url" "$clone_dir" 2>/dev/null || {
            echo -e "${RED}ERROR:${NC} Cannot clone $backup_url"
            return 1
        }
        cd "$clone_dir"
    fi

    # Rsync project files into backup dir
    # Exclude: .git, node_modules, logs, backups (avoid recursion)
    rsync -a --delete \
        --exclude='.git' \
        --exclude='node_modules' \
        --exclude='__pycache__' \
        --exclude='*.pyc' \
        --exclude='logs/' \
        --exclude='backups/' \
        --exclude='.DS_Store' \
        "$project_dir/" "$clone_dir/"

    # Stage and commit
    git add -A
    if git diff --cached --quiet; then
        echo "    No changes since last backup."
        return 0
    fi

    git commit -m "Automated backup: $slug at $timestamp

Source: $project_dir
Files: $(find "$clone_dir" -type f -not -path '*/.git/*' | wc -l) files
Audit: $(sqlite3 "$project_dir/database/$slug.db" 'SELECT count(*) FROM audit_log' 2>/dev/null || echo '?') entries"

    # Push to backup repo
    if git push --quiet origin main 2>/dev/null || git push --quiet origin master 2>/dev/null; then
        echo -e "    ${GREEN}Pushed to $backup_repo${NC}"
    else
        echo -e "    ${YELLOW}WARNING:${NC} Push failed (network or auth issue). Commit saved locally."
    fi
}

# ============================================================================
# Main
# ============================================================================

if [ -z "${1:-}" ]; then
    echo "Usage: $0 <slug> | --all"
    exit 1
fi

if [ "$1" = "--all" ]; then
    # Backup all projects that have directories
    for dir in "$PROJECTS_BASE"/*/; do
        slug=$(basename "$dir")
        [ "$slug" = "vibepilot" ] && continue  # vibepilot has its own backup system
        backup_project "$slug" || true
    done
else
    backup_project "$1"
fi

echo ""
echo "Backup complete."
