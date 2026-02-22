# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SYSTEM_REFERENCE.md`** ← **WHAT WE HAVE AND HOW IT WORKS** (start here!)
3. **`docs/GO_IRON_STACK.md`** ← **GO ARCHITECTURE SPEC** (the plan)
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`docs/prd_v1.4.md`** - Complete system specification

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-22
**Updated By:** GLM-5 - Session 23: Phase 1 COMPLETE - Go Governor running
**Session Focus:** Go Governor built, tested, connected to Supabase
**Direction:** Phase 2 next - GitHub Actions integration

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Phase 1 COMPLETE - Governor runs, connects to Supabase, API works

---

# SESSION 23: GO IRON STACK - PHASE 1 COMPLETE (2026-02-22)

## Phase 1 Status: ✅ COMPLETE

**Built and tested:**
- Go Governor binary: 6.5MB (target: <15MB) ✅
- Connects to Supabase via REST API ✅
- All components start: Sentry, Dispatcher, Janitor, Server ✅
- HTTP API works: /api/tasks, /api/models ✅
- Uses SUPABASE_SERVICE_KEY from vault/GitHub secrets ✅

**Files created in `governor/`:**
```
governor/
├── cmd/governor/main.go           # Entry point
├── internal/
│   ├── sentry/sentry.go           # Polls Supabase (15s, max 3)
│   ├── dispatcher/dispatcher.go   # Routes to GitHub/CLI
│   ├── janitor/janitor.go         # Resets stuck tasks
│   ├── server/server.go           # HTTP API + WebSocket
│   ├── config/config.go           # YAML config
│   ├── db/supabase.go             # REST API client
│   └── security/
│       ├── leak_detector.go       # IronClaw pattern
│       └── allowlist.go           # IronClaw pattern
├── pkg/types/types.go             # Shared types
├── go.mod                         # Minimal deps
├── governor.yaml                  # Configuration
└── Makefile
```

**Dependencies (minimal):**
- gopkg.in/yaml.v3 - config parsing
- github.com/gorilla/websocket - real-time updates
- Standard library (net/http) - Supabase REST API

**No external Postgres driver** - uses Supabase REST API like Python does.

## Remaining Phases

| Phase | Status | Description |
|-------|--------|-------------|
| 1. Foundation | ✅ COMPLETE | Go scaffold, Sentry, Dispatcher, Janitor, Server |
| 2. GitHub Integration | 🔄 NEXT | Actions dispatch, courier workflow, branch management |
| 3. HTTP Server | Pending | Dashboard wiring, WebSocket real-time |
| 4. Cutover | Pending | Parallel run with Python, verify, switch |

## How to Run

```bash
cd ~/vibepilot/governor
export SUPABASE_URL="https://qtpdzsinvifkgpxyxlaz.supabase.co"
export SUPABASE_SERVICE_KEY="<from vault or GitHub secrets>"
./governor
```

## Strategic Pivot (Context)

## Strategic Pivot

**Problem:** 
- GCE e2-standard-2: $64/mo (not sustainable)
- OpenCode runner alone: 1.4GB RAM
- Target: e2-micro (1GB total, free tier)
- Python cannot scale to 18 concurrent agents on 1GB

**Solution:** Go Iron Stack
- Replace Python orchestrator with Go "Governor" (10-20MB)
- Offload browser-use to GitHub Actions (7GB free runners)
- Single binary deployment with embedded UI
- Fits free tier perfectly

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | 10-20MB vs 1.4GB, goroutines for concurrency |
| Browser execution | GitHub Actions | 7GB free per runner, unlimited parallelism |
| Task dispatch | Poll-based | No 429 rate limits, controlled drip-feed |
| Deployment | Single binary | No venv/pip/drift |

## What Stays (DO NOT TOUCH)

- **Dashboard** - vibeflow repo, user is attached
- **Agent prompts** - config/prompts/*.md preserved
- **Architecture docs** - prd, philosophy, considerations
- **Supabase schema** - no migrations needed
- **Config files** - same JSON, Go reads them

## What Changes

- Python orchestrator → Go Governor
- Local browsers → GitHub Actions
- venv + pip → Single binary

## New Files Created

| File | Purpose |
|------|---------|
| `docs/SYSTEM_REFERENCE.md` | Single source of truth for "what we have" |
| `docs/GO_IRON_STACK.md` | Complete Go architecture specification |

## Migration Phases

1. **Foundation** (1 session) - Go scaffold, Sentry, Dispatcher
2. **GitHub Integration** (1 session) - Actions dispatch, workflows
3. **HTTP Server** (1 session) - API, WebSocket, embedded UI
4. **Cutover** (1 session) - Parallel run, verify, switch

---

# SESSION 22 FULL SUMMARY (2026-02-21)

## Infrastructure Fixes ✅

1. **OpenCode Runner Created** - Added to RUNNER_REGISTRY
2. **Process Memory Leak Fixed** - Popen with process group cleanup
3. **Models Table Status Respected** - Skips paused/benched in runner pool
4. **RPC Missing result Column Fixed** - Use fallback query with select("*")
5. **Escalation Preserves prompt_packet** - Merge, don't overwrite
6. **Stuck Task Detection** - 10min timeout, auto-reset
7. **Cooldown Auto-Reactivation** - Checks on startup + periodic
8. **Robust Plan Parsing** - 5 extraction methods, retry logic

## Swaps Made ✅

- **Kimi CLI removed** - Subscription cancelled, not cost effective
- **Kimi benched in DB** - Can reactivate via API later if needed
- **GLM-5 only active runner** - OpenCode is heavy (1.4GB) but working

## System Baseline

| Component | Current | Target |
|-----------|---------|--------|
| GCE Instance | e2-standard-2 ($64/mo) | e2-micro (free) |
| Orchestrator | 99 MB | Keep |
| OpenCode session | 1.4 GB | Replace with lean runner |
| VibePilot code | 16,106 lines Python | ~4k lines (Claw pattern) |
| Runners | 1 active (GLM-5) | Multiple swappable |

---

# CLAW FRAMEWORK RESEARCH

## Deep Dive Findings

### ZeroClaw (Rust, 8.8MB binary, 5MB RAM)

**What we should adopt:**
- **Provider traits** - Config-driven LLM swapping, no code changes
- **SQLite + FTS5** - Bundled, no external dependencies
- **Hot-config reload** - Changes apply without restart
- **Tool registry** - Tools self-describe, work with any LLM
- **Size optimization** - 8.8MB binary, <10ms startup

**Key code patterns:**
```rust
// Provider trait - swap LLMs via config
pub trait Provider: Send + Sync {
    async fn chat(&self, request: ChatRequest, model: &str) -> Result<ChatResponse>;
    fn supports_native_tools(&self) -> bool;
}

// Config-driven selection
default_provider = "openrouter"  // or "anthropic", "custom:https://..."
```

### NanoClaw (TypeScript, ~4k lines, 17% LLM context)

**What we should adopt:**
- **Fits in LLM context** - Entire codebase understandable by AI
- **1 file per concern** - db.ts, config.ts, types.ts
- **Skills over plugins** - Transform codebase, don't configure framework
- **File-based IPC** - Simple, debuggable, no Redis
- **SQLite direct** - No ORM, no abstraction

**Why 4k lines works:**
- No abstraction layers (no factories, managers)
- Single responsibility files
- Intentionally left out web dashboard, multiple channels, etc.

### IronClaw (Rust, 20k lines, security-focused)

**What we should adopt:**
- **Leak detection** - Scan tool outputs for secret patterns (portable to Python)
- **Credential injection** - Secrets injected at boundary, never in context
- **Allowlist validation** - HTTP requests only to approved endpoints
- **Docker sandbox** - Container-based isolation (not WASM for now)

**Patterns portable to Python:**
```python
# Leak detection - direct port
class LeakDetector:
    PATTERNS = [
        ("openai_key", r"sk-[a-zA-Z0-9]{20,}", "block"),
        ("github_token", r"gh[pousr]_[A-Za-z0-9_]{36,}", "block"),
    ]
```

---

## Patterns to Adopt in VibePilot

| From | Pattern | VibePilot Application |
|------|---------|----------------------|
| **ZeroClaw** | Config-driven providers | models.json → hot-swap runners |
| **ZeroClaw** | 5MB footprint | Audit and strip to essentials |
| **NanoClaw** | 4k lines | Consolidate, one file per concern |
| **NanoClaw** | Fits in context | AI can modify entire codebase |
| **IronClaw** | Leak detection | Add to tool output processing |
| **IronClaw** | Credential injection | Vault secrets at execution only |

---

## Current Codebase vs Claw Patterns

| We Have | Claw Approach | Gap |
|---------|---------------|-----|
| Runner registry (hardcoded) | Config-driven factory | Need config swap |
| Memory backend (pluggable) | SQLite only | Overengineered |
| 16k lines Python | 4k lines focused | Bloat |
| Vault (works) | Injection at boundary | Exposed in context |
| No leak detection | Regex scanner | Missing |

---

## Path Forward

### Option A: Test GLM Subscription Endpoint (10 min)
- Verify `api.z.ai/api/anthropic` works with subscription
- If yes → build lightweight runner

### Option B: Python Refactor with Claw Patterns (1 session)
- Audit 16k lines → identify dead code
- Apply config-driven providers
- Add leak detection
- Consolidate to ~4k lines

### Option C: Fork ZeroClaw, Customize (1-2 sessions)
- Take proven Rust runner
- Strip to core
- Add VibePilot integrations (Supabase, GitHub)

---

## GLM Subscription Research (from Gemini)

**Anthropic-compatible endpoint:**
```
Base URL: https://api.z.ai/api/anthropic
Format: Anthropic "Messages" format
Model: glm-4.7 or glm-5
Billing: Uses subscription quota (not pay-per-token)
```

**Other platforms supported:**
- Claude Code CLI (set ANTHROPIC_BASE_URL)
- Kilo Code / Roo Code (pre-set for Z-AI)
- Direct Python (anthropic library)

---

## Requirements for Web of Webs

VibePilot must support building WoW - a massively scaleable, secure, multimodal social network:

- ✅ Run on free tier with parallel agents
- ✅ Track every token (task → module → project)
- ✅ Track every model on every task
- ✅ ROI calculator works
- ✅ Farm to ANY web AI platform
- ✅ Real task data (not benchmarks)
- ✅ New strategies in minutes
- ✅ Know each LLM's strengths/weaknesses
- ✅ Never exceed free tier limits
- ✅ Complex tasks → in-house only (triple checked)
- ✅ Planner → 95% confidence, one-shot on weakest model
- ✅ Parallel: free web models + in-house QC

---

# ACTIVE MODELS

| Model ID | Status | Notes |
|----------|--------|-------|
| glm-5 (opencode) | ✅ ACTIVE | Only working runner (1.4GB) |
| kimi-cli | BENCHED | Subscription cancelled |
| gemini-api | PAUSED | Quota exhausted |
| deepseek-chat | PAUSED | Credit needed |

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat CURRENT_STATE.md` | This file |
| `cat AGENTS.md` | Mental model + workflow |
| `sudo journalctl -u vibepilot-orchestrator -f` | Orchestrator logs |
| `cd ~/vibepilot && source venv/bin/activate` | Activate venv |

---

# FILES MODIFIED THIS SESSION

| File | Change |
|------|--------|
| `runners/contract_runners.py` | Added OpenCodeContractRunner, process cleanup |
| `core/orchestrator.py` | Model status check, stuck task detection, cooldown reactivation |
| `agents/planner.py` | Robust plan parsing with retry |
| `task_manager.py` | Preserve prompt_packet on escalation |
| `AGENTS.md` | Added mental model section |
| `CURRENT_STATE.md` | This update |
