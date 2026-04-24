# PRD: Hello World Smoke Test

## Summary
Create a single standalone file `output/hello.json` containing a simple greeting payload. This is a pipeline smoke test -- the output must NOT modify any existing source code in this repository.

## Requirements

### Output
Create exactly ONE file: `output/hello.json`

The file must contain valid JSON with this structure:
```json
{
  "message": "Hello from VibePilot!",
  "timestamp": "<ISO 8601 timestamp of when the file was created>",
  "pipeline_test": true,
  "version": "1.0.0"
}
```

### Constraints
- Do NOT modify any existing files in this repository
- Do NOT touch any Go source code, config files, or prompts
- Create ONLY the `output/hello.json` file
- The `output/` directory may need to be created
- This is a SINGLE task -- do not decompose into multiple tasks

### Verification
- The file must be valid JSON
- The file must be parseable by any standard JSON parser
- The `message` field must contain the exact string "Hello from VibePilot!"
- The `timestamp` field must be a valid ISO 8601 datetime

## Success Criteria
- `output/hello.json` exists and is valid JSON
- No other files in the repository are modified

