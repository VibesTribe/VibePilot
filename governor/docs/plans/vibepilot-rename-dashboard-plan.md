# PLAN: VibePilot Dashboard Rename

## Overview
Rename all dashboard references from "vibeflow" to "VibePilot" branding. This is a text-only rebranding with zero visual or functional impact.

**PRD Reference:** `docs/prd/vibepilot-rename-dashboard-prd.md`
**Source Term:** vibeflow (case variants: vibeflow, Vibeflow, VIBEFLOW, Vibe-flow, vibe_flow)
**Target Term:** vibepilot (case-preserved equivalents)

## Task Summary
| Task | Category | Confidence | Dependencies |
|------|----------|------------|--------------|
| T001 | coding | 0.97 | none |
| T002 | configuration | 0.98 | none |
| T003 | documentation | 0.98 | none |
| T004 | ui | 0.97 | none |
| T005 | testing | 0.99 | T001-T004 |
| T006 | verification | 0.96 | T005 |

## P0 Feature Coverage
| P0 Feature | Task(s) |
|------------|---------|
| Code References | T001 |
| Configuration | T002 |
| Documentation | T003 |
| UI Text | T004 |
| Tests Pass | T005 |
| Verification | T006 |

---

## Tasks

### T001: Update Source Code References
**Confidence:** 0.97
**Dependencies:** none
**Type:** refactoring
**Category:** coding
**Requires Codebase:** true

#### Purpose
Update all string literals and comments in source code files that reference "vibeflow" to use "VibePilot" branding.

#### Prompt Packet
```markdown
# TASK: T001 - Update Source Code References

## CONTEXT
You are performing a text rebranding task for VibePilot. Replace all occurrences of "vibeflow" (in any case variant) with "vibepilot" equivalents in source code files. This is TEXT-ONLY - no functional changes.

**Known Scope:** ~455 occurrences across ~36 files. This is expected for a full rebrand.

## WHAT TO BUILD
Perform case-preserving text replacement:
- `vibeflow` → `vibepilot` (lowercase)
- `Vibeflow` → `VibePilot` (title case)
- `VIBEFLOW` → `VIBEPILOT` (uppercase)
- `Vibe-flow` → `Vibe-pilot` (hyphenated)
- `vibe_flow` → `vibe_pilot` (snake_case)

## FILES TO MODIFY
All `.ts`, `.tsx`, `.js`, `.jsx` files containing "vibeflow" references.

### Pre-Step: Identify Files
```bash
grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" -l 2>/dev/null | grep -v node_modules | grep -v ".git"
```

### Replacement Commands
For each file identified:
```bash
# Lowercase
sed -i 's/vibeflow/vibepilot/g' <file>
# Title case
sed -i 's/Vibeflow/VibePilot/g' <file>
# Uppercase
sed -i 's/VIBEFLOW/VIBEPILOT/g' <file>
# Hyphenated
sed -i 's/Vibe-flow/Vibe-pilot/g' <file>
# Snake case
sed -i 's/vibe_flow/vibe_pilot/g' <file>
```

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/JavaScript
- Focus on: string literals, console logs, error messages, comments, JSDoc
- DO NOT change: variable names, function names, file names, API endpoints

## ACCEPTANCE CRITERIA
- [ ] All source code files with vibeflow references updated
- [ ] Case preservation maintained
- [ ] No functional code changes
- [ ] `grep -ri "vibeflow" --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx"` returns zero results (excluding node_modules, .git)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["list of files"],
  "summary": "Updated X occurrences across Y files",
  "tests_written": [],
  "notes": "Patterns found and edge cases"
}
```

## DO NOT
- Change variable names or function names
- Change file names or API endpoints
- Add new features
- Modify test logic
- Stop if many files found (expected for full rebrand)
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["src/**/*.ts", "src/**/*.tsx"],
  "tests_required": ["existing test suite in T005"]
}
```

---

### T002: Update Configuration Files
**Confidence:** 0.98
**Dependencies:** none
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Purpose
Update package.json, environment variable files, and config files to use VibePilot naming.

#### Prompt Packet
```markdown
# TASK: T002 - Update Configuration Files

## CONTEXT
Update configuration files to reflect VibePilot branding. Only update user-facing text (comments, descriptions, metadata). Do NOT change actual environment variable names (would be breaking).

## WHAT TO BUILD
Update naming references in:
1. **package.json** - name, description, metadata
2. **.env.example** - comments and descriptions (NOT variable names)
3. **vite.config.ts** - comments only
4. **Other config files** - any naming references in comments/descriptions

## FILES TO MODIFY
- `package.json`
- `.env.example`
- `vite.config.ts`
- `tsconfig.json` (if has naming in comments)
- Any other config files with naming references

### For package.json
```json
{
  "name": "vibepilot-dashboard",
  "description": "VibePilot Dashboard - [description]"
}
```

### For .env.example
Update comments only:
```bash
# VibePilot API Key (was: Vibeflow API Key)
VITE_API_KEY=your_key_here
```

## TECHNICAL SPECIFICATIONS
- Valid JSON/JS format required
- Preserve all dependencies and scripts
- Keep version number unchanged
- Do NOT change actual environment variable names

## ACCEPTANCE CRITERIA
- [ ] package.json updated with VibePilot naming
- [ ] .env.example comments updated
- [ ] Config file comments updated
- [ ] All files remain valid/parseable
- [ ] No actual config values changed

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["package.json", ".env.example", "vite.config.ts"],
  "summary": "Updated X config files",
  "tests_written": [],
  "notes": "Fields updated"
}
```

## DO NOT
- Change actual environment variable names
- Change configuration values
- Change version number
- Modify dependencies
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["package.json", ".env.example"],
  "tests_required": []
}
```

---

### T003: Update Documentation Files
**Confidence:** 0.98
**Dependencies:** none
**Type:** documentation
**Category:** documentation
**Requires Codebase:** false

#### Purpose
Update all markdown documentation files to use VibePilot naming consistently.

#### Prompt Packet
```markdown
# TASK: T003 - Update Documentation Files

## CONTEXT
Update all markdown documentation files to use VibePilot branding. This includes READMEs, guides, and other docs.

## WHAT TO BUILD
Update naming references in:
1. **README.md** (root)
2. **docs/*.md** files
3. **CHANGELOG.md** (if exists)
4. **CONTRIBUTING.md** (if exists)
5. **Any subdirectory README files**

## FILES TO MODIFY
Find all markdown files with vibeflow references:
```bash
grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot --include="*.md" -l 2>/dev/null | grep -v node_modules | grep -v ".git"
```

### Replacement
For each file:
```bash
sed -i 's/vibeflow/vibepilot/g' <file>
sed -i 's/Vibeflow/VibePilot/g' <file>
sed -i 's/VIBEFLOW/VIBEPILOT/g' <file>
```

## TECHNICAL SPECIFICATIONS
- Markdown format
- Preserve all links
- Keep code blocks functional
- Maintain document structure
- Do NOT remove historical changelog entries

## ACCEPTANCE CRITERIA
- [ ] All markdown files use VibePilot naming
- [ ] Links still work
- [ ] Code examples updated
- [ ] Markdown renders correctly
- [ ] `grep -ri "vibeflow" --include="*.md"` returns zero results

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T003",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["README.md", "docs/*.md"],
  "summary": "Updated X documentation files",
  "tests_written": [],
  "notes": "Files updated"
}
```

## DO NOT
- Change file structure
- Remove content
- Break existing links
- Remove historical changelog entries
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["README.md", "docs/**/*.md"],
  "tests_required": []
}
```

---

### T004: Update UI Text (Titles, Headers, Labels)
**Confidence:** 0.97
**Dependencies:** none
**Type:** feature
**Category:** ui
**Requires Codebase:** true

#### Purpose
Update all user-facing text in the React application: page titles, headers, labels, buttons, placeholders.

#### Prompt Packet
```markdown
# TASK: T004 - Update UI Text

## CONTEXT
Update all user-facing text in the React dashboard to use VibePilot branding. This includes titles, headers, labels, buttons, placeholders, tooltips, and error messages.

## WHAT TO BUILD
Update text in:
1. **index.html** - `<title>` tag
2. **Page titles** - `<h1>`, `<h2>`, etc.
3. **Button labels**
4. **Input placeholders**
5. **Form labels**
6. **Tooltips and help text**
7. **Error/success messages**
8. **aria-labels**

## FILES TO MODIFY
- `index.html`
- `src/**/*.tsx` (all React components)

### Pre-Step: Identify Files
```bash
grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot/src --include="*.tsx" --include="*.html" -l 2>/dev/null
```

## TECHNICAL SPECIFICATIONS
- Language: TypeScript/React
- Preserve all JSX structure
- Keep accessibility attributes intact
- No functional changes

## ACCEPTANCE CRITERIA
- [ ] Page titles show VibePilot
- [ ] Browser tab title updated
- [ ] All headers use VibePilot naming
- [ ] All button labels updated
- [ ] All placeholders updated
- [ ] aria-labels updated
- [ ] No layout or functional changes

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T004",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["index.html", "src/**/*.tsx"],
  "summary": "Updated UI text across X files",
  "tests_written": [],
  "notes": "Text types updated"
}
```

## DO NOT
- Change component structure
- Modify styling or layout
- Change event handlers
- Alter form validation logic
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["index.html", "src/**/*.tsx"],
  "tests_required": ["existing test suite in T005"]
}
```

---

### T005: Run Test Suite and Build Verification
**Confidence:** 0.99
**Dependencies:** T001, T002, T003, T004 (code_context)
**Type:** testing
**Category:** testing
**Requires Codebase:** true

#### Purpose
Verify all changes pass tests and build succeeds before visual verification.

#### Prompt Packet
```markdown
# TASK: T005 - Run Test Suite and Build Verification

## CONTEXT
T001-T004 have completed text replacements. Now verify no breaking changes by running the test suite and build.

## DEPENDENCIES
- T001: Source code references updated
- T002: Configuration files updated
- T003: Documentation updated
- T004: UI text updated

## WHAT TO BUILD
Run verification commands and report results.

## VERIFICATION STEPS

### Step 1: Run Test Suite
```bash
npm test
```
Expected: All tests pass

### Step 2: Run Build
```bash
npm run build
```
Expected: Build succeeds without errors

### Step 3: Final Grep Verification
```bash
# Verify no vibeflow remains in source/config
grep -ri "vibeflow" /home/mjlockboxsocial/vibepilot --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" --include="*.json" --include="*.html" 2>/dev/null | grep -v node_modules | grep -v ".git"
```
Expected: Zero results (or only acceptable exceptions documented)

### Step 4: Smoke Test
```bash
npm run dev
# Verify dashboard starts without errors
```

## ACCEPTANCE CRITERIA
- [ ] `npm test` - all tests pass
- [ ] `npm run build` - succeeds
- [ ] `grep -ri "vibeflow"` - zero results in source files
- [ ] Dashboard starts without errors

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T005",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": [],
  "summary": "All tests pass, build succeeds",
  "verification_results": {
    "npm_test": "PASS",
    "npm_build": "PASS",
    "grep_check": "PASS - zero vibeflow in source",
    "smoke_test": "PASS - dashboard starts"
  },
  "notes": "Any issues found"
}
```

## DO NOT
- Skip any verification step
- Proceed to T006 if tests fail
- Ignore build errors
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [],
  "tests_passed": true,
  "build_succeeded": true
}
```

---

### T006: Visual Verification and Human Approval Workflow
**Confidence:** 0.96
**Dependencies:** T005 (summary)
**Type:** verification
**Category:** testing
**Requires Codebase:** false

#### Purpose
Create PR and guide human through visual verification. Per AGENTS.md, UI changes require human approval before merge.

#### Prompt Packet
```markdown
# TASK: T006 - Visual Verification and Human Approval

## CONTEXT
T001-T005 have completed all text replacements and verification. Now create a PR and guide human through final approval.

## DEPENDENCIES
- T005: All tests pass, build succeeds

## WHAT TO BUILD
1. Commit changes to feature branch
2. Create PR with clear description
3. Provide human verification instructions

## VERIFICATION WORKFLOW

### Step 1: Create Feature Branch
```bash
git checkout -b feature/vibepilot-rename-dashboard
git add .
git commit -m "Rebrand: Replace vibeflow with VibePilot across dashboard

- Updated source code references (T001)
- Updated configuration files (T002)
- Updated documentation (T003)
- Updated UI text (T004)
- All tests pass (T005)

PRD: docs/prd/vibepilot-rename-dashboard-prd.md"
```

### Step 2: Push and Create PR
```bash
git push origin feature/vibepilot-rename-dashboard
```

Create PR with body:
```markdown
## Summary
Replaced all "vibeflow" text references with "VibePilot" branding.

### Changes
- Source code: string literals and comments
- Configuration: package.json, .env.example
- Documentation: README, docs/*.md
- UI: titles, headers, labels

## Verification Completed
- [x] All tests pass
- [x] Build succeeds
- [x] grep verification: zero vibeflow in source

## Human Actions Required
- [ ] **File Review:** Review changed files in GitHub "Files changed" tab
- [ ] **Visual Verification:** Run dashboard locally and verify UI text
- [ ] **Approval:** Approve this PR before merging to main

## Files Changed
[List from T001-T004 outputs]
```

### Step 3: Human Verification Checklist
Provide these instructions:

**FILE REVIEW (GitHub PR):**
1. Open PR "Files changed" tab
2. Verify each replacement is correct
3. Check case preservation (Vibeflow→VibePilot, not Vibepilot)
4. Confirm no unintended changes

**VISUAL VERIFICATION (Local):**
```bash
npm run dev
```
5. Open dashboard in browser
6. Verify page titles show "VibePilot"
7. Verify headers and labels updated
8. Verify no broken functionality

## ACCEPTANCE CRITERIA
- [ ] Changes committed to feature branch
- [ ] PR created with description
- [ ] Human verification instructions provided
- [ ] Awaiting human approval (not auto-merged)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T006",
  "model_name": "[your model name]",
  "pr_url": "https://github.com/.../pull/XXX",
  "branch_name": "feature/vibepilot-rename-dashboard",
  "status": "awaiting_human_approval",
  "human_actions": [
    "Review PR files changed tab",
    "Run dashboard locally (npm run dev)",
    "Verify UI text shows VibePilot",
    "Approve PR if verification passes"
  ],
  "notes": "PR ready for human review"
}
```

## DO NOT
- Merge to main without human approval
- Skip creating a PR
- Push directly to main branch
- Auto-approve or self-merge
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [],
  "pr_created": true,
  "status": "awaiting_human_approval"
}
```

---

## Critical Path
T001-T004 (parallel) → T005 → T006 → Human Approval

## Confidence Summary
- T001: 0.97 - Well-defined text replacement, no ambiguity
- T002: 0.98 - Simple config updates, low risk
- T003: 0.98 - Documentation updates, no code risk
- T004: 0.97 - UI text updates, no functional changes
- T005: 0.99 - Verification task, clear pass/fail criteria
- T006: 0.96 - Human-dependent, clear workflow

**Overall Plan Confidence:** 0.97

## Revision Notes (v3)
Addresses Council feedback from Round 2:
- ✅ Created 6 tasks that systematically cover all 4 P0 features
- ✅ Removed escalation clause - scope is known (455 occurrences expected)
- ✅ Added T005 for test/build verification per PRD success criteria
- ✅ Clarified T006 visual verification is for UI text changes only
- ✅ Fixed plan path to match actual file location
- ✅ Each task has single responsibility and clear expected output
- ✅ Added .env files to T002 scope per PRD P0.2
