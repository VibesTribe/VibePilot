package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultHTTPTimeoutSecs  = 30
	DefaultErrorTruncateLen = 200
)

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
	path := table + "?select=*"

	for key, val := range filters {
		if key == "limit" {
			path = path + "&limit=" + fmt.Sprintf("%v", val)
		} else if key == "order" {
			path = path + "&order=" + fmt.Sprintf("%v", val)
		} else {
			path = path + "&" + key + "=eq." + fmt.Sprintf("%v", val)
		}
	}

	data, err := d.REST(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

func (d *DB) Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error) {
	result, err := d.REST(ctx, "POST", table, data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (d *DB) Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error) {
	path := table + "?id=eq." + id
	result, err := d.REST(ctx, "PATCH", path, data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (d *DB) Delete(ctx context.Context, table, id string) error {
	path := table + "?id=eq." + id
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
