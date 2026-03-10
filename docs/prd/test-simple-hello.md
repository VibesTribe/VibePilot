# PRD: Test-simple-hello

Priority: Low
Complexity: simple
Category: coding

## Context
Test that verify the V0.4.0 flow works end-to-end with proper task routing and execution.

## What to Build
Create a simple greeting function that takes a name and returns "Hello, {name}!"

## Files
- docs/prd/test-simple-hello.md (can be empty)

## Expected Output
- Task created with status "available"
- Task assigned to a model
- Task executed
- Task output committed to task branch
- Task status updated to "review"
- Supervisor reviews
- Task passes to testing or- Task passes
- Task merged to