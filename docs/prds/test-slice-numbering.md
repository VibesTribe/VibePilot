# PRD: Test Slice-Based Task Numbering

**Priority**: Low
**Complexity**: Simple
**Category**: coding
**Module**: general

## What to Build

Create a simple test script to verify slice-based numbering is working:

```python
#!/usr/bin/env python3
"""
Test slice-based task numbering
"""

print("✅ Slice-based numbering test")
print(f"Slice: general")
print(f"Test number: {1 + 1}")
print("Expected: task/general/T002 (since T001 exists)")
```

## Files

- `test_slice_numbering.py`

## Success Criteria

1. Task gets next sequential number (T002, not T001)
2. Branch name is `task/general/T002`
3. No collisions with existing tasks
4. Plan approves and task executes successfully


# Updated Tue Mar 31 06:00:57 PM EDT 2026
