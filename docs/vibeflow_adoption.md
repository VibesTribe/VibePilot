# VibePilot Enhancement: Adopt from Vibeflow v5

## From Vibeflow to VibePilot

Vibeflow v5 had 8,144 commits of production thinking. Key patterns to adopt:

### 1. REASON CODES (Add to task_manager.py)

```python
REASON_CODES = {
    "E001": "Missing dependency",
    "E002": "Selector changed", 
    "E003": "Session expired",
    "E004": "Schema invalid",
    "E005": "API timeout",
    "E006": "Rate limit hit",
    "E007": "Model hallucination detected",
    "E008": "Test failure",
    "E009": "Review rejection",
    "E010": "Context overflow",
}
```

### 2. 95% CONFIDENCE RULE (Add to Planner)

- No task enters plan if confidence < 0.95
- Auto-split tasks until children ≥ 0.95
- Store confidence in task_packets.tech_spec

### 3. WATCHER AGENT (New Agent)

```python
class WatcherAgent(Agent):
    """
    Monitors for:
    - Loops (same error 3+ times)
    - Drift (output doesn't match expected)
    - Timeouts (running > 30 min)
    - Token waste (repetitive context)
    
    Actions:
    - Emit reassigned with reason_code
    - Update platform registry success rate
    - Suggest model switch
    """
```

### 4. ROI CALCULATOR (Nightly Job)

```sql
-- Add to models table
ALTER TABLE models ADD COLUMN success_rate FLOAT DEFAULT 0;
ALTER TABLE models ADD COLUMN avg_latency_ms INT;
ALTER TABLE models ADD COLUMN total_tasks INT DEFAULT 0;
ALTER TABLE models ADD COLUMN roi_score FLOAT;

-- View for model selection
CREATE VIEW model_rankings AS
SELECT id, platform, 
       (success_rate * 0.4 + 
        (1 - token_used::FLOAT / NULLIF(token_limit, 0)) * 0.3 +
        (1 - request_used::FLOAT / NULLIF(request_limit, 0)) * 0.3) as roi
FROM models
WHERE status = 'active'
ORDER BY roi DESC;
```

### 5. CONTRACT VALIDATION (JSON Schemas)

Create `contracts/` folder with:
- `task_packet.schema.json`
- `event.schema.json`
- `prd.schema.json`
- `plan.schema.json`

Validate before inserting to DB.

### 6. PLATFORM REGISTRY (Enhance models table)

```sql
-- Already have: id, platform, courier, context_limit, strengths
-- Add:
ALTER TABLE models ADD COLUMN last_success TIMESTAMPTZ;
ALTER TABLE models ADD COLUMN last_failure TIMSTAMPTZ;
ALTER TABLE models ADD COLUMN consecutive_failures INT DEFAULT 0;
ALTER TABLE models ADD COLUMN ok_probe_url TEXT;
```

---

## Not Adopting (Different Architecture)

| Vibeflow Pattern | Why Not |
|------------------|---------|
| Event sourcing to files | Supabase handles state |
| @editable region patches | DB rows, not files |
| File-based manifests | Supabase queries |
| 5 CI workflows | Simplify for now |

---

## Implementation Order

1. ✅ Reason codes (add to handle_failure)
2. ✅ 95% confidence (add to planner)
3. ⏳ Watcher agent (new agent)
4. ⏳ ROI calculator (nightly job)
5. ⏳ Contract schemas (validation layer)
6. ⏳ Platform registry enhancement (schema update)

---

## Integration with Vibeflow Dashboard

The Vibeflow React dashboard can connect directly to Supabase:
```typescript
// apps/dashboard/src/lib/supabase.ts
import { createClient } from '@supabase/supabase-js'

export const supabase = createClient(
  import.meta.env.VITE_SUPABASE_URL,
  import.meta.env.VITE_SUPABASE_ANON_KEY
)

// Swap mock → live
const tasks = await supabase.from('tasks').select('*')
```

No API layer needed - Supabase JS client handles it.
