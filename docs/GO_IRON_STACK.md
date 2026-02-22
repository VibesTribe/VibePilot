# VibePilot Go Iron Stack Architecture

**Status:** PLANNING
**Created:** 2026-02-22
**Author:** GLM-5

---

## Executive Summary

Replace Python orchestrator with a Go-based "Governor" that:
- Uses 10-20MB RAM (vs 1.4GB+ current)
- Handles 100+ concurrent tasks via goroutines
- Offloads browser-use to GitHub Actions (7GB free runners)
- Deploys as a single binary with embedded UI
- Fits on e2-micro free tier

---

## 1. Why Go?

| Factor | Python (Current) | Go (Target) |
|--------|------------------|-------------|
| RAM | 150MB base + 1.4GB runner | 10-20MB total |
| Concurrency | GIL limits, process overhead | Goroutines (2KB each) |
| Deployment | venv + pip + drift | Single binary |
| Startup | Seconds | Milliseconds |
| Type safety | Runtime errors | Compile-time |
| Cross-compile | Complex | Native (GOOS/GOARCH) |

**We don't need ML libraries.** We need:
- Poll Supabase
- Dispatch to GitHub Actions / CLI tools
- Serve dashboard
- Manage concurrent tasks

All Go's sweet spot.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                     GOVERNOR (Go Binary)                            │
│  Target: 10-20MB RAM, e2-micro compatible                          │
├─────────────────────────────────────────────────────────────────────┤
│  Components:                                                        │
│  ├── Sentry (poller)     - Polls Supabase every 15s               │
│  ├── Dispatcher          - Routes tasks to runners                 │
│  ├── Janitor             - Resets stuck tasks (10min timeout)      │
│  ├── HTTP Server         - Dashboard + API endpoints               │
│  └── Embedded UI         - React dist/ via //go:embed              │
├─────────────────────────────────────────────────────────────────────┤
│  Concurrency:                                                       │
│  ├── Max 3 concurrent tasks (configurable)                         │
│  ├── Goroutine per task dispatch                                   │
│  └── Channel-based coordination                                    │
└─────────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   ┌───────────┐       ┌───────────────┐    ┌──────────────┐
   │ Supabase  │       │ GitHub        │    │ CLI Tools    │
   │           │       │ Actions       │    │ (local)      │
   │ - tasks   │       │               │    │              │
   │ - runs    │       │ - Couriers    │    │ - opencode   │
   │ - models  │       │ - 7GB RAM     │    │ - any CLI    │
   │ - config  │       │ - Free tier   │    │              │
   └───────────┘       └───────────────┘    └──────────────┘
```

---

## 3. Component Design

### 3.1 Sentry (Poller)

**Purpose:** Poll Supabase for ready tasks, drip-feed to dispatcher.

```go
type Sentry struct {
    db          *sql.DB
    pollInterval time.Duration  // 15 seconds
    maxInFlight  int            // 3 concurrent max
    dispatchCh   chan Task
}

func (s *Sentry) Run(ctx context.Context) {
    ticker := time.NewTicker(s.pollInterval)
    for {
        select {
        case <-ticker.C:
            tasks := s.pollReadyTasks()
            for _, task := range tasks {
                if s.inFlightCount() < s.maxInFlight {
                    s.dispatchCh <- task
                }
            }
        case <-ctx.Done():
            return
        }
    }
}

func (s *Sentry) pollReadyTasks() []Task {
    // Call RPC: get_available_for_routing(can_web=true, can_internal=true, can_mcp=false)
    // Filter: status='available', dependencies satisfied
    // Order by: priority ASC, created_at ASC
}
```

**Why poll vs webhooks:**
- No API bursts → no 429 rate limits
- Controlled drip-feed (max 3 at a time)
- Self-healing (stuck tasks auto-retry)

### 3.2 Dispatcher

**Purpose:** Route tasks to appropriate executor.

```go
type Dispatcher struct {
    ghClient    *github.Client
    localRunner *LocalRunner
    config      *Config
}

func (d *Dispatcher) Dispatch(task Task) error {
    switch task.RoutingFlag {
    case "web":
        // Dispatch to GitHub Actions
        return d.dispatchToGitHub(task)
    case "internal":
        // Run locally via CLI
        return d.dispatchLocal(task)
    case "mcp":
        // Future: MCP-connected runners
        return ErrNotImplemented
    }
}

func (d *Dispatcher) dispatchToGitHub(task Task) error {
    // Create workflow dispatch event
    // One branch per task ID: task/{task_id}
    // Workflow: .github/workflows/courier.yml
    payload := map[string]interface{}{
        "task_id":     task.ID,
        "prompt":      task.PromptPacket.Prompt,
        "platform":    task.AssignedTo,
        "branch_name": fmt.Sprintf("task/%s", task.ID),
    }
    return d.ghClient.Repositories.CreateDispatchEvent(ctx, owner, repo, "courier", payload)
}

func (d *Dispatcher) dispatchLocal(task Task) error {
    // Spawn CLI tool (opencode, etc.)
    // Capture output, update Supabase
    return d.localRunner.Run(task)
}
```

### 3.3 Janitor

**Purpose:** Reset stuck tasks, cleanup old branches.

```go
type Janitor struct {
    db          *sql.DB
    ghClient    *github.Client
    stuckTimeout time.Duration  // 10 minutes
}

func (j *Janitor) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            j.resetStuckTasks()
            j.cleanupMergedBranches()
        case <-ctx.Done():
            return
        }
    }
}

func (j *Janitor) resetStuckTasks() {
    // Find tasks where:
    //   status = 'in_progress'
    //   AND updated_at < NOW() - 10 minutes
    // Reset to 'available', increment attempts
}

func (j *Janitor) cleanupMergedBranches() {
    // Find branches matching task/* where task is 'merged'
    // Delete branch via GitHub API
}
```

### 3.4 HTTP Server + Embedded UI

```go
//go:embed dist
var staticFS embed.FS

func (s *Server) Start(addr string) error {
    // API endpoints
    http.HandleFunc("/api/tasks", s.handleTasks)
    http.HandleFunc("/api/task/{id}", s.handleTask)
    http.HandleFunc("/api/models", s.handleModels)
    http.HandleFunc("/api/platforms", s.handlePlatforms)
    http.HandleFunc("/api/roi", s.handleROI)
    
    // WebSocket for real-time updates
    http.HandleFunc("/ws", s.handleWebSocket)
    
    // Serve embedded React UI
    http.Handle("/", http.FileServer(http.FS(staticFS)))
    
    return http.ListenAndServe(addr, nil)
}
```

---

## 4. GitHub Actions Courier Workflow

**File:** `.github/workflows/courier.yml`

```yaml
name: Courier Task

on:
  repository_dispatch:
    types: [courier]

jobs:
  execute:
    runs-on: ubuntu-latest  # 7GB RAM, free tier
    
    steps:
      - name: Checkout task branch
        uses: actions/checkout@v4
        with:
          ref: ${{ github.event.client_payload.branch_name }}
      
      - name: Setup browser
        uses: browser-actions/setup-chrome@v1
      
      - name: Execute courier task
        env:
          TASK_ID: ${{ github.event.client_payload.task_id }}
          PROMPT: ${{ github.event.client_payload.prompt }}
          PLATFORM: ${{ github.event.client_payload.platform }}
          SUPABASE_URL: ${{ secrets.SUPABASE_URL }}
          SUPABASE_KEY: ${{ secrets.SUPABASE_SERVICE_KEY }}
        run: |
          # Run courier (Python or Go binary)
          python scripts/courier_execute.py \
            --task-id $TASK_ID \
            --prompt "$PROMPT" \
            --platform $PLATFORM
      
      - name: Commit results
        run: |
          git config user.name "VibePilot Courier"
          git config user.email "courier@vibepilot.ai"
          git add -A
          git commit -m "Task $TASK_ID: execution results"
          git push
```

---

## 5. Task Branch Strategy

**One branch per task ID:**

```
main
├── task/uuid-001  (Task #1 working branch)
├── task/uuid-002  (Task #2 working branch)
├── task/uuid-003  (Task #3 working branch)
└── ...
```

**Lifecycle:**

1. **Dispatch** → Create branch `task/{task_id}` from `main`
2. **Execute** → Courier works in branch, commits results
3. **Review** → Supervisor reviews in Supabase
4. **Merge** → Maintenance agent merges to `main`
5. **Cleanup** → Branch auto-deleted (GitHub setting)

**Benefits:**
- Isolation: Tasks never conflict
- Audit trail: Every task's output is a commit
- Auto-cleanup: GitHub deletes merged branches

---

## 6. Data Flow

### Task Execution Flow

```
1. Sentry polls Supabase (every 15s)
   ↓
2. Finds task with status='available', deps satisfied
   ↓
3. Dispatcher checks routing_flag:
   ├── 'web' → GitHub Actions (courier)
   └── 'internal' → Local CLI tool
   ↓
4. Task status → 'in_progress'
   ↓
5. Executor runs task:
   ├── Courier: browser-use on GitHub, commits to task branch
   └── Internal: CLI tool, returns output
   ↓
6. Result written to Supabase (task_runs table)
   ↓
7. Task status → 'review' (awaiting supervisor)
   ↓
8. Supervisor reviews → 'testing' → 'approval' → 'merged'
   ↓
9. Janitor cleans up branch
```

### Dependencies Flow

```
1. Task A has dependency on Task B
   ↓
2. Task A created with status='locked'
   ↓
3. Task B completes (status='merged')
   ↓
4. Janitor calls unlock_dependent_tasks RPC
   ↓
5. Task A status → 'available' (ready for dispatch)
```

---

## 7. Configuration

**File:** `governor.yaml`

```yaml
governor:
  poll_interval: 15s
  max_concurrent: 3
  stuck_timeout: 10m

supabase:
  url: ${SUPABASE_URL}
  service_key: ${SUPABASE_SERVICE_KEY}

github:
  token: ${GITHUB_TOKEN}
  owner: ${GITHUB_OWNER}
  repo: ${GITHUB_REPO}
  workflow: courier.yml

runners:
  internal:
    - id: opencode
      command: opencode
      ram_limit_mb: 500
      
courier:
  driver_model: gemini-2.0-flash
  platforms:
    - id: chatgpt
      url: https://chat.openai.com
      daily_limit: 40
    - id: claude
      url: https://claude.ai
      daily_limit: 10
    - id: gemini
      url: https://gemini.google.com
      daily_limit: 100

server:
  addr: :8080
  dashboard_dist: ./dist
```

---

## 8. Claw Patterns to Adopt

These patterns come from researching ZeroClaw, NanoClaw, and IronClaw frameworks. They're proven in production and directly applicable to Go.

### 8.1 From ZeroClaw (Rust, 8.8MB binary)

**Provider Interface (Config-Driven Swapping)**
```go
// internal/provider/provider.go
type Provider interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    SupportsNativeTools() bool
}

// Config selects provider at runtime
type ProviderConfig struct {
    Type string `json:"type"` // "anthropic", "openai", "custom"
    URL  string `json:"url"`  // For custom providers
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
    switch cfg.Type {
    case "anthropic":
        return NewAnthropicProvider(cfg)
    case "openai":
        return NewOpenAIProvider(cfg)
    case "custom":
        return NewCustomProvider(cfg.URL)
    }
}
```

**Hot-Config Reload**
```go
// internal/config/watcher.go
func (w *ConfigWatcher) Start() {
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add("config/")
    
    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                w.reloadConfig()
            }
        }
    }
}

func (w *ConfigWatcher) reloadConfig() {
    newCfg := LoadConfig("config/")
    w.onReload(newCfg) // Callback to update running components
}
```

**Tool Registry (Self-Describing)**
```go
// internal/tools/registry.go
type Tool struct {
    Name        string
    Description string
    InputSchema json.RawMessage
    Handler     func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

var registry = make(map[string]Tool)

func Register(t Tool) {
    registry[t.Name] = t
}

func GetToolDescriptions() []map[string]interface{} {
    // Returns tool definitions in format LLM understands
    var descs []map[string]interface{}
    for _, t := range registry {
        descs = append(descs, map[string]interface{}{
            "name":        t.Name,
            "description": t.Description,
            "input_schema": t.InputSchema,
        })
    }
    return descs
}
```

### 8.2 From NanoClaw (TypeScript, ~4k lines)

**One File Per Concern**
```
internal/
├── sentry/sentry.go      # ONLY polling logic
├── dispatcher/dispatcher.go  # ONLY routing logic
├── janitor/janitor.go    # ONLY cleanup logic
├── config/config.go      # ONLY config loading
├── db/db.go              # ONLY database operations
└── types/types.go        # ONLY type definitions
```

**No ORM, Direct SQL**
```go
// internal/db/db.go
func GetAvailableTasks(db *sql.DB) ([]Task, error) {
    query := `
        SELECT id, title, priority, routing_flag
        FROM tasks
        WHERE status = 'available'
        ORDER BY priority ASC, created_at ASC
        LIMIT 10
    `
    rows, err := db.Query(query)
    // ... direct scan into structs
}
```

**Keep It Small (<4k lines target)**
- No abstraction layers (no factories, managers, repositories)
- Single responsibility per file
- Prefer switch statements over interfaces where simple
- Inline what doesn't need abstraction

### 8.3 From IronClaw (Security-Focused)

**Leak Detection (Output Scanning)**
```go
// internal/security/leak_detector.go
type LeakDetector struct {
    patterns []leakPattern
}

type leakPattern struct {
    name    string
    regex   *regexp.Regexp
    action  string // "block", "redact", "warn"
}

var defaultPatterns = []leakPattern{
    {name: "openai_key", regex: regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`), action: "block"},
    {name: "github_token", regex: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`), action: "block"},
    {name: "supabase_key", regex: regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`), action: "block"},
    {name: "aws_key", regex: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), action: "block"},
}

func (d *LeakDetector) Scan(output string) (string, []LeakWarning) {
    var warnings []LeakWarning
    for _, p := range d.patterns {
        if p.regex.MatchString(output) {
            warnings = append(warnings, LeakWarning{
                Pattern: p.name,
                Action:  p.action,
            })
            if p.action == "block" {
                return "", warnings // Block output entirely
            }
            if p.action == "redact" {
                output = p.regex.ReplaceAllString(output, "[REDACTED]")
            }
        }
    }
    return output, warnings
}
```

**Credential Injection at Boundary**
```go
// Secrets NEVER in config or context
// Injected only at execution time

type SecureExecutor struct {
    vault *VaultClient
}

func (e *SecureExecutor) ExecuteCourier(task Task) error {
    // Get credentials JUST before use
    apiKey, err := e.vault.GetSecret("GEMINI_API_KEY")
    if err != nil {
        return err
    }
    
    // Inject into environment, not into task context
    cmd := exec.Command("courier", "--task-id", task.ID)
    cmd.Env = append(os.Environ(), fmt.Sprintf("GEMINI_API_KEY=%s", apiKey))
    
    return cmd.Run()
}
```

**HTTP Allowlist Validation**
```go
// internal/security/allowlist.go
type HTTPAllowlist struct {
    allowedHosts map[string]bool
}

var defaultAllowlist = []string{
    "api.supabase.co",
    "api.github.com",
    "api.anthropic.com",
    "api.openai.com",
}

func (a *HTTPAllowlist) ValidateURL(urlStr string) error {
    u, err := url.Parse(urlStr)
    if err != nil {
        return err
    }
    if !a.allowedHosts[u.Host] {
        return fmt.Errorf("host %s not in allowlist", u.Host)
    }
    return nil
}
```

### 8.4 Summary: Claw Patterns in Go Governor

| Pattern | Source | Go Implementation |
|---------|--------|-------------------|
| Config-driven providers | ZeroClaw | Provider interface + config selection |
| Hot-config reload | ZeroClaw | fsnotify watcher |
| Tool registry | ZeroClaw | Self-describing tools map |
| 10-20MB footprint | ZeroClaw | Single binary, minimal deps |
| 1 file per concern | NanoClaw | Clean package structure |
| Direct SQL | NanoClaw | database/sql, no ORM |
| <4k lines | NanoClaw | No abstraction bloat |
| Leak detection | IronClaw | Regex scanner on outputs |
| Credential injection | IronClaw | Vault at execution boundary |
| HTTP allowlist | IronClaw | URL validation before requests |

---

## 9. Project Structure

```
vibepilot-go/
├── cmd/
│   └── governor/
│       └── main.go           # Entry point
├── internal/
│   ├── sentry/
│   │   └── sentry.go         # Supabase poller
│   ├── dispatcher/
│   │   ├── dispatcher.go     # Task routing
│   │   ├── github.go         # GitHub Actions dispatch
│   │   └── local.go          # Local CLI runner
│   ├── janitor/
│   │   └── janitor.go        # Stuck task reset, cleanup
│   ├── server/
│   │   ├── server.go         # HTTP server
│   │   ├── api.go            # REST endpoints
│   │   └── websocket.go      # Real-time updates
│   ├── config/
│   │   ├── config.go         # Config loading
│   │   └── watcher.go        # Hot-reload (ZeroClaw pattern)
│   ├── provider/
│   │   ├── provider.go       # Provider interface (ZeroClaw pattern)
│   │   ├── anthropic.go      # Anthropic implementation
│   │   ├── openai.go         # OpenAI implementation
│   │   └── custom.go         # Custom endpoint
│   ├── tools/
│   │   ├── registry.go       # Tool registry (ZeroClaw pattern)
│   │   └── tools.go          # Built-in tools
│   ├── security/
│   │   ├── leak_detector.go  # Output scanning (IronClaw pattern)
│   │   ├── allowlist.go      # HTTP allowlist (IronClaw pattern)
│   │   └── vault.go          # Credential injection (IronClaw pattern)
│   └── db/
│       └── supabase.go       # Direct SQL, no ORM (NanoClaw pattern)
├── pkg/
│   └── types/
│       └── types.go          # Shared types (Task, Result, etc.)
├── dist/                     # React UI build output (embedded)
├── go.mod
├── go.sum
├── governor.yaml             # Configuration
└── Makefile
```

---

## 10. Migration Plan

### Phase 1: Foundation (1 session)
- [ ] Go project scaffold (same repo, new branch: `go-governor`)
- [ ] Supabase client (direct SQL, no ORM)
- [ ] Sentry poller
- [ ] Basic dispatcher
- [ ] Config loading + hot-reload watcher

### Phase 2: GitHub Integration (1 session)
- [ ] GitHub Actions dispatch
- [ ] Courier workflow file
- [ ] Branch management (one branch per task)
- [ ] Janitor cleanup
- [ ] Security: leak detector + allowlist

### Phase 3: HTTP Server (1 session)
- [ ] REST API endpoints
- [ ] WebSocket for real-time
- [ ] Embedded UI (//go:embed dist/)
- [ ] Dashboard integration
- [ ] Provider interface for future LLM swaps

### Phase 4: Cutover (1 session)
- [ ] Parallel run with Python
- [ ] Verify task execution
- [ ] Verify Claw patterns (leak detection, credential injection)
- [ ] Switch to Go Governor
- [ ] Merge go-governor branch to main
- [ ] Decommission Python orchestrator

---

## 11. What Stays the Same

| Component | Change? | Notes |
|-----------|---------|-------|
| Supabase schema | NO | No migrations needed |
| Config files (models.json, etc.) | NO | Same JSON, Go reads them |
| Agent prompts | NO | Preserved in config/prompts/ |
| Architecture docs | NO | prd, philosophy, etc. preserved |
| Dashboard | NO | vibeflow repo untouched |
| Runner contract | NO | Same input/output schema |

## 12. What Changes

| Component | Before | After |
|-----------|--------|-------|
| Orchestrator | Python (99MB+1.4GB) | Go Governor (10-20MB) |
| Browser execution | Local (RAM bottleneck) | GitHub Actions (7GB free) |
| Task dispatch | Webhooks (429 risk) | Poll-based (15s, controlled) |
| Deployment | venv + pip | Single binary |
| Branch management | Manual | One branch per task, auto-cleanup |

---

## 13. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Go learning curve | GLM-5 is proficient, patterns are clear |
| GitHub Actions limits | 2000 min/month free, ample for parallel couriers |
| Breaking dashboard | Dashboard untouched, same Supabase data |
| Task loss during cutover | Parallel run, verify before switch |
| Config drift | Same JSON files, Go just reads them |

---

## 14. Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| RAM usage | 1.4GB+ | <50MB |
| GCE cost | $64/mo | $0 (free tier) |
| Concurrent tasks | 1-2 | 3+ (configurable) |
| Deployment time | Minutes | Seconds |
| Startup time | Seconds | Milliseconds |

---

## Next Steps

1. **Human confirms this architecture**
2. Create Go project scaffold
3. Implement Sentry (poller)
4. Implement Dispatcher
5. Test with Supabase
6. Add GitHub Actions integration
7. Add HTTP server + embedded UI
8. Parallel run with Python
9. Cutover to Go Governor
10. Decommission Python orchestrator

---

**This document is the blueprint. Do not deviate without human approval.**
