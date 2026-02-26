package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/vibepilot/governor/internal/db"
)

var invalidFilterChars = regexp.MustCompile(`[<>'"\\;\-]`)

func sanitizeFilterValue(val interface{}) string {
	s := fmt.Sprintf("%v", val)
	s = invalidFilterChars.ReplaceAllString(s, "")
	return s
}

func sanitizeColumnName(name string) string {
	validName := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !validName.MatchString(name) {
		return ""
	}
	return name
}

type DBQueryTool struct {
	db *db.DB
}

func NewDBQueryTool(database *db.DB) *DBQueryTool {
	return &DBQueryTool{db: database}
}

func (t *DBQueryTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	table, ok := args["table"].(string)
	if !ok {
		return nil, fmt.Errorf("table parameter required")
	}
	table = sanitizeColumnName(table)
	if table == "" {
		return nil, fmt.Errorf("invalid table name")
	}

	path := table

	if columns, ok := args["columns"].([]any); ok && len(columns) > 0 {
		var validCols []string
		for _, c := range columns {
			col := sanitizeColumnName(fmt.Sprintf("%v", c))
			if col != "" {
				validCols = append(validCols, col)
			}
		}
		if len(validCols) > 0 {
			path = fmt.Sprintf("%s?select=%s", table, strings.Join(validCols, ","))
		}
	} else {
		path = table + "?select=*"
	}

	if where, ok := args["where"].(map[string]any); ok {
		for key, val := range where {
			col := sanitizeColumnName(key)
			if col == "" {
				continue
			}
			safeVal := sanitizeFilterValue(val)
			path = fmt.Sprintf("%s&%s=eq.%s", path, col, safeVal)
		}
	}

	if limit, ok := args["limit"].(float64); ok && limit > 0 && limit <= 1000 {
		path = fmt.Sprintf("%s&limit=%d", path, int(limit))
	}

	data, err := t.db.REST(ctx, "GET", path, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"data":    json.RawMessage(data),
	})
}

type DBUpdateTool struct {
	db *db.DB
}

func NewDBUpdateTool(database *db.DB) *DBUpdateTool {
	return &DBUpdateTool{db: database}
}

func (t *DBUpdateTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	table, ok := args["table"].(string)
	if !ok {
		return nil, fmt.Errorf("table parameter required")
	}
	table = sanitizeColumnName(table)
	if table == "" {
		return nil, fmt.Errorf("invalid table name")
	}

	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("id parameter required")
	}
	id = sanitizeFilterValue(id)

	data, ok := args["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("data parameter required")
	}

	path := fmt.Sprintf("%s?id=eq.%s", table, id)
	result, err := t.db.REST(ctx, "PATCH", path, data)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"id":      id,
		"result":  json.RawMessage(result),
	})
}

type DBInsertTool struct {
	db *db.DB
}

func NewDBInsertTool(database *db.DB) *DBInsertTool {
	return &DBInsertTool{db: database}
}

func (t *DBInsertTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	table, ok := args["table"].(string)
	if !ok {
		return nil, fmt.Errorf("table parameter required")
	}
	table = sanitizeColumnName(table)
	if table == "" {
		return nil, fmt.Errorf("invalid table name")
	}

	data, ok := args["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("data parameter required")
	}

	result, err := t.db.REST(ctx, "POST", table, data)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"table":   table,
		"result":  json.RawMessage(result),
	})
}

type DBRPCTool struct {
	db *db.DB
}

func NewDBRPCTool(database *db.DB) *DBRPCTool {
	return &DBRPCTool{db: database}
}

func (t *DBRPCTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name parameter required")
	}

	validRPC := regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	if !validRPC.MatchString(name) {
		return nil, fmt.Errorf("invalid RPC name format")
	}

	params, _ := args["params"].(map[string]any)
	if params == nil {
		params = make(map[string]any)
	}

	result, err := t.db.CallRPC(ctx, name, params)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"rpc":     name,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"rpc":     name,
		"result":  result,
	})
}

type MaintenanceCommandTool struct {
	db *db.DB
}

func NewMaintenanceCommandTool(database *db.DB) *MaintenanceCommandTool {
	return &MaintenanceCommandTool{db: database}
}

func (t *MaintenanceCommandTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command parameter required")
	}
	command = sanitizeFilterValue(command)

	params, _ := args["params"].(map[string]any)
	if params == nil {
		params = make(map[string]any)
	}

	rpcParams := map[string]any{
		"p_command": command,
		"p_params":  params,
	}

	result, err := t.db.CallRPC(ctx, "queue_maintenance_command", rpcParams)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"command": command,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"command": command,
		"result":  result,
	})
}
