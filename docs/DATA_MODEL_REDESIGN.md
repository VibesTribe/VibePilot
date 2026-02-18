# VibePilot Data Model Redesign

## Current State Analysis

### Existing Tables (Supabase)

| Table | Columns | Rows | Purpose | Problems |
|-------|---------|------|---------|----------|
| `tasks` | 25 | 33 | Task queue | Status mismatch (pending vs available), no auto-transition |
| `task_runs` | 25 | 27 | Execution history | Good, but needs better rate limit tracking |
| `models` | 36 | 15 | ??? | Conflates models, tools, access methods, limits |
| `platforms` | 34 | 17 | Web courier destinations | Disconnected from models |
| `projects` | 12 | 2 | Project tracking | OK |
| `task_packets` | 10 | 7 | Prompt storage | OK |
| `secrets_vault` | 4 | 6 | Encrypted keys | OK |

### Config Files

**config/models.json** (11 models):
- Defines capabilities, access_via, theoretical costs
- Links to platforms via `access_via`
- Has `browser_control` capability flag

**config/platforms.json** (6 usable + 4 blocked):
- Web platforms with rate limits, auth requirements
- API pricing for ROI calculation
- Routing hints by task type

### RUNNER_REGISTRY (Python)

```
kimi, kimi-cli, kimi-internal → KimiContractRunner
deepseek, deepseek-chat → DeepSeekContractRunner  
gemini, gemini-api, gemini-2.0-flash, gemini-2.5-flash → GeminiContractRunner
courier-* → CourierContractRunner
```

---

## Current Problems

### 1. Models Table Conflates Everything

Current `models` table mixes:
- **AI Model**: kimi-k2.5, glm-5, deepseek-chat
- **Tool/Interface**: kimi-cli, opencode
- **Access method**: api, subscription, web
- **Status**: active, paused, benched
- **Rate limits**: But only single values, no multi-window tracking

Result: Orchestrator can't answer "What models can I use right now?"

### 2. No Multi-Window Rate Limit Tracking

Current: `request_limit`, `request_used`, `token_limit`, `token_used`

Missing:
- Requests per minute (rolling 60s)
- Requests per day (resets midnight PT)
- Tokens per minute
- Tokens per day
- Actual usage tracking per window
- Reset times for each window

Result: We killed Gemini API in 60 seconds because we couldn't see RPM.

### 3. No Learning/History

No way to:
- Estimate task token cost before dispatch
- Learn which models perform best on which task types
- Track patterns in failures

Result: System can't improve routing decisions over time.

### 4. Models ↔ Platforms Disconnected

`config/models.json` says `gpt-4o-mini` is available via `chatgpt-web`
But `models` table has gpt-4o-mini as a row
And `platforms` table has chatgpt as a row
No database relationship between them

Result: Routing logic can't query "which platforms have gpt-4o?"

### 5. Task Status Flow Broken

Planner writes tasks as `pending`
Orchestrator looks for `available`
Nothing transitions between them

Result: Tasks sit stuck, manual intervention needed.

---

## Proposed New Schema

### 1. `models` (AI Capabilities Only)

```sql
CREATE TABLE models (
  id TEXT PRIMARY KEY,                    -- 'glm-5', 'kimi-k2.5', 'deepseek-chat'
  name TEXT,                              -- 'GLM-5', 'Kimi K2.5'
  provider TEXT,                          -- 'zhipu', 'moonshot', 'deepseek', 'anthropic'
  
  -- Capabilities (what this AI can do)
  capabilities TEXT[],                    -- ['text', 'code', 'vision', 'browser_use', 'reasoning']
  context_limit INT,                      -- 128000, 200000, etc.
  
  -- Theoretical costs (for ROI calculation when using web platforms)
  cost_input_per_1k_usd FLOAT,
  cost_output_per_1k_usd FLOAT,
  
  -- Metadata
  notes TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 2. `tools` (Interfaces)

```sql
CREATE TABLE tools (
  id TEXT PRIMARY KEY,                    -- 'opencode', 'kimi-cli', 'direct-api', 'courier'
  name TEXT,
  type TEXT,                              -- 'cli', 'api', 'courier'
  
  -- What can this tool do?
  supported_providers TEXT[],             -- ['zhipu', 'moonshot'] or ['all']
  has_codebase_access BOOLEAN DEFAULT FALSE,
  has_browser_control BOOLEAN DEFAULT FALSE,
  
  -- Runner class mapping
  runner_class TEXT,                      -- 'KimiContractRunner', 'GeminiContractRunner'
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 3. `access` (How We Reach Models + Limits + Usage)

```sql
CREATE TABLE access (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id TEXT REFERENCES models(id),
  tool_id TEXT REFERENCES tools(id),
  platform_id TEXT REFERENCES platforms(id),  -- NULL for non-courier access
  
  method TEXT,                            -- 'api', 'subscription', 'web_free_tier'
  priority INT DEFAULT 1,                 -- 0=subscription(best), 1=web, 2=api(paid)
  
  -- Status
  status TEXT DEFAULT 'active',           -- 'active', 'paused', 'benched', 'cooldown'
  status_reason TEXT,
  cooldown_until TIMESTAMPTZ,
  
  -- Rate Limits (what we're allowed)
  requests_per_minute INT,
  requests_per_hour INT,
  requests_per_day INT,
  tokens_per_minute INT,
  tokens_per_day INT,
  
  -- Current Usage (rolling windows)
  requests_last_minute INT DEFAULT 0,
  requests_last_hour INT DEFAULT 0,
  requests_today INT DEFAULT 0,
  tokens_last_minute INT DEFAULT 0,
  tokens_today INT DEFAULT 0,
  
  -- Reset tracking
  daily_reset_at TIMESTAMPTZ,             -- When daily limits reset
  
  -- Subscription info (if applicable)
  subscription_cost_usd FLOAT,
  subscription_started_at TIMESTAMPTZ,
  subscription_ends_at TIMESTAMPTZ,
  
  -- API credentials (if applicable)
  api_key_ref TEXT,                       -- Vault key reference
  
  -- Learning stats
  total_tasks INT DEFAULT 0,
  successful_tasks INT DEFAULT 0,
  failed_tasks INT DEFAULT 0,
  avg_tokens_per_task FLOAT DEFAULT 0,
  p95_tokens_per_task INT DEFAULT 0,
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 4. `platforms` (Web Destinations - keep mostly as-is)

```sql
-- Keep existing platforms table, add:
ALTER TABLE platforms ADD COLUMN models_available TEXT[];  -- Which models on this platform
```

### 5. `task_history` (Learning)

```sql
CREATE TABLE task_history (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id),
  access_id UUID REFERENCES access(id),
  
  task_type TEXT,                         -- 'code', 'research', 'refactor'
  estimated_tokens INT,
  actual_tokens_in INT,
  actual_tokens_out INT,
  actual_requests INT DEFAULT 1,
  
  success BOOLEAN,
  failure_reason TEXT,
  failure_code TEXT,
  
  duration_ms INT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Data Migration Plan

### Phase 1: Create New Tables (No Data Loss)

1. Create `models_new`, `tools`, `access`, `task_history`
2. Keep existing tables with `_backup` suffix
3. No changes to `tasks`, `task_runs`, `platforms` yet

### Phase 2: Populate New Tables

From config/models.json:
```
models_new:
- glm-5, kimi-k2.5, deepseek-chat, deepseek-r1
- gemini-2.0-flash, gemini-2.5-flash
- gpt-4o, gpt-4o-mini
- claude-sonnet-4-5, claude-haiku-4-5

tools:
- opencode (CLI, GLM subscription)
- kimi-cli (CLI, Kimi subscription)
- direct-api (API runner)
- courier (browser automation)

access:
- glm-5 via opencode (subscription, priority 0)
- kimi-k2.5 via kimi-cli (subscription, priority 0, 7 days left)
- deepseek-chat via direct-api (api, priority 2, paused - no credit)
- gemini-2.5-flash via direct-api (api, priority 2, paused - quota)
- gpt-4o via courier/chatgpt (web_free_tier, priority 1)
- claude-sonnet-4-5 via courier/claude (web_free_tier, priority 1)
- etc.
```

### Phase 3: Update Orchestrator

1. Query `access` table joined with `models` and `tools`
2. Check rate limits before dispatch
3. Record to `task_history` after completion
4. Update usage counters

### Phase 4: Wire Pipeline Flow

1. Planner writes tasks as `pending`
2. Trigger: Supervisor auto-reviews `pending` tasks
3. Supervisor approves → status becomes `available`
4. Orchestrator (always running) picks up `available` tasks

---

## Example Queries (New Schema)

### Get all available access methods for a task:

```sql
SELECT 
  a.id,
  m.id as model_id,
  m.capabilities,
  t.id as tool_id,
  a.method,
  a.priority,
  a.status,
  a.requests_today,
  a.requests_per_day,
  a.tokens_today,
  a.tokens_per_day
FROM access a
JOIN models m ON a.model_id = m.id
JOIN tools t ON a.tool_id = t.id
WHERE a.status = 'active'
  AND 'code' = ANY(m.capabilities)
  AND (a.requests_per_day IS NULL OR a.requests_today < a.requests_per_day * 0.8)
  AND (a.tokens_per_day IS NULL OR a.tokens_today < a.tokens_per_day * 0.8)
ORDER BY a.priority, a.successful_tasks::FLOAT / NULLIF(a.total_tasks, 0) DESC;
```

### Check if we can dispatch to this access method:

```sql
SELECT can_dispatch(a.id, estimated_tokens, estimated_requests)
FROM access a WHERE a.id = 'xxx';
```

### Get best model for task type based on history:

```sql
SELECT 
  a.id,
  a.model_id,
  AVG(th.actual_tokens_in + th.actual_tokens_out) as avg_tokens,
  COUNT(*) as tasks,
  SUM(CASE WHEN th.success THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate
FROM access a
JOIN task_history th ON a.id = th.access_id
WHERE th.task_type = 'code'
GROUP BY a.id, a.model_id
ORDER BY success_rate DESC, avg_tokens ASC;
```

---

## Questions Before Implementation

1. **Keep old models table?** Yes, rename to `models_backup` first
2. **Migrate existing data?** Yes, but config files are source of truth
3. **Update dashboard?** After backend works
4. **Test with single task first?** Yes, full validation before going live

---

## Status

- [x] Analyzed current state
- [ ] Review design with user
- [ ] Create new tables
- [ ] Populate from config files
- [ ] Update orchestrator
- [ ] Wire pipeline flow
- [ ] Test end-to-end
- [ ] Remove old tables
