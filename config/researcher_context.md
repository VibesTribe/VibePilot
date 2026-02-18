# VibePilot Researcher Context

**Purpose:** Compressed context for any model doing research on VibePilot improvements.
**Last Updated:** 2026-02-18

---

## What VibePilot Is

Sovereign AI execution engine. Human provides idea → coordinated multi-agent system executes with zero drift.

**Not a chatbot.** A production code generator using Planner → Council → Supervisor → Orchestrator → Runners/Couriers pipeline.

---

## Core Philosophy (Inviolable)

| Principle | Meaning |
|-----------|---------|
| **Zero vendor lock-in** | Everything swappable. Never dependent on one provider. |
| **Modular & swappable** | Change one component, nothing else breaks. |
| **Exit ready** | Pack up, hand over to anyone. All code in GitHub, state in Supabase. |
| **Reversible** | If it can't be undone, it can't be done. |
| **Always improving** | New ideas evaluated daily. Research feeds continuous evolution. |

**Strategic Mindset:**
- Backwards planning: Start with the dream, work back to today
- Options thinking: Many paths up the mountain, always have alternatives
- Preparation over hope: Consider every scenario before acting

---

## Current Architecture (v1.4)

### Pipeline Flow (NOW WORKING)
```
Human Idea → Consultant → PRD → Planner → Council → Supervisor → Orchestrator → Runners/Couriers
                                    ↓
                              Tasks: pending → available → in_progress → review → testing → approved → merged
```

### Key Components

**Orchestrator ("Vibes"):**
- Watches task queue, routes to best available model
- Uses new `access` table (links models to tools with rate limits, priority)
- Tracks ROI per model, per platform, per task type
- Routing priority: Web platforms (courier) → CLI subscriptions → APIs (last resort)

**Task Routing Flags (Q/W/M badges):**
Planner assigns each task a routing flag that Orchestrator uses for dispatch decisions:

| Flag | Badge | Meaning | Allowed Routes |
|------|-------|---------|----------------|
| `internal` | Q | Quality/internal control - too complex for web free tier | CLI, API, MCP-IDE only |
| `web` | W | Safe for web free tier - single file, no deps | Any (courier preferred, can overflow to CLI/API) |
| `mcp` | M | Route to user's connected IDE via MCP | MCP-IDE preferred, CLI fallback |

**When to flag `internal` (Q):**
- Multi-file changes (2+ existing files)
- Needs codebase awareness
- Creates new modules requiring imports
- Task has 4+ code_context dependencies

**Why flags matter:**
Web platforms (ChatGPT, Claude web) lose context every message and can't browse codebase. Q-flagged tasks are contractually kept away from courier/web platforms. Dashboard shows these badges for transparency.

**Data Model (Redesigned 2026-02-18):**
- `models_new`: 10 AI models (capabilities only)
- `tools`: 4 tools (opencode, kimi-cli, direct-api, courier)
- `access`: 15 records linking models to tools with limits, usage, priority
- `task_history`: Learning data for orchestrator

**Runners vs Couriers:**
- **Runners:** CLI/API execution (Kimi CLI, OpenCode, DeepSeek API, Gemini API)
- **Couriers:** Browser automation to leverage free web platforms (ChatGPT, Claude, Gemini web)

---

## Current State

### Active Access (by priority)
| Priority | Model | Tool | Method | Status |
|----------|-------|------|--------|--------|
| 0 | kimi-internal | kimi-cli | subscription | Active (7 days left at $0.99) |
| 0 | glm-5 | opencode | subscription | Active |
| 1 | gpt-4o, claude-* | courier | web_free_tier | Active |
| paused | gemini-*, deepseek-* | direct-api | api | Quota/credit exhausted |

### Working
- Orchestrator core with new schema
- Pipeline auto-flow (pending → available transitions working)
- RunnerPool loads from `access` table
- Rate limit tracking infrastructure

### In Progress
- Rate limit checking before dispatch (80% threshold)
- Usage tracking after task completion
- Orchestrator as continuous service (not manual start)

### Research Priorities

| Area | Why | Current Gap |
|------|-----|-------------|
| **Free tier models** | Cost optimization | Need more courier targets |
| **Rate limit strategies** | Respect 80% thresholds | Multi-window tracking |
| **Browser automation** | Courier robustness | LLM driver blocked by quotas |
| **Cost efficiency** | Maximize free tier usage | MoE patterns, caching |
| **Local inference** | Sovereignty | Llama.cpp, Ollama integration |

---

## Rate Limits Researched

**Gemini API Free Tier:**
| Model | RPM | RPD | TPM |
|-------|-----|-----|-----|
| gemini-2.5-flash | 10 | 250 | 250K |
| gemini-2.5-flash-lite | 15 | 1000 | 250K |
| gemini-1.5-flash | 15 | 1500 | 1M |

**DeepSeek API:**
- Dynamic throttling (no fixed limits)
- 5M free tokens for new users
- $0.28/1M input, $0.42/1M output

**Kimi/Moonshot API:**
- Tier-based (Tier0: 3 RPM, 500K TPM, 1.5M TPD)
- $1 minimum to activate

---

## What We're Looking For

Research should find things that improve VibePilot within our principles:

| Category | Examples |
|----------|----------|
| **New free tiers** | Web platforms, beta APIs, promotional access |
| **Rate limit optimization** | Better tracking, 80% threshold strategies, window management |
| **Courier targets** | New web platforms with free tiers |
| **Browser automation** | Visual techniques, error recovery, anti-detection |
| **Local inference** | Lightweight models, edge deployment, offline capability |
| **Cost efficiency** | Token optimization, caching, MoE patterns |
| **Quality control** | Output validation, error detection, self-correction |
| **Swap alternatives** | Replacements for any component (tools, DB, frameworks) |

---

## Evaluation Framework

**Always ask:**
1. What can we LEARN from this? (not just "should we use it?")
2. Does it align with zero vendor lock-in?
3. Can we swap it in/out easily?
4. What's the 80% safe usage level?
5. What are the alternatives?

**Tag findings:**
- **SIMPLE** - Minor add/update (e.g., new model to registry)
- **VET** - System change, needs Council review (e.g., architecture change)

---

## Output Requirements

Commit all research to `research-considerations` branch:
```bash
git checkout research-considerations
git add docs/research/
git commit -m "Research: [topic] - [brief summary]"
git push origin research-considerations
```

Include in findings:
- Full specs (context limits, pricing, rate limits)
- Source URLs (minimum 2)
- Relevance to VibePilot
- What we can learn/apply
- Pros/cons with trade-offs
- SIMPLE vs VET classification
