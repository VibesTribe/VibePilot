# PLAN: Dashboard Text Change - vibeflow to vibepilot

## Overview
Change the displayed text from "vibeflow" to "vibepilot" in the MissionHeader component of the VibePilot dashboard. This is a simple text replacement with no styling or layout changes.

## Project Context
- **Project:** VibePilot Dashboard
- **Type:** Text Change (Foundation Test)
- **Priority:** P1
- **PRD:** docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md

## Tasks

### T001: Change Dashboard Header Text from "vibeflow" to "vibepilot"
**Confidence:** 0.97
**Dependencies:** none
**Type:** feature
**Category:** coding
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T001 - Change Dashboard Header Text

## CONTEXT
The VibePilot dashboard currently displays "vibeflow" in the MissionHeader component. We need to update this to "vibepilot" to reflect the correct project name. This is a simple text change with NO modifications to styling, layout, or any other visual properties.

## DEPENDENCIES
None - this is a standalone change.

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

## TECHNICAL SPECIFICATIONS

### Implementation Steps
1. Search the file for the exact string "vibeflow" (lowercase)
2. Verify you've found the correct location in MissionHeader component
3. Replace "vibeflow" with "vibepilot" (lowercase)
4. Ensure no other changes are made to the file
5. Verify the change is a simple text replacement only

### Validation Checklist
- [ ] Only the text content changed
- [ ] No CSS/styling modifications
- [ ] No layout changes
- [ ] Component structure unchanged
- [ ] No other text or branding modified

## ACCEPTANCE CRITERIA
- [ ] Header displays "vibepilot" instead of "vibeflow"
- [ ] Color unchanged from original
- [ ] Font size unchanged from original
- [ ] Font family unchanged from original
- [ ] No other visual differences detectable
- [ ] Component renders without errors
- [ ] Responsive behavior preserved

## TESTS REQUIRED
Visual verification:
1. Dashboard loads without errors
2. Header displays "vibepilot" text
3. Visual appearance matches original (except word change)
4. Responsive behavior works on different screen sizes

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "summary": "Changed header text from 'vibeflow' to 'vibepilot' in MissionHeader component",
  "tests_written": [],
  "notes": "Simple text replacement, no styling changes made"
}
```

## DO NOT
- Change any other text or branding
- Modify header layout or positioning
- Add new features
- Change color scheme
- Modify fonts
- Add animations
- Change any styling properties
- Modify any other components
- Leave TODO comments
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["vibeflow/apps/dashboard/components/modals/MissionModals.tsx"],
  "tests_required": [],
  "acceptance_criteria_met": [
    "Header displays 'vibepilot' instead of 'vibeflow'",
    "All visual properties unchanged",
    "Component renders without errors"
  ]
}
```

---

## Summary

**Total Tasks:** 1
**Estimated Context:** ~3,000 tokens
**Critical Path:** T001
**Dependencies:** None

**Confidence Score:** 97%

This is a straightforward text replacement task with clear requirements and minimal risk. The PRD provides exact file location and clear acceptance criteria.
