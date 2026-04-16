#!/bin/bash
# Restore local configuration files from backup/
# Run after cloning repo on a fresh machine

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Restoring local config files ==="

# Hermes config
mkdir -p ~/.hermes
cp "$REPO_ROOT/backup/hermes/config.yaml" ~/.hermes/config.yaml
cp "$REPO_ROOT/backup/hermes/SOUL.md" ~/.hermes/SOUL.md
echo "[OK] Hermes config restored"

# Systemd services
mkdir -p ~/.config/systemd/user
mkdir -p ~/.config/systemd/user/hermes-gateway.service.d
mkdir -p ~/.config/systemd/user/vibepilot-governor.service.d

cp "$REPO_ROOT/backup/systemd/hermes-gateway.service" ~/.config/systemd/user/
cp "$REPO_ROOT/backup/systemd/hermes-override.conf" ~/.config/systemd/user/hermes-gateway.service.d/override.conf
cp "$REPO_ROOT/backup/systemd/vibepilot-governor.service" ~/.config/systemd/user/
cp "$REPO_ROOT/backup/systemd/governor-override.conf" ~/.config/systemd/user/vibepilot-governor.service.d/override.conf
echo "[OK] Systemd services restored"

# Reload systemd
systemctl --user daemon-reload
echo "[OK] systemd daemon-reloaded"

echo ""
echo "=== Still needed ==="
echo "1. Create ~/.governor_env with VAULT_KEY, SUPABASE_URL, SUPABASE_SERVICE_KEY"
echo "2. Login to Chrome (gmail, gemini, sheets)"
echo "3. Build governor: cd ~/VibePilot/governor && go build -o ~/vibepilot/governor/governor ./cmd/governor/"
echo "4. Start: systemctl --user enable --now hermes-gateway vibepilot-governor"
