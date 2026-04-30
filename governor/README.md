# VibePilot Go Governor

The Go-based orchestrator for VibePilot. Replaces the Python orchestrator with a lean, efficient binary.

## Quick Start

```bash
# Set environment variables
export SUPABASE_URL="your-supabase-url"
export SUPABASE_SERVICE_KEY="your-service-key"
export GITHUB_TOKEN="your-github-token"

# Build and run
make build
./governor
```

## Architecture

```
governor/
├── cmd/governor/main.go      # Entry point
├── internal/
│   ├── sentry/               # Polls Supabase for tasks
│   ├── dispatcher/           # Routes to GitHub Actions or local CLI
│   ├── janitor/              # Resets stuck tasks
│   ├── server/               # HTTP API + WebSocket
│   ├── config/               # Config loading + hot-reload
│   ├── db/                   # Supabase client (direct SQL)
│   └── security/             # Leak detection, allowlist
├── pkg/types/                # Shared types
└── governor.yaml             # Configuration
```

## Components

### Sentry
- Polls Supabase every 15 seconds
- Drip-feeds tasks (max 3 concurrent)
- No webhooks = no 429 rate limits

### Dispatcher
- Routes based on `routing_flag`:
  - `web` → GitHub Actions (courier)
  - `internal` → Local CLI tools

### Janitor
- Resets stuck tasks (10 min timeout)
- Escalates after max attempts

### Server
- REST API: `/api/tasks`, `/api/models`, etc.
- WebSocket: `/ws` for real-time updates
- Embedded UI: `//go:embed dist/`

### Security (Claw patterns)
- Leak detector: Scans outputs for API keys
- HTTP allowlist: Blocks unauthorized requests

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/tasks` | GET | List available tasks |
| `/api/task/{id}` | GET | Get task details |
| `/api/models` | GET | List models |
| `/api/platforms` | GET | List platforms |
| `/api/roi` | GET | ROI report |
| `/ws` | WS | Real-time updates |

## Configuration

See `governor.yaml` for all options. Environment variables are expanded automatically.

## Building

```bash
make build        # Build for current OS
make cross        # Cross-compile for Linux/Mac
make docker       # Build Docker image
```

## RAM Target

- Binary: ~15MB
- Runtime: ~20-30MB
- Total: <50MB (fits e2-micro free tier)
