# VibePilot Status

**Last Updated:** 2026-02-14 16:00 UTC  
**Session:** Strategic Guardrails + Master Plan Complete  
**Next Phase:** Council Activation + Prompt Caching

---

## Quick Status

| Component | Status | Notes |
|-----------|--------|-------|
| Core Schema | ✅ Complete | All tables in Supabase |
| Pipeline Test | ✅ Passed | Full 12-stage flow |
| Kimi Integration | ✅ Working | CLI dispatch functional |
| Role System | ✅ Defined | 7 roles, 2-3 skills each |
| Config System | ✅ Ready | `config/vibepilot.yaml` |
| **Guardrails** | ✅ Complete | `.context/` folder structure |
| **Master Plan** | ✅ Complete | Zero-ambiguity specification |
| **Migration Prep** | ✅ Ready | `scripts/prep_migration.sh` |
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

| What | Location | Purpose |
|------|----------|---------|
| **Master Plan** | `docs/MASTER_PLAN.md` | Zero-ambiguity system specification |
| **Guardrails** | `.context/guardrails.md` | Pre-code gates and production checklist |
| **Decision Log** | `.context/DECISION_LOG.md` | All architectural decisions |
| **Config (everything)** | `config/vibepilot.yaml` | Runtime configuration |
| **Session Log** | `docs/SESSION_LOG.md` | Session history |
| **PRD** | `docs/prd_v1.3.md` | Product requirements |
| **Architecture** | `docs/architecture_diagram.md` | Visual architecture |
| **Schemas** | `docs/schema_*.sql` | Database schemas |
| **Role Definitions** | `core/roles.py` | Role implementations |
| **Kimi Runner** | `runners/kimi_runner.py` | Kimi CLI integration |
| **Dual Orchestrator** | `dual_orchestrator.py` | Multi-model routing |
| **TaskManager** | `task_manager.py` | Task CRUD/claiming |
| **Agents** | `agents/` | Agent implementations |

---

## Next Steps

1. [ ] **Commit MASTER_PLAN.md** to GitHub
2. [ ] **DEC-004: Prompt Caching** - Implement in runners
3. [ ] **DEC-005: Kimi Swarm** - Add trigger to orchestrator
4. [ ] **DEC-006: TypeScript** - Decision needed
5. [ ] **Council Activation** - Wire 3 models to review roles
6. [ ] **Courier Agent** - Build for web platforms
7. [ ] **Migration Prep** - Test on fresh machine

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
