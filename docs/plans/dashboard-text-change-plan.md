# PLAN: Dashboard Text Change - vibeflow to vibepilot

## Overview
Change the displayed text from "vibeflow" to "vibepilot" in the MissionHeader component. This is a simple text replacement task with built-in verification steps.

## Project Context
- **Project:** VibePilot Dashboard
- **Type:** Text Change (Foundation Test)
- **Priority:** P1
- **PRD:** docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md

## Tasks

### T001: Replace Dashboard Header Text with Verification
**Confidence:** 0.88
**Dependencies:** none
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T001 - Replace Dashboard Header Text from "vibeflow" to "vibepilot"

## CONTEXT
Simple text replacement in the MissionHeader component. This task includes pre-checks, the change itself, and verification steps to ensure quality.

## DEPENDENCIES
None - standalone task.

## WHAT TO BUILD
Replace the text "vibeflow" with "vibepilot" in the MissionHeader component while preserving all existing styling properties.

## IMPLEMENTATION WORKFLOW

### Step 1: Pre-Check - Search for Multiple Occurrences
Before making changes, search the codebase for "vibeflow":

```bash
cd /home/mjlockboxsocial/vibepilot
rg -i "vibeflow" --type-add 'code:*.{ts,tsx,js,jsx,json,md}' -t code
```

**Decision Point:**
- If found ONLY in `vibeflow/apps/dashboard/components/modals/MissionModals.tsx` → Proceed to Step 2
- If found in MULTIPLE locations → STOP and report to human:
  - List all file paths and line numbers
  - Request scope decision: "Found 'vibeflow' in X locations. Which should be changed?"
  - Do NOT proceed until human clarifies scope

### Step 2: Make the Text Change

**File to Modify:**
- `vibeflow/apps/dashboard/components/modals/MissionModals.tsx`

**Change Required:**
- Find: `vibeflow`
- Replace with: `vibepilot`
- Preserve: All styling, layout, casing (lowercase)

**Implementation:**
1. Open the file
2. Search for the exact string "vibeflow" (lowercase)
3. Verify you're in the MissionHeader component
4. Replace "vibeflow" with "vibepilot"
5. Ensure NO other changes are made

### Step 3: Self-Verification Checklist
Before reporting completion, verify:

- [ ] Only the text "vibeflow" → "vibepilot" changed
- [ ] No CSS/styling modifications
- [ ] No layout changes
- [ ] Component structure unchanged
- [ ] File syntax valid (no compilation errors)
- [ ] No other text modified

### Step 4: Visual Verification
Perform visual check to confirm:

1. **Start dev server** (if not running):
   ```bash
   cd apps/dashboard
   npm run dev
   ```

2. **Open dashboard** and navigate to MissionHeader

3. **Visual inspection:**
   - [ ] Text displays "vibepilot" (lowercase)
   - [ ] Color unchanged
   - [ ] Font family/size/weight unchanged
   - [ ] Position/layout unchanged
   - [ ] No console errors

4. **If visual check FAILS:**
   - Do NOT proceed
   - Revert change: `git checkout vibeflow/apps/dashboard/components/modals/MissionModals.tsx`
   - Report failure with specific issues

### Step 5: Human Approval Request
After visual verification passes:

1. **Prepare commit** (but do NOT push):
   ```bash
   git add vibeflow/apps/dashboard/components/modals/MissionModals.tsx
   git commit -m "fix: update dashboard header text from vibeflow to vibepilot"
   ```

2. **Request human approval:**
   - Summary: "Changed header text 'vibeflow' → 'vibepilot' in MissionModals.tsx"
   - Visual verification: PASSED
   - Files modified: 1
   - Ready for: Push to feature branch and merge approval

3. **Wait for human approval before pushing**

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: TypeScript
- Framework: React
- File Type: .tsx

### Constraints (DO NOT CHANGE)
- Color scheme
- Font family, size, weight
- Letter spacing
- Text casing (keep lowercase)
- Position/layout

## ACCEPTANCE CRITERIA
- [ ] Pre-check completed: searched for multiple occurrences
- [ ] Header displays "vibepilot" instead of "vibeflow"
- [ ] All visual properties unchanged (color, font, size, layout)
- [ ] Component renders without errors
- [ ] Visual verification performed and passed
- [ ] Human approval requested
- [ ] Commit ready (not yet pushed)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "summary": "Changed header text from 'vibeflow' to 'vibepilot' with verification",
  "tests_written": [],
  "verification": {
    "precheck_occurrences": "single",
    "self_verification": "passed",
    "visual_verification": "passed",
    "human_approval_status": "requested"
  },
  "ready_for_commit": true,
  "notes": "Simple text replacement. Visual verification passed. Awaiting human approval to push."
}
```

Alternative output if multiple occurrences found:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": [],
  "summary": "Pre-check found multiple 'vibeflow' occurrences. Escalating to human.",
  "tests_written": [],
  "verification": {
    "precheck_occurrences": "multiple",
    "locations_found": [
      {"file": "path/to/file1.tsx", "line": 42},
      {"file": "path/to/file2.tsx", "line": 15}
    ]
  },
  "ready_for_commit": false,
  "blocked": true,
  "blocked_reason": "Multiple occurrences found. Human must decide scope.",
  "notes": "Found 'vibeflow' in X locations. Awaiting scope decision."
}
```

## DO NOT
- Skip the pre-check search
- Proceed if multiple occurrences found without human decision
- Change any styling, layout, or visual properties
- Modify any other components or text
- Commit without visual verification
- Push to main branch directly (requires feature branch per AGENTS.md)
- Skip human approval request
- Leave TODO comments
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "tests_required": [],
  "acceptance_criteria_met": [
    "Pre-check completed",
    "Text changed successfully",
    "Visual properties preserved",
    "Visual verification passed",
    "Human approval requested"
  ]
}
```

---

## Summary

**Total Tasks:** 1
**Estimated Context:** ~4,500 tokens
**Critical Path:** T001
**Dependencies:** None
**Confidence Score:** 88%

**Why 88% confidence (realistic assessment):**
- Task is technically straightforward (text replacement)
- External dependency on human approval (timing unpredictable)
- Visual verification requires subjective judgment
- No automated tests possible for visual changes
- Multiple occurrences risk requires potential human decision
- More realistic than 97% given these external factors

**Consolidation Benefits:**
- Single task instead of 3 (simpler execution)
- ~4,500 tokens vs ~11,000 tokens (60% reduction)
- Pre-check built into task (no separate research task)
- Verification steps inline (standard workflow, not separate task)
- Clearer execution path with decision points
- Realistic confidence reflects actual dependencies

**Risk Mitigation:**
- Pre-check addresses PRD-identified multiple occurrences risk
- Visual verification ensures no unintended changes
- Human approval gate for UI changes (per AGENTS.md)
- Clear escalation path if scope is larger than expected