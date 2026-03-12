# VibePilot Session Log

## Session: 2026-02-13 (Foundational Build)

### Phase 0: Audit & Cleanup ✅
- Removed `venv/` from git tracking (15,838 files)
- Created comprehensive `.gitignore`
- Removed `.env`, logs, test files from tracking
- **Files:** `.gitignore`, `docs/phase0_audit_report.md`

### Phase 1: Schema & Pipeline ✅
- Created core schema: `tasks`, `task_packets`, `models`, `task_runs`
- Added safety patches: 3-attempt limit, escalation, circular dependency check
- Created platform registry: ChatGPT, Claude, Gemini, Perplexity, etc.
- Added project tracking with real-time ROI
- **Files:** `docs/schema_v1_core.sql`, `docs/schema_safety_patches.sql`, `docs/schema_platforms.sql`, `docs/schema_project_tracking.sql`, `docs/schema_rls_fix.sql`

### Phase 2: Multi-Model Integration ✅
- Installed Kimi CLI (`~/.local/bin/kimi`)
- Added Kimi to model registry
- Corrected model registry: GLM-5 (primary), Kimi (parallel), DeepSeek (backup)
- Built KimiRunner for automatic dispatch
- **Files:** `runners/kimi_runner.py`, `docs/scripts/kimi_setup.sh`

### Phase 3: Role System ✅
- Created 7 focused roles with 2-3 skills each
- Bounded prompts to prevent drift
- Model preferences per role
- **Files:** `core/roles.py`

### Phase 4: Dual Orchestrator ✅
- Routes tasks between GLM-5 and Kimi
- Model selection based on task type
- **Files:** `dual_orchestrator.py`

### Model Registry (Final)
| Model | Platform | Role | Status |
|-------|----------|------|--------|
| GLM-5 | OpenCode | Primary coder/supervisor/planner | Active |
| Kimi K2.5 | CLI | Parallel executor/reviewer | Active |
| Gemini Flash | Google AI | Research/council | Active (limited) |
| DeepSeek | API | Backup | Paused |

### Architecture Principles Established
- 2-3 skills max per agent
- Models wear different hats (roles)
- Council = 3 independent models, no chat
- No community skills (build our own)
- 80% threshold = timeout/pause
- All state in Supabase

---

## Session: 2026-02-14 (Strategic Guardrails + Master Plan)

### Phase 5: Strategic Documentation ✅
Based on video insights about "Vibe Coding" traps and mental models:
- Created `.context/guardrails.md` - 8 pre-code gates, P-R-E-V-C workflow
- Created `.context/DECISION_LOG.md` - Template + 3 existing decisions documented
- Created `.context/agent_protocol.md` - Handoff rules, conflict resolution
- Created `.context/quick_reference.md` - One-page cheat sheet
- Created `.context/ops_handbook.md` - Disaster recovery, monitoring
- Created `scripts/prep_migration.sh` - Migration prep automation

### Phase 6: Zero-Ambiguity Master Plan ✅
Synthesized all documentation into single source of truth:
- Created `docs/MASTER_PLAN.md` - 858-line comprehensive specification
- Defined context isolation by agent role
- Specified vertical slicing requirements
- Documented task prompt format with zero ambiguity
- Established Council review protocol
- Defined swappability matrix
- Listed forbidden anti-patterns

### Key Decisions (DEC-004 through DEC-006 pending formal logging)
- **Context Isolation**: Task agents see ONLY their task (zero drift risk)
- **Vertical Slicing**: Every task = complete, testable feature
- **Prompt Engineering**: Explicit file names, data shapes, DO NOT lists
- **Sandbox Testing**: All maintenance changes tested before live
- **Config Swappability**: All swaps via config/vibepilot.yaml (zero code changes)

### Video Insights Applied
| Video | Concept | Implementation |
|-------|---------|----------------|
| Hussein Younes | Prompt Caching | DEC-004 (pending) |
| Vini (AI Coders) | Universal Context Standard | `.context/` folder structure |
| Vini (AI Coders) | P-R-E-V-C Workflow | `guardrails.md` |
| Moonshot/Together | Kimi K2.5 Swarm | DEC-005 (pending) |
| Vibe Coding critique | Mental Models | `MASTER_PLAN.md` full specification |
| OpenCode creator | TypeScript consideration | DEC-006 (pending, proposed) |

---

## Next Steps

**Immediate (Today/Tomorrow):**
- [ ] **Commit MASTER_PLAN.md** to GitHub
- [ ] **Test migration prep** - Run `scripts/prep_migration.sh` locally
- [ ] **DEC-004: Prompt Caching** - Implement in runners
- [ ] **DEC-005: Kimi Swarm Trigger** - Define in orchestrator
- [ ] **DEC-006: TypeScript Migration** - Decision needed

**Phase 7: Council Activation (Next):**
- [ ] Implement Council RPC in Supabase
- [ ] Wire 3 independent models to Council roles
- [ ] Test Council review workflow
- [ ] Create first plan for Council review

**Phase 8: Courier Agent:**
- [ ] Build courier agent for web platforms
- [ ] Gmail session management
- [ ] Chat URL capture
- [ ] Platform selection logic

**Phase 9: Voice Interface:**
- [ ] Deploy Cloudflare Worker
- [ ] Configure Deepgram account
- [ ] Add VibesButton to dashboard

**Phase 10: Migration:**
- [ ] Research cheaper hosting (Hetzner, etc.)
- [ ] Test setup.sh on fresh machine
- [ ] Plan migration window
- [ ] Execute migration with zero data loss

## Feature Requests
- **Multi-Project Support**: VibePilot can work on multiple projects simultaneously
  - Recipe app
  - Personal finance app
  - VibePilot itself (current)
  - Future massive legacy project
  - Need: Project selector in dashboard, tasks linked to projects (already in schema)

---

## How to Resume After Crash

1. Open new SSH: `ssh mjlockboxsocial@vibestribe`
2. Navigate: `cd ~/vibepilot && source venv/bin/activate`
3. Read this file: `cat docs/SESSION_LOG.md`
4. Check GitHub for latest: `git pull`
5. Continue from "Next Steps" above

---

## Context Window Safety

If approaching 80% context (200k+ tokens):
1. Say "Compress session" - I'll summarize to SESSION_LOG.md
2. Start fresh session
3. I read SESSION_LOG.md to restore context

---

## Session: 2026-03-11 Session 81

### Issue: Prompts Missing from Task Branches
- Governor binary runs from git working directory
- When task executes, git switches to `task/T001` branch
- Task branch doesn't include `prompts/` directory
- Supervisor fails: "supervisor.md: no such file or directory"

### Fix: Sync Prompts to Supabase
- `89732bb7` fix: sync prompts from GitHub to Supabase on startup
- `5664448f` feat: load prompts from Supabase with filesystem fallback
- Prompts now synced to Supabase on governor startup
- Agents load prompts from Supabase, not filesystem

### Test Result
- Task T001 executed successfully
- Created `pkg/hello/hello.go`
- Task reached `merged` status
- Flow works end-to-end

---

## Session: 2026-03-12 Session 82

### Cleanup & Fixes
1. Fixed RPC allowlist (`find_pending_resource_tasks`) - applied migration 090 to Supabase
2. Cleaned all test data from Supabase
3. Removed test PRD/plan files

### Test 1: Simple Greet Function
- **PRD:** `test-simple.md`
- **Result:** ✅ SUCCESS
- **Duration:** 2m 26s end-to-end
- **Output:** `governor/cmd/tools/greet.go` created
- **Flow:** PRD → Plan → Supervisor → Task → Review → Testing → Merged

### Dashboard Status Labels Fixed
- Changed `review` → "Reviewing" (was "Needs Review")
- Changed `testing` → "Testing"
- Added `merged` → "Merged" (separate from complete)
- Commits: `e0da8294`, `57731cf3` in vibeflow repo

### Test 2: Dashboard Header Rename
- **PRD:** `rename-dashboard-header.md`
- **Result:** ❌ FAILED
- **Issue:** Supervisor parse error - model returned markdown instead of JSON

### Bugs Identified
1. **Task Numbering Bug** - All tasks get T001, should increment
2. **Supervisor JSON Parse Error** - Model returns narrative text instead of JSON

### System Status
- Governor: Running cleanly
- Realtime: Connected
- ResourceRecovery: Working (no more spam)

---

*Last updated: 2026-03-12 Session 82 End*
