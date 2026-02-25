# Governor Rebuild Plan

**Date:** 2026-02-25
**Status:** Approved, Ready to Implement
**Session Context:** Recovered from terminal crash - this document captures all decisions

---

## The Problem

Current Go codebase: **8,287 lines**
- 70% is "agent logic" that should be in LLM prompts
- Hardcoded decision trees prevent VibePilot from functioning as designed
- Agents (supervisor, council, planner, etc.) need LLMs to think, but Go code tries to think for them

### What Go Currently Does (Wrong)
```go
// council.go - WRONG APPROACH
func (c *Council) ReviewPlan(...) {
    prompt := "COUNCIL PLAN REVIEW\n..." // Built in Go
    output := callLLM(prompt)
    result := parseJSON(output)          // Parsed in Go
    if result.Vote == "APPROVED" { ... }  // Decided in Go
}
```

### What Go Should Do (Right)
```go
// Tool for LLM to call
func ToolCreateBranch(ctx context.Context, name string) error {
    return exec.Command("git", "checkout", "-b", name).Run()
}
// LLM decides WHEN to call, not Go
```

---

## Core Principle: Everything Swappable

```
CONFIG defines everything:
- Which models → models.json
- Which platforms → destinations.json
- Which database → system.json
- Which git host → system.json
- Which courier driver → system.json
- Agent definitions → agents.json

CODE provides only:
- Generic tools (git, files, secrets, queries)
- Generic LLM session (call any model, parse tools)
- Generic event detection (watch DB for state changes)

PROMPTS provide:
- All agent intelligence
- All decision logic
- All learning/improvement patterns
```

**If we pack up and move:**
1. Clone GitHub repo
2. Set 3 env vars (SUPABASE_URL, SUPABASE_KEY, VAULT_KEY)
3. Everything else loads from config

---

## The VibePilot Flow (As Designed)

```
Human Idea
    │
    ▼
CONSULTANT (LLM + consultant.md + web_search tool)
    │
    ▼
PLANNER (LLM + planner.md + file_read, git_read, db tools)
    │
    ▼
SUPERVISOR TRIAGE
    ├── Simple? → APPROVE directly
    └── Complex? → CALL COUNCIL
            │
            ▼
        COUNCIL (3 LLMs + council.md + file_read, web_search)
        - Reviews, discusses, suggests fixes
        - Planner fixes, loops until consensus
            │
            ▼
        Supervisor approves
    │
    ▼
ORCHESTRATOR (LLM + orchestrator.md + db tools)
    - Routes tasks based on learned performance
    - Tests new models/platforms
    - Decreases usage of poor performers
    │
    ▼
TASK EXECUTION (Runner or Courier)
    │
    ▼
SUPERVISOR OUTPUT QC
    - Aligned? Complete? No injection? Safe?
    ├── Approve → Test → Merge
    └── Reject → Back to queue with notes
```

### Supervisor's 3 Jobs
1. **Plan Triage** - Simple → approve, Complex → council
2. **Output QC** - Validate task outputs are safe and complete
3. **System Improvement Triage** - New platforms → approve, Architecture changes → council

### Council Role
- Deep quality control for complex decisions
- Not just approve/reject - suggests, refines, improves
- Teaches: Planner learns, PRD learns, Researcher learns

### Human Involvement (Minimal)
- Consultant (interactive PRD creation)
- System research council feedback (final say)
- Visual UI/UX approval
- Credit alerts

---

## Architecture Decisions

### 1. RPC: Generic + Validation
```go
func (db *DB) CallRPC(ctx context.Context, name string, params map[string]any) (json.RawMessage, error) {
    // Validate RPC name against allowlist (security)
    // Call the RPC
    // Return raw JSON (flexibility)
}
```
- Not 50 hardcoded functions
- One generic caller with safety
- Any new RPC = add to allowlist, no code change

### 2. Event Detection: Abstraction Layer
```go
type EventWatcher interface {
    Subscribe(ctx context.Context, table string, onChange func(Event)) error
}

// Supabase implementation uses real-time (free tier)
type SupabaseWatcher struct { ... }

// Future: PlanetScaleWatcher, PostgresWatcher, etc.
```
- Today: Supabase real-time
- Tomorrow: Swap implementation, interface stays same

### 3. Parallel: Goroutines + Config Limits
```json
{
  "max_concurrent_per_module": 8,
  "max_concurrent_total": 160,
  "agent_timeout_seconds": 300
}
```

### 4. Tool Output: JSON
```json
{
  "success": true,
  "result": { ... },
  "error": null
}
```

### 5. Tool Security Architecture
```
tools.json        → Defines tool (name, params, security level)
agents.json       → Assigns 2-3 tools per agent

Security Layers:
Layer 1: Agent only knows about its 2-3 tools (prompt doesn't mention others)
Layer 2: Runtime validates tool is in agent's allowed list
Layer 3: Tool validates parameters against schema
Layer 4: Tool implementation has internal safety checks
```

---

## What Stays (Tools)

| Module | Lines | What It Does |
|--------|-------|--------------|
| `gitree/` | 252 | Git operations (branch, commit, merge, delete) |
| `vault/` | 253 | Decrypt secrets from vault table |
| `maintenance/` | 750 | File ops, sandbox testing, backup/rollback |
| `security/` | 130 | Leak detection, output sanitization |
| `courier/` | 200 | Pluggable dispatch (config chooses driver) |
| `db/` | 300 | Generic query/RPC, connection from config |
| `runtime/` | 400 | Event detection, LLM session, tool parsing, parallel |
| `config/` | 150 | Load all config files, validate schema |

**Total: ~2,435 lines**

---

## What Gets Deleted

| Module | Lines | Why Delete |
|--------|-------|------------|
| `orchestrator/orchestrator.go` | 705 | Hardcoded routing logic → orchestrator.md |
| `dispatcher/dispatcher.go` | 573 | Hardcoded execution flow → runtime |
| `council/council.go` | 565 | Prompt building → council.md |
| `planner/planner.go` | 537 | Validation rules → planner.md |
| `consultant/consultant.go` | 351 | Prompt building → consultant.md |
| `analyst/analyst.go` | 438 | Analysis logic → researcher.md |
| `agent/agent.go` | 570 | Redundant with config + runtime |
| `supervisor/supervisor.go` | ~400 | Logic → supervisor.md |
| `pool/` | varies | Redundant |
| Most of `db/supabase.go` | ~1000 | 50 functions → generic caller |

**Deleted: ~5,800+ lines**

**Net: 8,287 → ~2,500 lines (70% reduction)**

---

## File Structure After

```
governor/
├── main.go                     (~50 lines)
├── config/
│   ├── loader.go              (~100 lines)
│   └── validate.go            (~50 lines)
├── runtime/
│   ├── events.go              (~100 lines) - EventWatcher interface + Supabase impl
│   ├── session.go             (~150 lines) - LLM session, tool parsing
│   ├── parallel.go            (~100 lines) - Goroutine pool with limits
│   └── tools.go               (~50 lines) - Tool execution with validation
├── db/
│   ├── db.go                  (~100 lines) - Generic interface
│   ├── supabase.go            (~100 lines) - Supabase driver
│   └── rpc.go                 (~100 lines) - Generic RPC with allowlist
├── gitree/
│   └── gitree.go              (252 lines - KEEP)
├── vault/
│   └── vault.go               (253 lines - KEEP)
├── maintenance/
│   └── maintenance.go         (~750 lines - KEEP)
├── courier/
│   ├── courier.go             (~100 lines - simplified interface)
│   └── github_actions.go      (~100 lines - driver)
└── security/
    └── leak.go                (130 lines - KEEP)
```

---

## Config Files Structure

```
config/
├── system.json        # Database, vault, git, runtime settings
├── agents.json        # Agent definitions with 2-3 tools each
├── models.json        # Available models (exists, keep pattern)
├── destinations.json  # Where to run (exists, keep pattern)
├── tools.json         # Tool definitions + schemas
└── prompts/
    ├── supervisor.md
    ├── council.md
    ├── orchestrator.md
    ├── planner.md
    ├── consultant.md
    ├── maintenance.md
    ├── researcher.md
    ├── vibes.md
    ├── courier.md
    └── ...
```

---

## Config File Examples

### system.json (NEW)
```json
{
  "database": {
    "type": "supabase",
    "url_env": "SUPABASE_URL",
    "key_env": "SUPABASE_KEY"
  },
  "vault": {
    "key_env": "VAULT_KEY",
    "table": "secrets_vault"
  },
  "git": {
    "host": "github",
    "repo_env": "GITHUB_REPO",
    "token_env": "GITHUB_TOKEN"
  },
  "runtime": {
    "max_concurrent_per_module": 8,
    "max_concurrent_total": 160,
    "event_poll_interval_ms": 1000,
    "agent_timeout_seconds": 300
  }
}
```

### tools.json (NEW)
```json
{
  "git_create_branch": {
    "description": "Create a new git branch",
    "parameters": {
      "name": {"type": "string", "required": true}
    },
    "security_level": "write",
    "implementation": "gitree.CreateBranch"
  },
  "git_read_branch": {
    "description": "Read files from a branch",
    "parameters": {
      "branch": {"type": "string", "required": true},
      "path": {"type": "string", "required": false}
    },
    "security_level": "read",
    "implementation": "gitree.ReadBranch"
  },
  "git_commit": {
    "description": "Commit changes to current branch",
    "parameters": {
      "message": {"type": "string", "required": true}
    },
    "security_level": "write",
    "implementation": "gitree.Commit"
  },
  "git_merge": {
    "description": "Merge branch to target",
    "parameters": {
      "source": {"type": "string", "required": true},
      "target": {"type": "string", "required": true}
    },
    "security_level": "write",
    "implementation": "gitree.Merge"
  },
  "db_query": {
    "description": "Query database table",
    "parameters": {
      "table": {"type": "string", "required": true},
      "columns": {"type": "array", "items": "string"},
      "where": {"type": "object"},
      "limit": {"type": "integer"}
    },
    "security_level": "read",
    "implementation": "db.Query"
  },
  "db_update": {
    "description": "Update database record",
    "parameters": {
      "table": {"type": "string", "required": true},
      "id": {"type": "string", "required": true},
      "data": {"type": "object", "required": true}
    },
    "security_level": "write",
    "implementation": "db.Update"
  },
  "db_rpc": {
    "description": "Call database RPC function",
    "parameters": {
      "name": {"type": "string", "required": true},
      "params": {"type": "object"}
    },
    "security_level": "write",
    "implementation": "db.CallRPC"
  },
  "file_read": {
    "description": "Read file from repository",
    "parameters": {
      "path": {"type": "string", "required": true}
    },
    "security_level": "read",
    "implementation": "maintenance.ReadFile"
  },
  "file_write": {
    "description": "Write file to repository",
    "parameters": {
      "path": {"type": "string", "required": true},
      "content": {"type": "string", "required": true}
    },
    "security_level": "write",
    "implementation": "maintenance.WriteFile"
  },
  "sandbox_test": {
    "description": "Run code in sandbox for testing",
    "parameters": {
      "files": {"type": "array", "items": "object"},
      "test_command": {"type": "string"}
    },
    "security_level": "execute",
    "implementation": "maintenance.SandboxTest"
  },
  "vault_get": {
    "description": "Get secret from vault",
    "parameters": {
      "key": {"type": "string", "required": true}
    },
    "security_level": "secret",
    "implementation": "vault.GetSecret"
  },
  "web_search": {
    "description": "Search the web",
    "parameters": {
      "query": {"type": "string", "required": true}
    },
    "security_level": "read",
    "implementation": "web.Search"
  },
  "command_maintenance": {
    "description": "Queue a command for maintenance agent",
    "parameters": {
      "command": {"type": "string", "required": true},
      "params": {"type": "object"}
    },
    "security_level": "write",
    "implementation": "db.QueueMaintenanceCommand"
  }
}
```

### agents.json (SIMPLIFIED)
```json
{
  "version": "2.0",
  "agents": {
    "supervisor": {
      "prompt": "prompts/supervisor.md",
      "tools": ["db_query", "db_update", "git_read_branch", "command_maintenance"],
      "default_destination": "opencode",
      "description": "Triage quality control - decides simple vs complex, validates outputs"
    },
    "council": {
      "prompt": "prompts/council.md",
      "tools": ["db_query", "git_read_branch", "web_search"],
      "default_destination": "gemini-api",
      "members": 3,
      "consensus_rounds": 4,
      "description": "Deep QC for complex decisions - reviews, suggests, refines"
    },
    "orchestrator": {
      "prompt": "prompts/orchestrator.md",
      "tools": ["db_query", "db_update"],
      "default_destination": "gemini-api",
      "description": "Routes tasks based on learned performance, tests new options"
    },
    "planner": {
      "prompt": "prompts/planner.md",
      "tools": ["db_query", "file_read", "git_read_branch"],
      "default_destination": "opencode",
      "description": "Breaks PRD into atomic tasks with prompt packets"
    },
    "consultant": {
      "prompt": "prompts/consultant.md",
      "tools": ["web_search"],
      "default_destination": "gemini-api",
      "description": "Interactive PRD creation with human"
    },
    "researcher": {
      "prompt": "prompts/researcher.md",
      "tools": ["web_search"],
      "default_destination": "gemini-api",
      "description": "Daily research for system improvements"
    },
    "maintenance": {
      "prompt": "prompts/maintenance.md",
      "tools": ["git_create_branch", "git_commit", "git_merge", "file_read", "file_write", "sandbox_test"],
      "default_destination": "opencode",
      "description": "Git operations and system implementation - ONLY agent with write access"
    },
    "vibes": {
      "prompt": "prompts/vibes.md",
      "tools": ["db_query", "web_search", "file_read"],
      "default_destination": "gemini-api",
      "description": "Human interface - status, ROI, recommendations"
    },
    "courier": {
      "prompt": "prompts/courier.md",
      "tools": [],
      "default_destination": "courier-github-actions",
      "description": "Web platform execution via browser automation"
    },
    "internal_cli": {
      "prompt": "prompts/internal_cli.md",
      "tools": ["file_read"],
      "default_destination": "opencode",
      "description": "CLI execution with codebase access"
    },
    "internal_api": {
      "prompt": "prompts/internal_api.md",
      "tools": ["file_read"],
      "default_destination": "gemini-api",
      "description": "API execution (emergency/special cases)"
    },
    "tester_code": {
      "prompt": "prompts/tester_code.md",
      "tools": ["file_read", "sandbox_test"],
      "default_destination": "opencode",
      "description": "Code validation - runs tests, lint, typecheck"
    }
  }
}
```

---

## Runtime Pseudo-Code

### main.go
```go
func main() {
    cfg := config.Load("./config")
    db := db.Connect(cfg.Database)
    tools := tools.Register(cfg.Tools)
    
    watcher := runtime.NewEventWatcher(cfg.Database)
    pool := runtime.NewAgentPool(cfg.Runtime)
    
    for {
        events := watcher.Poll()
        for _, event := range events {
            agent := cfg.GetAgentForEvent(event.Type)
            pool.Submit(agent, event, db, tools)
        }
    }
}
```

### runtime/session.go
```go
func RunAgentSession(agent AgentConfig, event Event, db *DB, tools ToolRegistry) {
    prompt := loadPrompt(agent.Prompt)
    destination := cfg.GetDestination(agent.Destination)
    
    session := NewLLMSession(destination, prompt, agent.Tools)
    input := buildInput(event)
    
    for {
        output := session.Call(input)
        toolCalls := parseToolCalls(output)
        
        if len(toolCalls) == 0 {
            db.SaveResult(event.ID, output)
            return
        }
        
        for _, call := range toolCalls {
            if !agent.HasTool(call.Name) {
                return Error(fmt.Sprintf("Tool %s not available to agent %s", call.Name, agent.ID))
            }
            result := tools.Execute(call.Name, call.Args)
            input = appendToolResult(input, call.Name, result)
        }
    }
}
```

### Tool Parsing
```
LLM outputs:
TOOL: git_create_branch {"name": "task/T001-profile"}

Go parses:
- tool name: git_create_branch
- args: {"name": "task/T001-profile"}

Validates:
- Is git_create_branch in agent's tools list? Yes/No
- Do args match schema? Yes/No
- Execute and return JSON result
```

---

## Implementation Phases

### Phase 1: Foundation (Keep/Simplify)
1. Keep `gitree/`, `vault/`, `maintenance/`, `security/` as-is
2. Create `config/` loader
3. Create `db/rpc.go` generic caller
4. Simplify `db/supabase.go` to driver only
5. Simplify `courier/` to pluggable interface

### Phase 2: Runtime
6. Create `runtime/events.go` - EventWatcher interface + Supabase real-time
7. Create `runtime/session.go` - LLM calling loop
8. Create `runtime/tools.go` - Tool parsing with security validation
9. Create `runtime/parallel.go` - Goroutine pool

### Phase 3: Config Files
10. Create `config/system.json`
11. Create `config/tools.json`
12. Simplify `config/agents.json`
13. Verify `config/models.json`, `config/destinations.json` work with new system

### Phase 4: Delete Old Code
14. Delete `orchestrator/`
15. Delete `dispatcher/`
16. Delete `council/`
17. Delete `planner/`
18. Delete `consultant/`
19. Delete `analyst/`
20. Delete `agent/`
21. Delete `supervisor/`
22. Delete `pool/`

### Phase 5: Wire Up
23. Create new `main.go`
24. Test each agent type
25. Verify dashboard still works (reads from DB, should be unchanged)
26. Full integration test

---

## Courier Decision

Pluggable driver pattern:

```json
// system.json
"courier": {
  "driver": "github_actions",
  "max_concurrent": 3,
  "timeout_seconds": 300,
  "github_actions": {
    "repo": "vibesTribe/courier-worker",
    "workflow": "browser-use.yml"
  }
}
```

Today: GitHub Actions (free 7GB, 3 concurrent)
Tomorrow: Change driver config, implement new driver interface

---

## Swapping Examples

### Swap Supabase for PlanetScale
```json
// system.json
"database": {
  "type": "planetscale",
  "connection_string_env": "DATABASE_URL"
}
```
Add `db/planetscale.go` implementing same interface.

### Swap GitHub for GitLab
```json
// system.json
"git": {"host": "gitlab", ...}
```
Add `gitree/gitlab.go` implementing same interface.

### Swap Kimi CLI (gone) for OpenCode
```json
// agents.json
"planner": {"default_destination": "opencode"}
```
No code change.

### Swap Go for Python
```
governor/ → governor.py
```
Same config files. Same DB schema. Same prompts.

---

## Key Files to Read

| File | Purpose |
|------|---------|
| `docs/prd_v1.4.md` | Complete system specification |
| `docs/SYSTEM_REFERENCE.md` | What we have and how it works |
| `CURRENT_STATE.md` | Where we are now |
| `config/agents.json` | Current agent definitions |
| `config/prompts/*.md` | Agent intelligence |

---

## Notes for Next Session

1. Dashboard reads from Supabase - should continue working unchanged
2. All prompts in `config/prompts/*.md` stay the same
3. Git operations already working in `gitree/`
4. Vault already working in `vault/`
5. This is a rebuild of the GOVERNOR (Go code), not the whole system
6. The "agent logic" moves from Go code to LLM prompts
7. Go becomes a thin runtime + tools layer

---

**END OF HANDOFF DOCUMENT**
