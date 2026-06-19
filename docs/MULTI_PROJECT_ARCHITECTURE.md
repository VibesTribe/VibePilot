# VibePilot Multi-Project Architecture

## Strategic Implementation Plan

**Created:** June 2026
**Goal:** Enable VibePilot to manage multiple isolated projects (Sealed, ExtraYum, Web of Wisdom, and beyond) from a single installation with zero cross-contamination risk.

---

## 1. CURRENT STATE ANALYSIS

### What exists (20% ready)

| Layer | Status | Detail |
|-------|--------|--------|
| Database: projects table | EXISTS | `schema_project_tracking.sql` defines projects table with id, name, status, ROI metrics |
| Database: tasks.project_id | PARTIAL | ALTER TABLE adds column, but governor never filters on it |
| Memory: project-scoped KV | WORKING | `memory_project` table + `StoreProjectState(projectID, key, value)` fully functional |
| Config: api_key_ref fields | EXISTS | ModelConfig and ConnectorConfig have `api_key_ref` field, unused for per-project allocation |
| Config: GitConfig struct | EXISTS | Has github_owner, github_repo, repo_path fields, but hardcoded to single project |

### What does not exist (the gaps)

| Gap | Impact | Difficulty |
|-----|--------|-----------|
| Git config hardcoded to VibePilot | Governor cannot push/branch/merge any other repo | Medium |
| ContextBuilder hardcoded to one repoPath | Agents see VibePilot code when working on Sealed | Medium |
| No workspace isolation | No per-project directories, agents run in wrong cwd | High |
| Governor task claiming ignores project_id | All tasks mix in one queue regardless of project | Medium |
| Model pool is global | One project's usage starves all others | Medium |
| Frontend has zero project awareness | No project switcher, no honeycomb, no per-project dashboard | High |
| No project onboarding flow | Manual setup required (violates zero-dev-skill requirement) | Medium |

### Single points of hardcoding found

```
config/system.json:
  git.github_owner = "VibesTribe"      (hardcoded)
  git.github_repo = "VibePilot"        (hardcoded)
  git.repo_env = "GITHUB_REPO"         (single env var)
  git.repo_path = (from env)           (single path)

governor/internal/runtime/config.go:
  GitConfig struct has ONE owner, ONE repo, ONE path

governor/internal/runtime/context_builder.go:
  ContextBuilder takes single repoPath in constructor
  All code map / file tree loads from that one path

governor/internal/runtime/session.go:
  buildAgentContext uses single session factory

governor/cmd/governor/handlers_task.go:
  handleTaskAvailable claims tasks without project filter
```

---

## 2. DESIGN PRINCIPLES

These principles govern every implementation decision. Violations must be justified.

**P1. Per-Task Project Resolution**
No global "active project" state. Every task carries its project_id. The governor resolves project context (repo path, git config, model keys) from the task's project_id at claim time. This eliminates an entire class of "wrong project" bugs because there is no mutable global state to get stale.

**P2. Database as Source of Truth**
Project definitions live in PostgreSQL, not config files. The dashboard reads/writes project config via API. A config file (system.json) exists only for bootstrap (seeding the initial "vibepilot" project on first run). After bootstrap, all project CRUD goes through the database.

**P3. Workspace Isolation by Directory**
Each project has a dedicated filesystem path (e.g. `/home/vibes/projects/sealed/`). The governor sets the agent's working directory to the task's project path. Git operations use that project's remote. The context builder loads code only from that path. Cross-project file access is not blocked at the OS level (unnecessary complexity), but is prevented by context scoping — the agent never receives context about other projects' files.

**P4. Graceful Backward Compatibility**
The migration creates a "vibepilot" project and backfills all existing tasks with its project_id. The governor defaults to the vibepilot project when project_id is null. VibePilot continues working identically before and after migration. No big-bang cutover.

**P5. Modular Project Definitions**
A project definition is self-contained. Adding a project means inserting a database row (and cloning a repo), not changing code. The governor, context builder, git operations, and frontend all read project config dynamically. Zero hardcoded project assumptions in application code.

**P6. Fair Model Allocation**
Each project declares which API keys it may use. The cooldown watcher tracks per-key status. When a project's keys are exhausted, only that project waits. Shared keys share cooldowns, which is acceptable. Per-project key allocation prevents one project from starving another.

**P7. Isolation Defense in Depth**
Isolation is enforced at THREE layers, not one:
1. **Config layer**: project config explicitly names which repo, which keys, which path
2. **Data layer**: all queries filter by project_id
3. **Context layer**: agents receive context only from their project's codebase

If any single layer fails, the other two prevent cross-contamination.

---

## 3. PROJECT DATA MODEL

### Enhanced projects table

```sql
CREATE TABLE projects (
  -- Identity
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug            TEXT UNIQUE NOT NULL,          -- URL-safe: "sealed", "extrayum"
  display_name    TEXT NOT NULL,                  -- "Sealed", "ExtraYum"
  description     TEXT,
  status          TEXT DEFAULT 'active'
                  CHECK (status IN ('active', 'paused', 'completed', 'archived', 'setup')),

  -- Git / Repository
  github_owner    TEXT NOT NULL,                  -- "VibesTribe"
  github_repo     TEXT NOT NULL,                  -- "sealed"
  repo_path       TEXT NOT NULL,                  -- "/home/vibes/projects/sealed"
  default_branch  TEXT DEFAULT 'main',
  branch_prefix_task   TEXT DEFAULT 'task/',
  branch_prefix_module TEXT DEFAULT 'module/',
  protected_branches   TEXT[] DEFAULT '{"main","master"}',

  -- Tech stack (drives context building and test/deploy commands)
  tech_stack      TEXT DEFAULT 'auto',            -- auto-detected or explicit: "nextjs", "go", "react", etc.
  build_command   TEXT,
  test_command    TEXT,
  lint_command    TEXT,
  typecheck_command TEXT,
  deploy_command  TEXT,

  -- Deploy
  deploy_target   TEXT DEFAULT 'none',            -- "vercel", "cloudflare", "self-hosted", "none"
  deploy_url      TEXT,                           -- "https://sealed.vibestribe.rocks"

  -- Model allocation (JSONB array of env var names this project may use)
  -- Example: ["GEMINI_API_KEY_1", "GEMINI_API_KEY_2", "GLM_API_KEY"]
  model_keys      JSONB DEFAULT '[]'::jsonb,

  -- Connected services (for honeycomb links panel)
  -- Each entry: {"type": "github", "label": "GitHub Repo", "url": "https://..."}
  connected_services JSONB DEFAULT '[]'::jsonb,

  -- Theme / Branding
  theme           JSONB DEFAULT '{}'::jsonb,      -- {"primary_color": "#6366f1", "logo_url": null, ...}

  -- Cumulative metrics (existing, preserved)
  total_tasks           INT DEFAULT 0,
  completed_tasks       INT DEFAULT 0,
  total_tokens_used     BIGINT DEFAULT 0,
  total_theoretical_cost FLOAT DEFAULT 0,
  total_actual_cost     FLOAT DEFAULT 0,
  total_savings         FLOAT DEFAULT 0,

  -- Timestamps
  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW()
);
```

### Why model_keys is JSONB not a join table

Simplicity. The list of keys a project may use changes rarely (when you allocate resources, not per-task). A JSONB array on the projects row avoids a join table that would add complexity without value. If we later need per-key usage tracking per project, we add a `project_key_usage` table then — not now.

### Why connected_services is JSONB

Same reason. This is display data for the honeycomb links panel. It's a list of clickable URLs with labels and icons. No relational queries needed. JSONB keeps it simple and flexible (different project types have different connected services).

---

## 4. IMPLEMENTATION PHASES

Each phase is independently shippable. Each phase produces a working system. Stopping after any phase leaves VibePilot functional.

### PHASE 1: Database + Config Foundation
**Goal:** The projects table exists, VibePilot can read project definitions from DB.
**Risk:** Low (additive only, no behavior change).
**Shippable result:** Governor reads project config from DB (falls back to hardcoded for vibepilot).

#### 1.1 Migration: Enhance projects table
- Add columns: slug, github_owner, github_repo, repo_path, tech_stack, build/test/lint/deploy commands, deploy_target, deploy_url, model_keys, connected_services, theme
- Preserve existing columns (ROI metrics, status, etc.)
- Make the existing schema_project_tracking.sql a subset of the new table

#### 1.2 Seed the vibepilot project
- INSERT a row for "vibepilot" with slug="vibepilot", the current hardcoded git config values, repo_path="/home/vibes/vibepilot"
- UPDATE all existing tasks SET project_id = vibepilot's UUID WHERE project_id IS NULL
- This is the backward-comcompatibility bridge

#### 1.3 Add project resolution to config loader
- New method: `Config.GetProjectConfig(projectID string) *ProjectConfig`
- Reads from projects table via existing DB client
- Returns a struct with all git, repo, and path settings for that project
- Falls back to the hardcoded system.json git config if projectID is empty or "vibepilot" (graceful degradation)

#### 1.4 Add ProjectConfig type
```go
type ProjectConfig struct {
    ID              uuid.UUID
    Slug            string
    DisplayName     string
    GitHubOwner     string
    GitHubRepo      string
    RepoPath        string
    DefaultBranch   string
    BranchPrefix    BranchPrefixConfig
    ProtectedBranches []string
    TechStack       string
    BuildCommand    string
    TestCommand     string
    DeployCommand   string
    DeployTarget    string
    DeployURL       string
    ModelKeys       []string
    ConnectedServices []ConnectedService
    Theme           map[string]interface{}
}
```

---

### PHASE 2: Governor Core — Per-Task Project Resolution
**Goal:** When the governor claims a task, it loads that task's project config and uses it for all operations.
**Risk:** Medium (changes core governor behavior, but backward-compatible via defaults).
**Shippable result:** Governor correctly switches git context based on each task's project_id.

#### 2.1 Inject project context into task claiming
- In `handleTaskAvailable`, after claiming a task, read its project_id
- Resolve ProjectConfig from DB (Phase 1.3)
- Pass ProjectConfig to the session factory and agent dispatch

#### 2.2 Make ContextBuilder project-aware
- Change ContextBuilder from single repoPath to per-project instantiation
- OR: add a `SetProject(repoPath string)` method called before each task
- The code map, file tree, and KB context all load from the task's project repo path
- jcodemunch index per project (each project gets its own code index)

#### 2.3 Make git operations project-aware
- GitConfig currently reads from system.json (one config). Change to:
  - Git operations accept a ProjectConfig parameter
  - Owner, repo, remote, branch prefixes all come from the ProjectConfig
  - `git push` targets the project's repo, not a hardcoded one

#### 2.4 Update session factory
- `buildAgentContext` needs to know which project it's building context for
- System prompt includes: "You are working on project {display_name} in directory {repo_path}. Repository: {github_owner}/{github_repo}."
- Agent's system prompt makes the project context explicit and unambiguous

---

### PHASE 3: Workspace Isolation
**Goal:** Each project has its own directory. Agents work only in their project's directory.
**Risk:** High (getting this wrong = cross-contamination, the user's #1 fear).
**Shippable result:** Sealed tasks run in ~/projects/sealed/, VibePilot tasks run in ~/vibepilot/. Zero confusion.

#### 3.1 Define project directory structure
```
/home/vibes/projects/
  sealed/           <- git clone of VibesTribe/sealed
  extrayum/         <- git clone of VibesTribe/extrayum
  web-of-wisdom/    <- git clone of VibesTribe/web-of-wisdom
```
VibePilot itself stays at /home/vibes/vibepilot (no move needed).

#### 3.2 Enforce working directory per task
- When dispatching an agent for a task, set the terminal cwd to the project's repo_path
- This is done via the existing worktree mechanism (which already sets a working directory)
- The worktree base path becomes per-project: `{repo_path}/.worktrees/`

#### 3.3 Per-project jcodemunch index
- Each project's code index lives at `{repo_path}/.context/`
- ContextBuilder loads the correct index based on the active project
- Project A's agents never see Project B's code symbols

#### 3.4 Isolation verification (automated test)
- Write a test that:
  1. Creates tasks for two different projects
  2. Claims each task
  3. Verifies the agent context contains ONLY the correct project's file tree
  4. Verifies git operations target ONLY the correct remote
  5. Verifies no file paths from the other project appear in context
- This test runs in CI and prevents regressions

---

### PHASE 4: Task Queue Project Scoping
**Goal:** Tasks are queued and claimed per-project. One project's backlog doesn't block another.
**Risk:** Low-Medium (query filter addition).
**Shippable result:** Dashboard can show per-project task lists. Governor processes tasks from any project.

#### 4.1 Add project_id to task claiming query
- The task claiming SQL (in handleTaskAvailable) adds `AND project_id = $1`
- BUT: the governor should claim from ANY project's queue (round-robin or priority-based)
- So the claim query is: claim the highest-priority available task across ALL projects
- Then load that task's project context (Phase 2)

#### 4.2 Per-project task views in API
- `GET /api/tasks?project_id={id}` returns only that project's tasks
- Dashboard uses this when a project is selected

#### 4.3 Per-project metrics
- Existing ROI functions (get_project_roi, get_all_projects_roi) already work per-project
- Extend to include: active task count, queued task count, last task timestamp

---

### PHASE 5: Model Pool Management
**Goal:** Per-project key allocation prevents one project from starving others.
**Risk:** Medium (affects throughput and reliability).
**Shippable result:** Sealed burning through its Gemini keys doesn't freeze ExtraYum.

#### 5.1 Read project's model_keys
- When routing a task to a model, filter available connectors by:
  1. The model is appropriate for the task type (existing routing logic)
  2. The model's api_key_ref is in the task's project model_keys list
- If no models match both criteria, the task waits (not fails)

#### 5.2 Shared key handling
- Keys listed in multiple projects' model_keys are shared
- Cooldown is per-key (not per-project), so a shared key cooling down affects all projects using it
- This is acceptable and correct behavior

#### 5.3 Model allocation UI
- Dashboard shows which keys are allocated to which project
- Visual indicator when a project has too few keys (risk of frequent cooldowns)
- Simple reassignment UI (drag keys between projects) — Phase 7 polish

---

### PHASE 6: Frontend — Honeycomb + Project Switching
**Goal:** The visual layer. Honeycomb overview, project-scoped dashboard, connected services panel.
**Risk:** Medium (frontend work, but no backend changes needed by this point).
**Shippable result:** User sees all projects, clicks one, sees only that project's data.

#### 6.1 Honeycomb overview page
- New route: `/` or `/projects`
- Fetches `GET /api/projects` → array of project summaries
- Renders as a grid of cells (honeycomb pattern)
- Each cell shows:
  - Project name + logo/icon
  - Status indicator (green/active, yellow/paused, gray/archived)
  - Task count (active / total)
  - Connected service icons (GitHub, Vercel, Cloudflare, etc.)
  - Last activity timestamp
- Click a cell → navigate to `/p/{slug}`

#### 6.2 Project-scoped dashboard
- Route: `/p/{slug}`
- Dashboard extracts slug from URL
- All data fetches include `?project={slug}` or `?project_id={id}`
- Header bar shows: project display_name + logo + theme primary_color
- The existing dashboard components (TaskCard, MissionControl, AgentHangar, etc.) work unchanged — they just receive project-filtered data

#### 6.3 Connected services panel
- Sidebar or expandable panel showing the project's connected_services
- Each entry is a clickable link with an icon
- Example: GitHub → opens github.com/VibesTribe/sealed in new tab
- Auto-login handled by browser saved credentials (no magic needed)

#### 6.4 Project switcher
- Dropdown in the header (always visible)
- Shows current project name + icon
- Click to see list of all projects
- Switching navigates to `/p/{other-slug}`
- Also includes "All Projects" option → returns to honeycomb

#### 6.5 VibesChat project awareness
- When user is on `/p/sealed` and opens chat, the chat knows the active project
- Chat system prompt includes project context
- User can say "create a task" and it goes to the right project
- If no project is selected (on honeycomb page), chat defaults to asking which project

---

### PHASE 7: Zero-Skill Project Onboarding
**Goal:** Creating a new project is one click + one form. Zero terminal, zero config files.
**Risk:** Medium (orchestrates everything from Phases 1-6).
**Shippable result:** "Create New Project" wizard fully automates setup.

#### 7.1 Create Project API endpoint
- `POST /api/projects` with body: `{ slug, display_name, description, github_owner }`
- Server-side actions:
  1. INSERT project row in DB with status='setup'
  2. Call GitHub API to create repo (using stored PAT)
  3. Clone the empty repo to `/home/vibes/projects/{slug}/`
  4. Set up default branch, branch protection
  5. UPDATE project status='active'
  6. Return project URL: `/p/{slug}`

#### 7.2 Create Project wizard (UI)
- Step 1: Name your project (auto-generates slug)
- Step 2: Pick a tech stack (Next.js, Go API, React SPA, Other) — optional, defaults to auto-detect
- Step 3: Connected services (checkboxes: GitHub ✓, Vercel, Cloudflare) — pre-selects GitHub
- Step 4: Model allocation (pick which API keys to assign) — pre-selects sensible defaults
- Step 5: Review + Create
- On submit: calls POST /api/projects, shows progress indicator, navigates to new project on success

#### 7.3 GitHub repo automation
- Uses GitHub API via stored PAT (already configured in system.json)
- Creates repo with: name=slug, private=true, auto_init=true (README), default_branch=main
- Adds VibePilot's deploy key to the new repo (for push access)
- Returns repo URL, stored in projects.github_repo

#### 7.4 Post-creation setup
- Run initial jcodemunch index on the new (likely empty) repo
- Create a "Project Setup" task in the new project's queue (first task for the agents)
- The first task might be: "Initialize the project with the selected tech stack"

---

## 5. SEQUENCING RATIONALE

Why this order and not another:

**Phase 1 (DB+Config) before Phase 2 (Governor):** You can't make the governor project-aware if the project data doesn't exist yet. The database migration is a prerequisite for everything.

**Phase 2 (Governor) before Phase 3 (Workspace):** The governor needs to know which project it's working on before we can enforce workspace isolation. Per-task project resolution is the foundation that workspace isolation builds on.

**Phase 3 (Workspace) before Phase 4 (Task Queue):** Once workspaces are isolated, task queue scoping is a simple query filter. If you did it the other way, you'd have per-project queues but agents still running in the wrong directory.

**Phase 5 (Models) is independent:** Model allocation can happen at any point after Phase 2. It's listed here because it's lower priority than isolation (the current global pool works, just not optimally for multiple projects).

**Phase 6 (Frontend) after all backend:** The frontend displays what the backend provides. Building the honeycomb before the API endpoints exist would mean building against mocks.

**Phase 7 (Onboarding) last:** It depends on every other phase. You can't automate project creation if the project management infrastructure doesn't exist yet.

---

## 6. RISK MITIGATION

### Risk: Cross-project contamination (user's #1 fear)

**Mitigation: Defense in depth (P7)**
- Config layer: project config explicitly defines repo, keys, path — no ambiguity
- Data layer: all queries filter by project_id — no cross-project data leakage
- Context layer: agents only see their project's codebase — no wrong-project file access
- Automated test (Phase 3.4): regression test that verifies isolation

### Risk: Model key exhaustion freezing everything

**Mitigation: Per-project allocation (P6)**
- Each project has its own key list
- One project's exhaustion only affects that project (and projects sharing the same keys)
- Dashboard shows key allocation so the user can rebalance

### Risk: Breaking existing VibePilot during migration

**Mitigation: Graceful backward compatibility (P4)**
- Migration creates "vibepilot" project and backfills all tasks
- Governor defaults to vibepilot project when project_id is null
- system.json remains as fallback for bootstrap
- No big-bang cutover — VibePilot works identically before and after

### Risk: Governor complexity explosion

**Mitigation: Per-task resolution pattern (P1)**
- No global mutable state for "active project"
- Each task carries its project context
- The governor's claiming logic is unchanged except for reading one extra column
- Context builder gets a repoPath parameter (it already has one, just makes it dynamic)

### Risk: Frontend complexity

**Mitigation: Project switching = data filter, not code fork**
- Same dashboard components, different data
- Project selector is a dropdown that changes the API query parameter
- No separate codebase per project, no if/else per project in components

---

## 7. TARGET PROJECTS (known)

| Project | Slug | Scope | Tech (TBD) | Priority |
|---------|------|-------|------------|----------|
| VibePilot | vibepilot | Self-maintenance + infrastructure | Go + React | Active (existing) |
| Sealed | sealed | Music agreement/contract platform | TBD | First new project |
| ExtraYum | extrayum | AI-assisted recipe/cooking app | TBD | Second |
| Web of Wisdom | web-of-wisdom | Global social wisdom-sharing platform | TBD | Third (large) |
| (Music apps) | TBD | Various music-related applications | TBD | Future |

---

## 8. WHAT SUCCESS LOOKS LIKE

After all phases:

1. User opens dashboard → sees honeycomb with VibePilot, Sealed, ExtraYum cells
2. Clicks "Sealed" → dashboard loads with Sealed branding, Sealed tasks, Sealed git history
3. Creates a task for Sealed → task goes to Sealed's queue with Sealed's project_id
4. Governor claims the task → loads Sealed's repo path, git config, model keys
5. Agent works on Sealed → context from Sealed's codebase, git pushes to Sealed's repo
6. Meanwhile, VibePilot's maintenance cron runs → operates on VibePilot's repo, zero interference
7. User switches to ExtraYum → completely different project data, no confusion
8. User clicks "Create New Project" → wizard creates repo, sets up workspace, navigates to dashboard
9. At no point does any agent, task, git operation, or file access cross project boundaries

---

## 9. ESTIMATED EFFORT PER PHASE

These are rough estimates based on codebase complexity. Actual effort depends on agent capability and model availability.

| Phase | Description | Est. Tasks | Complexity |
|-------|-------------|------------|------------|
| 1 | DB + Config Foundation | 4-6 | Low |
| 2 | Governor Per-Task Resolution | 5-8 | Medium |
| 3 | Workspace Isolation | 4-6 | High |
| 4 | Task Queue Scoping | 2-3 | Low |
| 5 | Model Pool Management | 3-5 | Medium |
| 6 | Frontend Honeycomb + Switching | 8-12 | Medium-High |
| 7 | Zero-Skill Onboarding | 4-6 | Medium |
| **Total** | | **30-46 tasks** | |

---

## 10. DECISION LOG

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Where project config lives | Database (not config files) | Enables API-driven CRUD, zero-dev-skill onboarding, dashboard management |
| How governor knows active project | Per-task project_id (not global state) | Eliminates stale-state bugs, enables concurrent multi-project processing |
| Workspace isolation method | Per-project directories + context scoping | Container isolation is overkill; context scoping is sufficient and simpler |
| Model key allocation | Per-project key lists in JSONB | Simple, flexible, no join table needed for rarely-changing allocation data |
| Routing | Path-based (/p/{slug}) | Zero DNS ops, works immediately, upgradeable to subdomains later |
| Frontend approach | Same components, project-filtered data | No code fork per project, minimal frontend changes |
| Migration strategy | Backfill + default to vibepilot | Zero downtime, zero breakage, backward compatible |
