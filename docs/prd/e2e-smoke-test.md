# E2E Smoke Test: Greeting JSON

## Requirements

### FR-001: Generate greeting JSON file
- **Priority**: P1
- **Type**: NEW
- **Description**: Create a single file `output/hello.json` containing a JSON object with a greeting message, the current timestamp, and a status field.
- **Acceptance Criteria**:
  - File exists at `output/hello.json`
  - Contains valid JSON with keys: `greeting` (string), `timestamp` (ISO 8601), `status` (string "ok")

## Constraints
- Single file output only
- No modifications to any existing files
- No external dependencies
- Output directory: `output/`
