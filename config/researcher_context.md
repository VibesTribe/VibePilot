# VibePilot Researcher Context

**Purpose:** Compressed context (~3K tokens) for any model doing research. Model-agnostic, swappable.

---

## What VibePilot Is

Sovereign AI execution engine. Human provides idea → VibePilot executes with zero drift.

Not a chatbot. Not a wrapper. A production code generator using coordinated multi-agent execution.

---

## Core Principles (Inviolable)

1. **Zero vendor lock-in** - Everything swappable. Never dependent on one tool/model/provider.
2. **Build for change, not adoption** - Design for the next version, not current convenience.
3. **If it can't be undone, it can't be done** - Reversible always. No one-way doors.
4. **Everything through config** - New model/platform = config edit, not code change.
5. **Sovereign** - We own our data, our code, our destiny.

---

## Architecture (How It Works)

### Orchestrator
- Routes tasks to runners based on Q/W/M flags + scoring
- Q = Quick (simple models), W = Web (courier), M = MCP (tool-capable)
- Tracks success/failure, adjusts routing over time
- Handles cooldowns and quota exhaustion automatically

### Courier
- Browser automation (Playwright + browser-use) to leverage free web AI platforms
- Navigates to ChatGPT web, Claude web, Gemini web, etc.
- LLM driver sees the page, decides actions
- Currently blocked by: Gemini quota exhausted, DeepSeek needs credit

### Runners
- Contract interface - any model can run tasks
- API runners (Kimi, DeepSeek, Gemini, OpenRouter)
- Courier runner (web platforms)
- Return standardized result format with tokens, duration, success/fail

### ROI Tracking
- Per-model cost and quality tracking
- Per-platform efficiency metrics
- Informs routing decisions
- Dashboard shows real-time performance

### Supervision Flow
- SIMPLE changes: Auto-approved → Maintenance implements
- VET changes: Supervisor → Council (3 models) → Human approval → Maintenance

---

## Current State

**Working:**
- Orchestrator core functionality
- Runners (Kimi, DeepSeek, Gemini APIs)
- Courier browser automation (navigation works)
- Dashboard connected to Supabase
- Error handling for quota/credit exhaustion

**Blocked:**
- Courier LLM driver (Gemini quota, DeepSeek credit)
- Full end-to-end task execution via courier

**Active Focus:**
- Get courier fully operational
- Improve routing intelligence
- Research system (you are part of this!)

---

## What We're Looking For

Research should find things that could improve VibePilot:

| Category | Examples |
|----------|----------|
| **Routing improvements** | Better scoring, smarter model selection, cost optimization |
| **Courier robustness** | Visual automation techniques, error recovery, multi-platform patterns |
| **Cost efficiency** | MoE patterns, caching strategies, token optimization |
| **Quality control** | Output validation, error detection, recovery patterns |
| **New approaches** | Novel architectures, techniques that align with our principles |

---

## Important Context

- We are NOT invested in any specific tool or model
- We evaluate: "What can we LEARN from this?" not "Should we USE this?"
- Everything we build must be swappable
- Recommendations should specify if they're SIMPLE or VET (system change)

---

## Output Format

Follow `config/prompts/researcher.md` for Daily Briefing format.

Tag each finding as:
- **SIMPLE** - Minor add/update, auto-approved
- **VET** - System change, needs Council review

Include JSON suggestion blocks for Maintenance.
