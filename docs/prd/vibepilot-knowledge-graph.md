# PRD: VibePilot Knowledge Graph

## Overview
A persistent, agent-queryable knowledge base for VibePilot that stores all research findings, architecture decisions, and system mappings. Any agentic system (Hermes, Claude Code, future replacements) can query it to instantly understand what VibePilot is, what every component does, why decisions were made, what alternatives were considered and rejected, and what research supports the current state.

This is VibePilot's institutional memory. When an agent gets swapped, zero context is lost.

## Problem
- Every new agent session starts blind -- no memory of past research, decisions, or rationale
- Research (free APIs, models, platforms) gets done once, forgotten, then rediscovered weeks later
- Architecture decisions (why PocketBase, why Groq, why not SQLite) live only in scattered Gemini chats and session transcripts
- When swapping agents (Hermes → something else), all learned context vanishes
- No single place to see what every component does, why it exists, and what alternatives exist

## Target Users
1. **Agents** (primary) -- query via REST API to get context before making decisions
2. **Human** (secondary) -- visual graph in dashboard to explore, review, compare options
3. **System Researcher** (writer) -- daily research findings get ingested here

## Core Concepts

### Knowledge Nodes
Every notable thing in VibePilot's universe is a node:
- **Components** (governor, planner, courier, dashboard, etc.)
- **Models** (glm-5, llama-4-scout, gemini-2.5-flash, etc.)
- **Connectors** (groq-api, zai-api, openrouter-api, etc.)
- **Platforms** (chatgpt-web, claude-web, kimi-web, etc.)
- **Decisions** (why PocketBase over SQLite, why Go for governor, etc.)
- **Research findings** (free API discovered, new model benchmarked, etc.)
- **Technologies** (Supabase, PocketBase, Cloudflare, Vercel, etc.)

### Knowledge Edges
Relationships between nodes:
- `USES` -- governor USES groq-api
- `REJECTED_FOR` -- SQLite REJECTED_FOR PocketBase (with reason)
- `ALTERNATIVE_TO` -- OpenRouter ALTERNATIVE_TO direct API access
- `RESEARCHED_IN` -- finding RESEARCHED_IN session/date
- `DEPENDS_ON` -- dashboard DEPENDS_ON Supabase realtime
- `SUPERSEDED_BY` -- old model SUPERSEDED_BY new model

### Each Node Stores
- Title, type, status (active/considered/rejected/archived)
- Description (what it is, plain language)
- Context for VibePilot (why it matters here specifically)
- Pros and cons (within VibePilot's needs -- free, swappable, no lock-in)
- Related links (docs, URLs, config file paths)
- Decision history (when decided, who decided, what alternatives were considered)
- Last verified date (stale detection)

## Core Features

### 1. PocketBase Backend
- PocketBase instance running on X220 alongside governor
- Collections: `nodes`, `edges`, `research_entries`
- REST API for agent queries (any agent, any language, zero dependencies)
- Realtime subscriptions for dashboard live updates
- Go-native -- same language as governor, can embed or run alongside
- Auth tokens for agent access (simple key-based)

### 2. Agent Query Interface
Any agent can:
- `GET /api/collections/nodes/records?q=storage&status=active` -- search nodes
- `GET /api/collections/nodes/records/{id}` -- full node detail
- `GET /api/collections/edges/records?from={id}` -- what's connected to this
- `POST /api/collections/research_entries/records` -- add research finding
- Natural language queries via a simple search endpoint

### 3. System Researcher Integration
When the daily system researcher finds something:
1. Finding gets written as a research node
2. Linked to existing nodes it relates to (new model → linked to relevant connectors)
3. Status = "new" until human reviews via dashboard
4. Human approves/rejects/modifies → status updates → graph updates

### 4. Visual Graph Dashboard
Integrated into existing VibePilot dashboard:
- **Graph view**: Interactive node graph (Vis.js or D3)
  - Nodes colored by type (models=blue, connectors=green, decisions=orange, etc.)
  - Node border = status (solid=active, dashed=considered, strikethrough=rejected)
  - Click node → side panel shows full detail with docs, pros/cons, decision history
  - Click edge → shows relationship details
- **Comparison mode**: Select 2-3 nodes → side-by-side with pros/cons in VibePilot context
- **Search**: Filter graph by keyword, type, status
- **Timeline**: When was this added? What was happening then?

### 5. Git Backup & Restore
- PocketBase data exported to JSONL/SQLite dump on schedule
- Committed to VibePilot repo (or separate knowledge repo)
- Full restore from git at any time
- Backup triggers: on change, or every N minutes, or on researcher write
- `.gitignore` excludes runtime files, only backs up data

## Data Schema

### `nodes` collection
```
id, title, type (component|model|connector|platform|decision|research|technology),
status (active|considered|rejected|archived|new),
description, vibepilot_context,
pros (JSON array), cons (JSON array),
alternatives (JSON array of node IDs),
config_path (if relevant, e.g., governor/config/models.json),
external_urls (JSON array),
decision_history (JSON: [{date, agent, decision, reason, alternatives}]),
last_verified (date), created, updated
```

### `edges` collection
```
id, from_node (relation), to_node (relation),
relationship (uses|rejected_for|alternative_to|depends_on|researched_in|supersedes|related_to),
reason, created
```

### `research_entries` collection
```
id, title, source (url or session ref), summary,
relevance_score (1-5 for VibePilot), findings (JSON),
linked_nodes (JSON array of node IDs),
status (new|approved|rejected|archived),
discovered_date, reviewed_date, reviewed_by
```

## Architecture

```
System Researcher → writes findings → PocketBase
Agents (any) → query REST API → PocketBase
Dashboard → realtime subs + REST → PocketBase → Vis.js graph
Cron job → export PocketBase data → git commit → VibePilot repo backup
```

PocketBase runs as a systemd user service on X220, same pattern as governor.

## Tech Stack
- **Storage**: PocketBase (Go binary, REST API, realtime, built-in SQLite under the hood)
- **Visualization**: Vis.js network graph in existing React dashboard
- **Backup**: Git export to VibePilot repo
- **Agent access**: REST API (curl/fetch from any agent)
- **Hosting**: X220 (same machine as governor)

## Constraints
- Must be queryable by any agent via simple HTTP (no SDK dependency)
- Must survive agent swaps (data persists independent of who's reading it)
- Must backup to git (restore-from-scratch capability)
- Must integrate into existing dashboard (not a separate UI)
- All data must be VibePilot-contextual (not generic tech docs -- WHY it matters HERE)
- PocketBase instance stays lightweight (X220 has limited RAM)

## Success Criteria
- New agent session starts, queries wiki, gets full project context in one API call
- Human can visually explore the knowledge graph and understand any decision
- System researcher findings auto-ingest and appear in graph
- Git backup exists and has been tested for restore
- Every "why did we choose X?" question is answerable from the wiki

## Out of Scope (V1)
- Auto-sync between PocketBase and governor config (that's a separate feature)
- Public access (this is internal-only)
- Multi-project support (VibePilot only for now)
- Automated conflict resolution (human reviews)
- Mobile app (dashboard is responsive, that's enough)

## Dependencies
- PocketBase binary (single Go binary, ~20MB)
- Vis.js npm package (for dashboard graph component)
- Git repo write access (for backup)
- Existing VibePilot dashboard (for integration)
