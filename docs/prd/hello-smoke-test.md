# Hello World Smoke Test

## Requirements

### FR-001: Generate greeting JSON file
- **Priority**: P1
- **Type**: NEW
- **Description**: Create a single file `output/hello.json` containing a JSON object with a greeting message and ISO 8601 timestamp.
- **Acceptance Criteria**:
  - File exists at `output/hello.json`
  - Contains valid JSON with keys: `message` (string, value "Hello from VibePilot!"), `timestamp` (ISO 8601 string)
  - No other files are created or modified

## Constraints
- Single file output only
- No external dependencies or network calls
- No modifications to any existing files in the repository
- The output directory must be created if it does not exist
