package main

import (
	"context"
	"encoding/json"

	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
)

type dbCheckpointAdapter struct {
	db *db.DB
}

func (a *dbCheckpointAdapter) RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error) {
	result, err := a.db.RPC(ctx, fn, args)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (a *dbCheckpointAdapter) Save(ctx context.Context, taskID string, checkpoint core.Checkpoint) error {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Save(ctx, taskID, checkpoint)
}

func (a *dbCheckpointAdapter) Load(ctx context.Context, taskID string) (*core.Checkpoint, error) {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Load(ctx, taskID)
}

func (a *dbCheckpointAdapter) Delete(ctx context.Context, taskID string) error {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Delete(ctx, taskID)
}
