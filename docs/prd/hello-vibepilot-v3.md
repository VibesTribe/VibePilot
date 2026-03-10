# PRD: Hello VibePilot v3

Priority: Low
Complexity: Simple
Category: coding

## Context
The file governor/cmd/tools/hello.go currently has a comma in the output. We need exactly "Hello, VibePilot!" (with exclamation).

## What to Build
Update governor/cmd/tools/hello.go to:
- Keep the SayHello function
- Change main() to print "Hello, VibePilot!" exactly (with exclamation)

## Files
- `governor/cmd/tools/hello.go` - Update main function output

## Expected Output
- Running `go run ./cmd/tools/hello.go` should output:
- "Hello, World!"
- "Hello, VibePilot!"
