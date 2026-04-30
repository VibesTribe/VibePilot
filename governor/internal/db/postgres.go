package db
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB implements Database using native PostgreSQL via pgx.
// Calls stored functions directly via SQL instead of PostgREST HTTP.
type PostgresDB struct {
	pool         *pgxpool.Pool
	rpcAllowlist *RPCAllowlist
}

// Compile-time proof that PostgresDB satisfies Database.
var _ Database = (*PostgresDB)(nil)

// NewPostgres creates a new PostgresDB from a standard PostgreSQL connection string.
// Format: "postgres://user:pass@host:5432/dbname?sslmode=disable"
func NewPostgres(ctx context.Context, connString string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresDB{
		pool:         pool,
		rpcAllowlist: NewRPCAllowlist(),
	}, nil
}

func (p *PostgresDB) Close() error {
	p.pool.Close()
	return nil
}

// --- Core CRUD ---

func (p *PostgresDB) Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	query, args := p.buildSelectQuery(table, filters)
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	defer rows.Close()

	return rowsToJSON(rows)
}

func (p *PostgresDB) Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("insert: no data provided")
	}

	cols, vals, argIdx := p.buildInsertParts(data)
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *", table, cols, vals)

	rows, err := p.pool.Query(ctx, query, argIdx...)
	if err != nil {
		return nil, fmt.Errorf("postgres insert: %w", err)
	}
	defer rows.Close()

	return rowsToJSON(rows)
}

func (p *PostgresDB) Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("update: no data provided")
	}

	setClause, args := p.buildUpdateSet(data, 1)
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d RETURNING *", table, setClause, len(args))

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres update: %w", err)
	}
	defer rows.Close()

	return rowsToJSON(rows)
}

func (p *PostgresDB) Delete(ctx context.Context, table, id string) error {
	if !isValidTableName(table) {
		return fmt.Errorf("invalid table name: %s", table)
	}

	_, err := p.pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = $1", table), id)
	if err != nil {
		return fmt.Errorf("postgres delete: %w", err)
	}
	return nil
}

// --- RPC ---

func (p *PostgresDB) RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error) {
	if !p.rpcAllowlist.Allowed(name) {
		return nil, fmt.Errorf("RPC %s not in allowlist", name)
	}
	return p.callFunction(ctx, name, params)
}

func (p *PostgresDB) CallRPC(ctx context.Context, name string, params map[string]any) (json.RawMessage, error) {
	if !p.rpcAllowlist.Allowed(name) {
		return nil, fmt.Errorf("RPC %s not in allowlist", name)
	}
	result, err := p.callFunction(ctx, name, params)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (p *PostgresDB) CallRPCInto(ctx context.Context, name string, params map[string]any, dest any) error {
	raw, err := p.CallRPC(ctx, name, params)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

// callFunction executes a stored function, matching PostgREST return formats.
// Strategy: try SELECT * FROM fn() first (table-returning), fallback to SELECT fn() (scalar/void).
func (p *PostgresDB) callFunction(ctx context.Context, name string, params map[string]any) ([]byte, error) {
	paramStr, args := p.buildFunctionParams(params)

	// Phase 1: Try table-returning form: SELECT * FROM fn(p1=>$1, ...)
	query1 := fmt.Sprintf("SELECT * FROM %s(%s)", name, paramStr)
	rows, err := p.pool.Query(ctx, query1, args...)
	if err != nil {
		// Phase 2: Try scalar/void form: SELECT fn(p1=>$1, ...)
		query2 := fmt.Sprintf("SELECT %s(%s)", name, paramStr)
		row := p.pool.QueryRow(ctx, query2, args...)

		var result any
		if scanErr := row.Scan(&result); scanErr != nil {
			// void return or no rows — return null
			return []byte("null"), nil
		}
		return json.Marshal(result)
	}
	defer rows.Close()

	return rowsToJSON(rows)
}

// --- State machine helpers ---

func (p *PostgresDB) RecordStateTransition(ctx context.Context, entityType, entityID, fromState, toState, reason string, metadata map[string]any) error {
	_, err := p.RPC(ctx, "record_state_transition", map[string]any{
		"p_entity_type": entityType,
		"p_entity_id":   entityID,
		"p_from_state":  fromState,
		"p_to_state":    toState,
		"p_reason":      reason,
		"p_metadata":    metadata,
	})
	if err != nil {
		return fmt.Errorf("record state transition: %w", err)
	}
	return nil
}

func (p *PostgresDB) RecordPerformanceMetric(ctx context.Context, metricType, entityID string, duration time.Duration, success bool, metadata map[string]any) error {
	_, err := p.RPC(ctx, "record_performance_metric", map[string]any{
		"p_metric_type":      metricType,
		"p_entity_id":        entityID,
		"p_duration_seconds": duration.Seconds(),
		"p_success":          success,
		"p_metadata":         metadata,
	})
	if err != nil {
		return fmt.Errorf("record performance metric: %w", err)
	}
	return nil
}

func (p *PostgresDB) GetLatestState(ctx context.Context, entityType, entityID string) (toState string, reason string, createdAt time.Time, err error) {
	result, err := p.RPC(ctx, "get_latest_state", map[string]any{
		"p_entity_type": entityType,
		"p_entity_id":   entityID,
	})
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("get latest state: %w", err)
	}

	var states []struct {
		ToState          string    `json:"to_state"`
		TransitionReason string    `json:"transition_reason"`
		CreatedAt        time.Time `json:"created_at"`
	}

	if err := json.Unmarshal(result, &states); err != nil {
		return "", "", time.Time{}, fmt.Errorf("parse latest state: %w", err)
	}

	if len(states) == 0 {
		return "", "", time.Time{}, nil
	}

	return states[0].ToState, states[0].TransitionReason, states[0].CreatedAt, nil
}

func (p *PostgresDB) ClearProcessingAndRecordTransition(ctx context.Context, table, id, fromState, toState, reason string) error {
	_, err := p.RPC(ctx, "clear_processing", map[string]any{
		"p_table": table,
		"p_id":    id,
	})
	if err != nil {
		return fmt.Errorf("clear processing: %w", err)
	}

	if err := p.RecordStateTransition(ctx, table, id, fromState, toState, reason, nil); err != nil {
		return nil
	}

	return nil
}

// --- Domain queries ---

func (p *PostgresDB) GetDestination(ctx context.Context, id string) (*Destination, error) {
	rows, err := p.pool.Query(ctx,
		"SELECT id, name, type, status, command, endpoint, api_key_ref, models_available, timeout_seconds FROM destinations WHERE id = $1 LIMIT 1",
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres get destination: %w", err)
	}
	defer rows.Close()

	var dest Destination
	if !rows.Next() {
		return nil, fmt.Errorf("destination %s not found", id)
	}

	// Handle models_available which may be null or an array
	var modelsJSON []byte
	if err := rows.Scan(&dest.ID, &dest.Name, &dest.Type, &dest.Status, &dest.Command,
		&dest.Endpoint, &dest.APIKeyRef, &modelsJSON, &dest.TimeoutSeconds); err != nil {
		return nil, fmt.Errorf("scan destination: %w", err)
	}

	if len(modelsJSON) > 0 && string(modelsJSON) != "null" {
		_ = json.Unmarshal(modelsJSON, &dest.Models)
	}

	return &dest, nil
}

func (p *PostgresDB) GetRunners(ctx context.Context) ([]Runner, error) {
	rows, err := p.pool.Query(ctx,
		"SELECT id, model_id, tool_id, status, cost_priority, depreciation_score FROM runners WHERE status = 'active'",
	)
	if err != nil {
		return nil, fmt.Errorf("postgres get runners: %w", err)
	}
	defer rows.Close()

	var runners []Runner
	for rows.Next() {
		var r Runner
		if err := rows.Scan(&r.ID, &r.ModelID, &r.ToolID, &r.Status, &r.CostPriority, &r.Depreciation); err != nil {
			return nil, fmt.Errorf("scan runner: %w", err)
		}
		runners = append(runners, r)
	}
	return runners, nil
}

func (p *PostgresDB) GetTaskPacket(ctx context.Context, taskID string) (*TaskPacket, error) {
	// Try task_packets table first (populated by Supabase RPC in cloud mode)
	rows, err := p.pool.Query(ctx,
		"SELECT task_id, prompt, tech_spec, expected_output, context, version FROM task_packets WHERE task_id = $1 LIMIT 1",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres get task packet: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var tp TaskPacket
		if err := rows.Scan(&tp.TaskID, &tp.Prompt, &tp.TechSpec, &tp.ExpectedOutput, &tp.Context, &tp.Version); err != nil {
			return nil, fmt.Errorf("scan task packet: %w", err)
		}
		return &tp, nil
	}

	// Fallback: extract from tasks.result JSONB column (local Postgres mode).
	// Tasks are created with prompt_packet and expected_output in result JSONB.
	var resultJSON []byte
	err = p.pool.QueryRow(ctx,
		"SELECT result FROM tasks WHERE id = $1 LIMIT 1",
		taskID,
	).Scan(&resultJSON)
	if err != nil {
		return nil, fmt.Errorf("task packet not found for task %s (checked task_packets and tasks)", taskID)
	}

	var resultMap map[string]json.RawMessage
	if err := json.Unmarshal(resultJSON, &resultMap); err != nil {
		return nil, fmt.Errorf("parse task result JSONB: %w", err)
	}

	tp := &TaskPacket{TaskID: taskID}
	if raw, ok := resultMap["prompt_packet"]; ok {
		// prompt_packet may be a JSON string or plain text
		var s string
		if json.Unmarshal(raw, &s) == nil {
			tp.Prompt = s
		} else {
			tp.Prompt = string(raw)
		}
	}
	if raw, ok := resultMap["expected_output"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			tp.ExpectedOutput = s
		} else {
			tp.ExpectedOutput = string(raw)
		}
	}
	if raw, ok := resultMap["tech_spec"]; ok {
		tp.TechSpec = raw
	}
	if raw, ok := resultMap["context"]; ok {
		tp.Context = raw
	}

	if tp.Prompt == "" {
		return nil, fmt.Errorf("task packet has empty prompt_packet for task %s", taskID)
	}

	return tp, nil
}

// --- Query builder helpers ---

// buildSelectQuery translates PostgREST-style filter maps to SQL WHERE clauses.
// NOTE: ORDER BY and LIMIT are collected separately and appended AFTER WHERE
// to avoid SQL syntax errors from Go's random map iteration order.
func (p *PostgresDB) buildSelectQuery(table string, filters map[string]any) (string, []any) {
	// Collect parts separately to ensure correct SQL order:
	// SELECT ... WHERE ... ORDER BY ... LIMIT

	// "select" overrides the default "SELECT *" — PostgREST compatibility.
	// Supports comma-separated column lists: "select": "id,status,routing_flag_reason"
	colsClause := "*"
	if cols, ok := filters["select"]; ok {
		if s, ok := cols.(string); ok && s != "" {
			// Validate each column name to prevent injection
			parts := strings.Split(s, ",")
			valid := true
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if !isValidTableName(part) {
					valid = false
					break
				}
			}
			if valid {
				colsClause = s
			}
		}
	}

	query := "SELECT " + colsClause + " FROM " + table
	var whereClauses []string
	var args []any
	var orderBy, limitStr string
	argIdx := 1

	for key, val := range filters {
		// Special keys: select, limit, order, or
		switch key {
		case "select":
			// Already handled above
			continue
		case "limit":
			if n, ok := toInt(val); ok && n > 0 {
				limitStr = fmt.Sprintf(" LIMIT %d", n)
			}
			continue
		case "order":
			s := fmt.Sprintf("%v", val)
			// PostgREST: "created_at.desc.nullsfirst"
			parts := strings.SplitN(s, ".", 3)
			col := parts[0]
			dir := "ASC"
			if len(parts) > 1 {
				dir = strings.ToUpper(parts[1])
			}
			if dir != "ASC" && dir != "DESC" {
				dir = "ASC"
			}
			orderBy = fmt.Sprintf(" ORDER BY %s %s", col, dir)
			continue
		case "or":
			// PostgREST: "or=(status.eq.review,status.eq.testing)"
			// For now, handle via direct SQL would be complex.
			// The existing callers that use "or" are:
			//   recovery.go: "status": "in.(review,testing)" — that's actually the "in" operator
			// So this case may not be needed. Skip safely.
			continue
		}

		if !isValidTableName(key) {
			continue
		}

		valStr := fmt.Sprintf("%v", val)
		clause, nextIdx, newArgs := buildFilterClause(key, valStr, argIdx)
		if clause != "" {
			whereClauses = append(whereClauses, clause)
			args = append(args, newArgs...)
			argIdx = nextIdx
		}
	}

	// Assemble in correct SQL order: WHERE -> ORDER BY -> LIMIT
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	if orderBy != "" {
		query += orderBy
	}
	if limitStr != "" {
		query += limitStr
	}

	return query, args
}

// buildFilterClause translates PostgREST filter operators to SQL.
// Returns (clause, nextArgIdx, args).
func buildFilterClause(col, val string, argIdx int) (string, int, []any) {
	operators := map[string]string{
		"is.":   "IS",
		"not.":  "!=",
		"lt.":   "<",
		"lte.":  "<=",
		"gt.":   ">",
		"gte.":  ">=",
		"like.": "LIKE",
	}

	// Handle "in.(...)" — PostgREST in operator
	if strings.HasPrefix(val, "in.") {
		items := strings.TrimPrefix(val, "in.")
		items = strings.Trim(items, "()")
		parts := strings.Split(items, ",")
		placeholders := make([]string, len(parts))
		args := make([]any, len(parts))
		for i, part := range parts {
			placeholders[i] = "$" + strconv.Itoa(argIdx+i)
			args[i] = strings.TrimSpace(part)
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ",")), argIdx + len(parts), args
	}

	for prefix, op := range operators {
		if strings.HasPrefix(val, prefix) {
			actualVal := strings.TrimPrefix(val, prefix)
			if prefix == "is." && (actualVal == "null" || actualVal == "NULL") {
				return fmt.Sprintf("%s IS NULL", col), argIdx, nil
			}
			return fmt.Sprintf("%s %s $%d", col, op, argIdx), argIdx + 1, []any{actualVal}
		}
	}

	// Default: eq (equals)
	return fmt.Sprintf("%s = $%d", col, argIdx), argIdx + 1, []any{val}
}

// buildFunctionParams converts a params map to "p_name=>$1, ..." form.
func (p *PostgresDB) buildFunctionParams(params map[string]any) (string, []any) {
	if len(params) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(params))
	vals := make([]any, 0, len(params))
	for k, v := range params {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s => $%d", k, i+1)
	}

	return strings.Join(parts, ", "), vals
}

// buildInsertParts returns column names, value placeholders ($1, $2...), and args.
func (p *PostgresDB) buildInsertParts(data map[string]any) (string, string, []any) {
	keys := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	for k, v := range data {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	placeholders := make([]string, len(keys))
	for i := range keys {
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}

	return strings.Join(keys, ", "), strings.Join(placeholders, ", "), vals
}

// buildUpdateSet returns "col1=$1, col2=$2" and the args.
func (p *PostgresDB) buildUpdateSet(data map[string]any, startIdx int) (string, []any) {
	keys := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	for k, v := range data {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s = $%d", k, startIdx+i)
	}

	return strings.Join(parts, ", "), vals
}

// --- Result conversion ---

// rowsToJSON converts pgx rows to JSON matching PostgREST format:
//   - 0 rows: [] (empty array)
//   - 1 row: depends on caller context, but we always return array for consistency
//   - N rows: JSON array of objects
func rowsToJSON(rows pgx.Rows) (json.RawMessage, error) {
	fieldDescriptions := rows.FieldDescriptions()
	colNames := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		colNames[i] = string(fd.Name)
	}

	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read row values: %w", err)
		}

		row := make(map[string]any, len(colNames))
		for i, col := range colNames {
			row[col] = convertValue(values[i])
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]any{}
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal rows: %w", err)
	}
	return json.RawMessage(data), nil
}

// convertValue ensures pgx values are JSON-compatible.
// Handles pgtype types that don't serialize cleanly.
func convertValue(v any) any {
	if v == nil {
		return nil
	}

	// pgx returns pgtype types for some columns.
	// Most common ones that need conversion:
	switch val := v.(type) {
	case []byte:
		// Could be JSONB, text, or binary data.
		// Try parsing as JSON first.
		s := string(val)
		if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			var parsed any
			if json.Unmarshal(val, &parsed) == nil {
				return parsed
			}
		}
		return s
	case [16]byte:
		// UUID columns come as [16]byte from pgx when not using pgtype codec.
		// Convert to standard UUID string format.
		b := val[:]
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			binary.BigEndian.Uint32(b[0:4]),
			binary.BigEndian.Uint16(b[4:6]),
			binary.BigEndian.Uint16(b[6:8]),
			binary.BigEndian.Uint16(b[8:10]),
			b[10:])
	case pgtype.UUID:
		// pgtype.UUID serializes as [16]byte array by default — must convert to string
		return val.String()
	case pgtype.Timestamptz:
		if !val.Valid {
			return nil
		}
		return val.Time.Format(time.RFC3339Nano)
	case pgtype.Timestamp:
		if !val.Valid {
			return nil
		}
		return val.Time.Format(time.RFC3339Nano)
	case pgtype.Numeric:
		if !val.Valid {
			return nil
		}
		f64, err := val.Float64Value()
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return f64.Float64
	case pgtype.Text:
		if !val.Valid {
			return nil
		}
		return val.String
	case pgtype.Bool:
		if !val.Valid {
			return nil
		}
		return val.Bool
	case fmt.Stringer:
		return val.String()
	default:
		return v
	}
}

// toInt converts float64 (JSON numbers) or int to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
