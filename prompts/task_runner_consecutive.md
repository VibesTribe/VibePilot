# CONSECUTIVE TASK RUNNER

Execute task, review output, run tests, and report - all in ONE session.

---

## STAGE 1: EXECUTION

Read the prompt_packet and build exactly what it specifies.

Create files, write code, follow all requirements.

Output your work as:
```
=== EXECUTION COMPLETE ===
Files created:
- path/to/file1.ext
- path/to/file2.ext

=== END EXECUTION ===
```

---

## STAGE 2: SELF-REVIEW

Review what you just built:

**Quality Checks:**
- [ ] All required files created
- [ ] Code matches prompt_packet requirements
- [ ] No hardcoded secrets
- [ ] Code is clean and functional

**If any check fails:**
```
=== REVIEW FAILED ===
Issues:
- What went wrong
- How to fix it

=== FIXING NOW ===
[Fix the issues and redo Stage 1]
```

**If all checks pass:**
```
=== REVIEW PASSED ===
```

---

## STAGE 3: TESTING

Run tests or validation:

1. If Go code: Run `go build` to check compilation
2. If tests exist: Run test command
3. If executable: Run it and verify output

**Test Results:**
```
=== TEST RESULTS ===
Build: PASS/FAIL
Tests: PASS/FAIL
Output: [actual output if applicable]

=== END TESTS ===
```

**If tests fail:**
```
=== TESTS FAILED ===
Issues: [what failed]

=== FIXING NOW ===
[Fix the issues and redo from Stage 1]
```

**If tests pass:**
```
=== ALL TESTS PASSED ===
```

---

## STAGE 4: FINAL REPORT

After successful execution, review, and testing:

```json
{
  "task_id": "T001",
  "task_number": "T001",
  "execution": {
    "status": "complete",
    "files_created": [
      {
        "path": "governor/cmd/tools/hello_vibepilot_v2.go",
        "lines": 10
      }
    ],
    "files_modified": [],
    "summary": "Built Go hello world program"
  },
  "review": {
    "status": "passed",
    "checks": {
      "all_files_created": true,
      "requirements_met": true,
      "no_secrets": true,
      "code_quality": "good"
    }
  },
  "testing": {
    "status": "passed",
    "build_result": "PASS",
    "test_result": "PASS",
    "runtime_output": "Hello from VibePilot v2!"
  },
  "overall": {
    "result": "SUCCESS",
    "confidence": 0.99,
    "ready_to_merge": true
  }
}
```

---

## WORKFLOW RULES

1. **Stay in one session** - Don't exit, continue through all stages
2. **Fix failures** - If review or tests fail, fix and retry
3. **Output progress** - Show each stage completion marker
4. **Final JSON only** - End with valid JSON (no markdown wrapper)

---

## INPUT FORMAT

You'll receive:
```json
{
  "task_id": "T001",
  "task_number": "T001",
  "prompt_packet": "# TASK: T001...\n\n## What to Build\n...",
  "expected_output": {...}
}
```

---

## EXAMPLE FLOW

**Input:** Create hello world Go program

**Stage 1 - Execution:**
```
=== EXECUTION COMPLETE ===
Files created:
- governor/cmd/tools/hello_vibepilot_v2.go
Content: [show file]
=== END EXECUTION ===
```

**Stage 2 - Review:**
```
=== REVIEW PASSED ===
All files created ✓
Requirements met ✓
No secrets ✓
=== END REVIEW ===
```

**Stage 3 - Testing:**
```
=== TEST RESULTS ===
Build: PASS
Compiled successfully
Output: Hello from VibePilot v2!
=== ALL TESTS PASSED ===
```

**Stage 4 - Final Report:**
```json
{
  "task_id": "T001",
  "overall": {"result": "SUCCESS", "ready_to_merge": true}
}
```

---

## IMPORTANT

- **Consecutive stages** - Complete all 4 stages in one session
- **Self-correcting** - Fix issues before proceeding
- **Progress markers** - Use the === markers between stages
- **Final JSON** - End with clean JSON object
