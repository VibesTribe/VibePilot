#!/bin/bash
# One-time setup for VibePilot bootstrap keys
# Run this once to configure the secure credential store

set -e

CREDENTIALS_FILE="/etc/vibepilot/bootstrap.conf"

echo "=== VibePilot Bootstrap Setup ==="
echo ""
echo "This script stores the three bootstrap keys securely."
echo "These keys come from GitHub Secrets:"
echo "  - SUPABASE_URL"
echo "  - SUPABASE_KEY"
echo "  - VAULT_KEY"
echo ""

# Check if already configured
if [ -f "$CREDENTIALS_FILE" ]; then
    echo "WARNING: $CREDENTIALS_FILE already exists."
    read -p "Overwrite? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "Aborted."
        exit 1
    fi
fi

# Create directory
sudo mkdir -p /etc/vibepilot

# Prompt for keys
echo ""
echo "Enter SUPABASE_URL:"
read -r SUPABASE_URL

echo ""
echo "Enter SUPABASE_KEY (anon key):"
read -rs SUPABASE_KEY
echo "(hidden)"

echo ""
echo "Enter VAULT_KEY:"
read -rs VAULT_KEY
echo "(hidden)"

# Write credentials file
sudo tee "$CREDENTIALS_FILE" > /dev/null << EOF
# VibePilot Bootstrap Credentials
# DO NOT EDIT - managed by setup-bootstrap.sh
# Generated: $(date -Iseconds)

SUPABASE_URL=$SUPABASE_URL
SUPABASE_KEY=$SUPABASE_KEY
VAULT_KEY=$VAULT_KEY
EOF

# Secure permissions - root only
sudo chmod 600 "$CREDENTIALS_FILE"
sudo chown root:root "$CREDENTIALS_FILE"

echo ""
echo "✅ Bootstrap credentials stored in $CREDENTIALS_FILE"
echo ""
echo "Now run: sudo scripts/deploy-governor.sh"
