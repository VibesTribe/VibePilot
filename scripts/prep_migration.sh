#!/bin/bash
# VibePilot Migration Prep Script
# Run this before moving to a new host
# Ensures nothing is lost

set -e

echo "=== VibePilot Migration Prep ==="
echo ""

# Check for uncommitted changes
echo "[1/7] Checking git status..."
if ! git diff-index --quiet HEAD --; then
    echo "ERROR: Uncommitted changes detected!"
    echo "Please commit or stash before migration."
    git status --short
    exit 1
fi
echo "✓ All changes committed"

# Check for untracked files
echo ""
echo "[2/7] Checking for untracked files..."
UNTRACKED=$(git ls-files --others --exclude-standard)
if [ -n "$UNTRACKED" ]; then
    echo "WARNING: Untracked files found:"
    echo "$UNTRACKED"
    echo ""
    read -p "Add and commit these files? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git add .
        git commit -m "Pre-migration: add untracked files"
        echo "✓ Untracked files committed"
    else
        echo "ERROR: Untracked files not committed. Migration aborted."
        exit 1
    fi
else
    echo "✓ No untracked files"
fi

# Push to GitHub
echo ""
echo "[3/7] Pushing to GitHub..."
git push origin main
echo "✓ Pushed to GitHub"

# Check .env.example exists and is complete
echo ""
echo "[4/7] Checking .env.example..."
if [ ! -f ".env.example" ]; then
    echo "ERROR: .env.example not found!"
    echo "Create it with all required environment variables."
    exit 1
fi

# Compare .env and .env.example
if [ -f ".env" ]; then
    ENV_KEYS=$(grep -o '^[A-Za-z_][A-Za-z0-9_]*' .env | sort)
    EXAMPLE_KEYS=$(grep -o '^[A-Za-z_][A-Za-z0-9_]*' .env.example | sort)
    MISSING=$(comm -23 <(echo "$ENV_KEYS") <(echo "$EXAMPLE_KEYS"))
    if [ -n "$MISSING" ]; then
        echo "ERROR: Variables in .env but not in .env.example:"
        echo "$MISSING"
        exit 1
    fi
fi
echo "✓ .env.example is complete"

# Test setup.sh
echo ""
echo "[5/7] Checking setup.sh..."
if [ ! -f "setup.sh" ]; then
    echo "ERROR: setup.sh not found!"
    echo "Create it to enable one-command setup on new host."
    exit 1
fi
if [ ! -x "setup.sh" ]; then
    chmod +x setup.sh
fi
echo "✓ setup.sh exists and is executable"

# Export Supabase data
echo ""
echo "[6/7] Exporting Supabase data..."
EXPORT_FILE="migration/supabase_export_$(date +%Y%m%d_%H%M%S).sql"
mkdir -p migration
echo "Run this manually: supabase db dump -f $EXPORT_FILE"
echo "Or use Supabase dashboard: Database > Backups > Create backup"
echo "✓ Export path: $EXPORT_FILE (manual step)"

# Update SESSION_LOG
echo ""
echo "[7/7] Updating SESSION_LOG.md..."
cat >> docs/SESSION_LOG.md << EOF

---

## Migration Prep: $(date +%Y-%m-%d)

- Git status: Clean, all committed
- GitHub: Synced to origin/main
- Environment: .env.example complete
- Setup: setup.sh ready
- Supabase: Export pending (manual)

### Next Steps on New Host
1. git clone git@github.com:VibesTribe/VibePilot.git
2. cd VibePilot
3. cp .env.example .env
4. Edit .env with credentials
5. ./setup.sh
6. Verify Supabase connection
7. Resume from SESSION_LOG.md "Next Steps"

EOF
echo "✓ SESSION_LOG.md updated"

# Summary
echo ""
echo "=== Migration Prep Complete ==="
echo ""
echo "Checklist before moving:"
echo "  [ ] Supabase data exported"
echo "  [ ] .env credentials accessible (not just on this machine)"
echo "  [ ] Document any GCE-specific configs to remove"
echo ""
echo "On new host:"
echo "  1. git clone git@github.com:VibesTribe/VibePilot.git"
echo "  2. cd VibePilot && cp .env.example .env"
echo "  3. Edit .env with credentials"
echo "  4. ./setup.sh"
echo "  5. cat docs/SESSION_LOG.md"
echo ""
