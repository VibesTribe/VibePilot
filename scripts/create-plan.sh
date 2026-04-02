#!/bin/bash
# Create a plan from a PRD in Supabase

SUPABASE_URL="https://qtpdzsinvifkgpxyxlaz.supabase.co"
SUPABASE_SERVICE_KEY="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InF0cGR6c2lndmlma2dweHl4bGF6Iiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc3MDY2MTU3MCwiZXhwIjoyMDg2MjM3NTcwfQ.7ixiABm_tE91p1RoUtT2z4E8eLiwTfD7RgIk6E87_yQ"

PRD_PATH="${1:-docs/prd/hello-world.md}"

echo "Creating plan for PRD: $PRD_PATH"

curl -s -X POST "$SUPABASE_URL/rest/v1/plans" \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"prd_path\":\"$PRD_PATH\",\"status\":\"draft\"}"

echo ""
echo "Plan created. Monitor with: tail -f /tmp/governor.out"
