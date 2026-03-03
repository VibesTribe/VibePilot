# VibePilot Hardcoding Audit

**Date:** 2026-03-03
**Purpose:** Find all hardcoded values that violate "no hardcoding" principle

---

## Executive Summary

**Total Issues Found:** 50+

**Categories:**
1. **Timeouts** - Time values hardcoded
2. **Limits** - Concurrency, counts hardcoded
3. **Strings** - Branch names, remote names, status values
4. **URLs** - API endpoints hardcoded
5. **Defaults** - Fallback values hardcoded

---

## Critical Issues

### 1. Concurrency Limits (CRITICAL)

**Location:** `cmd/governor/main.go:124`

```go
cfg.System.Runtime.MaxConcurrentPerModule, 
cfg.System.Runtime.MaxConcurrentTotal, 
cfg.System.Concurrency.GetLimit("opencode")  // ← HARDCODED OVERRIDE
```

**Problem:** The `GetLimit("opencode")` overrides config values with hardcoded limit.

**Fix:** Remove hardcoded override. Read all limits from config.

---

### 2. Branch Names (HIGH)

**Location:** `internal/gitree/gitree.go:66`

```go
RemoteName = "origin"  // ← HARDCODED
```

**Location:** `internal/gitree/gitree.go:70`

```go
branchName = "task/" + taskID[:8]  // ← HARDCODED PREFIX
```

**Location:** `internal/gitree/gitree.go:356`

```go
branchName = "module/" + sliceID  // ← HARDCODED PREFIX
```

**Fix:** Read from config:
```json
{
  "git": {
    "remote_name": "origin",
    "task_branch_prefix": "task/",
    "module_branch_prefix": "module/"
  }
}
```

---

### 3. Status Strings (MEDIUM)

**Location:** `cmd/governor/main.go` (multiple lines)

```go
"p_status":  "in_progress",  // ← HARDCODED
"p_status":  "review",        // ← HARDCODED
"p_status":  "testing",       // ← HARDCODED
"p_status":  "available",     // ← HARDCODED
"p_status":  "escalated",     // ← HARDCODED
```

**Problem:** Status strings are defined in config but not consistently used.

**Fix:** Use config constants everywhere:
```go
cfg.GetTaskStatus("in_progress")  // Instead of hardcoded string
```

---

## All Hardcoded Values

### Timeouts

| File | Line | Hardcoded | Should Be |
|------|------|-----------|-----------|
| `supabase.go` | 17 | `30` | config.db.http_timeout_secs |
| `supabase.go` | 18 | `200` | config.db.error_truncate_len |
| `gitree.go` | 17 | `60s` | config.git.timeout_secs |
| `registry.go` | 14-16 | `30, 10, 30` | config.http.timeout, max_idle, idle_timeout |
| `sandbox_tools.go` | 16-18 | `60, 60, 120` | config.tools.sandbox_timeout, lint_timeout, typecheck_timeout |
| `runners.go` | 20 | `300` | config.execution.default_timeout_secs |
| `courier.go` | 32 | `30s` | config.courier.timeout_secs |
| `session.go` | 41 | `300s` | config.session.default_timeout_secs |

### Limits

| File | Line | Hardcoded | Should Be |
|------|------|-----------|-----------|
| `runners.go` | 23 | `[]string{"run", "--format", "json"}` | config.cli.default_args |
| `parallel.go` | 30 | `DefaultLimit: maxPerModule` | config.concurrency.default_limit |
| `web_tools.go` | 90 | `5` | config.web.max_topics |
| `web_tools.go` | 132 | `5` | config.web.max_related_topics |
| `web_tools.go` | 133 | `30s` | config.web.timeout_secs |
| `web_tools.go` | 217 | `10000` | config.web.max_response_len |

### Strings

| File | Line | Hardcoded | Should Be |
|------|------|-----------|-----------|
| `gitree.go` | 66 | `"origin"` | config.git.remote_name |
| `gitree.go` | 70 | `"task/"` | config.git.task_branch_prefix |
| `gitree.go` | 356 | `"module/"` | config.git.module_branch_prefix |
| `runners.go` | 23 | `"run", "--format", "json"` | config.cli.default_args |
| `courier.go` | 162 | `"https://api.github.com/repos/%s/dispatches"` | config.courier.github_api_url |

### URLs

| File | Line | Hardcoded | Should Be |
|------|------|-----------|-----------|
| `courier.go` | 162 | `https://api.github.com/repos/...` | config.courier.github_api_template |
| `config.go` | 702 | `https://api.duckduckgo.com/` | config.web.search_url |

### Status Values

| File | Lines | Count | Issue |
|------|-------|-------|-------|
| `main.go` | 275, 348, 374, 428, 454, 463, 472, 482, 565, 598 | 10+ | Status strings hardcoded despite being in config |

---

## What's Already Configurable (Good)

| Component | Config File | Status |
|-----------|-------------|--------|
| Max concurrent per module | `system.json` | ✅ Good |
| Max concurrent total | `system.json` | ✅ Good |
| Poll interval | `system.json` | ✅ Good |
| Processing timeout | `system.json` | ✅ Good |
| Revision max rounds | `plan_lifecycle.json` | ✅ Good |
| Council member count | `plan_lifecycle.json` | ✅ Good |

---

## What Needs To Be Configurable

### Priority 1: Blocking Issues

1. **Remove opencode hardcode override**
   - File: `cmd/governor/main.go:124`
   - Fix: Read all limits from `config/concurrency.json`

2. **Git branch prefixes**
   - File: `internal/gitree/gitree.go`
   - Fix: Add to `config/git.json`

3. **Default CLI args**
   - File: `internal/destinations/runners.go:23`
   - Fix: Add to `config/cli.json`

### Priority 2: Timeouts

Create `config/timeouts.json`:

```json
{
  "db": {
    "http_timeout_secs": 30,
    "error_truncate_len": 200
  },
  "git": {
    "timeout_secs": 60
  },
  "http": {
    "timeout_secs": 30,
    "max_idle_conns": 10,
    "idle_conn_timeout_secs": 30
  },
  "tools": {
    "sandbox_timeout_secs": 60,
    "lint_timeout_secs": 60,
    "typecheck_timeout_secs": 120
  },
  "execution": {
    "default_timeout_secs": 300
  },
  "courier": {
    "timeout_secs": 30,
    "poll_interval_secs": 5
  },
  "session": {
    "default_timeout_secs": 300
  },
  "web": {
    "timeout_secs": 30,
    "max_topics": 5,
    "max_related_topics": 5,
    "max_response_len": 10000
  }
}
```

### Priority 3: Status Strings

Already defined in config but not consistently used. Need to:
1. Create constants from config
2. Replace all hardcoded strings with config references

---

## Recommended Config Structure

```
config/
├── system.json          # Existing - good
├── plan_lifecycle.json  # Existing - good
├── routing.json         # Existing - good
├── destinations.json    # Existing - good
├── concurrency.json     # NEW - per-destination limits
├── git.json             # NEW - branch prefixes, remote name
├── timeouts.json        # NEW - all timeout values
├── cli.json             # NEW - CLI defaults
└── web.json             # NEW - web tool settings
```

---

## Immediate Fix Required

### The opencode Limit Problem

**Current Code:**
```go
// main.go:124
cfg.System.Concurrency.GetLimit("opencode")
```

**What it does:** Overrides config with hardcoded limit

**Why it's there:** You set it to 2 because 8 sessions were killing GLM

**Proper Fix:**

1. Create `config/concurrency.json`:
```json
{
  "limits": {
    "opencode": 2,
    "cline": 50,
    "api": 100,
    "mcp": 50
  },
  "defaults": {
    "cli": 2,
    "api": 10,
    "mcp": 10
  }
}
```

2. Read from config:
```go
limit := cfg.Concurrency.GetLimit(destinationType)
if limit == 0 {
    limit = cfg.Concurrency.GetDefault(destinationType)
}
```

3. Change limit by editing JSON:
```json
{
  "limits": {
    "opencode": 50  // Changed from 2 to 50 when you have more agents
  }
}
```

---

## Audit Complete

**Total violations:** 50+
**Critical:** 3 (opencode override, branch prefixes, CLI args)
**High:** 15 (timeouts)
**Medium:** 20+ (status strings)
**Low:** 12 (URLs, defaults)

---

*Next Step: Create config files and refactor code to use them*
