# PRD: VibePilot Knowledge Graph (Planned)

> **Status:** Planned — not yet in development. This PRD captures requirements and architecture decisions from consultant-user iteration. Ready for planning when prioritized.

## Overview

A persistent, agent-queryable knowledge base for VibePilot that stores all research findings, architecture decisions, and system mappings. Any agent (Hermes, Claude Code, future replacements) queries it to understand what VibePilot is, what every component does, why decisions were made, what alternatives were rejected, and what research supports the current state.

This is VibePilot's institutional memory. When an agent gets swapped, zero context is lost.

## Problem

- Every new agent session starts blind — no memory of past research, decisions, or rationale
- Research (free APIs, models, platforms) gets done once, forgotten, rediscovered weeks later
- Architecture decisions (why PocketBase, why Go, why not SQLite) live only in scattered chat transcripts
- When swapping agents, all learned context vanishes
- User discovers useful tools via YouTube/feed algorithms but they sit in bookmarks collecting dust until manually surfaced
- No single place to see what every component does, why it exists, what alternatives exist

## Target Users

1. **Agents** (primary) — query via REST API to get context before making decisions
2. **Human** (secondary) — visual graph dashboard to explore, review, compare options
3. **Research Agent** (writer) — daily research findings ingested here
4. **Council** (reviewer) — reviews all research before human sees it

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
- **Version entries** (dependency version tracked with date, changelog highlights)

### Knowledge Edges

Relationships between nodes:
- `USES` — governor USES groq-api
- `REJECTED_FOR` — SQLite REJECTED_FOR PocketBase (with reason)
- `ALTERNATIVE_TO` — OpenRouter ALTERNATIVE_TO direct API access
- `RESEARCHED_IN` — finding RESEARCHED_IN session/date
- `DEPENDS_ON` — dashboard DEPENDS_ON Supabase realtime
- `SUPERSEDED_BY` — old model SUPERSEDED_BY new model

### Each Node Stores

- Title, type, status (active/considered/rejected/archived)
- Description (what it is, plain language)
- Context for VibePilot (why it matters here specifically)
- Pros and cons (within VibePilot's needs — free, swappable, no lock-in)
- Related links (docs, URLs, config file paths)
- Decision history (when decided, who decided, what alternatives were considered)
- Version info (current version, last checked date, changelog link)
- Last verified date (stale detection)

### Relationship to .context/

The knowledge graph sits **alongside** `.context/`, not replacing it:
- `.context/` = code structure ("what exists in the codebase right now")
- Knowledge graph = decisions, research, rationale ("why it exists, what else we considered")

Both are agent-queryable. Together they give full context.

---

## Research Agent

### Two Modes

1. **Scheduled daily sweep** — automated scan of all discovery sources
2. **On-demand deep dive** — user triggers from dashboard/chat, full analysis of a specific topic

Both modes produce knowledge graph nodes that go through council review before human sees them.

### Discovery Sources

- **RSS feeds** — YouTube channels, GitHub repos/tags, newsletters, blogs (custom OPML or feed list)
- **VibePilot bookmarklet** — JavaScript bookmark in any browser, one-click save URL+title+tags to knowledge graph API. Works from any laptop, any browser. Replaces raindrop.io (OAuth was painful, several failed setup attempts).
- **Tech stack version monitoring** — PocketBase, Go, GLM, Supabase, Cloudflare, Vercel, every dependency
- **New LLM models** — HuggingFace, provider announcements, benchmarks
- **Free web AI platforms** — new ones discovered, existing ones status-checked (still free? still working?)
- **YouTube videos** — agent transcribes, summarizes, evaluates VibePilot relevance

### Research Agent Daily Cycle

1. Scan all RSS feeds and bookmarklet queue for new items
2. For each finding: summarize, rate relevance for VibePilot specifically
   - **Highly useful** — directly fills a gap or replaces something suboptimal
   - **Moderately useful** — relevant, worth considering when we get to that area
   - **Relevant but not now** — good to know, park it
   - **Not useful but watch** — not relevant today but evolving space
3. Create knowledge graph nodes with status `pending_review`
4. Batch all new nodes with full graph context → send to council

### On-Demand Deep Dive

User says "look into Puter.js" (from dashboard or chat):
1. Research agent reads repo, docs, transcribes intro video if exists
2. Produces full analysis in knowledge graph with rating and recommendation
3. Council reviews, adds context ("we looked at similar tool X last month and rejected because...")
4. User gets same review dock experience as daily sweep

---

## Council Review Pipeline (V1)

All research goes through council before human sees it. Unfiltered data doesn't hit the review dock.

### Council's Job

- Cross-reference new findings against existing knowledge graph
- "We rejected X before because Y, but now Z changed — reconsider?"
- "This new thing replaces nothing but fills gap we identified in [link→node]"
- "This is a better alternative to [existing node], here's the comparison"
- Produce recommendation per item with full context

### Review UX (Dashboard)

One daily review dock:
- All new items, rated, with council recommendations
- Each item expandable into full context view:
  - The finding itself (summary, source, rating)
  - Council recommendation and reasoning
  - Related past decisions and alternatives
  - Affected components
- Actions: **Approve** / **Reject** / **Request more research** / **Watch**
- Approved items update knowledge graph, rejected items marked with reason for future reference

---

## Core Features

### 1. PocketBase Backend

- PocketBase instance on X220 alongside governor
- Collections: `nodes`, `edges`, `research_entries`, `bookmarks`
- REST API for agent queries (any agent, any language, zero dependencies)
- Realtime subscriptions for dashboard live updates
- Go-native — same language as governor, can embed or run alongside
- Simple API key auth for agents

### 2. Agent Query Interface

Any agent can:
- Search nodes: `GET /api/collections/nodes/records?q=storage&status=active`
- Full node detail: `GET /api/collections/nodes/records/{id}`
- Related nodes: `GET /api/collections/edges/records?from={id}`
- Add research: `POST /api/collections/research_entries/records`
- Natural language queries via search endpoint

### 3. VibePilot Bookmarklet

Replaces raindrop.io. No OAuth. One click from any browser on any device:

```javascript
javascript:void(fetch('https://vibes-server/api/bookmarks',
  {method:'POST',headers:{'Content-Type':'application/json'},
  body:JSON.stringify({url:location.href,title:document.title,
  source:'bookmarklet'})}).then(r=>r.json()).then(d=>
  alert(d.status==='ok'?'Saved to VibePilot!':'Error: '+d.msg)))
```

Drag to bookmarks bar. Works from YouTube laptop, dev laptop, anywhere. API key in header or bookmarklet.

Research agent picks up bookmarked items during next sweep cycle.

### 4. Visual Graph Dashboard (PRIMARY INTERFACE)

The graph view is not supplementary — it is THE way to interact with the knowledge graph. Integrated into existing VibePilot dashboard.

**Graph View:**
- Interactive node graph (Vis.js or similar)
- Nodes colored by type (models=blue, connectors=green, decisions=orange, research=purple, etc.)
- Node border = status (solid=active, dashed=considered, strikethrough=rejected)
- Click node → side panel: full detail, docs, pros/cons, decision history
- Click edge → relationship details
- Review flags on nodes with pending council recommendations

**Review Dock:**
- Click "Review" on flagged node
- See: new finding + council recommendation + related past decisions/alternatives + affected components
- Approve / reject / request more research
- Approved → node updates, graph refreshes via realtime subscription

**Comparison Mode:**
- Select 2-3 nodes → side-by-side with pros/cons in VibePilot context

**Version Tracking:**
- Dependency versions as node properties
- Researcher checks for updates
- Version history with dates and changelog highlights

**Search & Filter:**
- Filter by keyword, type, status, date range
- Search across all node content

### 5. Git Backup & Restore

- PocketBase data exported to JSON on schedule
- Committed to VibePilot repo
- JSON diffs visible in GitHub commits
- Full restore from git at any time
- Backup triggers: on change, or scheduled interval
- `.gitignore` excludes PocketBase runtime files, only backs up data exports

---

## Data Schema

### `nodes` collection
```
id, title, type (component|model|connector|platform|decision|research|technology|version),
status (active|considered|rejected|archived|pending_review),
description, vibepilot_context,
pros (JSON array), cons (JSON array),
alternatives (JSON array of node IDs),
config_path (if relevant, e.g., governor/config/models.json),
external_urls (JSON array),
decision_history (JSON: [{date, agent, decision, reason, alternatives}]),
version_info (JSON: {current, last_checked, changelog_url}),
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
id, title, source (url or session ref), source_type (rss|bookmarklet|deep_dive|manual),
summary, relevance_rating (highly_useful|moderately_useful|relevant_not_now|watch),
findings (JSON), linked_nodes (JSON array of node IDs),
council_recommendation (JSON: {verdict, reasoning, related_nodes}),
status (pending_review|approved|rejected|archived|more_research_needed),
discovered_date, reviewed_date, reviewed_by
```

### `bookmarks` collection
```
id, url, title, tags (JSON array), source (bookmarklet|manual|rss),
status (new|processing|processed|skipped),
notes, created
```

---

## Architecture

```
Discovery Sources:
  RSS feeds ──┐
  Bookmarklet ─┤
  Manual entry─┤
               ▼
        Research Agent
               │
               ▼
     Knowledge Graph (PocketBase)
         nodes + edges
               │
               ▼
        Council Review
               │
               ▼
     Human Review Dock (Dashboard)
               │
       Approve / Reject
               │
               ▼
     Knowledge Graph Updated
               │
        ┌──────┴──────┐
        ▼              ▼
   Agent Queries    Git Backup
   (REST API)      (JSON export)
```

PocketBase runs as systemd user service on X220, same pattern as governor.

---

## Tech Stack

- **Storage:** PocketBase (Go binary, REST API, realtime, built-in SQLite)
- **Visualization:** Vis.js or similar network graph in existing React dashboard
- **Backup:** Git export to VibePilot repo (JSON diffs)
- **Agent access:** REST API (curl/fetch from any agent)
- **Bookmarklet:** Vanilla JS, no dependencies
- **Hosting:** X220 (same machine as governor)

---

## Constraints

- Must be queryable by any agent via simple HTTP (no SDK dependency)
- Must survive agent swaps (data persists independent of who's reading it)
- Must backup to git (restore-from-scratch capability)
- Must integrate into existing dashboard (not a separate UI)
- All data must be VibePilot-contextual (not generic tech docs — WHY it matters HERE)
- PocketBase instance stays lightweight (X220 has limited RAM)
- Council review is mandatory — no unfiltered research reaches human
- Bookmarklet must work from any browser on any device without OAuth

---

## Success Criteria

- New agent session queries knowledge graph, gets full project context in one API call
- Human can visually explore knowledge graph and understand any decision
- Daily research sweep discovers, rates, and queues items for review automatically
- Council adds meaningful context (past rejections, alternatives, affected components)
- Human review dock shows everything new in one place with approve/reject actions
- Git backup exists and has been tested for restore
- Every "why did we choose X?" question is answerable from the graph
- Bookmarklet works from any device with one click
- Dependency version tracking shows what's current and what's stale

---

## Out of Scope (V1)

- Auto-sync between PocketBase and governor config (separate feature)
- Public access (internal-only)
- Multi-project support (VibePilot only)
- Automated conflict resolution (human reviews)
- Mobile app (dashboard responsive is enough)
- Chrome extension (bookmarklet is V1, extension is V2)
- Email forwarding as input source (V2)
- Telegram bot as input source (V2)

---

## Dependencies

- PocketBase binary (single Go binary, ~20MB)
- Vis.js npm package (or similar graph library)
- Git repo write access (for backup)
- Existing VibePilot dashboard (for integration)
- RSS feed URLs (YouTube channels, GitHub repos, blogs)
- Research agent (prompts/daily_landscape_researcher.md exists as starting point)

---

## Open Questions

- Which graph visualization library? Vis.js vs D3 vs Cytoscape — test in dashboard first
- Backup schedule: on-change vs every N minutes vs daily
- PocketBase: standalone binary or embedded in governor?
- RSS feed format: OPML file in config or separate feeds.json?
- Should bookmarklet API key be per-user or shared system key?

---

*PRD produced by consultant agent (Hermes) through iterative conversation with user. Decisions captured: PocketBase over SQLite, bookmarklet over raindrop.io, council review mandatory in V1, dashboard graph is primary interface, coexists with .context/, two researcher modes (scheduled + on-demand).*
