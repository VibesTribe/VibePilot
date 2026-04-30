# Go Governor Implementation Patterns

**Purpose:** Critical patterns learned from Python system. Read this BEFORE modifying Go code.

**Last Updated:** 2026-02-22
**Learned From:** Testing against real Supabase, analyzing Python orchestrator

---

## 1. SUPABASE: REST vs RPC

### The Critical Difference

| Operation | REST API | RPC Call |
|-----------|----------|----------|
| Simple query/select | ✅ Works | ✅ Works |
| Atomic claim | ❌ Race conditions | ✅ `FOR UPDATE SKIP LOCKED` |
| Increment counter | ❌ Can't do `"attempts + 1"` | ✅ Use RPC or fetch-then-update |
| Conditional update | ⚠️ Requires filter hacking | ✅ Built into function |

### WRONG: SQL Expressions via REST

```go
// THIS FAILS - Supabase REST treats "attempts + 1" as literal string
body := map[string]interface{}{
    "attempts": "attempts + 1",  // ❌ ERROR: invalid input syntax for type integer
}
db.request(ctx, "PATCH", "tasks?id=eq."+taskID, body)
```

### RIGHT: Fetch-Then-Update

```go
// 1. Fetch current value
task, _ := db.GetTaskByID(ctx, taskID)

// 2. Increment in Go
newAttempts := task.Attempts + 1

// 3. Write back
body := map[string]interface{}{
    "attempts": newAttempts,  // ✅ Integer value, not expression
}
db.request(ctx, "PATCH", "tasks?id=eq."+taskID, body)
```

### RIGHT: Use RPC for Atomic Operations

```go
// claim_next_task uses FOR UPDATE SKIP LOCKED - guarantees no double-claiming
func (d *DB) ClaimTaskRPC(ctx context.Context, courier, platform, modelID string) (string, error) {
    // POST to /rpc/claim_next_task
    body := map[string]interface{}{
        "p_courier":   courier,
        "p_platform":  platform,
        "p_model_id":  modelID,
    }
    data, err := d.request(ctx, "POST", "rpc/claim_next_task", body)
    // Returns task ID or null
}
```

---

## 2. TASK PACKET STRUCTURE

### The `prompt` Column is a JSON String

```sql
-- task_packets table
prompt TEXT NOT NULL  -- This is a JSON STRING, not columns
```

### WRONG: Direct Struct Unmarshal

```go
// If prompt is stored as JSON string, this fails
var packet PromptPacket
json.Unmarshal(data, &packet)  // packet.Prompt will be the whole JSON string
```

### RIGHT: Two-Step Parse

```go
type PromptPacket struct {
    TaskID      string                 `json:"task_id"`
    Prompt      string                 `json:"prompt"`      // The actual prompt text
    Title       string                 `json:"title"`
    Constraints *Constraints           `json:"constraints"`
}

type TaskPacketRow struct {
    TaskID      string `json:"task_id"`
    Prompt      string `json:"prompt"`      // This is a JSON STRING
    TechSpec    string `json:"tech_spec"`   // Also a JSON string
    Version     int    `json:"version"`
}

// Fetch row, then parse the prompt field
var rows []TaskPacketRow
json.Unmarshal(data, &rows)

var packet PromptPacket
json.Unmarshal([]byte(rows[0].Prompt), &packet)
// Now packet.Prompt contains actual prompt text
```

---

## 3. RUNNING OPENCODE

### OpenCode is a TUI Tool

The `opencode` command is an interactive TUI, not a batch CLI.

### WRONG: Made-Up Flags

```go
cmd := exec.Command("opencode", "--task-packet", packetJSON)
// ❌ --task-packet doesn't exist
```

### RIGHT: Use `run` Subcommand

```go
// opencode run --format json "the prompt text"
cmd := exec.CommandContext(ctx, "opencode", "run", "--format", "json", prompt)
output, err := cmd.CombinedOutput()

// Parse JSON output
var result struct {
    Content      string `json:"content"`
    InputTokens  int    `json:"input_tokens"`
    OutputTokens int    `json:"output_tokens"`
}
json.Unmarshal(output, &result)
```

### Context: We're Running INSIDE OpenCode

The Go Governor runs on GCE inside an opencode session. The `opencode` binary is available.

---

## 4. FAILURE HANDLING PATTERN

### From Python task_manager.py

```python
def handle_failure(self, task_id, reason, error_details=None, error_code=None):
    task = self.get_task(task_id)
    attempts = task.get("attempts", 0)
    max_attempts = task.get("max_attempts", 3)
    new_attempts = attempts + 1

    if new_attempts >= max_attempts:
        # Escalate - human review needed
        db.update({
            "status": "escalated",
            "attempts": new_attempts,
            "assigned_to": None,
            "failure_notes": notes,
        })
    else:
        # Return to queue
        db.update({
            "status": "available",
            "attempts": new_attempts,
            "assigned_to": None,
            "failure_notes": notes,
        })
```

### Key Points

1. **Fetch first** - Get current attempts
2. **Increment in code** - Not in SQL
3. **Set status** - `available` for retry, `escalated` when done
4. **Clear assigned_to** - So next runner can claim

---

## 5. TASK STATUS FLOW

```
                    ┌──────────────────────────────────────────┐
                    │                                          │
                    ▼                                          │
pending → available → in_progress → review → testing → approval → merged
              │            │                                     
              │            ├──────────────────────────┐          
              │            │                          │          
              │            ▼                          ▼          
              │        (success)                  (failure)      
              │            │                          │          
              │            ▼                          ▼          
              │        record run              attempts < max?   
              │            │                     │          │    
              │            ▼                    YES         NO   
              │     unlock dependents            │          │    
              │            │                     ▼          ▼    
              │            ▼              available    escalated
              │        (done)                                    
              │                                                   
              └────────────────────────────────────────────────────
```

### Status Definitions

| Status | Meaning | Next Action |
|--------|---------|-------------|
| `pending` | Created, has unmet dependencies | Wait for deps |
| `available` | Ready to claim | Sentry picks up |
| `in_progress` | Being executed | Wait for completion |
| `review` | Execution done, needs review | Supervisor reviews |
| `testing` | Review passed, needs tests | Tester runs tests |
| `approval` | Tests passed, needs approval | Human or auto-approve |
| `merged` | Complete | Done |
| `escalated` | Failed max attempts | Human intervention |

---

## 6. DB METHODS NEEDED IN GO

### Essential (Currently Missing or Broken)

```go
// Atomic claim via RPC
ClaimTaskRPC(ctx, courier, platform, modelID string) (taskID string, error)

// Fetch-then-update pattern
GetTaskByID(ctx, taskID string) (*Task, error)
UpdateTaskAttempts(ctx, taskID string, attempts int, status string) error

// Proper packet parsing
GetPromptPacket(ctx, taskID string) (*PromptPacket, error)  // Returns parsed prompt
```

### RPC Calls (Prefer Over REST)

| Operation | RPC Function |
|-----------|--------------|
| Claim task | `claim_next_task(p_courier, p_platform, p_model_id)` |
| Get available | `get_available_for_routing(can_web, can_internal, can_mcp)` |
| Unlock deps | `unlock_dependent_tasks(completed_task_id)` |
| Check deps | `check_dependencies_complete(task_id)` |

---

## 7. TESTING STRATEGY

### Tiny Test Task

```bash
# Create minimal test task
task_id=$(uuidgen)
sb.table('tasks').insert({
    'id': task_id,
    'title': 'TEST: Echo hello',
    'type': 'test',
    'status': 'available',
    'priority': 1,
    'routing_flag': 'internal',
})

sb.table('task_packets').insert({
    'task_id': task_id,
    'prompt': '{"task_id": "'$task_id'", "prompt": "Say: Hello World"}',
    'version': 1,
})
```

### Expected Flow

1. Sentry finds task (15s poll)
2. Dispatcher calls `ClaimTaskRPC` → gets task_id
3. Dispatcher fetches packet, parses prompt
4. Dispatcher runs `opencode run --format json "Say: Hello World"`
5. Dispatcher records task_run
6. Dispatcher updates task status → `review`
7. Dispatcher calls `unlock_dependent_tasks`

---

## 8. ANTI-PATTERNS TO AVOID

| Anti-Pattern | Why It Fails |
|--------------|--------------|
| `"attempts + 1"` in REST body | Supabase treats as string literal |
| `opencode --task-packet` | Flag doesn't exist |
| Direct unmarshal of prompt column | It's a JSON string inside JSON |
| REST PATCH for claim | Race conditions, double-claiming |
| Hardcoded model selection | Should come from DB active models |

---

## UPDATE LOG

| Date | Change |
|------|--------|
| 2026-02-22 | Created from Python analysis and Go testing failures |
