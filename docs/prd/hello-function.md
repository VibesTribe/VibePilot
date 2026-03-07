# PRD: Add Hello Function

Priority: Low
Complexity: Simple
Category: coding

## Context

We need a simple hello function to test the VibePilot flow end-to-end.

## What to Build

Create a simple Go function in `governor/cmd/tools/hello.go` that:
- Prints "Hello from VibePilot!" to stdout
- Returns nil error
- Has a main function that calls it

## Files

- governor/cmd/tools/hello.go

## Expected Output

- File created at `governor/cmd/tools/hello.go`
- Running `go run ./cmd/tools/hello.go` prints "Hello from VibePilot!"
