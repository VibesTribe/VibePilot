# Governor Implementation Handoff Document

**Session:** 2026-02-23
**Purpose:** Full understanding captured for next session
**Status:** Phase 4 COMPLETE - Full orchestrator architecture working

---

## QUICK START

```bash
cd ~/vibepilot/governor
go build -o governor ./cmd/governor
./governor
```

**Expected output:**
```
VibePilot Governor dev starting...
Poll interval: 15s, Max concurrent: 3, Max per module: 8
Connected to Supabase
Sentry started: polling every 15s, max 3 concurrent, 8 per module
Dispatcher started
Orchestrator started
Janitor started: stuck timeout 10m0s
Server starting on :8080
```

---

## CURRENT STATE

### Code Stats

| Metric | Value | Target |
|--------|-------|--------|
| Total Go lines | ~2,700 | < 4,000 |
| Binary size | ~10MB | < 15MB |
| Dependencies | 3 direct | Minimal |
| Packages | 17 | Lean |

### What's Working

| Component | Status | File |
|-----------|--------|------|
| Sentry (poller) | ✅ | `sentry/sentry.go` |
| Dispatcher (router) | ✅ | `dispatcher/dispatcher.go` |
| Janitor (cleanup) | ✅ | `janitor/janitor.go` |
| DB client | ✅ | `db/supabase.go` |
| Pool (model selection) | ✅ | `pool/model_pool.go` |
| Security (leak detector) | ✅ | `security/leak_detector.go` |
| Module limiter | ✅ | `throttle/module_limiter.go` |
| Courier dispatcher | ✅ | `courier/dispatcher.go` |
| Courier webhook | ✅ | `courier/webhook.go` |
| WebSocket hub | ✅ | `server/hub.go` |
| HTTP server + API | ✅ | `server/server.go` |
| **Supervisor** | ✅ | `supervisor/supervisor.go` |
| **Orchestrator** | ✅ | `orchestrator/orchestrator.go` |
| **Maintenance** | ✅ | `maintenance/maintenance.go` |
| **Tester** | ✅ | `tester/tester.go` |

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/api/stats` | GET | Quick stats overview |
| `/api/tasks?status={status}` | GET | List tasks (default: available) |
| `/api/task/{id}` | GET | Get task packet |
| `/api/models` | GET | List active runners from DB |
| `/api/platforms` | GET | List web platforms |
| `/api/roi` | GET | ROI summary from task_runs |
| `/api/limits` | GET | Per-module concurrent counts |
| `/ws` | WS | Real-time task events |
| `/webhook/courier` | POST | GitHub Actions callback |

---

## ARCHITECTURE

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        GO GOVERNOR                                   │
├─────────────────────────────────────────────────────────────────────┤
│  SENTRY (poller)                                                     │
│  - Polls every 15s                                                   │
│  - Max 3 concurrent global                                           │
│  - Max 8 per module (slice_id)                                       │
│  - Acquires slot from ModuleLimiter                                  │
│  - Sends to dispatch channel                                         │
├─────────────────────────────────────────────────────────────────────┤
│  DISPATCHER                                                          │
│  - Receives task from channel                                        │
│  - Checks routing_flag:                                              │
│    - 'web' → Courier (GitHub Actions)                                │
│    - 'internal' → Pool → Local CLI                                   │
│  - Claims task, executes, records result                             │
│  - Releases slot on completion/failure                               │
├─────────────────────────────────────────────────────────────────────┤
│  COURIER DISPATCHER (Phase 2)                                        │
│  - Enqueues web tasks                                                │
│  - Max 3 in-flight, 30s stagger                                      │
│  - Dispatches to GitHub Actions                                      │
│  - Webhook receives completion                                       │
├─────────────────────────────────────────────────────────────────────┤
│  JANITOR                                                             │
│  - Resets stuck tasks (10min timeout)                                │
│  - Calls refresh_limits RPC                                          │
├─────────────────────────────────────────────────────────────────────┤
│  SERVER (Phase 3)                                                    │
│  - HTTP API endpoints                                                │
│  - WebSocket hub for real-time updates                               │
│  - Courier webhook handler                                           │
└─────────────────────────────────────────────────────────────────────┘
```

### Module Limiter Flow

```
Sentry.poll()
    ↓
Check: CanDispatch(slice_id)?
    ├── YES → Acquire(slice_id) → Dispatch
    └── NO → Skip task (module at capacity)

Dispatcher.execute()
    ↓
... task runs ...
    ↓
releaseSlot(slice_id) on ANY exit:
- Success
- Failure
- Claim failed
- No runner
- No packet
```

---

## KEY PATTERNS

### 1. Module Limiter (8 per slice)

```go
// throttle/module_limiter.go
type ModuleLimiter struct {
    maxPerModule int           // 8
    active       map[string]int // slice_id -> count
}

// Acquire before dispatch
if !limiter.Acquire(task.SliceID) {
    continue // Skip, at capacity
}

// Release on any exit path
defer limiter.Release(task.SliceID)
```

### 2. Courier Dispatch (3 concurrent, staggered)

```go
// courier/dispatcher.go
type Dispatcher struct {
    queue       chan Task
    maxInFlight int           // 3
    stagger     time.Duration // 30s
}

// Stagger to avoid GitHub rate limits
time.Sleep(d.stagger)
d.client.Repositories.Dispatch(ctx, owner, repo, dispatch)
```

### 3. WebSocket Hub

```go
// server/hub.go
type Hub struct {
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

// Single goroutine owns the map (no mutex)
func (h *Hub) Run() {
    for {
        select {
        case msg := <-h.broadcast:
            for client := range h.clients {
                client.send <- msg
            }
        }
    }
}
```

### 4. Slot Release on All Paths

```go
// dispatcher/dispatcher.go
func (d *Dispatcher) execute(ctx context.Context, task Task) {
    // Every error path must release slot
    if err != nil {
        d.releaseSlot(task.SliceID)
        return
    }
    
    // Success path also releases
    d.releaseSlot(task.SliceID)
}
```

---

## FILES STRUCTURE

```
governor/
├── cmd/governor/main.go          # Entry point
├── internal/
│   ├── config/config.go          # Config loading
│   ├── db/supabase.go            # REST client + RPC calls
│   ├── sentry/sentry.go          # Task poller
│   ├── dispatcher/dispatcher.go  # Task router + executor
│   ├── janitor/janitor.go        # Stuck task cleanup
│   ├── pool/model_pool.go        # Runner selection
│   ├── security/leak_detector.go # Secret scanning
│   ├── throttle/
│   │   └── module_limiter.go     # 8 per module enforcement
│   ├── courier/
│   │   ├── dispatcher.go         # GitHub Actions dispatch
│   │   └── webhook.go            # Completion callback
│   └── server/
│       ├── server.go             # HTTP API
│       └── hub.go                # WebSocket hub
├── pkg/types/types.go            # Shared types
├── governor.yaml.example         # Sample config
└── go.mod                        # Dependencies
```

---

## CONFIGURATION

```yaml
# governor.yaml
governor:
  poll_interval: 15s
  max_concurrent: 3
  stuck_timeout: 10m
  max_per_module: 8

supabase:
  url: ${SUPABASE_URL}
  service_key: ${SUPABASE_SERVICE_KEY}

github:
  token: ${GITHUB_TOKEN}
  owner: your-username
  repo: vibepilot
  workflow: courier.yml

courier:
  enabled: false  # Set true when GitHub secrets configured
  max_in_flight: 3
  stagger: 30s
  callback_url: http://your-server:8080/webhook/courier

server:
  addr: :8080

security:
  allowed_hosts:
    - api.supabase.co
    - api.github.com
```

---

## GITHUB ACTIONS WORKFLOW

**File:** `.github/workflows/courier.yml`

Triggers on `repository_dispatch` event type `courier_task`:
1. Checks out task branch
2. Sets up Python + browser-use
3. Executes browser automation with Gemini
4. Posts result to callback URL

---

## WHAT'S NOT YET IMPLEMENTED

| Feature | Phase | Notes |
|---------|-------|-------|
| Council deliberation | 5 | Multi-lens review (supervisor/council/council.go) |
| Command queue polling | 5 | Maintenance polls maintenance_commands table |
| Config hot-reload | 5 | fsnotify watcher |
| Embedded React UI | Future | //go:embed dist/ |
| Vibes interface | Future | Human chat interface |
| MCP server | Future | External tool access |

---

## SUPERVISOR (Implemented)

### 4 Actions Only

| Action | When | What Happens |
|--------|------|--------------|
| **APPROVE** | Output OK, all deliverables present | Route to Tester (or merge if test/docs type) |
| **REJECT** | Missing deliverables, quality issues, secrets | Return to queue with notes: WHY | ISSUES | SUGGESTION |
| **COUNCIL** | Security, auth, architecture, refactor, priority 1 | Route to Council → Human reviews recommendations |
| **HUMAN** | Visual/ui_ux changes, council recommendations | Set status `awaiting_human` |

### Quality Checks

- All deliverables created?
- Scope creep detected (extra files)?
- Secrets detected (sk-, ghp_, AKIA, password literals)?
- Code quality warnings (TODO, FIXME, print statements)

### NeedsCouncil() Logic

Triggers Council review when:
- Task type is "security"
- Title contains: auth, authentication, architecture, refactor
- Priority <= 1 (critical tasks)

---

## MAINTENANCE (Implemented)

### Operations

| Operation | Function | Notes |
|-----------|----------|-------|
| Create branch | `CreateBranch()` | Creates from main, pushes to origin |
| Commit output | `CommitOutput()` | Writes files, task_output.txt, commits, pushes |
| Read output | `ReadBranchOutput()` | Returns list of changed files |
| Merge branch | `MergeBranch()` | Merges task branch to target |
| Delete branch | `DeleteBranch()` | Deletes local and remote |

### Output Handling

- If result has `files` key → writes each file
- If result has `output` key → writes task_output.txt
- If nothing to commit → returns error (task produced no output)

---

## ORCHESTRATOR (Implemented)

### Flow

```
Task completes → OnTaskComplete()
     ↓
Create branch (if needed)
     ↓
Commit output to branch
     ↓
processSupervisorDecision()
     ↓
APPROVE → testing (or merge if test/docs)
REJECT → back to queue with notes
COUNCIL → awaiting_human (pending implementation)
HUMAN → awaiting_human
     ↓
Merge → merged (or awaiting_human if conflict)
```

---

## PYTHON INTEGRATION

### What Go Replaces (DONE)

| Python | Go |
|--------|-----|
| `core/orchestrator.py` (task routing) | `dispatcher/`, `orchestrator/` |
| `task_manager.py` (claim/record) | `db/supabase.go` |
| `agents/supervisor.py` | `supervisor/supervisor.go` |
| `agents/maintenance.py` | `maintenance/maintenance.go` |
| `agents/executioner.py` | `tester/tester.go` |
| Polling loop | `sentry/sentry.go` |

### What Stays Python

| Component | Why |
|-----------|-----|
| `agents/planner.py` | Task planning from PRD |
| `agents/consultant.py` | PRD generation |
| `agents/council.py` | Multi-lens deliberation (Phase 5) |
| `runners/contract_runners.py` | Runner implementations |

---

## TESTING

### Manual Test

```bash
# Create test task
python -c "
import os, json, uuid
from supabase import create_client
sb = create_client(os.environ['SUPABASE_URL'], os.environ['SUPABASE_SERVICE_KEY'])
task_id = str(uuid.uuid4())
sb.table('tasks').insert({
    'id': task_id,
    'title': 'Test: Echo Hello',
    'status': 'available',
    'priority': 1,
    'routing_flag': 'internal',
    'slice_id': 'test-slice'
}).execute()
sb.table('task_packets').insert({
    'task_id': task_id,
    'prompt': json.dumps({'prompt': 'Echo: Hello World'}),
    'version': 1
}).execute()
print(f'Created task {task_id}')
"

# Run governor
./governor

# Watch logs for:
# - Sentry: dispatching task
# - Dispatcher: selected runner
# - Dispatcher: task completed successfully
```

### Verify in Supabase

```sql
SELECT id, status, assigned_to FROM tasks WHERE title LIKE 'Test:%';
SELECT * FROM task_runs ORDER BY created_at DESC LIMIT 1;
```

---

## COMMON ISSUES

### "no runner available"
- Check `runners` table has active runners
- Run `python scripts/admin_setup.py --create-runners`

### "claim failed"
- Task may have been claimed by another process
- Check `status` is `available` before dispatch

### "module at capacity"
- 8 tasks already running in that slice
- Wait for completion or increase `max_per_module`

### "courier not configured"
- Set `courier.enabled: true` in config
- Set `GITHUB_TOKEN` environment variable

---

## NEXT PHASES

### Phase 4: Parallel Run
1. Run Go and Python together
2. Go marks tasks with `claimed_by='governor'`
3. Compare 50+ task outcomes
4. Verify ROI calculations match
5. Switch to Go-only

### Phase 5: Supervisor + Council
1. Add `supervisor/reviewer.go`
2. Add `council/deliberation.go`
3. Config-driven prompts (load from files)
4. Hot-reload with fsnotify

### Future
- Vibes interface (human chat)
- MCP server (external access)
- Voice interface

---

## FILES TO READ NEXT SESSION

1. This document: `docs/GOVERNOR_HANDOFF.md`
2. Current state: `CURRENT_STATE.md`
3. System reference: `docs/SYSTEM_REFERENCE.md`
4. Core philosophy: `docs/core_philosophy.md`
5. Full PRD: `docs/prd_v1.4.md`

---

## SESSION HISTORY

| Date | Phase | Changes |
|------|-------|---------|
| 2026-02-22 | 1 | Initial Sentry, Dispatcher, Janitor |
| 2026-02-22 | 2 | Intelligent routing, model pool |
| 2026-02-22 | 3 | Pool-based runner selection |
| 2026-02-23 | 2 | Module limiter (8/slice), Courier dispatch |
| 2026-02-23 | 3 | WebSocket hub, Real API endpoints, Health check |

---

**END OF HANDOFF**
