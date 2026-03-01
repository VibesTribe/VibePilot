package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

var defaultRPCAllowlist = map[string]bool{
	"get_available_tasks":         true,
	"get_task_by_id":              true,
	"claim_task":                  true,
	"update_task_status":          true,
	"reset_task":                  true,
	"record_task_run":             true,
	"calculate_roi":               true,
	"get_best_runner":             true,
	"record_runner_result":        true,
	"increment_in_flight":         true,
	"decrement_in_flight":         true,
	"log_orchestrator_event":      true,
	"append_routing_history":      true,
	"record_failure":              true,
	"get_heuristic":               true,
	"upsert_heuristic":            true,
	"get_recent_failures":         true,
	"record_heuristic_result":     true,
	"get_problem_solution":        true,
	"record_solution_result":      true,
	"create_planner_rule":         true,
	"get_planner_rules":           true,
	"record_planner_rule_applied": true,
	"deactivate_planner_rule":     true,
	"create_supervisor_rule":      true,
	"get_supervisor_rules":        true,
	"record_supervisor_rule_hit":  true,
	"create_tester_rule":          true,
	"get_tester_rules":            true,
	"record_tester_rule_hit":      true,
	"archive_runner":              true,
	"boost_runner":                true,
	"revive_runner":               true,
	"log_security_audit":          true,
	"vibes_query":                 true,
	"get_dashboard_stats":         true,
	"create_plan":                 true,
	"update_plan_status":          true,
	"add_council_review":          true,
	"set_council_consensus":       true,
	"create_tasks":                true,
	"create_task_with_packet":     true,
	"record_planner_revision":     true,
	"unlock_dependent_tasks":      true,
	"find_orphaned_sessions":      true,
	"recover_orphaned_session":    true,
	"record_model_failure":        true,
	"record_model_success":        true,
	"check_model_availability":    true,
	"get_model_score_for_task":    true,
	"get_event_checkpoint":        true,
	"update_event_checkpoint":     true,
	"increment_revision_round":    true,
	"check_revision_limit":        true,
	"record_revision_feedback":    true,
	"store_council_reviews":       true,
}

type RPCAllowlist struct {
	mu    sync.RWMutex
	names map[string]bool
}

func NewRPCAllowlist() *RPCAllowlist {
	return &RPCAllowlist{
		names: make(map[string]bool),
	}
}

func (a *RPCAllowlist) Add(name string) {
	a.mu.Lock()
	a.names[name] = true
	a.mu.Unlock()
}

func (a *RPCAllowlist) Remove(name string) {
	a.mu.Lock()
	delete(a.names, name)
	a.mu.Unlock()
}

func (a *RPCAllowlist) Allowed(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.names[name] || defaultRPCAllowlist[name]
}

func (a *RPCAllowlist) List() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	seen := make(map[string]bool)
	var result []string

	for name := range defaultRPCAllowlist {
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	for name := range a.names {
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}

	return result
}

func (d *DB) CallRPC(ctx context.Context, name string, params map[string]any) (json.RawMessage, error) {
	if !d.rpcAllowlist.Allowed(name) {
		return nil, fmt.Errorf("RPC %s not in allowlist", name)
	}

	result, err := d.rpc(ctx, name, params)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(result), nil
}

func (d *DB) CallRPCInto(ctx context.Context, name string, params map[string]any, dest any) error {
	raw, err := d.CallRPC(ctx, name, params)
	if err != nil {
		return err
	}

	if len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, dest)
}

type RPCCall struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

func ParseRPCCall(data string) (*RPCCall, error) {
	var call RPCCall
	if err := json.Unmarshal([]byte(data), &call); err != nil {
		return nil, fmt.Errorf("parse RPC call: %w", err)
	}
	return &call, nil
}
