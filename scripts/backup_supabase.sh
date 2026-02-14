#!/bin/bash
# VibePilot Supabase Backup Script
# Run daily to ensure data safety

set -e

# Configuration
BACKUP_DIR="backups"
RETENTION_DAYS=30
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/vibepilot_${TIMESTAMP}.sql"

echo "VibePilot Supabase Backup"
echo "========================="
echo ""

# Create backup directory
mkdir -p $BACKUP_DIR

# Load environment
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
else
    echo "ERROR: .env not found"
    exit 1
fi

# Check for supabase CLI
if ! command -v supabase &> /dev/null; then
    echo "Supabase CLI not found."
    echo ""
    echo "Option 1: Install Supabase CLI"
    echo "  See: https://supabase.com/docs/guides/cli"
    echo ""
    echo "Option 2: Manual backup via dashboard"
    echo "  1. Go to https://supabase.com/dashboard"
    echo "  2. Select your project"
    echo "  3. Database → Backups → Create backup"
    echo ""
    echo "Option 3: Use pg_dump directly"
    echo "  pg_dump \$SUPABASE_URL > $BACKUP_FILE"
    echo ""
    exit 1
fi

# Run backup
echo "Creating backup..."
supabase db dump -f $BACKUP_FILE

if [ -f "$BACKUP_FILE" ]; then
    SIZE=$(du -h $BACKUP_FILE | cut -f1)
    echo "✓ Backup created: $BACKUP_FILE ($SIZE)"
else
    echo "ERROR: Backup failed"
    exit 1
fi

# Clean old backups
echo ""
echo "Cleaning backups older than $RETENTION_DAYS days..."
find $BACKUP_DIR -name "vibepilot_*.sql" -mtime +$RETENTION_DAYS -delete
echo "✓ Cleanup complete"

# List current backups
echo ""
echo "Current backups:"
ls -lh $BACKUP_DIR/vibepilot_*.sql 2>/dev/null | tail -5

echo ""
echo "Backup complete!"
