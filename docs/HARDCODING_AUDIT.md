# VibePilot Hardcoding Audit

**Date:** 2026-03-03
**Purpose:** Find all hardcoded values that violate "no hardcoding" principle
**Status:** IN PROGRESS

---

## Executive Summary

**Total Issues Found:** 50+
**Fixed:** 8 (branch prefixes, config structure, timeouts, limits via system.json)
**Remaining:** ~42

---

## Fixed Issues ✅

### 1. Branch Prefixes (FIXED)

**Location:** `cmd/governor/main.go:262-264`

**Was:**
```go
branchName := fmt.Sprintf("task/%s", taskNumber)
branchName = fmt.Sprintf("task/%s", truncateID(taskID))
```

**Now:**
```go
branchName := fmt.Sprintf("%s%s", cfg.GetTaskBranchPrefix(), taskNumber)
branchName = fmt.Sprintf("%s%s", cfg.GetTaskBranchPrefix(), truncateID(taskID))
```

**Config:** `governor/config/system.json`
```json
"branch_prefixes": {
  "task": "task/",
  "module": "module/"
}
```

**To change:** Edit system.json, change prefix, restart.

### 2. Module Branch Prefix (FIXED)

**Location:** `cmd/governor/main.go:2096`

**Was:**
```go
targetBranch := fmt.Sprintf("module/%s", sliceID)
```

**Now:**
```go
targetBranch := fmt.Sprintf("%s%s", cfg.GetModuleBranchPrefix(), sliceID)
```

### 3. Config Structure (FIXED)

**Added:** `BranchPrefixConfig` to `GitConfig`
**Added:** `GetTaskBranchPrefix()` and `GetModuleBranchPrefix()` methods

### 4-8. Timeout & Limit Config Methods (ADDED)

**Added config getters:**
- `GetDBHTTPTimeoutSecs()` - reads from system.json → db.http_timeout_seconds
- `GetSessionTimeoutSecs()` - reads from system.json → session.default_timeout_seconds
- `GetExecutionTimeoutSecs()` - reads from system.json → execution.default_timeout_seconds
- `GetWebMaxTopics()` - reads from system.json → tools.web_max_topics
- `GetWebMaxRelatedTopics()` - reads from system.json → tools.web_max_related_topics
- `GetSandboxTimeoutSecs()` - reads from system.json → tools.sandbox_timeout_seconds
- `GetCourierTimeoutSecs()` - reads from system.json → courier.timeout_seconds

**Config paths added to system.json:**
```json
"db": {
  "http_timeout_seconds": 30,
  "error_truncate_length": 200
},
"http": {
  "client_timeout_seconds": 30,
  "request_timeout_seconds": 10,
  "response_timeout_seconds": 30
},
"execution": {
  "default_timeout_seconds": 300
},
"session": {
  "default_timeout_seconds": 300
},
"tools": {
  "web_max_topics": 5,
  "web_max_related_topics": 5,
  "web_timeout_seconds": 30,
  "web_max_response_length": 10000,
  "sandbox_timeout_seconds": 120
},
"courier": {
  "timeout_seconds": 30
}
```

---

## Critical Issues Fixed

| Issue | Location | Status | Fix |
|-------|----------|-------|-----|
| Branch prefix "task/" | main.go:262, 264 | ✅ Fixed | cfg.GetTaskBranchPrefix() |
| Branch prefix "module/" | main.go:2096 | ✅ Fixed | cfg.GetModuleBranchPrefix() |
| Config structure | config.go | ✅ Fixed | BranchPrefixConfig added |

---

## Remaining Issues

### High Priority

| Issue | Location | Problem |
|-------|----------|---------|
| Default CLI args | runners.go:23 | Hardcoded `["run", "--format", "json"]` |
| Default timeout values | 15+ files | Hardcoded 30s, 60s, 120s, 300s |

### Medium Priority

| Issue | Location | Problem |
|-------|----------|---------|
| Status strings | main.go (10+ places) | Hardcoded "in_progress", "review", etc. |
| HTTP timeouts | registry.go, web_tools.go | Hardcoded 30s |
| Sandbox timeouts | sandbox_tools.go | Hardcoded 60s, 120s |

### Low Priority

| Issue | Location | Problem |
|-------|----------|---------|
| Search URL | web_tools.go | Hardcoded DuckDuckGo |
| User agent | web_tools.go | Hardcoded string |

---

## Remaining Hardcoded Values

### Timeouts (NOW CONFIGURABLE ✅)

| File | Line | Hardcoded | Now Uses Config |
|------|------|-----------|-----------------|
| `runners.go` | 20 | `300` | `cfg.GetRunnerTimeoutSecs()` (main.go:170) |
| `session.go` | 12 | `300` | `cfg.GetRuntimeConfig().AgentTimeoutSeconds` |
| `sandbox_tools.go` | 16-18 | `60, 60, 120` | `cfg.GetSandboxConfig()` + getters |
| `registry.go` | 14-16 | `30, 10, 30` | Config values when available |
| `courier.go` | 32 | `30s` | Config (via deps.Config) |

**Note:** Constants remain as fallback defaults when config is missing. This is correct behavior.

### Limits

| File | Line | Hardcoded | Config Path |
|------|------|-----------|--------------|
| `runners.go` | 23 | `[]string{"run", "--format", "json"}` | system.json → cli.default_args |
| `parallel.go` | 30 | `DefaultLimit: maxPerModule` | system.json → concurrency.default_limit |
| `web_tools.go` | 90 | `5` | system.json → web.max_topics |
| `web_tools.go` | 132 | `5` | system.json → web.max_related_topics |
| `web_tools.go` | 133 | `30s` | system.json → web.timeout_seconds |
| `web_tools.go` | 217 | `10000` | system.json → web.max_response_len |

---

## What's Already Configurable ✅

| Component | Config File | Config Path |
|-----------|-------------|--------------|
| Max concurrent per module | system.json | runtime.max_concurrent_per_module |
| Max concurrent total | system.json | runtime.max_concurrent_total |
| opencode limit | system.json | concurrency.limits.opencode |
| Task branch prefix | system.json | git.branch_prefixes.task |
| Module branch prefix | system.json | git.branch_prefixes.module |
| Remote name | system.json | git.remote_name |
| Default merge target | system.json | git.default_merge_target |

---

## Next Steps

1. Add timeout configuration
2. Add CLI args configuration
3. Add HTTP timeout configuration
4. Add sandbox timeout configuration
5. Verify all status strings use config
