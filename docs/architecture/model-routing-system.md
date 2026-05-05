# Model Routing System

**Status:** Active documentation
**Last updated:** 2026-05-05
**Scope:** How VibePilot selects models and connectors for task execution

---

## Overview

When a task needs to be executed, the governor must decide: which model, on which connector, and is it available right now? This system spans 3 config files, 4 Go modules, and makes decisions in under 100ms.

---

## The Three Config Files

### 1. connectors.json (Where we can reach models)

Location: governor/config/connectors.json
Purpose: Defines every API endpoint, CLI tool, and web platform we can use

Each connector has:
- id: unique identifier (e.g. "gemini-api-courier", "groq-api", "hermes")
- type: "api" (HTTP endpoint), "cli" (local command), or "web" (browser platform)
- status: "active", "inactive", or "disabled"
- models_available: which models this connector can access
- rate_limits: requests per minute/day, tokens per minute/day
- api_key_ref: which vault key to use (for API connectors)
- provides_tools: what capabilities the connector offers (read, write, bash, etc)

Key distinction: Connectors are DESTINATIONS. A model might be available on multiple connectors (e.g. gemini-2.5-flash is on 4 Gemini API connectors with different keys).

### 2. models.json (What models we know about)

Location: governor/config/models.json
Purpose: Profiles every model with its capabilities, limits, costs, and learned data

Each model has:
- id: matches what the provider API expects (e.g. "gemini-2.5-flash-lite")
- access_via: which connectors can reach this model
- context_limit: max tokens the model can handle
- rate_limits: model-specific limits (may differ from connector limits)
- api_pricing: cost per million tokens (0 for free)
- learned: performance data (avg duration, failure rates, best task types)
- status: "active", "benched", "inactive"

The "learned" field is populated by the governor over time. It tracks which models perform best for which task types. This data feeds into routing decisions.

### 3. routing.json (How we pick)

Location: governor/config/routing.json
Purpose: Defines routing strategies and agent restrictions

Key section: strategies.free_cascade
- Ordered list of 19 models in priority order
- Router tries them in order, skipping any that are in cooldown
- Round-robin rotation distributes load across models

Agent restrictions:
- internal_only: planner, supervisor, council, orchestrator, maintenance, watcher, tester (never use web platforms)
- default: consultant, researcher, courier, task_runner (can use web or internal)

Selection criteria:
- status: "active" (skip inactive/benched)
- not_at_limit: respect rate limits
- prefer_learned_best: use learned scores to prefer proven models

---

## The Four Go Modules

### 1. router.go (The Decision Maker)

Entry point: Router.SelectRouting(ctx, req) -> RoutingResult

Flow:
1. If task flagged "internal", skip web routing entirely
2. If no courier agent available, skip web routing
3. Try web routing first (for courier tasks):
   - Check if destination platform is available
   - Check courier model + connector availability
   - Check platform rate limits
4. Fall back to internal routing:
   - Check if agent has a pinned model (from agents.json)
   - If pinned model available, use it
   - Otherwise, use cascade from routing.json

Cascade selection (selectByCascade):
1. Get model list from free_cascade strategy
2. Round-robin rotation (each call starts from different position)
3. For each model in cascade:
   - Find a connector that can reach it
   - Check UsageTracker.CanMakeRequestVia (rate limits + cooldown)
   - If available, add to candidates
4. Score candidates:
   - Primary: learned score (0-1, from past performance on this task type)
   - Tiebreaker: lowest load (fewest requests this minute)
5. Return best candidate
6. If ALL models in cooldown -> fall back to Hermes connector (glm-5, always available)

Token estimation (EstimateTokens):
- Prose content: ~4 chars/token
- Code-heavy content: ~3 chars/token (detected by structural character ratio)
- Response budget varies by role: planner 2x input, supervisor 0.5x, courier 0.25x
- Compared against model context_limit to skip models that can't fit the task

### 2. usage_tracker.go (The Rate Limit Enforcer)

Key method: CanMakeRequestVia(ctx, modelID, connectorID, estimatedTokens) -> RequestDecision

Checks (in order):
1. Model-level cooldown: has this model been rate-limited recently?
2. Model-level rate limits: RPM/RPD from models.json
3. Connector-level rate limits: shared TPD/RPD from connectors.json
4. Token budget: will this request exceed remaining tokens?
5. Context window: can the model fit estimatedTokens?

Returns: {CanProceed: bool, Reason: string, WaitTime: duration}

Also provides:
- GetMinuteRequestCount: how many requests this model got in the last minute (for load balancing)
- GetModelLearnedScore: performance score for model+taskType combination (for scoring)

### 3. connector_tracker.go (The Connector Health Monitor)

Tracks per-connector usage:
- Registers connectors with their shared limits (e.g. Groq org-level 100K TPD)
- Records token usage per request (input + output tokens)
- Maintains sliding windows: RPM (1-min), RPH (1-hour), RPD (1-day), TPM, TPD
- Checks if a connector can handle a request given its current usage
- Persists state to database, loads on restart

The key insight: Groq has ORG-LEVEL shared limits. All Groq models share the same 100K TPD pool. The connector tracker ensures we don't exceed this across all models.

### 4. cooldown_watcher.go (The Recovery Prober)

Background goroutine that runs every 2 minutes:
1. Finds models whose cooldown recently expired
2. Sends a lightweight health probe via the connector
3. If probe fails: extends cooldown (don't let dead models cycle)
4. If probe succeeds: logs confirmation, model becomes available for routing

This prevents the "cooldown loop" problem: model fails -> cooldown -> cooldown expires -> router tries it -> fails again -> repeat forever. The watcher verifies recovery before the router trusts the model again.

---

## The Full Flow: Task to Execution

```
Task arrives
    |
    v
Router.SelectRouting(role, taskType, estimatedTokens)
    |
    v
Is agent restricted to internal only? --yes--> Skip web
    |
    no
    v
Courier available? --no--> Internal only
    |
    yes
    v
Try web routing (courier + destination platform)
    |
    v
Web available? --yes--> Return web result
    |
    no
    v
Agent has pinned model? --yes--> Check pin availability
    |                              |
    | available                    unavailable
    v                              v
Use pinned model              Try cascade (19 models)
                                   |
                                   v
                              Round-robin rotation
                                   |
                                   v
                              For each model:
                                1. Find connector
                                2. Check rate limits
                                3. Check cooldown
                                4. Check token budget
                                   |
                                   v
                              Score candidates:
                                - Learned score (primary)
                                - Load balance (tiebreaker)
                                   |
                                   v
                              Best candidate returned
                                   |
                              All in cooldown?
                                   |
                                   v
                              Fall back to Hermes (glm-5)
```

---

## What's Missing (Known Gaps)

1. No scanner verifies models still exist on provider APIs
   - models.json was last manually curated April 30
   - Models could have been removed and we'd only discover on task failure

2. No session-start health check
   - CooldownWatcher probes on 2-min intervals but doesn't do full availability scan
   - A model could be gone for hours before cooldown cycle catches it

3. No auto-update mechanism for config
   - New free models appear on OpenRouter regularly
   - Requires manual JSON editing to add them

4. No documentation of this system (until now)
   - Multiple people have asked "how does model selection work"
   - Knowledge was tribal, spread across config files and Go code

These gaps are addressed by the KB Intelligence Strategic Plan v2, Phase 1.

---

## Config File Locations

| File | Path | Purpose |
|------|------|---------|
| connectors.json | governor/config/connectors.json | Where we reach models |
| models.json | governor/config/models.json | What models we know |
| routing.json | governor/config/routing.json | How we pick models |
| agents.json | governor/config/agents.json | Agent definitions + model pins |
| system.json | governor/config/system.json | Global settings |

Note: CONFIG lives in governor/config/ (set by GOVERNOR_CONFIG_DIR env var). The config/ directory at repo root is a copy and may be stale.

---

## Quick Reference: Adding a New Model

1. Add connector entry in connectors.json (if new provider)
2. Add model profile in models.json with access_via pointing to connector
3. Add model ID to routing.json free_cascade (in priority position)
4. Restart governor: systemctl --user restart vibepilot-governor
5. Verify: check governor logs for "[Router] Cascade routing" showing new model

## Quick Reference: Removing a Model

1. Set status: "inactive" in models.json (don't delete, keep for history)
2. Remove from routing.json free_cascade
3. Restart governor
4. Alternatively: set status: "benched" to keep it discoverable but unused

## Quick Reference: Checking Model Health

```bash
# Governor logs showing routing decisions
journalctl --user -u vibepilot-governor --since "1 hour ago" | grep "\[Router\]"

# Cooldown state in database
psql -d vibepilot -c "SELECT model_id, connector_id, cooldown_until FROM model_cooldowns WHERE cooldown_until > now();"

# Usage tracker state
psql -d vibepilot -c "SELECT * FROM connector_usage ORDER BY connector_id;"
```
