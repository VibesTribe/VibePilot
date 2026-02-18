# VibePilot: What Is Where

Quick reference for navigating the project. Update when structure changes.

---

## Running VibePilot

```bash
# Always use venv Python
cd ~/vibepilot
./venv/bin/python <script>

# Example: Run orchestrator
./venv/bin/python core/orchestrator.py

# Example: Run tests
./venv/bin/python -m pytest tests/
```

---

## Key Directories

| Directory | What's There |
|-----------|--------------|
| `config/` | JSON config files (models, roles, destinations, routing) |
| `core/` | Orchestrator, config_loader, telemetry |
| `runners/` | Contract runners (kimi, deepseek, gemini, courier) |
| `agents/` | Agent implementations (supervisor, council, etc) |
| `scripts/` | Utility scripts (sync, backup, migrations) |
| `tests/` | Test files |
| `docs/` | Documentation, schemas, PRD |
| `prompts/` | Agent prompts (legacy location) |
| `config/prompts/` | Agent prompts (canonical location) |
| `venv/` | Python virtual environment (USE THIS) |

---

## Config Files (`config/`)

| File | Purpose | Status |
|------|---------|--------|
| `destinations.json` | WHERE tasks execute (cli/web/api) | NEW v1.1 |
| `roles.json` | WHAT job is done (13 roles) | NEW v2.0 |
| `models.json` | WHO provides intelligence (LLMs) | v2.0 |
| `routing.json` | WHY/strategy (web_first, priorities, throttle) | NEW v1.0 |
| `tools.json` | HOW execution happens (browser-use, etc) | v2.0 |
| `routing_contract.json` | Routing decision schema | NEW v1.0 |
| `skills.json` | Skill definitions | v1.0 |
| `agents.json` | OLD - replaced by roles.json | DEPRECATED |
| `platforms.json` | OLD - replaced by destinations.json | DEPRECATED |

---

## Supabase Tables

| Table | Maps To | Notes |
|-------|---------|-------|
| `models` | models.json + CLI/API destinations | Internal models (Q tier) |
| `platforms` | web destinations | Web platforms (W tier) |
| `tasks` | Task queue | Has routing_flag column |
| `task_runs` | Execution records | Has tokens_in/out, courier tracking |
| `secrets_vault` | API keys | Encrypted, use vault_manager.py |
| `skills` | skills.json | Optional sync |
| `tools` | tools.json | Optional sync |
| `prompts` | prompts/*.md | Optional sync |

**Does NOT have:** destinations, roles, routing tables (JSON-only for now)

---

## Sync: JSON ↔ Supabase

```bash
# Import: JSON → Supabase
./venv/bin/python scripts/sync_config_to_supabase.py

# Export: Supabase → JSON
./venv/bin/python scripts/sync_config_to_supabase.py --export
```

**Currently syncs:** models.json, platforms.json, skills.json, tools.json, prompts/

**NOT YET synced:** destinations.json, roles.json, routing.json

---

## Vault (API Keys)

```python
# Location: vault_manager.py
from vault_manager import get_api_key, VaultManager

# Get a key
key = get_api_key('GEMINI_API_KEY')

# Add a key
vault = VaultManager()
vault.ingest_secret('GEMINI_API_KEY', 'your-key-here')
```

**Keys in vault:**
- DEEPSEEK_API_KEY ✓
- GITHUB_TOKEN ✓
- GEMINI_API_KEY - NOT YET ADDED

**Bootstrap keys (in .env, not vault):**
- SUPABASE_URL
- SUPABASE_KEY
- VAULT_KEY

---

## Dashboard (Vibeflow)

**Location:** `~/vibeflow/`

**Key file:** `apps/dashboard/lib/vibepilotAdapter.ts`
- Reads from Supabase tables (models, platforms, tasks, task_runs)
- Transforms to dashboard shape
- Calculates ROI metrics

**Supabase client:** `apps/dashboard/lib/supabase.ts`

---

## Courier Runner

**Location:** `runners/contract_runners.py`
- `CourierContractRunner` class (lines 413-704)
- Uses browser-use for web automation
- Needs LLM with `browser_control` capability

**Hardcoded platforms (line 427-443):**
```python
WEB_PLATFORMS = {
    "chatgpt": {...},
    "claude": {...},
    "gemini": {...},
    # MISSING: huggingchat, deepseek-web, copilot-web
}
```

---

## Orchestrator

**Location:** `core/orchestrator.py`
- `ConcurrentOrchestrator` class
- `CooldownManager` - handles 80% pause
- `UsageTracker` - tracks usage
- `RunnerPool` - loads available runners

**Gap:** `_call_runner()` doesn't have courier dispatch yet

---

## Database Schemas

**Location:** `docs/supabase-schema/` (committed to GitHub)

**TO APPLY SCHEMAS:**
1. Wait for file to be committed to GitHub
2. Open GitHub: `VibePilot/tree/main/docs/supabase-schema/`
3. Copy SQL content from file
4. Go to Supabase Dashboard → SQL Editor
5. Paste and Run

**DO NOT:** Try to click links in opencode output (user cannot click)
**ALWAYS:** Commit schema files to `docs/supabase-schema/` so user can access from GitHub

| File | What It Adds |
|------|--------------|
| `schema_v1_core.sql` | Core tables (models, tasks, task_runs) |
| `schema_v1.1_routing.sql` | routing_flag, slice_id, task_number |
| `schema_v1.4_roi_enhanced.sql` | tokens_in/out, courier tracking, ROI functions |
| `schema_intelligence.sql` | Model/platform intelligence, weekly reports |
| `schema_performance_fix.sql` | Index fixes for Supabase warnings |
| `001_data_model_redesign.sql` | NEW: models_new, tools, access, task_history tables |

---

## Quick Checks

```bash
# Check venv packages
./venv/bin/pip list | grep -iE "browser-use|google-genai|supabase"

# Check vault keys
./venv/bin/python -c "from vault_manager import get_api_key; print(get_api_key('DEEPSEEK_API_KEY'))"

# Check git status
git status

# Check recent commits
git log --oneline -10

# Validate configs
./venv/bin/python core/config_loader.py
```

---

## Cleanup Log

Record what was removed/cleaned up to avoid re-investigating.

### Courier API Call Reality (2026-02-17)

**CRITICAL: browser-use is NOT 1 API call per task.**

Each browser-use step = 1 LLM API call. A simple task takes 4-8 calls:
1. Navigate to URL → 1 call
2. Find input element → 1 call  
3. Type prompt → 1 call
4. Submit → 1 call
5. Wait for response → 1-2 calls
6. Extract answer → 1 call

**Impact on free tier limits:**

| Platform | Limit | Courier Tasks/Hour (est) |
|----------|-------|--------------------------|
| Gemini API | 15 req/min, 1500/day | ~2-3/min, ~200/day |
| DeepSeek API | Credit-based | Depends on credit |

**Recommendation:**
- Simple queries → Use internal API (1 call)
- Complex/long tasks → Use courier (worth 4-8 calls)
- Research tasks → Courier to web platform (free access to best models)

### 2026-02-17 Session

| Item | Action | Reason |
|------|--------|--------|
| streamlit | REMOVED from requirements.txt | Failed experiment, never used |
| asyncio | REMOVED from requirements.txt | Built into Python, unnecessary |
| browser-use | ADDED to requirements.txt | Already in venv, needed for courier |
| google-genai | ADDED to requirements.txt | Already in venv, needed for courier |
| playwright | INSTALLED in venv | Browser automation for courier |
| langchain-openai | INSTALLED in venv | LLM interface for browser-use |
| langchain-google-genai | INSTALLED in venv | Alternative Gemini interface |
| test_*.py in root | Ignored via .gitignore | Messy test scripts, kept out of git |

**Investigated but kept:**
- `venv/` - Contains browser-use v0.11.9, google-genai v1.63.0, all needed packages
- `agents.json` - Kept as deprecated, roles.json replaces
- `platforms.json` - Kept as deprecated, destinations.json replaces

### Courier Status (2026-02-17)

**Working:**
- Playwright/Chromium installed and launches
- Browser navigates to web platforms
- No-auth platform URLs identified: chatgate.ai (ChatGPT), chat.deepseek.com

**Remaining:**
- LLM adapter for browser-use needs correct interface
- browser-use 0.11.x expects specific model interface (provider, ainvoke, etc.)
- LangChain models work but need Pydantic v2 compatibility fix

---

## Notes

- Always use `./venv/bin/python` - system Python doesn't have packages
- Config files are version-controlled, Supabase is live state
- Dashboard reads from Supabase, not JSON directly
- Courier needs GEMINI_API_KEY in vault before first test
