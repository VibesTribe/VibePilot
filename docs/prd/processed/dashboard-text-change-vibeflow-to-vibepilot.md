# PRD: Change "vibeflow" to "vibepilot" in Dashboard Header

**Project:** VibePilot Dashboard  
**Task ID:** DASHBOARD-001  
**Type:** Text Change  
**Priority:** P1 (Foundation Test)  
**Confidence:** 99%  

---

## 1. Objective

Change the displayed text from "vibeflow" to "vibepilot" in the MissionHeader component of the dashboard.

---

## 2. Requirements

### 2.1 Exact Change Required
- **Current text:** `vibeflow`
- **New text:** `vibepilot`
- **Location:** MissionHeader component (apps/dashboard/components/)

### 2.2 Constraints (MUST NOT CHANGE)
- Color scheme (keep existing colors)
- Font family (keep existing font)
- Font size (keep existing size)
- Font weight (keep existing weight)
- Letter spacing (keep existing spacing)
- Text casing (keep lowercase)
- Position/layout (keep existing placement)

### 2.3 Visual Consistency
- Text must remain visually identical in all respects except the word itself
- No visual regression in header appearance
- Responsive behavior unchanged

---

## 3. Technical Details

### 3.1 File Location
```
vibeflow/apps/dashboard/components/modals/MissionModals.tsx
```

### 3.2 Component Structure
- Component: `MissionHeader`
- Text appears in header title area
- Currently displays project name/branding

### 3.3 Search Pattern
Search for exact string: `"vibeflow"` (lowercase)

### 3.4 Implementation Approach
1. Locate "vibeflow" text in MissionHeader component
2. Replace with "vibepilot"
3. Verify no other styling changes occurred
4. Test visual appearance matches original

---

## 4. Acceptance Criteria

- [ ] Header displays "vibepilot" instead of "vibeflow"
- [ ] Color unchanged from original
- [ ] Font size unchanged from original
- [ ] Font family unchanged from original
- [ ] No other visual differences detectable
- [ ] Component renders without errors
- [ ] Responsive behavior preserved

---

## 5. Out of Scope

- Changing any other text or branding
- Modifying header layout or positioning
- Adding new features
- Changing color scheme
- Font modifications
- Animation changes

---

## 6. Success Criteria

**Task is complete when:**
1. Dashboard header shows "vibepilot" text
2. Visual appearance identical to original (except word change)
3. Human confirms no unintended changes
4. Change deployed and visible in production

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Wrong file changed | Low | Medium | Verify file path before change |
| Style accidentally modified | Low | Low | Code review, visual check |
| Text appears in multiple places | Medium | Low | Search entire codebase for "vibeflow" |

---

**PRD Status:** READY FOR PLANNER  
**Created By:** Consultant Agent  
**Date:** 2026-02-21
