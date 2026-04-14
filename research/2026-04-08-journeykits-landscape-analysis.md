# JourneyKits Landscape Analysis for VibePilot

**Date:** 2026-04-08
**Source:** https://www.journeykits.ai/browse (95 kits)
**Purpose:** Identify kits whose patterns improve VibePilot's modular, agnostic, configurable, flexible, robust architecture.

---

## What We Have (VibePilot Today)

### Core Engine
- **Go binary** (~4-8k lines) with YAML DAG pipeline engine
- **DAG nodes:** prompt, bash, approval, agent, emit, loop (6 types)
- **Config-driven:** models.json, connectors.json, agents.json, routing.json
- **State:** Supabase (tasks, task_runs, plans, models, platforms, events)
- **Code:** GitHub (PRDs, plans, migrations, prompts, config)
- **Dashboard:** Vercel/React (read-only view of Supabase state)

### Pipeline (code-pipeline.yaml)
- Flow A: PRD -> Plan -> Supervisor Review -> (Council if complex) -> Create Tasks
- Flow B: Task -> Execute -> Supervisor Review -> Test -> (Visual QA if UI) -> Merge -> Record
- Flow C: Research -> Supervisor Review -> (Council if architecture) -> Human if approved

### Agent Roles
- Planner, Supervisor, Council, Task Runner, Tester, Visual Tester, Orchestrator, Courier, Researcher, Consultant, Maintenance, Watcher

### What's Missing / Weak
1. **No self-improvement loop** -- agents execute but don't learn from outcomes
2. **No structured research** -- web search is ad-hoc, no confidence levels or citation tracking
3. **No context preservation** -- sessions die, no survival across compaction/crashes
4. **No codebase awareness** -- agents don't know the project structure when executing
5. **No parallel audit** -- Council exists but isn't formalized as parallel lanes
6. **No cron/scheduled monitoring** -- system is reactive, not proactive
7. **No token cost optimization** -- no compression, no caching strategy
8. **No fleet sync** -- single machine, no cross-session state replication
9. **No skill drift detection** -- prompts/configs can rot without detection
10. **No security scanning** -- leak_detector.go exists but isn't in pipeline

---

## Kit Analysis: What Maps Where

### TIER 1 -- Directly Improves VibePilot Components

#### 1. Council Lane Pattern (matt-clawd/council-lane-pattern)
- **What:** Parallel audit lanes converging into ranked digest. Idempotency, coverage gating, rollup locking.
- **Maps to:** Council agent role + `council-plan-review` / `council-research-review` nodes
- **Current gap:** Council is a single agent call, not parallel lanes. No coverage gating (if one model times out, council might pass incomplete). No rollup lock (concurrent councils could conflict).
- **How it improves VibePilot:**
  - Make council a DAG sub-pipeline with parallel lanes (security, architecture, user_alignment)
  - Add `coverage_threshold` parameter (100% = all lanes must complete)
  - Add `rollup_lock_ttl_ms` to prevent duplicate digests
  - Structured JSON evidence per lane instead of free-text opinions
- **Config surface:** New `council_lanes` section in agents.json or a dedicated council-lanes.yaml
- **Risk:** Low. Additive -- doesn't change existing flow, just formalizes council.

#### 2. Self-Improve Harness (giorgio/self-improve-harness)
- **What:** Proposer -> Scorer -> Approval Queue -> Rollback -> Audit Log. Claude-powered.
- **Maps to:** The entire "record-outcomes" node + Maintenance agent role
- **Current gap:** `record-outcomes` just emits an event. No feedback loop back into prompts, routing, or config.
- **How it improves VibePilot:**
  - After N completed tasks, analyze success/failure patterns
  - Propose prompt tweaks, routing changes, model preference updates
  - Score proposals against historical data
  - Queue for supervisor approval, with rollback if regression detected
  - Audit log in Supabase (new table: `improvement_proposals`)
- **Config surface:** `self_improve` section in routing.json or new self-improve.yaml
- **Risk:** Medium. Needs new Supabase table and careful rollback logic. Start manual, automate gradually.

#### 3. Autonomous Quality Loop (citadel/autonomous-improve)
- **What:** Score against rubric -> Attack weakest axis -> Verify no regressions -> Document learning -> Loop.
- **Maps to:** Supervisor review + Tester validation
- **Current gap:** Supervisor approves/rejects binary. No scoring rubric, no targeted improvement.
- **How it improves VibePilot:**
  - Add quality rubric to task output (correctness, completeness, style, security)
  - Score 1-10 per axis. If any axis < 7, regenerate with focus on that axis
  - Compare before/after scores to verify no regressions
  - Store learning in task_runs.result as structured data
- **Config surface:** Rubric definition in agents.json under supervisor config
- **Risk:** Low. Extends existing supervisor behavior with scoring instead of binary pass/fail.

#### 4. Codebase Map (citadel/codebase-map)
- **What:** Structural index -- files, exports, imports, dependency graph, roles. Keyword search.
- **Maps to:** Context building for all agent roles
- **Current gap:** Agents get prompt_packet but no structural awareness of the codebase they're modifying.
- **How it improves VibePilot:**
  - Generate codebase map at plan creation time
  - Inject relevant slices into task prompt_packets
  - Agents know what files exist, what imports they need, what patterns to follow
  - Update map on merge (post-task hook)
- **Config surface:** Map stored in Supabase or as a JSON file in the repo
- **Risk:** Low. Read-only enrichment of existing context.

#### 5. Systematic Debugging (citadel/systematic-debugging)
- **What:** Observe -> Hypothesize -> Verify -> Fix. No changes without confirmed root cause.
- **Maps to:** Task revision loop (`task-revision` node) + Tester role
- **Current gap:** Failed tests -> re-execute with "fix it" instruction. No structured diagnosis.
- **How it improves VibePilot:**
  - Add structured diagnosis step between test failure and re-execution
  - Agent must state hypothesis before attempting fix
  - Track hypotheses in task_runs for learning
- **Config surface:** revision_strategy in agents.json
- **Risk:** Low. Replaces free-form "fix it" with structured protocol.

#### 6. Context Guard (lilu/context-guard)
- **What:** Persistent context protection. Safeguard files survive sessions, rate limits, compaction.
- **Maps to:** The entire runtime/session management layer
- **Current gap:** Context is built per-session from scratch. No persistence of working context.
- **How it improves VibePilot:**
  - Save compressed context after each task completion
  - Restore on session restart instead of rebuilding from scratch
  - Critical for long-running multi-task plans
- **Config surface:** context_store in config or Supabase
- **Risk:** Medium. Needs careful invalidation logic when codebase changes.

#### 7. Structured Research (citadel/structured-research)
- **What:** Findings with confidence levels, 2-4 queries, 3-6 sources, persistent doc.
- **Maps to:** System Researcher agent + Research flow (Flow C)
- **Current gap:** Research is unstructured. No confidence scoring, no citation tracking.
- **How it improves VibePilot:**
  - Research outputs become structured: finding, confidence (high/medium/low), sources, contradictions
  - Supervisor evaluates confidence-weighted findings, not raw text
  - Persist in research/ directory with standard frontmatter
- **Config surface:** research_template in agents.json
- **Risk:** Low. Formats existing output.

#### 8. Daemon Campaigns (citadel/daemon-campaigns)
- **What:** Self-rescheduling trigger chain + watchdog recovery. Keeps campaigns alive across sessions.
- **Maps to:** Plan lifecycle + maintenance agent
- **Current gap:** Plans can stall if governor restarts mid-pipeline. No watchdog recovery.
- **How it improves VibePilot:**
  - Stale plan detection (plan with no activity for N hours)
  - Auto-restart stalled tasks
  - Watchdog timer on in-progress nodes
- **Config surface:** daemon config in plan_lifecycle.json
- **Risk:** Low. Adds resilience to existing flow.

---

### TIER 2 -- Patterns Worth Borrowing

#### 9. Shared Canon Memory (kevin-bigham/shared-canon-memory)
- **What:** Multi-agent shared memory for decisions, failed approaches, conventions.
- **Value:** Agents could share "we tried X and it failed" knowledge. Reduces repeated mistakes.
- **Implementation:** Supabase table `agent_memory` with agent_id, category, key, value, timestamp.

#### 10. Skill Drift Detector (matt-clawd/skill-drift-detector)
- **What:** Nightly audit for missing triggers, overlapping descriptions, oversized files.
- **Value:** VibePilot's prompts/ and config/ files can drift. Automated quality check.
- **Implementation:** Cron job that validates config files against schemas, checks prompt sizes.

#### 11. Cron Observability Stack (matt-clawd/cron-observability-stack)
- **What:** Lifecycle tracking, delta-based failure alerting, stale-run recovery.
- **Value:** Monitor VibePilot's own scheduled tasks (research, maintenance, cleanup).

#### 12. ITP + Parallel Agent Cost Saver (maxcoo/itp-parallel-agent-cost-saver)
- **What:** Token compression, prompt-cache economics, grouped parallel swarms.
- **Value:** Direct token cost reduction. We use free tiers -- compression means more tasks per session.

#### 13. Agent Fleet Sync (teddy/agent-fleet-sync)
- **What:** Multi-machine state replication. Auto-log changes, replicate fleet-wide.
- **Value:** When VibePilot runs on multiple machines (phone + laptop + cloud). Future-proofing.

#### 14. GitHub Issue & PR Auto-Triage (matt-clawd/github-issue-triage)
- **What:** Semantic analysis, configurable rules, label/priority/dedup.
- **Value:** VibePilot creates branches and PRs. Auto-triage would close the loop.

#### 15. Content Security Pipeline (matt-clawd/content-security-pipeline)
- **What:** Two-tier sanitize/score before storage. Prompt injection detection.
- **Value:** VibePilot accepts PRDs and research from external sources. Needs input sanitization.

#### 16. Error Alert Investigator (matt-clawd/error-alert-investigator)
- **What:** Root cause analysis for production alerts from Sentry/Datadog/PagerDuty.
- **Value:** When VibePilot has production monitoring. Not needed yet but pattern is good.

#### 17. Notification Priority Queue (matt-clawd/notification-priority-queue)
- **What:** 4-tier priority queue with batching, dedup, LLM classification.
- **Value:** Dashboard notifications. Don't flood the human with every event.

#### 18. 5-Pass Code Review (citadel/code-review-5pass)
- **What:** Correctness, security, performance, readability, consistency. Structured findings.
- **Value:** Upgrade supervisor review from binary to multi-axis scoring. Overlaps with #3 (Quality Loop).

#### 19. Project-Aware Scaffold (citadel/project-scaffold)
- **What:** Read codebase, find exemplars, generate matching files.
- **Value:** When creating new files, agents should follow existing patterns. Overlaps with #4 (Codebase Map).

#### 20. Kit Improver (giorgio/kit-improver)
- **What:** 3-source feedback loop + optional 3-agent audit for improving kits.
- **Value:** Meta-pattern for improving VibePilot's own pipeline definitions.

---

### NOT RELEVANT (Skipped)

These kits solve problems VibePilot doesn't have or are too domain-specific:
- Food Journal, Earnings Preview, Daily Brief, Vocabulary Builder (personal productivity)
- Expert Tweeter, Content Repurposer, SEO Optimizer (content marketing)
- Meltwater Lead Gen, Client Health Monitor, CRM (sales/CRM)
- LLM Fine-Tuning, Ghidra MCP (specialized ML/reverse engineering)
- Voice Transcription Loop, RSI Starter Loop, Kata Improvement (niche)
- Business Strategy Council, Innovation Scout, Competitive Pricing (enterprise)
- OpenClaw-specific kits (gateway watchdog, hook dev protocol, cron health)
- MSP Security Sweep, Bitdefender (IT ops)
- Rescue Fiction Manuscript, Contract Clause Extractor (legal/creative)

---

## Priority Implementation Order

Based on impact vs effort:

| Priority | Kit Pattern | Effort | Impact | Maps To |
|----------|-------------|--------|--------|---------|
| 1 | Codebase Map | Low | High | All agent context |
| 2 | Autonomous Quality Loop | Low | High | Supervisor scoring |
| 3 | Structured Research | Low | Medium | Researcher output format |
| 4 | Daemon Campaigns | Low | Medium | Plan resilience |
| 5 | Systematic Debugging | Low | Medium | Revision protocol |
| 6 | Council Lane Pattern | Medium | High | Council formalization |
| 7 | Self-Improve Harness | Medium | High | Learning loop |
| 8 | Context Guard | Medium | Medium | Session persistence |
| 9 | Shared Canon Memory | Medium | Medium | Cross-agent knowledge |
| 10 | ITP Cost Saver | Medium | Medium | Token optimization |

---

## Hard Constraints for Implementation

### The Planner Disaster Rule (April 2026)

An AI agent shortened planner and supervisor prompts to save tokens. System broke for two days because it wasn't using correct prompts. Root cause: agents editing their own instructions without evidence or approval.

**Constraint: Any file an agent reads at runtime must live on GitHub, version controlled.**

Skills follow the same pattern as prompts:
- Skills in `skills/*.md` on GitHub
- Governor syncs at startup alongside prompts
- Changes go through: propose (with evidence) -> supervisor review -> human approval -> git commit -> governor sync

**Self-improvement is not auto-editing.** The cycle is:
1. Agent runs N tasks, builds track record in task_runs
2. Agent spots pattern with data ("3/5 code reviews missed security with current prompt")
3. Agent proposes change to improvement_proposals with evidence
4. Supervisor reviews evidence against execution history
5. Human has final say on prompt/skill changes
6. Approved -> committed to GitHub -> picked up on next sync

An agent must demonstrate understanding before proposing changes. Not "I think this is better" but "here's the data showing X failed N times on pattern Y."

---

## Cross-Reference: Other Research Sources

This analysis complements findings from:
- **Lean-Ctx** (yvgude/lean-ctx) -- Rust binary for token compression. Complements Context Guard.
- **Agent Skills** (addyosmani/agent-skills) -- Lifecycle commands (/spec through /ship). Maps to VibePilot agent roles.
- **Archon v5** (coleam00/Archon) -- YAML DAG inspiration, git worktree isolation, refiner pattern.
- **Anthropic Agents** (April 2026 update) -- Orchestrator-Specialist pattern, context compaction. Validates VibePilot's architecture.

---

*This document should be updated whenever new kits are published or VibePilot components change.*
