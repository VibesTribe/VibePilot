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

End your response with this JSON (no markdown, just raw JSON):

```json
{
  "task_id": "T001",
  "status": "complete",
  "files_created": ["file1.py", "file2.txt"],
  "files_modified": [],
  "verification": {
    "build_status": "PASS",
    "execution_status": "PASS",
    "output": "Actual output if applicable"
  },
  "summary": "Brief description of what was built",
  "model_note": "If you know your model name/version, state it here (e.g., 'gemini-2.5-flash', 'deepseek-chat'). If unsure, write 'unknown'."
}
```

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
- Output ONLY JSON at the end (no markdown wrapper)
- Follow prompt_packet exactly
- Include model_note in your JSON output stating what model/platform you are

---

## BEGIN

Execute the task completely. Verify your work. Output JSON result.
