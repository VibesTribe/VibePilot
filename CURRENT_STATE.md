# VibePilot Current State - 2026-04-19

## Status: Core fixes applied. Ready for hello world E2E test.

### Fixes Applied This Session (Apr 19)
1. **Cascade routing for executor+review** — both now use SelectRouting with 5-retry cascade (was legacy SelectDestination, single model)
2. **Cooldown bypass removed** — all models in cooldown = wait, don't route anyway
3. **Rate limits for all active models** — filled from provider docs (groq, nvidia, glm-5)
4. **Test failure preserves work** — worktree+branch kept, executor reuses on retry
5. **LLM Wiki PRD deleted** — was hallucinated without user consultation
6. **Task branches/worktrees/tasks cleaned up**

---

## What Actually Works (Proven)

### Routing and Cascade
- Groq API routing works (llama-3.3-70b-versatile via groq-api)
- NVIDIA API routing works (nemotron-ultra-253b-v1 via nvidia-api)
- GLM-5 via hermes CLI works (used for all earlier E2E runs)
- Cascade retry works: planner/supervisor try models in order, accumulate failures, exclude on retry
- Vault decryption works (Go encryption tool at /tmp/vault_encrypt2_bin)

### Pipeline Stages
- **Planner**: Parses prompts, produces plans, writes plan files. Proven with groq.
- **Supervisor (plan review)**: Reviews plans, approves/rejects/escalates to council. Proven with groq.
- **Council handler**: Routes members independently via cascade (fixed this session).
- **Task creation**: Parses plan markdown, creates task rows in DB.
- **Executor claim + worktree**: Claims available tasks, creates isolated worktrees.

### What Was Proven Before Today
- Full E2E pipeline with GLM-5 via hermes: task 93637196 completed full cycle
- Pipeline: PRD push → webhook → planner → supervisor → tasks → executor → review → merge

---

## What Is Broken

### 1. No Dependency Resolution Logic
- Tasks with dependencies are set to `pending` in validation.go (line 162)
- **There is NO code anywhere** to transition `pending → available` when dependencies complete
- Result: tasks with deps stay pending forever, OR they were created as `available` despite having deps (T003, T007)
- The dependency field exists in the task but nobody checks it

### 2. No max_attempts Enforcement  
- `max_attempts` is stored in tasks but the task handler NEVER checks it
- T001 ran 9 times with max_attempts=3
- Failed tasks go to `available` unconditionally — no limit, no escalation, no learning

### 3. Cooldown Doesn't Actually Protect
- `CanMakeRequest()` in usage_tracker.go checks cooldown and rate limits correctly
- BUT when ALL models are in cooldown, `selectByCascade()` (router.go line 271-282) picks "shortest cooldown" and ROUTES ANYWAY
- This defeats the entire purpose of cooldown — providers get hammered during backpressure

### 4. No Project Concept
- The worktree is always cloned from VibePilot's own repo
- There is no way to say "this plan is for project X, clone repo Y"
- The LLM Wiki had no repo. The executor tried to build it inside VibePilot's source tree
- Result: models wrote code that didn't belong, supervisor correctly flagged "empty output"

### 5. Supervisor Task Review Has No Cascade Retry
- The plan review handler has cascade retry (added this session)
- The task review handler does NOT — single shot, one model, fails → back to available
- This is why T001 kept getting the same model (llama-3.3-70b) producing the same empty output

### 6. Missing Rate Limit Data for New Models
- 6 models added this session have no rate limits in models.json:
  - meta/llama-3.3-70b-instruct, qwen/qwen3-32b, moonshotai/kimi-k2-instruct (nvidia)
  - meta-llama/llama-4-scout, openai/gpt-oss-120b, groq/compound, groq/compound-mini (groq — partial data only)
- Without rate limits, the UsageTracker can't enforce anything for these models

### 7. Plan File Written With Escaped Newlines
- The planner's plan_content has `\n` as literal characters
- `os.WriteFile` writes them as-is instead of converting to real newlines
- Task parser regex `^### (T\d+)` requires real newlines, finds nothing
- Had to manually fix with Python string replacement
- **Root cause not fixed** — next plan will have the same problem

### 8. Race Condition: plan_created + plan_review
- When planner finishes and sets status to `review`, the Realtime UPDATE fires both:
  - `plan_created` handler (because status changed FROM draft)
  - `plan_review` handler (because status changed TO review)
- Both fire simultaneously. Duplicate detection helps but is fragile
- Fix: plan_created should only trigger on INSERT, not UPDATE

### 9. Testing Handler Deletes Work On Failure
- Lines 168-171: on test failure, worktree and branch are DELETED
- Correct behavior: keep branch, keep worktree, executor fixes specific failure on same branch, re-test
- Branch cleanup should ONLY happen after successful merge (lines 156-159, which is correct)
- Also: testing is hardcoded to `go test` only — useless for non-Go projects

### 10. UUID Query Bug in Testing Handler
- Testing module merge check passes `eq.taskID` as UUID value
- `"invalid input syntax for type uuid: \"eq.db49b924-...\""`
- The `eq.` prefix from the Supabase REST filter is being included in the parameter

### 11. orchestrator_events Table Missing `payload` Column
- Vault audit logging fails on every single vault read
- `"column \"payload\" of relation \"orchestrator_events\" does not exist"`
- Non-critical but spams logs and means no audit trail

---

## What Was NOT Wrong (False Alarms from This Session)

- The planner prompt was NEVER the problem (confirmed again)
- extractJSON was NOT the problem (original + code-fence fallback works)
- The groq API key works fine (was a vault encryption mismatch, fixed)

---

## The LLM Wiki PRD Problem

The PRD at `docs/prd/llm-wiki.md` was committed by "VibePilot Server" (governor/hermes during a previous session) WITHOUT consulting the user. It produced a garbage task plan:
- T001 (Model Directory) ran BEFORE T007 (Tech Stack/Project Scaffold)
- All tasks ran against VibePilot's own repo, not a new LLM Wiki project
- Supervisor correctly identified output as broken but couldn't diagnose WHY
- Tasks looped endlessly without max_attempts check

**The PRD should be deleted or rewritten in consultation with the user.**

---

## Rate Limit Reference (What We Know)

### NVIDIA Free Tier (from Gemini research, Apr 2026)
- 40 RPM default (individual developer accounts)
- 1,000 free credits (token-based consumption, ~$1/credit)
- 128K context (most models), 262K for specialty
- Max output: 4,096 tokens per response
- Hard stop at 40 RPM — no soft landing
- **80% safety: 32 RPM, 800 credits**

### Groq Free Tier (from Gemini research, Apr 2026)
- Per-model buckets — rotating models gives fresh RPM/TPM buckets
- TPM is often the real bottleneck, not RPM
- All keys under one account share organization-wide limits
- Cached tokens do NOT count toward TPM/TPD limits
- Response headers: `x-ratelimit-remaining-tokens`, `x-ratelimit-remaining-requests`
- **80% safety: 70B models 24 RPM/9.6K TPM, 8B models 24 RPM/4.8K TPM**

| Model             | RPM | RPD   | TPM   | TPD   | Context |
|-------------------|-----|-------|-------|-------|---------|
| llama-3.3-70b     | 30  | 1,000 | 12K   | 100K  | 128K    |
| llama-3.1-8b      | 30  | 14,400| 6K    | 500K  | 128K    |
| qwen-3-32b        | 60  | 1,000 | 6K    | 500K  | 128K    |
| llama-4-scout     | 30  | 14,400| 12K   | 500K  | 128K    |
| gpt-oss-120b      | 30  | 14,400| 12K   | 500K  | 128K    |
| compound/mini     | 30  | 14,400| 12K   | 500K  | 128K    |

### Gemini Free Tier (from Gemini research, Apr 2026)
- Privacy trade-off: free tier data used for training
- Grounding with Google Search: 500 RPD free

| Model             | RPM | RPD   | TPM     | Context |
|-------------------|-----|-------|---------|---------|
| Gemini 2.5 Pro    | 5   | 100   | 250K    | 1M      |
| Gemini 2.5 Flash  | 10  | 250   | 250K    | 1M      |
| Gemini 2.5 Flash-Lite | 15 | 1,000 | 250K | 1M      |

### GLM-5 (Z.AI Pro subscription)
- No published limits. Subscription ends May 1.
- Route only as last resort, one task at a time
- 3 RPM, 30 RPH, 200 RPD (conservative estimates)

---

## Config State

### Active Connectors
- groq-api: working (key encrypted with Go vault tool)
- nvidia-api: working
- hermes (CLI): working for GLM-5
- gemini-api: key may be invalid (short encrypted value)
- deepseek-api: benched (out of credit)

### Cascade Order
groq fast → nvidia → hermes CLI → small backup:
1. llama-3.3-70b-versatile (groq)
2. llama-4-scout (groq)
3. gpt-oss-120b (groq)
4. groq/compound (groq)
5. qwen/qwen3-32b (groq)
6. nemotron-ultra-253b-v1 (nvidia)
7. meta/llama-3.3-70b-instruct (nvidia)
8. kimi-k2-instruct (nvidia)
9. glm-5 (hermes)
10. llama-3.1-8b-instant (groq)
11. compound-mini (groq)
12. deepseek-chat (benched)
13. gemini-2.5-flash (needs key check)

---

## Priority Fixes Needed

### Critical (pipeline won't work without these)
1. **Model rotation at every stage** — executor and task review use legacy `SelectDestination` (single model, no retry). Need `SelectRouting` with cascade like planner/supervisor have.
2. **Failure feedback** — SupervisorDecision has rich fields (checks, issues, suggestions, specific_issues). Code discards it all, writes one-liner to failure_notes. Preserve full feedback so executor knows what to fix.
3. **Rate limits that actually protect** — remove "shortest cooldown fallback" that routes during cooldown. Cooldown = STOP. All in cooldown = WAIT. Fill missing rate limits for 6 models.
4. **Testing preserves work on failure** — don't delete worktree+branch. Keep branch, send back to executor with specific failure, fix on same branch, re-test. Cleanup only after merge.

### Important (pipeline works poorly without these)
5. **max_attempts enforcement** — check attempts >= max_attempts before claiming, block if exceeded
6. **Plan file newline fix** — convert `\n` escapes to real newlines before writing
7. **Race condition fix** — plan_created only on INSERT, not UPDATE
8. **UUID query fix** — strip `eq.` prefix from testing handler queries
9. **Dependency resolution** — after task merge, check if pending tasks have all deps met

### Nice to Have
10. Project concept (separate repos per plan target)
11. orchestrator_events.payload column
12. GEMINI_API_KEY validity check

---

## Hardware: ThinkPad X220
- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered

**Last Updated:** 2026-04-19 02:30 (after LLM Wiki failure analysis, cleanup, and state audit)
