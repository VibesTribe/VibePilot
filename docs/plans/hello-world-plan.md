# PLAN: Hello World

## Overview
Create a simple hello function in Go that returns a greeting.

## Tasks

### T001: Create SayHello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create SayHello Function

## Context
Need a reusable greeting function for the governor tools package.

## What to Build
Create `governor/cmd/tools/hello.go` with:
- Function `SayHello(name string) string`
- If name is empty, return "Hello, World!"
- Otherwise return "Hello, {name}!"
- Add package declaration `package tools`

## Files
- `governor/cmd/tools/hello.go` - The SayHello function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```

### T002: Write Unit Tests
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Write Unit Tests

## Context
Verify the SayHello function works correctly for all cases.

## What to Build
Create `governor/cmd/tools/hello_test.go` with:
- Test `TestSayHello` covering:
  - Empty string returns "Hello, World!"
  - Name "Alice" returns "Hello, Alice!"
  - Name "Bob" returns "Hello, Bob!"
- Use standard Go testing package

## Files
- `governor/cmd/tools/hello_test.go` - Unit tests
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/cmd/tools/hello_test.go"],
  "tests_written": ["governor/cmd/tools/hello_test.go"]
}
```
