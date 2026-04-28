# VibePilot Agent Guide

**This project has a mandatory rules file. Read `.hermes.md` in the repo root first.**

It contains:
- 10 non-negotiable rules that exist because agents keep making the same mistakes
- Where everything lives (knowledge.db, index.db, config files)
- How to query for what you need instead of loading everything
- Post-task discipline (update docs after every task)
- Human boundaries (non-technical, visual decisions only)

**Quick start:**
1. Read `.hermes.md` -- 10 rules, ~2K tokens
2. Query `.context/knowledge.db` for specifics when you need them
3. Query `.context/index.db` for code lookups
4. `CURRENT_STATE.md` and `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md` for current state

**For humans:**
- Setup, startup, and rebuild instructions: `docs/STARTUP_GUIDE.md`
- All commands, config locations, and the two-copy system explained there

**For agents that can't read local files**, these are also on GitHub:
- https://github.com/VibesTribe/VibePilot/blob/main/.hermes.md
- https://github.com/VibesTribe/VibePilot/blob/main/docs/STARTUP_GUIDE.md
- https://github.com/VibesTribe/VibePilot/blob/main/CURRENT_STATE.md
- https://github.com/VibesTribe/VibePilot/blob/main/VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md
