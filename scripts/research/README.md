# VibePilot Research System

Automated research from external sources (Raindrop, Gmail, Web) to feed continuous improvement.

## Components

### 1. Raindrop Research (`raindrop_researcher.py`)

Researches bookmarks from Raindrop.io collections for VibePilot relevance.

**Setup:**
1. Make your Raindrop collections public (Share → Make Public):
   - `vibeflow` - for general AI/tech bookmarks
   - `vibepilot` - specifically for VibePilot research items

2. Run research on last 7 days:
   ```bash
   python scripts/research/raindrop_researcher.py --collection vibeflow --days 7
   ```

3. Output goes to: `docs/research/raindrop-<collection>-<date>.md`

**For Private Collections:**
```bash
python scripts/research/raindrop_auth.py  # Run OAuth flow
```

**Automated Checking:**
```bash
# Check for new items (exits quietly if none)
python scripts/research/raindrop_researcher.py --collection vibepilot --watch

# Twice daily cron job (add to crontab):
0 9,15 * * * cd ~/vibepilot && python scripts/research/raindrop_researcher.py --collection vibepilot --watch && git add docs/research/ && git commit -m "Research: $(date +%Y-%m-%d) raindrop digest" && git push origin research-considerations
```

### 2. Daily Digest (Coming Soon)

Twice-daily research job:
- Check Raindrop (vibepilot folder)
- Check Gmail (flagged newsletters)
- Web research (new models, platforms)
- Compile and commit to research-considerations

## Research Output Format

All findings committed to `research-considerations` branch:
```
docs/research/
  raindrop-vibeflow-20260218.md
  raindrop-vibepilot-20260218.md
  daily-2026-02-18.md
```

## Evaluation Criteria

Research findings are scored 1-10 on VibePilot relevance:

| Score | Priority | Action |
|-------|----------|--------|
| 7-10 | 🔴 HIGH | Deep research, create tasks |
| 4-6 | 🟡 MEDIUM | Review for applicability |
| 1-3 | 🟢 LOW | Quick scan only |

**Categories tracked:**
- `models_platforms` - New AI models, free tiers, rate limits
- `architecture` - Agent patterns, orchestration, workflows
- `courier_browser` - Browser automation, web platforms
- `cost_optimization` - Pricing, efficiency, caching
- `infrastructure` - Supabase, GitHub, deployment

## Commit Workflow

All research commits to `research-considerations` branch:

```bash
git checkout research-considerations
git add docs/research/
git commit -m "Research: <date> - <brief summary>"
git push origin research-considerations
```

Research findings are reviewed by Council before implementation.
