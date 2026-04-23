#!/bin/bash
# Restore VibePilot PostgreSQL from GitHub backup
# Usage: git clone https://github.com/VibesTribe/knowledgebase.git /tmp/kb-restore && bash /path/to/restore-from-github.sh /tmp/kb-restore
set -e

DUMP_DIR="${1:-.}/pg-dump"
DUMP_FILE="$DUMP_DIR/vibepilot.sql"

echo "=== Restoring VibePilot PostgreSQL ==="

# Check if PG is running
if ! pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo "ERROR: PostgreSQL not running on localhost:5432"
    echo "Start it first: sudo systemctl start postgresql"
    exit 1
fi

# Check if dump exists
if [ ! -f "$DUMP_FILE" ]; then
    echo "ERROR: SQL dump not found at $DUMP_FILE"
    echo "Clone the data repo first: git clone https://github.com/VibesTribe/knowledgebase.git"
    exit 1
fi

# Create database and user if they don't exist
echo "Ensuring database and user exist..."
PGPASSWORD=vibepilot psql -h localhost -U vibes -d vibepilot -c "SELECT 1" > /dev/null 2>&1 || {
    echo "L0g0n" | sudo -S -u postgres psql -c "CREATE USER vibes WITH PASSWORD 'vibepilot';" 2>/dev/null || true
    echo "L0g0n" | sudo -S -u postgres psql -c "CREATE DATABASE vibepilot OWNER vibes;" 2>/dev/null || true
    echo "L0g0n" | sudo -S -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE vibepilot TO vibes;" 2>/dev/null || true
}

# Restore from SQL dump
echo "Restoring from $DUMP_FILE ..."
PGPASSWORD=vibepilot psql -h localhost -U vibes -d vibepilot -f "$DUMP_FILE" 2>&1 | tail -5

echo ""
echo "=== Restore complete ==="
echo "Verify: curl -s http://localhost:8080/api/dashboard | python3 -m json.tool | head -20"
