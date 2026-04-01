# PRD: Test Simple Feature

**Priority:** Low
**Complexity:** Simple
**Category:** coding

## Context
Test PRD to verify:
1. T001 numbering bug is fixed
2. Tasks execute successfully
3. Phase transitions work correctly
4. No infinite loops

## What to Build

### T001: Create hello world function
- **Type:** feature
- **Category:** coding
- **Slice:** general
- **Dependencies:** none
- **Confidence:** 0.95
- **Requires Codebase:** true

Create a simple `HelloWorld()` function in Go that returns "Hello, VibePilot!"

File: `governor/cmd/test/hello.go`

```go
package test

func HelloWorld() string {
    return "Hello, VibePilot!"
}
```

### T002: Create test for hello world
- **Type:** feature
- **Category:** coding
- **Slice:** general
- **Dependencies:** T001
- **Confidence:** 0.95
- **Requires Codebase:** true

Create a test for the HelloWorld function.

File: `governor/cmd/test/hello_test.go`

```go
package test

import "testing"

func TestHelloWorld(t *testing.T) {
    result := HelloWorld()
    expected := "Hello, VibePilot!"
    if result != expected {
        t.Errorf("Expected %q, got %q", expected, result)
    }
}
```

### T003: Run tests
- **Type:** feature
- **Category:** testing
- **Slice:** general
- **Dependencies:** T002
- **Confidence:** 0.95
- **Requires Codebase:** true

Run `go test ./governor/cmd/test/` to verify the tests pass.

## Expected Output
1. Three tasks created with numbers T001, T002, T003
2. Tasks execute sequentially (T001 → T002 → T003)
3. All tasks complete successfully
4. Code committed to task branches
5. Final merge to module branch
