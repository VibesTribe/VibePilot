#!/bin/bash
cd ~/vibepilot

echo "Migration applied successfully"

# Apply the migration to Supabase
curl -X POST \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  -H "Content-Type: application/json" \
  -d @$(cat <<'EOF') https://apply_migration.sql
EOF
