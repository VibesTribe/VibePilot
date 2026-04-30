---
from: glm-5
to: kimi
type: task
created: 2026-02-18T21:45:00Z
---

## Research: Dependency RPC Schema Mismatch

### Problem
The RPC functions in Supabase are broken. They expect `dependencies` column to be `jsonb`, but it's currently `uuid[]`:

```sql
-- This FAILS because uuid[] doesn't support ?| operator
WHERE t.dependencies ?| ARRAY[p_completed_task_id::TEXT]
```

### What I Found
```
✗ unlock_dependent_tasks - ERROR: operator does not exist: uuid[] ?| text[]
✗ check_dependencies_complete - ERROR
✗ get_available_tasks - ERROR: structure mismatch
✗ claim_next_task - ERROR: multiple conflicting versions
✗ get_available_for_routing - NOT FOUND
```

### Research Needed

1. **Schema History**: Why is dependencies `uuid[]`? Was it always this way, or did something change?

2. **Impact Analysis**: If we change to jsonb:
   - Any existing data with dependencies?
   - Any other code that reads/writes dependencies?

3. **Approach Comparison**:
   - Option A: Change column to jsonb, fix RPCs
   - Option B: Keep uuid[], rewrite RPCs with `= ANY()` syntax
   - Pros/cons of each?

4. **Files to Check**:
   - `docs/supabase-schema/*.sql` - migration history
   - `task_manager.py` - how dependencies are written
   - `agents/planner.py` - how dependencies are set

### Expected Response
Recommendation with:
- Which approach to take
- SQL migration if needed
- Any code changes required
