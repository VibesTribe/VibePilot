# VibePilot System Context
<!-- LIVE - This file explains what each component does and why. Researcher and council agents receive this. -->
<!-- Updated automatically by generate_system_context.py. Manual edits to version numbers will be overwritten. -->

## What VibePilot Does
VibePilot is an automated software factory. It takes feature requests (PRDs), analyzes them, plans implementation, generates code via AI agents, reviews the output, and produces working code on GitHub branches for human review. The user is a solo developer building this to generate income.

## Core Components

### Governor (Go binary)
The brain. Written in Go 1.24, compiled to a single ~30MB binary. Runs as a persistent process on the X220. Handles all orchestration: receiving PRDs, routing through pipeline stages, dispatching tasks to AI models, managing state transitions, running council reviews. Uses pgx v5.9.2 for Postgres and mcp-go v0.47.1 for KB access. Chosen because Go binaries are lightweight, fast startup, no runtime dependencies.

### PostgreSQL 16.13
All state lives here. 100+ tables. Single database called "vibepilot". Uses pgvector 0.6.0 for semantic search across KB documents and code symbols. Uses pg_trgm 1.6 for fuzzy text search. Chosen because it handles everything: relational data, vector search, JSON, full-text search. No need for Redis, Elasticsearch, or separate vector DB.

### Hermes Agent v0.9.0
Multi-platform AI agent gateway. Handles Telegram, Discord, and CLI interfaces. Routes messages to AI models, manages conversation context, provides tool access (terminal, browser, file I/O, web search). Config at ~/.hermes/config.yaml. This consultant chat runs through Hermes. Chosen because it provides unified agent interface across all platforms with built-in tools and skill system.

### Next.js Dashboard (Vercel)
Monitoring UI at vibes.vibestribe.rocks. Shows tasks, ROI, model health, review queue, system status. Auto-deploys from GitHub main branch via Vercel. Built with Next.js, deploys as static/SSR. The MissionHeader component shows live system pills and review queue. Chosen for zero-cost hosting, automatic deploys, and React ecosystem.

### Courier Agents (Browser Automation)
Playwright 1.60.0 powers courier agents that interact with web AI platforms (DeepSeek, ChatGPT, Claude, etc.) via browser automation. These run locally on the X220. Each courier session opens a browser, navigates to a web AI platform, submits prompts, extracts responses. This is how VibePilot uses models that have no API or where the API is paid but the web interface is free.

### Cloudflare Tunnel (cloudflared 2026.3.0)
Exposes the governor API at api.vibestribe.rocks without opening ports. The dashboard calls this for live data (model health, task status, ROI numbers). Chosen because it's free, secure, and requires no DDNS or port forwarding.

### Knowledge Base (KB)
Postgres-backed code intelligence. Stores code symbols, doc sections, architecture decisions, non-negotiable rules, and knowledge items. Agents query it via kb_context_pack RPC to get relevant context for their task. Indexed via jcodemunch and graphify tools. Chosen so agents can understand the full codebase without reading every file.

## AI Model Access (How We Get AI Output)

### Free Web Platforms (courier/browser automation)
DeepSeek Web, ChatGPT Web, Claude Web, Gemini Web, Kimi Web, Qwen Web, Poe. Accessed via Playwright browser automation. Free tier, rate limited per platform. These are the workhorses for code generation tasks.

### Free API Access
OpenRouter (free models only, hard rule), Groq (free tier), NVIDIA NIM (free tier), Gemini API (20 req/day per key, 3 keys). Used for lighter tasks, routing decisions, quick analysis.

### Primary Agent Model
GLM-5 via Z.AI Pro subscription. Free through mid-2026. 900M context. Used for this consultant chat, Hermes agent sessions, and most agentic work.

## Pipeline Stages
PRD received -> Analysis (understand requirements) -> Planning (break into tasks) -> Code Generation (courier agents write code) -> Review (council evaluates) -> Human Review (dashboard queue) -> Merge. Each stage has event-driven state transitions in the governor.

## Research Pipeline
Researcher agent scans for improvements (new models, pricing changes, stack updates, better approaches). Council (3 members with different lenses: user alignment, architecture, feasibility) evaluates findings. Approved items go to human review queue in the dashboard.

## What We Do NOT Have
No Docker, no Kubernetes, no cloud VMs, no GPU, no paid APIs, no local model inference. Everything runs on a single X220 laptop with 16GB RAM and a spinning HDD.

## Cost
$0/month target. Everything is free tier or free subscription. The only exception is the Z.AI Pro subscription which is free through mid-2026.

## Licensing
All tools must be MIT or Apache 2.0 (open source) or have generous free tiers (SaaS). No proprietary licensed tools that could restrict commercial use.
