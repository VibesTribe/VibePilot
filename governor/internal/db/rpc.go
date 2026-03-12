package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

var defaultRPCAllowlist = map[string]bool{
	// Task management
	"get_available_tasks":     true,
	"update_task_status":      true,
	"update_task_assignment":  true,
	"update_task_branch":      true,
	"create_task_with_packet": true,
	"create_task_run":         true,
	"unlock_dependent_tasks":  true,
	"unlock_dependents":       true,
	"increment_task_attempts": true,
	"append_failure_notes":    true,
	"set_failure_reason":      true,
	"claim_task":              true,
	"claim_for_review":        true,
	"transition_task":         true,

	// Plan lifecycle
	"create_plan":              true,
	"update_plan_status":       true,
	"increment_revision_round": true,
	"check_revision_limit":     true,
	"record_revision_feedback": true,
	"record_planner_revision":  true,
	"add_council_review":       true,
	"set_council_consensus":    true,
	"store_council_reviews":    true,

	// Processing state (migration 042)
	"set_processing":           true,
	"clear_processing":         true,
	"find_stale_processing":    true,
	"recover_stale_processing": true,

	// Failure & learning
	"record_failure":              true,
	"record_model_failure":        true,
	"record_model_success":        true,
	"get_model_score_for_task":    true,
	"check_model_availability":    true,
	"check_platform_availability": true,

	// Planner learning
	"create_planner_rule":         true,
	"get_planner_rules":           true,
	"record_planner_rule_applied": true,
	"deactivate_planner_rule":     true,

	// Supervisor learning
	"create_supervisor_rule":           true,
	"get_supervisor_rules":             true,
	"record_supervisor_rule":           true,
	"record_supervisor_rule_triggered": true,

	// Tester learning
	"create_tester_rule":     true,
	"get_tester_rules":       true,
	"record_tester_rule_hit": true,

	// Test results (migration 043)
	"create_test_result":        true,
	"update_test_result_status": true,

	// Research flow
	"create_research_suggestion":        true,
	"update_research_suggestion_status": true,

	// Maintenance
	"create_maintenance_command": true,

	// Session recovery
	"find_orphaned_sessions":      true,
	"recover_orphaned_session":    true,
	"find_pending_resource_tasks": true,

	// Event tracking
	"get_event_checkpoint":    true,
	"update_event_checkpoint": true,

	// Task checkpoints (migration 057)
	"save_checkpoint":             true,
	"load_checkpoint":             true,
	"delete_checkpoint":           true,
	"find_tasks_with_checkpoints": true,

	// Heuristics
	"get_heuristic":              true,
	"upsert_heuristic":           true,
	"get_recent_failures":        true,
	"record_heuristic_result":    true,
	"get_problem_solution":       true,
	"record_solution_result":     true,
	"record_solution_on_success": true,

	// Runner management
	"archive_runner":       true,
	"boost_runner":         true,
	"revive_runner":        true,
	"get_best_runner":      true,
	"record_runner_result": true,
	"increment_in_flight":  true,
	"decrement_in_flight":  true,

	// Logging
	"log_orchestrator_event": true,
	"log_security_audit":     true,
	"append_routing_history": true,

	// State tracking (migration 050)
	"record_state_transition":   true,
	"record_performance_metric": true,
	"get_latest_state":          true,

	// Vibes interface
	"vibes_query": true,

	// Dashboard (may be used by frontend)
	"get_dashboard_stats": true,

	// Task creation (migration 077)
	"create_task_if_not_exists": true,
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
