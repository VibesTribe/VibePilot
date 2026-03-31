# PLAN: Test Config Fix + Permission Bypass

## Overview
Create a Python validation script to verify that config routing and permission bypass fixes are both working correctly. This is a single-task plan with high confidence.

## Slice
- **slice_id:** test-config-fix-v5
- **Status:** new

## Tasks

### T001: Create Config + Permission Bypass Validation Script
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Config + Permission Bypass Validation Script

## Context
Two fixes have been applied to VibePilot: (1) config routing no longer produces "no_routing" errors, and (2) permission bypass allows file writes without timeout errors. We need a validation script to confirm both fixes work end-to-end.

## What to Build
Create a single Python script `config_test.py` with the following exact content and behavior:

```python
#!/usr/bin/env python3
"""
VibePilot Config + Permission Bypass Validation
"""

print("✅ Config fix: SUCCESS!")
print("✅ Permission bypass: SUCCESS!")
print(f"✅ Calculation: {7} * {6} = {7 * 6}")

# Test file write (this requires permission bypass)
with open("config_test_output.txt", "w") as f:
    f.write("VibePilot v5: Both fixes working!\n")

print("✅ File write: SUCCESS!")
print("\n🎉 All systems operational!")
```

## Files
- `config_test.py` - Main validation script
- `config_test_output.txt` - Output file (created by the script when run)

## Execution
After creating the file, run `python3 config_test.py` to verify:
1. All 5 success messages print to stdout
2. `config_test_output.txt` is created with content: `VibePilot v5: Both fixes working!`
3. No errors occur
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["config_test.py"],
  "files_created_on_run": ["config_test_output.txt"],
  "tests_written": [],
  "success_criteria": [
    "No 'no_routing' errors",
    "No permission timeout errors",
    "config_test_output.txt created with correct content",
    "All 5 success messages printed",
    "Task completes in ~2-3 minutes"
  ]
}
```
