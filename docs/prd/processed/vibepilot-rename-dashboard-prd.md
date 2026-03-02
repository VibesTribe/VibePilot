# PRD: VibePilot Dashboard Rebrand — "vibeflow" → "vibepilot"

**Document Version:** 1.0  
**Author:** The Consultant (Spec Engineer)  
**Date:** 2026-02-21  
**Status:** READY FOR IMPLEMENTATION  
**Classification:** Text-Only Change (Zero Visual Delta)  

---

## 1. OVERVIEW

### 1.1 Product Definition
VibePilot is an AI-native task orchestration platform with a real-time terminal dashboard. This PRD governs the complete text replacement of the legacy product name "vibeflow" with "vibepilot" across all dashboard touchpoints—**with zero visual or functional changes**.

### 1.2 Scope Boundary
```
IN SCOPE:
├── dashboard/terminal_dashboard.py (header text verification)
├── Documentation files (*.md) containing "vibeflow"
├── Component headers referencing legacy name
└── Log/output strings displaying branding

OUT OF SCOPE:
├── Color schemes, typography, spacing
├── Layout modifications
├── Feature additions or removals
├── Database schema changes
└── API endpoint modifications
```

### 1.3 Change Classification
| Attribute | Value |
|-----------|-------|
| Change Type | Text replacement |
| Risk Level | Minimal |
| Rollback Complexity | Trivial (git revert) |
| QA Requirement | Visual diff + string search validation |

---

## 2. OBJECTIVES

### 2.1 Primary Goal
Achieve 100% brand consistency by replacing every instance of "vibeflow" with "vibepilot" in dashboard-facing text while maintaining **pixel-perfect visual parity**.

### 2.2 Success Criteria (Quantified)
| Metric | Target |
|--------|--------|
| Text occurrences replaced | 100% |
| Visual regression | Zero delta |
| Build status | Pass |
| Test suite | 100% pass |
| grep -r "vibeflow" dashboard/ | Returns empty |

### 2.3 Non-Goals
- No new features
- No bug fixes
- No performance optimization
- No dependency updates
- No architectural changes

---

## 3. TECHNICAL STACK

### 3.1 Current Implementation
| Layer | Technology | Version |
|-------|------------|---------|
| Dashboard Runtime | Python | 3.11+ |
| Terminal UI | ANSI escape codes | Native |
| Data Layer | Supabase Client | 2.x |
| Config Management | python-dotenv | 1.0+ |
| Testing | pytest | 7.x |

### 3.2 Files Requiring Modification
```
dashboard/
└── terminal_dashboard.py        # Header banner text (line 110)

docs/
├── *.md                         # 14 files containing "vibeflow"
├── research/*.md                # Research docs
└── prd/                         # Legacy PRD references

config/
└── prompts/consultant.md        # Brand references

plans/
└── *.json                       # Plan metadata
```

### 3.3 Tools Required
- `ripgrep` (rg) — fast string searching
- `sed` / Python script — batch replacement
- `git diff` — change verification
- `pytest` — regression testing

---

## 4. FEATURES

### 4.1 Feature Matrix

| ID | Feature | Description | Priority | Effort |
|----|---------|-------------|----------|--------|
| F1 | Header Rebrand | Change "VIBEFLOW" → "VIBEPILOT" in terminal_dashboard.py | P0 | 5 min |
| F2 | Documentation Sync | Update all .md files referencing legacy name | P0 | 15 min |
| F3 | Config Cleanup | Update prompt templates and configs | P1 | 10 min |
| F4 | Plan Metadata | Update JSON plan files brand references | P2 | 5 min |
| F5 | Validation Suite | Automated grep check for residual occurrences | P0 | 10 min |

### 4.2 P0 (Critical Path) Detail

#### F1: Dashboard Header Rebrand
```python
# BEFORE (line 110 terminal_dashboard.py):
print(f"  VIBEFLOW DASHBOARD  |  {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")

# AFTER:
print(f"  VIBEPILOT DASHBOARD  |  {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
```

**Validation:**
```bash
$ python dashboard/terminal_dashboard.py | head -5
======================================================================
  VIBEPILOT DASHBOARD  |  2026-02-21 14:32:01
======================================================================
```

#### F5: Validation Suite
```bash
#!/bin/bash
# validate_no_vibeflow.sh

COUNT=$(grep -ri "vibeflow" dashboard/ docs/ config/ 2>/dev/null | wc -l)
if [ "$COUNT" -eq 0 ]; then
    echo "✓ Validation passed: No 'vibeflow' occurrences found"
    exit 0
else
    echo "✗ Validation failed: $COUNT occurrences remain"
    grep -ri "vibeflow" dashboard/ docs/ config/
    exit 1
fi
```

---

## 5. ARCHITECTURE

### 5.1 Current System Context
```
┌─────────────────────────────────────────────────────────────┐
│                     VibePilot System                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │  Dashboard  │◄───│  Supabase    │    │  Task Queue  │   │
│  │  (Python)   │    │  (Postgres)  │    │  (Redis)     │   │
│  └─────────────┘    └──────────────┘    └──────────────┘   │
│         │                                                   │
│         ▼                                                   │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Terminal Output (ANSI)                  │    │
│  │  ┌───────────────────────────────────────────────┐  │    │
│  │  │  VIBEPILOT DASHBOARD  |  2026-02-21 14:32:01  │  │    │
│  │  └───────────────────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Change Impact Analysis
| Component | Impact | Reason |
|-----------|--------|--------|
| terminal_dashboard.py | Low | Single string literal change |
| Documentation | Low | Content-only, no structure change |
| Config files | Low | String replacement |
| Tests | None | No test logic references brand name |
| Database | None | No schema or data migration required |
| API | None | No endpoint changes |

### 5.3 Data Flow (Unchanged)
```
User → dashboard.py → Supabase → Render → Terminal
                    ↑
         (Read-only ops, no writes)
```

---

## 6. SECURITY REQUIREMENTS

### 6.1 Risk Assessment
| Vector | Risk | Mitigation |
|--------|------|------------|
| Code Injection | None | No user input processing |
| Data Exposure | None | No credential changes |
| Audit Trail | Low | Git commit maintains history |

### 6.2 Compliance
- [x] No PII handling changes
- [x] No authentication flow modifications
- [x] No authorization rule changes
- [x] No encryption requirement changes

### 6.3 Review Requirements
- [ ] Code review (single reviewer sufficient)
- [ ] No security team review required (text-only)

---

## 7. DEPLOYMENT STRATEGY

### 7.1 Environment Strategy
```
Local Dev → Feature Branch → PR Review → Merge to Main
     │              │              │            │
     ▼              ▼              ▼            ▼
  Verify       Run tests       Visual      Deploy
  changes      (pytest)        check       (git)
```

### 7.2 Branch Configuration
```bash
# Create feature branch per AGENTS.md
git checkout -b feature/vibeflow-to-vibepilot-rename
git push origin feature/vibeflow-to-vibepilot-rename

# After human approval
git checkout main
git merge feature/vibeflow-to-vibepilot-rename
```

### 7.3 CI/CD Pipeline
| Stage | Command | Pass Criteria |
|-------|---------|---------------|
| Lint | `ruff check dashboard/` | No errors |
| Test | `pytest tests/ -v` | 100% pass |
| Validate | `./validate_no_vibeflow.sh` | Exit 0 |

### 7.4 Rollback Plan
```bash
# Emergency rollback (30 seconds)
git revert HEAD --no-edit
git push origin main
```

### 7.5 Monitoring
- Post-deploy: Run dashboard, visually verify header
- Check: `python dashboard/terminal_dashboard.py | grep -i vibepilot`

---

## 8. SUCCESS METRICS

### 8.1 Quantitative KPIs
| Metric | Baseline | Target | Measurement |
|--------|----------|--------|-------------|
| Brand consistency | Partial | 100% | `grep -r "vibeflow" . \| wc -l` = 0 |
| Build time | ~5s | <10s | CI pipeline duration |
| Test pass rate | 100% | 100% | pytest exit code |
| Files modified | — | 15+ | git diff --stat |
| Lines changed | — | 20-50 | git diff --shortstat |

### 8.2 Qualitative Indicators
- [ ] Human visual confirmation of dashboard header
- [ ] No stakeholder complaints about broken references
- [ ] Documentation reads consistently with "VibePilot"

### 8.3 Definition of Done
```
□ All P0 features implemented
□ Validation script passes (zero "vibeflow" occurrences)
□ Code review approved
□ Human tested and approved
□ Merged to main
□ Deployed to production environment
□ No rollback required within 24 hours
```

---

## 9. APPENDIX

### 9.1 Search Results Summary
| Path | Occurrences | Action |
|------|-------------|--------|
| dashboard/terminal_dashboard.py | 0 (already done) | Verify only |
| docs/*.md | 14 | Replace |
| config/prompts/consultant.md | 1 | Replace |
| plans/*.json | 2 | Replace |

### 9.2 Exact Replacements Required
```python
# dashboard/terminal_dashboard.py (line 110)
# ALREADY CORRECT - verify only

# docs/prd_v1.3.md
# Replace: vibeflow → vibepilot (context: product references)

# docs/vibeflow_dashboard_analysis.md  
# File rename to: docs/vibepilot_dashboard_analysis.md
# Content: vibeflow → vibepilot

# docs/vibeflow_review.md
# File rename to: docs/vibepilot_review.md
# Content: vibeflow → vibepilot

# config/prompts/consultant.md
# Replace: vibeflow → vibepilot
```

### 9.3 Change Log
| Date | Author | Change |
|------|--------|--------|
| 2026-02-21 | The Consultant | Initial PRD creation |

---

**END OF DOCUMENT**
