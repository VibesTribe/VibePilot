#!/bin/bash
# Deploy VibePilot Governor
# Reads bootstrap keys from secure credential store and starts the service

set -e

CREDENTIALS_FILE="/etc/vibepilot/bootstrap.conf"
GOVERNOR_DIR="/home/mjlockboxsocial/vibepilot/governor"
SERVICE_NAME="vibepilot-governor"

echo "=== VibePilot Governor Deploy ==="

# Check credentials file exists
if [ ! -f "$CREDENTIALS_FILE" ]; then
    echo "ERROR: Bootstrap credentials not configured."
    echo "Run: sudo scripts/setup-bootstrap.sh"
    exit 1
fi

# Read credentials (source the file)
source "$CREDENTIALS_FILE"

# Verify all keys present
if [ -z "$SUPABASE_URL" ] || [ -z "$SUPABASE_SERVICE_KEY" ] || [ -z "$VAULT_KEY" ]; then
    echo "ERROR: Missing bootstrap keys in $CREDENTIALS_FILE"
    echo "Required: SUPABASE_URL, SUPABASE_SERVICE_KEY, VAULT_KEY"
    echo "Run: sudo scripts/setup-bootstrap.sh"
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
cp /home/mjlockboxsocial/vibepilot/scripts/governor.service /etc/systemd/system/

# Create override file with environment variables
# This is the secure way to pass env vars to systemd without EnvironmentFile
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
