# PLAN: Dashboard Text Change - vibeflow to vibepilot

## Overview
Change the displayed text from "vibeflow" to "vibepilot" in the MissionHeader component of the VibePilot dashboard. This plan addresses identified risks through a phased approach: scope discovery, controlled execution, and human-gated deployment.

## Project Context
- **Project:** VibePilot Dashboard
- **Type:** Text Change (Foundation Test)
- **Priority:** P1
- **PRD:** docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md

## Risk Mitigation Strategy
The PRD identifies a Medium-likelihood risk that "vibeflow" may appear in multiple locations. This plan uses a phased approach:
1. **Discovery Phase**: Search entire codebase to identify all occurrences
2. **Decision Gate**: Human reviews findings and approves scope
3. **Execution Phase**: Make approved changes only
4. **Verification Phase**: Human visual verification before commit
5. **Approval Gate**: Human approval required before merge (per AGENTS.md)

## Tasks

### T001: Search Codebase for All "vibeflow" Occurrences
**Confidence:** 0.98
**Dependencies:** none
**Type:** research
**Category:** research
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T001 - Search Codebase for "vibeflow" Occurrences

## CONTEXT
Before making text changes, we need to identify ALL locations where "vibeflow" appears in the codebase. The PRD identifies this as a Medium-likelihood risk. This discovery task will inform the scope of changes needed.

## DEPENDENCIES
None - this is a standalone research task.

## WHAT TO BUILD
Perform a comprehensive search of the entire codebase for the string "vibeflow" (case-insensitive) and report all findings with file paths and line numbers.

## FILES TO SEARCH
Search all files in the repository, including:
- Source code (.ts, .tsx, .js, .jsx)
- Configuration files (.json, .yaml, .yml, .env.example)
- Documentation (.md)
- Tests (.test.ts, .spec.ts)

Exclude:
- node_modules/
- .git/
- dist/
- build/
- Any lock files (package-lock.json, yarn.lock)

## TECHNICAL SPECIFICATIONS

### Search Approach
1. Use grep or ripgrep to search for "vibeflow" (case-insensitive)
2. For each occurrence, record:
   - File path (relative to repository root)
   - Line number
   - Context (surrounding code/text)
   - Type (code, comment, documentation, config)
3. Categorize findings by likelihood of needing change:
   - **Must Change**: User-facing text, branding
   - **May Change**: Internal references, comments
   - **Likely Keep**: Package names, URLs, technical identifiers

### Output Format
Return a structured report with:
- Total occurrence count
- Categorized list of all occurrences
- Recommendation for each category

## ACCEPTANCE CRITERIA
- [ ] All files in repository searched (excluding ignored directories)
- [ ] Every occurrence of "vibeflow" identified and documented
- [ ] Each occurrence categorized by change necessity
- [ ] Report ready for human review

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": [],
  "summary": "Found X occurrences of 'vibeflow' across Y files",
  "tests_written": [],
  "findings": {
    "total_occurrences": 0,
    "files_affected": [],
    "by_category": {
      "must_change": [],
      "may_change": [],
      "likely_keep": []
    }
  },
  "recommendation": "Proceed with T002 / Escalate for scope decision / etc",
  "notes": "Any important observations about the occurrences"
}
```

## DO NOT
- Modify any files during this search
- Skip any directories or file types
- Make assumptions about which occurrences should change
- Filter results based on assumptions
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [],
  "tests_required": [],
  "acceptance_criteria_met": [
    "Complete codebase search performed",
    "All occurrences documented and categorized",
    "Recommendation provided for next steps"
  ]
}
```

---

### T002: Change Dashboard Header Text (Conditional on T001)
**Confidence:** 0.88
**Dependencies:** T001 (code_context)
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T002 - Change Dashboard Header Text from "vibeflow" to "vibepilot"

## CONTEXT
Following the discovery search in T001, this task executes the approved text change. This task should ONLY proceed if T001 confirms that "vibeflow" appears only in the MissionHeader component, OR if human has explicitly approved the scope of changes based on T001's findings.

## DEPENDENCIES

### Code Context Dependencies
- T001: Review the search findings before proceeding. Check:
  - How many occurrences were found?
  - What was the recommendation?
  - Has human approved the scope?

**GATE CONDITION**: Do NOT proceed with this task unless:
1. T001 found "vibeflow" only in MissionModals.tsx, OR
2. Human has explicitly approved changing multiple occurrences

If T001 found multiple occurrences and no human approval received, STOP and report back.

## WHAT TO BUILD
Replace the text "vibeflow" with "vibepilot" in the MissionHeader component while preserving all existing styling properties:
- Color scheme (unchanged)
- Font family (unchanged)
- Font size (unchanged)
- Font weight (unchanged)
- Letter spacing (unchanged)
- Text casing (keep lowercase)
- Position/layout (unchanged)

## FILES TO MODIFY
- `vibeflow/apps/dashboard/components/modals/MissionModals.tsx` - Locate and replace "vibeflow" with "vibepilot" in the MissionHeader component

If T001 identified additional occurrences requiring change (with human approval), modify those files as well.

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: TypeScript
- Framework: React
- File Type: .tsx (TypeScript React component)

### Implementation Steps
1. Review T001 findings to confirm approved scope
2. Open MissionModals.tsx
3. Search for the exact string "vibeflow" (lowercase)
4. Verify you've found the correct location in MissionHeader component
5. Replace "vibeflow" with "vibepilot" (lowercase)
6. Ensure no other changes are made to the file
7. Verify the change is a simple text replacement only
8. If multiple files approved, repeat for each file

### Validation Checklist (Self-Check Before Reporting Complete)
- [ ] Only the text content changed
- [ ] No CSS/styling modifications
- [ ] No layout changes
- [ ] Component structure unchanged
- [ ] No other text or branding modified (beyond approved scope)
- [ ] File still compiles (no syntax errors)

## ACCEPTANCE CRITERIA
- [ ] Header displays "vibepilot" instead of "vibeflow"
- [ ] Color unchanged from original
- [ ] Font size unchanged from original
- [ ] Font family unchanged from original
- [ ] No other visual differences detectable
- [ ] Component renders without errors
- [ ] Responsive behavior preserved

## TESTS REQUIRED
This task prepares the change but does NOT include automated tests. Visual verification is handled in T003.

Self-validation only:
1. File syntax is valid (no compilation errors)
2. Only text content changed (diff shows single string replacement)
3. No styling properties modified

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "summary": "Changed header text from 'vibeflow' to 'vibepilot' in MissionHeader component",
  "tests_written": [],
  "notes": "Simple text replacement, no styling changes made. Ready for T003 visual verification.",
  "ready_for_visual_verification": true
}
```

## DO NOT
- Proceed if T001 found multiple occurrences without human approval
- Change any other text or branding (unless explicitly approved)
- Modify header layout or positioning
- Add new features
- Change color scheme
- Modify fonts
- Add animations
- Change any styling properties
- Modify any other components
- Leave TODO comments
- Commit changes (visual verification required first)
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "tests_required": [],
  "acceptance_criteria_met": [
    "Header text changed to 'vibepilot'",
    "All visual properties code-verified unchanged",
    "File syntax valid",
    "Ready for visual verification"
  ]
}
```

---

### T003: Visual Verification and Human Approval Workflow
**Confidence:** 0.90
**Dependencies:** T002 (summary)
**Type:** verification
**Category:** testing
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T003 - Visual Verification and Human Approval

## CONTEXT
After T002 makes the text change, this task defines the verification and approval workflow. Per AGENTS.md, UI/dashboard changes require human approval before merge. Per PRD success criteria #3, human must confirm no unintended changes.

## DEPENDENCIES

### Summary Dependencies
- T002: Text change has been made to MissionModals.tsx, changing "vibeflow" to "vibepilot" in the header. File is modified but not yet committed.

## WHAT TO BUILD
This task does NOT modify code. It provides the workflow instructions for visual verification and human approval.

## WORKFLOW TO EXECUTE

### Phase 1: Local Visual Verification
1. **Start development server** (if not already running)
   - Command: `npm run dev` or appropriate dev command for the dashboard
   - Location: apps/dashboard/

2. **Open dashboard in browser**
   - Navigate to the MissionHeader component location
   - Verify header displays "vibepilot" (not "vibeflow")

3. **Visual inspection checklist** (executor performs):
   - [ ] Text shows "vibepilot" (lowercase)
   - [ ] Text color unchanged
   - [ ] Font family unchanged
   - [ ] Font size unchanged
   - [ ] Font weight unchanged
   - [ ] Letter spacing unchanged
   - [ ] Position/layout unchanged
   - [ ] No visual regressions elsewhere on page
   - [ ] Responsive behavior works (test mobile/tablet/desktop)
   - [ ] No console errors

4. **If visual check fails**: 
   - Do NOT proceed to commit
   - Report failure with specific issues
   - Return to T002 for correction

### Phase 2: Human Approval Request
After visual verification passes, request human approval:

5. **Prepare summary for human**:
   - Files modified: MissionModals.tsx
   - Change made: "vibeflow" → "vibepilot"
   - Visual verification: PASSED
   - Request: "Ready to commit. Please approve for merge to main branch."

6. **Wait for human approval**:
   - Do NOT commit until approval received
   - Human may request additional verification or changes

7. **Upon approval**:
   - Commit with message: "fix: update dashboard header text from vibeflow to vibepilot"
   - Push to feature branch (NOT main)
   - Per AGENTS.md, this is a UI change requiring feature branch

### Phase 3: Deployment (Future - Not Part of This Task)
- Human will review and merge to main
- Deployment happens after merge

## ACCEPTANCE CRITERIA
- [ ] Visual verification performed and passed
- [ ] Human approval requested with clear summary
- [ ] Waiting for human approval (or approval received)
- [ ] If approved, commit ready but NOT pushed to main

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T003",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": [],
  "summary": "Visual verification PASSED. Awaiting human approval for commit.",
  "tests_written": [],
  "visual_verification": {
    "status": "passed",
    "checklist_results": {
      "text_displays_vibepilot": true,
      "color_unchanged": true,
      "font_unchanged": true,
      "layout_unchanged": true,
      "no_regressions": true,
      "responsive_works": true,
      "no_console_errors": true
    }
  },
  "human_approval_status": "pending",
  "ready_for_commit": false,
  "notes": "Executor performed visual verification. Waiting for human to approve before commit."
}
```

Alternative output if visual check fails:
```json
{
  "task_id": "T003",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": [],
  "summary": "Visual verification FAILED. Issues detected.",
  "tests_written": [],
  "visual_verification": {
    "status": "failed",
    "issues": ["Issue 1", "Issue 2"]
  },
  "human_approval_status": "not_requested",
  "ready_for_commit": false,
  "notes": "Return to T002 for correction",
  "requires_revision": true
}
```

## DO NOT
- Skip visual verification
- Commit without human approval
- Push to main branch directly (use feature branch)
- Proceed if visual check fails
- Rush the human approval process
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [],
  "tests_required": [],
  "acceptance_criteria_met": [
    "Visual verification performed",
    "Human approval workflow initiated",
    "Waiting for approval or revision as needed"
  ]
}
```

---

## Summary

**Total Tasks:** 3
**Estimated Context:** 
- T001: ~5,000 tokens (codebase search)
- T002: ~4,000 tokens (with T001 context)
- T003: ~2,000 tokens (workflow only)
- **Total:** ~11,000 tokens

**Critical Path:** T001 → T002 → T003

**Dependencies:**
- T001: none
- T002: depends on T001 (code_context) - must review search results
- T003: depends on T002 (summary) - executes after change made

**Confidence Score:** 92% (weighted average)

**Risk Mitigation:**
- T001 addresses PRD-identified risk of multiple occurrences
- T002 has gate condition to prevent premature execution
- T003 ensures human approval per AGENTS.md requirements
- Visual verification workflow fully specified

**Why Confidence is 92% (not 97%):**
- External dependency on human approval (unpredictable timing)
- Visual verification requires subjective judgment
- Multiple occurrences risk requires human decision if found
- No automated tests possible for visual changes
- Realistic assessment given these dependencies