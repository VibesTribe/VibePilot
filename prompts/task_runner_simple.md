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

## OUTPUT FORMAT - CRITICAL

End your response with ONLY this JSON. No markdown. No code fences. No explanations before or after.

The `files_created` field MUST be an array of objects with `path` AND `content`:

```json
{
  "task_id": "T001",
  "status": "complete",
  "files_created": [
    {"path": "output/hello.json", "content": "{\"message\": \"Hello, World!\"}"}
  ],
  "summary": "Brief description of what was built"
}
```

### WRONG - DO NOT OUTPUT THIS:
```json
{
  "files_created": ["output/hello.json"]
}
```

That format is REJECTED. String arrays without content are useless. Every file MUST have its full content in the `content` field.

---

## INPUT EXAMPLE

```json
{
  "task_id": "T001",
  "prompt_packet": "Create hello.py that prints 'Hello World'"
}
```

---

## RULES

- Stay in ONE session - don't exit early
- Fix issues before completing
- Output ONLY raw JSON at the end - no ```json``` wrapper, no explanations
- Follow prompt_packet exactly
- Every file in files_created MUST be {"path": "...", "content": "..."}, NOT a string
- File content goes in the JSON output, not just in verification.output

---

## BEGIN

Execute the task completely. Verify your work. Output JSON result.
