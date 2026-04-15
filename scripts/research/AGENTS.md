# VibePilot Research System

**For any LLM/agent taking the System Researcher role:**

## Quick Start

1. **Pull latest from research-considerations branch:**
   ```bash
   git checkout research-considerations
   git pull origin research-considerations
   ```

2. **Check Raindrop token:**
   - Look in: `scripts/research/.raindrop_token.json`
   - If missing/expired: Run `python scripts/research/raindrop_get_token.py`

3. **Research bookmarks:**
   ```bash
   cd ~/vibepilot
   source venv/bin/activate
   python scripts/research/vibepilot_watcher.py
   ```

4. **Commit findings:**
   ```bash
   git add docs/research/
   git commit -m "Research: $(date +%Y-%m-%d) digest"
   git push origin research-considerations
   ```

## What We Research

The VIBEPILOT Raindrop collection (ID: 67118576) contains curated bookmarks.

**15 Research Categories:**
1. AI/ML Models & Platforms
2. Agent Architecture & Patterns
3. Browser Automation
4. Infrastructure & Deployment
5. Security & Privacy
6. Testing & QA
7. UI/UX & Developer Experience
8. Local/Edge Inference
9. Cost Optimization
10. Data Management
11. API Design & Integration
12. Workflow & Task Management
13. Learning Resources
14. Community & Ecosystem
15. Performance & Scaling

**Principle:** Learn from everything, even if we don't adopt directly.

## Cron Schedule (Automated)

Runs twice daily via cron:
- **10:00 AM** - Morning digest
- **8:00 PM** - Evening digest

```bash
0 10,20 * * * cd ~/vibepilot && git checkout research-considerations && git pull && source venv/bin/activate && python scripts/research/vibepilot_watcher.py && git add docs/research/ && git commit -m "VibePilot digest $(date +\%Y-\%m-\%d_\%H\%M)" && git push origin research-considerations
```

## Files

| File | Purpose |
|------|---------|
| `scripts/research/vibepilot_watcher.py` | Main watcher script |
| `scripts/research/.raindrop_token.json` | OAuth token (14-day expiry) |
| `scripts/research/.vibepilot_state.json` | Tracks last check time |
| `docs/research/vibepilot-digest-*.md` | Generated reports |

## Token Refresh

If token expires:
1. User visits: `https://raindrop.io/oauth/authorize?client_id=6995298e703bf1e11e73890a&redirect_uri=https%3A%2F%2Fvibestribe.github.io%2Fvibeflow%2F&response_type=code`
2. User provides authorization code
3. Run: `python scripts/research/raindrop_get_token.py`
4. Enter code when prompted

## Rules

- **Always commit to research-considerations branch**
- **Never commit to main without human approval**
- **Tag findings as SIMPLE (auto-approve) or VET (Council review)**
- **Focus on: What can we LEARN, not just what can we USE**

---
*This system is portable - works from any GCE instance, laptop, or environment.*
