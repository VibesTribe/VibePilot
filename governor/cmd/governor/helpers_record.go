package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

// fetchRecord fills in the record map from event.Record if available,
// or falls back to querying the database using event.ID and event.Table.
// This is needed because pgnotify events have Record=nil (only id/table/status),
// while Supabase webhook events have the full record.
func fetchRecord(ctx context.Context, database db.Database, event runtime.Event) (map[string]any, error) {
	// Try unmarshalling the embedded record first
	if len(event.Record) > 0 {
		var record map[string]any
		if err := json.Unmarshal(event.Record, &record); err == nil {
			return record, nil
		}
	}

	// Fallback: query the database for the full row
	if event.ID == "" || event.Table == "" {
		return nil, fmt.Errorf("no record data and no id/table to query")
	}

	result, err := database.Query(ctx, event.Table, map[string]any{
		"id":    event.ID,
		"limit": 1,
	})
	if err != nil {
		return nil, fmt.Errorf("query %s id=%s: %w", event.Table, event.ID, err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("parse %s rows: %w", event.Table, err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no %s row found with id=%s", event.Table, event.ID)
	}

	return rows[0], nil
}
