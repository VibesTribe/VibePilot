# PLAN: String Utilities

## Overview
Create string utility functions for common operations including reverse and truncate.

## Tasks

### T001: Implement Reverse Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Implement Reverse Function

## Context
Create a string utility function that reverses a string. This is a foundational utility for string manipulation.

## What to Build
Implement `reverse(s string) string` function that:
- Returns an empty string when input is empty
- Returns the reversed string for non-empty input
- Handles UTF-8 characters correctly

## Files
- `pkg/strings/utils.go` - Add the reverse function

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/strings/utils.go"],
  "tests_written": []
}
```
```

---

### T002: Implement Truncate Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T002 - Implement Truncate Function

## Context
Create a string utility function that truncates strings with ellipsis for display purposes.

## What to Build
Implement `truncate(s string, maxLen int) string` function that:
- Returns empty string when input is empty
- Returns original string if len(s) <= maxLen
- Returns truncated string with "..." suffix if len(s) > maxLen
- maxLen includes the ellipsis (e.g., maxLen=5 means 2 chars + "...")

## Files
- `pkg/strings/utils.go` - Add the truncate function

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/strings/utils.go"],
  "tests_written": []
}
```
```

---

### T003: Write Unit Tests
**Confidence:** 0.98
**Category:** testing
**Dependencies:** ["T001", "T002"]

#### Prompt Packet
```markdown
# TASK: T003 - Write Unit Tests

## Context
Comprehensive unit tests ensure the string utilities work correctly across all edge cases.

## What to Build
Create unit tests in `pkg/strings/utils_test.go` covering:

**Reverse function tests:**
- Empty string returns empty
- Single character returns same
- Multi-character string returns reversed
- UTF-8 characters handled correctly

**Truncate function tests:**
- Empty string returns empty
- String shorter than maxLen returns unchanged
- String equal to maxLen returns unchanged
- String longer than maxLen returns truncated with ellipsis
- maxLen of 3 returns just "..."

## Files
- `pkg/strings/utils_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["pkg/strings/utils_test.go"],
  "tests_written": ["pkg/strings/utils_test.go"]
}
```
```
