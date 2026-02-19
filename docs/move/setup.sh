#!/bin/bash
set -e

echo "=========================================="
echo "VIBEPILOT SETUP"
echo "=========================================="

# Check prerequisites
echo ""
echo "[1/6] Checking prerequisites..."

if ! command -v python3 &> /dev/null; then
    echo "ERROR: python3 not found"
    exit 1
fi

if ! command -v pip &> /dev/null; then
    echo "ERROR: pip not found"
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo "ERROR: git not found"
    exit 1
fi

echo "  python3: $(python3 --version)"
echo "  pip: $(pip --version | cut -d' ' -f1-2)"
echo "  git: $(git --version | cut -d' ' -f3)"

# Check .env exists
echo ""
echo "[2/6] Checking .env file..."

if [ ! -f ".env" ]; then
    echo "ERROR: .env file not found"
    echo ""
    echo "Create it with:"
    echo "  cat > .env << 'EOF'"
    echo "  SUPABASE_URL=https://qtpdzsinvifkgpxyxlaz.supabase.co"
    echo "  SUPABASE_KEY=<your-key>"
    echo "  VAULT_KEY=<your-key>"
    echo "  EOF"
    exit 1
fi

# Check required env vars
source .env
MISSING=0

if [ -z "$SUPABASE_URL" ]; then
    echo "  ERROR: SUPABASE_URL not set"
    MISSING=1
fi

if [ -z "$SUPABASE_KEY" ]; then
    echo "  ERROR: SUPABASE_KEY not set"
    MISSING=1
fi

if [ -z "$VAULT_KEY" ]; then
    echo "  ERROR: VAULT_KEY not set"
    MISSING=1
fi

if [ $MISSING -eq 1 ]; then
    exit 1
fi

echo "  All required variables present"

# Create venv
echo ""
echo "[3/6] Creating Python virtual environment..."

if [ -d "venv" ]; then
    echo "  venv already exists, skipping"
else
    python3 -m venv venv
    echo "  Created venv"
fi

# Install dependencies
echo ""
echo "[4/6] Installing dependencies..."

source venv/bin/activate
pip install --quiet --upgrade pip
pip install --quiet -r requirements.txt
echo "  Dependencies installed"

# Test Supabase connection
echo ""
echo "[5/6] Testing Supabase connection..."

python3 << 'PYEOF'
import os
import sys
from dotenv import load_dotenv

load_dotenv()

try:
    from supabase import create_client
    client = create_client(os.environ['SUPABASE_URL'], os.environ['SUPABASE_KEY'])
    result = client.table('tasks').select('id').limit(1).execute()
    print("  Connection successful")
except Exception as e:
    print(f"  ERROR: {e}")
    sys.exit(1)
PYEOF

# Install systemd service
echo ""
echo "[6/6] Installing systemd service..."

if [ "$EUID" -ne 0 ]; then
    echo "  Need sudo for systemd install..."
fi

sudo cp scripts/vibepilot-orchestrator.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vibepilot-orchestrator
echo "  Service installed and enabled"

# Done
echo ""
echo "=========================================="
echo "SETUP COMPLETE"
echo "=========================================="
echo ""
echo "Start the service:"
echo "  sudo systemctl start vibepilot-orchestrator"
echo ""
echo "Check status:"
echo "  systemctl status vibepilot-orchestrator"
echo ""
echo "View logs:"
echo "  journalctl -u vibepilot-orchestrator -f"
echo ""
