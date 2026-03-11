package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultHTTPTimeoutSecs  = 30
	DefaultErrorTruncateLen = 200
)

var tableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func isValidTableName(name string) bool {
	return tableNameRegex.MatchString(name)
}

type DBConfig struct {
	HTTPTimeoutSecs  int
	ErrorTruncateLen int
}

type DB struct {
	url          string
	key          string
	client       *http.Client
	rpcAllowlist *RPCAllowlist
	config       *DBConfig
}

func New(url, key string) *DB {
	return NewWithConfig(url, key, nil)
}

func NewWithConfig(url, key string, cfg *DBConfig) *DB {
	if cfg == nil {
		cfg = &DBConfig{
			HTTPTimeoutSecs:  DefaultHTTPTimeoutSecs,
			ErrorTruncateLen: DefaultErrorTruncateLen,
		}
	}

	timeout := DefaultHTTPTimeoutSecs
	if cfg.HTTPTimeoutSecs > 0 {
		timeout = cfg.HTTPTimeoutSecs
	}

	return &DB{
		url:          url,
		key:          key,
		rpcAllowlist: NewRPCAllowlist(),
		config:       cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (d *DB) Close() error {
	d.client.CloseIdleConnections()
	return nil
}

func (d *DB) REST(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	return d.RESTWithHeaders(ctx, method, path, body, nil)
}

func (d *DB) RESTWithHeaders(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := d.url + "/rest/v1/" + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apikey", d.key)
	req.Header.Set("Authorization", "Bearer "+d.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode >= 400 {
		truncateLen := d.config.ErrorTruncateLen
		if truncateLen <= 0 {
			truncateLen = DefaultErrorTruncateLen
		}
		errBody := string(data)
		if len(errBody) > truncateLen {
			errBody = errBody[:truncateLen] + "...(truncated)"
		}
		return nil, fmt.Errorf("supabase %d: %s", resp.StatusCode, errBody)
	}

	return data, nil
}

func (d *DB) rpc(ctx context.Context, name string, params interface{}) ([]byte, error) {
	return d.REST(ctx, "POST", "rpc/"+name, params)
}

func (d *DB) RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error) {
	if !d.rpcAllowlist.Allowed(name) {
		return nil, fmt.Errorf("RPC %s not in allowlist", name)
	}
	return d.rpc(ctx, name, params)
}

func (d *DB) Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	path := table + "?select=*"

	for key, val := range filters {
		if !isValidTableName(key) && key != "limit" && key != "order" && key != "or" {
			continue
		}
		switch key {
		case "limit":
			path = path + "&limit=" + url.QueryEscape(fmt.Sprintf("%v", val))
		case "order":
			path = path + "&order=" + url.QueryEscape(fmt.Sprintf("%v", val))
		case "or":
			path = path + "&or=(" + url.QueryEscape(fmt.Sprintf("%v", val)) + ")"
		default:
			valStr := fmt.Sprintf("%v", val)
			if strings.HasPrefix(valStr, "is.") || strings.HasPrefix(valStr, "not.") || strings.HasPrefix(valStr, "lt.") || strings.HasPrefix(valStr, "lte.") || strings.HasPrefix(valStr, "gt.") || strings.HasPrefix(valStr, "gte.") || strings.HasPrefix(valStr, "like.") || strings.HasPrefix(valStr, "ilike.") || strings.HasPrefix(valStr, "in.") {
				path = path + "&" + url.QueryEscape(key) + "=" + url.QueryEscape(valStr)
			} else {
				path = path + "&" + url.QueryEscape(key) + "=eq." + url.QueryEscape(valStr)
			}
		}
	}

	data, err := d.REST(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

func (d *DB) Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}
	result, err := d.REST(ctx, "POST", table, data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (d *DB) Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}
	path := table + "?id=eq." + url.QueryEscape(id)
	result, err := d.REST(ctx, "PATCH", path, data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (d *DB) Delete(ctx context.Context, table, id string) error {
	if !isValidTableName(table) {
		return fmt.Errorf("invalid table name: %s", table)
	}
	path := table + "?id=eq." + url.QueryEscape(id)
	_, err := d.REST(ctx, "DELETE", path, nil)
	return err
}

type Destination struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	Command        string   `json:"command,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	APIKeyRef      string   `json:"api_key_ref,omitempty"`
	Models         []string `json:"models_available,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

func (d *DB) GetDestination(ctx context.Context, id string) (*Destination, error) {
	data, err := d.REST(ctx, "GET", "destinations?id=eq."+id+"&limit=1", nil)
	if err != nil {
		return nil, err
	}

	var dests []Destination
	if err := json.Unmarshal(data, &dests); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(dests) == 0 {
		return nil, fmt.Errorf("destination %s not found", id)
	}

	return &dests[0], nil
}

type Runner struct {
	ID           string  `json:"id"`
	ModelID      string  `json:"model_id"`
	ToolID       string  `json:"tool_id"`
	Status       string  `json:"status"`
	CostPriority int     `json:"cost_priority"`
	Depreciation float64 `json:"depreciation_score"`
}

func (d *DB) GetRunners(ctx context.Context) ([]Runner, error) {
	data, err := d.REST(ctx, "GET", "runners?status=eq.active&select=id,model_id,tool_id,status,cost_priority,depreciation_score", nil)
	if err != nil {
		return nil, err
	}

	var runners []Runner
	if err := json.Unmarshal(data, &runners); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return runners, nil
}

type TaskPacket struct {
	TaskID         string          `json:"task_id"`
	Prompt         string          `json:"prompt"`
	TechSpec       json.RawMessage `json:"tech_spec,omitempty"`
	ExpectedOutput string          `json:"expected_output,omitempty"`
	Context        json.RawMessage `json:"context,omitempty"`
	Version        int             `json:"version,omitempty"`
}

func (d *DB) GetTaskPacket(ctx context.Context, taskID string) (*TaskPacket, error) {
	data, err := d.REST(ctx, "GET", "task_packets?task_id=eq."+taskID+"&limit=1", nil)
	if err != nil {
		return nil, err
	}

	var packets []TaskPacket
	if err := json.Unmarshal(data, &packets); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(packets) == 0 {
		return nil, fmt.Errorf("task packet not found for task %s", taskID)
	}

	return &packets[0], nil
}
