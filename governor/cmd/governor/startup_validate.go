package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// startupValidate checks all dependencies before the governor starts accepting work.
// Returns the number of errors found. Logs each issue with a specific fix suggestion.
func startupValidate(configDir string, database interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errors := 0

	// 1. Config directory exists and has required files
	errors += validateConfigDir(configDir)

	// 2. Prompts directory exists
	errors += validatePromptsDir(configDir)

	// 3. Active connector commands are actually available in PATH
	errors += validateConnectorCommands(configDir)

	// 4. Required Supabase RPCs exist
	errors += validateRPCs(ctx, database)

	// 5. Agent IDs used in handlers match config
	errors += validateAgentIDs(configDir)

	if errors > 0 {
		log.Printf("[Startup] FAILED: %d issue(s) found. Fix above before deploying tasks.", errors)
	} else {
		log.Printf("[Startup] All validations passed")
	}

	return errors
}

func validateConfigDir(configDir string) int {
	errors := 0
	required := []string{
		"connectors.json",
		"agents.json",
		"models.json",
		"system.json",
		"routing.json",
	}

	for _, f := range required {
		path := filepath.Join(configDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("[Startup] MISSING: config/%s not found at %s", f, configDir)
			errors++
		}
	}

	// Check pipelines subdirectory
	pipelinesDir := filepath.Join(configDir, "pipelines")
	if info, err := os.Stat(pipelinesDir); err != nil || !info.IsDir() {
		log.Printf("[Startup] WARNING: config/pipelines/ directory not found at %s", configDir)
	}

	return errors
}

func validatePromptsDir(configDir string) int {
	// Prompts dir is typically at GOVERNOR_PROMPTS_DIR or ../prompts relative to config
	promptsDir := os.Getenv("GOVERNOR_PROMPTS_DIR")
	if promptsDir == "" {
		// Try sibling directory: if config is at ./config, prompts is at ../prompts
		promptsDir = filepath.Join(filepath.Dir(configDir), "prompts")
	}

	info, err := os.Stat(promptsDir)
	if err != nil || !info.IsDir() {
		log.Printf("[Startup] MISSING: prompts directory not found (tried %s). Set GOVERNOR_PROMPTS_DIR or create directory.", promptsDir)
		return 1
	}

	// Check for essential prompt files
	essential := []string{"planner.md", "supervisor.md", "task_runner_simple.md"}
	missing := []string{}
	for _, f := range essential {
		if _, err := os.Stat(filepath.Join(promptsDir, f)); err != nil {
			missing = append(missing, f)
		}
	}

	if len(missing) > 0 {
		log.Printf("[Startup] WARNING: missing prompt files: %s", strings.Join(missing, ", "))
	}

	return 0
}

func validateConnectorCommands(configDir string) int {
	errors := 0

	// Load connectors config to find active CLI connectors
	type connectorStub struct {
		ID      string   `json:"id"`
		Type    string   `json:"type"`
		Status  string   `json:"status"`
		Command string   `json:"command"`
		CLIArgs []string `json:"cli_args"`
	}
	type connectorsConfig struct {
		Connectors []connectorStub `json:"destinations"`
	}

	cfg, err := loadJSONFile[connectorsConfig](filepath.Join(configDir, "connectors.json"))
	if err != nil {
		log.Printf("[Startup] ERROR: cannot parse connectors.json: %v", err)
		return 1
	}

	activeCLI := 0
	for _, c := range cfg.Connectors {
		if c.Status == "active" && c.Type == "cli" {
			activeCLI++
			// Check command is in PATH
			path, err := exec.LookPath(c.Command)
			if err != nil {
				log.Printf("[Startup] ERROR: active connector '%s' command '%s' not found in PATH", c.ID, c.Command)
				errors++
			} else {
				log.Printf("[Startup] OK: connector '%s' → %s", c.ID, path)
			}

			// Check for {PROMPT} placeholder in args
			hasPrompt := false
			for _, arg := range c.CLIArgs {
				if strings.Contains(arg, "{PROMPT}") {
					hasPrompt = true
					break
				}
			}
			if !hasPrompt && len(c.CLIArgs) > 0 {
				log.Printf("[Startup] WARNING: connector '%s' has CLI args but no {PROMPT} placeholder -- prompt won't be passed", c.ID)
			}
		}
	}

	if activeCLI == 0 {
		log.Printf("[Startup] ERROR: no active CLI connectors configured -- no agent can execute tasks")
		errors++
	}

	return errors
}

func validateRPCs(ctx context.Context, database interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}) int {
	// RPCs required for the core pipeline: plan → supervisor → task → review
	// These MUST exist or the pipeline breaks silently
	requiredRPCs := []string{
		"update_plan_status",
		"claim_task",
		"transition_task",
		"create_task_run",
		"record_model_success",
		"record_model_failure",
		"set_processing",
		"clear_processing",
		"record_performance_metric",
		"record_failure",
		"calculate_run_costs",
		"claim_for_review",
		"get_planner_rules",
		"save_checkpoint",
		"load_checkpoint",
		"delete_checkpoint",
		"find_stale_processing",
		"recover_stale_processing",
		"update_task_branch",
		"unlock_dependent_tasks",
		"get_next_task_number_for_slice",
		"log_orchestrator_event",
		"store_council_reviews",
		"set_council_consensus",
		"record_planner_revision",
		"record_revision_feedback",
		"create_maintenance_command",
		"check_platform_availability",
		"get_model_score_for_task",
		"get_problem_solution",
		"recover_orphaned_session",
		"get_latest_state",
		"record_state_transition",
		"log_security_audit",
		"update_research_suggestion_status",
	}

	// Optional RPCs -- not yet wired in Go code, warn only
	optionalRPCs := []string{
		"get_supervisor_rules",
		"get_slice_task_info",
		"find_tasks_with_checkpoints",
		"store_memory",
		"recall_memories",
		"create_planner_rule",
		"update_maintenance_command_status",
		"queue_maintenance_command",
		"get_heuristic",
		"get_recent_failures",
		"get_tester_rules",
		"find_orphaned_sessions",
		"find_pending_resource_tasks",
		"get_model_performance",
		"get_failure_patterns",
		"get_latest_state",
		"record_state_transition",
		"log_security_audit",
		"update_research_suggestion_status",
	}

	errors := 0

	// Probe each RPC with matching params to check existence.
	// We can't query pg_proc via REST API, so we call each RPC with safe dummy params.
	// If it returns 404/PGRST202, the function doesn't exist.
	// Any other error (FK violation, no rows, etc.) means it EXISTS and is working.
	rpcTestParams := map[string]map[string]interface{}{
		"update_plan_status":       {"p_plan_id": "00000000-0000-0000-0000-000000000000", "p_status": "test"},
		"claim_task":               {"p_task_id": "00000000-0000-0000-0000-000000000000", "p_worker_id": "test", "p_model_id": "test", "p_routing_flag": "test"},
		"transition_task":          {"p_task_id": "00000000-0000-0000-0000-000000000000", "p_new_status": "test"},
		"create_task_run":          {"p_task_id": "00000000-0000-0000-0000-000000000000", "p_status": "test", "p_model_id": "test", "p_connector_id": "test"},
		"record_model_success":     {"p_model_id": "test"},
		"record_model_failure":     {"p_model_id": "test"},
		"set_processing":           {"p_table": "tasks", "p_id": "00000000-0000-0000-0000-000000000000"},
		"clear_processing":         {"p_table": "tasks", "p_id": "00000000-0000-0000-0000-000000000000"},
		"record_performance_metric": {"p_metric_type": "test", "p_metric_name": "test"},
		"record_failure":           {"p_task_id": "00000000-0000-0000-0000-000000000000"},
		"calculate_run_costs":      {"p_run_id": "00000000-0000-0000-0000-000000000000"},
		"claim_for_review":         {"p_task_id": "00000000-0000-0000-0000-000000000000"},
		"get_planner_rules":        {"p_applies_to": "test"},
		"save_checkpoint":          {"p_task_id": "00000000-0000-0000-0000-000000000000", "p_step": "test"},
		"load_checkpoint":          {"p_task_id": "00000000-0000-0000-0000-000000000000"},
		"delete_checkpoint":        {"p_task_id": "00000000-0000-0000-0000-000000000000"},
		"find_stale_processing":    {"p_table": "tasks"},
		"recover_stale_processing": {"p_table": "tasks", "p_id": "00000000-0000-0000-0000-000000000000"},
		"update_task_branch":       {"p_task_id": "00000000-0000-0000-0000-000000000000", "p_branch_name": "test"},
		"unlock_dependent_tasks":   {"p_completed_task_id": "00000000-0000-0000-0000-000000000000"},
		"get_next_task_number_for_slice": {"p_slice_id": "00000000-0000-0000-0000-000000000000"},
		"log_orchestrator_event":   {"p_event_type": "test"},
		"store_council_reviews":    {"p_plan_id": "00000000-0000-0000-0000-000000000000", "p_mode": "test"},
		"set_council_consensus":    {"p_plan_id": "00000000-0000-0000-0000-000000000000", "p_consensus": "test"},
		"record_planner_revision":  {"p_plan_id": "00000000-0000-0000-0000-000000000000"},
		"record_revision_feedback": {"p_plan_id": "00000000-0000-0000-0000-000000000000"},
		"create_maintenance_command": {"p_command_type": "test"},
		"check_platform_availability": {"p_platform_id": "test"},
		"get_model_score_for_task": {"p_model_id": "test"},
		"get_problem_solution":     {"p_failure_type": "test"},
		"recover_orphaned_session": {"p_session_id": "test"},
		"get_latest_state":         {"p_entity_type": "test", "p_entity_id": "test"},
		"record_state_transition":  {"p_entity_type": "test", "p_entity_id": "test"},
		"log_security_audit":       {"p_operation": "test"},
		"update_research_suggestion_status": {"p_id": "00000000-0000-0000-0000-000000000000", "p_status": "test"},
	}

	missing := []string{}
	for _, rpc := range requiredRPCs {
		params, ok := rpcTestParams[rpc]
		if !ok {
			params = map[string]interface{}{}
		}
		_, err := database.RPC(ctx, rpc, params)
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "PGRST202") {
				missing = append(missing, rpc)
			}
			// Other errors (FK violations, not found rows) mean the RPC exists
		}
	}

	if len(missing) > 0 {
		log.Printf("[Startup] ERROR: %d required Supabase RPCs missing: %s", len(missing), strings.Join(missing, ", "))
		log.Printf("[Startup] FIX: Run missing migrations from https://github.com/VibesTribe/VibePilot/tree/main/migrations")
		errors += len(missing)
	}

	// Check optional RPCs -- just warn
	optMissing := []string{}
	for _, rpc := range optionalRPCs {
		_, err := database.RPC(ctx, rpc, map[string]interface{}{})
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "PGRST202") {
				optMissing = append(optMissing, rpc)
			}
		}
	}

	if len(optMissing) > 0 {
		log.Printf("[Startup] WARNING: %d optional RPCs missing: %s", len(optMissing), strings.Join(optMissing, ", "))
	}

	return errors
}

func validateAgentIDs(configDir string) int {
	errors := 0

	// Agent IDs hardcoded in handler files
	handlerAgentIDs := map[string][]string{
		"handlers_task.go":     {"task_runner", "supervisor"},
		"handlers_plan.go":     {"planner", "supervisor"},
		"handlers_council.go":  {"council"},
		"handlers_research.go": {"supervisor", "council"},
	}

	// Load agent IDs from config
	type agentStub struct {
		ID string `json:"id"`
	}
	type agentsConfig struct {
		Agents []agentStub `json:"agents"`
	}

	cfg, err := loadJSONFile[agentsConfig](filepath.Join(configDir, "agents.json"))
	if err != nil {
		log.Printf("[Startup] ERROR: cannot parse agents.json: %v", err)
		return 1
	}

	configIDs := map[string]bool{}
	for _, a := range cfg.Agents {
		configIDs[a.ID] = true
	}

	for file, ids := range handlerAgentIDs {
		for _, id := range ids {
			if !configIDs[id] {
				log.Printf("[Startup] ERROR: handler %s uses agent ID '%s' but no such agent in agents.json", file, id)
				errors++
			}
		}
	}

	// Also check that every agent's default_connector references an active connector
	type connStub struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	type connectorsCfg struct {
		Connectors []connStub `json:"destinations"`
	}

	activeConnectors := map[string]bool{}
	connCfg, err := loadJSONFile[connectorsCfg](filepath.Join(configDir, "connectors.json"))
	if err == nil {
		for _, c := range connCfg.Connectors {
			if c.Status == "active" {
				activeConnectors[c.ID] = true
			}
		}

		type agentFull struct {
			ID               string `json:"id"`
			DefaultConnector string `json:"default_connector"`
		}
		type agentsFull struct {
			Agents []agentFull `json:"agents"`
		}

		agentCfg, err := loadJSONFile[agentsFull](filepath.Join(configDir, "agents.json"))
		if err == nil {
			for _, a := range agentCfg.Agents {
				if a.DefaultConnector != "" && !activeConnectors[a.DefaultConnector] {
					log.Printf("[Startup] WARNING: agent '%s' uses connector '%s' which is not active", a.ID, a.DefaultConnector)
				}
			}
		}
	}

	// Check models access_via includes active connectors
	type modelStub struct {
		ID        string   `json:"id"`
		AccessVia []string `json:"access_via"`
	}
	type modelsConfig struct {
		Models []modelStub `json:"models"`
	}

	modelCfg, err := loadJSONFile[modelsConfig](filepath.Join(configDir, "models.json"))
	if err == nil {
		for _, m := range modelCfg.Models {
			hasActive := false
			for _, via := range m.AccessVia {
				if activeConnectors[via] {
					hasActive = true
					break
				}
			}
			if !hasActive && len(m.AccessVia) > 0 {
				log.Printf("[Startup] WARNING: model '%s' access_via %v has no active connectors", m.ID, m.AccessVia)
			}
		}
	}

	return errors
}

// loadJSONFile is a generic helper to load and parse a JSON config file
func loadJSONFile[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &result, nil
}

// startupDBAdapter wraps *db.DB to satisfy the validation interface
type startupDBAdapter struct {
	db interface {
		RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
	}
}

func (a *startupDBAdapter) RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error) {
	return a.db.RPC(ctx, name, params)
}
