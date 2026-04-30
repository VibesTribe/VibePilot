# PRD Template (VibePilot Standard)

> This template defines the structure for all VibePilot PRDs.
> Every section is required. If a section doesn't apply, state why explicitly.
> The planner agent parses this document -- structure matters.

---

## {Project Name}

**Status:** DRAFT | APPROVED  
**Created:** {date}  
**Scope:** {MVP | Full | Phase N}  
**Source:** {Discovery session reference or user input summary}

---

## 1. Summary

{2-3 sentences: What is this, who is it for, what problem does it solve.}

### User Intent
{What the user actually wants, in their own words or close paraphrase. This is the anchor -- everything traces back to here.}

### Success Criteria
{How we know this is done. Measurable, observable, no ambiguity.}
- {criterion 1}
- {criterion 2}

---

## 2. Architecture Decisions

### Tech Stack
| Layer | Technology | Rationale |
|-------|-----------|-----------|
| {Frontend/API/Database/etc} | {choice} | {why this, not that} |

### System Design
{High-level architecture: components, boundaries, data flow. Diagram or structured description.}

### Patterns & Conventions
{Design patterns, coding conventions, naming rules, file organization that agents must follow.}

---

## 3. Requirements

### P1 -- Must Have (MVP)

#### FR-{001}: {requirement title}
**Priority:** P1  
**Traceability:** {which discovery finding or user intent this maps to}  
**Description:** {what this requirement means, clearly and completely}

**Scenarios:**
- GIVEN {initial state}, WHEN {action}, THEN {expected outcome}
- GIVEN {edge case state}, WHEN {action}, THEN {expected outcome}

**Acceptance:** {how to verify this is correctly implemented}

---

#### FR-{002}: {requirement title}
{same structure}

### P2 -- Should Have
{same FR structure, P2 items}

### P3 -- Nice to Have
{same FR structure, P3 items}

---

## 4. Data Contracts

### Entities
```
{EntityName}:
  {field_name}: {type} [{required|optional}] [{constraints}]
  ...
```

### API Endpoints
```
{METHOD} {path}
  Request: {shape}
  Response: {shape}
  Errors: {possible error codes and meanings}
```

### Configuration Schema
```
{config_key}: {type} [{default}] [{description}]
```

---

## 5. Implementation Notes

### File Organization
{Where new files go, how they're named, what existing patterns to follow.}

### Dependencies
{External packages, services, or APIs this project depends on.}

### Environment
{Required env vars, config files, service dependencies.}

---

## 6. Testing Strategy

| Type | Coverage Target | Tools |
|------|----------------|-------|
| Unit | {target}% | {tools} |
| Integration | {target}% | {tools} |
| E2E | {target}% | {tools} |

### Critical Test Scenarios
{The tests that MUST pass before this is considered done.}

---

## 7. Quality Checklist

- [ ] Every FR has a scenario with GIVEN/WHEN/THEN
- [ ] Every FR traces to a user intent or discovery finding
- [ ] All data contracts are fully typed
- [ ] Error handling defined for every external call
- [ ] Edge cases addressed (bad input, failures, concurrency)
- [ ] Testing strategy defined
- [ ] File organization follows existing conventions
- [ ] No unresolved [NEEDS CLARIFICATION] markers
- [ ] Architecture decisions have rationale
- [ ] Maintenance story is clear (how agents will maintain this)

---

## 8. Open Questions
{List any remaining [NEEDS CLARIFICATION] items. Max 3. If more, discovery phase is incomplete.}
