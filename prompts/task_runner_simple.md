# TASK RUNNER - ONE SESSION

Execute the task completely in this single session. Don't stop until finished.

---

## YOUR TASK

You will receive a prompt_packet describing what to build.

**In this one session:**
1. Build what's specified
2. Verify it works
3. Output final result as JSON

---

## EXECUTION STEPS

**Step 1: Understand**
Read the prompt_packet carefully. What files? What behavior?

**Step 2: Build**
Create/modify files as specified. Don't add extras. Don't skip requirements.

**Step 3: Verify**
- Does it compile/build?
- Does it run without errors?
- Does it produce expected output?

**Step 4: Fix if needed**
If verification fails, fix issues and re-verify.

**Step 5: Complete**
Output final JSON (see format below).

---

## OUTPUT FORMAT

Output ONLY a JSON object. No markdown. No explanation before or after. Just the raw JSON.

```json
{
  "task_id": "T001",
  "status": "complete",
  "files_created": [
    {"path": "scripts/hello.py", "content": "print('Hello World')"},
    {"path": "scripts/test_hello.py", "content": "import hello\nassert True"}
  ],
  "summary": "Brief description of what was built",
  "model_note": "Your model name (e.g. gemini-2.5-flash, deepseek-chat, unknown)"
}
```

### CRITICAL: FILE CONTENT IS MANDATORY

Every entry in `files_created` MUST be an object with BOTH `path` AND `content`:

CORRECT: `{"path": "hello.py", "content": "print('hello')"}`
WRONG: `"hello.py"` (string path without content will be REJECTED)

Include the FULL content of every file you created or modified. No truncation. No placeholders. No "... rest of code here".

---

## INPUT EXAMPLE

You will receive:
```json
{
  "task_id": "T001",
  "task_number": "T001",
  "prompt_packet": "# TASK: T001\n\n## What to Build\nCreate hello.py that prints 'Hello World'",
  "expected_output": {"task_id": "T001", "files_created": [...]}
}
```

---

## RULES

- Stay in ONE session - don't exit early
- Fix issues before completing
- Output ONLY JSON at the end - no prose, no markdown wrapper, no code fences around the JSON
- Follow prompt_packet exactly
- Include model_note stating what model/platform you are
- Every file you create MUST appear in files_created with full content

---

## BEGIN

Execute the task completely. Verify your work. Output JSON result.
