# Runner Interface Contract

Every runner in VibePilot MUST follow this exact interface. This is the contract that makes everything swappable.

## The Contract

```
INPUT:  JSON via stdin or --task flag
OUTPUT: JSON via stdout
EXIT:   0 = success, 1 = failure
```

## Command Line Interface

Every runner MUST support:

```bash
# Execute a task
python runners/courier.py --task task_packet.json --output result.json

# OR via stdin
cat task_packet.json | python runners/courier.py > result.json

# Health check (required for ok_probe)
python runners/courier.py --probe
# Must exit 0 and print "OK" if healthy
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (task completed) |
| 1 | Failure (task failed, check output for reason) |
| 2 | Probe success (only for --probe mode) |

## Input Format

Task packet JSON passed to runner:

```json
{
  "task_id": "P1-T001",
  "title": "Task title",
  "objectives": ["Objective 1", "Objective 2"],
  "deliverables": ["file1.py", "file2.py"],
  "prompt": "Full prompt text with instructions...",
  "context": "Why this task exists",
  "output_format": {
    "type": "code|text|json|markdown|files",
    "language": "python|typescript|..."
  },
  "constraints": {
    "max_tokens": 2000,
    "timeout_seconds": 120,
    "no_attachments": true
  },
  "runner_context": {
    "platform": "chatgpt",
    "model": "gpt-4o",
    "attempt_number": 1,
    "previous_failures": []
  }
}
```

Internal runners also receive:

```json
{
  "task_packet": { ... },
  "codebase_files": {
    "relevant_file.py": "...content...",
    "another_file.py": "...content..."
  }
}
```

## Output Format

Every runner MUST return this exact structure:

```json
{
  "task_id": "P1-T001",
  "status": "success|failed|partial|timeout",
  "output": "The actual output content or summary",
  "artifacts": ["path/to/file1.py", "path/to/file2.py"],
  "errors": [
    {
      "code": "RESOURCE_LIMIT",
      "message": "Platform rate limit exceeded"
    }
  ],
  "metadata": {
    "model": "gpt-4o",
    "platform": "chatgpt",
    "chat_url": "https://chat.openai.com/c/abc123",
    "tokens_in": 500,
    "tokens_out": 1200,
    "duration_seconds": 45,
    "runner_version": "1.0.0"
  },
  "feedback": {
    "output_matched_prompt": true,
    "suggested_next_step": "complete|retry|reassign|split|escalate",
    "failure_reason": null,
    "virtual_cost": 0.0024
  }
}
```

## Courier-Specific Requirements

Couriers (web platform runners) MUST additionally return:

```json
{
  "metadata": {
    "chat_url": "https://...",  // REQUIRED for revision requests
    "platform": "chatgpt",
    "model": "gpt-4o"
  }
}
```

Without chat_url, revisions require full context redo.

## Internal Runner Requirements

Internal runners (CLI/API) MUST additionally return:

```json
{
  "metadata": {
    "files_read": 5,
    "files_modified": 2,
    "git_branch": "task/P1-T001-auth"
  }
}
```

## Probe Mode

Every runner MUST implement `--probe`:

```bash
python runners/courier.py --probe
```

Expected output:
```
OK
```

Exit code: 0

If probe fails (missing dependencies, auth expired, platform down):
```
PROBE_FAILED: [reason]
```

Exit code: 1

## Error Handling

On failure, runner MUST:

1. Set status to "failed"
2. Include failure_reason in feedback
3. Include error details in errors array
4. Exit with code 1

Example failure output:
```json
{
  "task_id": "P1-T001",
  "status": "failed",
  "output": null,
  "errors": [
    {
      "code": "PLATFORM_DOWN",
      "message": "ChatGPT returned 503, service unavailable"
    }
  ],
  "feedback": {
    "output_matched_prompt": false,
    "suggested_next_step": "reassign",
    "failure_reason": "platform_down"
  }
}
```

## Token Tracking (Required)

Even failed attempts MUST report tokens burned:

```json
{
  "metadata": {
    "tokens_in": 300,
    "tokens_out": 50
  },
  "feedback": {
    "virtual_cost": 0.0008
  }
}
```

Virtual cost = what this would have cost via API. Used for ROI tracking.

## Language Agnostic

The interface is JSON in, JSON out. Runners can be:
- Python
- TypeScript
- Bash
- Any language that can read stdin and write stdout

Only the config path changes:
```json
{
  "id": "courier",
  "runner": "runners/courier.py"  // or "runners/courier.ts"
}
```

## Validation

Runners SHOULD validate:
1. Input JSON matches task_packet schema
2. Required fields present
3. Constraints respected (timeout, max_tokens)

If validation fails:
```json
{
  "task_id": null,
  "status": "failed",
  "errors": [{"code": "INVALID_INPUT", "message": "Missing required field: prompt"}],
  "feedback": {
    "suggested_next_step": "retry",
    "failure_reason": "invalid_input"
  }
}
```

## Summary

| Requirement | Description |
|-------------|-------------|
| Input | JSON via stdin or --task flag |
| Output | JSON via stdout (exact schema) |
| Exit codes | 0 = success, 1 = failure |
| --probe | Health check, prints "OK" |
| chat_url | Required for couriers |
| tokens | Required even on failure |
| virtual_cost | Required for ROI tracking |

This contract is NON-NEGOTIABLE. Every runner must follow it exactly.
