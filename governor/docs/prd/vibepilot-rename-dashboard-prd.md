# PRD: VibePilot Dashboard Rename

## Overview
Rename all dashboard references from current naming to "VibePilot" branding. This is a text-only rebranding with zero visual or functional impact.

## Objectives
- Update all text references to use VibePilot branding
- Maintain existing functionality without changes
- Ensure all documentation reflects new naming
- Verify no breaking changes

## Success Criteria
- All visible text shows "VibePilot" branding
- All tests pass
- No functional changes to dashboard behavior
- Documentation is consistent with new naming

## Tech Stack
- Language: TypeScript/JavaScript
- Framework: React
- Testing: Vitest

## Features

### P0 Critical
1. **Code References Update**
   - Update string literals in source code
   - Update comments referencing old naming
   - Acceptance: No references to old naming in source

2. **Configuration Update**
   - Update package.json
   - Update environment variables
   - Update config files
   - Acceptance: All configs use VibePilot naming

3. **Documentation Update**
   - Update README files
   - Update inline documentation
   - Acceptance: All docs use VibePilot naming

4. **UI Text Update**
   - Update page titles
   - Update headers
   - Update labels
   - Acceptance: All visible text uses VibePilot

## Out of Scope
- Visual design changes
- Functional changes
- API changes
- Database schema changes
