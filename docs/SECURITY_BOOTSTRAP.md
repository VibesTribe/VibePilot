# Security Bootstrap Architecture

**Read this before touching any credentials.**

---

## The Problem We're Solving

1.5 million API keys were stolen from OpenClaw users recently. Agents with file access could read `.env` files. We prevent this by never storing keys in files.

---

## Three Bootstrap Keys, Process Environment Only

| Key | Source | Purpose |
|-----|--------|---------|
| `SUPABASE_URL` | GitHub Secrets | Database endpoint |
| `SUPABASE_KEY` | GitHub Secrets | Anon key - reads from vault via RLS policy |
| `VAULT_KEY` | GitHub Secrets | Decrypts secrets from vault |

**These are the ONLY keys that exist before runtime.** Everything else is encrypted in the vault.

---

## Architecture

```
GitHub Secrets (deploy time)
        │
        ▼
┌─────────────────────────────────────┐
│  Process Environment (memory only)  │
│  - SUPABASE_URL                     │
│  - SUPABASE_KEY                     │
│  - VAULT_KEY                        │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│  Governor starts                    │
│  - Connects to Supabase with KEY    │
│  - Creates Vault with VAULT_KEY     │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│  Vault retrieves secrets at runtime │
│  - GITHUB_TOKEN                     │
│  - DEEPSEEK_API_KEY                 │
│  - GEMINI_API_KEY                   │
│  - etc.                             │
└─────────────────────────────────────┘
```

---

## What NEVER Happens

- ❌ NO `.env` files with keys
- ❌ NO `EnvironmentFile=` in systemd
- ❌ NO hardcoded keys in code
- ❌ NO keys in bash commands
- ❌ NO keys in git commits

---

## RLS Policy for Vault

The `secrets_vault` table uses Row Level Security:

```sql
-- Service role gets full access
CREATE POLICY "vault_service_role_full" ON secrets_vault
  FOR ALL TO service_role
  USING (true) WITH CHECK (true);

-- Anon/authenticated can SELECT (read only)
CREATE POLICY "vault_authenticated_read" ON secrets_vault
  FOR SELECT TO authenticated
  USING (true);
```

This allows `SUPABASE_KEY` (anon) to read from vault, but only `SUPABASE_SERVICE_KEY` can write.

---

## Deployment

Keys are injected from GitHub Secrets at deploy time:

```bash
# Deploy script (run by human or CI/CD)
export SUPABASE_URL="${FROM_GITHUB_SECRET}"
export SUPABASE_KEY="${FROM_GITHUB_SECRET}"
export VAULT_KEY="${FROM_GITHUB_SECRET}"
systemctl restart vibepilot-governor
```

---

## For New Agents/Sessions

Before touching any credentials:

1. Read this file
2. Understand: keys come from process environment → vault
3. Never read from `.env` files
4. Never hardcode keys anywhere

---

## Files That Reference This

- `governor/config/system.json` - defines which env vars to use
- `scripts/governor.service` - systemd unit (NO EnvironmentFile)
- `governor/internal/vault/vault.go` - vault implementation
- `governor/internal/db/supabase.go` - database connection
- `vault_manager.py` - Python vault implementation

---

## Update Log

| Date | Change |
|------|--------|
| 2026-02-27 | Created after session wasted by hardcoded keys confusion |
