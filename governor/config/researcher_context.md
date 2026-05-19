# VibePilot Researcher Context - Current System State

This file describes what VibePilot currently has and how it works.
Researchers and council members MUST read this before evaluating any finding.

Last verified: 2026-05-18

## Hardware (Non-Negotiable)

- ThinkPad X220, i5-2520M (AVX only, no AVX2)
- 16GB RAM shared across all services
- Spinning HDD (I/O is the bottleneck)
- CANNOT run: Ollama, Docker containers, local LLMs, anything needing AVX2
- CAN run: Go binaries (~30MB), PostgreSQL, Node.js, Python services

## Core Architecture

- Go governor binary (single process, low memory, PostgreSQL backend via pgx)
- PostgreSQL database (all state, no Supabase dependency)
- Next.js dashboard on Vercel (auto-deploys from GitHub main branch)
- Knowledge base: GitHub-hosted markdown with MCP server for agent queries
- Hermes agent gateway: multi-platform chat/CLI agent with tool access
- Cloudflare tunnel: exposes api.vibestribe.rocks, graphs.vibestribe.rocks

## Pipeline (How Tasks Get Done)

1. PRD written (by human or consultant agent)
2. Planner agent creates plan with tasks
3. Tasks dispatched to courier agents (GitHub Actions runners)
4. Courier executes code changes on feature branches
5. Supervisor reviews output
6. Human reviews and merges (or dashboard Review Hub)

## Model Access (What We Use Now)

### API Connectors (automated, governor-managed)
- Z.AI GLM-5: primary model, free subscription through June/July 2026
- Gemini API (3 keys): general, researcher, visual QA (free tier, 20 req/day per key)
- Groq API: Llama 4 Scout, Qwen 3 32B, Llama 3.3 70B (free tier, fast inference)
- NVIDIA NIM: DeepSeek V3, Nemotron models (free tier)
- OpenRouter: 20+ free models (free only, never paid, $18 lesson learned)
- DeepSeek API: out of credits, only via NVIDIA NIM fallback

### Web Destinations (courier agents, browser-harvested)
- ChatGPT, Claude, Gemini Web, Kimi, DeepSeek Web, Qwen Web, Mistral Web
- NotegPT, Poe, Perplexity, HuggingChat, AiZolo, Chatbox

### Model Catalog
- 175 entries tracked in model_catalog table
- TokenFinder scanner verifies free model availability every 30 minutes
- Auto-benches models that fail rate limits, auto-unbenches after cooldown

## Research Pipeline (How Research Gets Done)

1. Researcher agent scans landscape (models, platforms, tools, pricing)
2. Each finding becomes a research_suggestion with status pending
3. Suggestions collected into daily research_reports
4. Report routed to 3-member council (user_alignment, architecture, cost lenses)
5. Council votes per-item: approve/watch/reject
6. Approved items go to Review Hub for human decision
7. Human approves -> EventResearchApproved -> consultant generates PRD
8. PRD enters normal pipeline (planner -> tasks -> courier -> review)

## Council System
- 3 members with different lenses: user_alignment, architecture, feasibility
- Sequential review (member 1, then 2, then 3 with prior votes visible)
- Research council gets system context via KB context pack
- Output: per-item JSON with vote, reasoning, concerns

## Review Hub (Dashboard)
- Shows pending_human items from research_reports
- Human can approve, watch, or reject each item
- Approved items trigger consultant -> PRD -> pipeline
- Rejected/watched items are archived
- Items with human_decision set do NOT reappear (reviewed_at timestamp set)

## Key Files and Paths
- Governor binary: ~/vibepilot/governor/governor (systemd service vibepilot-governor)
- Config dir: ~/vibepilot/governor/config/ (models.json, agents.json, connectors.json, platforms.json)
- PRD templates: ~/vibepilot/governor/config/templates/prd-template.md
- Prompts: ~/vibepilot/governor/config/prompts/ (per-agent .md files)
- Knowledge base: ~/knowledgebase/ (git repo, MCP server on port 8888)
- Research output: ~/knowledgebase/research/
- PRDs: ~/vibepilot/docs/prd/

## Cost Model
- All API access is free tier or free subscription
- GLM-5 via Z.AI Pro: $0 (subscription, ends June/July 2026)
- Gemini API: $0 (free tier, 20 req/day per key, 3 keys)
- Groq: $0 (free tier)
- NVIDIA NIM: $0 (free tier)
- OpenRouter: $0 (free models only, hard rule, never paid)
- Web destinations: $0 (browser automation via courier agents)
- Total monthly cost target: $0

## What We Do NOT Have (Do Not Recommend These)
- No paid API keys (ever)
- No local model inference (hardware cannot support it)
- No Docker/Kubernetes (too heavy for X220)
- No Supabase dependency (fully migrated to local PostgreSQL)
- No visual pipeline builder (aspirational, not built)
- No drag-and-drop agent orchestrator (aspirational, not built)

## Strategic Direction
- Automate everything that can be automated
- Free-tier-first, always
- Self-healing and self-monitoring
- Human in the loop for decisions, not execution
- Every finding must justify itself against what we already have
