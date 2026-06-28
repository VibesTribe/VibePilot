# PLAN: Consultant Agent for VibePilot

## Overview
This plan outlines the process for developing the Consultant Agent, which aims to transform user app ideas into fully-approved PRDs.

## Tasks

### T001: Understand User App Idea
**Confidence:** 0.98
**Category:** prompt_engineering
**Dependencies:** none
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T001 - Understand User App Idea

## Context
The user has an app idea and needs to describe it to the Consultant Agent. The Consultant Agent's role is to help transform this idea into a zero-ambiguity, fully-approved PRD.

## What to Build
Prompt the user to describe their app idea by asking for the following:

1.  **What is the app?** What problem does it solve or what need does it fulfill?
2.  **Who is it for?** (Personal use, a specific business, a broad audience?)
3.  **What are the core features?** What should it be able to do?
4.  **How do you envision it being used?** (e.g., on a phone, web browser, voice-controlled, hands-free?)

Ensure the user's response is logged or stored for subsequent processing.

## Files
- N/A
```

#### Expected Output
```json
{
  "task_id": "T001",
  "user_prompt": "Please describe your app idea focusing on: 1. What is the app? What problem does it solve or what need does it fulfill? 2. Who is it for? (Personal use, a specific business, a broad audience?) 3. What are the core features? What should it be able to do? 4. How do you envision it being used? (e.g., on a phone, web browser, voice-controlled, hands-free?)",
  "user_description": "[User's detailed description of their app idea]"
}
```

---",
  "tasks": [
    {
      "task_id": "T001",
      "title": "Understand User App Idea",
      "category": "prompt_engineering",
      "confidence": 0.98,
      "dependencies": [],
      "target_files": [],
      "prompt_packet": "# TASK: T001 - Understand User App Idea

## Context
The user has an app idea and needs to describe it to the Consultant Agent. The Consultant Agent's role is to help transform this idea into a zero-ambiguity, fully-approved PRD.

## What to Build
Prompt the user to describe their app idea by asking for the following:

1.  **What is the app?** What problem does it solve or what need does it fulfill?
2.  **Who is it for?** (Personal use, a specific business, a broad audience?)
3.  **What are the core features?** What should it be able to do?
4.  **How do you envision it being used?** (e.g., on a phone, web browser, voice-controlled, hands-free?)

Ensure the user's response is logged or stored for subsequent processing.

## Files
- N/A",
      "expected_output": {
        "files_created": [],
        "tests_written": [],
        "user_prompt": "Please describe your app idea focusing on: 1. What is the app? What problem does it solve or what need does it fulfill? 2. Who is it for? (Personal use, a specific business, a broad audience?) 3. What are the core features? What should it be able to do? 4. How do you envision it being used? (e.g., on a phone, web browser, voice-controlled, hands-free?)",
        "user_description": "[User's detailed description of their app idea]"
      }
    }
  ],
  "total_tasks": 1,
  "status": "review