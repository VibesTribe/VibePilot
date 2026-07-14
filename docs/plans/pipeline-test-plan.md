# PLAN: Pipeline Test — Courier Task

## Overview
A single courier task to verify the PIF pipeline fix is working end-to-end.
This task requires NO codebase access — purely web research.

## Tasks

### T001: Research and summarize 3 new features in PostgreSQL 17
**Confidence:** 0.95
**Category:** research
**Dependencies:** none
**Target Files:** knowledgebase/pg17-features.md

#### Prompt Packet
```
# TASK: T001 - Research and summarize 3 new features in PostgreSQL 17

## Context
This task is part of a pipeline test to verify the PIF pipeline fix is working end-to-end.
The goal is to produce a markdown summary of 3 new features in PostgreSQL 17 relevant for application developers.

## What to Build
Research and summarize 3 new features in PostgreSQL 17 that are relevant for application developers.
Save the summary as a markdown file at `knowledgebase/pg17-features.md`.
The summary should contain 3-5 bullet points with brief explanations.

Do NOT modify any existing files. Only create `knowledgebase/pg17-features.md`.

## Files
- `knowledgebase/pg17-features.md` - The output markdown file
```

#### Expected Output
```json
{
  "files_created": ["knowledgebase/pg17-features.md"],
  "tests_written": []
}
```
",
  "tasks": [
    {
      "task_number": "T001",
      "title": "Research and summarize 3 new features in PostgreSQL 17",
      "category": "research",
      "confidence": 0.95,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Research and summarize 3 new features in PostgreSQL 17

## Context
This task is part of a pipeline test to verify the PIF pipeline fix is working end-to-end.
The goal is to produce a markdown summary of 3 new features in PostgreSQL 17 relevant for application developers.

## What to Build
Research and summarize 3 new features in PostgreSQL 17 that are relevant for application developers.
Save the summary as a markdown file at `knowledgebase/pg17-features.md`.
The summary should contain 3-5 bullet points with brief explanations.

Do NOT modify any existing files. Only create `knowledgebase/pg17-features.md`.

## Files
- `knowledgebase/pg17-features.md` - The output markdown file",
      "expected_output": {
        "files_created": ["knowledgebase/pg17-features.md"],
        "tests_written": []
      }
    }
  ],
  "total_tasks": 1,
  "status": "review