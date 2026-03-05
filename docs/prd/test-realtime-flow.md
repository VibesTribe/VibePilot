# Test PRD: Realtime Flow Verification

Minimal test to verify Supabase Realtime triggers the full governor flow.

## Purpose
Verify realtime events flow from Supabase → Governor → Event handlers.

## Requirements
1. Print "Hello, Realtime!" to console
2. Exit with code 0

## Success Criteria
- Governor receives realtime event
- Plan status changes from draft → processing
- No errors in logs

## Priority
Low - test only
