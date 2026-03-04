# Webhook Test 2

## Purpose
Verify GitHub webhooks work end-to-end after firewall fix.

## Expected Flow
1. GitHub sends push webhook to governor
2. Governor detects this PRD file
3. Plan created in Supabase
4. Planner reviews plan
5. Tasks generated
