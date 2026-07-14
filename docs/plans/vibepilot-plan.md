# PLAN: VibePilot PIF System Review

## Overview
Review of the Project Isolation Framework (PIF) implementation to identify gaps, hardcoded assumptions, race conditions, and provide recommendations for hardening.

## Tasks

### T001: Review PIF System and Write Assessment Report
**Confidence:** 0.96
**Category:** analysis
**Dependencies:** none
**Target Files:** knowledgebase/pif-assessment.md

#### Prompt Packet
# TASK: T001 - Review PIF System and Write Assessment Report

## Context
The Project Isolation Framework (PIF) has been implemented across the system. This task requires reviewing the PIF implementation to identify any remaining gaps, hardcoded assumptions, race conditions, or potential issues, and writing an assessment report. No code changes are required - only analysis and documentation.

## What to Build
Create a markdown assessment report at `knowledgebase/pif-assessment.md` that covers:
1. Any data isolation gaps still present in the PIF implementation (examine how project_id is used across database tables, API endpoints, etc.)
2. Any hardcoded VibePilot assumptions that would break for a new project (look for literal strings, hardcoded IDs, project-specific configurations)
3. Any race conditions or edge cases in project_id propagation (check how project_id is passed between components, async operations, etc.)
4. Recommendations for hardening the PIF system (specific, actionable suggestions to improve robustness)

The review should specifically examine:
- server.go: Look at the batch endpoint implementation for project_id handling
- KB server: Check how project filtering is implemented for knowledge base access
- Hermes project system: Review how projects are isolated in the Hermes agent system
- pif_scaffold.py: Verify the scaffolding script properly sets up project isolation

## Files
- `knowledgebase/pif-assessment.md` - The assessment report to be created (this is the only file to create)

#### Expected Output
{
  "files_created": ["knowledgebase/pif-assessment.md"],
  "tests_written": []
}
",
  "tasks": [
    {
      "task_number": "T001",
      "title": "Review PIF System and Write Assessment Report",
      "category": "analysis",
      "confidence": 0.96,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Review PIF System and Write Assessment Report

## Context
The Project Isolation Framework (PIF) has been implemented across the system. This task requires reviewing the PIF implementation to identify any remaining gaps, hardcoded assumptions, race conditions, or potential issues, and writing an assessment report. No code changes are required - only analysis and documentation.

## What to Build
Create a markdown assessment report at `knowledgebase/pif-assessment.md` that covers:
1. Any data isolation gaps still present in the PIF implementation (examine how project_id is used across database tables, API endpoints, etc.)
2. Any hardcoded VibePilot assumptions that would break for a new project (look for literal strings, hardcoded IDs, project-specific configurations)
3. Any race conditions or edge cases in project_id propagation (check how project_id is passed between components, async operations, etc.)
4. Recommendations for hardening the PIF system (specific, actionable suggestions to improve robustness)

The review should specifically examine:
- server.go: Look at the batch endpoint implementation for project_id handling
- KB server: Check how project filtering is implemented for knowledge base access
- Hermes project system: Review how projects are isolated in the Hermes agent system
- pif_scaffold.py: Verify the scaffolding script properly sets up project isolation

## Files
- `knowledgebase/pif-assessment.md` - The assessment report to be created (this is the only file to create)",
      "expected_output": {
        "files_created": ["knowledgebase/pif-assessment.md"],
        "tests_written": []
      }
    }
  ],
  "total_tasks": 1,
  "status": "review