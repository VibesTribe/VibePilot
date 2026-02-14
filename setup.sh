#!/bin/bash
# VibePilot One-Command Setup
# Run this on a fresh machine to get up and running

set -e  # Exit on error

echo "========================================"
echo "VibePilot Setup"
echo "========================================"
echo ""

# Check for required tools
echo "[1/6] Checking prerequisites..."

check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "ERROR: $1 is not installed"
        echo "Please install: $2"
        exit 1
    fi
}

check_command python3 "sudo apt install python3"
check_command pip3 "sudo apt install python3-pip"
check_command git "sudo apt install git"
check_command curl "sudo apt install curl"

echo "✓ All prerequisites installed"
echo ""

# Check for .env
echo "[2/6] Checking environment configuration..."

if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        echo "ERROR: .env file not found"
        echo ""
        echo "Please run:"
        echo "  cp .env.example .env"
        echo "  # Edit .env with your credentials"
        echo ""
        echo "Then run ./setup.sh again"
        exit 1
    else
        echo "ERROR: .env.example not found"
        echo "Are you in the vibepilot directory?"
        exit 1
    fi
fi

# Verify .env has required variables
REQUIRED_VARS=("SUPABASE_URL" "SUPABASE_KEY" "GLM_API_KEY" "GITHUB_TOKEN")
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if ! grep -q "^${var}=" .env; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    echo "ERROR: Missing required environment variables:"
    for var in "${MISSING_VARS[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Please add them to .env"
    exit 1
fi

echo "✓ Environment configured"
echo ""

# Create virtual environment
echo "[3/6] Creating Python virtual environment..."

if [ -d "venv" ]; then
    echo "venv/ already exists, skipping..."
else
    python3 -m venv venv
    echo "✓ Virtual environment created"
fi
echo ""

# Install dependencies
echo "[4/6] Installing Python dependencies..."

source venv/bin/activate

if [ -f "requirements.txt" ]; then
    pip install --upgrade pip --quiet
    pip install -r requirements.txt --quiet
    echo "✓ Dependencies installed"
else
    echo "WARNING: requirements.txt not found, skipping"
fi
echo ""

# Verify Supabase connection
echo "[5/6] Testing Supabase connection..."

python3 -c "
import os
from dotenv import load_dotenv
load_dotenv()

url = os.getenv('SUPABASE_URL')
key = os.getenv('SUPABASE_KEY')

if not url or not key:
    print('ERROR: SUPABASE_URL or SUPABASE_KEY not set')
    exit(1)

try:
    from supabase import create_client
    db = create_client(url, key)
    # Try a simple query
    result = db.table('models').select('id').limit(1).execute()
    print('✓ Supabase connection successful')
except Exception as e:
    print(f'ERROR: Could not connect to Supabase: {e}')
    exit(1)
"

if [ $? -ne 0 ]; then
    echo ""
    echo "Supabase connection failed. Check your SUPABASE_URL and SUPABASE_KEY"
    exit 1
fi
echo ""

# Verify GitHub access
echo "[6/6] Testing GitHub access..."

GITHUB_TOKEN=$(grep '^GITHUB_TOKEN=' .env | cut -d'=' -f2)

if curl -s -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user | grep -q '"login"'; then
    echo "✓ GitHub access verified"
else
    echo "WARNING: GitHub token may be invalid or rate-limited"
    echo "Continuing anyway..."
fi
echo ""

# Final status
echo "========================================"
echo "Setup Complete!"
echo "========================================"
echo ""
echo "To start working:"
echo "  1. source venv/bin/activate"
echo "  2. cat CURRENT_STATE.md  # Read current state"
echo "  3. cat CHANGELOG.md      # See recent changes"
echo ""
echo "To run VibePilot:"
echo "  python dual_orchestrator.py"
echo ""
echo "Happy building!"
