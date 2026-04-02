# PRD: Final Routing Test

**Priority:** Low
**Complexity:** Simple
**Category:** coding

## Context
Final test to verify complete routing fix:
1. Planner (glm-5) routes to claude-code ✅
2. Supervisor (glm-5) routes to claude-code
3. Tasks execute in second Claude CLI session

## What to Build

### T001: Create simple test file
- **Type:** feature
- **Category:** coding
- **Slice:** general
- **Dependencies:** none
- **Confidence:** 0.95
- **Requires Codebase:** true

Create a simple test file.

File: `governor/cmd/test/routing_final_test.go`

```go
package test

func RoutingFinalTest() bool {
    return true
}
```

## Expected Output
1. Plan created successfully
2. Plan reviewed and approved
3. Tasks created and execute successfully
