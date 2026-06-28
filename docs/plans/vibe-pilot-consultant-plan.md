# PLAN: VibePilot Consultant Agent - Phase 0: Constraints 

## Overview
This plan focuses on extracting constraints and the operating envelope for the VibePilot Consultant Agent, as per Phase 0 of its development. This phase is crucial for establishing immutable context for all subsequent phases of the agent's operation.

## Tasks

### T001: Extract Hard Constraints
**Confidence:** 0.98
**Category:** requirements gathering
**Dependencies:** none
**Target Files:** PRD/constraints.md

#### Prompt Packet
```
# TASK: T001 - Extract Hard Constraints

## Context
This task is the first step in defining the operating envelope for the VibePilot Consultant Agent. Hard constraints are non-negotiable requirements that must be adhered to throughout the agent's lifecycle.

## What to Build
As the Consultant Agent, carefully review the provided app idea (from the PRD content input) and identify any **hard constraints**. These are absolute limitations or non-negotiable requirements. Examples include:

*   **Performance Requirements:** "The system must respond within 500ms."
*   **Security Mandates:** "All data must be encrypted at rest and in transit."
*   **Regulatory Compliance:** "Must adhere to GDPR standards."
*   **Technology Stacks:** "Must use Go for backend services."
*   **Integration Requirements:** "Must integrate with existing XYZ API."

**Do NOT include soft preferences or suggestions at this stage.** Focus solely on what is absolutely required and cannot be compromised.

Format your findings as a list of clear, concise statements. If no hard constraints are identified from the provided input, state "No hard constraints identified."

## Files
- `PRD/constraints.md` - This file will contain the extracted hard constraints.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["PRD/constraints.md"],
  "tests_written": []
}
```

### T002: Extract Soft Preferences
**Confidence:** 0.98
**Category:** requirements gathering
**Dependencies:** T001
**Target Files:** PRD/preferences.md

#### Prompt Packet
```
# TASK: T002 - Extract Soft Preferences

## Context
Following the identification of hard constraints, this task focuses on capturing soft preferences. These are desirable attributes or recommendations that are not strictly mandatory but would enhance the app idea.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **soft preferences**. These are desirable but not essential aspects. Examples include:

*   **User Experience Goals:** "A clean and intuitive user interface is preferred."
*   **Scalability Suggestions:** "Consider a microservices architecture for future scalability."
*   **Maintainability Aspects:** "Prefer well-documented code."
*   **Optional Integrations:** "Integration with Slack for notifications would be beneficial."

These preferences should be clearly distinguished from hard constraints identified in T001.

Format your findings as a list of clear, concise statements. If no soft preferences are identified, state "No soft preferences identified."

## Files
- `PRD/preferences.md` - This file will contain the extracted soft preferences.
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["PRD/preferences.md"],
  "tests_written": []
}
```

### T003: Extract Forbidden Patterns
**Confidence:** 0.98
**Category:** requirements gathering
**Dependencies:** T001, T002
**Target Files:** PRD/forbidden_patterns.md

#### Prompt Packet
```
# TASK: T003 - Extract Forbidden Patterns

## Context
This task identifies specific patterns or approaches that should be actively avoided in the development of the app idea. This is crucial for preventing architectural missteps or undesirable outcomes.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **forbidden patterns**. These are specific practices, technologies, or architectural styles that must NOT be used. Examples include:

*   **Anti-patterns:** "Avoid monolithic architectures."
*   **Specific Technologies:** "Do not use Flash for any part of the application."
*   **Security Vulnerabilities:** "Prevent the use of deprecated cryptographic algorithms."
*   **Performance Bottlenecks:** "Avoid synchronous operations that could block the main thread."

Clearly state what should be avoided and, if possible, briefly explain why (e.g., "Avoid storing sensitive user data in plain text due to security risks.").

Format your findings as a list of clear, concise statements. If no forbidden patterns are identified, state "No forbidden patterns identified."

## Files
- `PRD/forbidden_patterns.md` - This file will contain the identified forbidden patterns.
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["PRD/forbidden_patterns.md"],
  "tests_written": []
}
```

### T004: Extract Reversibility Requirements
**Confidence:** 0.98
**Category:** requirements gathering
**Dependencies:** T001, T002, T003
**Target Files:** PRD/reversibility.md

#### Prompt Packet
```
# TASK: T004 - Extract Reversibility Requirements

## Context
This task focuses on understanding any specific requirements or preferences related to the reversibility of actions or states within the application. This is important for features like undo/redo, state rollback, or audit trails.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **reversibility requirements**. This includes any explicit needs for undo functionality, state rollback capabilities, or requirements for immutability and auditability.

Examples include:

*   "Users must be able to undo their last 5 actions."
*   "The system must maintain an immutable log of all changes."
*   "Data must be recoverable to a previous state within the last 24 hours."
*   "The application should support a 'soft delete' mechanism for data."

If no specific reversibility requirements are mentioned, state "No specific reversibility requirements identified."

Format your findings as a list of clear, concise statements.

## Files
- `PRD/reversibility.md` - This file will contain the extracted reversibility requirements.
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["PRD/reversibility.md"],
  "tests_written": []
}
```
",
  "tasks": [
    {
      "task_id": "T001",
      "title": "Extract Hard Constraints",
      "category": "requirements gathering",
      "confidence": 0.98,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Extract Hard Constraints

## Context
This task is the first step in defining the operating envelope for the VibePilot Consultant Agent. Hard constraints are non-negotiable requirements that must be adhered to throughout the agent's lifecycle.

## What to Build
As the Consultant Agent, carefully review the provided app idea (from the PRD content input) and identify any **hard constraints**. These are absolute limitations or non-negotiable requirements. Examples include:

*   **Performance Requirements:** "The system must respond within 500ms."
*   **Security Mandates:** "All data must be encrypted at rest and in transit."
*   **Regulatory Compliance:** "Must adhere to GDPR standards."
*   **Technology Stacks:** "Must use Go for backend services."
*   **Integration Requirements:** "Must integrate with existing XYZ API."

**Do NOT include soft preferences or suggestions at this stage.** Focus solely on what is absolutely required and cannot be compromised.

Format your findings as a list of clear, concise statements. If no hard constraints are identified from the provided input, state "No hard constraints identified."

## Files
- `PRD/constraints.md` - This file will contain the extracted hard constraints.
",
      "expected_output": {
        "files_created": [
          "PRD/constraints.md"
        ],
        "tests_written": []
      }
    },
    {
      "task_id": "T002",
      "title": "Extract Soft Preferences",
      "category": "requirements gathering",
      "confidence": 0.98,
      "dependencies": [
        "T001"
      ],
      "prompt_packet": "# TASK: T002 - Extract Soft Preferences

## Context
Following the identification of hard constraints, this task focuses on capturing soft preferences. These are desirable attributes or recommendations that are not strictly mandatory but would enhance the app idea.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **soft preferences**. These are desirable but not essential aspects. Examples include:

*   **User Experience Goals:** "A clean and intuitive user interface is preferred."
*   **Scalability Suggestions:** "Consider a microservices architecture for future scalability."
*   **Maintainability Aspects:** "Prefer well-documented code."
*   **Optional Integrations:** "Integration with Slack for notifications would be beneficial."

These preferences should be clearly distinguished from hard constraints identified in T001.

Format your findings as a list of clear, concise statements. If no soft preferences are identified, state "No soft preferences identified."

## Files
- `PRD/preferences.md` - This file will contain the extracted soft preferences.
",
      "expected_output": {
        "files_created": [
          "PRD/preferences.md"
        ],
        "tests_written": []
      }
    },
    {
      "task_id": "T003",
      "title": "Extract Forbidden Patterns",
      "category": "requirements gathering",
      "confidence": 0.98,
      "dependencies": [
        "T001",
        "T002"
      ],
      "prompt_packet": "# TASK: T003 - Extract Forbidden Patterns

## Context
This task identifies specific patterns or approaches that should be actively avoided in the development of the app idea. This is crucial for preventing architectural missteps or undesirable outcomes.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **forbidden patterns**. These are specific practices, technologies, or architectural styles that must NOT be used. Examples include:

*   **Anti-patterns:** "Avoid monolithic architectures."
*   **Specific Technologies:** "Do not use Flash for any part of the application."
*   **Security Vulnerabilities:** "Prevent the use of deprecated cryptographic algorithms."
*   **Performance Bottlenecks:** "Avoid synchronous operations that could block the main thread."

Clearly state what should be avoided and, if possible, briefly explain why (e.g., "Avoid storing sensitive user data in plain text due to security risks.").

Format your findings as a list of clear, concise statements. If no forbidden patterns are identified, state "No forbidden patterns identified."

## Files
- `PRD/forbidden_patterns.md` - This file will contain the identified forbidden patterns.
",
      "expected_output": {
        "files_created": [
          "PRD/forbidden_patterns.md"
        ],
        "tests_written": []
      }
    },
    {
      "task_id": "T004",
      "title": "Extract Reversibility Requirements",
      "category": "requirements gathering",
      "confidence": 0.98,
      "dependencies": [
        "T001",
        "T002",
        "T003"
      ],
      "prompt_packet": "# TASK: T004 - Extract Reversibility Requirements

## Context
This task focuses on understanding any specific requirements or preferences related to the reversibility of actions or states within the application. This is important for features like undo/redo, state rollback, or audit trails.

## What to Build
As the Consultant Agent, review the provided app idea (from the PRD content input) and identify any **reversibility requirements**. This includes any explicit needs for undo functionality, state rollback capabilities, or requirements for immutability and auditability.

Examples include:

*   "Users must be able to undo their last 5 actions."
*   "The system must maintain an immutable log of all changes."
*   "Data must be recoverable to a previous state within the last 24 hours."
*   "The application should support a 'soft delete' mechanism for data."

If no specific reversibility requirements are mentioned, state "No specific reversibility requirements identified."

Format your findings as a list of clear, concise statements.

## Files
- `PRD/reversibility.md` - This file will contain the extracted reversibility requirements.
",
      "expected_output": {
        "files_created": [
          "PRD/reversibility.md"
        ],
        "tests_written": []
      }
    }
  ],
  "total_tasks": 4,
  "status": "review