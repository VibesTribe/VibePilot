# VibePilot Status

**Last Updated:** 2026-02-13 23:55 UTC  
**Session:** Foundational Build Complete  
**Next Phase:** Council Activation + Courier Agent

---

## Quick Status

| Component | Status | Notes |
|-----------|--------|-------|
| Core Schema | ✅ Complete | All tables in Supabase |
| Pipeline Test | ✅ Passed | Full 12-stage flow |
| Kimi Integration | ✅ Working | CLI dispatch functional |
| Role System | ✅ Defined | 7 roles, 2-3 skills each |
| Config System | ✅ Ready | `config/vibepilot.yaml` |
| GLM-5 (OpenCode) | ✅ Active | Primary coder/supervisor |
| Kimi K2.5 | ✅ Active | Parallel executor |
| Gemini Flash | ✅ Active | Research (rate-limited) |
| DeepSeek | ⏸️ Paused | Backup, low credits |

---

## Terminal Crash Recovery

If your SSH terminal crashes, run this in a new terminal:

```bash
cd ~/vibepilot && cat STATUS.md
```

Then to continue:

```bash
cd ~/vibepilot
git pull
source venv/bin/activate
# Continue from "Next Steps" below
```

---

## Where Things Are

| What | Location |
|------|----------|
| **Config (everything)** | `config/vibepilot.yaml` |
| **Session Log** | `docs/SESSION_LOG.md` |
| **PRD** | `docs/prd_v1.3.md` |
| **Architecture** | `docs/architecture_diagram.md` |
| **Schemas** | `docs/schema_*.sql` |
| **Role Definitions** | `core/roles.py` |
| **Kimi Runner** | `runners/kimi_runner.py` |
| **Dual Orchestrator** | `dual_orchestrator.py` |
| **TaskManager** | `task_manager.py` |
| **Agents** | `agents/` |

---

## Next Steps

1. [ ] **Project Selector** - Multi-project support (recipe app, finance app, VibePilot, etc.)
2. [ ] **Activate Council** - 3-model independent review for plans
3. [ ] **Build Courier Agent** - Dispatch to web platforms
4. [ ] **Gemini Integration** - Add API runner for research
5. [ ] **Voice Interface** - "Talk to Vibes" via Deepgram/Kokoro
6. [ ] **Dashboard Connection** - Vibeflow → Supabase

---

## Model Registry

| Model | Platform | Role | Status |
|-------|----------|------|--------|
| GLM-5 | OpenCode | Primary coder/supervisor | ✅ Active |
| Kimi K2.5 | CLI | Parallel executor | ✅ Active |
| Gemini Flash | Google AI | Research/council | ✅ Active (limited) |
| DeepSeek | API | Backup | ⏸️ Paused |

---

## Key Commands

```bash
# Test pipeline
python docs/scripts/pipeline_test.py

# Test Kimi dispatch
python docs/scripts/kimi_dispatch_demo.py

# Test dual orchestrator
python dual_orchestrator.py

# Check model registry
source venv/bin/activate && python -c "
from supabase import create_client
from dotenv import load_dotenv
import os
load_dotenv()
db = create_client(os.getenv('SUPABASE_URL'), os.getenv('SUPABASE_KEY'))
for m in db.table('models').select('*').execute().data:
    print(f\"{m['id']}: {m['status']}\")
"
```

---

## Architecture Principles

- **2-3 skills max per agent** - Prevents scope creep
- **Models wear hats** - Same model, different roles
- **Config-driven** - Edit `config/vibepilot.yaml`, not code
- **State in Supabase** - All data in DB, not files
- **80% threshold** - Pause/warn before limits
- **No agent chat** - Council reviews independently

---

## Context Window Safety

If approaching context limit, say **"Compress session"**:

1. I update `docs/SESSION_LOG.md` with full summary
2. Start fresh session
3. I read `STATUS.md` and `SESSION_LOG.md` to restore context

---

## Files To Know

```
vibepilot/
├── STATUS.md              ← YOU ARE HERE
├── config/
│   └── vibepilot.yaml     ← ONE file for all config
├── docs/
│   ├── SESSION_LOG.md     ← Detailed session history
│   ├── prd_v1.3.md        ← Full PRD
│   └── schema_*.sql       ← Database schemas
├── core/
│   └── roles.py           ← Role definitions
├── runners/
│   └── kimi_runner.py     ← Kimi CLI integration
├── agents/                ← 8 agent implementations
├── task_manager.py        ← Task CRUD/claiming
└── dual_orchestrator.py   ← Multi-model routing
```

---

**For AI Assistant:** Always read this file first when starting a new session. Check "Next Steps" for priorities. Read `docs/SESSION_LOG.md` for detailed history.
