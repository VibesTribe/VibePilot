# VibePilot Video Insights - Session 2026-02-14

**Source:** Gemini analysis of 3 videos
**Date:** 2026-02-14

---

# 1. Senior Engineer Database Rules

**Video:** Database design best practices
**Application:** VibePilot Supabase schema

## Rules

| Rule | What | Why |
|------|------|-----|
| Naming | `lowercase_plural` tables, `snake_case` columns | Standard SQL, portable |
| Timestamps | Every table: `created_at`, `updated_at` | Auditing, debugging, bot detection |
| Primary Keys | UUID/string `id`, never business logic (email) | IDs stable even if business changes |
| Join Tables | Many-to-many uses junction table | Can't link directly |

## Relationship Patterns

| Type | Pattern | Example |
|------|---------|---------|
| 1:1 | FK + UNIQUE | `users` ↔ `user_preferences` |
| 1:N | FK on many side | `portfolios` → `holdings` |
| N:N | Junction table | `users` ↔ `stocks` via `watchlist_items` |

## Checklist for Schema

- [ ] All tables have `id` (UUID/string) as primary key
- [ ] All tables have `created_at`, `updated_at`
- [ ] All FKs marked and indexed
- [ ] 1:1 relationships have UNIQUE constraint on FK
- [ ] N:N relationships use junction tables
- [ ] All names lowercase_snake_case
- [ ] No business logic in primary keys

## Why This Matters for VibePilot

- **Swappability:** Standard SQL = easy to move from Supabase to any Postgres
- **Auditability:** Timestamps on everything = debugging easier
- **Stability:** UUID IDs = nothing breaks when business logic changes

---

# 2. Noiseless AI Memory Protocol

**Video:** Manolo Remiddi
**Application:** Token cost reduction, context management

## Key Concepts

| Concept | What | Savings |
|---------|------|---------|
| Noiseless Compression | Compress chat to "shorthand" functions/signals | 80% token reduction |
| Memory Anchoring | Hash-tagged blocks, fetch raw on demand | Keep compressed, expand when needed |
| Single Source of Truth (SSOT) | Document explaining system architecture to AI | Prevents assumptions, bugs |
| Symbiotic Shield | Virus/injection check layer | Security for generated code |

## Application to VibePilot

### Current Problem
- `SESSION_LOG.md` grows unbounded
- Each session re-reads same context
- 400k+ tokens to maintain "vibe"

### Solution: Noiseless Compression

**Before (Verbose):**
```
On 2026-02-13, we discussed the role system. We decided each role 
should have 2-3 skills maximum. This prevents drift and hallucination.
GLM-5 will be the primary coder. Kimi will be parallel executor.
```

**After (Noiseless):**
```
DEC-003: roles.bounded_skills | 2026-02-13 | max 3 skills/role
MODEL: glm-5 → primary | kimi → parallel | gemini → research
```

### Memory Anchoring

```yaml
# Compressed block
memory_block:
  id: "mem_001"
  hash: "a3f8b2"
  compressed: "DEC-001..DEC-005: architecture decisions"
  uncompressed_location: ".context/DECISION_LOG.md#L1-L200"
  
# AI sees compressed, can "fetch" full if needed
```

### SSOT Injection

At session start, inject document explaining VibePilot to itself:

```markdown
# VibePilot Self-Awareness

You are VibePilot's execution engine. Here is your architecture:

[Core architecture explanation]

Current state: [link to CURRENT_STATE.md]

What you MUST preserve:
- Context isolation (task agents see only their task)
- Config-driven swaps (no code edits for changes)
- Council process (iterative for PRDs, one-shot for updates)

What you MUST NOT do:
- Delete core files without human approval
- Modify schema without Council review
- Add "helpful" features not in PRD
```

---

# 3. Navigation-Based Context (vs RAG)

**Video:** Adam Lucek
**Application:** Context management without vector DB fuzziness

## Key Concepts

| Concept | What | Benefit |
|---------|------|---------|
| Context Navigation | Model gets terminal tools (`ls`, `cat`, `grep`) | Explores like human dev |
| Recursive Calls | Sub-LLM in fresh context for sub-tasks | Main context stays clean |
| Prompt Caching | Keep 1M+ tokens cached, pay only for slices | Massive cost savings |

## Application to VibePilot

### Current Approach (Problem)
- Feed entire codebase to model
- Vector search for relevant chunks
- Fuzzy, loses precision

### New Approach: Navigation

```python
# Instead of feeding all files, give tools
agent_tools = [
    "ls",      # List directory structure
    "cat",     # Read specific file
    "grep",    # Search for pattern
    "find",    # Find files by name
]

# Model explores like a human:
# 1. ls ~/vibepilot/
# 2. ls core/
# 3. cat core/roles.py
# 4. grep "class.*Agent" agents/
```

### Recursive Sub-LLM

```python
# Main context: ~10k tokens
# Sub-task needs 100k tokens? Spawn fresh context

result = spawn_sub_llm(
    task="Extract all DECISION_LOG entries about caching",
    context=get_file(".context/DECISION_LOG.md"),
    return_only="summary"  # Not raw output
)

# Main context receives: ~500 token summary
# Sub-LLM handled the 100k context separately
```

---

# 4. Awareness Agents

**From both videos**

## Concept

Tiny watcher agent that auto-injects context based on keywords.

## How It Works

```
Human mentions: "Check the GitHub workflow"
Awareness Agent detects: "GitHub" keyword
Agent auto-injects:
  - GitHub-related memory blocks
  - Current workflow files
  - Recent git commits
```

## Application to VibePilot

| Keyword | Auto-Inject |
|---------|-------------|
| "schema" | `.context/DECISION_LOG.md` (schema decisions), `docs/schema_*.sql` |
| "model" | `config/vibepilot.yaml` (models section), Supabase `models` table |
| "council" | `.context/guardrails.md` (Council process), `docs/MASTER_PLAN.md` (Council section) |
| "rollback" | `CHANGELOG.md` (recent changes), known good commits |

---

# Strategic Synthesis for Next Session

## Priority 1: Schema Audit (Senior Rules)

Check existing schema against senior engineer rules:
- [ ] All tables have `id`, `created_at`, `updated_at`
- [ ] All FKs indexed and marked
- [ ] 1:1 relationships have UNIQUE
- [ ] N:N use junction tables
- [ ] snake_case everywhere

## Priority 2: Self-Awareness Document (SSOT)

Create document explaining VibePilot to itself:
- What VibePilot is
- Current architecture
- What to preserve
- What to never do

## Priority 3: Noiseless Compression

Convert verbose docs to compressed signals:
- `SESSION_LOG.md` → noiseless format
- Memory anchoring with hash links
- Fetch-on-demand for raw detail

## Priority 4: Navigation Tools (Future)

Give agents terminal tools instead of feeding all context:
- `ls`, `cat`, `grep`, `find`
- Explore only what's needed
- Recursive sub-LLM for large tasks

---

# Strategic Synthesis - VibePilot Specific

## What We're Keeping

| Item | Decision | Why |
|------|----------|-----|
| Senior Schema Rules | DEC-011 (Pending) | Standard practices, immediate value |
| Prompt Caching | DEC-007 (Pending) | 75% cost savings, high impact |

## What We Rejected (And Why)

| Item | Rejected Because |
|------|------------------|
| Separate SSOT Document | Duplicates CURRENT_STATE.md - one source of truth is enough |
| Noiseless Compression | Loses WHY (reasoning), not just WHAT - summary/detail pattern already exists |
| Navigation Tools | Security risk, complexity - directory index in CURRENT_STATE.md solves navigation |
| Awareness Agent | Heuristic risk, over-engineering - explicit structure is clearer |

## Simpler Solution Applied

Added to `CURRENT_STATE.md`:
- **WHAT AI MUST PRESERVE** - Non-negotiables
- **WHAT AI MUST NEVER DO** - Causes of system instability

One file. No new documents. No complexity.

---

# Pending Decisions

| ID | Topic | Status |
|----|-------|--------|
| DEC-011 | Schema Senior Rules Audit | Proposed |
| DEC-012 | Self-Awareness SSOT Document | Proposed |
| DEC-013 | Noiseless Compression Protocol | Proposed |
| DEC-014 | Navigation-Based Context | Proposed |
| DEC-015 | Awareness Agent | Proposed |

---

*Captured: 2026-02-14*
*Source: Gemini analysis of senior engineer + memory management videos*
