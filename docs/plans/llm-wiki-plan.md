# PLAN: LLM Wiki

## Overview
Static site for LLM model directory, detail pages, and comparison tool.

## Tasks

### T001: Setup Project Structure
**Confidence:** 0.99
**Category:** setup
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Setup Project Structure
## Context
Bootstrap the project using Vite + React for static site generation.

## What to Build
1. Initialize Vite project with React template
2. Configure `vite.config.js` for static asset handling
3. Set up src directory with components, assets, and JSON data folders
4. Install dependencies: react, react-dom, @tanstack/react-table

## Files
- `package.json` - Define dependencies and build scripts
- `vite.config.js` - Add static asset configuration
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["package.json", "vite.config.js"],
  "tests_required": ["npm install && npm run dev"]
}
```

### T002: Create Model JSON Structure
**Confidence:** 0.98
**Category:** data
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Model JSON Structure
## Context
Define the data format for LLM models according to the PRD.

## What to Build
Create `src/data/models.json` with schema:
```
{
  "id": "string",
  "name": "string",
  "provider": "string",
  "params": "number",
  "contextWindow": "number",
  "openSource": "boolean",
  "releaseDate": "string",
  "pricing": {
    "input": "number",
    "output": "number"
  },
  "benchmarks": {
    "mmlu": "number",
    "humanEval": "number",
    "gpqa": "number"
  },
  "strengths": "string[]",
  "weaknesses": "string[]",
  "accessOptions": "string[]"
}
```
Add 3 placeholder entries for testing.

## Files
- `src/data/models.json` - Initial data structure
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["src/data/models.json"],
  "tests_required": ["JSON schema validation"]
}
```

### T003: Create Provider JSON Structure
**Confidence:** 0.97
**Category:** data
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T003 - Create Provider JSON Structure
## Context
Define provider data format with required PRD fields.

## What to Build
Create `src/data/providers.json` with schema:
```
{
  "id": "string",
  "name": "string",
  "models": "string[]",
  "pricingStructure": "string",
  "freeTier": "boolean",
  "apiQualityNotes": "string"
}
```
Add 2 placeholder providers (OpenAI, Anthropic).

## Files
- `src/data/providers.json` - Provider directory data
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["src/data/providers.json"],
  "tests_required": ["provider schema validation"]
}
```

### T004: Build Model Directory UI
**Confidence:** 0.96
**Category:** frontend
**Dependencies:** T001, T002

#### Prompt Packet
```
# TASK: T004 - Build Model Directory UI
## Context
Create interactive grid/list view for model browsing.

## What to Build
1. Component: `ModelCard.jsx` displaying key stats
2. Component: `ModelFilters.jsx` with:
   - Provider filter
   - Open-source toggle
   - Parameter size slider
3. Integrate react-table for sorting (params, date, cost)
4. Style with mobile-responsive CSS grid

## Files
- `src/components/ModelCard.jsx` - Reusable model display
- `src/components/ModelFilters.jsx` - Filtering system
- `src/styles/models.css` - Responsive styling
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["ModelCard.jsx", "ModelFilters.jsx", "models.css"],
  "tests_required": ["filter functionality test"]
}
```

### T005: Implement Model Detail Pages
**Confidence:** 0.95
**Category:** frontend
**Dependencies:** T001, T002

#### Prompt Packet
```
# TASK: T005 - Implement Model Detail Pages
## Context
Dynamic pages for detailed model information.

## What to Build
1. Create `ModelDetail.jsx` with URL param routing
2. Display all fields from models.json:
   - Architecture summary
   - Training data highlights
   - Benchmark table
   - Pricing breakdown
3. Add "back to directory" navigation

## Files
- `src/pages/model/[id].jsx` - Detail page component
- `src/components/BenchmarkTable.jsx` - Reusable table
```

#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": ["[id].jsx", "BenchmarkTable.jsx"],
  "tests_required": ["routing test"]
}
```

### T006: Develop Provider Directory
**Confidence:** 0.95
**Category:** frontend
**Dependencies:** T001, T003

#### Prompt Packet
```
# TASK: T006 - Develop Provider Directory
## Context
Simple provider listing with key details.

## What to Build
1. Component: `ProviderCard.jsx` showing provider name, models offered
2. Page: `ProvidersPage.jsx` with filter by model count
3. Link provider names to detail modals

## Files
- `src/components/ProviderCard.jsx` - Provider display
- `src/pages/providers.jsx` - Directory layout
```

#### Expected Output
```json
{
  "task_id": "T006",
  "files_created": ["ProviderCard.jsx", "providers.jsx"],
  "tests_required": ["provider filter test"]
}
```

### T007: Implement Search Functionality
**Confidence:** 0.96
**Category:** frontend
**Dependencies:** T002, T003

#### Prompt Packet
```
# TASK: T007 - Implement Search Functionality
## Context
Client-side search across models/providers.

## What to Build
1. Integrate ` Fuse.js` for instant search
2. Create search bar component in `App.jsx`
3. Index model/provider names, descriptions, strengths
4. Enable natural language queries (e.g., "open source coding models")

## Files
- `src/components/SearchBar.jsx` - Search interface
- `src/utils/search.js` - Fuse configuration
```

#### Expected Output
```json
{
  "task_id": "T007",
  "files_created": ["SearchBar.jsx", "search.js"],
  "tests_required": ["search relevance test"]
}
```

### T008: Build Comparison Tool UI
**Confidence:** 0.94
**Category:** frontend
**Dependencies:** T002

#### Prompt Packet
```
# TASK: T008 - Build Comparison Tool UI
## Context
Side-by-side model comparison for 2-4 models.

## What to Build
1. Component: `ComparisonTable.jsx` with dynamic columns
2. Feature detection for differences in benchmarks/pricing
3. Recommendation engine based on selected use cases

## Files
- `src/components/ComparisonTable.jsx` - Comparison layout
- `src/utils/comparison.js` - Diff logic
```

#### Expected Output
```json
{
  "task_id": "T008",
  "files_created": ["ComparisonTable.jsx", "comparison.js"],
  "tests_required": ["comparison accuracy test"]
}
```

### T009: Optimize Performance
**Confidence:** 0.97
**Category:** frontend
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T009 - Optimize Performance
## Context
Ensure sub-2s load time on mobile.

## What to Build
1. Image optimization (WebP format)
2. Code splitting for routes
3. Add caching headers to `vite.config.js`
4. Service worker registration for offline access

## Files
- `src/RegisterServiceWorker.js` - Offline caching
- `vite.config.js` - Performance optimizations
```

#### Expected Output
```json
{
  "task_id": "T009",
  "files_created": ["RegisterServiceWorker.js"],
  "tests_required": ["Lighthouse audit"]
}
```

### T010: Audit Accessibility
**Confidence:** 0.95
**Category:** testing
**Dependencies:** T004, T005, T006

#### Prompt Packet
```
# TASK: T010 - Audit Accessibility
## Context
Verify WCAG AA compliance across components.

## What to Build
1. Run Axe accessibility tests
2. Fix contrast ratios in dark mode
3. Add ARIA labels for filter/slider components
4. Implement keyboard navigation for search

## Files
- `src/styles/accessibility.css` - A11y fixes
- `axe-report.html` - Test results
```

#### Expected Output
```json
{
  "task_id": "T010",
  "files_created": ["accessibility.css"],
  "tests_required": ["WCAG compliance report"]
}
```

### T011: Mobile Responsiveness
**Confidence:** 0.96
**Category:** frontend
**Dependencies:** T004, T005, T006, T008

#### Prompt Packet
```
# TASK: T011 - Mobile Responsiveness
## Context
Ensure all layouts work on mobile devices.

## What to Build
1. Add CSS media queries for <768px
2. Convert filters to collapsible drawers
3. Simplify comparison table on mobile
4. Test touch targets (≥44px)

## Files
- `src/styles/mobile.css` - Responsive overrides
- `src/components/MobileFilters.jsx` - Mobile UI
```

#### Expected Output
```json
{
  "task_id": "T011",
  "files_created": ["mobile.css", "MobileFilters.jsx"],
  "tests_required": ["mobile usability test"]
}
```

### T012: License Audit
**Confidence:** 0.99
**Category:** legal
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T012 - License Audit
## Context
Verify all dependencies use MIT/Apache licenses.

## What to Build
1. Run `npm ls --json` > licenses.json
2. Check each package license field
3. Document exceptions (if any)

## Files
- `licenses.json` - Dependency licenses
- `AUDIT_LICENSES.md` - Compliance report
```

#### Expected Output
```json
{
  "task_id": "T012",
  "files_created": ["licenses.json", "AUDIT_LICENSES.md"],
  "tests_required": ["license compliance check"]
}
```

### T013: Populate Initial Data
**Confidence:** 0.95
**Category:** data
**Dependencies:** T002, T003

#### Prompt Packet
```
# TASK: T013 - Populate Initial Data
## Context
Seed models.json with 50+ entries.

## What to Build
1. Research 50 LLMs from public sources
2. Fill `models.json` with accurate data:
   - Parameters
   - Benchmarks
   - Pricing
   - Source links
3. Validate data consistency

## Files
- `src/data/models.json` - 50+ entries
- `DATA_SOURCES.md` - Data provenance documentation
```

#### Expected Output
```json
{
  "task_id": "T013",
  "files_created": ["models.json", "DATA_SOURCES.md"],
  "tests_required": ["data accuracy spot check"]
}
```

### T014: Deploy to GitHub Pages
**Confidence:** 0.98
**Category:** deployment
**Dependencies:** T001, T009, T013

#### Prompt Packet
```
# TASK: T014 - Deploy to GitHub Pages
## Context
Automate static site deployment.

## What to Build
1. Configure `package.json` deploy script:
   ```json
   "deploy": "vite build && vite preview"
   ```
2. Set up GitHub Actions workflow
3. Test deployment to gh-pages branch

## Files
- `.github/workflows/deploy.yml` - CI/CD pipeline
- `README.md` - Deployment instructions
```

#### Expected Output
```json
{
  "task_id": "T014",
  "files_created": ["deploy.yml"],
  "tests_required": ["deployment smoke test"]
}
```

