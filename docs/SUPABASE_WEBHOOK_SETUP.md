# Supabase Webhook Setup

## Your GCE External IP

```
34.45.124.117
```

## Webhook Configuration

Go to your Supabase dashboard:
```
https://supabase.com/dashboard/project/YOUR_PROJECT_ID/database/webhooks
```

(Replace `YOUR_PROJECT_ID` with your actual project ID from the URL when you're logged in)

### Create Webhooks

You need to create **6 separate webhooks** (one for each table we want to monitor).

**For each webhook, fill in:**

---

### Webhook 1: Tasks Table

**Name:**
```
governor-tasks
```

**Table:**
```
tasks
```

**Events:**
```
Insert, Update
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

**Secret:** (leave empty for now)

---

### Webhook 2: Plans Table

**Name:**
```
governor-plans
```

**Table:**
```
plans
```

**Events:**
```
Insert, Update
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

---

### Webhook 3: PRD Files Table

**Name:**
```
governor-prd-files
```

**Table:**
```
prd_files
```

**Events:**
```
Insert, Update
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

---

### Webhook 4: Research Suggestions Table

**Name:**
```
governor-research
```

**Table:**
```
research_suggestions
```

**Events:**
```
Insert, Update
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

---

### Webhook 5: Maintenance Commands Table

**Name:**
```
governor-maintenance
```

**Table:**
```
maintenance_commands
```

**Events:**
```
Insert
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

---

### Webhook 6: Test Results Table

**Name:**
```
governor-test-results
```

**Table:**
```
test_results
```

**Events:**
```
Insert
```

**Webhook URL:**
```
http://34.45.124.117:8080/webhooks
```

---

## Test the Webhooks

After creating all 6 webhooks:

### Test 1: Create a Plan
```sql
INSERT INTO plans (project_id, prd_path, status)
VALUES (NULL, 'docs/prd/test.md', 'draft');
```

### Test 2: Check governor logs
```bash
sudo journalctl -u vibepilot-governor -n 20 --no-pager
```

Look for:
```
[Webhooks] Processed EventPlanCreated from plans
```

### Test 3: Clean up test data
```sql
DELETE FROM plans WHERE prd_path = 'docs/prd/test.md';
```

---

## Architecture

```
Supabase (INSERT/UPDATE)
        ↓
    Webhook HTTP POST
        ↓
    Governor :8080/webhooks
        ↓
    Verify HMAC signature (optional)
        ↓
    Map to EventType
        ↓
    EventRouter.Route(event)
        ↓
    Handler functions (handlers_*.go)
```

## Event Mapping

| Table | Status Change | Event Triggered |
|-------|---------------|-----------------|
| `tasks` | status = 'available' | `EventTaskAvailable` |
| `tasks` | status = 'review' | `EventTaskReview` |
| `tasks` | status = 'testing'/'approval' | `EventTaskCompleted` |
| `plans` | status = 'draft' | `EventPlanCreated` |
| `plans` | status = 'review' | `EventPlanReview` |
| `plans` | status = 'council_review' | `EventCouncilReview` |
| `plans` | status = 'approved' | `EventPlanApproved` |
| `research_suggestions` | status = 'ready' | `EventResearchReady` |
| `test_results` | INSERT | `EventTestResults` |
| `maintenance_commands` | INSERT | `EventMaintenanceCmd` |

---

## Troubleshooting

### Webhook not firing

1. Check Supabase webhook logs: Database → Webhooks → [Your webhook] → Logs
2. Check governor is running: `sudo systemctl status vibepilot-governor`
3. Check port is open: `curl http://34.45.124.117:8080/webhooks` (should return 405 Method Not Allowed)

### Events not being processed

Check the webhook server is receiving events:
```bash
sudo journalctl -u vibepilot-governor -f | grep -i webhook
```

---

## Security Note

Currently webhooks are unsecured (no secret). This is fine for development, but for production you should:

1. Add a secret to each webhook in Supabase
2. Store the secret in the vault:
```bash
python3 scripts/vault_manager.py set supabase_webhook_secret "your-secret-here"
```
3. Update `system.json`:
```json
{
  "webhooks": {
    "enabled": true,
    "port": 8080,
    "path": "/webhooks",
    "secret_vault_key": "supabase_webhook_secret"
  }
}
```

But for now, unsecured webhooks work fine for testing.
