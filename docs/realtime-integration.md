# Supabase Realtime Integration

## Overview

VibePilot uses Supabase Realtime (WebSocket-based) instead of pg_net webhooks for database change notifications.

### Why Realtime Instead of pg_net Webhooks

| pg_net Webhooks | Supabase Realtime |
|-----------------|-------------------|
| ❌ Worker often broken | ✅ Separate infrastructure |
| ❌ Requests queue indefinitely | ✅ Instant delivery |
| ❌ Outbound HTTP (egress) | ✅ Inbound WebSocket (no egress) |
| ❌ Rate limited | ✅ Included in free tier |
| ❌ Hard to debug | ✅ Clear connection status |

## Architecture

```
┌──────────────┐     WebSocket      ┌──────────────────┐
│   Governor   │◄───────────────────│ Supabase Realtime│
│  (Go client) │    wss://.../realtime    Server       │
└──────────────┘                    └──────────────────┘
       │                                    │
       │ Routes events                      │ Watches
       ▼                                    ▼
┌──────────────┐                    ┌──────────────────┐
│ EventRouter  │                    │   PostgreSQL     │
│  (handlers)  │                    │  (publication)   │
└──────────────┘                    └──────────────────┘
```

## How It Works

1. **Governor connects** to Supabase Realtime via WebSocket on startup
2. **Subscribes to INSERT events** on monitored tables (plans, tasks, etc.)
3. **When INSERT happens**, Realtime pushes event to Governor
4. **EventRouter processes** the event through existing handlers
5. **Idempotency check** prevents duplicate processing

## Configuration

### Database Setup (One-time)

Run this SQL in Supabase SQL Editor:

```sql
-- Enable Realtime on monitored tables
ALTER PUBLICATION supabase_realtime ADD TABLE public.plans;
ALTER PUBLICATION supabase_realtime ADD TABLE public.tasks;
ALTER PUBLICATION supabase_realtime ADD TABLE public.maintenance_commands;
ALTER PUBLICATION supabase_realtime ADD TABLE public.research_suggestions;
ALTER PUBLICATION supabase_realtime ADD TABLE public.test_results;
```

### Governor Configuration

Realtime is automatically enabled when `SUPABASE_URL` is set. The URL is converted:
- `https://xyz.supabase.co` → `wss://xyz.supabase.co/realtime/v1/websocket`

No additional configuration needed.

## Monitored Tables

| Table | Events | Triggers |
|-------|--------|----------|
| `plans` | INSERT | Plan creation flow |
| `tasks` | INSERT | Task execution |
| `maintenance_commands` | INSERT | Maintenance processing |
| `research_suggestions` | INSERT | Research flow |
| `test_results` | INSERT | Test result handling |

**Note:** Only INSERT events are subscribed to avoid thundering herd on UPDATEs.

## Implementation

### Files

| File | Purpose |
|------|---------|
| `governor/internal/realtime/client.go` | WebSocket client implementation |
| `governor/internal/runtime/config.go` | `GetRealtimeURL()` method |
| `governor/cmd/governor/main.go` | Client initialization |
| `docs/supabase-schema/063_enable_realtime.sql` | Database setup |

### Key Features

- **Automatic reconnection** with exponential backoff
- **Heartbeat** every 30 seconds to keep connection alive
- **Resubscription** on reconnect
- **Nested payload parsing** (Supabase wraps data in `data` field)
- **Event routing** through existing EventRouter

## Debugging

### Check Connection Status

```bash
sudo journalctl -u vibepilot-governor --since "1 minute ago" | grep Realtime
```

Expected output:
```
[Realtime] Connecting to xyz.supabase.co
[Realtime] Connected successfully
[Realtime] Subscribed to table plans (event: INSERT)
```

### Test Realtime

Insert a plan and watch for the event:

```bash
# Insert test plan
curl -X POST "https://xyz.supabase.co/rest/v1/plans" \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"status": "draft", "prd_path": "test.md"}'

# Check logs
sudo journalctl -u vibepilot-governor --since "10 seconds ago" | grep -E "Realtime|EventPlan"
```

Expected output:
```
[Realtime] INSERT on plans (id: abc-123)
[EventPlanCreated] Processing plan abc-123
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "Connection refused" | Firewall blocking | Check GCP firewall allows outbound 443 |
| "Table not in publication" | Schema not run | Run 063_enable_realtime.sql |
| "Already being processed" | Normal idempotency | This is correct behavior |
| No events received | Realtime disabled in Supabase | Check Dashboard → Database → Replication |

## Migration from pg_net Webhooks

The old pg_net webhook triggers can be left in place (they're not firing anyway). They don't interfere with Realtime.

To remove them (optional):
```sql
DROP TRIGGER IF EXISTS governor-plans ON plans;
-- Repeat for other tables
```

## Performance

- **Memory:** ~10MB for WebSocket client
- **CPU:** Minimal (event-driven)
- **Network:** One WebSocket connection, ~1KB per event
- **Latency:** <100ms from INSERT to event delivery

## History

- **2026-03-05:** Replaced pg_net webhooks with Realtime
  - pg_net worker was broken (requests queuing indefinitely)
  - Realtime provides more reliable delivery
  - No egress charges (inbound WebSocket)
