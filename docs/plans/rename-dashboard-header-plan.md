# PLAN: Rename Dashboard Header

## Overview
Change the dashboard header from "Vibeflow" to "vibepilot" for brand consistency.

## Tasks

### T001: Update Header Brand Text
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Update Header Brand Text

## Context
The dashboard header currently displays "Vibeflow" and needs to be changed to "vibepilot" for brand consistency.

## What to Build
Change the brand text in the dashboard header from "Vibeflow" to "vibepilot".

## Files
- `~/vibeflow/apps/dashboard/components/MissionHeader.tsx` - Line 309 contains the brand text

## Instructions
1. Open ~/vibeflow/apps/dashboard/components/MissionHeader.tsx
2. Find line 309: `<span className="mission-header__brand">Vibeflow</span>`
3. Change "Vibeflow" to "vibepilot" (lowercase as per PRD)
4. Save the file
5. Verify no other changes were made
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["~/vibeflow/apps/dashboard/components/MissionHeader.tsx"],
  "tests_written": [],
  "verification": "grep -n 'vibepilot' ~/vibeflow/apps/dashboard/components/MissionHeader.tsx returns line 309"
}
```
