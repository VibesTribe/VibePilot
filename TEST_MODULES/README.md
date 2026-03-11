# TEST MODULES

This folder contains isolated test modules. Each module is a self-contained branch that can be:
- Tested independently
- Cleaned up easily
- Merged to main only after full validation

## Structure

```
TEST_MODULES/
├── <slice_id>/
│   └── (module branch content)
```

## Flow

1. Task output → task branch (task/T001)
2. Task approved → merge to module branch (TEST_MODULES/<slice_id>)
3. Task branch deleted
4. All module tasks complete → full module test
5. Module passes → merge to main
6. Module branch deleted

This ensures main branch is never contaminated with untested code.

