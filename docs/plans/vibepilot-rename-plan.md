# PLAN: VibePilot Dashboard Rebrand — "vibeflow" → "vibepilot"

## Overview
Complete text replacement of legacy product name "vibeflow" with "vibepilot" across all dashboard-facing documentation and configuration files. This is a text-only change with zero visual or functional impact.

## Scope
- 13+ markdown documentation files
- 1 JSON plan file
- 0 Python files (dashboard already updated)
- File renames for consistency

## Tasks

### T001: Audit and catalog all vibeflow occurrences
**Confidence:** 0.98
**Dependencies:** none
**Type:** research
**Category:** research
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T001 - Audit and catalog all vibeflow occurrences

## CONTEXT
Before making any changes, we need a complete inventory of all "vibeflow" references in the codebase. This ensures we don't miss any occurrences and provides a baseline for validation.

## DEPENDENCIES
None

## WHAT TO BUILD
Create a comprehensive audit report listing:
1. Every file containing "vibeflow" (case-insensitive)
2. Line numbers and context for each occurrence
3. Classification of each occurrence (product reference, URL, filename, etc.)
4. Files that need renaming vs content-only changes

## FILES TO CREATE
- `docs/plans/vibeflow-audit-report.md` - Detailed audit findings

## TECHNICAL SPECIFICATIONS

### Search Commands
```bash
# Find all occurrences
grep -rn "vibeflow" --include="*.md" --include="*.json" --include="*.py" dashboard/ docs/ config/ 2>/dev/null

# Find files with vibeflow in name
find docs/ -name "*vibeflow*" -type f

# Count total occurrences
grep -r "vibeflow" dashboard/ docs/ config/ 2>/dev/null | wc -l
```

### Expected Output Format
```markdown
# vibeflow Audit Report

## Summary
- Total occurrences: X
- Files affected: Y
- Files to rename: Z

## Detailed Findings

### Files Requiring Content Changes
1. `docs/GO_IRON_STACK.md`
   - Line 45: "vibeflow repo untouched"
   - Classification: Product reference

2. `docs/USEFUL_COMMANDS.md`
   - Line 12: "https://vibeflow-dashboard.vercel.app/"
   - Classification: URL
   ...

### Files Requiring Rename + Content Changes
1. `docs/vibeflow_dashboard_analysis.md`
   - Rename to: `docs/vibepilot_dashboard_analysis.md`
   - Content changes: 3 occurrences
```

## ACCEPTANCE CRITERIA
- [ ] All files containing "vibeflow" identified
- [ ] Each occurrence classified by type (product ref, URL, filename, etc.)
- [ ] Files categorized as "content-only" vs "rename + content"
- [ ] Audit report saved to `docs/plans/vibeflow-audit-report.md`
- [ ] Total count of occurrences documented

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model]",
  "files_created": ["docs/plans/vibeflow-audit-report.md"],
  "files_modified": [],
  "summary": "Audited X occurrences across Y files, Z require renaming",
  "tests_written": [],
  "notes": "Key findings: [brief summary]"
}
```

## DO NOT
- Make any actual changes to files
- Modify the current PRD file
- Skip any directories
```

#### Expected Output
```json
{
  "files_created": ["docs/plans/vibeflow-audit-report.md"],
  "files_modified": [],
  "tests_required": []
}
```

---

### T002: Update documentation files (content-only changes)
**Confidence:** 0.97
**Dependencies:** T001 (summary)
**Type:** documentation
**Category:** documentation
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T002 - Update documentation files (content-only changes)

## CONTEXT
Replace all "vibeflow" text occurrences with "vibepilot" in documentation files that don't require renaming. These are references to the product name, URLs, and descriptions.

## DEPENDENCIES

### Summary Dependencies
- T001: Audit identified X files requiring content-only changes (no renaming needed). These files contain product references, URLs, and descriptions that need updating.

## WHAT TO BUILD
Update the following files by replacing "vibeflow" with "vibepilot" (case-sensitive where appropriate):

1. `docs/GO_IRON_STACK.md`
2. `docs/research/vibes_interface_specification.md`
3. `docs/USEFUL_COMMANDS.md`
4. `docs/WHAT_WHERE.md`
5. `docs/infrastructure_gap_analysis.md`
6. `docs/SYSTEM_CLEANUP.md`
7. `docs/UPDATE_CONSIDERATIONS.md`
8. `docs/SYSTEM_REFERENCE.md`

### Replacement Rules
- "vibeflow" → "vibepilot" (lowercase)
- "Vibeflow" → "VibePilot" (title case)
- "VIBEFLOW" → "VIBEPILOT" (uppercase)
- URLs: Update if they reference our deployed dashboard
- External URLs (github.com/VibesTribe/vibeflow): Keep as-is (external repo reference)

## FILES TO MODIFY
- `docs/GO_IRON_STACK.md` - Update product reference
- `docs/research/vibes_interface_specification.md` - Update dashboard URLs and references
- `docs/USEFUL_COMMANDS.md` - Update URLs
- `docs/WHAT_WHERE.md` - Update directory reference
- `docs/infrastructure_gap_analysis.md` - Update repo reference
- `docs/SYSTEM_CLEANUP.md` - Update directory references
- `docs/UPDATE_CONSIDERATIONS.md` - Update source reference
- `docs/SYSTEM_REFERENCE.md` - Update dashboard location and URLs

## TECHNICAL SPECIFICATIONS

### Replacement Command
```bash
# For each file, use sed or manual editing
sed -i 's/vibeflow/vibepilot/g' docs/filename.md
sed -i 's/Vibeflow/VibePilot/g' docs/filename.md
sed -i 's/VIBEFLOW/VIBEPILOT/g' docs/filename.md
```

### Validation
After changes:
```bash
grep -n "vibeflow" docs/filename.md
# Should return empty or only external references
```

## ACCEPTANCE CRITERIA
- [ ] All 8 files updated with correct replacements
- [ ] Case-sensitive replacements applied correctly
- [ ] External URLs preserved where appropriate
- [ ] No unintended changes to formatting or structure
- [ ] grep search shows no remaining "vibeflow" (except external refs)

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model]",
  "files_created": [],
  "files_modified": [
    "docs/GO_IRON_STACK.md",
    "docs/research/vibes_interface_specification.md",
    "docs/USEFUL_COMMANDS.md",
    "docs/WHAT_WHERE.md",
    "docs/infrastructure_gap_analysis.md",
    "docs/SYSTEM_CLEANUP.md",
    "docs/UPDATE_CONSIDERATIONS.md",
    "docs/SYSTEM_REFERENCE.md"
  ],
  "summary": "Updated 8 documentation files with vibeflow → vibepilot replacements",
  "tests_written": [],
  "notes": "External GitHub URLs preserved as vibeflow"
}
```

## DO NOT
- Rename any files (that's a separate task)
- Modify the current PRD or old PRD files
- Change URLs to external vibeflow repos
- Alter document structure or formatting
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": [
    "docs/GO_IRON_STACK.md",
    "docs/research/vibes_interface_specification.md",
    "docs/USEFUL_COMMANDS.md",
    "docs/WHAT_WHERE.md",
    "docs/infrastructure_gap_analysis.md",
    "docs/SYSTEM_CLEANUP.md",
    "docs/UPDATE_CONSIDERATIONS.md",
    "docs/SYSTEM_REFERENCE.md"
  ],
  "tests_required": []
}
```

---

### T003: Rename and update vibeflow-specific documentation files
**Confidence:** 0.96
**Dependencies:** T001 (summary)
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T003 - Rename and update vibeflow-specific documentation files

## CONTEXT
Several documentation files have "vibeflow" in their filenames and content. These need to be renamed to "vibepilot" and their internal content updated to match.

## DEPENDENCIES

### Summary Dependencies
- T001: Audit identified 4 files requiring both rename and content updates.

## WHAT TO BUILD
Rename the following files and update their content:

1. `docs/vibeflow_dashboard_analysis.md` → `docs/vibepilot_dashboard_analysis.md`
2. `docs/vibeflow_review.md` → `docs/vibepilot_review.md`
3. `docs/vibeflow_adoption.md` → `docs/vibepilot_adoption.md`
4. `docs/research/raindrop-vibeflow-20260219.md` → `docs/research/raindrop-vibepilot-20260219.md`

## FILES TO CREATE
- `docs/vibepilot_dashboard_analysis.md` (renamed from vibeflow version)
- `docs/vibepilot_review.md` (renamed from vibeflow version)
- `docs/vibepilot_adoption.md` (renamed from vibeflow version)
- `docs/research/raindrop-vibepilot-20260219.md` (renamed from vibeflow version)

## FILES TO DELETE
- `docs/vibeflow_dashboard_analysis.md`
- `docs/vibeflow_review.md`
- `docs/vibeflow_adoption.md`
- `docs/research/raindrop-vibeflow-20260219.md`

## TECHNICAL SPECIFICATIONS

### Process for Each File
1. Read original file content
2. Replace all "vibeflow" with "vibepilot" (case-sensitive)
3. Write to new filename
4. Delete old file

### Git Commands
```bash
# Better to use git mv for tracking
mv docs/vibeflow_dashboard_analysis.md docs/vibepilot_dashboard_analysis.md
# Then edit content
```

### Validation
```bash
# Verify old files gone
ls docs/vibeflow*.md
# Should return nothing

# Verify new files exist
ls docs/vibepilot*.md
# Should show new files
```

## ACCEPTANCE CRITERIA
- [ ] All 4 files renamed successfully
- [ ] Content updated with vibeflow → vibepilot replacements
- [ ] Old files removed
- [ ] New files contain all original content (minus brand changes)
- [ ] File permissions preserved

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T003",
  "model_name": "[your model]",
  "files_created": [
    "docs/vibepilot_dashboard_analysis.md",
    "docs/vibepilot_review.md",
    "docs/vibepilot_adoption.md",
    "docs/research/raindrop-vibepilot-20260219.md"
  ],
  "files_modified": [],
  "summary": "Renamed and updated 4 vibeflow-specific documentation files",
  "tests_written": [],
  "notes": "Used git mv for proper tracking"
}
```

## DO NOT
- Modify content beyond vibeflow → vibepilot replacements
- Delete files without creating replacements
- Change file structure or organization
- Skip any of the 4 identified files
```

#### Expected Output
```json
{
  "files_created": [
    "docs/vibepilot_dashboard_analysis.md",
    "docs/vibepilot_review.md",
    "docs/vibepilot_adoption.md",
    "docs/research/raindrop-vibepilot-20260219.md"
  ],
  "files_modified": [],
  "tests_required": []
}
```

---

### T004: Update old PRD files
**Confidence:** 0.97
**Dependencies:** T001 (summary)
**Type:** documentation
**Category:** documentation
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T004 - Update old PRD files

## CONTEXT
Update the legacy PRD file that contains vibeflow references. This is historical documentation that should be updated for consistency.

## DEPENDENCIES

### Summary Dependencies
- T001: Audit identified docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md as an old PRD with many vibeflow references.

## WHAT TO BUILD
Update the legacy PRD file by replacing "vibeflow" with "vibepilot" where it references our product. Keep historical context where appropriate.

## FILES TO MODIFY
- `docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md` - Update product references

## TECHNICAL SPECIFICATIONS

### Replacement Rules
- Product name references: "vibeflow" → "vibepilot"
- File paths in vibeflow repo: Keep as-is (historical accuracy)
- Component names: Keep as-is if they were actually named that way

### Context to Preserve
This is a historical PRD documenting a previous change. Some references to "vibeflow" may be intentionally historical and should be preserved if they describe the state of the system at that time.

## ACCEPTANCE CRITERIA
- [ ] Product name references updated to "vibepilot"
- [ ] Historical context preserved where appropriate
- [ ] Document still reads naturally
- [ ] No unintended changes to technical details

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T004",
  "model_name": "[your model]",
  "files_created": [],
  "files_modified": ["docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md"],
  "summary": "Updated legacy PRD with product name changes",
  "tests_written": [],
  "notes": "Preserved historical context where appropriate"
}
```

## DO NOT
- Remove or significantly alter the document structure
- Change technical specifications that were accurate at the time
- Update the current PRD (vibepilot-rename-dashboard-prd.md)
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["docs/prd/dashboard-text-change-vibeflow-to-vibepilot.md"],
  "tests_required": []
}
```

---

### T005: Update config files
**Confidence:** 0.98
**Dependencies:** T001 (summary)
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T005 - Update config files

## CONTEXT
Update configuration files and prompt templates that contain "vibeflow" references.

## DEPENDENCIES

### Summary Dependencies
- T001: Audit identified config files with vibeflow references requiring updates.

## WHAT TO BUILD
Search for and update any "vibeflow" references in config/ directory, particularly in prompt templates.

## FILES TO MODIFY
- `config/prompts/consultant.md` - Update brand references in prompt template

## TECHNICAL SPECIFICATIONS

### Search Command
```bash
find config/ -type f -exec grep -l "vibeflow" {} \;
```

### Replacement Rules
- System name references: "vibeflow" → "vibepilot"
- Product descriptions: Update to reflect current branding
- Prompt context: Ensure AI agents understand the system is now called VibePilot

## ACCEPTANCE CRITERIA
- [ ] All config files with vibeflow references identified
- [ ] References updated to vibepilot
- [ ] Prompt templates still functional
- [ ] No syntax errors introduced

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T005",
  "model_name": "[your model]",
  "files_created": [],
  "files_modified": ["config/prompts/consultant.md"],
  "summary": "Updated config files with brand name changes",
  "tests_written": [],
  "notes": "Verified prompt templates remain functional"
}
```

## DO NOT
- Change prompt logic or structure
- Modify unrelated configuration settings
- Break existing prompt functionality
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["config/prompts/consultant.md"],
  "tests_required": []
}
```

---

### T006: Update plan metadata files
**Confidence:** 0.98
**Dependencies:** T001 (summary)
**Type:** configuration
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T006 - Update plan metadata files

## CONTEXT
Update JSON plan files that contain "vibeflow" in their metadata or content.

## DEPENDENCIES

### Summary Dependencies
- T001: Audit identified docs/plans/vibeflow-test-plan.json with brand references.

## WHAT TO BUILD
Update the plan JSON file and rename it to reflect current branding.

## FILES TO CREATE
- `docs/plans/vibepilot-test-plan.json` (renamed from vibeflow version)

## FILES TO DELETE
- `docs/plans/vibeflow-test-plan.json`

## TECHNICAL SPECIFICATIONS

### Process
1. Read `docs/plans/vibeflow-test-plan.json`
2. Update "name" field: "vibeflow-to-vibepilot-test" → "vibepilot-test" or keep as historical reference
3. Search for any other "vibeflow" strings in JSON content
4. Write to `docs/plans/vibepilot-test-plan.json`
5. Delete old file

### JSON Structure to Check
```json
{
  "name": "vibeflow-to-vibepilot-test",  // Update this
  ...
}
```

## ACCEPTANCE CRITERIA
- [ ] Plan JSON file renamed
- [ ] Internal references updated
- [ ] Valid JSON structure maintained
- [ ] Old file removed

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T006",
  "model_name": "[your model]",
  "files_created": ["docs/plans/vibepilot-test-plan.json"],
  "files_modified": [],
  "summary": "Renamed and updated plan metadata file",
  "tests_written": [],
  "notes": "JSON structure preserved and validated"
}
```

## DO NOT
- Break JSON syntax
- Remove important metadata
- Change plan structure or logic
```

#### Expected Output
```json
{
  "files_created": ["docs/plans/vibepilot-test-plan.json"],
  "files_modified": [],
  "tests_required": []
}
```

---

### T007: Create validation script
**Confidence:** 0.99
**Dependencies:** none
**Type:** coding
**Category:** testing
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T007 - Create validation script

## CONTEXT
Create an automated validation script that checks for any remaining "vibeflow" occurrences in the codebase. This will be used to verify the rebranding is complete.

## DEPENDENCIES
None

## WHAT TO BUILD
Create a bash script that:
1. Searches for "vibeflow" in all relevant directories
2. Counts occurrences
3. Reports findings
4. Exits with appropriate status code

## FILES TO CREATE
- `scripts/validate_no_vibeflow.sh` - Validation script
- `scripts/.gitkeep` - Ensure scripts directory exists

## TECHNICAL SPECIFICATIONS

### Script Requirements
```bash
#!/bin/bash
# validate_no_vibeflow.sh
# Validates that no "vibeflow" references remain in the codebase

# Directories to check
DIRS="dashboard/ docs/ config/"

# Count occurrences (case-insensitive)
COUNT=$(grep -ri "vibeflow" $DIRS 2>/dev/null | grep -v "Binary file" | wc -l)

# Check results
if [ "$COUNT" -eq 0 ]; then
    echo "✓ Validation PASSED: No 'vibeflow' occurrences found"
    echo "  Checked directories: $DIRS"
    exit 0
else
    echo "✗ Validation FAILED: $COUNT occurrence(s) of 'vibeflow' found"
    echo ""
    echo "Locations:"
    grep -ri "vibeflow" $DIRS 2>/dev/null | grep -v "Binary file"
    exit 1
fi
```

### File Permissions
```bash
chmod +x scripts/validate_no_vibeflow.sh
```

## ACCEPTANCE CRITERIA
- [ ] Script created in scripts/ directory
- [ ] Executable permissions set
- [ ] Searches dashboard/, docs/, and config/ directories
- [ ] Returns exit code 0 if no occurrences found
- [ ] Returns exit code 1 and lists occurrences if found
- [ ] Handles binary files gracefully

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T007",
  "model_name": "[your model]",
  "files_created": [
    "scripts/validate_no_vibeflow.sh",
    "scripts/.gitkeep"
  ],
  "files_modified": [],
  "summary": "Created validation script to check for vibeflow occurrences",
  "tests_written": [],
  "notes": "Script is executable and ready to run"
}
```

## DO NOT
- Make the script overly complex
- Search irrelevant directories (node_modules, .git, etc.)
- Require external dependencies
```

#### Expected Output
```json
{
  "files_created": ["scripts/validate_no_vibeflow.sh"],
  "files_modified": [],
  "tests_required": []
}
```

---

### T008: Run validation and verify complete rebranding
**Confidence:** 0.99
**Dependencies:** T002, T003, T004, T005, T006, T007 (all content updates + validation script)
**Type:** testing
**Category:** testing
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T008 - Run validation and verify complete rebranding

## CONTEXT
Execute the validation script created in T007 to verify all "vibeflow" references have been successfully replaced. This is the final verification step.

## DEPENDENCIES

### Code Context Dependencies
- T007: Run the validation script at scripts/validate_no_vibeflow.sh

## WHAT TO BUILD
Run the validation script and document the results. If any occurrences remain, identify and fix them.

## FILES TO CREATE
- `docs/plans/validation-report.md` - Final validation results

## TECHNICAL SPECIFICATIONS

### Validation Process
```bash
# Run the validation script
./scripts/validate_no_vibeflow.sh

# If it fails, investigate:
grep -ri "vibeflow" dashboard/ docs/ config/ 2>/dev/null

# Document findings
```

### Validation Report Format
```markdown
# VibePilot Rebrand Validation Report

**Date:** [timestamp]
**Validator:** [model name]

## Validation Results

### Automated Check
- Script: `scripts/validate_no_vibeflow.sh`
- Status: PASSED/FAILED
- Occurrences found: X

### Manual Verification
- [ ] Dashboard header displays "VIBEPILOT"
- [ ] Documentation reads consistently
- [ ] No broken references

### Files Modified Summary
- Total files modified: X
- Files renamed: Y
- Content-only changes: Z

### Recommendations
[Any final notes or recommendations]
```

## ACCEPTANCE CRITERIA
- [ ] Validation script executed successfully
- [ ] Zero "vibeflow" occurrences found in dashboard/, docs/, config/
- [ ] Validation report created documenting results
- [ ] If any occurrences found, they are fixed and re-validated
- [ ] All modified files listed in report

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T008",
  "model_name": "[your model]",
  "files_created": ["docs/plans/validation-report.md"],
  "files_modified": [],
  "summary": "Validation complete: 0 vibeflow occurrences found",
  "tests_written": [],
  "notes": "All rebranding changes verified successfully"
}
```

## DO NOT
- Skip validation if script fails
- Ignore remaining occurrences
- Mark complete until validation passes
- Modify files without re-running validation
```

#### Expected Output
```json
{
  "files_created": ["docs/plans/validation-report.md"],
  "files_modified": [],
  "tests_required": []
}
```

---

## Task Summary

| Task | Title | Dependencies | Confidence | Category |
|------|-------|--------------|------------|----------|
| T001 | Audit vibeflow occurrences | none | 0.98 | research |
| T002 | Update documentation files | T001 | 0.97 | documentation |
| T003 | Rename vibeflow-specific docs | T001 | 0.96 | configuration |
| T004 | Update old PRD files | T001 | 0.97 | documentation |
| T005 | Update config files | T001 | 0.98 | configuration |
| T006 | Update plan metadata | T001 | 0.98 | configuration |
| T007 | Create validation script | none | 0.99 | testing |
| T008 | Run validation | T002-T007 | 0.99 | testing |

## Critical Path
T001 → T002/T003/T004/T005/T006 (parallel) → T008

## Estimated Total Context
~12,000 tokens (well within 32K limit)

## Risk Assessment
- **Low Risk:** Text-only changes, easily reversible via git
- **Validation:** Automated script ensures completeness
- **Rollback:** Simple git revert if issues arise
