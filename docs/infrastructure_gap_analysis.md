# Infrastructure Gap Analysis

**Created:** 2026-02-19
**Session:** 16
**Status:** Ready for Kimi Build

---

## Reference Document

`docs/vibepilot_process.md` - Complete system flow, approved by human

---

## WHAT EXISTS

| Component | Location | Status |
|-----------|----------|--------|
| Orchestrator core | `core/orchestrator.py` | ✅ Has cooldown, 80% threshold, concurrent execution |
| Supervisor agent | `agents/supervisor.py` | ⚠️ Exists but conflates decide + execute |
| Maintenance agent | `agents/council/maintenance.py` | ⚠️ Exists for council role, not git operator |
| Runners | `runners/` | ✅ base_runner, kimi_runner, api_runner exist |
| Runner contracts | `runners/contract_runners.py` | ✅ Contract definition exists |
| Prompts | `prompts/`, `config/prompts/` | ⚠️ Exist but need updates per role_logic_review.md |
| Supabase tasks table | `docs/supabase-schema/schema_v1_core.sql` | ✅ Working |
| Dependency RPCs | `docs/supabase-schema/005_dependencies_jsonb.sql` | ✅ Working |
| Systemd service | `scripts/vibepilot-orchestrator.service` | ✅ File exists, not installed |
| Dashboard | Separate repo (vibeflow) | ✅ Live at vercel |
| PRD/Plan doc structure | `docs/prd/`, `docs/plans/` | ✅ Directory structure exists |

---

## WHAT'S MISSING

### 1. maintenance_commands Table (NEW)

**What:** Supabase table for Supervisor → Maintenance commands

**Schema:**
```sql
CREATE TABLE maintenance_commands (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  command_type TEXT NOT NULL CHECK (command_type IN (
    'create_branch', 
    'commit_code', 
    'merge_branch', 
    'delete_branch',
    'tag_release'
  )),
  payload JSONB NOT NULL,
  status TEXT DEFAULT 'pending' CHECK (status IN (
    'pending', 
    'in_progress', 
    'completed', 
    'failed'
  )),
  idempotency_key TEXT UNIQUE NOT NULL,
  approved_by TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  executed_at TIMESTAMPTZ,
  result JSONB,
  error_message TEXT
);

CREATE INDEX idx_maintenance_commands_status ON maintenance_commands(status);
CREATE INDEX idx_maintenance_commands_created ON maintenance_commands(created_at);
```

**File to create:** `docs/supabase-schema/014_maintenance_commands.sql`

---

### 2. Maintenance Agent Refactor

**Current:** `agents/council/maintenance.py` - does council review

**Needed:** New `agents/maintenance.py` that:
- Polls `maintenance_commands` table for pending commands
- Validates command against allowlist
- Executes git operations (create branch, commit, merge, delete, tag)
- Reports success/failure back to table
- Has NO decision-making authority

**Key methods:**
```python
class MaintenanceAgent:
    def poll_commands(self) -> List[Command]
    def validate_command(self, cmd: Command) -> bool
    def execute_create_branch(self, payload: dict) -> Result
    def execute_commit_code(self, payload: dict) -> Result
    def execute_merge_branch(self, payload: dict) -> Result
    def execute_delete_branch(self, payload: dict) -> Result
    def execute_tag_release(self, payload: dict) -> Result
    def report_result(self, cmd_id: str, result: dict) -> None
```

---

### 3. Supervisor Refactor

**Current:** `agents/supervisor.py` - has git operations mixed in

**Needed:**
- Remove all direct git operations
- Add method to insert commands to `maintenance_commands`
- Keep git READ access for reviewing branches
- Add Council trigger method (calls Orchestrator)

**Changes:**
```python
# REMOVE:
# - git.Repo operations
# - direct branch creation/deletion

# ADD:
def command_create_branch(self, task_id: str, branch_name: str) -> str:
    """Insert command, return command_id"""
    
def command_commit_code(self, branch: str, code: dict) -> str:
    """Insert command, return command_id"""

def command_merge_branch(self, source: str, target: str, delete_source: bool) -> str:
    """Insert command, return command_id"""

def trigger_council_review(self, doc_path: str, lenses: List[str]) -> str:
    """Call Orchestrator to route council review"""
```

---

### 4. Council Routing via Orchestrator

**Current:** Council uses fixed agents in `agents/council/`

**Needed:** 
- Orchestrator method to route council review to available models
- Orchestrator aggregates votes and returns to Supervisor
- Council becomes a function, not fixed agents

**Add to Orchestrator:**
```python
def route_council_review(
    self, 
    doc_path: str, 
    lenses: List[str],
    context_type: str  # 'project' or 'system'
) -> CouncilResult:
    """
    1. Determine context (project: PRD+Plan, system: full context)
    2. Find available models
    3. Assign lenses to models (parallel if 3+, sequential if 1)
    4. Collect votes
    5. Aggregate and return
    """
```

---

### 5. Branch Lifecycle Code

**What:** Functions that tie Supervisor decisions to Maintenance commands

**Flow:**
```
Orchestrator assigns task
    ↓
Supervisor.command_create_branch("T001", "task/T001-desc")
    ↓
Maintenance picks up, creates branch, reports success
    ↓
...task execution...
    ↓
Supervisor.command_commit_code("task/T001", code_output)
    ↓
Maintenance commits, reports success
    ↓
...tests pass...
    ↓
Supervisor.command_merge_branch("task/T001", "module/user-auth", delete_source=True)
    ↓
Maintenance merges, deletes task branch, reports success
```

---

### 6. Rate Limit Countdown

**Current:** Orchestrator has `CooldownManager` with 80% threshold

**Needed:**
- Track per-platform rate limits (ChatGPT: 50/5hr, Claude: 40/day, etc.)
- Track reset times (ChatGPT: rolling 5hr, Claude: daily reset)
- Expose countdown via API for dashboard
- Display "Available in Xh Ym" in logs

**Add to Orchestrator:**
```python
class RateLimitTracker:
    def get_platform_status(self, platform: str) -> PlatformStatus:
        """Returns: available, countdown_seconds, daily_remaining, etc."""
    
    def get_all_platform_status(self) -> Dict[str, PlatformStatus]:
        """For dashboard display"""
```

---

### 7. Runner Contract Enforcement

**Current:** `runners/contract_runners.py` defines contracts

**Needed:**
- Validation that runner output matches expected format
- Reject malformed output before Supervisor sees it
- Log contract violations

```python
def validate_runner_output(output: dict) -> ValidationResult:
    """
    Required: task_id, status, output, metadata
    If courier: must have chat_url
    If internal: must have files list
    """
```

---

### 8. End-to-End Integration Test

**Test flow:**
```
1. Create test task in Supabase (status='available')
2. Orchestrator picks it up
3. Runner executes (mock or simple task)
4. Supervisor reviews output
5. Maintenance creates branch
6. Tests pass (mock)
7. Maintenance merges to module
8. Verify task status = 'complete'
9. Verify branch deleted
```

**File:** `tests/test_full_flow.py`

---

### 9. Orchestrator Systemd Installation

**Current:** Service file exists at `scripts/vibepilot-orchestrator.service`

**Needed:**
- Install on production server
- Enable and start service
- Verify logs flowing to journald

```bash
sudo cp scripts/vibepilot-orchestrator.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vibepilot-orchestrator
sudo systemctl start vibepilot-orchestrator
sudo systemctl status vibepilot-orchestrator
```

---

## BUILD ORDER (For Kimi Parallel Execution)

### Phase A: Schema + Config (Can run in parallel)
1. Create `docs/supabase-schema/014_maintenance_commands.sql`
2. Update `config/agents.json` with capability declarations
3. Create `config/maintenance_commands.json` with allowlist

### Phase B: Core Agents (Can run in parallel after A)
4. Refactor `agents/maintenance.py` (new file, git operator)
5. Refactor `agents/supervisor.py` (remove git write, add commands)
6. Add council routing to `core/orchestrator.py`
7. Add rate limit countdown to `core/orchestrator.py`

### Phase C: Integration (Sequential, after B)
8. Add runner contract validation
9. Write end-to-end test
10. Install orchestrator as systemd service
11. Run full test, verify works

---

## FILES TO CREATE

| File | Purpose |
|------|---------|
| `docs/supabase-schema/014_maintenance_commands.sql` | Command queue table |
| `agents/maintenance.py` | Git operator agent (NEW) |
| `config/maintenance_commands.json` | Command allowlist |
| `tests/test_full_flow.py` | Integration test |

## FILES TO MODIFY

| File | Changes |
|------|---------|
| `agents/supervisor.py` | Remove git write, add command queue |
| `core/orchestrator.py` | Add council routing, rate limit countdown |
| `config/agents.json` | Add capability declarations |
| `prompts/supervisor.md` | Update to match new role |
| `prompts/maintenance.md` | Update for git operator role |
| `prompts/internal_cli.md` | Remove git, clarify return-only |

---

## SUCCESS CRITERIA

- [ ] One task flows from pending → complete without human intervention
- [ ] Task branch created, code committed, merged to module, branch deleted
- [ ] All commands logged in maintenance_commands table
- [ ] Rate limit countdown visible
- [ ] Council review works (even with 1 model doing 3 passes)
- [ ] Orchestrator running as systemd service
- [ ] Dashboard shows task progress

---

## READY FOR KIMI

This analysis is complete. Kimi can:
- Use 100 subagents to build all Phase A items in parallel
- Then Phase B items in parallel
- Then Phase C sequentially

**Estimated time with parallel execution:** 1-2 sessions
