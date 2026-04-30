#!/bin/bash
# VibePilot Branch Cleanup Script
# Removes stale task branches and prunes remote references

set -e

REPO_PATH="${1:-$HOME/vibepilot}"
FORCE="${2:-false}"

echo "=== VibePilot Branch Cleanup ==="
echo "Repository: $REPO_PATH"
echo ""

cd "$REPO_PATH"

# Check if on a task branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ $CURRENT_BRANCH =~ ^task/ ]]; then
  echo "⚠️  You are on task branch: $CURRENT_BRANCH"
  echo "Switching to main branch..."
  git checkout main
  echo ""
fi

# Count task branches before
TASK_BRANCHES_BEFORE=$(git branch | grep "^  task/" | wc -l)
echo "Found $TASK_BRANCHES_BEFORE local task branches"

if [ "$TASK_BRANCHES_BEFORE" -eq 0 ]; then
  echo "✅ No task branches to clean"
  exit 0
fi

# List task branches
echo ""
echo "Task branches to delete:"
git branch | grep "^  task/" | sed 's/^/  /'

# Confirm unless forced
if [ "$FORCE" != "true" ]; then
  echo ""
  read -p "Delete these branches? (y/N): " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 1
  fi
fi

# Delete local task branches
echo ""
echo "Deleting local task branches..."
DELETED=0
for branch in $(git branch | grep "^  task/" | sed 's/^[ \t]*//'); do
  if git branch -D "$branch" 2>/dev/null; then
    echo "  ✓ Deleted: $branch"
    ((DELETED++))
  else
    echo "  ✗ Failed: $branch (may be already deleted)"
  fi
done

# Prune remote references
echo ""
echo "Pruning remote references..."
git remote prune origin

# Show remaining branches
echo ""
echo "=== Summary ==="
echo "Deleted: $DELETED branches"
echo ""
echo "Remaining branches:"
git branch | sed 's/^/  /'

# Show module branches
echo ""
echo "Module branches:"
git branch | grep -E "^  TEST_MODULES" | sed 's/^/  /' || echo "  None"

echo ""
echo "✅ Cleanup complete"
