# PLAN: Playwright Aria Snapshot Enhancement for VibePilot Couriers

## Overview
Replace screenshot-based vision with structured accessibility trees, add HAR recording for debugging, and adopt safe code manipulation patterns to improve courier performance and maintainability.

## Tasks

### T001: Implement Aria Snapshot with Boxes
**Confidence:** 0.97
**Category:** coding
**Dependencies:** none
**Target Files:** governor/internal/connectors/courier.go, scripts/courier_local.py, scripts/courier_run.py

#### Prompt Packet
```
# TASK: T001 - Implement Aria Snapshot with Boxes

## Context
Current courier implementation uses screenshot-based vision for AI decision-making, which is expensive, slow, and brittle to UI changes. This task replaces that approach with Playwright 1.60's ariaSnapshot({ box: true }) to extract accessibility trees with spatial bounding box information for AI decision-making where DOM interaction suffices.

## What to Build
Modify the courier's browser interaction logic to:
1. Replace screenshot-based vision calls with page.ariaSnapshot({ box: true }) in appropriate locations
2. Process the returned accessibility tree structure to extract AI-relevant information
3. Maintain fallback to existing methods only when accessibility tree is insufficient (per edge cases)
4. Ensure the spatial bounding box information is utilized for AI decision-making where relevant
5. Do not modify files unrelated to Playwright browser automation

## Files
- `governor/internal/connectors/courier.go` - Core courier logic where page actions are performed
- `scripts/courier_local.py` - Local browser harness execution logic
- `scripts/courier_run.py` - Main courier execution workflow
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["governor/internal/connectors/courier.go", "scripts/courier_local.py", "scripts/courier_run.py"],
  "tests_required": []
}
```

### T002: Implement HAR Recording for Debugging
**Confidence:** 0.96
**Category:** coding
**Dependencies:** ["T001"]
**Target Files:** governor/internal/connectors/courier.go, scripts/courier_local.py, scripts/courier_run.py

#### Prompt Packet
```
# TASK: T002 - Implement HAR Recording for Debugging

## Context
Couriers currently lack integrated request tracing, making debugging difficult and requiring re-runs to analyze failures. This task adds HAR recording capability using Playwright's tracing.startHar()/tracing.stopHar() to enable replay and analysis of failed interactions.

## What to Build
Add HAR recording functionality to the courier:
1. Implement tracing.startHar() to begin HTTP archive recording
2. Implement tracing.stopHar() to end recording and save HAR data
3. Make HAR recording opt-in or temporary to avoid accidental credential leakage
4. Ensure HAR recording does not interfere with normal courier operation
5. Provide ability to save/export HAR files for debugging sessions only
6. Handle HAR recording failures gracefully (log error, continue run)

## Files
- `governor/internal/connectors/courier.go` - Core courier lifecycle management
- `scripts/courier_local.py` - Local execution where tracing would be initiated
- `scripts/courier_run.py` - Main execution workflow for HAR integration
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["governor/internal/connectors/courier.go", "scripts/courier_local.py", "scripts/courier_run.py"],
  "tests_required": []
}
```

### T003: Implement noDefaults CDP Connections
**Confidence:** 0.95
**Category:** coding
**Dependencies:** ["T001", "T002"]
**Target Files:** governor/internal/connectors/courier.go

#### Prompt Packet
```
# TASK: T003 - Implement noDefaults CDP Connections

## Context
When attaching to existing Chrome sessions via Chrome DevTools Protocol, current implementation may alter user's browser state (clearing cookies, resetting settings), causing unintended side effects during courier operations.

## What to Build
Modify CDP connection logic to use { noDefaults: true } when attaching to existing Chrome sessions:
1. Identify all locations where CDP connections are established
2. Modify connection options to include { noDefaults: true }
3. Ensure this preserves user's logged-in state and browser configuration
4. Verify noDefaults only applies to headed CDP connections as appropriate
5. Do not affect headless mode operation
6. Maintain backward compatibility with existing connection logic

## Files
- `governor/internal/connectors/courier.go` - Contains CDP connection logic in courierWaiter and related functions
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["governor/internal/connectors/courier.go"],
  "tests_required": []
}
```

### T004: Implement str_replace Code Manipulation Pattern
**Confidence:** 0.95
**Category:** coding
**Dependencies:** ["T001", "T002", "T003"]
**Target Files:** governor/internal/connectors/courier.go

#### Prompt Packet
```
# TASK: T004 - Implement str_replace Code Manipulation Pattern

## Context
Current code modification patterns in couriers use partial matches that can lead to silent errors from unintended replacements. This task adopts an exact-match/replace pattern: fail if not unique, preventing silent errors from partial matches.

## What to Build
Implement exact-match/replace pattern for code modifications in courier self-modification routines:
1. Replace existing string replacement logic with exact-match/replace pattern
2. Ensure modifications fail if the target string is not found exactly once
3. Prevent silent errors from partial matches that could corrupt code
4. Increase safety and predictability of automated code changes
5. Apply this pattern to all code modification routines in the courier
6. Maintain existing functionality while improving reliability

## Files
- `governor/internal/connectors/courier.go` - Contains code manipulation utilities for self-modification
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["governor/internal/connectors/courier.go"],
  "tests_required": []
}
```
},
  "tasks": [
    {
      "task_number": "T001",
      "title": "Implement Aria Snapshot with Boxes",
      "category": "coding",
      "confidence": 0.97,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Implement Aria Snapshot with Boxes

## Context
Current courier implementation uses screenshot-based vision for AI decision-making, which is expensive, slow, and brittle to UI changes. This task replaces that approach with Playwright 1.60's ariaSnapshot({ box: true }) to extract accessibility trees with spatial bounding box information for AI decision-making where DOM interaction suffices.

## What to Build
Modify the courier's browser interaction logic to:
1. Replace screenshot-based vision calls with page.ariaSnapshot({ box: true }) in appropriate locations
2. Process the returned accessibility tree structure to extract AI-relevant information
3. Maintain fallback to existing methods only when accessibility tree is insufficient (per edge cases)
4. Ensure the spatial bounding box information is utilized for AI decision-making where relevant
5. Do not modify files unrelated to Playwright browser automation

## Files
- `governor/internal/connectors/courier.go` - Core courier logic where page actions are performed
- `scripts/courier_local.py` - Local browser harness execution logic
- `scripts/courier_run.py` - Main courier execution workflow",
      "expected_output": {
        "files_created": [],
        "files_modified": [
          "governor/internal/connectors/courier.go",
          "scripts/courier_local.py",
          "scripts/courier_run.py"
        ],
        "tests_required": []
      }
    },
    {
      "task_number": "T002",
      "title": "Implement HAR Recording for Debugging",
      "category": "coding",
      "confidence": 0.96,
      "dependencies": [
        "T001"
      ],
      "prompt_packet": "# TASK: T002 - Implement HAR Recording for Debugging

## Context
Couriers currently lack integrated request tracing, making debugging difficult and requiring re-runs to analyze failures. This task adds HAR recording capability using Playwright's tracing.startHar()/tracing.stopHar() to enable replay and analysis of failed interactions.

## What to Build
Add HAR recording functionality to the courier:
1. Implement tracing.startHar() to begin HTTP archive recording
2. Implement tracing.stopHar() to end recording and save HAR data
3. Make HAR recording opt-in or temporary to avoid accidental credential leakage
4. Ensure HAR recording does not interfere with normal courier operation
5. Provide ability to save/export HAR files for debugging sessions only
6. Handle HAR recording failures gracefully (log error, continue run)

## Files
- `governor/internal/connectors/courier.go` - Core courier lifecycle management
- `scripts/courier_local.py` - Local execution where tracing would be initiated
- `scripts/courier_run.py` - Main execution workflow for HAR integration",
      "expected_output": {
        "files_created": [],
        "files_modified": [
          "governor/internal/connectors/courier.go",
          "scripts/courier_local.py",
          "scripts/courier_run.py"
        ],
        "tests_required": []
      }
    },
    {
      "task_number": "T003",
      "title": "Implement noDefaults CDP Connections",
      "category": "coding",
      "confidence": 0.95,
      "dependencies": [
        "T001",
        "T002"
      ],
      "prompt_packet": "# TASK: T003 - Implement noDefaults CDP Connections

## Context
When attaching to existing Chrome sessions via Chrome DevTools Protocol, current implementation may alter user's browser state (clearing cookies, resetting settings), causing unintended side effects during courier operations.

## What to Build
Modify CDP connection logic to use { noDefaults: true } when attaching to existing Chrome sessions:
1. Identify all locations where CDP connections are established
2. Modify connection options to include { noDefaults: true }
3. Ensure this preserves user's logged-in state and browser configuration
4. Verify noDefaults only applies to headed CDP connections as appropriate
5. Do not affect headless mode operation
6. Maintain backward compatibility with existing connection logic

## Files
- `governor/internal/connectors/courier.go` - Contains CDP connection logic in courierWaiter and related functions",
      "expected_output": {
        "files_created": [],
        "files_modified": [
          "governor/internal/connectors/courier.go"
        ],
        "tests_required": []
      }
    },
    {
      "task_number": "T004",
      "title": "Implement str_replace Code Manipulation Pattern",
      "category": "coding",
      "confidence": 0.95,
      "dependencies": [
        "T001",
        "T002",
        "T003"
      ],
      "prompt_packet": "# TASK: T004 - Implement str_replace Code Manipulation Pattern

## Context
Current code modification patterns in couriers use partial matches that can lead to silent errors from unintended replacements. This task adopts an exact-match/replace pattern: fail if not unique, preventing silent errors from partial matches.

## What to Build
Implement exact-match/replace pattern for code modifications in courier self-modification routines:
1. Replace existing string replacement logic with exact-match/replace pattern
2. Ensure modifications fail if the target string is not found exactly once
3. Prevent silent errors from partial matches that could corrupt code
4. Increase safety and predictability of automated code changes
5. Apply this pattern to all code modification routines in the courier
6. Maintain existing functionality while improving reliability

## Files
- `governor/internal/connectors/courier.go` - Contains code manipulation utilities for self-modification",
      "expected_output": {
        "files_created": [],
        "files_modified": [
          "governor/internal/connectors/courier.go"
        ],
        "tests_required": []
      }
    }
  ],
  "total_tasks": 4,
  "status": "review