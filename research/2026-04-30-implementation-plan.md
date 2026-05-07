# Implementation Plan: VibePilot Infrastructure Improvements
**Date:** 2026-04-30
**Status:** FOR REVIEW -- present findings, get approval before implementing

---

## ITEM 1: Upgrade jcodemunch v1.43.0 → v1.80.1

### What we have
- pipx installed at ~/.local/bin/jcodemunch-mcp, version 1.43.0
- Also have jdatamunch-mcp 0.8.3
- 3 indexed repos: vibeflow, vibepilot, VibePilot (note: 2 VibePilot indexes -- case sensitivity)
- lean-ctx 3.1.2 also installed
- NOT in Hermes MCP config (hooks use it directly via build.sh)

### What we'd gain
- MUNCH compact encoding (45.5% median byte savings on wire)
- AST pattern matching (search_ast -- anti-pattern detection across 70+ languages)
- PR risk profiling (composite risk score from blast radius + complexity + churn + test gaps)
- Symbol provenance (git lineage of any symbol)
- Response secret redaction (AWS/GCP/Azure/JWT/GitHub tokens scrubbed)
- Embedding drift detection (catch silent provider model changes)
- Auto-watch (automatically indexes repos when tools are called against them)
- Agent config auditing (CLAUDE.md, .cursorrules, etc. -- token waste detection)

### What it involves
1. `pipx upgrade jcodemunch-mcp` (one command)
2. Re-index the VibePilot repo (triggers on first use or manually)
3. Verify build.sh still works (it calls `jcodemunch-mcp index` and `lean-ctx read`)
4. Test that .context/ rebuild still produces valid output
5. Clean up the duplicate VibePilot index (case issue)

### Risk
LOW. jcodemunch is backward-compatible (MUNCH is opt-in). No config format changes
expected between 1.43 and 1.80 that break existing usage. If something breaks, `pipx install jcodemunch-mcp==1.43.0` downgrades.

### NOT doing
- Not adding jcodemunch to Hermes MCP config (separate conversation)
- Not enabling MUNCH format by default (just making it available)
- Not changing build.sh (it works, just benefits from new version automatically)

---

## ITEM 2: LogAct Intent Logging + Append-Only Events

### What we have
- `orchestrator_events` table IS append-only (uuid PK, created_at, no updated_at)
- 10 event types actively being logged
- Trigger: `vp_notify_change()` fires on insert
- NO intent logging exists (0 mentions in Go code)
- Events capture WHAT HAPPENED (task_dispatched, plan_approved) but not WHAT WAS PLANNED

### What LogAct recommends (from our research)
1. Intent logging: record what agent PLANS before execution
2. Safety voter: cheap model cross-checks intent
3. Append-only task_events ← WE ALREADY HAVE THIS
4. Stupidity diagnosis: agent reads own failed output, rewrites

### What we actually need vs what's over-engineering

**NEED NOW:**
- Intent field on task_runs or as a new event_type. When the planner creates tasks,
  store the intent (what each task is supposed to accomplish) BEFORE execution starts.
- This gives the supervisor something concrete to compare against during output review.
- Currently supervisor gets prompt_packet + git diff but not "what was the planner thinking"

**DEFER:**
- Safety voter (separate model cross-check) -- over-engineering for our scale. Our supervisor
  already does this function. Would make sense if we had adversarial inputs (we don't).
- Stupidity diagnosis -- our analyst agent already does backwards reasoning on failures.

### What it involves
1. Add `intent` JSONB column to `task_runs` (what the planner expects this task to produce)
2. Update planner handler to write intent when creating tasks
3. Update supervisor's review context to include the original intent
4. This is a coherent unit: planner writes → supervisor reads. Both must ship together.

### Risk
MEDIUM. Requires DB migration + planner handler change + supervisor context change.
Must verify dashboard doesn't read task_runs.intent (it shouldn't -- this is internal).

---

## ITEM 3: JourneyKits Review (Decision, Not Implementation)

### What we have
- 20 patterns mapped to VibePilot gaps, prioritized in research doc
- Top 5 by impact: Codebase Map, Quality Loop, Structured Research, Daemon Campaigns, Systematic Debugging
- We now HAVE a codebase map (jcodemunch + .context/)
- We now HAVE structured research format (researcher prompt + knowledgebase)
- So the top 2 are partially done already

### What needs decisions from you
1. **Autonomous Quality Loop (#3)** -- Score task output on rubric instead of binary pass/fail?
   My take: Yes, but not today. This changes supervisor behavior. After knowledgebase is running.
2. **Daemon Campaigns (#8)** -- Stale plan detection + auto-restart?
   My take: Yes. Governor restarts already lose in-flight state. Quick win.
3. **Systematic Debugging (#5)** -- Structured hypothesis before fix?
   My take: Our analyst agent already does this. May be redundant.
4. **Self-Improve Harness (#7)** -- Propose → Score → Approve → Rollback for prompts/config?
   My take: This IS the research agent's ratchet loop. Defer to knowledgebase work.
5. **Context Guard (#6)** -- Persist compressed context across sessions?
   My take: .context/ already does this. Redundant.
6. **Council Lane Pattern (#1)** -- Parallel audit lanes?
   My take: Future. Council isn't the bottleneck right now.

### This item is ASK, not DO. Presenting for your input.

---

## ITEM 4: .context/ Hooks Async Fix

### What we have
- 4 hooks: post-checkout, post-commit, post-merge, pre-commit
- All call `build.sh` SYNCHRONOUSLY
- build.sh runs: lean-ctx map scan (~30s on x220), jcodemunch index (~60s), knowledge.db build (~10s)
- Total: ~100 seconds blocking git operations
- pre-commit already has a guard: only runs if Go/config files changed
- post-checkout/post-merge have NO change detection -- always rebuild

### What's the fix
Two approaches (can combine):

**A. Skip if nothing changed (cheap check):**
```
# Compare last build timestamp against source file mtimes
if [ "$(find governor/ -newer .context/boot.md -name '*.go' | wc -l)" -eq 0 ]; then
    echo "[.context] No source changes since last build, skipping"
    exit 0
fi
```

**B. Run in background (non-blocking):**
```
# Fire-and-forget rebuild
bash "$CTX_DIR/build.sh" --quiet &
echo "[.context] Rebuilding in background (PID $!)"
```

### What I recommend
Combine both: Check if source changed. If yes, run in background.
Pre-commit stays synchronous (we need the files staged), but with the change guard.
Post-checkout and post-merge go background + change guard.

### Risk
LOW. The hooks already work. We're just making them not block git.
Worst case: .context/ is briefly stale during a fast sequence of checkouts. build.sh is idempotent.

---

## IMPLEMENTATION ORDER

1. **jcodemunch upgrade** -- 5 min, zero risk, immediate capability gain
2. **.context/ hooks async** -- 15 min, low risk, daily quality-of-life improvement
3. **LogAct intent logging** -- 45 min, medium risk, needs DB migration + handler changes
4. **JourneyKits decisions** -- discussion, no code

Items 1 and 2 are independent, can be done in sequence today.
Item 3 needs careful implementation after 1 and 2.
Item 4 is a conversation, not code.
