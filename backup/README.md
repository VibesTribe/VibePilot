# Backup: Local Configuration Files

These files are machine-specific and NOT in the repo naturally.
They're backed up here for disaster recovery.

## What's in here

| File | Purpose | Restore to |
|---|---|---|
| `hermes/config.yaml` | Hermes model, fallbacks, cwd | `~/.hermes/config.yaml` |
| `hermes/SOUL.md` | Hermes personality (currently empty template) | `~/.hermes/SOUL.md` |
| `systemd/hermes-gateway.service` | Hermes systemd service (has TERMINAL_CWD) | `~/.config/systemd/user/hermes-gateway.service` |
| `systemd/hermes-override.conf` | Hermes env overrides (API keys, governor env) | `~/.config/systemd/user/hermes-gateway.service.d/override.conf` |
| `systemd/vibepilot-governor.service` | Governor systemd service | `~/.config/systemd/user/vibepilot-governor.service` |
| `systemd/governor-override.conf` | Governor env overrides | `~/.config/systemd/user/vibepilot-governor.service.d/override.conf` |

## How to restore on a fresh machine

1. Clone repo: `git clone https://github.com/VibesTribe/VibePilot.git ~/VibePilot`
2. Copy repo to running location: `cp -r ~/VibePilot ~/vibepilot`
3. Run install: `bash ~/VibePilot/.context/tools/install.sh`
4. Restore configs: `bash ~/VibePilot/backup/restore.sh`
5. Set up governor env: copy `.governor_env` template, fill in secrets from vault
6. Build governor: `cd ~/VibePilot/governor && go build -o ~/vibepilot/governor/governor ./cmd/governor/`
7. Start services: `systemctl --user daemon-reload && systemctl --user enable --now hermes-gateway vibepilot-governor`

## What's NOT here (must recreate)

- `~/.governor_env` -- contains VAULT_KEY, SUPABASE_URL, SUPABASE_SERVICE_KEY. Get from password manager.
- `~/.config/chrome-debug/` -- Chrome profile with logged-in sessions. Must re-login.
- SSH keys -- must regenerate and add to GitHub.
