# Test Webhook PRD

## Purpose
Test that GitHub webhooks correctly detect PRD files and create plans automatically.

## Requirements
- Verify webhook receives push event
- Confirm plan is created in Supabase
- Validate the automated flow works end-to-end

## Success Criteria
- Plan appears in Supabase with status='draft'
- Governor logs show webhook processing
- No errors in governor logs
