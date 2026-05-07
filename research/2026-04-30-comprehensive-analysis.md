# Comprehensive Analysis: VibePilot Improvements & Knowledgebase/Research Agent
**Date:** 2026-04-30 | **Updated with verification**
**Sources:** TODO.md, 5 Gemini conversations, live system state, codebase

---

## VERIFICATION STATUS (corrections from initial draft)

| Item | Initial Claim | Verified Reality |
|------|--------------|------------------|
| E2E pipeline | "NOT DONE" | DONE. Task d5823cd1 ran full loop: 3 attempts, 2 failures with feedback, 3rd passed, tested, merged. |
| Nemotron hanging | "Needs circuit breakers" | SOLVED. Fixed model IDs in routing.json (minimax/m2.5:free corrected). Routing working. |
| Graphifyy | Gemini recommended as tool | BUST. 1-star repo (AlokPrasad09/Graphifyy). Not the multi-modal KG tool Gemini described. Fabricated. |
| Cloudflare Mesh | New networking approach | Real concept but we already have cloudflared tunnel. Mesh = "many-to-many" upgrade. Not needed now. |
| ACP (Agent Client Protocol) | Gemini's new discovery | Real protocol. JSON-RPC 2.0 for Dashboard→Agent streaming. We should track but not implement yet. |
| Model pricing table | Gemini's detailed breakdown | UNVERIFIED. Likely fabricated/embellished prices. Must check actual provider sites. |
| Qwen 3.6 Coder | Listed as top model | UNVERIFIED. May not exist as described. Need to check OpenRouter. |
| "MUNCH Protocol" | J Gravelle's wire format | VERIFIED REAL. We USE jcodemunch v1.43 config (installed via pipx at ~/.local/bin/jcodemunch-mcp). SPEC_MUNCH.md is a published spec. v1.80.1 is latest. 45.5% median byte savings on top of 95% retrieval savings. User says J Gravelle now has token reduction for any external MCP server too -- needs verification if this is a new feature or the published MUNCH spec being adoptable by any server. |
| Context-OS / Tastematter | Tool for "stigmergic patterns" | UNVERIFIED. Gemini referenced YouTube video by Jacob Dietle. Need to verify independently. |

---

## PART 1: VibePilot System Improvements

### From TODO.md (Verified Against Current State)

#### DONE (Remove from list)
- ~~E2E Pipeline Test~~ -- VERIFIED DONE. Task d5823cd1 full loop completed.
- ~~Governor systemd service~~ -- VERIFIED DONE. Running as systemd user service.
- ~~Full fallback chain test~~ -- PARTIALLY DONE. Model IDs fixed, routing working.

#### STILL TODO -- MEDIUM PRIORITY (Now becomes HIGH before legacy ends)

#### HIGH PRIORITY

**3. Visual QA Agent** [TODO #5]
- Browser-use captures screenshots after dashboard changes.
- Compare to baseline, flag regressions.
- Present to human for UI/UX yes/no.
- My take: Directly supports the "human_review = UI/UX only" principle. Could leverage Hyperagent's $1000 credits for the image comparison part.

**4. Daily Landscape Research Cron** [TODO #6]
- Researcher prompt exists at `prompts/daily_landscape_researcher.md`.
- Needs wiring: cron → researcher → findings → supervisor review → config update.
- My take: This IS the research agent. This is the #1 deliverable for the knowledgebase workstream.

**5. Dashboard Model Management** [TODO #8]
- Dashboard shows models but can't add/edit them.
- Need self-service form that calls API.
- My take: Medium priority, but would make the system truly self-service.

#### MEDIUM PRIORITY

**6. LogAct Patterns** [TODO #9]
- Intent logging (record what agent PLANS before execution).
- Safety voter (cheap model cross-checks intent).
- Append-only task_events table.
- "Stupidity diagnosis" (agent reads own failed output, rewrites).
- My take: The append-only events table and intent logging are genuinely valuable. The "safety voter" is over-engineering for now.
- STATUS: Medium priority, but should be prioritized for the 3-day window. Directly improves VibePilot's own reliability.

**7. JourneyKits Implementation** [TODO #10]
- 95 kits scanned, 20 mapped to VibePilot gaps.
- Need review and decision on which patterns to adopt.
- My take: Should be a knowledgebase research task, not a coding task.

**8. .context/ Hooks Async** [TODO #11]
- Hooks rebuild knowledge layer synchronously, causing timeouts on x220.
- My take: Quick win, reduces friction every session.

**9. Upgrade jcodemunch** [NEW - discovered during verification]
- We're on v1.43 config, latest is v1.80.1.
- 37 versions behind. Missing: MUNCH compact encoding, AST pattern matching, PR risk profiling, symbol provenance, response secret redaction, embedding drift detection, auto-watch, and much more.
- These features directly improve VibePilot's code analysis and context efficiency.
- My take: Should be HIGH priority. Upgrade alone gives us massive capability improvements for free.

**10. MarkItDown (Microsoft Research)**
- Source: Gemini chat, VERIFIED REAL (github.com/microsoft/markitdown)
- Converts any file type (Word, Excel, PPT, PDF, images, audio) to clean Markdown for LLM ingestion.
- Has MCP server support.
- My take: Could be the ingestion layer for the knowledgebase. When researcher finds PDFs/docs, run through MarkItDown before storing.

**11. ACP (Agent Client Protocol)**
- Source: Gemini chat referencing YouTube video "MCP vs ACP"
- VERIFIED REAL protocol concept. JSON-RPC 2.0 for structured Dashboard→Agent streaming.
- MCP = Agent talks to tools. ACP = Dashboard talks to Agent.
- Would standardize how our dashboard receives governor/courier status updates.
- My take: Worth tracking. Our SSE bridge already does something similar but less standardized. Not urgent.

**12. File Handoff Pattern (85% Context Reduction)**
- Source: Gemini chat referencing Mansel Scheffel video on skill chaining
- Three techniques: Context forking, File Handoffs (disk as memory), Programmatic Placeholders.
- Demonstrated 51K → 8K token reduction.
- My take: Our .context/ layer already does retrieval optimization. The File Handoff pattern (write intermediates to /tmp/, pass paths not data) is something we should verify we're already doing or adopt. Directly relevant to post-May-2 weekly prompt cap.

**13. Karpathy's "Ratchet Loop" for Auto-Research**
- Source: Gemini chat
- Agent proposes change → scores against metric → keep if positive, revert if negative.
- My take: This IS the pattern for the research agent's model/config optimization loop. High value for knowledgebase workstream.

**14. Harness Engineering Concept**
- Source: Gemini chat
- Shift from prompt engineering to harness engineering. The harness is responsible for up to 6x performance difference.
- My take: This IS what VibePilot already is. Validates our approach. The research agent should track harness engineering patterns.

**15. Representation Engineering / Steering Vectors**
- Source: Gemini chat (Eigenvectors video)
- Interesting research but requires model internals we don't have via API.
- My take: DEFER. File under "future research" for knowledgebase.

**16. RL Rubric-Based Supervision**
- Source: Gemini chat (HuggingFace workshop)
- Grade agent output on rubric → use as RL signal. Small specialized models as "compiled expertise."
- My take: The rubric scoring is a good pattern for our supervisor. DEFER implementation but research the pattern.

**17. Talon (AI SOC) Architecture**
- Source: Gemini chat
- MCP-driven security agent. Containerized execution, Memory Palace pattern.
- My take: Not directly needed. The containerized execution model is worth noting for couriers.

**18. Cloudflare Mesh**
- Source: Gemini chat
- We already have cloudflared tunnel. Mesh = many-to-many upgrade for phone + x220 as single network.
- My take: Nice to have. We can remote into x220 via tunnel already. Not critical.

**19. Context-OS / Tastematter (Jacob Dietle)**
- Source: Gemini chat referencing YouTube video
- "Stigmergic pattern" -- agent learns by observing "desire paths" (frequently-used files).
- My take: UNVERIFIED tool. Concept aligns with our approach but need to verify independently before investing time.

---

## PART 2: Knowledgebase & Research Agent

### Architecture Decisions Needed

**A. Research Agent Core Design**

Based on all sources, the research agent should:

1. **Run on a schedule** (2x daily via GitHub Actions cron, as per existing design)
2. **Scan multiple sources**: GitHub trending, HuggingFace new models, provider changelogs, RSS feeds (sources.txt), YouTube channels (AI engineering), OpenRouter new models
3. **Score findings** against VibePilot's needs using a rubric:
   - Is it free or low-cost?
   - MIT/Apache license?
   - Does it fill a gap in our model routing?
   - Does it replace a paid dependency?
   - Is it relevant to our architecture (agents, orchestration, harness)?
4. **Write to knowledgebase repo** as Markdown+Frontmatter (as per existing design)
5. **Update map.json** index
6. **Trigger supervisor review** for high-impact findings
7. **Log everything** with Decision Log (why adopted, why rejected, when reconsidered)

**B. The "Ratchet Loop" for Research**

From the Karpathy analysis:
- Researcher proposes a model/tool change
- System scores it against ROI metric
- If positive: promote to testing → if passes, update routing.json
- If negative: log rejection reason, set reconsideration date
- This turns the knowledgebase from a passive archive into an active optimizer

**C. Bookmarklet for Human Input**

From prior sessions (approved design):
- Vanilla JS bookmarklet
- POSTs URLs/titles to API
- Zero auth (local network only)
- Captures things the human finds during the day
- Feeds into researcher's daily queue alongside automated sources
- My take: Essential for making the knowledgebase a "human + AI" system. The human finds things on YouTube/Reddit/Twitter, bookmarklet captures them, researcher evaluates and integrates.

**D. Knowledgebase Repo Structure**

From memory: VibesTribe/knowledgebase
- Markdown + Frontmatter for human-readability
- map.json for index
- Decision Log for every tool/model adoption/rejection
- sources.txt for RSS feed list

Frontmatter schema for each entry:
```yaml
---
title: "MiniMax M2.5"
category: model
status: adopted | rejected | watching | deprecated
date_researched: 2026-04-29
date_adopted: 2026-04-30  # if applicable
date_rejected: # if applicable
rejection_reason: # if applicable
reconsider_on: # date to re-evaluate
sources:
  - https://openrouter.ai/models
tags: [free, coding, openrouter]
related: [nemotron-3-super, qwen-3.6-coder]
decision_log_url: # link to decision entry
---
```

**E. Dashboard DOCS Tab (vis.js Graph)**

From memory: Shows adopted/rejected/watching relationships.
- Nodes = tools/models/patterns
- Edges = relationships (replaces, competes-with, depends-on)
- Status colors: green (adopted), red (rejected), yellow (watching), grey (deprecated)
- My take: This is the visual layer that makes the knowledgebase accessible. Should read from map.json and render in the dashboard.

### What Gemini Got Wrong / Over-Hyped

1. **"Sovereign Hybrid Cloud"** -- Buzzword salad. We're a local Postgres + GitHub stack. Not cloud.
2. **Graphify** -- Gemini recommended this but I haven't verified it exists as described. Need to check.
3. **"MUNCH Protocol" claims** -- Gemini kept citing jCodeMunch specs that may or may not exist. Verify before depending on.
4. **Steering Vectors** -- Cool research but not actionable for API-based models we use.
5. **Model prices/availability** -- Gemini's pricing table may be fabricated or outdated. Verify against actual provider sites.
6. **"Qwen 3.6 Coder"** -- Verify this actually exists. May be Qwen 3 with Gemini's embellishment.
7. **Context-OS / Tastematter** -- Interesting concept but verify it's a real tool before adding to research queue.

### Research Agent: Practical Implementation Plan

Given 3-day legacy window, what's buildable:

**Day 1 (Today):**
1. Set up knowledgebase repo structure (VibesTribe/knowledgebase)
2. Create frontmatter schema + first entries (seed with known models/tools)
3. Create sources.txt with RSS feeds
4. Build the bookmarklet (already designed, quick implementation)
5. Wire bookmarklet POST endpoint to governor API

**Day 2:**
1. Build researcher prompt (refine existing `prompts/daily_landscape_researcher.md`)
2. Set up GitHub Actions workflow for 2x daily cron
3. Researcher writes findings to knowledgebase repo
4. Wire map.json auto-update on commit

**Day 3:**
1. Dashboard DOCS tab with vis.js graph
2. Decision Log integration
3. Test the full loop: researcher finds → writes → dashboard shows → human reviews
4. First real research run

---

## PART 3: Priority Matrix (Updated for 3-Day Legacy Window)

### Must Do (Before May 2 -- VibePilot Infrastructure)
These improve VibePilot ITSELF -- the factory, not the products:

- [ ] **Upgrade jcodemunch v1.43 → v1.80.1** -- Free massive capability boost (MUNCH encoding, AST patterns, risk profiles, provenance)
- [ ] **LogAct intent logging + append-only events** -- Improves VibePilot reliability
- [ ] **JourneyKits review** -- 20 patterns mapped to gaps, need decisions
- [ ] **.context/ hooks async fix** -- Quick win, reduces friction
- [ ] **Knowledgebase repo setup + seed data** -- VibesTribe/knowledgebase
- [ ] **Bookmarklet implementation** -- Human input into researcher
- [ ] **Researcher prompt refinement** -- `prompts/daily_landscape_researcher.md`
- [ ] **MarkItDown integration** -- Ingestion layer for knowledgebase

### Should Do (This Week -- Knowledgebase + Research Agent)
- [ ] **GitHub Actions cron for researcher** -- 2x daily automated research
- [ ] **Decision Log system** -- Why adopted/rejected/reconsidered
- [ ] **Ratchet loop pattern for config optimization** -- Research agent's scoring mechanism
- [ ] **Full fallback chain stress test** -- Verify Groq, NVIDIA NIM, OpenRouter all work
- [ ] **Dashboard DOCS tab (vis.js)** -- Visual knowledgebase browser
- [ ] **sources.txt RSS feed curation** -- What the researcher scans

### Nice to Have (Next 2 Weeks)
- [ ] Dashboard model management (self-service forms)
- [ ] ACP protocol investigation (dashboard ↔ agent streaming)
- [ ] File Handoff pattern verification/adoption
- [ ] Visual QA agent (screenshot comparison)
- [ ] RL rubric scoring for supervisor
- [ ] Cloudflare Mesh (remote terminal from phone)

### Research Only (Track, Don't Build)
- [ ] Context-OS / Tastematter (verify it exists first)
- [ ] Steering vectors / Representation Engineering
- [ ] Hyperagent as Tier 2 courier (burn free credits for heavy tasks)
- [ ] Small model "compiled expertise" training
