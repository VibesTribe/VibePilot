# VibePilot Current State
**Last Updated:** 2026-03-11 Session 81 (20:50 UTC)
**Status:** BROKEN - Flow not completing, tasks stuck

---

## SESSION 81 FAILURE - READ THIS FIRST

**What I did wrong this session:**
1. Manually called RPCs (`claim_for_review`, `claim_task`) to "test" things - this left stale `processing_by` values
2. Manually PATCHed tasks in the database - broke the flow
3. Made band-aid fixes instead of understanding the real problem
4. Added complexity (duplicate detection "fix") instead of debugging actual failure
5. Wasted ~200k tokens and entire session without fixing anything

**What I should have done:**
1. Clean database (run the SQL)
2. Delete all git branches
3. Rebuild governor binary
4. Trigger ONE test
5. Watch logs WITHOUT TOUCHING ANYTHING
6. Debug from actual failure point

**DO NOT DO THESE THINGS:**
- DO NOT manually call RPCs like `claim_for_review`, `claim_task`
- DO NOT manually PATCH tasks in the database
- DO NOT manually set `processing_by` or `processing_at`
- DO NOT "test" individual parts - test the WHOLE flow
- DO NOT add more complexity - fix the actual bug

---

## THE ACTUAL PROBLEM (Unknown)

We don't know why tasks get stuck. The Session 80 atomic operations SHOULD work.

**To find the real problem:**
1. Clean everything
2. Run ONE test
3. Watch logs at each step
4. Find where it ACTUALLY fails
5. Fix THAT specific issue

---

## MIGRATIONS APPLIED SESSION 81

| Migration | Purpose |
|-----------|---------|
| 086_restore_processing_functions.sql | Restore set_processing/clear_processing |
| 088_add_missing_task_columns.sql | Add task_number, confidence, category, etc. |

---

## GO CODE CHANGED SESSION 81

| File | Change | May or may not be correct |
|------|--------|---------------------------|
| `governor/internal/db/rpc.go` | Added RPCs to allowlist | Probably correct |
| `governor/cmd/governor/handlers_task.go` | Fixed claim_task params | Probably correct |
| `governor/cmd/governor/validation.go` | Use result.jsonb | Probably correct |
| `governor/internal/realtime/client.go` | Include processing_by in event key | Band-aid, may not be needed |

---

## CORRECT FLOW (From Session 80)

```
available → in_progress → review → testing → merged
                              ↓         ↓
                           (fail)    (fail)
                              ↓         ↓
                           available ←──┘
```

The atomic RPCs handle this. No manual intervention needed.

---

## NEXT SESSION - DO THIS EXACTLY

### Step 1: Clean Database
Run in Supabase SQL Editor:
```sql
DELETE FROM task_runs;
DELETE FROM task_packets;
DELETE FROM tasks;
DELETE FROM plans;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL;
SELECT 'Clean' as status;
```

### Step 2: Clean Git Branches
```bash
cd ~/vibepilot
git checkout main
git pull origin main
git branch -D $(git branch | grep -E "task/|TEST_MODULES" | tr -d ' *') 2>/dev/null
git push origin --delete $(git branch -r | grep -E "task/|TEST_MODULES" | sed 's/origin\///') 2>/dev/null
```

### Step 3: Rebuild Governor
```bash
cd ~/vibepilot/governor && go build -o governor ./cmd/governor && sudo systemctl restart governor
```

### Step 4: Trigger ONE Test
```bash
curl -s -X POST http://localhost:8080/webhooks -H "Content-Type: application/json" -H "X-GitHub-Event: push" -d '{"ref": "refs/heads/main", "repository": {"full_name": "VibesTribe/VibePilot"}, "commits": [{"id": "test", "added": ["docs/prd/test-hello-world.md"], "removed": [], "modified": []}]}'
```

### Step 5: Watch Logs - DO NOT TOUCH ANYTHING
```bash
journalctl -u governor -f
```

### Step 6: Check Database State
```bash
sudo bash -c 'source <(systemctl show governor -p Environment | sed "s/Environment=//" | tr " " "\n") && curl -s "${SUPABASE_URL}/rest/v1/tasks?select=id,status,processing_by,assigned_to" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"'
```

### Step 7: Find Where It Fails
- If stuck at `review` with `processing_by=null` → handler not firing
- If stuck at `review` with `processing_by=set` → handler started but didn't finish
- If stuck at `testing` → same logic
- Find the ACTUAL failure point from logs

### Step 8: Fix THAT Issue
- Not a band-aid
- Not more complexity
- The actual bug

---

## KEY FILES

| File | Purpose |
|------|---------|
| `docs/supabase-schema/084_clean_task_flow.sql` | Atomic RPCs - the real solution |
| `governor/cmd/governor/handlers_task.go` | Task execution and review handlers |
| `governor/cmd/governor/handlers_testing.go` | Testing handler |
| `governor/internal/realtime/client.go` | Event routing |

---

## REMEMBER

The Session 80 atomic operations were designed to solve this. They should work. If they don't, there's a specific bug somewhere. Find it. Don't add more complexity. Don't manually muck with the database. Just find and fix the actual bug.
