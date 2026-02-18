# VibePilot Researcher Context

**Purpose:** Comprehensive context for System Researcher analyzing improvements for VibePilot.  
**Last Updated:** 2026-02-18 (Session 14)  
**Target:** Non-devs AND devs (broader than dev-only tools like Dan's Bowser)

---

## What VibePilot Is

Sovereign AI execution engine. Human provides idea → coordinated multi-agent system executes with zero drift.

**Not a chatbot.** A production code generator using Planner → Council → Supervisor → Orchestrator → Runners/Couriers pipeline.

**Future Vision (95% Confidence Goal):**
- One-shot success on weakest models with lowest limits
- Ephemeral agents (active during task, inactive after, reactivatable for revisions)
- Branch-per-task → Slice branch → Main workflow
- 95% confidence atomic tasks for weak models, internal models for complex tasks

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

## Research Tagging System (CRITICAL)

Every finding MUST be tagged. This determines approval flow.

### SIMPLE Tag
**Auto-approved, can be implemented immediately**

| What Qualifies | Examples | Data Required |
|----------------|----------|---------------|
| **New free web platform** for courier | New AI platform with free tier | Platform name, URL, free tier limits, login method |
| **New model to registry** (existing platform) | New model on OpenRouter | Model ID, context limits, pricing, rate limits |
| **Documentation update** | README improvement, research findings | Summary of change, why it helps |
| **Rate limit data** | Updated limits for existing model | Full rate limit breakdown |
| **Research findings** (information only) | Analysis of GitHub repo, video | What we learned, no implementation |

**SIMPLE Process:**
```
Research → Find finding → Tag SIMPLE → Commit to research-considerations → Maintenance implements
```

**Full Breakdown Required For:**
- **NEW platforms/models:** Context limits, pricing, rate limits (RPM/RPD/TPM), access methods, strengths/weaknesses, source URLs

---

### VET Tag
**Requires Council review + human approval before implementation**

| What Qualifies | Examples | Why VET |
|----------------|----------|---------|
| **Architecture changes** | Swapping orchestrator, changing data model | Affects whole system |
| **Component swaps** | Replacing Supabase, changing Git provider | Exit readiness impact |
| **New features** | Adding voice interface, changing courier implementation | Significant complexity |
| **Workflow changes** | Modifying pipeline stages, approval gates | Governance impact |
| **Cost structure changes** | New subscription strategy, pricing model | Financial impact |
| **Security changes** | Vault implementation, auth changes | Risk management |

**VET Process:**
```
Research → Find finding → Tag VET → Commit to research-considerations → Council reviews → Human approves/rejects → If approved → Planner creates tasks → Supervisor/Council vets plan → Orchestrator dispatches → Task agents execute → Supervisor validates alignment → Tester tests → Module test → Merge to main
```

**Note:** If finding has **visual UI/UX changes**, human must approve before merge even after testing passes.

---

## Current Architecture (v1.4)

### Pipeline Flow (NOW WORKING)
```
Human Idea → Consultant → PRD → Planner → Council → Supervisor → Orchestrator → Runners/Couriers
                                    ↓
                              Tasks: pending → available → in_progress → review → testing → approved → merged
```

### Task Execution & Branch Strategy (Future State)
```
1. Task created (Planner)
2. Branch: task/T001-task-name
3. Agent spawned (Runner/Courier) → executes → delivers output
4. Agent goes INACTIVE (not destroyed, context preserved)
5. Supervisor validates output vs. expected
6. Tester tests output
7. If pass: Merge to slice branch (e.g., auth-module)
8. When all slice tasks complete: Module test
9. If pass: Merge to main

If revision needed: Reactivate same agent (warm start, fast)
```

### Key Components

**Orchestrator ("Vibes"):**
- Watches task queue, routes to best available model
- Uses `access` table (links models to tools with limits, priority)
- Tracks ROI per model, per platform, per task type
- **Routing priority:** Web platforms (courier) → CLI subscriptions → APIs (last resort)
- **Smart routing:** Routes based on task complexity, model performance history

**Task Routing Flags (Q/W/M badges):**
Planner assigns flags for dispatch decisions:

| Flag | Badge | Meaning | Allowed Routes |
|------|-------|---------|----------------|
| `internal` | Q | Quality/internal - complex, needs codebase | CLI, API, MCP-IDE only |
| `web` | W | Safe for web free tier - single file | Any (courier preferred) |
| `mcp` | M | Route to user's IDE via MCP | MCP-IDE preferred, CLI fallback |

**When to flag Q (internal):**
- Multi-file changes (2+ existing files)
- Needs codebase awareness
- Creates new modules
- 4+ code_context dependencies

**Data Model:**
- `models_new`: AI models (capabilities only)
- `tools`: 4 tools (opencode, kimi-cli, direct-api, courier)
- `access`: Links models to tools with limits, usage, priority
- `task_history`: Learning data for orchestrator

---

## Current State

### Active Access (by priority)
| Priority | Model | Tool | Method | Status |
|----------|-------|------|--------|--------|
| 0 | kimi-internal | kimi-cli | subscription | Active |
| 0 | glm-5 | opencode | subscription | Active |
| 1 | gpt-4o, claude-* | courier | web_free_tier | Active |
| paused | gemini-*, deepseek-* | direct-api | api | Quota/credit exhausted |

### Rate Limits Researched

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
- Tier0: 3 RPM, 500K TPM, 1.5M TPD
- $1 minimum to activate

---

## What We're Looking For

### 15 Research Categories

| Category | What to Find | Why It Matters |
|----------|--------------|----------------|
| **AI/ML Models** | New LLMs, free tiers, APIs, local models (Ollama, llama.cpp) | Cost optimization, vendor diversity |
| **Agent Architecture** | Orchestration patterns, workflows, multi-agent, swarms | Improve our pipeline efficiency |
| **Browser Automation** | Playwright, Puppeteer, CDP, computer-use, accessibility tree | Courier improvements, token efficiency |
| **Infrastructure** | Supabase alternatives, hosting, CI/CD, serverless | Exit readiness, cost optimization |
| **Security** | Vault patterns, zero trust, sandboxing, RBAC | Protect user data |
| **Testing** | Validation, benchmarking, observability, monitoring | Quality assurance |
| **UI/UX** | Dashboard patterns, CLI design, developer experience | Vibes interface improvements |
| **Local/Edge** | On-device inference, offline, self-hosted, federated | Sovereignty, privacy |
| **Cost Optimization** | Caching, compression, quantization, MoE | Reduce token costs |
| **Data Management** | Vector DBs, RAG, embeddings, context handling | Improve memory/context |
| **API Integration** | REST, GraphQL, webhooks, MCP, protocols | Better integrations |
| **Workflow Management** | Task queues, schedulers, async, DAGs | Orchestrator improvements |
| **Learning Resources** | Tutorials, papers, benchmarks, best practices | Continuous improvement |
| **Community** | Open source, ecosystems, plugins, templates | Expand capabilities |
| **Performance** | Scaling, latency, throughput, profiling | Speed optimization |

---

## Key Research Findings (Recent)

### Token Efficiency Critical Learning

**From Indy Dev Dan/Bowser Research:**

| Approach | Method | Tokens/Step | Use Case |
|----------|--------|-------------|----------|
| **Vision-based** | Screenshot → OCR/Vision model | 2,000-5,000 | Current courier approach |
| **Accessibility Tree** | DOM structure as text | 100-300 | **Dan's approach - 10-50x better** |

**Quantified Impact:**
- Current (vision): 5M tokens/day (100 tasks × 10 steps × 5K)
- Proposed (tree): 200K tokens/day (100 tasks × 10 steps × 200)
- **Savings: ~$525/year** (DeepSeek pricing) + faster execution

**Implementation:**
```yaml
courier:
  method: accessibility_tree  # NOT vision
  steps:
    - snapshot  # Get DOM structure as text
    - click: e21  # Reference by ID
```

**Tag:** VET (changes courier implementation)

---

### Architecture Comparison: VibePilot vs. Indy Dev Dan

| Aspect | Indy Dev Dan (Bowser) | VibePilot | Key Difference |
|--------|----------------------|-----------|----------------|
| **Target User** | Developers only (CLI) | Non-devs AND devs | Broader accessibility |
| **Human Interface** | CLI commands | Natural language + Web UI | Lower barrier |
| **Planning** | Manual (YAML stories) | Automatic (PRD → tasks) | Autonomy |
| **Governance** | None | Council review | Safety |
| **Learning** | Static | Adaptive routing | Improvement over time |
| **Scope** | Browser automation | Full SDLC | Completeness |
| **Token Efficiency** | Accessibility tree (10-50x) | Currently vision | **Adopt Dan's approach** |
| **Agents** | Destroyed after task | Ephemeral (inactive, reactivatable) | Revision speed |

**Learning from Dan:**
- ✅ Token efficiency (accessibility tree for courier)
- ✅ YAML story format for courier tasks
- ✅ Layered architecture (validates our design)
- ✅ Parallel agent spawning

**VibePilot Advantages:**
- ✅ Planning (Dan requires manual YAML)
- ✅ Governance (Council review)
- ✅ Learning (adaptive routing)
- ✅ Multi-modal (not just browser)
- ✅ Web interface (non-dev accessible)

---

### Ephemeral Agents Pattern (Future State)

**Current:** Agent destroyed after task
**Proposed:** Agent goes INACTIVE (not destroyed)

```
Active → Inactive (dormant) → Reactivated (if revision needed)
         ↑___________________________|
         
Benefits:
- Context preserved for revisions
- Faster reactivation than cold start
- Resource suspended (no compute cost)
- Debugging possible
```

**Implementation:**
```python
class EphemeralAgent:
    def execute(self):
        self.state = 'active'
        result = run_task()
        self.state = 'inactive'  # Dormant, not destroyed!
        return result
    
    def revise(self, feedback):
        self.state = 'active'  # Warm start
        return revise_task(feedback)
```

**Tag:** VET (architecture change)

---

## Evaluation Framework

**Always ask:**
1. What can we LEARN from this? (not just "should we use it?")
2. Does it align with zero vendor lock-in?
3. Can we swap it in/out easily?
4. What's the 80% safe usage level?
5. What are the alternatives?

**Comparison Chart Required For:**
- Component swaps (old vs new approach)
- New tools vs existing tools
- Token cost comparisons
- Rate limit comparisons

**Pros/Cons Format:**
```
Pros:
- Specific benefit with quantified impact where possible
- Another benefit

Cons:
- Specific cost/drawback
- Another drawback

Trade-off: [Summary of key decision]
```

---

## Output Requirements

### For SIMPLE Findings
```
## Finding: [Brief name]
**Tag:** SIMPLE
**Category:** [One of 15 categories]

**What:** One sentence description
**Why:** Why it matters to VibePilot
**Data:** Key specs (if platform/model)
**Source:** [URL]

**Suggested Action:** Specific implementation
```

### For VET Findings
```
## Finding: [Brief name]
**Tag:** VET
**Category:** [One of 15 categories]

**Problem Solved:** What issue does this address?
**How It Works:** Technical explanation
**Relevance to VibePilot:** Specific applications

**Comparison:**
| Aspect | Current | Proposed | Impact |
|--------|---------|----------|--------|
| [Metric] | [Value] | [Value] | [Change] |

**Pros:**
- Benefit 1 (quantified if possible)
- Benefit 2

**Cons:**
- Drawback 1
- Drawback 2

**Trade-offs:** Summary of key decisions

**Learning Opportunities:**
- Pattern 1 we can adopt
- Pattern 2 we can adapt

**Recommendation:** Adopt/Adapt/Learn only
**Priority:** High/Medium/Low
**Source:** [URLs]
```

### Commit Requirements
```bash
git checkout research-considerations
git add docs/research/
git commit -m "Research: [topic] - [brief summary] - [SIMPLE/VET]"
git push origin research-considerations
```

**Must Include:**
- Full specs (context limits, pricing, rate limits for platforms/models)
- Source URLs (minimum 2)
- Relevance to VibePilot
- What we can learn/apply
- Pros/cons with trade-offs
- SIMPLE vs VET classification with justification

---

## Research Priorities (Current Focus)

| Priority | Area | Why | Tag Likely |
|----------|------|-----|------------|
| **1** | Courier token efficiency | 10-50x cost reduction | VET |
| **2** | New free web platforms | Expand courier targets | SIMPLE |
| **3** | Rate limit strategies | Respect 80% thresholds | VET |
| **4** | Accessibility tree approach | From Dan's research | VET |
| **5** | Local inference | Sovereignty (Ollama, etc.) | VET |
| **6** | Cost efficiency patterns | Caching, MoE, quantization | SIMPLE/VET |

---

*Remember: Research feeds continuous evolution. Find patterns, not just tools. Tag correctly (SIMPLE vs VET). Quantify impact where possible. Commit to research-considerations.*
