# VibePilot Migration Guide

Copy-paste everything to a fresh server. Done.

---

## What You Need Before Starting

1. **GitHub access** - SSH key or token to clone repo
2. **3 bootstrap keys** - stored somewhere safe (password manager)
   - SUPABASE_URL
   - SUPABASE_KEY (service_role key)
   - VAULT_KEY (encryption key for secrets vault)

---

## Step 1: Clone Repo

```bash
git clone git@github.com:VibesTribe/VibePilot.git
cd VibePilot
```

---

## Step 2: Create .env File

```bash
cat > .env << 'EOF'
# --- VIBEPILOT BOOTSTRAP ---
SUPABASE_URL=https://qtpdzsinvifkgpxyxlaz.supabase.co
SUPABASE_KEY=<your-service-role-key>
VAULT_KEY=<your-vault-encryption-key>
EOF
```

Replace `<your-service-role-key>` and `<your-vault-encryption-key>` with actual values.

---

## Step 3: Run Setup Script

```bash
chmod +x docs/move/setup.sh
./docs/move/setup.sh
```

This does:
- Create Python venv
- Install all dependencies
- Verify Supabase connection
- Install systemd service

---

## Step 4: Verify It's Running

```bash
systemctl status vibepilot-orchestrator
journalctl -u vibepilot-orchestrator -f
```

---

## Quick Commands (After Setup)

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-orchestrator` | Check if running |
| `systemctl restart vibepilot-orchestrator` | Restart service |
| `systemctl stop vibepilot-orchestrator` | Stop service |
| `journalctl -u vibepilot-orchestrator -f` | Live logs |
| `journalctl -u vibepilot-orchestrator --since "1 hour ago"` | Recent logs |

---

## What Survives a Move

| Thing | Where | Survives? |
|-------|-------|-----------|
| Code | GitHub | Yes |
| Config | GitHub + Supabase | Yes |
| Secrets | Supabase Vault | Yes |
| Tasks/Runs | Supabase | Yes |
| Logs | Local server | No (regenerate) |
| venv | Local server | No (recreated by setup.sh) |

---

## Troubleshooting

### Service won't start
```bash
journalctl -u vibepilot-orchestrator -n 50
```

### Can't connect to Supabase
```bash
# Check .env exists and has values
cat .env

# Test connection manually
source venv/bin/activate
python -c "from supabase import create_client; import os; c=create_client(os.environ['SUPABASE_URL'], os.environ['SUPABASE_KEY']); print(c.table('tasks').select('id').limit(1).execute())"
```

### Port/firewall issues
Orchestrator doesn't expose any ports - it only connects outbound to Supabase. No firewall config needed.
