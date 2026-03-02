# PRD: Add Heartbeat Log

**Project:** VibePilot Governor  
**Task ID:** GOV-003  
**Type:** Code Change  
**Priority:** P0 (Simple Test)  
**Confidence:** 99%  

---

## Problem

Governor doesn't log periodic heartbeat messages to show it's alive.

---

## Solution

Add a heartbeat log every 60 seconds.

---

## Requirements

- Log "Heartbeat: still running" every 60 seconds
- Use existing logging infrastructure
- No external dependencies

---

## Acceptance Criteria

- [ ] Heartbeat message appears in logs
- [ ] Interval is 60 seconds
- [ ] Uses standard log package

---

## Files Affected

- `governor/cmd/governor/main.go`

---

## Out of Scope

- Configurable interval
- Metrics collection
- Health check endpoints
