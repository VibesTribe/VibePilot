# PLAN: VibePilot Knowledge Graph

## Overview

Create a persistent, agent-queryable knowledge base for VibePilot that stores all research findings, architecture decisions, and system mappings.

## Tasks

### T001: Set up PocketBase Backend
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none
#### Prompt Packet
```markdown
# TASK: T001 - Set up PocketBase Backend
## Context
PocketBase is chosen as the storage solution for the knowledge graph due to its ease of use, REST API, and realtime capabilities.
## What to Build
Set up a PocketBase instance on X220, create the necessary collections (`nodes`, `edges`, `research_entries`, `bookmarks`), and ensure it is accessible via REST API.
## Files
- `pocketbase/config.json` - Configuration file for PocketBase
- `pocketbase/data.json` - Initial data for PocketBase collections
```
#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pocketbase/config.json", "pocketbase/data.json"],
  "tests_written": []
}

### T002: Implement Agent Query Interface
**Confidence:** 0.96
**Category:** coding
**Dependencies:** T001
#### Prompt Packet
```markdown
# TASK: T002 - Implement Agent Query Interface
## Context
Agents need to query the knowledge graph via a simple HTTP API.
## What to Build
Implement REST API endpoints for searching nodes, retrieving node details, and adding research entries.
## Files
- `agents/api.py` - API implementation for agent queries
- `agents/tests.py` - Tests for API endpoints
```
#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["agents/api.py", "agents/tests.py"],
  "tests_written": ["agents/tests.py"]
}

### T003: Develop VibePilot Bookmarklet
**Confidence:** 0.95
**Category:** coding
**Dependencies:** T001
#### Prompt Packet
```markdown
# TASK: T003 - Develop VibePilot Bookmarklet
## Context
A bookmarklet is needed to allow users to easily save URLs and titles to the knowledge graph.
## What to Build
Create a vanilla JavaScript bookmarklet that sends a POST request to the PocketBase API to add a new bookmark.
## Files
- `bookmarklet.js` - Bookmarklet JavaScript code
- `bookmarklet/tests.js` - Tests for bookmarklet functionality
```
#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["bookmarklet.js", "bookmarklet/tests.js"],
  "tests_written": ["bookmarklet/tests.js"]
}

### T004: Implement Council Review Pipeline
**Confidence:** 0.94
**Category:** coding
**Dependencies:** T002
#### Prompt Packet
```markdown
# TASK: T004 - Implement Council Review Pipeline
## Context
All research findings must go through a council review before being presented to humans.
## What to Build
Implement a council review pipeline that allows council members to review, comment, and approve or reject research findings.
## Files
- `council/review.py` - Council review pipeline implementation
- `council/tests.py` - Tests for council review pipeline
```
#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["council/review.py", "council/tests.py"],
  "tests_written": ["council/tests.py"]
}

### T005: Integrate with Existing Dashboard
**Confidence:** 0.92
**Category:** coding
**Dependencies:** T003, T004
#### Prompt Packet
```markdown
# TASK: T005 - Integrate with Existing Dashboard
## Context
The knowledge graph must be integrated into the existing VibePilot dashboard.
## What to Build
Integrate the knowledge graph into the existing dashboard, including a graph view, review dock, and search functionality.
## Files
- `dashboard/graph_view.js` - Graph view implementation
- `dashboard/review_dock.js` - Review dock implementation
- `dashboard/search.js` - Search functionality implementation
```
#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": ["dashboard/graph_view.js", "dashboard/review_dock.js", "dashboard/search.js"],
  "tests_written": []
}

### T006: Implement Git Backup and Restore
**Confidence:** 0.90
**Category:** coding
**Dependencies:** T001
#### Prompt Packet
```markdown
# TASK: T006 - Implement Git Backup and Restore
## Context
The knowledge graph data must be backed up to git and restorable in case of data loss.
## What to Build
Implement a git backup and restore mechanism for the knowledge graph data.
## Files
- `backup/backup.py` - Backup implementation
- `backup/restore.py` - Restore implementation
```
#### Expected Output
```json
{
  "task_id": "T006",
  "files_created": ["backup/backup.py", "backup/restore.py"],
  "tests_written": []
}

## Total Tasks: 6
