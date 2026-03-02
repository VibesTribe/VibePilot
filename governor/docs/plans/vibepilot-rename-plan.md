# PLAN: VibePilot Dashboard Rename

## Overview
Text-only rebranding of all dashboard references to "VibePilot" naming. Zero visual or functional impact - purely string replacement across codebase, config, and documentation.

## Summary
- **Total Tasks:** 8
- **Critical Path:** T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008
- **Estimated Context:** ~5,000 tokens per task
- **All tasks can run in parallel** (no code dependencies between text replacements)

---

## Tasks

### T001: Update Source Code String Literals
**Confidence:** 0.98
**Dependencies:** none
**Type:** refactoring
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T001 - Update Source Code String Literals

## CONTEXT
This is a text-only rebranding task. Update all string literals in source code files that reference old naming conventions to use "VibePilot" branding. No functional changes - only text replacement.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Search and replace all string literals containing old naming references with "VibePilot" equivalent:
- Window titles
- Console logs
- Error messages
- Default values
- Hardcoded strings visible to users

## FILES TO MODIFY
- All `.ts`, `.tsx`, `.js`, `.jsx` files in `/src/` directory
- Focus on string literals only (not variable names, function names, or file names)

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/JavaScript
- Framework: React
- Use grep/ripgrep to find all occurrences
- Case-sensitive replacements where appropriate
- Preserve original capitalization style (e.g., "VibePilot" not "vibepilot" in titles)

## ACCEPTANCE CRITERIA
- [ ] All string literals with old naming replaced
- [ ] No functional code changes
- [ ] All tests still pass
- [ ] No references to old naming in string literals

## TESTS REQUIRED
Run existing test suite to verify no breaking changes:
1. `npm test` or `npm run test` - all tests pass
2. Manual grep to verify no old naming in strings

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["list of files modified"],
  "summary": "Updated X string literals across Y files",
  "tests_written": [],
  "notes": "Any patterns found or edge cases"
}
```

## DO NOT
- Change variable names or function names
- Change file names
- Change API endpoints
- Add new features
- Modify test logic
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["src/**/*.ts", "src/**/*.tsx"],
  "tests_required": ["existing test suite"]
}
```

---

### T002: Update Source Code Comments
**Confidence:** 0.98
**Dependencies:** none
**Type:** refactoring
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T002 - Update Source Code Comments

## CONTEXT
Update all code comments that reference old naming to use "VibePilot" branding. Comments-only changes - no code modifications.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Search and replace naming references in:
- Single-line comments (//)
- Multi-line comments (/* */)
- JSDoc comments
- TODO/FIXME comments

## FILES TO MODIFY
- All `.ts`, `.tsx`, `.js`, `.jsx` files in `/src/` directory
- Comment sections only

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/JavaScript
- Preserve comment formatting and indentation
- Maintain any links or references

## ACCEPTANCE CRITERIA
- [ ] All comments with old naming updated
- [ ] No code logic changes
- [ ] Comment formatting preserved

## TESTS REQUIRED
1. Grep verification: no old naming in comments
2. Existing test suite passes

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["list of files modified"],
  "summary": "Updated X comments across Y files",
  "tests_written": [],
  "notes": "Any patterns found"
}
```

## DO NOT
- Change code logic
- Remove comments
- Add new comments beyond naming updates
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["src/**/*.ts", "src/**/*.tsx"],
  "tests_required": ["existing test suite"]
}
```

---

### T003: Update package.json
**Confidence:** 0.99
**Dependencies:** none
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T003 - Update package.json

## CONTEXT
Update the package.json file to reflect VibePilot branding in name, description, and metadata fields.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update these fields in package.json:
- name: Update to vibepilot-dashboard or similar
- description: Update description text
- author: If applicable
- repository: If url needs updating
- Any other metadata fields with old naming

## FILES TO MODIFY
- `package.json` (root level)

## TECHNICAL SPECIFICATIONS
- Valid JSON format required
- Preserve all dependencies and scripts
- Keep version number unchanged

## ACCEPTANCE CRITERIA
- [ ] package.json name updated
- [ ] description reflects VibePilot branding
- [ ] Valid JSON (can be parsed)
- [ ] All scripts still work

## TESTS REQUIRED
1. `npm install` runs successfully
2. JSON is valid (parse check)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T003",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["package.json"],
  "summary": "Updated package.json with VibePilot branding",
  "tests_written": [],
  "notes": "Fields updated: [list]"
}
```

## DO NOT
- Change version number
- Modify dependencies
- Change scripts unless naming-related
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["package.json"],
  "tests_required": []
}
```

---

### T004: Update Environment Variables and Config Files
**Confidence:** 0.98
**Dependencies:** none
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T004 - Update Environment Variables and Config Files

## CONTEXT
Update configuration files and environment variable references to use VibePilot naming where user-facing.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update naming references in:
- .env.example (comments and variable descriptions)
- config files (vite.config, tsconfig descriptions if any)
- Any .env template files

Note: Do NOT change actual environment variable names (would be breaking). Only update comments and descriptions.

## FILES TO MODIFY
- `.env.example`
- `vite.config.ts` (comments only)
- `tsconfig.json` (comments if any)
- Other config files with naming references

## TECHNICAL SPECIFICATIONS
- Preserve all actual configuration values
- Only update comments and descriptions
- Keep file formats valid

## ACCEPTANCE CRITERIA
- [ ] Config file comments updated
- [ ] .env.example comments updated
- [ ] All config files still valid/parseable
- [ ] No actual config values changed

## TESTS REQUIRED
1. Config files parse correctly
2. Build still works: `npm run build`

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T004",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["list of config files"],
  "summary": "Updated config files with VibePilot naming",
  "tests_written": [],
  "notes": "Files updated and what changed"
}
```

## DO NOT
- Change actual environment variable names
- Change configuration values
- Break existing setups
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [".env.example", "vite.config.ts"],
  "tests_required": []
}
```

---

### T005: Update README Files
**Confidence:** 0.99
**Dependencies:** none
**Type:** documentation
**Category:** documentation
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T005 - Update README Files

## CONTEXT
Update all README markdown files to use VibePilot branding consistently.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update all README files:
- Main README.md
- Any subdirectory README files
- Update titles, descriptions, examples
- Update installation instructions if they reference old naming
- Update any code examples that show old naming

## FILES TO MODIFY
- `README.md` (root)
- `docs/README.md` if exists
- Any other README.md files in subdirectories

## TECHNICAL SPECIFICATIONS
- Markdown format
- Preserve all links
- Keep code blocks functional
- Maintain document structure

## ACCEPTANCE CRITERIA
- [ ] All README files use VibePilot naming
- [ ] Links still work
- [ ] Code examples updated
- [ ] Markdown renders correctly

## TESTS REQUIRED
1. Markdown syntax check
2. Link verification (if automated available)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T005",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["README.md", "other READMEs"],
  "summary": "Updated X README files with VibePilot branding",
  "tests_written": [],
  "notes": "Sections updated"
}
```

## DO NOT
- Change file structure
- Remove content
- Break existing links
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["README.md"],
  "tests_required": []
}
```

---

### T006: Update Inline Documentation
**Confidence:** 0.98
**Dependencies:** none
**Type:** documentation
**Category:** documentation
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T006 - Update Inline Documentation

## CONTEXT
Update inline documentation files (separate from code comments) such as guides, changelogs, and help text.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update naming in:
- docs/ directory markdown files
- CHANGELOG.md
- CONTRIBUTING.md
- Any help text files
- User guide documents

## FILES TO MODIFY
- `CHANGELOG.md`
- `CONTRIBUTING.md`
- `docs/*.md` files
- Any other documentation files

## TECHNICAL SPECIFICATIONS
- Markdown format
- Preserve dates and version references
- Keep formatting consistent

## ACCEPTANCE CRITERIA
- [ ] All doc files use VibePilot naming
- [ ] No broken links
- [ ] Formatting preserved

## TESTS REQUIRED
1. Markdown syntax check
2. File existence verification

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T006",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["list of doc files"],
  "summary": "Updated X documentation files",
  "tests_written": [],
  "notes": "Files and sections updated"
}
```

## DO NOT
- Remove historical changelog entries
- Change dates or versions
- Delete documentation
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["CHANGELOG.md", "CONTRIBUTING.md", "docs/**/*.md"],
  "tests_required": []
}
```

---

### T007: Update Page Titles and Headers
**Confidence:** 0.98
**Dependencies:** none
**Type:** feature
**Category:** ui
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T007 - Update Page Titles and Headers

## CONTEXT
Update all page titles and header text in the React application to use VibePilot branding.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update:
- `<title>` tags (in index.html and dynamically set)
- `<h1>`, `<h2>`, etc. header elements
- Page component titles
- Browser tab titles
- Any title props passed to components

## FILES TO MODIFY
- `index.html` (title tag)
- `src/**/*.tsx` (header elements and title props)
- Layout components
- Page components

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/React
- Framework: React
- Preserve all JSX structure
- Keep dynamic title setting logic intact

## ACCEPTANCE CRITERIA
- [ ] All page titles show VibePilot
- [ ] All headers use VibePilot naming
- [ ] Browser tab titles updated
- [ ] No layout changes

## TESTS REQUIRED
1. Visual verification (manual or snapshot)
2. Existing test suite passes
3. Check page renders correctly

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T007",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["index.html", "src/**/*.tsx"],
  "summary": "Updated page titles and headers across X files",
  "tests_written": [],
  "notes": "Components modified"
}
```

## DO NOT
- Change component structure
- Modify styling
- Change functionality
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["index.html", "src/**/*.tsx"],
  "tests_required": ["existing test suite"]
}
```

---

### T008: Update Labels and UI Text
**Confidence:** 0.98
**Dependencies:** none
**Type:** feature
**Category:** ui
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T008 - Update Labels and UI Text

## CONTEXT
Update all remaining UI text elements - labels, buttons, placeholders, tooltips, and any other user-facing text.

## DEPENDENCIES
None. This task is independent.

## WHAT TO BUILD
Update text in:
- Button labels
- Input placeholders
- Form labels
- Tooltips
- Error messages displayed to users
- Success messages
- Info text
- Any aria-labels referencing old naming

## FILES TO MODIFY
- `src/**/*.tsx` (all React components)
- Any i18n/translation files if they exist
- Constant files with UI text

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/React
- Preserve all JSX structure
- Keep accessibility attributes intact
- Maintain text formatting

## ACCEPTANCE CRITERIA
- [ ] All button text updated
- [ ] All placeholders updated
- [ ] All labels updated
- [ ] All user messages updated
- [ ] Accessibility labels updated
- [ ] No functional changes

## TESTS REQUIRED
1. Existing test suite passes
2. UI text assertions updated if needed
3. Accessibility check (aria-labels)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T008",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["src/**/*.tsx"],
  "summary": "Updated UI text across X components",
  "tests_written": [],
  "notes": "Text types updated: [labels, buttons, etc.]"
}
```

## DO NOT
- Change component logic
- Modify styling or layout
- Change event handlers
- Alter form validation logic
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["src/**/*.tsx"],
  "tests_required": ["existing test suite"]
}
```

---

## Critical Path
T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008

**Note:** All tasks are independent and can run in parallel. The critical path shown is sequential but not required.

## Final Verification
After all tasks complete:
1. Run full test suite: `npm test`
2. Run build: `npm run build`
3. Manual smoke test of dashboard
4. Grep entire codebase for any remaining old naming references
