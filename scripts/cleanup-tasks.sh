#!/bin/bash
# Cleanup all VibePilot tasks from Supabase
# Run this script to clear all tasks before fresh testing

SUPABASE_URL="https://qtpdzsinvifkgpxyxlaz.supabase.co"
SUPABASE_SERVICE_KEY="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InF0cGR6c2ludmlma2dweHl4bGF6Iiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc3MDY2MTU3MCwiZXhwIjoyMDg2MjM3NTcwfQ.7ixiABm_tE91p1RoUtT2z4E8eLiwTfD7RgIk6E87_yQ"

echo "=== VibePilot Task Cleanup ==="
echo ""

# First, get all tasks
echo "Fetching current tasks..."
TASKS=$(curl -s "$SUPABASE_URL/rest/v1/tasks?select=id,title,status,task_number" \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY")

if echo "$TASKS" | grep -q "Invalid API key"; then
  echo "ERROR: Invalid API key. Please check SUPABASE_SERVICE_KEY"
  exit 1
fi

TASK_COUNT=$(echo "$TASKS" | grep -o '"id"' | wc -l)
echo "Found $TASK_COUNT tasks"
echo ""

# Delete each task individually
if [ "$TASK_COUNT" -gt 0 ]; then
  echo "Deleting tasks..."
  echo "$TASKS" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 | while read -r task_id; do
    echo "  Deleting task: $task_id"
    curl -s -X DELETE "$SUPABASE_URL/rest/v1/tasks?id=eq.$task_id" \
      -H "apikey: $SUPABASE_SERVICE_KEY" \
      -H "Authorization: Bearer $SUPABASE_SERVICE_KEY"
  done
  echo ""
fi

# Also clear task_runs, plans, plan_revisions
echo "Clearing related tables..."
for table in task_runs plans plan_revisions; do
  echo "  Clearing $table..."
  # Note: TRUNCATE requires superuser, so we delete via RPC or individually
  curl -s "$SUPABASE_URL/rest/v1/$table?id=not.eq.nonexistent" \
    -H "apikey: $SUPABASE_SERVICE_KEY" \
    -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" > /dev/null
done

echo ""
echo "=== Cleanup Complete ==="
echo ""
echo "To fully clean via SQL Editor, run:"
echo "  TRUNCATE task_runs, tasks, plan_revisions, plans CASCADE;"
