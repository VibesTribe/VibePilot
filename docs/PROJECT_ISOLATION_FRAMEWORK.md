# PROJECT ISOLATION FRAMEWORK (PIF)
**Version:** 1.0
**Status:** Design — Pending Review
**Date:** June 30, 2026
**Principles:** Modular, Configurable, Agnostic, Swappable, Restorable, Transferable, Zero Drift

---

## 0. WHY THIS EXISTS

VibePilot builds products. Each product is a separate company, a separate codebase,
a separate database, a separate agent identity. Products must be portable — they can be
sold, transferred, scaled independently, or handed to another team.

This document defines the standard structure for every project VibePilot creates. It is
binding: no project begins development until its PIF scaffold exists and is verified.

The first project to use this framework will be **Sealed**. Every subsequent project
inherits the same structure automatically.

---

## 1. THE SELF-CONTAINED PROJECT UNIT

Every project is a self-contained directory on disk with zero external dependencies
(except infrastructure like electricity and network). The structure:

```
~/projects/{slug}/
  vibepilot.toml          ← Project manifest (how VibePilot interacts with this project)
  repo/                   ← The application codebase (own git remote)
  database/               ← Database files, migrations, seed data, connection config
  skills/                 ← Agent skills specific to this project
  memories/               ← Agent memories specific to this project
  knowledgebase/          ← Project-specific KB (docs, research, templates)
  research/               ← Research scan configs, reports, council decisions
  config/                 ← Model keys, deploy targets, API endpoints, secrets refs
  backups/                ← Automated git-backed snapshots
  export.sh               ← One-command packaging for transfer/exit
  README.md               ← Project overview, setup instructions
```

### 1.1 vibepilot.toml (The Manifest)

This is the contract between VibePilot and the project. It declares everything
VibePilot needs to know without hardcoding anything:

```toml
[project]
slug = "sealed"
display_name = "Sealed"
description = "AI-native e-signature and contract agreement platform"
status = "active"

[repo]
remote = "git@github.com:VibesTribe/sealed.git"
local_path = "repo/"
default_branch = "main"
protected_branches = ["main"]

[agent]
# Which agent system to use. Swappable. VibePilot reads this and dispatches accordingly.
runtime = "hermes"          # "hermes" | "claude-code" | "opencode" | "kilo" | "custom"
profile = "sealed"          # Hermes profile name, or CLI config name
working_dir = "repo/"

[database]
# The project's own database. VibePilot's PostgreSQL is NOT used.
type = "sqlite"             # "sqlite" | "postgres" | "supabase" | "none"
edge_path = "database/sealed.db"
cloud_provider = "supabase" # "supabase" | "neon" | "none"
cloud_url_ref = "SEALED_SUPABASE_URL"  # env var reference, never the actual URL

[deploy]
target = "cloudflare"       # "cloudflare" | "vercel" | "docker" | "none"
frontend_url = "https://sealed.icu"
backend_url = "https://api.sealed.icu"
edge_node = "x220"          # Which machine runs the edge component

[model_keys]
# Which API keys this project may use. Empty = inherit from VibePilot.
# Populated = project has its own keys.
keys = []                   # e.g. ["SEALED_OPENAI_KEY", "SEALED_STRIPE_KEY"]

[isolation]
database_separate = true    # Project has its own DB, not sharing VibePilot's
kb_separate = true          # Project has its own KB server root
research_separate = true    # Project has its own research pipeline
skills_separate = true      # Project has its own agent skills
memories_separate = true    # Project has its own agent memories

[backup]
enabled = true
destination = "github"      # "github" | "s3" | "local"
repo = "VibesTribe/sealed-backup"  # Private backup repo
schedule = "0 3 * * *"     # Daily at 3 AM
```

### 1.2 Key Properties

**Zero External Coupling:** The project directory contains everything needed to run,
build, and maintain the application. No symlinks to ~/vibepilot, no shared config files,
no implicit dependencies on VibePilot's database or skills.

**Agent Agnostic:** The `[agent]` section declares which runtime to use. VibePilot reads
this and dispatches tasks accordingly. Switching from Hermes to Claude Code is a config
change, not a code change.

**Database Independent:** Each project declares its own database. VibePilot's PostgreSQL
stores project metadata (name, slug, deploy URL) in the `projects` table for the hexagon
overview, but the project's actual data lives in its own database.

**Exportable:** `export.sh` packages the entire directory into a tar.gz. The archive
contains everything needed to deploy the project on a completely new machine without
VibePilot. Transfer, sell, hand off — one command.

---

## 2. ISOLATION LAYERS (DETAILED)

### Layer 1: FILESYSTEM ISOLATION

Each project lives in `~/projects/{slug}/`. VibePilot's code never reads from or writes
to another project's directory. Agent working directories are set to the project's
`repo/` path. Git operations happen in that repo only.

**Enforcement:** VibePilot sets the agent's cwd to `~/projects/{slug}/repo/`. The agent
literally cannot see other projects' files unless it deliberately navigates outside
(which violates the project context).

### Layer 2: DATABASE ISOLATION

Each project has its own database, declared in vibepilot.toml. Options:

- **SQLite (edge):** `~/projects/{slug}/database/{slug}.db` — local file, zero setup,
  perfect for edge deployment, replicatable via Litestream.
- **Supabase (cloud):** Project's own Supabase project — managed PostgreSQL with
  pgvector, real-time subscriptions, auth. Free tier sufficient for initial launch.
- **PostgreSQL (self-hosted):** Separate database (not schema) in VibePilot's Postgres
  instance. Named `sealed_db` not `vibepilot`. Uses separate credentials.

**VibePilot's role:** The `projects` table in VibePilot's DB stores only metadata
(slug, display_name, deploy_url, connected_services) for the hexagon overview. It does
NOT store project application data.

**The dashboard boundary:** When you view Sealed on the hexagon dashboard, you see
Sealed's tasks (project-scoped in VibePilot's task queue) and project metadata. You do
NOT see VibePilot's costs, counters, or research reports. Each project has its own
ROI panel, its own counters.

### Layer 3: SKILL ISOLATION

Each project has its own skills directory: `~/projects/{slug}/skills/`. These are loaded
when VibePilot dispatches a task for that project. The agent sees ONLY that project's
skills plus any explicitly shared VibePilot skills (like git operations, code review
patterns).

Examples:
- Sealed skills: `contract-law-basics`, `opensign-integration`, `stripe-payments`,
  `merkle-audit-trail`
- VibePilot skills: `system-admin`, `model-health-check`, `postgres-tuning`

VibePilot skills are available as a shared library. Project skills override or extend.
No VibePilot skill is loaded unless explicitly requested.

### Layer 4: MEMORY ISOLATION

Each project gets its own Hermes profile with its own memories:
`~/.hermes/profiles/{slug}/memories/`. When working on Sealed, the agent's memory
contains Sealed context (contract patterns, OpenSign quirks, Stripe webhook configs) —
NOT VibePilot's memories (X220 tablet fixes, dashboard CSS bugs).

VibePilot memories stay in the default profile. They never leak into project profiles.

### Layer 5: KNOWLEDGEBASE ISOLATION

Each project has its own KB root: `~/projects/{slug}/knowledgebase/`. The KB server
serves content from the project's root when the project is active.

VibePilot's KB (architecture docs, system maps) stays separate. A project's KB contains
its own domain knowledge (Sealed: contract clause library, legal research, integration
guides).

### Layer 6: RESEARCH ISOLATION

Each project can have its own research pipeline with:
- Different scan sources (Sealed scans legal tech, not AI models)
- Different council lenses (Sealed: legal, security, UX — not architecture, feasibility)
- Different schedule
- Different LLM model preferences

Research reports go into `~/projects/{slug}/research/` and into the project's own
research_reports table (in the project's DB, not VibePilot's).

### Layer 7: DEPLOYMENT ISOLATION

Each project deploys independently. Sealed → Cloudflare Pages/Workers. VibePilot →
Vercel. Different pipelines, different domains, different credentials. A deploy of one
never affects the other.

---

## 3. THE PROJECT LIFECYCLE

### 3.1 Creation (Scaffolding)

Creating a new project is a single action — the hexagon "New Project" button or a
VibePilot command. The scaffold process:

1. **Validate:** Check slug uniqueness, name format.
2. **Create directory:** `~/projects/{slug}/` with all subdirectories.
3. **Generate vibepilot.toml:** From a template, filled with project details.
4. **Initialize git repo:** Create the GitHub repo, clone to `repo/`.
5. **Create Hermes profile:** `~/.hermes/profiles/{slug}/` with empty skills, memories.
6. **Initialize database:** Create SQLite file or configure Supabase connection.
7. **Generate export.sh:** From template.
8. **Register in VibePilot:** Insert row into projects table (metadata only).
9. **Create backup repo:** Private GitHub repo for automated backups.

### 3.2 Development

During development, VibePilot:
- Dispatches tasks to the project's agent (using declared runtime + profile)
- Routes model requests using the project's declared model keys
- Runs the project's git operations in the project's repo
- Provides the hexagon dashboard for monitoring (project-scoped view)
- Optionally runs the project's research pipeline

The project's agent has full context for the project and zero context for VibePilot's
internals (unless explicitly shared).

### 3.3 Deployment

Each project declares its deploy target in vibepilot.toml. VibePilot triggers deploys
but doesn't own the deploy pipeline. The project's `repo/` contains everything needed
for deployment (Dockerfile, wrangler.toml, vercel.json, etc.).

### 3.4 Backup & Restore

Automated backups git-commit the entire project directory (excluding large binaries and
secrets) to a private backup repo. Restore is a git clone + config fill-in.

### 3.5 Export / Transfer / Exit

`export.sh` creates a self-contained archive:

```bash
./export.sh [--include-db] [--include-secrets]
```

- Without flags: repo code, skills, KB, research, configs (no secrets, no DB data).
- With `--include-db`: includes database dumps.
- With `--include-secrets`: includes decrypted secrets (for full transfer).

The archive can be deployed on a completely new machine. VibePilot is not required
at runtime for the deployed project.

### 3.6 Archival

When a project is completed, sold, or abandoned:
- Final backup runs
- Status changes to "archived" in projects table
- Hexagon shows it greyed out
- Directory stays on disk (not deleted) but no new tasks dispatched
- Can be fully exported at any time

---

## 4. SHARED vs ISOLATED (THE BOUNDARY TABLE)

| Resource             | Shared (VibePilot)              | Isolated (Per Project)                     |
|----------------------|---------------------------------|--------------------------------------------|
| **Application code** | —                               | Each project has own repo                   |
| **Database**         | VibePilot metadata only         | Each project has own DB                     |
| **Skills**           | Shared library (git, code review) | Each project has own skills                |
| **Memories**         | VibePilot's profile memories    | Each project has own profile memories       |
| **Knowledgebase**    | VibePilot KB                    | Each project has own KB root                |
| **Research**         | AI model landscape scan         | Each project has own research pipeline      |
| **Model keys**       | VibePilot's key pool            | Each project declares own keys (or inherits)|
| **Deploy**           | VibePilot on Vercel             | Each project has own deploy target          |
| **Costs / ROI**      | VibePilot's ROI panel           | Each project has own costs                  |
| **Tasks**            | VibePilot's task queue (shared) | Filtered by project_id                      |
| **Agent runtime**    | Hermes (default)                | Declared per project in vibepilot.toml      |
| **Dashboard**        | Hexagon overview (shared)       | Project dashboard (scoped data)             |
| **Cloudflare Tunnel**| VibePilot manages tunnels       | Each project can have own tunnel            |
| **Backups**          | VibePilot state backup          | Each project has own backup repo            |

---

## 5. AGENT AGNOSTICISM

VibePilot does not assume the project uses Hermes. The `[agent]` section in
vibepilot.toml declares the runtime:

- `runtime = "hermes"` → VibePilot dispatches via Hermes with the project's profile
- `runtime = "claude-code"` → VibePilot dispatches via Claude Code CLI
- `runtime = "opencode"` → VibePilot dispatches via OpenCode CLI
- `runtime = "kilo"` → VibePilot dispatches via Kilo CLI
- `runtime = "custom"` → VibePilot dispatches via a custom command declared in config

This means:
- Different projects can use different agent systems simultaneously
- Switching agent systems for a project is a config change, not a migration
- The project's code, DB, and KB are agent-agnostic — they're just files and databases

VibePilot provides the dispatch glue: it reads the manifest, resolves the runtime, sets
up the working directory, and invokes the agent with the project's context.

---

## 6. IMPLEMENTATION PHASES

### Phase A: Scaffold System (BUILD FIRST)
- Create `vibepilot project create` command (or enhance the hexagon "New Project" flow)
- Generate directory structure, vibepilot.toml, export.sh
- Initialize Hermes profile per project
- Register in projects table

### Phase B: Context Switching (WHEN SCAFFOLDING WORKS)
- When VibePilot dispatches a task, load the project's skills + memories + KB
- Set agent cwd to project's repo/
- Filter dashboard data by project

### Phase C: Database Separation (WHEN CONTEXT SWITCHING WORKS)
- Each project declares its DB in vibepilot.toml
- VibePilot's dashboard queries the project's DB for project-specific data
- project_costs and system_counters get project_id columns (VibePilot's DB, for
  the dashboard's ROI panel only)

### Phase D: Research Isolation (WHEN DB SEPARATION WORKS)
- Each project can have its own research pipeline
- Different sources, different council, different schedule
- Reports go to project's own DB / directory

### Phase E: Export & Transfer (WHEN PROJECT IS STABLE)
- export.sh functional
- Test: export Sealed, deploy on a clean machine, verify it runs

### Phase F: Backup Automation (ONGOING)
- Git-backed backups to private repo
- Schedule via cron
- Verify restore works

---

## 7. SEALED AS THE FIRST TEST CASE

Sealed will be the first project created through the PIF scaffold system. This is
fitting — Sealed is a real commercial product with its own:

- **Tech stack:** Go backend, Next.js frontend, SQLite/Supabase, Cloudflare deploy
- **Legal domain:** Contract law, e-signatures, Merkle audit trails
- **Payment system:** Stripe with credit-based pricing
- **Infrastructure:** OpenSign hosted on X220 via Cloudflare Tunnel
- **Agent needs:** Contract law skills, Stripe integration, OpenSign knowledge

If the PIF works for Sealed, it works for everything that comes after.

### Sealed-Specific Isolation Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Database | SQLite (edge) + Supabase (cloud) | Offline-first, per blueprint |
| Agent | Hermes (profile: sealed) | Initially, switchable later |
| Deploy | Cloudflare Pages/Workers | Per blueprint, free tier |
| Skills | Contract law, OpenSign, Stripe | Project-specific |
| Research | Legal tech, e-sig laws, competitors | Different from VibePilot's AI model scan |
| KB | Contract templates, clause library | Legal domain knowledge |
| OpenSign | Hosted on X220, Cloudflare Tunnel | Free tier, local control |

---

## 8. GUARDRAILS (NON-NEGOTIABLE)

1. **No hardcoded project assumptions in VibePilot code.** VibePilot reads
   vibepilot.toml. It does not assume "sealed" or "vibepilot" anywhere except the
   vibepilot slug guard pattern already established.

2. **No cross-project data access.** Project A's agent cannot read Project B's database,
   skills, or memories. Enforced by directory structure and profile separation.

3. **No monolithic shared state — within OR between projects.** The only shared
   state between projects is the `projects` table (metadata) and the task queue
   (task_id → project_id mapping). Everything else is per-project. WITHIN each
   project's codebase, the same principle applies: components are modular,
   atomic, and loosely coupled. Fixing the payment flow must never break the
   signing flow. Fixing the auth system must never break the document generation.
   Vertical slices, clean interfaces, zero implicit dependencies between modules.
   The "fix one thing, break three others" pattern is a design failure, not an
   accepted reality.

4. **Secrets in encrypted vault, never plaintext.** All API keys, tokens,
   passwords live in an encrypted vault (VibePilot's existing vault system).
   Keys are injected as environment variables at runtime — the agent references
   them by name (e.g., `SEALED_STRIPE_SECRET_KEY`) but never sees or handles the
   actual value. No .env files with plaintext secrets in the project directory.
   The vault loads briefly, provides the values, and the values exist only in the
   process environment for the duration of the task. Agents cannot leak, reveal,
   or accidentally commit secrets because they never have direct access to them.
   This is safe without being confusing — the agent just uses the env var name.

5. **Every project must have export.sh.** No exceptions. If it can't be exported, it's
   not done.

6. **Every project must have a backup repo.** Automated, git-backed, verifiable restore.

7. **One change at a time.** Each isolation layer is built, tested, verified before the
   next one starts. No big-bang refactors.

---

## 9. RELATIONSHIP TO EXISTING WORK

### Already Complete (Phases 1-5 of Multi-Project Backend)
- DB migration 047: projects table with full schema
- ProjectResolver: loads project config from DB by slug
- RepoManager: per-project git operations
- Task queue scoping: tasks filtered by project_id
- Model pool per-project key filtering

### This Document Extends
- Frontend hexagon overview (Phase 6 — complete)
- Project creation flow (complete — POST /api/projects)
- Adds: isolation framework on top of the existing backend

### What Changes
- New Project form in hexagon now triggers full scaffold (not just a DB row)
- VibePilot's dashboard adds per-project data scoping (project_costs, counters)
- Hermes profiles created per project
- KB server configured for multiple roots

---

## END OF DOCUMENT

This framework is the foundation for every product VibePilot builds. It exists to
ensure that what we build is portable, maintainable, and owned — not entangled.
