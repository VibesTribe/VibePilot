# Legacy Python Code

**This code is NOT used.** Kept for reference only.

---

## What Replaced It

The Go governor (`~/vibepilot/governor/`) handles everything:

| Python (Legacy) | Go (Current) |
|-----------------|--------------|
| `orchestrator.py` | `governor/cmd/governor/main.go` |
| `task_manager.py` | `governor/internal/db/` |
| `vault_manager.py` | `governor/internal/vault/` |
| `runners/` | `governor/internal/destinations/` |
| `core/` | `governor/internal/runtime/` |

---

## Why It's Here

- Reference for how things worked
- In case we need to port something missed
- Historical record

---

## Don't Use It

- No Python services are running
- `.env` is empty (keys are in systemd override)
- Python `load_dotenv()` won't find anything

If you think you need this code, you probably need to update the Go governor instead.
