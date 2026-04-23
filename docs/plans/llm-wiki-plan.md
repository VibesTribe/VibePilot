# PLAN: LLM Wiki — Knowledge Base for Large Language Models

## Overview
A Karpathy-style reference wiki for large language models. Single-page, fast, searchable. The missing manual for every notable LLM — what it is, how it works, what it's good at, what it costs, where to try it.

## Tasks

### T001: Model Directory Grid/List Implementation
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Model Directory Grid/List Implementation

## Context
Create the model directory with grid/list view, filters, and sorting.

## What to Build
Implement a responsive grid/list view for the model directory with the following filters and sorting options:
- Filters: provider, size, open/closed, cost tier, capabilities
- Sort by: release date, parameter count, cost, benchmark scores
Each grid item should display: name, provider, parameter count, context window, pricing, open/closed source

## Files
- `src/components/ModelDirectory.js` - Implement grid/list view and filters
- `src/utils/data.js` - Handle data loading and filtering
```

#### Expected Output
```json
{
  \"files_created\": [\"src/components/ModelDirectory.js\", \"src/utils/data.js\"],
  \"tests_written\": []
}
```

### T002: Model Detail Page Implementation
**Confidence:** 0.97
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Model Detail Page Implementation

## Context
Create detailed pages for each model with architecture summary, training data, benchmark scores, pricing, strengths, weaknesses, and API access options.

## What to Build
Implement a detailed page for each model, including:
- Architecture summary (transformer variant, MoE, etc.)
- Training data highlights (what we know)
- Benchmark scores (MMLU, HumanEval, GPQA, etc.) in a simple table
- Pricing breakdown (input/output per 1M tokens)
- Known strengths and weaknesses (honest, not marketing)
- API access options (which providers offer it)
- Release date and version history
- Links to paper, model card, try-it-now URLs

## Files
- `src/components/ModelDetail.js` - Implement detailed page for each model
- `src/utils/data.js` - Update data handling for model details
```

#### Expected Output
```json
{
  \"files_created\": [\"src/components/ModelDetail.js\"],
  \"tests_written\": []
}
```

### T003: Comparison Tool Implementation
**Confidence:** 0.95
**Category:** coding
**Dependencies:** T001, T002

#### Prompt Packet
```
# TASK: T003 - Comparison Tool Implementation

## Context
Implement a comparison tool for 2-4 models, highlighting differences in benchmarks, pricing, context, and capabilities.

## What to Build
Create a comparison tool that allows users to select 2-4 models and view a side-by-side comparison, including:
- Differences in benchmarks, pricing, context, and capabilities
- Recommendation based on use case

## Files
- `src/components/ComparisonTool.js` - Implement comparison tool
- `src/utils/data.js` - Update data handling for comparison
```

#### Expected Output
```json
{
  \"files_created\": [\"src/components/ComparisonTool.js\"],
  \"tests_written\": []
}
```

### T004: Provider Directory Implementation
**Confidence:** 0.96
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T004 - Provider Directory Implementation

## Context
Create a directory for providers, including what models they offer, pricing structure, and free tier details.

## What to Build
Implement a provider directory, including:
- Each provider (OpenAI, Anthropic, Google, Meta, Mistral, Deepseek, etc.)
- What models they offer, pricing structure, free tier details
- API quality and reliability notes

## Files
- `src/components/ProviderDirectory.js` - Implement provider directory
- `src/utils/data.js` - Update data handling for providers
```

#### Expected Output
```json
{
  \"files_created\": [\"src/components/ProviderDirectory.js\"],
  \"tests_written\": []
}
```

### T005: Search Implementation
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T005 - Search Implementation

## Context
Implement instant search across all models, providers, and capabilities.

## What to Build
Create a search function that allows users to search across all models, providers, and capabilities, including:
- Instant search results
- Natural language search (e.g., \"cheapest model for coding\")

## Files
- `src/components/Search.js` - Implement search function
- `src/utils/data.js` - Update data handling for search
```

#### Expected Output
```json
{
  \"files_created\": [\"src/components/Search.js\"],
  \"tests_written\": []
}
```

### T006: Data Structure and JSON Files Implementation
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T006 - Data Structure and JSON Files Implementation

## Context
Create the data structure for models and providers, and implement JSON files for data storage.

## What to Build
Implement the data structure for models and providers, including:
- Each model entry has: id, name, provider, params, context window, open_source flag, release date, pricing (input/output per 1M tokens), benchmarks (MMLU, HumanEval, GPQA), strengths array, weaknesses array, access options array
- Each provider entry has: id, name, models, pricing structure, free tier details
Create JSON files for data storage: models.json, providers.json

## Files
- `src/utils/data.js` - Implement data structure and JSON files
- `models.json` - Store model data
- `providers.json` - Store provider data
```

#### Expected Output
```json
{
  \"files_created\": [\"models.json\", \"providers.json\"],
  \"tests_written\": []
}
```

### T007: Tech Stack and Deployment Implementation
**Confidence:** 0.97
**Category:** coding
**Dependencies:** T006

#### Prompt Packet
```
# TASK: T007 - Tech Stack and Deployment Implementation

## Context
Implement the tech stack using a static site generator (Vite + React or plain HTML/JS) and deploy on GitHub Pages or Cloudflare Pages.

## What to Build
Implement the tech stack, including:
- Static site generator (Vite + React or plain HTML/JS)
- Deployment on GitHub Pages or Cloudflare Pages

## Files
- `package.json` - Configure dependencies and scripts
- `vite.config.js` - Configure Vite
```

#### Expected Output
```json
{
  \"files_created\": [\"package.json\", \"vite.config.js\"],
  \"tests_written\": []
}
```
