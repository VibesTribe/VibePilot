# PIF: Project Isolation Framework — Complete Reference

## Overview
VibePilot uses a Project Isolation Framework (PIF) that gives each project its own
isolated directory, database partition, kanban, knowledgebase, code graph, and cost tracking.

## Phases A-H (ALL COMPLETE)

### Phase A: Scaffold System
- `scripts/pif_scaffold.py` creates: directory structure, vibepilot.toml, export.sh, restore.sh,
  .hermes.md, Hermes profile, git repo, GitHub remote, backup repo, SQLite DB
- Called automatically by `POST /api/projects` (handleProjectCreate)

### Phase B: Context Switching
- `governor/internal/runtime/project_context.go` loads project's .hermes.md, skills, file tree
- Agent sees ONLY the active project's files, not VibePilot's codebase

### Phase C: Database Separation
- project_id on: tasks, task_runs, project_costs, system_counters, project_snapshots, agent_sessions
- `update_project_cumulative()` rolls up per-project ROI
- `GET /api/project-roi?slug=X` returns per-project ROI data

### Phase D: Research Isolation
- project_id on: research_suggestions, research_reports, research_bookmarks, research_queue

### Phase E: Export & Transfer
- `export.sh` creates signed archive with secret scrubbing (--include-db for database)
- `restore.sh` verifies SHA256 signature before extraction

### Phase F: Backup Automation
- `scripts/pif_backup.sh --all` runs nightly at 3 AM (cron, no-agent script mode)
- Each project backed up to its private GitHub backup repo

### Phase G: Per-Project Systems
- project_id on: project_todos (kanban), kb_files, kb_knowledge_items, kb_doc_sections, kb_canon, kb_skills
- New table: code_graph_snapshots (per-project Understand Anything graphs)
- REST APIs: /api/todos, /api/todos/<id>, /api/code-graph?project=X
- Dashboard batch serves all tables filtered by project_id

### Phase H: Project Intake
- `POST /api/project-intake` accepts PRD markdown + tech stack
- Parses PRD headers/bullets into kanban items with auto-categories
- Saves PRD.md and TECH_STACK.md to project knowledgebase/
- Updates project .hermes.md with blueprint summary

## Key API Endpoints
- `GET /api/dashboard?project=<slug>` — all project data, filtered by project_id
- `GET /api/todos?project=<slug>` — per-project kanban items
- `POST /api/todos` — create todo (requires project_slug)
- `PATCH /api/todos/<id>` — update todo status/title/etc
- `DELETE /api/todos/<id>` — delete todo
- `GET /api/code-graph?project=<slug>` — per-project code graph snapshot
- `GET /api/project-roi?slug=<slug>` — per-project ROI data
- `POST /api/project-intake` — submit PRD, seed kanban + KB
- `POST /api/projects` — create new project (triggers scaffold)
- `POST /api/projects/scaffold` — re-run scaffold for existing project

## Database Tables with project_id
tasks, task_runs, project_costs, system_counters, subscription_history,
project_todos, research_suggestions, research_reports, research_bookmarks,
research_queue, kb_files, kb_knowledge_items, kb_doc_sections, kb_canon,
kb_skills, code_graph_snapshots, project_snapshots, agent_sessions

## Token Tracking Chain
1. Task completes -> `create_task_run()` inserts run with project_id
2. `increment_lifetime_counters(tokens, cost, project_id)` updates per-project counter
3. `aggregate_task_costs(task_id)` sums task_runs -> updates tasks.total_tokens_in/out
4. `update_project_cumulative(project_id)` sums tasks -> updates projects table
5. `get_project_roi(project_id)` reads from projects table

## Migrations
048-053 in governor/supabase/migrations/
