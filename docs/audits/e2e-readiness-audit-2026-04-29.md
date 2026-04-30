# VibePilot E2E Readiness Audit
**Date**: 2026-04-29  
**Scope**: Full pipeline readiness for end-to-end smoke test  
**Auditor**: Hermes Agent  

---

## Summary

| Area | Status | Risk |
|------|--------|------|
| Infrastructure | PASS | None |
| DB Constraints | PASS | None |
| Event Flow | PASS | None |
| Model Routing | PASS | Low |
| Known Bugs | NOTED | Medium |
| Git State | PASS | None |
| Config Files | PASS | None |

**Overall: READY with caveats** — 3 known issues to be aware of, none block a smoke test.

---

## 1. Infrastructure

| Check | Result |
|-------|--------|
| Governor running (localhost:8080) | OK — PID active, systemctl --user |
| Database (vibepilot) | OK — tables intact, 0 unclaimed tasks |
| Cloudflared tunnel | OK — `https://webhooks.vibestribe.rocks` → 200 |
| Dashboard (Vercel) | OK — auto-deploys from GitHub push |
| Managed git repo | OK — clean, main at `5b1468d5` |

---

## 2. DB Constraint Consistency

**Tasks CHECK constraint** (12 statuses):
`pending, available, locked, in_progress, received, review, testing, complete, merge_pending, merged, failed, human_review`

**Plans CHECK constraint** (6 statuses):
`draft, pending, active, complete, cancelled, superseded`

Go-written statuses: all 12 task statuses and 6 plan statuses confirmed in handler code.
- `locked` and `received` exist in CHECK but are intermediate states not currently written — not a bug.
- `transition_task` RPC delegates validation to CHECK constraint (single source of truth).

---

## 3. Event Flow

Every `router.On` registration in `main.go` has a matching handler and pgnotify channel:

| Channel | Handler | File |
|---------|---------|------|
| task_available | mapEvent | listener.go |
| task_review | mapEvent | listener.go |
| task_testing | mapEvent | listener.go |
| task_approval | mapEvent | listener.go |
| task_merge_pending | mapEvent | listener.go |
| task_human_review | mapEvent | listener.go |
| plan_created | mapPlanEvent | listener.go |
| plan_complete | mapPlanEvent | listener.go |

**transition_task** emits NOTIFY for every status change, so all intermediate transitions are covered.

---

## 4. Model Routing

| Metric | Count |
|--------|-------|
| Destinations | 26 (6 CLI, 10 API, 10 web) |
| Models | 66 |
| Vault keys | 15 (all API connectors covered) |

**Strategy**: `default` with priority `[external, internal]`.  
**Agent restrictions**: planner, supervisor, council, orchestrator, maintenance, watcher, tester = `internal_only`.

**Models to note**:
- `deepseek-chat` / `deepseek-reasoner` = paused (intentional)
- `kimi-k2-instruct` = offline (intentional)
- Several models = benched (normal)

---

## 5. Known Issues (Not Blockers)

### 5a. extractJSON robustness (LOW RISK)
- **Location**: `governor/internal/runtime/decision.go:274`
- **What**: Two-strategy parser (code fence first, brace-matching fallback)
- **Risk**: Could fail on malformed model output with unbalanced braces in text
- **Impact**: Task would fail and go to recovery — self-healing
- **Recommendation**: Accept for smoke test. Strengthen if it causes failures.

### 5b. `create_task_with_packet` RPC unused (NO RISK)
- **What**: RPC exists in allowlist, never called by Go code
- **Possibility**: Dead code from abandoned feature, or reserved for future use
- **Impact**: None — unused code doesn't affect pipeline
- **Recommendation**: Leave alone. Can remove in cleanup pass later.

### 5c. ReviewPanel → Governor disconnect (MEDIUM RISK)
- **What**: Dashboard ReviewPanel dispatches GitHub Actions (`approve_review.yml`, `request_changes.yml`) via `dispatch.trigger()`. Governor only learns about results through pgnotify events when the workflow completes.
- **Impact**: If GitHub Actions workflow fails silently, dashboard shows "approved" but governor never transitions the task.
- **Mitigation**: Recovery loop in governor handles stuck tasks.
- **Recommendation**: Accept for smoke test. Consider adding a webhook callback from the GitHub Action to governor.

---

## 6. Git State

| Repo | Branch | Status |
|------|--------|--------|
| Managed VibePilot | main | Clean at `5b1468d5` |
| Hermes context repo | main | Dirty (`.context/` files, `CURRENT_ISSUES.md`) |

**Stale branches identified** (from previous session):
- `fix/pipeline-events-timestamps-and-data` (local)
- `fix/robust-output-parser` (local)
- `remotes/origin/TEST_MODULES/general` (remote)

**Old E2E artifacts on GitHub**:
- `docs/plans/e2e-smoke-test-greeting-output-plan.md`
- `docs/prd/e2e-smoke-test.md`

These should be cleaned before the fresh E2E run.

---

## 7. Config Verification

| Config | Valid | Notes |
|--------|-------|-------|
| system.json | OK | checkpoint_interval, max_task_attempts, etc. |
| routing.json | OK | strategies, agent_restrictions, selection_criteria |
| models.json | OK | 66 models with proper structure |
| connectors.json | OK | 26 destinations |

All config files parse correctly and have consistent cross-references.

---

## Recommendations

1. **PROCEED with E2E smoke test** — no blockers found
2. **Clean stale branches** after E2E (not urgent)
3. **Clean old E2E artifacts** from GitHub before fresh run
4. **Monitor** for extractJSON failures — if they occur, strengthen parser
5. **Consider** adding GitHub Action → governor callback for ReviewPanel (post-E2E)

---

## Audit Methodology

- Direct PostgreSQL queries for constraint inspection
- Go source code analysis for handler/registration coverage
- Config file validation (JSON parsing)
- Process/service status checks
- HTTP endpoint verification (tunnel, governor)
