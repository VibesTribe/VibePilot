# Model Routing System

**Status:** Active documentation
**Last updated:** 2026-05-06
**Scope:** How VibePilot selects models and connectors for task execution

---

## Overview

When a task needs to be executed, the governor must decide: which model, on which connector, and is it available right now? This system spans live DB data, 3 config files, 4 Go modules, and an auto-discovery scanner that keeps everything current.

---

## The Data Sources (Live vs Static)

### models.json (Live, auto-generated)

Location: governor/config/models.json

This file is now **auto-generated every 30 minutes** by TokenFinder v2 (governor/scripts/tokenfinder_v2.py). It reads from the `model_catalog` PostgreSQL table which contains verified free models from 4 providers.

Each model has:
- id: matches what the provider API expects (e.g. "gemini-2.5-flash-lite", "groq/llama-3.3-70b-versatile")
- access_via: which connectors can reach this model (mapped from provider: openrouter→openrouter-api, etc.)
- context_limit: max tokens the model can handle
- capabilities: code, reasoning, instruction, vision, embedding, text
- status: "active" or "benched"

The old static models.json has been renamed to models.json.legacy for reference.

### model_catalog (Source of Truth)

Location: PostgreSQL table `model_catalog`

This is the live source. Updated every 30 min by TokenFinder. Contains:
- 194 verified free models across OpenRouter, Groq, NVIDIA, Gemini
- Standard API pricing for ROI calculator (pricing_input/pricing_output)
- Rate limits with scope info (per_key, org, account, per_model+account)
- Capability tags using task_routing.json detection rules
- Cooldown state (GLM 5.1 benched until May 8)

### models_live.json (Human-readable snapshot)

Location: governor/config/models_live.json

Same data as model_catalog but in a human-readable JSON format with pricing and rate limits.

### connectors.json (Static, manual)

Location: governor/config/connectors.json

Defines every API endpoint, CLI tool, and web platform we can use. Still manually managed because connectors change rarely.

### routing.json (Live cascade, semi-auto)

Location: governor/config/routing.json

The free_cascade strategy is updated to match live model IDs. Currently manually maintained but designed to be auto-generated.

### Task Routing Rules (Config-driven)

Location: governor/config/task_routing.json

Maps task types (coding, analysis, embedding, image_generation, research, general) to required capabilities. Governs which models the router considers for each task type.

---

## Auto-Discovery: TokenFinder v2

Every 30 minutes, TokenFinder v2:

1. Scans OpenRouter (29 free models), Groq (12), NVIDIA (134), Gemini (26)
2. Tags capabilities using task_routing.json rules (code, reasoning, instruction, vision, embedding)
3. Stores standard API pricing for ROI calculations
4. Stores rate limits with scope information
5. Benchmarks old models not found in latest scan
6. Writes models.json for the governor
7. Pushes all changes to GitHub

This means models are never more than 30 minutes stale. New free models from OpenRouter are discovered within 30 minutes of appearing.

---

## The Three Config Files (Legacy Reference)

### 1. connectors.json (Where we can reach models)

Location: governor/config/connectors.json
Purpose: Defines every API endpoint, CLI tool, and web platform we can use

Each connector has:
- id: unique identifier (e.g. "gemini-api-general", "groq-api", "nvidia-api", "openrouter-api")
- type: "api" (HTTP endpoint), "cli" (local command), or "web" (browser platform)
- status: "active", "inactive", or "disabled"
- models_available: which models this connector can access
- rate_limits: requests per minute/day, tokens per minute/day
- api_key_ref: which vault key to use (for API connectors)
- provides_tools: what capabilities the connector offers (read, write, bash, etc)

### 2. routing.json (How we pick)

Location: governor/config/routing.json
Purpose: Defines routing strategies and agent restrictions

Key section: strategies.free_cascade
- Ordered list of 15 models in priority order (updated for live data)
- Router tries them in order, skipping any that are in cooldown
- Round-robin rotation distributes load across models

Agent restrictions:
- internal_only: planner, supervisor, council, orchestrator, maintenance, watcher, tester
- default: consultant, researcher, courier, task_runner

### 3. task_routing.json (What capabilities each task needs)

Location: governor/config/task_routing.json
Purpose: Maps task types to required model capabilities

Task types:
- coding → requires "code" capability
- analysis → requires "reasoning"
- embedding → requires "embedding"
- image_analysis → requires "vision"
- research → requires "reasoning" + "instruction"
- general → requires "instruction"

Capability detection uses regex patterns AND family assumptions (e.g., all gemini-flash models get code+reasoning automatically).

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

## What's Now Fixed (Previously Known Gaps)

| Gap | Status | How |
|-----|--------|-----|
| No scanner verifies models on provider APIs | **FIXED** | TokenFinder v2 scans all 4 providers every 30 min |
| No session-start health check | **FIXED** | TokenFinder runs on startup + every 30 min via cron |
| No auto-update mechanism for config | **FIXED** | TokenFinder auto-generates models.json + pushes to GitHub |
| No documentation of this system | **FIXED** | This document |

---

## Config File Locations

| File | Path | Purpose |
|------|------|---------|
| model_catalog | PostgreSQL table | Live source of truth, 194 verified free models |
| models.json | governor/config/models.json | Auto-generated from model_catalog every 30 min |
| models.json.legacy | governor/config/models.json.legacy | Original static config (preserved for reference) |
| models_live.json | governor/config/models_live.json | Human-readable snapshot with pricing |
| connectors.json | governor/config/connectors.json | Where we reach models |
| routing.json | governor/config/routing.json | How we pick models (cascade) |
| task_routing.json | governor/config/task_routing.json | Task-to-capability mapping |
| agents.json | governor/config/agents.json | Agent definitions + model pins |
| system.json | governor/config/system.json | Global settings |
| tokenfinder_v2.py | governor/scripts/tokenfinder_v2.py | Auto-discovery scanner |

---

## Quick Reference: How Models Stay Current

Nothing to do manually. TokenFinder runs every 30 min:

```bash
# Check scan results
psql -d vibepilot -c "SELECT * FROM provider_scan_state ORDER BY last_scan_at DESC;"

# View active models
psql -d vibepilot -c "SELECT provider, COUNT(*) FROM model_catalog WHERE status = 'active' GROUP BY provider;"

# Check governor routing logs
journalctl --user -u vibepilot-governor --since "1 hour ago" | grep "\[Router\]"

# Check cooldowns
psql -d vibepilot -c "SELECT id, status, status_reason FROM model_catalog WHERE status = 'benched';"
```

## Quick Reference: Force a Scanner Run

```bash
cd ~/vibepilot && python3 governor/scripts/tokenfinder_v2.py
```

## Quick Reference: Restart Governor After Config Changes

```bash
systemctl --user restart vibepilot-governor
```
