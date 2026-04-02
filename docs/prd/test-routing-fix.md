# PRD: Test Routing Fix

**Priority:** Low
**Complexity:** Simple
**Category:** coding

## Context
Test PRD to verify the routing fix works:
1. Router uses agent's configured model from agents.json
2. Planner (glm-5) routes correctly to claude-code
3. Tasks execute in second Claude CLI session

## What to Build

### T001: Create test file
- **Type:** feature
- **Category:** coding
- **Slice:** general
- **Dependencies:** none
- **Confidence:** 0.95
- **Requires Codebase:** true

Create a simple test file to verify routing works.

File: `governor/cmd/test/routing_test.go`

```go
package test

import "testing"

func TestRoutingFix(t *testing.T) {
    // Test that routing fix is working
    result := "routing-fixed"
    expected := "routing-fixed"
    if result != expected {
        t.Errorf("Expected %q, got %q", expected, result)
    }
}
```

## Expected Output
1. Plan is created successfully (routing to planner works)
2. Tasks are generated
3. Tasks execute successfully in second Claude session
