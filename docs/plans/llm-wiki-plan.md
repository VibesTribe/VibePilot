# PLAN: LLM Wiki — Knowledge Base for Large Language Models
## Overview
Create a static site for LLM reference with model directory, detail pages, search, and provider listings.
## Tasks
### T001: Create Model Data Structure
**Confidence:** 0.99
**Category:** data
**Dependencies:** none
#### Prompt Packet
```
# TASK: T001 - Model Data Structure
## Context
Define JSON schema for LLM models per PRD specs.
## What to Build
Create models.json with fields: id, name, provider, params, context_window, open_source, release_date, pricing, benchmarks, strengths, weaknesses, access_options.
## Files
- `data/models.json` - Core model data
```
#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["data/models.json"],
  "tests_required": ["schema_validation.js"]
}
```
### T002: Implement Model Directory UI
**Confidence:** 0.96
**Category:** frontend
**Dependencies:** T001
#### Prompt Packet
```
# TASK: T002 - Model Directory UI
## Context
Build interactive grid/list for model browsing.
## What to Build
- React component with filtering (provider, open_source) and sorting (release_date, params).
- Responsive design with mobile support.
## Files
- `src/components/ModelDirectory.jsx` - Main UI component
```
#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["src/components/ModelDirectory.jsx"],
  "tests_required": ["directory_snapshot.test.js"]
}
```
### T003: Develop Model Detail Pages
**Confidence:** 0.97
**Category:** frontend
**Dependencies:** T001
#### Prompt Packet
```
# TASK: T003 - Model Detail Pages
## Context
Display detailed model specs and benchmarks.
## What to Build
- Dynamic page template using URL parameters.
- Render architecture, pricing, and benchmark tables.
## Files
- `src/pages/models/[id].jsx` - Detail page template
```
#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["src/pages/models/[id].jsx"],
  "tests_required": ["detail_page_e2e.test.js"]
}
```
### T004: Create Provider Data Structure
**Confidence:** 0.99
**Category:** data
**Dependencies:** none
#### Prompt Packet
```
# TASK: T004 - Provider Data Structure
## Context
Define JSON schema for LLM providers.
## What to Build
Create providers.json with fields: name, models, pricing_structure, free_tier, api_quality.
## Files
- `data/providers.json` - Provider listings
```
#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["data/providers.json"],
  "tests_required": ["provider_schema_check.js"]
}
```
### T005: Implement Provider Directory
**Confidence:** 0.95
**Category:** frontend
**Dependencies:** T004
#### Prompt Packet
```
# TASK: T005 - Provider Directory
## Context
List LLM providers with key details.
## What to Build
- Simple grid showing provider names, models offered, and free tier status.
## Files
- `src/components/ProviderList.jsx` - Provider UI component
```
#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": ["src/components/ProviderList.jsx"],
  "tests_required": ["provider_render.test.js"]
}
```
### T006: Add Search Functionality
**Confidence:** 0.96
**Category:** frontend
**Dependencies:** T001, T004
#### Prompt Packet
```
# TASK: T006 - Search Implementation
## Context
Enable instant search across models/providers.
## What to Build
- Client-side search using Fuse.js with custom weights for name/provider.
## Files
- `src/utils/search.js` - Search logic
```
#### Expected Output
```json
{
  "task_id": "T006",
  "files_created": ["src/utils/search.js"],
  "tests_required": ["search_query.test.js"]
}
```
### T007: Build Comparison Tool
**Confidence:** 0.95
**Category:** frontend
**Dependencies:** T001
#### Prompt Packet
```
# TASK: T007 - Model Comparison
## Context
Enable side-by-side model comparisons.
## What to Build
- Select 2-4 models and display differences in benchmarks, pricing, and context.
## Files
- `src/components/ModelCompare.jsx` - Comparison UI
```
#### Expected Output
```json
{
  "task_id": "T007",
  "files_created": ["src/components/ModelCompare.jsx"],
  "tests_required": ["comparison_snapshot.test.js"]
}
```
### T008: Configure Deployment
**Confidence:** 0.99
**Category:** devops
**Dependencies:** T001-T007
#### Prompt Packet
```
# TASK: T008 - Deployment Setup
## Context
Deploy static site to hosting provider.
## What to Build
- GitHub Pages deployment via CI/CD.
## Files
- `.github/workflows/deploy.yml` - Deployment workflow
```
#### Expected Output
```json
{
  "task_id": "T008",
  "files_created": [".github/workflows/deploy.yml"],
  "tests_required": []
}
```
### T009: Ensure Accessibility & Offline Support
**Confidence:** 0.98
**Category:** frontend
**Dependencies:** T002, T003, T005, T007
#### Prompt Packet
```
# TASK: T009 - Accessibility/Offline
## Context
Meet WCAG AA and offline requirements.
## What to Build
- Implement service worker for caching.
- Add ARIA labels and color contrast checks.
## Files
- `src/sw.js` - Service worker
```
#### Expected Output
```json
{
  "task_id": "T009",
  "files_created": ["src/sw.js"],
  "tests_required": ["accessibility_audit.md"]
}
```
### T010: Apply Licensing
**Confidence:** 0.99
**Category:** legal
**Dependencies:** none
#### Prompt Packet
```
# TASK: T010 - Licensing
## Context
Ensure compliant open-source license.
## What to Build
- Add MIT license to repository.
## Files
- `LICENSE` - MIT license file
```
#### Expected Output
```json
{
  "task_id": "T010",
  "files_created": ["LICENSE"],
  "tests_required": []
}
```
