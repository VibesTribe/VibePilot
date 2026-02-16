# Vibeflow Deep Review - Learnings for VibePilot

## What Vibeflow Does Right (Must Preserve)

### 1. Task Packets Are NOT Just Schemas

Task packets have consistent STRUCTURE:
```json
{
  "task_id": "A1.1",
  "title": "Implement Orchestrator runtime",
  "context": "Why this task exists, what it's for",
  "files": ["src/core/orchestrator.ts"],
  "acceptance_criteria": [
    "Specific testable criterion 1",
    "Specific testable criterion 2"
  ]
}
```

Not just a schema - actual template with:
- task_id (structured, not random UUID)
- title (clear, action-oriented)
- context (WHY, not just what)
- files (explicit list)
- acceptance_criteria (testable)

### 2. Skills Are Declarative, Not Hardcoded

```json
{
  "id": "visual_execution",
  "name": "Visual Execution",
  "category": "execution",
  "runner": "./visual_execution.runner.mjs",
  "ok_probe": {
    "command": "node skills/visual_execution.runner.mjs --probe",
    "timeout_seconds": 60
  }
}
```

Skills loaded from `registry.json`. Add skill = edit JSON. No code changes.

Categories:
- `core` - Required, can't delete (dag_executor)
- `execution` - Task execution (visual, text, cli)
- `maintenance` - System updates

### 3. Phases Not Fixed Numbers

Not "20 tasks in 5 phases". Phases are:
- P0: Dashboard & Telemetry Bootstrap
- PA: Core Control & Safety
- PB: Adapters & Skill Bridge
- PC: Validation & Watcher Safety

Tasks grouped by purpose, not count.

### 4. Visual Execution = Couriers

Vibeflow's `visual_execution` skill:
- Uses Browser-Use
- Connects to online AI studios
- Shared Google session for auth
- Returns structured output JSON

This IS the courier concept. Not "fallback when API fails" - it's the PRIMARY way to use web platforms.

### 5. Each Task Self-Verifiable

Every task has:
- Files explicitly listed
- Acceptance criteria testable
- CI validates before merge

No ambiguity. If criteria unclear, task is wrong.

### 6. OK Probes

Every skill has an `ok_probe`:
```json
"ok_probe": {
  "command": "node skills/visual_execution.runner.mjs --probe",
  "timeout_seconds": 60
}
```

Run probe = verify skill works. Not hoping.

---

## What VibePilot Needs (Based on Vibeflow)

### Task Packet Template (Not Just Schema)

```json
{
  "task_id": "T001",
  "title": "Clear action-oriented title",
  "context": "WHY this task exists and what it enables",
  "phase": "P1_Foundation",
  "files_to_create": ["path/to/file.py"],
  "files_to_modify": [],
  "dependencies": ["T000"],
  "acceptance_criteria": [
    "Specific testable criterion 1",
    "Specific testable criterion 2"
  ],
  "prompt_packet": "Full instructions...",
  "routing": {
    "type": "internal | courier",
    "requires_codebase": false,
    "courier_platform": "chatgpt | claude | gemini | any"
  },
  "output_requirements": {
    "must_include": ["task_number", "model_name", "chat_url"],
    "format": "json"
  }
}
```

### Skills Registry (Not Hardcoded Agents)

```json
{
  "skills": [
    {
      "id": "generate_prd",
      "name": "Generate PRD",
      "category": "planning",
      "runner": "python skills/generate_prd.py",
      "ok_probe": {
        "command": "python skills/generate_prd.py --probe",
        "timeout_seconds": 30
      }
    }
  ]
}
```

### Courier Clarification

**Couriers ARE the visual_execution skill from Vibeflow:**
- Go to web AI platform (ChatGPT, Claude, Gemini)
- Submit prompt
- Return: output + chat_url + model_name + task_number
- Use free tiers to learn model strengths
- ROI tracking informs future API/subscription decisions

**Internal = requires_codebase**
**Courier = independent task, web platform**

---

## My Mistakes (What Not To Repeat)

1. **Hardcoded counts** - "20 tasks", "5 workers", "5 phases" - NO
2. **Schema without template** - JSON schema doesn't teach how to write packets
3. **Courier as fallback** - WRONG. Courier is primary for web platforms
4. **Planning before understanding** - Created plan before reviewing Vibeflow

---

## Next Session

1. Create task packet TEMPLATES (not just schema)
2. Create skills registry structure
3. Map existing VibePilot code to skills
4. Identify what needs building (without hardcoding counts)
5. THEN create plan with proper structure

---

**Key Learning:** Vibeflow solves these problems. Study it. Don't reinvent.
