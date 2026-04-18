# PLAN: Pipeline Smoke Test - Hello Function

## Overview
Minimal smoke test to verify the full VibePilot pipeline: create a simple Go package with a Greet function and its tests. Zero external dependencies, zero modifications to existing code.

## Tasks

### T001: Create hello package with Greet function and tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create hello package with Greet function and tests

## Context
This is a pipeline smoke test. The goal is to verify the full VibePilot orchestration pipeline (planner → executor → tester → reviewer → merger) can deliver a trivial but real Go package end-to-end. No external dependencies, no existing file modifications.

## What to Build

1. Create directory `governor/internal/hello/` if it does not exist.

2. Create `governor/internal/hello/hello.go`:
```go
package hello

// Greet returns a greeting for the given name.
func Greet(name string) string {
	return "Hello, " + name + "!"
}
```

3. Create `governor/internal/hello/hello_test.go`:
```go
package hello

import "testing"

func TestGreet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal name",
			input:    "World",
			expected: "Hello, World!",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "Hello, !",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Greet(tt.input)
			if result != tt.expected {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
```

4. Run `go build ./...` from the `governor/` directory and confirm it succeeds.
5. Run `go test ./internal/hello/...` from the `governor/` directory and confirm both test cases pass.

## Constraints
- ONLY create files under `governor/internal/hello/`
- Do NOT modify any existing files
- No external dependencies

## Files
- `governor/internal/hello/hello.go` - Greet function implementation
- `governor/internal/hello/hello_test.go` - Table-driven tests for Greet
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "governor/internal/hello/hello.go",
    "governor/internal/hello/hello_test.go"
  ],
  "tests_written": [
    "governor/internal/hello/hello_test.go"
  ],
  "build_passed": true,
  "test_passed": true,
  "existing_files_modified": false
}
```
