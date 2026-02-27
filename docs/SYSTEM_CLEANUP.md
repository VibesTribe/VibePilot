# System Cleanup Notes

**Date:** 2026-02-26
**Total Disk Used:** 15GB / 30GB (52%)

**Target:** e2-micro (1GB RAM, 30GB disk)

---

## e2-micro Constraints

| Resource | e2-micro | Current Usage |
|----------|----------|---------------|
| RAM | **1 GB** | 1.7 GB (on 8GB machine) |
| Disk | 30 GB | 15 GB |
| CPU | 2 vCPU (burstable) | Adequate |

### Memory Breakdown (Current)

| Process | RAM | On e2-micro? |
|---------|-----|--------------|
| opencode (this session) | 791 MB | ❌ NO - too heavy |
| pyright (LSP) | 358 MB | ❌ NO - dev only |
| gopls (Go LSP) | 235 MB | ❌ NO - dev only |
| openclaw-gateway | 100 MB | ❌ NO - not needed |
| dockerd | 60 MB | ❌ NO - not using |
| containerd | 16 MB | ❌ NO - not using |
| tailscaled | 66 MB | ⚠️ Maybe - VPN |
| Google ops agent | ~100 MB | ⚠️ Maybe - monitoring |
| **Governor (Go)** | **~50 MB** | ✅ YES - lightweight |
| System overhead | ~200 MB | ✅ Required |
| **Minimum viable** | **~300 MB** | |
| **Headroom** | **~700 MB** | |

---

## Services to Disable for e2-micro

### Not Needed (Remove)

| Service | RAM Saved | How to Disable |
|---------|-----------|----------------|
| openclaw-gateway | ~100 MB | `sudo systemctl disable openclaw-gateway` |
| docker | ~80 MB | `sudo systemctl disable docker containerd` |
| exim4 (mail) | ~15 MB | `sudo systemctl disable exim4` |

### Optional (Evaluate)

| Service | RAM | Keep? |
|---------|-----|-------|
| tailscaled | 66 MB | If VPN needed |
| google-ops-agent | 100 MB | If monitoring needed |

### Required

| Service | RAM | Purpose |
|---------|-----|---------|
| systemd-journald | 38 MB | Logging |
| sshd | ~10 MB | Remote access |
| Governor | ~50 MB | VibePilot execution |

---

## OpenCode CLI Problem

**Current situation:**
- opencode uses 791 MB RAM
- Not viable on e2-micro
- Needed for LLM agent execution

**Options:**

| Option | RAM | Notes |
|--------|-----|-------|
| 1. Build lightweight CLI | ~50 MB | Custom Go binary that calls APIs directly |
| 2. Use API directly | ~10 MB | Governor already does this for API runners |
| 3. Remote opencode | 0 MB | Run opencode locally, governor on e2-micro |
| 4. Kimi CLI alternative | ~100 MB | If subscription renewed |

**Recommended:** Option 2 or 3
- Governor already has API runner code
- Gemini/DeepSeek APIs work without heavy CLI
- Local opencode → triggers governor on e2-micro

---

## Disk Usage Breakdown

### Top Directories

| Location | Size | What |
|----------|------|------|
| `/home/mjlockboxsocial/.cache/` | **1.6G** | Caches (see below) |
| `/home/mjlockboxsocial/.local/share/` | **1.1G** | opencode, uv |
| `/home/mjlockboxsocial/vibepilot/` | **910M** | Main repo |
| `/home/mjlockboxsocial/go/` | **845M** | Go installation |
| `/var/log/` | **1.2G** | System logs |
| `/tmp/` | **1.4G** | Temp files |
| `/home/mjlockboxsocial/vibeflow/` | **173M** | Dashboard |
| `/home/mjlockboxsocial/venv/` | **22M** | Python venv |

### Cache Breakdown (1.6G)

| Cache | Size | Safe to Delete? |
|-------|------|-----------------|
| `ms-playwright` | 622M | ✅ Yes - browser automation, may not need |
| `uv` | 413M | ⚠️ Maybe - Python package cache |
| `go-build` | 324M | ✅ Yes - Go build cache, regenerates |
| `pip` | 86M | ⚠️ Maybe - pip cache |
| `gopls` | 86M | ⚠️ No - Go language server |
| `opencode` | 8.5M | No - this session |

### .local/share Breakdown (1.1G)

| Directory | Size | Safe to Delete? |
|-----------|------|-----------------|
| `opencode` | 691M | No - this session |
| `uv` | 331M | ⚠️ Maybe - Python tooling |

### /tmp Contents (1.4G)

| Item | Size | Safe to Delete? |
|------|------|-----------------|
| `go-build*` (3 dirs) | ~220M | ✅ Yes - old Go builds |
| `google-chrome-stable_current_amd64.deb` | 116M | ✅ Yes - installer, already installed |
| `node-compile-cache` | 62M | ✅ Yes - old node cache |
| `browser-use-user-data-dir-*` | ~40M | ✅ Yes - old browser sessions |
| `jiti` | 22M | ✅ Yes - old build cache |
| `gobuild` | 19M | ✅ Yes - old Go builds |

### /var/log Contents (1.2G)

System logs - can be cleaned with:
```bash
sudo journalctl --vacuum-time=7d
sudo rm /var/log/*.gz /var/log/*.1 2>/dev/null
```

---

## Cleanup Commands (Run Tomorrow)

### Safe Immediate Cleanup (~800M)

```bash
# Clean /tmp builds and installers
rm -rf /tmp/go-build* /tmp/gobuild /tmp/jiti /tmp/node-compile-cache
rm -f /tmp/google-chrome-stable_current_amd64.deb
rm -rf /tmp/browser-use-user-data-dir-*

# Clean Go build cache
go clean -cache

# Clean vibepilot logs (already did 150M)
# Already done - kept last 100 lines of orchestrator.log
```

### Optional Cleanup (~1G)

```bash
# If not using Playwright
rm -rf ~/.cache/ms-playwright

# Clean uv cache (will redownload if needed)
rm -rf ~/.cache/uv ~/.local/share/uv

# Clean pip cache
rm -rf ~/.cache/pip
```

### System Logs (~500M-1G)

```bash
# Keep last 7 days of logs
sudo journalctl --vacuum-time=7d

# Remove old rotated logs
sudo rm -f /var/log/*.gz /var/log/*.[0-9]
```

---

## Python vs Go Code

| Language | Files | Lines | Status |
|----------|-------|-------|--------|
| **Python** | 61 | 17,060 | Legacy (mostly replaced) |
| **Go** | 23 | 5,206 | Active (governor) |

### What Go REPLACED (Python → Go)

| Component | Python (Old) | Go (New) | Status |
|-----------|--------------|----------|--------|
| Governor/Orchestrator | `main.py` (~500 lines) | `cmd/governor/main.go` (470 lines) | ✅ Replaced |
| Event loop | `sentry.py` | `internal/runtime/events.go` (347 lines) | ✅ Replaced |
| Task claiming | `dispatcher.py` | `internal/db/rpc.go` + SQL | ✅ Replaced |
| Agent sessions | `agent_sessions.py` | `internal/runtime/session.go` (199 lines) | ✅ Replaced |
| Tool execution | `tools/*.py` | `internal/tools/*.go` (~1000 lines) | ✅ Replaced |
| Vault access | Direct in agents | `internal/vault/vault.go` (253 lines) | ✅ Replaced |
| Git operations | `git_ops.py` | `internal/gitree/gitree.go` (252 lines) | ✅ Replaced |
| CLI runners | `runners/cli_runner.py` | `internal/destinations/runners.go` (311 lines) | ✅ Replaced |
| API runners | `runners/api_runner.py` | `internal/destinations/runners.go` | ✅ Replaced |
| Leak detection | None | `internal/security/leak_detector.go` (69 lines) | ✅ New in Go |

### What Python is STILL USED

| File | Purpose | Still Needed? |
|------|---------|---------------|
| `vault_manager.py` | Encrypt/decrypt secrets in Supabase | ⚠️ Yes - but Go vault.go can replace |
| `scripts/backup_supabase.sh` | Daily backups | ✅ Yes - bash script |
| `scripts/research/raindrop_researcher.py` | Daily research from bookmarks | ⚠️ Paused - could rewrite in Go |
| `scripts/cleanup_zombies.sh` | Hourly cleanup | ✅ Yes - bash script |

### What Python is LEGACY (Can Delete)

| Directory/File | Lines | Status |
|----------------|-------|--------|
| `main.py` | ~500 | ❌ Replaced by Go governor |
| `agents/*.py` | ~3000 | ❌ Replaced by prompt files |
| `runners/*.py` | ~1500 | ❌ Replaced by Go runners |
| `sentry.py` | ~400 | ❌ Replaced by Go events |
| `dispatcher.py` | ~300 | ❌ Replaced by Go routing |
| `tools/*.py` | ~800 | ❌ Replaced by Go tools |
| `git_ops.py` | ~200 | ❌ Replaced by gitree |
| `universal_test.py` | ~200 | ❌ Legacy testing |

**Total Python legacy code: ~7000 lines** - can be removed

### Python We Should Keep (For Now)

| File | Why Keep |
|------|----------|
| `vault_manager.py` | Used for manual secret management |
| `scripts/research/*.py` | Research automation (paused) |
| `venv/` | Python environment if needed |

### Full Migration Path (Future)

1. **vault_manager.py** → Go `vault/vault.go` (already done, Python is wrapper)
2. **raindrop_researcher.py** → Go researcher agent
3. **Delete** all `agents/*.py`, `runners/*.py`, `main.py`, etc.
4. **Result:** Pure Go codebase, ~5000 lines vs 17000 lines Python

---

## Cron Jobs Status

**Active:**
- Supabase backup (2am daily)
- Zombie cleanup (hourly)

**Paused (waiting for Gemini):**
- Auto-commit chat
- Raindrop research (2x daily)

**Backup:** `/tmp/crontab_backup.txt`

---

## Potential Space Savings

| Action | Space Saved |
|--------|-------------|
| Clean /tmp | ~400M |
| Go clean -cache | ~300M |
| Remove Playwright | ~600M |
| Clean uv/pip cache | ~500M |
| Clean system logs | ~500M-1G |
| **Total possible** | **~2.5-3G** |

---

## Notes

- opencode directory (691M in .local/share, 153M in .cache) is this session - don't delete
- Go installation (845M) is needed
- vibepilot repo (910M) is needed
- vibeflow dashboard (173M) is needed

---

## e2-micro Setup Commands

### Disable Unneeded Services

```bash
# Stop and disable openclaw
sudo systemctl stop openclaw-gateway 2>/dev/null
sudo systemctl disable openclaw-gateway 2>/dev/null

# Stop and disable docker (if not using)
sudo systemctl stop docker containerd 2>/dev/null
sudo systemctl disable docker containerd 2>/dev/null

# Stop and disable exim4 (mail server - not needed)
sudo systemctl stop exim4 2>/dev/null
sudo systemctl disable exim4 2>/dev/null
```

### Check What's Running

```bash
# Memory usage
free -h

# Top memory processes
ps aux --sort=-%mem | head -15

# Running services
systemctl list-units --type=service --state=running
```

---

## Lightweight CLI Option (Future)

**Problem:** opencode uses 791MB, not viable on e2-micro

**Solution:** Build custom lightweight CLI in Go

### Design

```
vibe-cli (Go binary, ~10MB)
├── Reads prompt from file/stdin
├── Calls LLM API directly (Gemini/DeepSeek)
├── Returns output
└── No language servers, no heavy deps
```

### Implementation

```go
// Simple CLI that calls Gemini API
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
)

func main() {
    prompt := os.Args[1]
    apiKey := os.Getenv("GEMINI_API_KEY")
    
    // Call Gemini API
    resp, _ := http.Post(
        "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key="+apiKey,
        "application/json",
        bytes.NewReader([]byte(`{"contents":[{"parts":[{"text":"`+prompt+`"}]}]}`)),
    )
    defer resp.Body.Close()
    
    // Parse and print response
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    // ... extract text and print
}
```

**Estimated size:** ~10MB binary, ~50MB runtime
**RAM usage:** ~20-50MB

### Alternative: Direct API Calls

Governor already has this in `destinations/runners.go`:
- `APIRunner.callGemini()`
- `APIRunner.callOpenAICompatible()`

Could expose as standalone CLI without full governor.

---

## Summary: e2-micro Ready?

| Component | Current | e2-micro Ready? |
|-----------|---------|-----------------|
| Governor (Go) | 50 MB | ✅ Yes |
| Vault (Go) | 10 MB | ✅ Yes |
| API runners | 20 MB | ✅ Yes |
| System services | 200 MB | ⚠️ Need to trim |
| opencode | 791 MB | ❌ No - run locally |
| Python legacy | 0 MB (not running) | ✅ N/A |
| **Total viable** | **~280 MB** | ✅ Fits in 1GB |

**Action items:**
1. Disable openclaw, docker, exim4
2. Clean caches and logs
3. Run governor standalone
4. Use opencode locally or build lightweight CLI
