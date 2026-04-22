package db

import (
	"context"
	"encoding/json"
	"time"
)

// Database is the abstract interface for all database operations.
// Implementations: PostgREST (Supabase or self-hosted), native Postgres (pgx), SQLite.
// Handlers depend on this interface, never on concrete types.
type Database interface {
	// Core CRUD
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error)
	Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error)
	Delete(ctx context.Context, table, id string) error

	// Typed RPC helpers
	CallRPC(ctx context.Context, name string, params map[string]any) (json.RawMessage, error)
	CallRPCInto(ctx context.Context, name string, params map[string]any, dest any) error

	// State machine helpers
	RecordStateTransition(ctx context.Context, entityType, entityID, fromState, toState, reason string, metadata map[string]any) error
	RecordPerformanceMetric(ctx context.Context, metricType, entityID string, duration time.Duration, success bool, metadata map[string]any) error
	GetLatestState(ctx context.Context, entityType, entityID string) (toState string, reason string, createdAt time.Time, err error)
	ClearProcessingAndRecordTransition(ctx context.Context, table, id, fromState, toState, reason string) error

	// Domain queries
	GetDestination(ctx context.Context, id string) (*Destination, error)
	GetRunners(ctx context.Context) ([]Runner, error)
	GetTaskPacket(ctx context.Context, taskID string) (*TaskPacket, error)

	// Lifecycle
	Close() error
}

// Compile-time proof that DB satisfies Database.
var _ Database = (*DB)(nil)
