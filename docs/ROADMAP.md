# VibePilot Roadmap
**Updated: April 18, 2026**

## Immediate (Before May 1)
- [ ] Apply migration 124 (Supabase SQL Editor)
- [ ] Verify pipeline runs clean end-to-end with real PRD
- [ ] Fix cascade retry bug (falls through on connector failure, not retry same model)
- [ ] Improve planner output parsing (handle GLM-5 malformed JSON with backticks)
- [ ] Implement testing-to-main merge (step 3) with testing folder approach

## Credit & API Key Management
- [ ] Wire credit tracking: record cost per task_run, deduct from credit_remaining_usd
- [ ] Dashboard admin panel for adding/updating API keys + credit amounts
  - MUST encrypt key client-side or via Edge Function before hitting Supabase
  - NEVER store plaintext keys in any table
  - Support: "I just added $X credit to Y api" → update credit_remaining_usd
  - Support: "Here's a new API key for X" → encrypt + store in secrets_vault
  - Dashboard chat integration for natural language key/credit updates
- [ ] Low-credit alerts: when credit_remaining_usd < credit_alert_threshold, notify
- [ ] Research current API pricing (input/output per 1M tokens) for new models added

## Pipeline Improvements
- [ ] Module → main merge (merge all testing branches to main when slice complete)
- [ ] Recovery from stale worktrees (clean up VibePilot-work/ automatically)
- [ ] Robust GLM-5 JSON parsing (strip backticks, handle markdown-wrapped JSON)
- [ ] Proper git worktree management (create real git worktrees, not just directories)
- [ ] Visual QA agent (screenshot + Gemini multimodal to verify before human review)

## Architecture
- [ ] Contract Registry (Gemini's suggestion) -- JSON Schema per slice, validated on change
- [ ] Confidence decomposition -- tasks split until 95%+ confidence each
- [ ] n8n-like visual pipeline config (long-term vision)

## Real Projects (Prove the Pipeline)
- [ ] LLM Wiki app (Karpathy-style, first real project)
- [ ] Extra Yum (recipe app)
- [ ] Webs of Wisdom
