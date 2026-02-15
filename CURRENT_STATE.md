# VibePilot Current State

**Required reading: THREE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/prd_v1.4.md`** - Complete system specification (NEW - read this)
3. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all three → Know everything → Do anything**

---

**Last Updated:** 2026-02-15 06:30 UTC
**Updated By:** GLM-5 (Agent definitions + prompts complete, tech stack documented)
**Known Good Commit:** `07d5fd62` (Maintenance agent added)

---

# WHAT IS VIBEPILOT

Sovereign AI execution engine. Human provides idea → VibePilot executes with zero drift.

**Core Rules:**
- All state in Supabase
- All code in GitHub
- All changes via config (zero code edits for swaps)
- Context isolation per agent (task agents see ONLY their task)
- Council for PRDs/plans (iterative), system updates (one-shot vote)

---

# KNOWN GOOD STATE

| Commit | Date | Status | Notes |
|--------|------|--------|-------|
| `aaabc5c5` | 2026-02-15 | ✅ Verified | PRD v1.4 complete (operational details) |
| `46423d69` | 2026-02-14 | ✅ Verified | Vault-based secrets, migration ready |
| `c5c5b143` | 2026-02-14 | ✅ Verified | Schema audit, caching, Council RPC |

**If everything breaks:**
```bash
git checkout aaabc5c5
```

---

# ACTIVE WORK

| Task | Status | Agent | Started | Notes |
|------|--------|-------|---------|-------|
| None | - | - | - | Ready for next session |

---

# CURRENT ISSUES

| Issue | Status | Impact | Notes |
|-------|--------|--------|-------|
| Kimi subscription ending | 🟡 Soon | Low | CLI auth transfers with ~/.kimi/ |
| GCE cost ($24/2wks) | 🟡 Action needed | High | Ready for Hetzner move (~€4/mo) |
| Secrets in .env | ✅ FIXED | - | Now in vault |

---

# WHERE WE ARE

## Built & Working

| Component | What It Does | Location |
|-----------|--------------|----------|
| Core Schema | Stores all state | Supabase: tasks, task_packets, models, platforms, projects, task_runs |
| Model Registry | Tracks available AI models | Supabase `models` table + `config/vibepilot.yaml` |
| Platform Registry | Tracks web AI platforms | Supabase `platforms` table |
| Kimi CLI Runner | Executes tasks via Kimi | `runners/kimi_runner.py` |
| API Runner with Caching | DeepSeek, Gemini, OpenRouter | `runners/api_runner.py` |
| Dual Orchestrator | Routes tasks to right model | `orchestrator.py` |
| Role System | Defines agent capabilities | `core/roles.py` + `config/vibepilot.yaml` |
| **Vault** | Encrypted secret storage | `vault_manager.py` + Supabase `secrets_vault` |
| **Agent Definitions** | Complete spec for all 11 agents | `agents/agent_definitions.md` |
| **Agent Prompts** | Full prompts for all 10 agents | `prompts/*.md` (10 files - all complete) |
| **Tech Stack Decisions** | Documented technology choices | `docs/tech_stack.md` |

## Vault (Secret Management)

**Purpose:** API keys encrypted in Supabase, never in .env files.

**Bootstrap Keys (must be set manually):**
- `SUPABASE_URL`
- `SUPABASE_KEY`
- `VAULT_KEY` (Fernet key: `LgbwdSxxDwTaeCN5Ed2J6ETrLhtIFhf2tfeEO0bVABg=`)

**Secrets in Vault:**
- `DEEPSEEK_API_KEY` ✅
- `GITHUB_TOKEN` ✅
- `GEMINI_API_KEY` ✅
- `OPENROUTER_API_KEY` ✅

**To add a secret:**
```python
from vault_manager import VaultManager
vault = VaultManager()
vault.ingest_secret('KEY_NAME', 'key_value')
```

**Runners use vault automatically:** `get_api_key('DEEPSEEK_API_KEY')`

## Not Yet Built

| Component | What It Will Do | Notes |
|-----------|-----------------|-------|
| Council RPC | Multi-model consensus | Schema ready, needs Python integration |
| Courier Agent | Dispatch to web platforms | Definition + prompt ready (Phase 3) |
| Dashboard | Visual monitoring | Frontend in Vibeflow, needs Supabase connection |
| Voice Interface | Talk to Vibes | Designed in `docs/voice_interface.md` |
| Email Notifications | Daily summaries via Gmail | browser-use approach decided |
| Consultant Prompt | User has notes to add | Stub in `prompts/consultant.md` |

---

# WHERE WE'RE GOING

## Immediate (Next Session)

1. ~~**Schema Audit + Validation Script** - Check existing schema, create auto-validator (DEC-011)~~ ✅ DONE
2. ~~**Prompt Caching** - Add to runners, 75% cost savings on Council (DEC-007)~~ ✅ DONE
3. ~~**Apply Schema Fixes** - Run `docs/schema_timestamp_fixes.sql` in Supabase~~ ✅ DONE
4. ~~**Test Council RPC** - Run `docs/schema_council_rpc.sql`, test functions~~ ✅ DONE

## Near Term

5. **Kimi Swarm** - Add trigger to orchestrator (DEC-008)
6. **Courier Agent** - Dispatch to web platforms
7. ~~**Migration Prep** - Test setup.sh, prep for cheaper hosting~~ ✅ DONE (vault ready)
8. **TypeScript Decision** - DEC-006 (migrate or not?)

## Future

8. **Voice Interface** - Talk to Vibes
9. **Multi-Project** - Recipe app, finance app, VibePilot, legacy project

---

# 30-SECOND SWAPS (Zero Code Changes)

| What to Swap | How | Time |
|--------------|-----|------|
| Add model | Edit `config/vibepilot.yaml` → add to `models:` section | 30s |
| Remove model | Edit `config/vibepilot.yaml` → set `status: paused` | 30s |
| Swap default model | Edit `config/vibepilot.yaml` → change `default_model` | 30s |
| Add platform | Edit `config/vibepilot.yaml` → add to `platforms:` section | 30s |
| Change threshold | Edit `config/vibepilot.yaml` → update `thresholds:` section | 30s |
| Update prompt | Edit `config/vibepilot.yaml` → update `prompts:` section | 30s |
| Add skill to role | Edit `config/vibepilot.yaml` → update `roles:` section | 30s |

**All swaps:**
1. Edit `config/vibepilot.yaml`
2. Save (hot-reloads)
3. Update CHANGELOG.md
4. Done

---

# UPDATE RESPONSIBILITY MATRIX

| If You Change... | Update These Files |
|------------------|-------------------|
| Config (model, platform, role, prompt, threshold) | `config/vibepilot.yaml` + `CHANGELOG.md` |
| Add new file | `CURRENT_STATE.md` (directory index) + `CHANGELOG.md` |
| Remove file | `CURRENT_STATE.md` (directory index) + `CHANGELOG.md` |
| Architecture | `docs/MASTER_PLAN.md` + `CHANGELOG.md` + `CURRENT_STATE.md` |
| Decision made | `.context/DECISION_LOG.md` + `CURRENT_STATE.md` (key decisions) |
| Component built | `CURRENT_STATE.md` (where we are) + `CHANGELOG.md` |
| Priority changed | `CURRENT_STATE.md` (where we're going) + `CHANGELOG.md` |
| Issue found/resolved | `CURRENT_STATE.md` (current issues) + `CHANGELOG.md` |
| Anything | `CHANGELOG.md` (always) |

**Rule: Every change → CHANGELOG.md. Every structural change → CURRENT_STATE.md.**

---

# QUICK FIX GUIDE

## "Something broke, I don't know what"

```bash
# 1. Check if it worked before
git log --oneline -10

# 2. Rollback to last known good
git checkout 5a2f118b

# 3. If that fixes it, bisect to find bad commit
git bisect start
git bisect bad HEAD
git bisect good 5a2f118b
# Git will guide you through finding the bad commit

# 4. Once found, rollback just that commit
git bisect reset
git revert <bad_commit_hash>
```

## "Config change broke something"

```bash
# 1. Check what changed
git diff HEAD~1 config/vibepilot.yaml

# 2. Revert the config file
git checkout HEAD~1 -- config/vibepilot.yaml

# 3. Test

# 4. If fixed, commit the revert
git add config/vibepilot.yaml
git commit -m "Revert config change that broke X"
```

## "Database issue"

```bash
# 1. Check Supabase status
# Go to supabase.com dashboard

# 2. If table corrupted, restore from backup
# Supabase → Database → Backups → Restore

# 3. Check schema
psql $SUPABASE_URL -c "\dt"

# 4. Check recent changes
psql $SUPABASE_URL -c "SELECT * FROM task_runs ORDER BY created_at DESC LIMIT 10"
```

## "Agent stuck in loop"

```bash
# 1. Check task status in Supabase
# tasks table → find stuck task → check attempts

# 2. If > 3 attempts, escalate
UPDATE tasks SET status = 'escalated' WHERE id = '<task_id>';

# 3. Check CHANGELOG.md for recent changes that might have caused it

# 4. Rollback if needed
git revert <recent_commit>
```

## "Need to move to new server"

```bash
# NEW SERVER (30 minutes max):
# 1. Store these 3 in GitHub Secrets:
#    - SUPABASE_URL
#    - SUPABASE_KEY
#    - VAULT_KEY

# 2. On new server:
git clone git@github.com:VibesTribe/VibePilot.git
cd VibePilot

# 3. Set bootstrap keys (one of these methods):
#    Option A: GitHub CLI
gh secret set SUPABASE_URL --body "https://..."
gh secret set SUPABASE_KEY --body "eyJ..."
gh secret set VAULT_KEY --body "LgbwdSxx..."

#    Option B: Create .env manually
cat > .env << EOF
SUPABASE_URL=https://qtpdzsinvifkgpxyxlaz.supabase.co
SUPABASE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
VAULT_KEY=LgbwdSxxDwTaeCN5Ed2J6ETrLhtIFhf2tfeEO0bVABg=
EOF

# 4. Run setup
./setup.sh

# 5. Verify vault works
source venv/bin/activate
python -c "from vault_manager import VaultManager; v=VaultManager(); print(v.get_secret('DEEPSEEK_API_KEY')[:8])"
# Should print: sk-dba57

# Done - all other secrets come from vault automatically
```

---

# MIGRATION CHECKLIST

**Before Move:**
- [x] All changes committed to GitHub ✅
- [x] Vault stores all API keys ✅
- [x] `.env` reduced to 3 bootstrap keys ✅
- [x] `setup.sh` exists and works ✅
- [ ] Store 3 keys in GitHub Secrets (or secure location)
- [x] CHANGELOG.md up to date ✅
- [x] CURRENT_STATE.md up to date ✅

**After Move:**
- [ ] `git clone` works
- [ ] Set 3 bootstrap env vars
- [ ] `./setup.sh` runs without errors
- [ ] Vault retrieval works (test with DeepSeek key)
- [ ] Kimi CLI auth transferred (`~/.kimi/` folder)
- [ ] Read CURRENT_STATE.md
- [ ] Continue from "WHERE WE'RE GOING"

---

# SOURCE OF TRUTH INDEX

## Changelog (Audit Trail)

| File | Purpose | Update Frequency |
|------|---------|------------------|
| `CHANGELOG.md` | Full audit trail of all changes | After EVERY change |

**Contains:** What changed, when, why, commit hash, rollback command

## Supabase (All State)

| Table | What It Stores | Key Fields |
|-------|----------------|------------|
| `tasks` | Task queue + state | id, status, priority, dependencies, project_id |
| `task_packets` | Versioned prompts | task_id, prompt, tech_spec, version |
| `models` | AI model registry | id, platform, status, limits, usage |
| `platforms` | Web AI platforms | id, type, capabilities, limits, status |
| `projects` | Multi-project tracking | id, name, status, roi_cumulative |
| `task_runs` | Execution history | task_id, model_id, platform, tokens, duration |

**Connection:** `.env` → `SUPABASE_URL`, `SUPABASE_KEY`

## GitHub (All Code + Docs)

| Path | What It Does | Update When |
|------|--------------|-------------|
| `CURRENT_STATE.md` | Context restoration | Every session |
| `CHANGELOG.md` | Audit trail | Every change |
| `setup.sh` | One-command setup | Rare (new machine) |
| `.env.example` | Environment template | New variable needed |
| `config/vibepilot.yaml` | ALL runtime config | Config change |
| `.context/DECISION_LOG.md` | Full decision details | New decision |
| `.context/guardrails.md` | Pre-code gates | Rare |
| `docs/MASTER_PLAN.md` | Full specification | Architecture change |
| `core/roles.py` | Role definitions | Role change |
| `runners/kimi_runner.py` | Kimi CLI integration | Runner change |
| `orchestrator.py` | Multi-model routing | Orchestrator change |
| `task_manager.py` | Task CRUD | Task logic change |
| `agents/` | Agent implementations | Agent change |

**Repo:** `git@github.com:VibesTribe/VibePilot.git`

## GCE/Terminal (Ephemeral - NOT Source of Truth)

| Path | Purpose | Persistent? |
|------|---------|-------------|
| `~/vibepilot/` | Project clone | No (re-clone) |
| `~/vibepilot/venv/` | Python venv | No (recreate) |
| `~/vibepilot/.env` | Secrets | No (recreate from .env.example) |
| `~/.local/bin/kimi` | Kimi CLI | No (reinstall) |

**Server:** `ssh mjlockboxsocial@vibestribe`
**Nothing on GCE is source of truth.**

---

# DIRECTORY INDEX

```
~/vibepilot/
│
├── CURRENT_STATE.md          # THIS FILE - start here
├── CHANGELOG.md              # Audit trail - rollback info
├── README.md                 # GitHub landing page
├── TEMP_CRON_COMMANDS.md     # DELETE AFTER USE - cron setup
├── setup.sh                  # One-command setup for fresh machine
├── .env                      # Secrets (NOT in git)
├── .env.example              # Secret template (in git) - COPY THIS
├── requirements.txt          # Python dependencies
│
├── config/
│   └── vibepilot.yaml        # ALL runtime config (edit this for swaps)
│
├── .context/                 # Strategic docs
│   ├── guardrails.md         # Pre-code gates
│   ├── DECISION_LOG.md       # Full decision details
│   ├── agent_protocol.md     # Agent coordination
│   ├── quick_reference.md    # Cheat sheet
│   └── ops_handbook.md       # Disaster recovery
│
├── docs/
│   ├── MASTER_PLAN.md        # Full specification
│   ├── SESSION_LOG.md        # Session history
│   ├── UPDATE_CONSIDERATIONS.md  # Daily improvement input
│   ├── prd_v1.4.md           # Product requirements v1.4
│   ├── tech_stack.md         # Technology decisions (NEW)
│   ├── schema_*.sql          # Database schemas (9 files)
│   │   schema_v1_core.sql
│   │   schema_safety_patches.sql
│   │   schema_platforms.sql
│   │   schema_project_tracking.sql
│   │   schema_rls_fix.sql
│   │   schema_reset.sql
│   │   schema_timestamp_fixes.sql
│   │   schema_council_rpc.sql
│   │   └── scripts/          # Test/demo scripts
│
├── core/
│   └── roles.py              # Role definitions
│
├── runners/
│   ├── kimi_runner.py        # Kimi CLI integration
│   └── api_runner.py         # API runner with caching
│
├── agents/
│   └── agent_definitions.md  # Complete agent specs (NEW)
│
├── prompts/                  # Agent prompts (NEW)
│   ├── planner.md
│   ├── supervisor.md
│   ├── council.md
│   ├── orchestrator.md
│   ├── testers.md
│   ├── system_researcher.md
│   ├── watcher.md
│   ├── maintenance.md
│   └── consultant.md (stub - awaiting user notes)
│
├── scripts/
│   ├── backup_supabase.sh    # Daily backup automation
│   └── prep_migration.sh     # Migration prep
│
├── venv/                     # Python venv (ignored)
└── __pycache__/              # Cache (ignored)
```

---

# KEY DECISIONS

| ID | Decision | Status | Summary |
|----|----------|--------|---------|
| DEC-001 | Dual Orchestrator | ✅ | GLM-5 primary, Kimi parallel, Gemini research |
| DEC-002 | State in Supabase | ✅ | All state in DB, code in GitHub |
| DEC-003 | Bounded Roles | ✅ | 2-3 skills max per role |
| DEC-004 | Council Two-Process | ✅ | One-shot for updates, iterative for PRDs |
| DEC-005 | Context Isolation | ✅ | Task agents see ONLY their task |
| DEC-006 | TypeScript Migration | ⏳ | Pending decision |
| DEC-007 | Prompt Caching | ⏳ | Add to API runners |
| DEC-008 | Kimi Swarm | ⏳ | Trigger for wide tasks |
| DEC-009 | Council Feedback Summary | ✅ | Supervisor summarizes to prevent bloat |
| DEC-010 | Single Source of Truth | ✅ | CURRENT_STATE.md for context |
| DEC-011 | Schema Senior Rules Audit | ⏳ | Apply senior engineer DB rules |

Full details: `.context/DECISION_LOG.md`
Daily input: `docs/UPDATE_CONSIDERATIONS.md`

**Rejected/Simplified:**
- DEC-012/013/014/015: Over-engineering. Solved by Must Preserve/Never Do sections above.

---

# ARCHITECTURE QUICK REF

## Agent Context Levels

| Agent | Sees | Why |
|-------|------|-----|
| Task Agent | ONLY their task | Zero drift |
| Planner | PRD, system overview | Create atomic slices |
| Council | Everything | Thorough review |
| Supervisor | Plan + task | Validate output |
| Maintenance | Everything (sandbox) | System changes |
| Researcher | System overview | Find improvements |
| Tester | Only code | Objective testing |

## Council Process

| Type | Rounds | For |
|------|--------|-----|
| One-shot vote | 1 | System updates, maintenance, features |
| Iterative consensus | 3-4 | PRDs, full vertical slice plans |

## Model Lenses

| Model | Catches |
|-------|---------|
| GPT | Drift from user intent |
| Gemini | Opportunities |
| GLM-5 | Build issues, vulnerabilities |

---

# QUICK START

```
NEW SESSION:
1. Read CURRENT_STATE.md (this file)
2. Read CHANGELOG.md (recent changes)
3. Check "WHERE WE'RE GOING" for priorities
4. Check "CURRENT ISSUES" for blockers
5. Check "ACTIVE WORK" for in-progress tasks
6. Start work
7. Update this file and CHANGELOG.md when done

SOMETHING BROKE:
1. Check "KNOWN GOOD STATE" for rollback target
2. Check CHANGELOG.md for recent changes
3. Check "QUICK FIX GUIDE" for common issues
4. Rollback if debugging takes > 10 min

MAKING A CHANGE:
1. Check "UPDATE RESPONSIBILITY MATRIX"
2. Make change
3. Update required files
4. Always update CHANGELOG.md
```

---

# HOW TO UPDATE THIS FILE

**After every session:**
1. Update "Last Updated" timestamp
2. Update "KNOWN GOOD STATE" if all tests pass
3. Update "ACTIVE WORK" if starting/finishing task
4. Update "WHERE WE ARE" if components changed
5. Update "WHERE WE'RE GOING" if priorities changed
6. Add new decisions to "KEY DECISIONS"
7. Update "CURRENT ISSUES" as they arise/resolve
8. **Always update CHANGELOG.md**

---

*Token target: <4500 | Actual: ~4300*
*Context restoration: TWO files (this + CHANGELOG)*
