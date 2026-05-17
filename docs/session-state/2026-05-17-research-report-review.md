# Session State: Research Report Per-Item Review Pipeline

## Date: May 17, 2026 (early morning session)

## What was built
Per-item review system for research reports. Instead of council reviewing a whole suggestion, findings are now grouped into reports with individual items. Each item gets its own council recommendation (approve/watch/reject) and human decision buttons in the dashboard.

## Files Changed

### Backend (vibepilot repo)
- `governor/cmd/governor/handlers_research.go` - Major changes:
  - handleResearchReady creates research_reports + research_report_items rows
  - handleReportCouncilReview runs N council members on all items in parallel
  - aggregateAndSaveCouncilResults tallies votes per-item
  - writeReportDecisionDoc writes formatted KB doc with per-item analysis
  - Types moved to package level: councilMemberResult, reportItemResult
  - EventReportCouncil event constant
- `governor/internal/runtime/decision.go` - Added ParseReportCouncilVotes function
- `governor/internal/runtime/events.go` - Added EventReportCouncil constant
- `governor/internal/webhooks/server.go` - 3 new endpoints + routes:
  - GET /api/research-reports (list with item counts)
  - GET /api/research-reports/{id} (detail via RPC)
  - PATCH /api/report-items/{id} (set decision, auto-triggers PRD when all decided)

### Frontend (vibeflow repo)
- `apps/dashboard/components/ResearchReportPanel.tsx` - NEW: Modal with per-item cards
- `apps/dashboard/components/MissionHeader.tsx` - Wired ResearchReportPanel for research items

### Database
- 4 new SQL RPCs: get_report_for_review, set_report_item_decision, report_undecided_count, update_report_status
- Tables already existed: research_reports, research_report_items

### Knowledge Base
- `knowledgebase/research/research-report-per-item-review.md` - Full pipeline doc

## What's NOT deployed yet
- Governor binary built to /tmp/governor_new but NOT copied to live location
- Frontend built to ~/vibeflow/dist/ but NOT deployed
- The new binary needs: stop service, cp /tmp/governor_new ~/vibepilot/governor/governor, start service

## Remaining Work (kanban items 142-144)

### #142 - Monthly watch/rejected review cron
- Add endpoint or handler that queries items with human_decision IN ('watch','reject') where last_reviewed_at > 30 days ago
- Re-create review_items for human re-evaluation
- Pattern: look at handlers_maint.go, add API endpoint, add crontab entry

### #143 - Deploy and E2E test
- Stop governor: pkill -f 'governor/governor'
- Copy binary: cp /tmp/governor_new ~/vibepilot/governor/governor
- Start governor: systemctl start vibepilot-governor (or the appropriate start command)
- Deploy frontend: check where dist/ is served from, copy updated files
- Test flow: trigger research -> verify report created -> council runs -> dashboard shows items -> approve/reject -> PRD trigger

### #144 - KB docs for report-based review
- Already wrote research-report-per-item-review.md
- May need to update existing research pipeline docs to reflect new flow

## Build Status
- Go backend: BUILDS CLEAN (go build -o /tmp/governor_new ./cmd/governor/)
- Vite frontend: BUILDS CLEAN (npx vite build in apps/dashboard/)

## Important Notes
- The old research_suggestions path still works for legacy items (13 pending)
- New research will go through the report-based pipeline
- councilMemberResult and reportItemResult are package-level types in handlers_research.go
- EventReportCouncil is registered in RegisterHandlers via router.On()
- The frontend Review pill already shows review_items grouped by type. Research items get "Review Items" button, others get "Review" link
