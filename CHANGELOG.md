# VibePilot Changelog

**Purpose:** Full audit trail of all changes. Anyone/any agent can see what, where, when, why.

**Update Frequency:** After EVERY change (file add, update, remove, merge, branch delete)

---

# 2026-02-14

## 19:00 UTC - GLM-5
**Commit:** `872b6e21`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Added Must Preserve/Never Do sections, simplified priorities
- `.context/DECISION_LOG.md` - Marked DEC-012 to DEC-015 as rejected with reasoning
- `docs/video_insights_2026-02-14.md` - Added what was rejected and why

**Why:** Vetted research suggestions against VibePilot's specific needs. Rejected over-engineering in favor of simpler approach.

**Decisions:**
- DEC-012, 013, 014, 015: Rejected (over-engineering, duplicates, complexity)
- Solution: Add Must Preserve/Never Do to CURRENT_STATE.md instead

**Priorities Updated:**
1. Schema Audit + Validation Script (DEC-011)
2. Prompt Caching (DEC-007)
3. Council RPC

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 18:35 UTC - GLM-5
**Commit:** `98668742`
**Type:** Add
**Files Added:**
- `docs/video_insights_2026-02-14.md` - Senior engineer rules, noiseless memory, navigation context

**Files Changed:**
- `CURRENT_STATE.md` - Updated decisions, priorities, directory index
- `.context/DECISION_LOG.md` - Added DEC-011 through DEC-015

**Why:** Capture video insights for next session:
- Senior engineer schema rules (portability, auditability)
- Noiseless compression (80% token reduction)
- Navigation-based context (terminal tools vs RAG)
- Awareness agents (auto-inject by keyword)

**New Proposed Decisions:**
- DEC-011: Schema Senior Rules Audit
- DEC-012: Self-Awareness SSOT Document
- DEC-013: Noiseless Compression Protocol
- DEC-014: Navigation-Based Context
- DEC-015: Awareness Agent

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 18:10 UTC - GLM-5
**Commit:** `8df8c51e`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Updated known good commit
- `CHANGELOG.md` - Added this entry

**Why:** Update known good commit after restructure

**Rollback:**
```bash
git revert 8df8c51e
```

---

## 18:05 UTC - GLM-5
**Commit:** `5719ea0f`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Major restructure for comprehensive clarity

**Added:**
- KNOWN GOOD STATE section (verified working commit for rollback)
- ACTIVE WORK section (what's in progress)
- 30-SECOND SWAPS section (zero code change swaps)
- UPDATE RESPONSIBILITY MATRIX (if X changes, update Y)
- QUICK FIX GUIDE (common issues and fixes)
- MIGRATION CHECKLIST (pack up and move)
- Required reading clarification (TWO files: this + CHANGELOG)

**Why:** Any agent/human reads TWO files and knows everything. No debugging loops of doom. Stress-free architecture.

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 17:50 UTC - GLM-5
**Commit:** `4ad011e3`
**Type:** Update
**Files Changed:**
- `CHANGELOG.md` - Added entry for commit 8b104062

**Why:** Changelog must track itself

**Rollback:**
```bash
git revert 4ad011e3
```

---

## 17:45 UTC - GLM-5
**Commit:** `8b104062`
**Type:** Add
**Files Added:**
- `CHANGELOG.md` - Full audit trail for easy rollback

**Files Changed:**
- `CURRENT_STATE.md` - Added CHANGELOG references

**Why:** Track every change with timestamps for easy rollback. Prevent debugging when rollback is faster.

**Rollback:**
```bash
git revert 8b104062
```

---

## 17:35 UTC - GLM-5
**Commit:** `0715bfae`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Added source of truth index, directory index

**Why:** Prevent Supabase queries and ls commands just to see structure

**Rollback:**
```bash
git revert 0715bfae
```

---

## 16:50 UTC - GLM-5
**Commit:** `a8c7d17b`
**Type:** Add
**Files Added:**
- `CURRENT_STATE.md` - Single source of truth for context restoration

**Why:** 77k tokens to understand state was unsustainable. Now one file.

**Decisions:** DEC-009 (Council feedback summary), DEC-010 (Single source of truth)

**Rollback:**
```bash
git revert a8c7d17b
```

---

## 16:15 UTC - GLM-5
**Commit:** `b41a98b6`
**Type:** Update
**Files Changed:**
- `docs/MASTER_PLAN.md` - Clarified Council two-process model
- `.context/DECISION_LOG.md` - Updated DEC-004

**Why:** Council isn't one-size-fits-all. PRDs need iterative, updates need one-shot.

**Rollback:**
```bash
git revert b41a98b6
```

---

## 15:50 UTC - GLM-5
**Commit:** `8eec28b1`
**Type:** Update
**Files Changed:**
- `docs/MASTER_PLAN.md` - Refined Council process based on real experience
- `.context/DECISION_LOG.md` - Added DEC-004, DEC-005

**Why:** Real experience showed 3 models need 4 rounds for consensus on PRDs

**Rollback:**
```bash
git revert 8eec28b1
```

---

## 15:20 UTC - GLM-5
**Commit:** `b8c4ee32`
**Type:** Add
**Files Added:**
- `docs/MASTER_PLAN.md` - 858-line zero-ambiguity specification

**Files Changed:**
- `STATUS.md` - Updated structure
- `docs/SESSION_LOG.md` - Added Phase 5, Phase 6

**Why:** Unified specification for all agents, context isolation by role

**Rollback:**
```bash
git revert b8c4ee32
```

---

## 14:30 UTC - GLM-5
**Commit:** `ed2e425d`
**Type:** Add
**Files Added:**
- `.context/guardrails.md` - 8 pre-code gates, P-R-E-V-C workflow
- `.context/DECISION_LOG.md` - Template + 3 documented decisions
- `.context/agent_protocol.md` - Handoff rules, conflict resolution
- `.context/quick_reference.md` - One-page cheat sheet
- `.context/ops_handbook.md` - Disaster recovery, monitoring
- `scripts/prep_migration.sh` - Migration prep automation

**Why:** Strategic safeguards to prevent "vibe coding" traps

**Rollback:**
```bash
git revert ed2e425d
```

---

# 2026-02-13

## 23:50 UTC - Human
**Commit:** `6a97eaaa`
**Type:** Add
**Files Added:**
- `docs/video summary ideas` - Video insights (prompt caching, context standard, Kimi swarm)

**Why:** Capture video learnings for future implementation

**Rollback:**
```bash
git revert 6a97eaaa
```

---

## 22:30 UTC - GLM-5
**Commit:** `26502559`
**Type:** Update
**Files Changed:**
- `docs/SESSION_LOG.md` - Added multi-project support to roadmap

**Why:** Support recipe app, finance app, VibePilot, legacy project simultaneously

**Rollback:**
```bash
git revert 26502559
```

---

## 21:50 UTC - GLM-5
**Commit:** `eded835c`
**Type:** Add
**Files Added:**
- `STATUS.md` - Root-level status and recovery

**Why:** Quick recovery after terminal crash

**Rollback:**
```bash
git revert eded835c
```

---

## 21:00 UTC - GLM-5
**Commit:** `4141f826`
**Type:** Add
**Files Added:**
- `docs/SESSION_LOG.md` - Session history
- `config/vibepilot.yaml` - Config-driven architecture

**Why:** Single config file for all runtime changes

**Rollback:**
```bash
git revert 4141f826
```

---

## 20:00 UTC - GLM-5
**Commit:** `6cb215c0`
**Type:** Add
**Files Changed:**
- `core/roles.py` - Role system
- `dual_orchestrator.py` - Gemini orchestrator option

**Why:** 2-3 skills max per role, models wear hats

**Rollback:**
```bash
git revert 6cb215c0
```

---

## 19:00 UTC - GLM-5
**Commit:** `b51acf8d`
**Type:** Add
**Files Added:**
- `docs/scripts/kimi_dispatch_demo.py` - Kimi dispatch demo

**Why:** Test Kimi CLI integration

**Rollback:**
```bash
git revert b51acf8d
```

---

## 18:00 UTC - GLM-5
**Commit:** `fc145ea2`
**Type:** Add
**Files Added:**
- `runners/kimi_runner.py` - Kimi runner for automatic dispatch

**Why:** Integrate Kimi CLI as parallel executor

**Rollback:**
```bash
git revert fc145ea2
```

---

## 17:00 UTC - GLM-5
**Commit:** `9f0fbac1`
**Type:** Add
**Files Added:**
- `docs/scripts/kimi_setup.sh` - Kimi CLI setup commands

**Why:** Document Kimi installation

**Rollback:**
```bash
git revert 9f0fbac1
```

---

## 16:00 UTC - GLM-5
**Commit:** `c425b24a`
**Type:** Add
**Files Added:**
- `docs/scripts/pipeline_test.py` - Pipeline test script

**Why:** Test full 12-stage task flow

**Rollback:**
```bash
git revert c425b24a
```

---

## 15:00 UTC - GLM-5
**Commit:** `8c5d6111`
**Type:** Update
**Files Changed:**
- `docs/schema_rls_fix.sql` - RLS fix for backend access

**Why:** Allow backend to query without RLS blocking

**Rollback:**
```bash
git revert 8c5d6111
```

---

## Earlier (see git log)
- `52ae359f` - Fix ROUND() function
- `8867b16e` - Add voice interface + project tracking
- `3527f775` - Add VibePilot v1.3 PRD + Platform Registry
- `170b3fdf` - Add VibePilot v1.2 architecture diagram
- `d3086fc5` - Add Vibeflow v5 adoption analysis
- `3d4d40de` - Add safety patches + escalation logic
- `b7966925` - Add TaskManager for new schema
- `62f816fd` - Add VibePilot Core Schema v1.0
- `052aa579` - Phase 2: Core Agent Implementation
- `c888a932` - Add Supabase schema reset SQL

---

# ROLLBACK PROCEDURES

## Single Commit Rollback

```bash
# See what commit did
git show <commit_hash>

# Rollback (creates new commit that undoes changes)
git revert <commit_hash>

# Push rollback
git push origin main
```

## Multiple Commits Rollback

```bash
# Rollback to specific point (DESTRUCTIVE - use carefully)
git reset --hard <commit_hash>

# Force push (only if you're sure)
git push origin main --force
```

## File-Level Rollback

```bash
# Restore specific file to specific commit
git checkout <commit_hash> -- path/to/file

# Commit the restoration
git add path/to/file
git commit -m "Rollback path/to/file to <commit_hash>"
git push origin main
```

## Full System Rollback (Nuclear Option)

```bash
# 1. Clone fresh
git clone git@github.com:VibesTribe/VibePilot.git vibepilot-rollback
cd vibepilot-rollback

# 2. Checkout specific commit
git checkout <commit_hash>

# 3. Update remote
git push origin main --force

# 4. On GCE, re-clone
cd ~
rm -rf vibepilot
git clone git@github.com:VibesTribe/VibePilot.git
cd vibepilot
source venv/bin/activate  # If venv exists, or recreate
```

---

# BRANCH TRACKING

## Active Branches

| Branch | Purpose | Status |
|--------|---------|--------|
| `main` | Production | Active |

## Merged & Deleted Branches

| Branch | Merged | Deleted | Commit | Notes |
|--------|--------|---------|--------|-------|
| (none yet) | - | - | - | - |

---

# HOW TO UPDATE THIS FILE

**After EVERY change:**

```markdown
## HH:MM UTC - <Agent/Human>
**Commit:** `<hash>`
**Type:** Add | Update | Remove | Merge
**Files Added:** (if any)
**Files Changed:** (if any)
**Files Removed:** (if any)
**Why:** <one line reason>
**Decisions:** DEC-XXX (if applicable)
**Rollback:** `git revert <hash>`
---
```

**After EVERY merge:**
1. Update "Merged & Deleted Branches" table
2. Include branch name, merge commit, deletion timestamp

**This file is the audit trail. Keep it accurate.**

---

*Last updated: 2026-02-14 17:35 UTC*
*Next entry: After next change*
