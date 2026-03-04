# GitHub Webhook Setup

## Your GCE External IP

```
34.45.124.117
```

## Webhook Configuration

Go to your GitHub repository:
```
Settings → Webhooks → Add webhook
```

### Fill in these fields:

**Payload URL:**
```
http://34.45.124.117:8080/webhooks
```

**Content type:**
```
application/json
```

**Secret:**
```
(leave empty for now)
```

**SSL verification:**
```
Disable SSL verification
```
*Note: GitHub requires HTTPS in production. For development with HTTP, disable SSL verification.*

**Which events:**
```
Select: Just the push event
```

**Active:**
```
✓ (checked)
```

## Test the Webhook

After adding the webhook:

1. GitHub will send a test ping
2. Check governor logs:
```bash
sudo journalctl -u vibepilot-governor -f
```

3. Create a test PRD file:
```bash
cd ~/vibepilot
echo "# Test PRD" > docs/prd/test-webhook.md
git add docs/prd/test-webhook.md
git commit -m "test: webhook PRD detection"
git push origin main
```

4. Watch for these log messages:
```
[GitHub Webhooks] Processing push to VibesTribe/VibePilot (1 commits)
[GitHub Webhooks] New PRD detected (added): docs/prd/test-webhook.md
[GitHub Webhooks] Created plan for PRD: docs/prd/test-webhook.md
```

5. Check Supabase for the new plan:
```sql
SELECT * FROM plans WHERE prd_path = 'docs/prd/test-webhook.md';
```

## Cleanup Test

```bash
cd ~/vibepilot
git rm docs/prd/test-webhook.md
git commit -m "test: cleanup webhook test"
git push origin main
```

## Architecture

```
GitHub Push (docs/prd/*.md)
        ↓
POST http://34.45.124.117:8080/webhooks
        ↓
GitHub webhook handler detects PRD files
        ↓
Create plan via create_plan RPC
        ↓
Plan inserted (status='draft')
        ↓
Supabase webhook fires
        ↓
Governor receives EventPlanCreated
        ↓
Planner → Supervisor → Tasks → Execution
```

## Troubleshooting

### Webhook not firing
1. Check GitHub webhook settings: Settings → Webhooks → [your webhook] → Recent Deliveries
2. Check governor is running: `sudo systemctl status vibepilot-governor`
3. Check logs: `sudo journalctl -u vibepilot-governor -f`

### SSL errors
For development with HTTP, SSL verification must be disabled in GitHub webhook settings.

### No PRD detected
- Ensure file path starts with `docs/prd/`
- Ensure file ends with `.md`
- Check file doesn't contain `/processed/` in path (those are skipped)
