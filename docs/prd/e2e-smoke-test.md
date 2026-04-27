# E2E Smoke Test — Greeting Output

## Requirements

### FR-001: Generate greeting JSON
- **Priority**: P1
- **Type**: NEW
- **Description**: Create `output/hello.json` containing a greeting object with a "greeting" key and a "timestamp" key with the current ISO 8601 timestamp.
- **Acceptance Criteria**: File exists at `output/hello.json` with valid JSON containing both keys.

## Constraints
- Single file output in `output/` directory
- No existing files modified
- No external dependencies
