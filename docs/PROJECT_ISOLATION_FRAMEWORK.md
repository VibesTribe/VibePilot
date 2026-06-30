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
  memories/               ← Agent memories, namespaced (planner/, architect/, etc.)
  knowledgebase/          ← Stable project knowledge (API docs, architecture, business rules)
  research/               ← Temporary findings (benchmarks, comparisons) — graduates to KB
  config/                 ← Model keys, deploy targets, API endpoints, secrets refs
  backups/                ← Automated snapshots (git-backed) — NOT inside main backup cycle
  logs/                   ← Operational data: execution logs, audit trail, cache
  export.sh               ← Package project for transfer (with secret scrub + signing)
  restore.sh              ← Unpack and restore on a clean machine (with signature verify)
  README.md               ← Project overview, setup instructions
```

### 1.1 vibepilot.toml (The Manifest)

This is the contract between VibePilot and the project. It declares everything
VibePilot needs to know without hardcoding anything:

```toml
[manifest]
version = 1                    # Schema version — bump when manifest format changes
framework_min = "1.0"          # Minimum VibePilot version that understands this manifest
framework_max = "2.x"          # Maximum compatible version

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

[execution]
# Execution profile — how the agent should behave on this project
cost_limit_usd = 5.00       # Daily spend cap — agent halted if exceeded
retry_policy = "conservative"  # "conservative" | "aggressive" | "none"
approval_required = true    # Human must approve before deploying/merging

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

[network]
# Egress allowlist — agent can ONLY contact these domains. Default: no internet.
egress_allow = []           # e.g. ["api.stripe.com:443", "github.com:443", "api.openai.com:443"]

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

5. **Every project must have export.sh AND restore.sh.** No exceptions. export.sh
   packages the project; restore.sh unpacks it on a clean machine. Both must be
   tested. Portability is bidirectional — if it can't be restored, the export is
   worthless.

6. **Every project must have a backup repo.** Automated, git-backed, verifiable
   restore.

7. **One change at a time.** Each isolation layer is built, tested, verified before the
   next one starts. No big-bang refactors.

8. **Network egress is isolated per project.** (Added per council review.) Each
   project declares which external URLs its agent may contact (e.g.,
   api.stripe.com, github.com). Default: no internet access unless explicitly
   declared in vibepilot.toml. This prevents a compromised or buggy agent from
   exfiltrating data to unexpected destinations.

9. **Secrets are scrubbed on export.** (Added per council review.) export.sh runs
   a redaction pass across the entire project directory before packaging — it
   scans for anything that looks like an API key, token, or credential and
   replaces it with a placeholder reference. This catches secrets the agent may
   have accidentally written into files despite the vault system.

10. **Audit entries record WHO acted.** (Added per council review.) Every log
    entry includes the actor: which agent runtime (Hermes vs Claude Code), which
    session, or whether it was a human action. Without this, the audit trail
    can't answer the questions audits exist for.

11. **Export archives are signed.** (Added per council review.) export.sh produces
    a checksum/signature for the archive. restore.sh verifies it before
    extracting anything. This catches tampering or corruption during transfer.

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

---

## 10. OPEN DESIGN DECISIONS (From Council Review — Need Your Decision)

These are the three areas where the four AI models (Gemini, ChatGPT, Claude,
DeepSeek) disagreed. Each needs your call before we finalize.

### Decision 1: Should Projects Talk to Each Other?

**The question:** Do projects need to communicate at all? For example, should Sealed
be able to ask VibePilot for model key status, or trigger a research scan?

**Gemini says YES — event bus:** Projects publish events to a shared message bus.
Project A doesn't know about Project B, but they can react to each other's events.
Like how different apps on your phone can share notifications.

**Claude says NO — contradicts isolation:** Any communication channel between projects
is a deliberate hole in the isolation wall. If Sealed is sold and transferred, you
don't want it secretly depending on VibePilot's event bus. Communication should be
default-deny, orchestrator-mediated only, and treated as an explicit exception.

**DeepSeek says YES but carefully:** A "command queue" where projects send signed
messages. VibePilot acts as a mailman — it delivers the message but doesn't let
projects see each other's addresses. Default deny unless vibepilot.toml explicitly
allows it.

| Option | Pros | Cons |
|--------|------|------|
| No communication (Claude) | Maximum isolation, simplest to reason about, easiest transfer/sale | Projects can't share resources even when it would be useful |
| Orchestrator-mediated (DeepSeek) | Controlled, logged, default-deny. VibePilot decides what passes | Adds complexity to VibePilot. Projects still depend on orchestrator for comms |
| Open event bus (Gemini) | Flexible, projects can react to each other | Direct coupling, harder to transfer/sell, isolation holes |

**My recommendation:** Claude's approach — default deny, orchestrator-mediated only.
Projects should NOT talk to each other directly. If Sealed needs something, it asks
VibePilot (the orchestrator), VibePilot decides whether to fulfill the request, and
logs it. This keeps the isolation wall solid and makes projects truly standalone.
We can always open this up later if needed — but starting permissive and tightening
is much harder than starting tight and loosening.

---

### Decision 2: Should We Deduplicate Shared Files Across Projects?

**The question:** If VibePilot and Sealed both use the same 500MB knowledge file
(e.g., a shared reference library), should we store one copy and point both
projects at it, or store separate copies?

**Gemini says YES — content-addressable storage:** Hash the file content, store one
copy, both projects reference it by hash. Saves disk space.

**Claude says NO — side channel risk:** If Project A can detect that Project B has
the same file (by checking if a hash already exists in the shared store), that's an
information leak. A competitor or buyer could deduce what knowledge your other
projects use.

**DeepSeek says YES but encrypt first:** Encrypt each project's files with a
project-specific key before hashing. Same plaintext produces different hashes, so
the side channel disappears. You still get dedup but only for files that are
identical AND encrypted with the same key (i.e., within the same project).

| Option | Pros | Cons |
|--------|------|------|
| No dedup (Claude) | Maximum isolation, zero information leak between projects | Disk usage grows linearly with number of projects |
| Per-project dedup only (DeepSeek) | Dedup works within a project (backups, versions). No cross-project leak | Doesn't help if 5 projects all have the same 500MB file |
| Cross-project CAS (Gemini) | Maximum space savings | Side channel: projects can detect each other's files |

**My recommendation:** Claude's approach — no cross-project dedup. Per-project dedup
is fine (for backups and version history within a project). But across projects,
separate copies. Disk space is cheap. Isolation is not. A 500MB file duplicated
across 5 projects costs 2.5GB — trivial. The side channel risk is not worth saving
$0.10 in storage.

---

### Decision 3: How Should the Audit Log Handle Problems?

**The question:** When something goes wrong (agent crashes, writes bad data, project
state gets corrupted), how should the audit log help recover?

**Gemini says "self-healing":** The system replays the audit log, detects where things
went wrong, and automatically rolls back to a known good state.

**Claude says "self-healing" is dangerous:** Audit logs must be append-only and
immutable. "Self-healing" implies the system rewrites or edits log entries — which
destroys the tamper-evidence that makes the log valuable. If the log can be changed,
you can't trust it.

**DeepSeek says Merkle DAG reconciliation:** The log is append-only (never edited).
"Self-healing" means the system replays the log, compares the current state hash
against the expected hash, and if they don't match, it ADDS a new entry (a
"compensation event") that says "revert transaction 123." The log is never edited —
corrections are appended.

| Option | Pros | Cons |
|--------|------|------|
| Append-only, no auto-recovery (Claude) | Maximum trust in log integrity | Recovery from corruption is manual |
| Merkle DAG with compensation events (DeepSeek) | Log stays immutable. Recovery is automatic but transparent — every correction is itself logged | More complex to implement. Compensation events can chain |
| Self-healing with edits (Gemini) | Simplest automatic recovery | Log can't be trusted as tamper-evidence. Edits destroy audit value |

**My recommendation:** DeepSeek's Merkle DAG approach. The log is never edited — it's
append-only and cryptographically chained (each entry references the hash of the
previous one, like a mini blockchain). When something goes wrong, the system detects
the mismatch and appends a correction entry — it never rewrites history. This gives
us both tamper-evidence (you can verify the log hasn't been altered) AND automatic
recovery (the system can detect and correct problems by appending, not editing).

---

## END OF DOCUMENT

---

## RESOLVED DECISIONS (Approved by User — June 30, 2026)

All three open design decisions have been reviewed and approved.

**Decision 1: Inter-Project Communication → APPROVED: Default deny, orchestrator-mediated only.**
No direct communication between projects. If a project needs something, it asks
VibePilot. Projects are isolated neighbors — they don't share doorbells.
A problem with Sealed is Sealed's problem. A problem with VibePilot is everyone's
problem, but projects shouldn't be messing with VibePilot.

**Decision 2: File Deduplication → APPROVED: No cross-project dedup.**
Separate copies per project. Disk space is cheap. Isolation is not.
Per-project dedup (for backups/versions within a project) is fine.

**Decision 3: Audit Log → APPROVED: Append-only Merkle DAG with compensation events.**
The log is never edited. Corrections are appended. History is immutable.
Recovery is automatic but transparent — every correction is itself logged.

These decisions are now binding. Do not re-open without explicit user direction.
