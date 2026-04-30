# Supabase Webhooks Setup

## Quick Setup

Your GCE external IP: `34.45.124.117`
Port 8080 is already open (firewall rule: `allow-streamlit-8080`)

## Steps

### 1. Create Webhook in Supabase

1. Go to **Database → Webhooks** in your Supabase dashboard
2. Click **Create a new hook**
3. Configure:
   - **Name:** `governor-events`
   - **Enabled:** Yes
   - **URL:** `http://34.45.124.117:8080/webhooks`
   - **Secret:** (from vault - see below)
   - **HTTP method:** POST
   - **Content type:** JSON

### 2. Select Tables and Events

Add webhooks for these tables:

| Table | Events |
|-------|--------|
| `tasks` | INSERT, UPDATE |
| `plans` | INSERT, UPDATE |
| `prd_files` | INSERT, UPDATE |
| `research_suggestions` | INSERT, UPDATE |
| `maintenance_commands` | INSERT |
| `test_results` | INSERT |

### 3. Get Webhook Secret

The secret is stored in the vault. To retrieve it:

```bash
# Option 1: From Supabase SQL editor (service role required)
SELECT * FROM secrets_vault WHERE key_name = 'webhook_secret';

# Option 2: Create a new one if needed
# In Supabase SQL editor:
SELECT vault.create_secret('webhook_secret', 'your-new-secret-here');
```

Or generate a random one:
```bash
openssl rand -hex 32
```

Then store it:
```sql
SELECT vault.create_secret('webhook_secret', '<generated-secret>');
```

### 4. Test the Webhook

After creating the webhook:

1. Watch governor logs:
   ```bash
   sudo journalctl -u vibepilot-governor -f
   ```

2. Create a test task in Supabase:
   ```sql
   INSERT INTO tasks (project_id, title, status)
   VALUES (NULL, 'Test webhook task', 'available');
   ```

3. Check logs for:
   ```
   [Webhooks] Processed EventTaskAvailable from tasks
   ```

### 5. Verify Event Flow

The webhook server maps table changes to events:

| Table | Status Change | Event |
|-------|---------------|-------|
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

## Troubleshooting

### Webhook not firing

1. Check Supabase webhook logs: Database → Webhooks → [Your webhook] → Logs
2. Check governor is running: `sudo systemctl status vibepilot-governor`
3. Check port is open: `curl http://34.45.124.117:8080/webhooks` (should return 405 Method Not Allowed)

### Invalid signature errors

The webhook secret in Supabase must match what's in the vault:
```sql
SELECT * FROM secrets_vault WHERE key_name = 'webhook_secret';
```

### Events not being processed

Check the webhook server is receiving events:
```bash
sudo journalctl -u vibepilot-governor -f | grep -i webhook
```

## Architecture

```
Supabase (INSERT/UPDATE)
        ↓
    Webhook HTTP POST
        ↓
    Governor :8080/webhooks
        ↓
    Verify HMAC signature
        ↓
    Map to EventType
        ↓
    EventRouter.Route(event)
        ↓
    Handler functions (handlers_*.go)
```

## Security

- **HMAC verification:** All webhooks must be signed with the secret
- **Vault storage:** Secret is encrypted at rest in Supabase
- **Service role only:** Only service_role key can read the vault
- **No exposure:** Secret never appears in code or config files
