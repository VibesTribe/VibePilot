# Vibeflow Dashboard Analysis

**Live Dashboard (Mock Data):** https://vibestribe.github.io/vibeflow/
**GitHub Repo:** https://github.com/VibesTribe/vibeflow

---

## Dashboard Structure (mission-control-mockup.tsx)

### Layout (3-column)
```
+----------+------------------------+----------+
| SLICE    |      CENTER HUB        |  AGENT   |
| DOCK     |   (scrollable grid)    |  HANGAR  |
| (left)   |                        |  (right) |
|          |                        |          |
| [Docs]   |  +-----------------+   |  [Add]   |
|          |  |   Slice Hub     |   |          |
| ○ Slice1 |  |  with orbiting  |   |  Agent1  |
| ○ Slice2 |  |    agents       |   |  Agent2  |
| ○ Slice3 |  +-----------------+   |  Agent3  |
| ...      |                        |  ...     |
+----------+------------------------+----------+
```

### Header (sticky top)
- **Vibes Orb** - Audio agent button, gold pulse animation
- **MISSION CONTROL · Vibeflow** - Title
- **Token Counter** - Pac-Man icon + count, opens ROI modal
- **View Logs** - Opens system logs modal

### Modals (6)
1. **Docs Modal** - Links to PRD, Task List, Technical docs
2. **Add Platform** - Form to add new platform/model
3. **ROI Calculator** - Token usage, virtual costs
4. **Logs** - Timestamped events (rate limits, timeouts, costs)
5. **Agent Details** - Tasks, success rate, tokens, warnings
6. **Slice Details** - Task list with status, locked state

### Agent Tiers (Badges)
- **W** (Worker) - Blue
- **M** (Maker) - Purple
- **Q** (Quality) - Amber

### Agent Status Colors
- testing: Yellow (#ffcc33)
- received: Cyan (#00e5ff)
- in progress: Blue (#3385ff)
- approved: Green (#10ffb0)
- human review: Orange (#ff9900)

### Current Mock Data

**Agents (7 in demo):**
| Agent | Tier | Task | Status | Warning |
|-------|------|------|--------|---------|
| Gemini | W | Task 1.24 | in progress | Timeout (refresh in 2h) |
| OpenAI GPT-5 | W | Task 1.25 | received | Out of credits |
| Claude QC | Q | Task 1.26 | testing | None |
| Roo IDE | M | Task 1.27 | approved | None |
| Cursor | M | Task 2.03 | in progress | None |
| Cline | M | Task 2.04 | received | None |
| OpenCode | Q | Task 2.05 | testing | None |

**Slices (12 in demo):**
| Slice | Tasks Done | Total | Tokens |
|-------|------------|-------|--------|
| Data Ingestion | 18 | 25 | 40,000 |
| Data Analysis | 12 | 20 | 32,000 |
| Auth & RBAC | 0 | 14 | 2,000 |
| Orchestration | 14 | 14 | 51,000 |
| UI Polish | 2 | 10 | 3,000 |
| MCP Bridges | 1 | 18 | 1,200 |
| Observability | 9 | 12 | 18,000 |
| Billing | 0 | 8 | 0 |
| Docs | 8 | 8 | 4,000 |
| CLI | 3 | 9 | 2,200 |
| Deploy | 5 | 11 | 7,600 |
| Benchmarks | 0 | 6 | 0 |

---

## Connecting to VibePilot (What Needs to Change)

### Data Sources (Supabase)
| Dashboard Component | VibePilot Source |
|---------------------|------------------|
| Agents | `config/models.json` + `task_runs` table |
| Slices | `tasks` table grouped by phase/type |
| Tokens | `task_runs.tokens_used` |
| Warnings | Platform limits from `config/platforms.json` |
| ROI | API pricing from `platforms.json` × tokens |

### Key Changes Needed
1. **Replace mock data arrays** with Supabase queries
2. **Add Supabase client** setup (similar to terminal_dashboard.py)
3. **Map VibePilot concepts** to Vibeflow UI:
   - VibePilot tasks → Vibeflow slices (grouped)
   - VibePilot models → Vibeflow agents
   - VibePilot task_runs → Vibeflow token counts
4. **Add real-time updates** (optional: WebSocket or polling)

### Files to Modify (in Vibeflow repo)
```
apps/dashboard/
├── src/
│   ├── lib/
│   │   └── supabase.ts     # NEW: Supabase client
│   ├── hooks/
│   │   └── useVibePilot.ts # NEW: Data fetching hooks
│   └── components/
│       └── mission-control-mockup.tsx  # MODIFY: Use real data
```

---

## Confirmed Baseline (2026-02-16)

**Branch:** `feature/admin-control-center-ui`

**Vercel Preview:** https://vibeflow-dashboard-git-feature-admi-1e8c37-vibestribes-projects.vercel.app/

**Includes:**
- Clean wide layout (no side columns)
- Admin Control Center modal
- All UI work preserved

**Next Step:** Create `feature/vibepilot-supabase` from this baseline
