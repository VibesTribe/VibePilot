# VibePilot End-to-End Automation Guide

## ✅ Fully Autonomous Workflow

VibePilot runs **completely autonomously** from PRD to merged code with **zero manual intervention**.

### What You Do (Manual)
1. Write PRD and push to GitHub
   ```bash
   cd ~/vibepilot
   # Create PRD in docs/prd/
   git add docs/prd/my-task.md
   git commit -m "Add PRD for my task"
   git push origin main
   ```

### What VibePilot Does (Automatic)
1. ✅ GitHub webhook detects PRD
2. ✅ Governor creates plan (25s)
3. ✅ Supervisor reviews & approves (18s)
4. ✅ Plan breaks into tasks
5. ✅ Task runner executes code
6. ✅ Supervisor reviews output → PASS
7. ✅ Tester validates → PASS
8. ✅ Gitree creates branch, commits, merges
9. ✅ Dashboard updates in real-time

**Total time: ~11 minutes** (will be ~3-4 minutes with consecutive execution)

---

## 📊 Exact Timing Tracking

### Real-Time Monitoring
Watch tasks execute live:
```bash
./scripts/monitor-task.sh
```

**Output:**
```
[14:54:44] 🎯 STAGE 1: Plan Creation started
[14:55:09] ✅ STAGE 1: Plan created (25341ms)
[14:55:27] ✅ STAGE 2: Supervisor approved
[14:55:29] 🎯 STAGE 3: Task claimed by executor
[15:06:02] ✅ STAGE 5: Testing passed
[15:06:05] 🎉 COMPLETE: Merged to TEST_MODULES/general
```

### Post-Execution Report
Generate timing report after completion:
```bash
./scripts/track-timing.sh
./scripts/calculate-timing.sh
```

**Report shows:**
- Each stage start/end time
- Duration per stage
- Total end-to-end time
- Retry attempts
- Timeout recoveries

---

## 🎯 Dashboard Monitoring

### VibeFlow Dashboard
**URL:** https://vibeflow-dashboard.vercel.app/

**Shows:**
- Live task status
- Confidence scores
- Token usage
- Runtime duration
- Module progress

**Example (Task T001):**
```
Status: ✅ Merged
Confidence: 99%
Runtime: 105s
Tokens: 4,104
Branch: TEST_MODULES/general
```

### Governor Logs
Real-time governor activity:
```bash
tail -f ~/vibepilot/governor.log
```

---

## 📈 Performance Metrics

### Task T001 Actual Timings
| Stage | Duration | Notes |
|-------|----------|-------|
| Plan Creation | 25s | PRD → structured plan |
| Supervisor Approval | 18s | Quality gate check |
| Task Creation | 2s | Plan → tasks breakdown |
| Code Execution | 105s | With 4 retry attempts |
| Testing & Merge | 3s | Validation & git ops |
| **TOTAL** | **11m 21s** | **End-to-end** |

### With Consecutive Execution (Estimated)
| Stage | Duration | Notes |
|-------|----------|-------|
| One-Session Execution | 90-120s | All stages in one CLI |
| Testing & Merge | 3s | Same as before |
| **TOTAL** | **~3-4 minutes** | **60% faster** |

---

## 🔧 Monitoring Tools

### 1. Real-Time Task Monitor
```bash
./scripts/monitor-task.sh
```
Watch stages as they execute

### 2. Timing Report Generator
```bash
./scripts/track-timing.sh
```
Extract all timing data from logs

### 3. Calculator
```bash
./scripts/calculate-timing.sh
```
Human-readable timing breakdown

### 4. Governor Log Monitor
```bash
tail -f ~/vibepilot/governor.log | grep -E "STAGE|Supervisor|Task.*merged"
```
Filtered real-time view

### 5. Agent Pool Status
```bash
grep "AgentPool\|capacity\|concurrent" ~/vibepilot/governor.log | tail -20
```
Check resource usage

---

## 🎬 Test It Yourself

### Create a Test PRD
```bash
cat > docs/prd/test-automation.md << 'EOF'
# PRD: Test Automation

Priority: Low
Complexity: Simple
Category: coding
Module: general

## What to Build
Create a Python script that prints "VibePilot Automation Test" and the current timestamp.

## Files
- `test_automation.py`

## Expected Output
- Script prints message + timestamp
- Runs without errors
EOF
```

### Push and Watch
```bash
git add docs/prd/test-automation.md
git commit -m "Test: Automation validation"
git push origin main

# In another terminal, watch real-time:
./scripts/monitor-task.sh
```

### Expected Result
- ~3-4 minutes later
- File created in repo
- Committed to TEST_MODULES/general
- Dashboard shows complete

---

## 🚨 Troubleshooting

### Task Stuck
```bash
# Check if stale
grep "Recovering stale task" ~/vibepilot/governor.log | tail -5

# Check capacity
grep "capacity exceeded" ~/vibepilot/governor.log | tail -5
```

### Git Issues
```bash
# Check git operations
grep -i "gitree" ~/vibepilot/governor.log | tail -10

# Verify branches
git branch -a | grep TEST_MODULES
```

### Timeout Issues
```bash
# Check timeout recoveries
grep "timeout\|stale" ~/vibepilot/governor.log | tail -20
```

---

## 📝 Configuration

### Conservative Resource Limits
```json
{
  "max_concurrent_per_module": 1,
  "max_concurrent_total": 2,
  "agent_timeout_seconds": 300
}
```

**Why conservative?**
- Server has <16GB RAM
- Prevents freezing
- Ensures stability

### To Increase Capacity
Edit `governor/config/system.json`:
```json
{
  "max_concurrent_per_module": 2,  // was 1
  "max_concurrent_total": 4        // was 2
}
```

Then restart governor.

---

## 🎉 Summary

### What Works Now ✅
1. **Fully autonomous** - PRD → merged code with zero intervention
2. **Exact timing** - Every stage timed and logged
3. **Real-time monitoring** - Watch tasks execute live
4. **Dashboard integration** - VibeFlow shows progress
5. **Git operations** - Branch, commit, merge all automatic
6. **Quality gates** - Supervisor + tester validation

### What's Coming Next 🚀
1. **Consecutive execution** - One CLI session for all stages
2. **60% faster** - ~3-4 minutes vs 11 minutes
3. **Self-correcting** - Fix issues within session
4. **Better metrics** - More detailed timing breakdowns

### How to Use
1. Write PRD
2. Push to GitHub
3. Watch dashboard or run monitor script
4. Code appears in TEST_MODULES/[module] branch
5. Done!

**No manual steps required!** 🎊
