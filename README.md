# VibePilot

**Sovereign AI Execution Engine**

Human provides idea → VibePilot executes with zero drift.

---

## Quick Start

```bash
# 1. Clone
git clone git@github.com:VibesTribe/VibePilot.git
cd VibePilot

# 2. Configure
cp .env.example .env
# Edit .env with your credentials

# 3. Setup
./setup.sh

# 4. Start
source venv/bin/activate
python orchestrator.py
```

---

## Documentation

| File | Purpose |
|------|---------|
| `CURRENT_STATE.md` | **Start here** - What, where, current state |
| `CHANGELOG.md` | History, changes, rollback commands |
| `docs/MASTER_PLAN.md` | Full system specification |
| `docs/UPDATE_CONSIDERATIONS.md` | Daily improvement input |

**Read CURRENT_STATE.md + CHANGELOG.md → Know everything → Do anything**

---

## Architecture

- **State:** Supabase (tasks, models, platforms, projects)
- **Code:** GitHub (versioned, tracked)
- **Config:** `config/vibepilot.yaml` (all runtime config)
- **Models:** GLM-5 (primary), Kimi (parallel), Gemini (research)

---

## Key Features

- **Context Isolation:** Task agents see ONLY their task (zero drift)
- **Config-Driven Swaps:** Change models/roles in 30 seconds, no code edits
- **Council Review:** Iterative consensus for PRDs, one-shot for updates
- **Single Source of Truth:** Two files for full context restoration

---

## Maintenance

```bash
# Daily backup
./scripts/backup_supabase.sh

# Prep for migration
./scripts/prep_migration.sh

# Validate schema (TODO)
./scripts/validate_schema.sh
```

---

## Status

**Built & Working:** Core schema, model registry, Kimi runner, dual orchestrator, role system

**Next:** Schema audit, prompt caching, Council RPC, courier agent

---

## License

MIT

---

**For AI assistants:** Read `CURRENT_STATE.md` first. It contains everything you need.
