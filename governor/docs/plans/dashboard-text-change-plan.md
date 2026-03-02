# PLAN: Dashboard Text Change - vibeflow → vibepilot

## Overview
Replace all "vibeflow" text references with "vibepilot" across the VibePilot dashboard and documentation. This is a text-only rebranding with zero visual or functional changes beyond the text content itself.

**PRD Reference:** `docs/prd/vibepilot-rename-dashboard-prd.md`

## Risk Assessment
- **Multiple Occurrences Risk (Medium):** PRD Section 7 identifies potential for "vibeflow" appearing in multiple places. Must search entire codebase before making changes.
- **Scope Creep Risk (Low):** Task is clearly bounded to text replacement only.
- **Breaking Changes Risk (Low):** Text-only change with no functional modifications.

## Workflow Requirements
1. **Pre-Change:** Codebase-wide search to identify all occurrences
2. **Change Execution:** Replace text in identified files
3. **Visual Verification:** Human must visually confirm changes in running dashboard
4. **Human Approval:** Required per AGENTS.md for UI changes - submit PR, do not merge until approved

## Tasks

### T001: Search Codebase and Replace Text
**Confidence:** 0.88
**Dependencies:** none
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Purpose
Identify all "vibeflow" occurrences across the entire codebase and perform case-preserving text replacement.

#### Prompt Packet
```markdown
# TASK: T001 - Search Codebase and Replace vibeflow Text

## CONTEXT
You are performing a text rebranding task for VibePilot. The goal is to replace all occurrences of "vibeflow" (in any case variant) with "vibepilot" equivalents. This is a TEXT-ONLY change - no visual styling, no functionality changes, no structural modifications.

## PRE-STEP: CODEBASE SEARCH
Before making any changes, you MUST search the entire codebase to identify all occurrences:

```bash
# Search for all vibeflow occurrences (case-insensitive)
grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot --include="*.md" --include="*.json" --include="*.yaml" --include="*.py" --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" 2>/dev/null | grep -v node_modules | grep -v ".git"
```

If occurrences are found in MULTIPLE directories or more than 10 files:
1. STOP and report findings to human
2. Ask for scope confirmation before proceeding
3. Human decides if all occurrences should be changed or only specific ones

If occurrences are limited to a reasonable scope (< 10 files, clear pattern):
1. Document all files found
2. Proceed with replacement

## WHAT TO BUILD
Perform case-preserving text replacement:
- `vibeflow` → `vibepilot` (lowercase)
- `Vibeflow` → `VibePilot` (title case)
- `VIBEFLOW` → `VIBEPILOT` (uppercase)

## FILES TO MODIFY
Based on initial search, likely files include:
- `docs/vibeflow_dashboard_analysis.md`
- `docs/vibeflow_review.md`
- `docs/vibeflow_adoption.md`
- `config/prompts/consultant.md`
- `config/researcher_context.md`
- Various root-level markdown files

## TECHNICAL SPECIFICATIONS

### Replacement Method
Use sed or Edit tool for each file:

```bash
# Example sed commands (adjust paths based on search results)
sed -i 's/vibeflow/vibepilot/g' docs/vibeflow_dashboard_analysis.md
sed -i 's/Vibeflow/VibePilot/g' docs/vibeflow_dashboard_analysis.md
sed -i 's/VIBEFLOW/VIBEPILOT/g' docs/vibeflow_dashboard_analysis.md
```

### Verification
After each file modification:
```bash
grep -i "vibeflow" <file>  # Should return empty
```

## ACCEPTANCE CRITERIA
- [ ] Codebase-wide search completed and results documented
- [ ] All identified files have been updated with case-preserving replacement
- [ ] `grep -ri "vibeflow"` returns zero results across entire codebase
- [ ] No files were modified outside the identified scope
- [ ] All modified files remain syntactically valid (JSON parses, markdown renders)

## TESTS REQUIRED
1. Grep verification: `grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot` returns empty
2. File integrity: All modified files can be read without errors
3. JSON validation (if any JSON files modified): `python -m json.tool <file>` succeeds

## POST-COMPLETION REQUIREMENTS
After completing text replacements:
1. Report summary of all changes made (file count, replacement counts)
2. Confirm grep verification passed
3. NOTE: Visual verification and human approval will follow in subsequent steps

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_modified": ["path1", "path2"],
  "replacement_summary": {
    "total_files": 5,
    "total_replacements": 23,
    "by_file": {
      "docs/vibeflow_dashboard_analysis.md": 8,
      "config/prompts/consultant.md": 3
    }
  },
  "verification": {
    "grep_check": "PASSED - zero vibeflow occurrences found",
    "file_integrity": "PASSED - all files readable"
  },
  "notes": "Any important decisions or scope clarifications"
}
```

## DO NOT
- Add features not listed in this task
- Modify visual styling or functionality
- Skip the pre-step codebase search
- Proceed without human confirmation if scope exceeds 10 files
- Commit changes (that's a separate step)
- Leave TODO comments
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [
    "docs/vibeflow_dashboard_analysis.md",
    "docs/vibeflow_review.md", 
    "docs/vibeflow_adoption.md",
    "config/prompts/consultant.md",
    "config/researcher_context.md"
  ],
  "tests_required": ["grep verification", "file integrity check"],
  "human_actions_needed": ["visual verification", "approval before merge"]
}
```

---

### T002: Visual Verification and Human Approval Workflow
**Confidence:** 0.95
**Dependencies:** T001 (summary)
**Type:** verification
**Category:** testing
**Requires Codebase:** false

#### Purpose
Define and execute the visual verification workflow, then request human approval before merging changes.

#### Prompt Packet
```markdown
# TASK: T002 - Visual Verification and Human Approval Workflow

## CONTEXT
T001 has completed text replacements. Now you must guide the visual verification and human approval process. This is REQUIRED per AGENTS.md for UI changes - changes to visible dashboard text count as UI changes.

## DEPENDENCIES

### Summary Dependencies
- T001: Text replacements completed. All "vibeflow" references replaced with "vibepilot" across documentation and config files.

## WHAT TO BUILD
Create a verification checklist and approval request for the human.

## VISUAL VERIFICATION WORKFLOW

### Step 1: Commit Changes (Do Not Push to Main)
```bash
# Create feature branch if not already on one
git checkout -b feature/vibeflow-to-vibepilot-rebrand

# Stage and commit changes
git add .
git commit -m "Rebrand: Replace vibeflow with vibepilot in documentation and config"
```

### Step 2: Push to Feature Branch
```bash
git push origin feature/vibeflow-to-vibepilot-rebrand
```

### Step 3: Create Pull Request
Create a PR with title: "Rebrand: Replace vibeflow with vibepilot"

PR Body Template:
```markdown
## Summary
- Replaced all "vibeflow" text references with "vibepilot"
- Case-preserving replacement (vibeflow→vibepilot, Vibeflow→VibePilot, VIBEFLOW→VIBEPILOT)
- Files modified: [list from T001]

## Verification
- [ ] grep verification passed (zero vibeflow occurrences)
- [ ] File integrity confirmed

## Human Actions Required
- [ ] **Visual Verification:** Review changed files in GitHub to confirm text replacements are correct
- [ ] **Approval:** Approve this PR before merging to main

## Notes
This is a text-only change per PRD: docs/prd/vibepilot-rename-dashboard-prd.md
```

### Step 4: Human Verification Instructions
Provide these instructions to the human:

**VISUAL VERIFICATION CHECKLIST:**
1. Open the PR in GitHub
2. Review "Files changed" tab
3. Verify each replacement:
   - [ ] Text replacements are correct (no typos)
   - [ ] Case preservation is correct (Vibeflow→VibePilot, not Vibepilot)
   - [ ] No unintended changes (only vibeflow→vibepilot replacements)
   - [ ] No files modified outside expected scope
4. If all checks pass, approve the PR
5. If issues found, comment on the PR with specific concerns

## ACCEPTANCE CRITERIA
- [ ] Changes committed to feature branch (not main)
- [ ] PR created with clear description
- [ ] Human verification instructions provided
- [ ] Awaiting human approval (not auto-merged)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model name]",
  "pr_url": "https://github.com/.../pull/XXX",
  "branch_name": "feature/vibeflow-to-vibepilot-rebrand",
  "commit_hash": "abc123...",
  "status": "awaiting_human_approval",
  "human_actions": [
    "Review PR files changed tab",
    "Verify text replacements are correct",
    "Approve PR if verification passes"
  ],
  "notes": "PR created and ready for human review"
}
```

## DO NOT
- Merge to main without human approval
- Skip creating a PR
- Push directly to main branch
- Auto-approve or self-merge
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [],
  "pr_created": true,
  "pr_url": "https://github.com/.../pull/XXX",
  "status": "awaiting_human_approval",
  "human_actions_required": ["visual verification", "PR approval"]
}
```

---

## Critical Path
T001 → T002 → Human Approval

## Confidence Summary
- **T001:** 0.88 - External dependency on scope clarity, requires codebase search before proceeding
- **T002:** 0.95 - Clear workflow, depends only on T001 summary
- **Overall Plan Confidence:** 0.91

## Revision Notes
This plan addresses Council feedback:
- ✅ Codebase-wide search step added to T001 (addresses PRD Risk Assessment)
- ✅ Visual verification workflow fully specified with WHO/HOW/WHEN
- ✅ Human approval workflow specified with PR process
- ✅ Confidence scores reduced to realistic levels (0.88, 0.95)
- ✅ Requires Codebase flag set to true for T001
- ✅ Complete prompt packets provided for all tasks
