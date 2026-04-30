# VibePilot Operational Handbook

Disaster recovery, monitoring, and operational procedures.

---

## Monitoring & Alerting

### Key Metrics to Track

| Metric | Threshold | Alert |
|--------|-----------|-------|
| Task failure rate | > 10% | Warning |
| Task failure rate | > 30% | Critical |
| Average task duration | > 5 min | Warning |
| API response time | > 5s | Warning |
| Token usage per day | > 1M | Review cost |
| Supabase connection errors | > 0 | Critical |
| Agent timeout rate | > 5% | Warning |

### Logging Standards

Every log entry must include:
- Timestamp (ISO 8601)
- Agent/model name
- Task ID (if applicable)
- Log level (DEBUG, INFO, WARN, ERROR)
- Context (what was happening)
- Error details (if applicable)

Example:
```
2026-02-14T15:30:00Z [glm-5] [task-123] [ERROR] 
Context: Executing search debouncing implementation
Error: Timeout after 30s waiting for Kimi response
Action: Retrying with exponential backoff (attempt 2/3)
```

---

## Disaster Recovery

### Scenario 1: Supabase Outage

**Symptoms:**
- Connection errors to Supabase
- Tasks stuck in 'executing' state
- No state updates possible

**Recovery:**
1. Check Supabase status page
2. If confirmed outage, switch to degraded mode:
   - Queue operations locally in `migration/local_queue.json`
   - Continue with read-only operations
   - Log all would-be updates
3. When Supabase restores:
   - Replay queued operations
   - Verify no data loss
   - Update SESSION_LOG with incident

**Prevention:**
- Use Supabase connection pooling
- Implement circuit breaker pattern
- Have local queue ready for emergencies

### Scenario 2: GitHub Outage

**Symptoms:**
- Cannot push/pull code
- CI/CD blocked

**Recovery:**
1. Continue local development
2. Commit locally with descriptive messages
3. When GitHub restores:
   - `git push origin main`
   - Verify all commits synced
   - Update SESSION_LOG

**Prevention:**
- Local git history is sufficient short-term
- Critical: ensure .git directory is not lost

### Scenario 3: Primary Model (GLM-5) Down

**Symptoms:**
- Timeout on GLM-5 requests
- Tasks stuck in 'planning' state

**Recovery:**
1. Orchestrator auto-routes to DeepSeek (backup)
2. Log degradation in `task_runs`
3. Alert human
4. When GLM-5 restores, no action needed (orchestrator routes back)

**Prevention:**
- Dual orchestrator with fallback logic
- Model health checks before routing

### Scenario 4: Agent Coordination Failure

**Symptoms:**
- Two agents modified same file
- Task state inconsistent
- Conflicting handoffs

**Recovery:**
1. Identify conflict source from logs
2. Lock affected tasks (status = 'blocked')
3. Human reviews and resolves conflict
4. Update task records with correct state
5. Resume from resolved state

**Prevention:**
- File hash checks before writes
- Atomic Supabase updates
- Clear handoff protocol

### Scenario 5: Complete Data Loss (Worst Case)

**Symptoms:**
- Server destroyed, no backup
- Supabase data corrupted/deleted

**Recovery:**
1. Provision new server
2. `git clone` VibePilot from GitHub
3. Restore Supabase from backup
4. Configure .env from secure storage
5. Run `./setup.sh`
6. Verify functionality
7. Resume from SESSION_LOG

**Prevention:**
- Daily Supabase backups
- GitHub as code source of truth
- .env backed up in secure location (NOT in git)
- SESSION_LOG maintained after every session

---

## Secret Management

### Storage
- **Never** commit secrets to git
- Use `.env` locally (in .gitignore)
- Use environment variables in production
- Use Supabase vault for API keys if needed

### Rotation Schedule

| Secret | Rotation Frequency | Process |
|--------|-------------------|---------|
| Supabase credentials | Quarterly | Regenerate in dashboard, update .env |
| API keys (OpenAI, etc.) | Bi-annually | Regenerate in provider dashboard |
| SSH keys | Annually | Generate new key, update authorized_keys |

### Rotation Procedure

1. Generate new secret in provider dashboard
2. Update `.env` locally
3. Test with new secret
4. Update all environments
5. Revoke old secret
6. Document rotation in SESSION_LOG

---

## Backup Strategy

### What to Backup

| Asset | Frequency | Location |
|-------|-----------|----------|
| Supabase data | Daily | Supabase managed + manual exports |
| GitHub repo | Continuous | GitHub + local clone |
| .env file | On change | Secure location (NOT GitHub) |
| SESSION_LOG.md | After each session | Git committed |

### Backup Commands

```bash
# Supabase backup
supabase db dump -f backups/supabase_$(date +%Y%m%d).sql

# Full project backup (excluding venv, logs)
tar --exclude='venv' --exclude='*.log' --exclude='__pycache__' \
    -czf backups/vibepilot_$(date +%Y%m%d).tar.gz .

# Encrypted .env backup
gpg --symmetric --cipher-algo AES256 .env -o backups/env_$(date +%Y%m%d).gpg
```

### Restore Procedure

```bash
# Restore Supabase
supabase db reset
psql -f backups/supabase_YYYYMMDD.sql

# Restore project
tar -xzf backups/vibepilot_YYYYMMDD.tar.gz

# Restore .env
gpg -d backups/env_YYYYMMDD.gpg > .env
```

---

## API Version Management

### Problem
External APIs (OpenAI, Anthropic, Google) deprecate versions. Code breaks silently.

### Solution

1. **Pin API versions in code:**
   ```python
   # Don't: openai.ChatCompletion.create(...)
   # Do: openai.chat.completions.create(
   #       model="gpt-4-0613",  # Pinned version
   #       ...
   #     )
   ```

2. **Monitor deprecation notices:**
   - Subscribe to provider changelogs
   - Check API version headers in responses

3. **Test before upgrade:**
   - New API version = new feature branch
   - Full test suite before merge
   - Update DECISION_LOG with upgrade rationale

---

## Performance Tuning

### Database Optimization

```sql
-- Check slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Add indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_status 
ON tasks(status) WHERE status NOT IN ('complete', 'cancelled');

CREATE INDEX IF NOT EXISTS idx_task_packets_to_agent 
ON task_packets(to_agent, created_at DESC);
```

### Connection Pooling

```python
# In database connection
from sqlalchemy import create_engine
engine = create_engine(
    DATABASE_URL,
    pool_size=10,
    max_overflow=20,
    pool_pre_ping=True
)
```

---

## Operational Runbook

### Daily Tasks
- [ ] Check error logs
- [ ] Review task failure rate
- [ ] Verify backups completed

### Weekly Tasks
- [ ] Review token usage trends
- [ ] Check for API deprecation notices
- [ ] Test restore procedure (sample)

### Monthly Tasks
- [ ] Rotate secrets if due
- [ ] Review and archive old tasks
- [ ] Update dependencies (with testing)
- [ ] Cost optimization review

---

## Incident Response

### Severity Levels

| Level | Description | Response Time |
|-------|-------------|---------------|
| P0 | System down, no workaround | 15 min |
| P1 | Major feature broken | 1 hour |
| P2 | Feature degraded | 4 hours |
| P3 | Minor issue, workaround exists | 24 hours |

### Incident Log Template

```markdown
## Incident: [Date] - [Short Description]

**Severity:** P0/P1/P2/P3
**Duration:** [start time] to [end time]
**Impact:** [what was affected]

### Timeline
- [time] Issue detected
- [time] Investigation started
- [time] Root cause identified
- [time] Fix implemented
- [time] Verified resolved

### Root Cause
[Why did this happen?]

### Resolution
[What fixed it?]

### Prevention
[How do we prevent recurrence?]

### Action Items
- [ ] Item 1
- [ ] Item 2
```

---

*Last updated: 2026-02-14*
*Review this document monthly.*
