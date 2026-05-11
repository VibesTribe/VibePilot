package visualqa

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGXDB adapts a pgxpool.Pool to the visualqa DB interface.
type PGXDB struct {
	pool *pgxpool.Pool
}

// NewPGXDB creates a new DB adapter from a pgx connection string.
func NewPGXDB(ctx context.Context, connString string) (*PGXDB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("[VisualQA] parse pgx config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("[VisualQA] create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("[VisualQA] ping failed: %w", err)
	}
	return &PGXDB{pool: pool}, nil
}

// NewPGXDBFromPool creates a DB adapter from an existing pgxpool.Pool.
func NewPGXDBFromPool(pool *pgxpool.Pool) *PGXDB {
	return &PGXDB{pool: pool}
}

// Close closes the underlying pool (only call if you created the pool via NewPGXDB).
func (d *PGXDB) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

// Exec executes a query using pgx.
func (d *PGXDB) Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	return d.pool.Exec(ctx, query, args...)
}

// Query executes a query and returns pgx rows.
func (d *PGXDB) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return d.pool.Query(ctx, query, args...)
}

// SQLDB returns a database/sql DB wrapper around the pgx pool for the baseline.go code
// that uses database/sql. This creates a separate connection but shares the same postgres instance.
func (d *PGXDB) SQLDB() *sql.DB {
	// pgx stdlib adapter - open using the same connection string
	return nil // Not needed since we rewrote baseline.go to not use database/sql
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ensureDir creates a directory and all parent directories.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// removeDir removes a directory and all contents.
func removeDir(path string) error {
	return os.RemoveAll(path)
}
