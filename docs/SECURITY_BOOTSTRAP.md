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
| `SUPABASE_SERVICE_KEY` | GitHub Secrets | Service role - bypasses RLS, reads/writes vault |
| `VAULT_KEY` | GitHub Secrets | Decrypts secrets from vault |

**These are the ONLY keys that exist before runtime.** Everything else is encrypted in the vault.

**IMPORTANT:** We use `SUPABASE_SERVICE_KEY` (not anon key) because:
- The vault table has RLS enabled
- Only service_role can read/write the vault
- Anon key is BLOCKED by RLS policy
- This is intentional security - prevents compromised agents from dumping vault

---

## Architecture

```
GitHub Secrets (deploy time)
        │
        ▼
┌─────────────────────────────────────┐
│  systemd override.conf (root-only)  │
│  - SUPABASE_URL                     │
│  - SUPABASE_SERVICE_KEY             │
│  - VAULT_KEY                        │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│  Governor process (memory only)     │
│  - Reads from os.Getenv()           │
│  - Uses SERVICE_KEY for DB access   │
│  - Uses VAULT_KEY for decryption    │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│  Vault retrieves secrets at runtime │
│  - GITHUB_TOKEN                     │
│  - DEEPSEEK_API_KEY                 │
│  - GEMINI_API_KEY                   │
│  - RAINDROP_ACCESS_TOKEN            │
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
- ❌ NO anon key for vault access (blocked by RLS)

---

## RLS Policy for Vault

The `secrets_vault` table uses Row Level Security:

```sql
-- Service role gets full access (governor uses this)
CREATE POLICY "vault_service_role_full" ON secrets_vault
  FOR ALL TO service_role
  USING (true) WITH CHECK (true);

-- Anon/authenticated are BLOCKED
CREATE POLICY "vault_no_delete" ON secrets_vault
  FOR DELETE TO authenticated USING (false);
CREATE POLICY "vault_no_insert" ON secrets_vault
  FOR INSERT TO authenticated WITH CHECK (false);
CREATE POLICY "vault_no_update" ON secrets_vault
  FOR UPDATE TO authenticated USING (false) WITH CHECK (false);
```

**Why this matters:** Even if an agent somehow gets the anon key, it CANNOT read the vault. Only the service key (stored root-only in systemd override) can access it.

---

## Where Keys Live

| Location | What's There | Who Can Read |
|----------|--------------|--------------|
| `/etc/systemd/.../override.conf` | All 3 bootstrap keys | root only |
| `.env` | NOTHING (empty) | Doesn't matter |
| GitHub Secrets | Backup copy | GitHub only |

**The systemd override file is the ONLY place keys exist on the server.**

---

## Deployment

### Manual Deploy (one-time setup)

```bash
# 1. Get bootstrap keys from GitHub Secrets
# 2. Run setup script (prompts for keys)
sudo scripts/setup-bootstrap.sh

# 3. Deploy
sudo scripts/deploy-governor.sh
```

### GitHub Actions (automated)

See `.github/workflows/deploy-governor.yml`

---

## For New Agents/Sessions

Before touching any credentials:

1. **Read this file first**
2. Keys are in systemd override (root-only - you CAN'T read it)
3. Never look for `.env` files - they're empty by design
4. Never change `key_env` from `SUPABASE_SERVICE_KEY` to `SUPABASE_KEY`
5. If you need a secret, it's in the vault (use Go vault implementation)
6. **DO NOT add RLS policy for anon** - it defeats the security model

---

## Files That Reference This

| File | What It Does |
|------|--------------|
| `governor/config/system.json` | Defines `key_env: SUPABASE_SERVICE_KEY` |
| `governor/internal/vault/vault.go` | Full architecture docs in comments |
| `governor/internal/runtime/config.go` | DatabaseConfig with KeyEnv |
| `scripts/governor.service` | Systemd unit (NO EnvironmentFile) |
| `scripts/setup-bootstrap.sh` | Prompts for 3 bootstrap keys |
| `scripts/deploy-governor.sh` | Injects keys to systemd override |

---

## Update Log

| Date | Change |
|------|--------|
| 2026-02-27 | Fixed: SERVICE_KEY not anon key (correct architecture) |
| 2026-02-27 | Keys removed from .env, stored only in systemd override |
| 2026-02-27 | Old Python orchestrator disabled |
