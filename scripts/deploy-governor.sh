#!/bin/bash
# Deploy VibePilot Governor
# Reads bootstrap keys from secure credential store and starts the service
# Works on any machine -- paths derived from script location

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
GOVERNOR_DIR="$REPO_DIR/governor"
SERVICE_NAME="vibepilot-governor"
CURRENT_USER="$(whoami)"
CREDENTIALS_FILE="/etc/v...conf"

echo "=== VibePilot Governor Deploy ==="
echo "Repo: $REPO_DIR"
echo "User: $CURRENT_USER"

# Check credentials file exists
if [ ! -f "$CREDENTIALS_FILE" ]; then
    echo "ERROR: Bootstrap credentials not configured."
    echo "Run: sudo ./scripts/setup-bootstrap.sh"
    exit 1
fi

# Read credentials (source the file)
source "$CREDENTIALS_FILE"

# Verify all keys present
if [ -z "$SUPABASE_URL" ] || [ -z "$SUPABASE_SERVICE_KEY" ] || [ -z "$VAULT_KEY" ]; then
    echo "ERROR: Missing bootstrap keys in $CREDENTIALS_FILE"
    echo "Required: SUPABASE_URL, SUPABASE_SERVICE_KEY, VAULT_KEY"
    echo "Run: sudo ./scripts/setup-bootstrap.sh"
    exit 1
fi

echo "✅ Bootstrap credentials loaded"

# Build governor
echo "Building governor..."
cd "$GOVERNOR_DIR"
go build -o governor ./cmd/governor
echo "✅ Build complete"

# Install systemd service
echo "Installing systemd service..."
cp "$REPO_DIR/scripts/governor.service" /etc/systemd/system/

# Update service file for current user and path
sed -i "s|/home/[^/]*|/home/$CURRENT_USER|g" /etc/systemd/system/vibepilot-governor.service
sed -i "s|User=[^ ]*|User=$CURRENT_USER|g" /etc/systemd/system/vibepilot-governor.service
sed -i "s|Group=[^ ]*|Group=$CURRENT_USER|g" /etc/systemd/system/vibepilot-governor.service

# Create override file with environment variables
mkdir -p /etc/systemd/system/$SERVICE_NAME.service.d
cat > /etc/systemd/system/$SERVICE_NAME.service.d/override.conf << EOF
[Service]
Environment="SUPABASE_URL=$SUPABASE_URL"
Environment="SUPABASE_SERVICE_KEY=$SUPABASE_SERVICE_KEY"
Environment="VAULT_KEY=$VAULT_KEY"
EOF
chmod 600 /etc/systemd/system/$SERVICE_NAME.service.d/override.conf

# Reload and restart
systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl restart $SERVICE_NAME

echo ""
echo "✅ Governor deployed and started"
echo ""
echo "Check status: systemctl status $SERVICE_NAME"
echo "View logs: journalctl -u $SERVICE_NAME -f"
