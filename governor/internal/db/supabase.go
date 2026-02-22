package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vibepilot/governor/pkg/types"
)

type DB struct {
	db *sql.DB
}

func New(url, serviceKey string) (*DB, error) {
	connStr := fmt.Sprintf("%s?user=postgres&password=%s", url, serviceKey)
	
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetAvailableTasks(ctx context.Context) ([]types.Task, error) {
	query := `
		SELECT id, title, type, priority, status, routing_flag, assigned_to,
		       dependencies, slice_id, phase, task_number, branch_name,
		       attempts, max_attempts, created_at, updated_at
		FROM tasks
		WHERE status = 'available'
		ORDER BY priority ASC, created_at ASC
		LIMIT 10
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query available tasks: %w", err)
	}
	defer rows.Close()

	var tasks []types.Task
	for rows.Next() {
		var t types.Task
		var depsJSON []byte
		var routingFlag string

		err := rows.Scan(
			&t.ID, &t.Title, &t.Type, &t.Priority, &t.Status,
			&routingFlag, &t.AssignedTo, &depsJSON,
			&t.SliceID, &t.Phase, &t.TaskNumber, &t.BranchName,
			&t.Attempts, &t.MaxAttempts, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		t.RoutingFlag = types.RoutingFlag(routingFlag)

		if len(depsJSON) > 0 {
			json.Unmarshal(depsJSON, &t.Dependencies)
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (d *DB) GetTaskPacket(ctx context.Context, taskID string) (*types.PromptPacket, error) {
	query := `
		SELECT task_id, prompt, title, context
		FROM task_packets
		WHERE task_id = $1
		ORDER BY version DESC
		LIMIT 1
	`

	var pp types.PromptPacket
	var context sql.NullString

	err := d.db.QueryRowContext(ctx, query, taskID).Scan(
		&pp.TaskID, &pp.Prompt, &pp.Title, &context,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task packet: %w", err)
	}

	if context.Valid {
		pp.Context = context.String
	}

	return &pp, nil
}

func (d *DB) ClaimTask(ctx context.Context, taskID, modelID string) error {
	query := `
		UPDATE tasks
		SET status = 'in_progress',
		    assigned_to = $1,
		    started_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2 AND status = 'available'
	`

	result, err := d.db.ExecContext(ctx, query, modelID, taskID)
	if err != nil {
		return fmt.Errorf("failed to claim task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task %s not available for claiming", taskID)
	}

	return nil
}

func (d *DB) UpdateTaskStatus(ctx context.Context, taskID string, status types.TaskStatus) error {
	query := `
		UPDATE tasks
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := d.db.ExecContext(ctx, query, status, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

func (d *DB) CreateTaskRun(ctx context.Context, run *types.TaskRun) error {
	query := `
		INSERT INTO task_runs (
			id, task_id, courier, platform, model_id, status,
			tokens_in, tokens_out, chat_url,
			started_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5,
			$6, $7, $8, NOW()
		) RETURNING id
	`

	err := d.db.QueryRowContext(ctx, query,
		run.TaskID, run.Courier, run.Platform, run.ModelID, run.Status,
		run.TokensIn, run.TokensOut, run.ChatURL,
	).Scan(&run.ID)

	if err != nil {
		return fmt.Errorf("failed to create task run: %w", err)
	}

	return nil
}

func (d *DB) CompleteTaskRun(ctx context.Context, runID string, result []byte, errStr string) error {
	query := `
		UPDATE task_runs
		SET status = CASE WHEN $2 = '' THEN 'success' ELSE 'failed' END,
		    result = $1,
		    error = $2,
		    completed_at = NOW()
		WHERE id = $3
	`

	_, err := d.db.ExecContext(ctx, query, result, errStr, runID)
	if err != nil {
		return fmt.Errorf("failed to complete task run: %w", err)
	}

	return nil
}

func (d *DB) GetStuckTasks(ctx context.Context, timeout time.Duration) ([]types.Task, error) {
	query := `
		SELECT id, title, attempts, max_attempts
		FROM tasks
		WHERE status = 'in_progress'
		  AND updated_at < NOW() - $1
	`

	rows, err := d.db.QueryContext(ctx, query, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to query stuck tasks: %w", err)
	}
	defer rows.Close()

	var tasks []types.Task
	for rows.Next() {
		var t types.Task
		err := rows.Scan(&t.ID, &t.Title, &t.Attempts, &t.MaxAttempts)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stuck task: %w", err)
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (d *DB) ResetTask(ctx context.Context, taskID string, escalate bool) error {
	var status types.TaskStatus = types.StatusAvailable
	if escalate {
		status = types.StatusEscalated
	}

	query := `
		UPDATE tasks
		SET status = $1,
		    attempts = attempts + 1,
		    started_at = NULL,
		    updated_at = NOW()
		WHERE id = $2
	`

	_, err := d.db.ExecContext(ctx, query, status, taskID)
	if err != nil {
		return fmt.Errorf("failed to reset task: %w", err)
	}

	return nil
}

func (d *DB) UnlockDependentTasks(ctx context.Context, completedTaskID string) ([]string, error) {
	query := `
		SELECT id FROM tasks
		WHERE status = 'locked'
		  AND dependencies::jsonb @> $1::jsonb
		  AND NOT EXISTS (
		    SELECT 1 FROM tasks dep
		    WHERE dep.id::text IN (
		      SELECT jsonb_array_elements_text($1::jsonb)
		    )
		    AND dep.status != 'merged'
		  )
	`

	depsJSON := fmt.Sprintf(`["%s"]`, completedTaskID)
	
	rows, err := d.db.QueryContext(ctx, query, depsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to find dependent tasks: %w", err)
	}
	defer rows.Close()

	var unlockedIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan dependent task: %w", err)
		}
		unlockedIDs = append(unlockedIDs, id)
	}

	for _, id := range unlockedIDs {
		if err := d.UpdateTaskStatus(ctx, id, types.StatusAvailable); err != nil {
			return nil, fmt.Errorf("failed to unlock task %s: %w", id, err)
		}
	}

	return unlockedIDs, nil
}

func (d *DB) GetModel(ctx context.Context, modelID string) (*types.Model, error) {
	query := `
		SELECT id, name, platform, vendor, context_limit, status,
		       status_reason, access_type, tokens_used,
		       tasks_completed, tasks_failed, success_rate,
		       cooldown_expires_at
		FROM models
		WHERE id = $1
	`

	var m types.Model
	var cooldown sql.NullTime

	err := d.db.QueryRowContext(ctx, query, modelID).Scan(
		&m.ID, &m.Name, &m.Platform, &m.Vendor, &m.ContextLimit,
		&m.Status, &m.StatusReason, &m.AccessType, &m.TokensUsed,
		&m.TasksCompleted, &m.TasksFailed, &m.SuccessRate,
		&cooldown,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	if cooldown.Valid {
		m.CooldownExpires = &cooldown.Time
	}

	return &m, nil
}

func (d *DB) IncrementModelUsage(ctx context.Context, modelID string, tokensIn, tokensOut int, success bool) error {
	query := `
		UPDATE models
		SET tokens_used = tokens_used + $2,
		    tasks_completed = tasks_completed + CASE WHEN $4 THEN 1 ELSE 0 END,
		    tasks_failed = tasks_failed + CASE WHEN $4 THEN 0 ELSE 1 END,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := d.db.ExecContext(ctx, query, modelID, tokensIn+tokensOut, tokensIn, success)
	if err != nil {
		return fmt.Errorf("failed to increment model usage: %w", err)
	}

	return nil
}
