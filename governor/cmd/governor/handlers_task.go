package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/connectors"
	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/security"
)

type TaskHandler struct {
	database       db.Database
	factory        *runtime.SessionFactory
	pool           *runtime.AgentPool
	connRouter     *runtime.Router
	git            *gitree.Gitree
	checkpointMgr  *core.CheckpointManager
	leakDetector   *security.LeakDetector
	cfg            *runtime.Config
	usageTracker   *runtime.UsageTracker
	worktreeMgr    *gitree.WorktreeManager
	courierRunner  *connectors.CourierRunner
	vault          vaultProvider
	contextBuilder *runtime.ContextBuilder
}

// vaultProvider abstracts secret access for the task handler.
type vaultProvider interface {
	GetSecret(ctx context.Context, keyName string) (string, error)
}

func NewTaskHandler(
	database db.Database,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	checkpointMgr *core.CheckpointManager,
	leakDetector *security.LeakDetector,
	cfg *runtime.Config,
	usageTracker *runtime.UsageTracker,
	worktreeMgr *gitree.WorktreeManager,
	courierRunner *connectors.CourierRunner,
	v vaultProvider,
) *TaskHandler {
	return &TaskHandler{
		database:      database,
		factory:       factory,
		pool:          pool,
		connRouter:    connRouter,
		git:           git,
		checkpointMgr: checkpointMgr,
		leakDetector:  leakDetector,
		cfg:           cfg,
		usageTracker:  usageTracker,
		worktreeMgr:   worktreeMgr,
		courierRunner: courierRunner,
		vault:         v,
	}
}

// SetContextBuilder injects the code map context builder for targeted file injection.
func (h *TaskHandler) SetContextBuilder(cb *runtime.ContextBuilder) {
	h.contextBuilder = cb
}

func (h *TaskHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventTaskAvailable, h.handleTaskAvailable)
	router.On(runtime.EventTaskReview, h.handleTaskReview)
}

// ============================================================================
// TASK EXECUTION: available → in_progress → review
// ============================================================================

func (h *TaskHandler) handleTaskAvailable(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskAvailable] Parse error: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	taskCategory := getString(task, "category")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// DEPENDENCY GATE: Block tasks whose dependencies aren't complete
	if deps, ok := task["dependencies"].([]any); ok && len(deps) > 0 {
		allComplete := true
		for _, dep := range deps {
			depNum, _ := dep.(string)
			if depNum == "" {
				continue
			}
			depRows, err := h.database.Query(ctx, "tasks", map[string]any{
				"task_number": depNum,
				"select":      "id,status",
			})
			if err != nil || len(depRows) == 0 {
				log.Printf("[TaskAvailable] Task %s: dependency %s not found, blocking", taskNumber, depNum)
				allComplete = false
				break
			}
			var depTasks []map[string]any
			if json.Unmarshal(depRows, &depTasks) == nil && len(depTasks) > 0 {
				depStatus, _ := depTasks[0]["status"].(string)
				if depStatus != "merged" && depStatus != "complete" && depStatus != "completed" {
					log.Printf("[TaskAvailable] Task %s: dependency %s not complete (status=%s), reverting to pending", taskNumber, depNum, depStatus)
					allComplete = false
					break
				}
			}
		}
		if !allComplete {
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "pending",
			})
			return
		}
		log.Printf("[TaskAvailable] Task %s: all dependencies complete, proceeding", taskNumber)
	}

	// Get task packet
	taskPacket, err := h.database.GetTaskPacket(ctx, taskID)
	if err != nil {
		if result, ok := task["result"].(map[string]any); ok {
			if prompt, ok := result["prompt_packet"].(string); ok && prompt != "" {
				taskPacket = &db.TaskPacket{TaskID: taskID, Prompt: prompt}
			}
		}
		if taskPacket == nil || taskPacket.Prompt == "" {
			log.Printf("[TaskAvailable] No packet for %s", truncateID(taskID))
			return
		}
	}

	// On retry: append supervisor revision notes to prompt so executor fixes issues
	// failure_notes being non-empty means a previous supervisor review rejected this task
	if failureNotes, ok := task["failure_notes"].(string); ok && failureNotes != "" {
		taskPacket.Prompt += "\n\n## PREVIOUS ATTEMPT FEEDBACK (fix these issues)\n" + failureNotes
		log.Printf("[TaskAvailable] Task %s retry: appended revision notes (%d bytes)", truncateID(taskID), len(failureNotes))
	}

	// Inject targeted file contents for task_runner (context_policy: targeted)
	// Read target_files from task result JSONB and load actual file contents
	if result, ok := task["result"].(map[string]any); ok {
		if rawFiles, ok := result["target_files"]; ok {
			var targetFiles []string
			switch v := rawFiles.(type) {
			case []string:
				targetFiles = v
			case []interface{}:
				for _, f := range v {
					if s, ok := f.(string); ok {
						targetFiles = append(targetFiles, s)
					}
				}
			}
			if len(targetFiles) > 0 && h.contextBuilder != nil {
				fileContext := h.contextBuilder.BuildTargetedContext(targetFiles)
				if fileContext != "" {
					taskPacket.Prompt += fileContext
					log.Printf("[TaskAvailable] Task %s: injected %d target files into prompt", taskNumber, len(targetFiles))
				}
			}
		}
	}

	// Route to model with cascade retry — same pattern as planner/supervisor
	var routingResult *runtime.RoutingResult
	var failedModels []string

	// If this task was previously failed by a specific model, exclude that model
	// to prevent re-assigning to the same one that failed.
	if flagReason := getString(task, "routing_flag_reason"); flagReason != "" {
		if after, ok := strings.CutPrefix(flagReason, "test_failed_by:"); ok && after != "" {
			failedModels = append(failedModels, after)
			log.Printf("[TaskAvailable] Task %s: excluding test-failed model %s", taskNumber, after)
		}
		if after, ok := strings.CutPrefix(flagReason, "exec_failed_by:"); ok && after != "" {
			failedModels = append(failedModels, after)
			log.Printf("[TaskAvailable] Task %s: excluding exec-failed model %s", taskNumber, after)
		}
	}
	var modelID, connectorID, routingFlag string
	var connConfig *runtime.ConnectorConfig
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		var routeErr error
		if attempt > 0 {
			log.Printf("[TaskAvailable] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "task_runner",
			TaskType:      taskCategory,
			RoutingFlag:   "", // empty = router decides (web courier if available, internal fallback)
			ExcludeModels: failedModels,
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[TaskAvailable] No routing for task %s (attempt %d)", truncateID(taskID), attempt+1)
			// All models in cooldown or unavailable — stop, don't retry
			return
		}

		modelID = routingResult.ModelID
		connectorID = routingResult.ConnectorID
		connConfig = h.cfg.GetConnector(connectorID)
		routingFlag = h.deriveRoutingFlag(connConfig)

		// Check pool capacity
		if !h.pool.HasCapacity(sliceID, connectorID) {
			log.Printf("[TaskAvailable] Task %s pending - no capacity (slice=%s, dest=%s)", truncateID(taskID), sliceID, connectorID)
			failedModels = append(failedModels, modelID)
			continue
		}

		// Claim task
		workerID := fmt.Sprintf("executor:%s:%d", modelID, time.Now().UnixNano())
		claimed, err := h.database.RPC(ctx, "claim_task", map[string]any{
			"p_task_id":        taskID,
			"p_worker_id":      workerID,
			"p_model_id":       modelID,
			"p_routing_flag":   routingFlag,
			"p_routing_reason": fmt.Sprintf("Routed via %s (attempt %d)", connectorID, attempt+1),
		})
		if err != nil || !parseBool(claimed) {
			log.Printf("[TaskAvailable] Task %s claim failed (model=%s): err=%v", truncateID(taskID), modelID, err)
			failedModels = append(failedModels, modelID)
			continue
		}

		// Successfully claimed
		log.Printf("[TaskAvailable] Task %s claimed by %s via %s", truncateID(taskID), modelID, connectorID)
		break
	}

	if routingResult == nil {
		log.Printf("[TaskAvailable] No routing available for task %s after %d attempts", truncateID(taskID), maxRetries)
		return
	}

	// Setup branch
	branchName := h.buildBranchName(sliceID, taskNumber, taskID)
	attempts := 0
	if v, ok := task["attempts"].(float64); ok {
		attempts = int(v)
	}

	var worktreePath string

	if h.worktreeMgr != nil {
		// Worktree mode: isolated checkout per task
		existingPath := h.worktreeMgr.GetWorktreePath(taskID)
		if attempts > 0 && existingPath != "" {
			// Check if worktree directory still exists (preserved after test failure)
			if _, err := os.Stat(existingPath); err == nil {
				// Reuse existing worktree and branch — iterative fix, not fresh start
				worktreePath = existingPath
				log.Printf("[TaskAvailable] Task %s retry: reusing existing worktree at %s", truncateID(taskID), worktreePath)
			} else {
				// Worktree gone — create fresh
				wtInfo, err := h.worktreeMgr.CreateWorktree(ctx, taskID, branchName)
				if err != nil {
					log.Printf("[TaskAvailable] Worktree create failed for %s: %v, falling back to branch-only", truncateID(taskID), err)
					h.git.CreateBranch(ctx, branchName)
				} else {
					worktreePath = wtInfo.Path
					log.Printf("[TaskAvailable] Worktree created for %s at %s", truncateID(taskID), worktreePath)
				}
			}
		} else {
			// First attempt: create fresh worktree
			wtInfo, err := h.worktreeMgr.CreateWorktree(ctx, taskID, branchName)
			if err != nil {
				log.Printf("[TaskAvailable] Worktree create failed for %s: %v, falling back to branch-only", truncateID(taskID), err)
				h.git.CreateBranch(ctx, branchName)
			} else {
				worktreePath = wtInfo.Path
				log.Printf("[TaskAvailable] Worktree created for %s at %s", truncateID(taskID), worktreePath)
			}
		}
	} else {
		// Legacy mode: single directory, branch checkout
		if attempts > 0 {
			h.git.DeleteBranch(ctx, branchName)
		}
		h.git.CreateBranch(ctx, branchName)
	}

	h.database.RPC(ctx, "update_task_branch", map[string]any{
		"p_task_id":     taskID,
		"p_branch_name": branchName,
	})

	h.saveCheckpoint(ctx, taskID, "execution_start", 0, "", nil)
	runStart := time.Now()

	// Web courier dispatch: if router selected a web platform, use CourierRunner
	if routingFlag == "web" && h.courierRunner != nil {
		platformURL := routingResult.PlatformURL
		platformID := routingResult.PlatformID
		log.Printf("[TaskAvailable] Courier dispatch for %s → %s (%s)", truncateID(taskID), platformID, platformURL)

		err = h.pool.SubmitWithDestination(ctx, sliceID, connectorID, func() error {
			return h.executeCourierTask(ctx, task, taskPacket, taskID, taskNumber, modelID, connectorID, branchName, taskCategory, platformID, platformURL, runStart)
		})
		if err != nil {
			log.Printf("[TaskAvailable] Courier pool error for %s: %v", truncateID(taskID), err)
			h.failTask(ctx, taskID, modelID, branchName, "courier_submit_failed")
		}
		return
	}

	// Internal execution: standard agent dispatch via pool
	err = h.pool.SubmitWithDestination(ctx, sliceID, connectorID, func() error {
		return h.executeTask(ctx, task, taskPacket, taskID, taskNumber, modelID, connectorID, branchName, taskCategory, worktreePath, runStart)
	})
	if err != nil {
		log.Printf("[TaskAvailable] Pool error for %s: %v", truncateID(taskID), err)
		h.failTask(ctx, taskID, modelID, branchName, "pool_submit_failed")
	}
}

func (h *TaskHandler) executeTask(
	ctx context.Context,
	task map[string]any,
	taskPacket *db.TaskPacket,
	taskID, taskNumber, modelID, connectorID, branchName, taskCategory, worktreePath string,
	runStart time.Time,
) error {

	var contextData map[string]any
	if len(taskPacket.Context) > 0 {
		json.Unmarshal(taskPacket.Context, &contextData)
	}

	session, err := h.factory.CreateWithConnector(ctx, "task_runner", taskCategory, connectorID)
	if err != nil {
		log.Printf("[TaskHandler] Session create failed for task %s: %v", truncateID(taskID), err)
		h.failTask(ctx, taskID, modelID, branchName, "session_create_failed")
		return err
	}

	// Build session params -- include worktree path if available
	sessionParams := map[string]any{
		"task_id":         taskID,
		"task_number":     taskNumber,
		"title":           getString(task, "title"),
		"type":            getString(task, "type"),
		"category":        taskCategory,
		"prompt_packet":   taskPacket.Prompt,
		"expected_output": taskPacket.ExpectedOutput,
		"context":         contextData,
		"dependencies":    task["dependencies"],
		"event":           "task_available",
	}
	if worktreePath != "" {
		sessionParams["worktree_path"] = worktreePath
		sessionParams["repo_path"] = worktreePath
	}

	// Execute with timeout — prevent hung workers from locking tasks forever
	execCtx, execCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer execCancel()

	result, err := session.Run(execCtx, sessionParams)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			log.Printf("[TaskHandler] TIMEOUT for task %s after 5m", truncateID(taskID))
			h.failTask(ctx, taskID, modelID, branchName, "execution_timeout")
			return fmt.Errorf("execution timeout")
		}
		// Check for rate limit (HTTP 429)
		if h.usageTracker != nil && modelID != "" {
			if isRateLimitError(err) {
				log.Printf("[TaskAvailable] Rate limit hit for model %s via %s, recording cooldown", modelID, connectorID)
				h.usageTracker.RecordRateLimit(ctx, modelID)
				// Also cooldown ALL models on the same connector (shared limits)
				if connectorID != "" {
					// Parse retry-after from error if available, otherwise use recovery config
					cooldownMins := h.getRecoveryCooldown(modelID)
					h.usageTracker.RecordConnectorCooldown(ctx, connectorID, cooldownMins)
				}
			}
			h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, time.Since(runStart).Seconds(), false)
		}
		h.failTask(ctx, taskID, modelID, branchName, "execution_error")
		return err
	}

	// Compact session for context history
	h.factory.Compact(ctx, result, taskID)

	// Security scan
	cleanOutput, leaks := h.leakDetector.Scan(result.Output)
	if len(leaks) > 0 {
		log.Printf("[TaskAvailable] %d leaks redacted in %s", len(leaks), truncateID(taskID))
	}

	duration := time.Since(runStart)
	tokensIn := result.TokensIn
	tokensOut := result.TokensOut
	totalTokens := tokensIn + tokensOut

	// Record usage with tracker
	if h.usageTracker != nil {
		if err := h.usageTracker.RecordUsage(ctx, modelID, tokensIn, tokensOut); err != nil {
			log.Printf("[TaskAvailable] UsageTracker RecordUsage error for %s: %v", modelID, err)
		}
	}

	// Parse output
	taskOutput, parseErr := runtime.ParseTaskRunnerOutput(result.Output)
	var files []runtime.File
	var summary string
	if parseErr != nil {
		summary = cleanOutput
	} else {
		files = taskOutput.Files
		summary = taskOutput.Summary
	}

	// Build execution result for supervisor review
	executionResult := map[string]any{
		"files": func() []map[string]any {
			result := make([]map[string]any, len(files))
			for i, f := range files {
				result[i] = map[string]any{
					"path":    f.Path,
					"content": f.Content,
				}
			}
			return result
		}(),
		"summary":    summary,
		"raw_output": cleanOutput,
		"status":     "complete",
	}

	// Commit output to branch
	h.commitOutput(ctx, branchName, files, cleanOutput, summary, modelID, taskID, duration.Seconds())

	// Record task run with execution result
	costs := h.calculateCosts(ctx, modelID, tokensIn, tokensOut)
	if _, err := h.database.RPC(ctx, "create_task_run", map[string]any{
		"p_task_id":                       taskID,
		"p_model_id":                      modelID,
		"p_courier":                       connectorID,
		"p_platform":                      h.deriveRoutingFlag(h.cfg.GetConnector(connectorID)),
		"p_status":                        "success",
		"p_tokens_in":                     tokensIn,
		"p_tokens_out":                    tokensOut,
		"p_tokens_used":                   totalTokens,
		"p_courier_model_id":              nil,
		"p_courier_tokens":                0,
		"p_courier_cost_usd":              0,
		"p_platform_theoretical_cost_usd": costs.Theoretical,
		"p_total_actual_cost_usd":         costs.Actual,
		"p_total_savings_usd":             costs.Savings,
		"p_started_at":                    runStart,
		"p_completed_at":                  time.Now(),
		"p_result":                         executionResult,
	}); err != nil {
		log.Printf("[TaskHandler] ERROR create_task_run failed for task %s model %s: %v", taskID, modelID, err)
	}

	// Deduct cost from model's credit_remaining_usd (if model has credit tracking)
	if costs.Actual > 0 {
		if _, err := h.database.RPC(ctx, "deduct_model_credit", map[string]any{
			"p_model_id": modelID,
			"p_cost_usd": costs.Actual,
		}); err != nil {
			log.Printf("[TaskHandler] ERROR deduct_model_credit failed for model %s: %v", modelID, err)
		}
	}

	// Atomically transition to review
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "review",
	})

	h.recordSuccess(ctx, taskID, modelID, taskCategory, duration.Seconds(), totalTokens)

	// Record successful completion with tracker
	if h.usageTracker != nil {
		h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, duration.Seconds(), true)
	}

	h.deleteCheckpoint(ctx, taskID)

	log.Printf("[TaskAvailable] Task %s → review", truncateID(taskID))
	return nil
}

// executeCourierTask dispatches a task to a web platform via the CourierRunner.
// The courier navigates to the platform URL, pastes the prompt, and returns the response.
func (h *TaskHandler) executeCourierTask(
	ctx context.Context,
	task map[string]any,
	taskPacket *db.TaskPacket,
	taskID, taskNumber, modelID, connectorID, branchName, taskCategory, platformID, platformURL string,
	runStart time.Time,
) error {
	// Resolve the LLM API key for browser-use from the vault
	// The courier uses the same connector's key to drive browser automation
	llmAPIKey := ""
	llmKeyRef := h.deriveLLMKeyRef(connectorID)
	if h.vault != nil && llmKeyRef != "" {
		if key, err := h.vault.GetSecret(ctx, llmKeyRef); err == nil {
			llmAPIKey = key
		} else {
			log.Printf("[CourierTask] Warning: could not resolve LLM key %s: %v", llmKeyRef, err)
		}
	}

	// Build the courier task packet with ALL fields the GitHub Action needs
	packet := map[string]any{
		"task_id":          taskID,
		"task_prompt":      taskPacket.Prompt,
		"branch_name":      branchName,
		"llm_provider":     h.deriveLLMProvider(connectorID),
		"llm_model":        modelID,
		"llm_api_key":      llmAPIKey,
		"web_platform_url": platformURL,
		"supabase_url":     h.cfg.GetDatabaseURL(),
		"supabase_key":     h.cfg.GetDatabaseKey(),
	}

	packetJSON, err := json.Marshal(packet)
	if err != nil {
		log.Printf("[CourierTask] Failed to marshal packet for %s: %v", truncateID(taskID), err)
		h.failTask(ctx, taskID, modelID, branchName, "courier_packet_failed")
		return err
	}

	// Execute via CourierRunner with timeout
	courierCtx, courierCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer courierCancel()

	output, tokensIn, tokensOut, err := h.courierRunner.Run(courierCtx, string(packetJSON), 600)
	if err != nil {
		log.Printf("[CourierTask] Courier failed for %s: %v", truncateID(taskID), err)
		// Record failure learning data for the fueling model
		if h.usageTracker != nil {
			if isRateLimitError(err) {
				h.usageTracker.RecordRateLimit(ctx, modelID)
				if connectorID != "" {
					h.usageTracker.RecordConnectorCooldown(ctx, connectorID, 30)
				}
			}
			h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, time.Since(runStart).Seconds(), false)
		}
		h.failTask(ctx, taskID, modelID, branchName, "courier_execution_failed")
		return err
	}

	duration := time.Since(runStart)
	log.Printf("[CourierTask] Courier completed for %s in %.1fs (tokens: %d/%d)", truncateID(taskID), duration.Seconds(), tokensIn, tokensOut)

	// Commit output and transition to review, same as internal execution
	summary := output
	if len(output) > 500 {
		summary = output[:500] + "..."
	}
	h.commitOutput(ctx, branchName, nil, output, summary, modelID, taskID, duration.Seconds())

	// Record task run via RPC (params match create_task_run signature exactly)
	totalTokens := tokensIn + tokensOut
	h.database.RPC(ctx, "create_task_run", map[string]any{
		"p_task_id":      taskID,
		"p_model_id":     modelID,
		"p_status":       "success",
		"p_tokens_in":    tokensIn,
		"p_tokens_out":   tokensOut,
		"p_tokens_used":  totalTokens,
		"p_courier":      "github-actions",
		"p_platform":     platformID,
		"p_started_at":   runStart.UTC().Format(time.RFC3339),
		"p_completed_at": time.Now().UTC().Format(time.RFC3339),
	})

	// Transition task to review (params match transition_task signature)
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "review",
		"p_result": map[string]any{
			"output":           output,
			"model_id":         modelID,
			"routing_flag":     "web",
			"platform_id":      platformID,
			"tokens_used":      totalTokens,
			"duration_seconds": duration.Seconds(),
		},
	})

	h.recordSuccess(ctx, taskID, modelID, taskCategory, duration.Seconds(), totalTokens)

	if h.usageTracker != nil {
		// Record token usage to in-memory windows (minute/hour/day/week)
		if err := h.usageTracker.RecordUsage(ctx, modelID, tokensIn, tokensOut); err != nil {
			log.Printf("[CourierTask] UsageTracker RecordUsage error for %s: %v", modelID, err)
		}
		// Record platform message for free-tier limit tracking
		h.usageTracker.RecordPlatformMessage(ctx, platformID, totalTokens)
		// Record completion for learned data (avg duration, best_for_task_types)
		h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, duration.Seconds(), true)
	}

	h.deleteCheckpoint(ctx, taskID)
	log.Printf("[CourierTask] Task %s → review", truncateID(taskID))
	return nil
}

// deriveLLMProvider maps a connector ID to an LLM provider name for the courier packet.
func (h *TaskHandler) deriveLLMProvider(connectorID string) string {
	switch {
	case strings.Contains(connectorID, "groq"):
		return "groq"
	case strings.Contains(connectorID, "nvidia"):
		return "nvidia"
	case strings.Contains(connectorID, "gemini"):
		return "google"
	case strings.Contains(connectorID, "openrouter"):
		return "openrouter"
	case strings.Contains(connectorID, "deepseek"):
		return "deepseek"
	default:
		return connectorID
	}
}

// deriveLLMKeyRef maps a connector ID to the vault key name for its API key.
func (h *TaskHandler) deriveLLMKeyRef(connectorID string) string {
	switch {
	case strings.Contains(connectorID, "groq"):
		return "GROQ_API_KEY"
	case strings.Contains(connectorID, "nvidia-api"):
		return "NVIDIA_API_KEY"
	case strings.Contains(connectorID, "gemini-api-courier"):
		return "GEMINI_COURIER_KEY"
	case strings.Contains(connectorID, "gemini-api-researcher"):
		return "GEMINI_RESEARCHER_KEY"
	case strings.Contains(connectorID, "gemini-api-visual"):
		return "GEMINI_VISUAL_TESTER_KEY"
	case strings.Contains(connectorID, "gemini-api-general"):
		return "GEMINI_GENERAL_KEY"
	case strings.Contains(connectorID, "gemini"):
		return "GEMINI_GENERAL_KEY"
	case strings.Contains(connectorID, "openrouter"):
		return "OPENROUTER_API_KEY"
	default:
		return ""
	}
}

// ============================================================================
// SUPERVISOR REVIEW: review → testing (approved) OR available (fail)
// ============================================================================

func (h *TaskHandler) handleTaskReview(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		return
	}

	taskID := getString(task, "id")
	taskType := getString(task, "type")
	taskNumber := getString(task, "task_number")
	modelID := getString(task, "assigned_to")
	sliceID := getStringOr(task, "slice_id", "review")

	if taskID == "" {
		return
	}

	branchName := h.buildBranchName(sliceID, taskNumber, taskID)

	// Claim for review
	reviewerID := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": reviewerID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskReview] Task %s already being reviewed", truncateID(taskID))
		return
	}

	// Route to supervisor with cascade retry — same pattern as planner/executor
	var routingResult *runtime.RoutingResult
	var failedModels []string
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		var routeErr error
		if attempt > 0 {
			log.Printf("[TaskReview] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "supervisor",
			TaskType:      taskType,
			RoutingFlag:   "internal",
			ExcludeModels: failedModels,
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[TaskReview] No supervisor for task %s (attempt %d)", truncateID(taskID), attempt+1)
			// All models in cooldown — stop, don't retry
			return
		}
		break // routing found
	}

	if routingResult == nil {
		log.Printf("[TaskReview] No routing available for task %s after %d attempts", truncateID(taskID), maxRetries)
		return
	}

	session, err := h.factory.CreateWithConnector(ctx, "supervisor", taskType, routingResult.ConnectorID)
	if err != nil {
		log.Printf("[TaskReview] Session error for %s: %v", truncateID(taskID), err)
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.ConnectorID, func() error {
		// Get context for review
		taskPacket, _ := h.database.GetTaskPacket(ctx, taskID)
		taskRunData, _ := h.database.Query(ctx, "task_runs", map[string]any{"task_id": taskID, "order": "created_at.desc", "limit": 1})
		var taskRuns []map[string]any
		var latestRun map[string]any
		if err := json.Unmarshal(taskRunData, &taskRuns); err == nil && len(taskRuns) > 0 {
			latestRun = taskRuns[0]
		}

		// Supervisor with timeout — prevent hung reviews from locking tasks
		reviewCtx, reviewCancel := context.WithTimeout(ctx, 2*time.Minute)
		defer reviewCancel()

		reviewStart := time.Now()
		result, err := session.Run(reviewCtx, map[string]any{
			"task":        task,
			"event":       "task_review",
			"task_packet": taskPacket,
			"task_run":    latestRun,
		})
		reviewDuration := time.Since(reviewStart).Seconds()
		supervisorModelID := routingResult.ModelID
		if err != nil {
			// Record supervisor model failure
			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, false)
			}
			h.database.RPC(ctx, "record_model_failure", map[string]any{
				"p_model_id":         supervisorModelID,
				"p_task_type":        "supervisor_review",
				"p_failure_class":    "session_error",
				"p_failure_detail":   err.Error(),
				"p_duration_seconds": reviewDuration,
			})
			if reviewCtx.Err() == context.DeadlineExceeded {
				log.Printf("[TaskReview] TIMEOUT reviewing task %s after 2m (supervisor=%s)", truncateID(taskID), supervisorModelID)
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "review",
					"p_failure_reason": "supervisor_review_timeout",
				})
				// Release processing_by lock so task can be re-claimed
				h.database.Update(ctx, "tasks", taskID, map[string]any{
					"processing_by": nil,
				})
				return nil
			}
			log.Printf("[TaskReview] Session failed for %s: %v — releasing lock for re-claim", truncateID(taskID), err)
			// Release processing_by lock so the task can be re-claimed by recovery or realtime
			h.database.Update(ctx, "tasks", taskID, map[string]any{
				"processing_by": nil,
			})
			return nil
		}

		// Compact session for context history
		h.factory.Compact(ctx, result, taskID)

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[TaskReview] Parse error for %s: %v, retrying...", truncateID(taskID), parseErr)

			// Retry with explicit JSON enforcement
			retrySession, retryErr := h.factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)
			if retryErr == nil {
				retryResult, retryRunErr := retrySession.Run(ctx, map[string]any{
					"previous_output": result.Output,
					"parse_error":     parseErr.Error(),
					"instruction":     "Your previous response was not valid JSON. Parse the previous output and respond with ONLY the JSON object. No markdown. No explanations.",
				})
				if retryRunErr == nil {
					decision, parseErr = runtime.ParseSupervisorDecision(retryResult.Output)
				}
			}

			if parseErr != nil {
				log.Printf("[TaskReview] Retry also failed to parse for %s: %v", truncateID(taskID), parseErr)
				// Record supervisor model failure for bad output format
				if h.usageTracker != nil {
					h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, false)
				}
				h.database.RPC(ctx, "record_model_failure", map[string]any{
					"p_model_id":         supervisorModelID,
					"p_task_type":        "supervisor_review",
					"p_failure_class":    "json_parse_error",
					"p_failure_detail":   fmt.Sprintf("Failed to produce valid JSON after retry: %v", parseErr),
					"p_duration_seconds": reviewDuration,
				})
				// Set to failed status instead of leaving in limbo
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "failed",
					"p_failure_reason": fmt.Sprintf("JSON parse failed after retry: %v", parseErr),
				})
				return nil
			}
			log.Printf("[TaskReview] Retry succeeded for %s", truncateID(taskID))
		}

		log.Printf("[TaskReview] Task %s decision: %s", truncateID(taskID), decision.Decision)

	switch decision.Decision {
	case "approved", "pass":
		// Approved → testing
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": "testing",
		})

		// Record supervisor approval as success for the supervisor model.
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, true)
		}
		h.database.RPC(ctx, "record_model_success", map[string]any{
			"p_model_id":         supervisorModelID,
			"p_task_type":        "supervisor_review",
			"p_duration_seconds": reviewDuration,
			"p_tokens_used":      result.TokensIn + result.TokensOut,
		})

		// Also record a success signal for the executor model (passed review).
		h.database.RPC(ctx, "update_model_learning", map[string]any{
			"p_model_id":         modelID,
			"p_task_type":        taskType,
			"p_outcome":          "review_passed",
			"p_failure_class":    "",
			"p_failure_category": "",
			"p_failure_detail":   "",
		})
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, modelID, "review", time.Since(reviewStart).Seconds(), true)
		}
		h.recordEvent(ctx, "approved", taskID, modelID, "review_passed", map[string]any{
			"checks":          decision.Checks,
			"supervisor_model": supervisorModelID,
		})
		log.Printf("[TaskReview] Task %s → testing (supervisor=%s approved)", truncateID(taskID), supervisorModelID)

		case "fail", "failed":
			// Failed → back to available with full failure context
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "unknown"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" && len(decision.Issues) > 0 {
				failureDetail = decision.Issues[0].Description
			}

			// Build full failure notes including ReturnFeedback
			failureNotes := fmt.Sprintf("[%s] %s", failureClass, failureDetail)
			if decision.ReturnFeedback.Summary != "" {
				failureNotes += "\n\n" + decision.ReturnFeedback.Summary
			}
			if len(decision.ReturnFeedback.SpecificIssues) > 0 {
				failureNotes += "\n\nIssues:"
				for _, issue := range decision.ReturnFeedback.SpecificIssues {
					failureNotes += "\n- " + issue
				}
			}

		h.recordFailure(ctx, modelID, taskID, failureClass)
		h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
		// Record supervisor model success (it correctly identified the failure)
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, true)
		}
		h.recordEvent(ctx, "failure", taskID, modelID, failureClass, map[string]any{
			"class": failureClass, "detail": failureDetail,
			"supervisor_model": supervisorModelID,
		})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": failureNotes,
			})
			// Exclude the executor model from retry so a different model picks it up
			h.database.Update(ctx, "tasks", taskID, map[string]any{
				"routing_flag_reason": fmt.Sprintf("exec_failed_by:%s", modelID),
			})
			log.Printf("[TaskReview] Task %s failed: %s (%s) → available (excluding model %s)", truncateID(taskID), failureClass, failureDetail, modelID)

		case "needs_revision":
			// Needs revision → back to available with FULL feedback for retry
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "needs_revision"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" && len(decision.Issues) > 0 {
				failureDetail = decision.Issues[0].Description
			}

			// Build structured revision feedback from supervisor's ReturnFeedback
			revisionNotes := fmt.Sprintf("[%s] %s", failureClass, failureDetail)
			if decision.ReturnFeedback.Summary != "" {
				revisionNotes += "\n\n" + decision.ReturnFeedback.Summary
			}
			if len(decision.ReturnFeedback.SpecificIssues) > 0 {
				revisionNotes += "\n\nIssues to fix:"
				for _, issue := range decision.ReturnFeedback.SpecificIssues {
					revisionNotes += "\n- " + issue
				}
			}
			if len(decision.ReturnFeedback.Suggestions) > 0 {
				revisionNotes += "\n\nSuggestions:"
				for _, s := range decision.ReturnFeedback.Suggestions {
					revisionNotes += "\n- " + s
				}
			}

		h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
		// Record supervisor model success (it correctly identified revision needed)
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, true)
		}
		h.recordEvent(ctx, "revision_needed", taskID, modelID, failureClass, map[string]any{
			"class": failureClass, "detail": failureDetail, "revision_notes": revisionNotes,
			"supervisor_model": supervisorModelID,
		})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": revisionNotes,
			})
			log.Printf("[TaskReview] Task %s needs revision: %s (%s) → available", truncateID(taskID), failureClass, failureDetail)

	case "council_review":
		// Complex → escalate to council
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": "council_review",
		})
		// Record supervisor success (correctly identified complexity)
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, true)
		}
		log.Printf("[TaskReview] Task %s → council_review (supervisor=%s escalated)", truncateID(taskID), supervisorModelID)

		case "reroute":
			// Reroute → back to available for different assignment
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "model_limitation"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" {
				failureDetail = "Supervisor recommends different model"
			}
		h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
		// Record supervisor model success (correctly identified model limitation)
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, supervisorModelID, "supervisor_review", reviewDuration, true)
		}
		h.recordEvent(ctx, "reroute", taskID, modelID, failureClass, map[string]any{
			"class": failureClass, "detail": failureDetail,
			"supervisor_model": supervisorModelID,
		})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": fmt.Sprintf("[%s] %s", failureClass, failureDetail),
			})
			log.Printf("[TaskReview] Task %s reroute → available", truncateID(taskID))

		default:
			// Unknown decision → human review
			log.Printf("[TaskReview] Unknown decision '%s' for %s → awaiting_human", decision.Decision, truncateID(taskID))
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "awaiting_human",
			})
		}

		return nil
	})
	if err != nil {
		log.Printf("[TaskReview] Submit error: %v", err)
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func (h *TaskHandler) failTask(ctx context.Context, taskID, modelID, branchName, reason string) {
	h.recordFailure(ctx, modelID, taskID, reason)
	// Clean up worktree if active
	if h.worktreeMgr != nil {
		h.worktreeMgr.RemoveWorktree(ctx, taskID)
	}
	h.git.DeleteBranch(ctx, branchName)
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":        taskID,
		"p_new_status":     "available",
		"p_failure_reason": reason,
	})
	log.Printf("[TaskHandler] Task %s failed: %s → available", truncateID(taskID), reason)
}

func (h *TaskHandler) buildBranchName(sliceID, taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}

	// Use slice-based naming: task/{slice_id}/{task_number}
	// Example: task/general/T001, task/auth/T002
	if sliceID != "" && taskNumber != "" {
		return prefix + sliceID + "/" + taskNumber
	}

	// Fallback to task number only (for backwards compatibility)
	if taskNumber != "" {
		return prefix + taskNumber
	}

	return prefix + truncateID(taskID)
}

func (h *TaskHandler) getTargetBranch(sliceID string) string {
	if sliceID == "" || sliceID == "default" || sliceID == "review" || sliceID == "testing" {
		sliceID = "general"
	}
	return "TEST_MODULES/" + sliceID
}

func (h *TaskHandler) deriveRoutingFlag(conn *runtime.ConnectorConfig) string {
	if conn == nil {
		return "internal"
	}
	switch conn.Type {
	case "mcp":
		return "mcp"
	case "web":
		return "web"
	default:
		return "internal"
	}
}

func (h *TaskHandler) commitOutput(ctx context.Context, branchName string, files []runtime.File, rawOutput, summary, modelID, taskID string, duration float64) error {
	outputMap := map[string]any{
		"raw_output": rawOutput,
		"model_id":   modelID,
		"task_id":    taskID,
		"duration":   duration,
		"summary":    summary,
	}
	if len(files) > 0 {
		fileMaps := make([]any, len(files))
		for i, f := range files {
			fileMaps[i] = map[string]any{"path": f.Path, "content": f.Content}
		}
		outputMap["files"] = fileMaps
	}
	return h.git.CommitOutput(ctx, branchName, outputMap)
}

func (h *TaskHandler) recordSuccess(ctx context.Context, taskID, modelID, taskType string, durationSeconds float64, tokensUsed int) {
	if modelID == "" {
		return
	}
	h.database.RPC(ctx, "record_model_success", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_duration_seconds": durationSeconds,
		"p_tokens_used":      tokensUsed,
	})
	// Feed success into model learning for competency tracking
	h.database.RPC(ctx, "update_model_learning", map[string]any{
		"p_model_id":       modelID,
		"p_task_type":      taskType,
		"p_outcome":        "success",
		"p_failure_class":  "",
		"p_failure_category": "",
		"p_failure_detail": "",
	})
}

func (h *TaskHandler) recordFailure(ctx context.Context, modelID, taskID, failureType string) {
	if modelID == "" {
		return
	}
	h.database.RPC(ctx, "record_model_failure", map[string]any{
		"p_model_id":         modelID,
		"p_task_id":          taskID,
		"p_failure_type":     failureType,
		"p_failure_category": runtime.CategorizeFailure(failureType),
	})
}

func (h *TaskHandler) recordIssues(ctx context.Context, taskID, modelID, taskType string, issues []runtime.Issue) {
	for _, issue := range issues {
		h.database.RPC(ctx, "record_failure", map[string]any{
			"p_task_id":          taskID,
			"p_failure_type":     issue.Type,
			"p_failure_category": runtime.CategorizeFailure(issue.Type),
			"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
			"p_model_id":         modelID,
			"p_task_type":        taskType,
		})
	}
}

// recordModelLearning writes structured failure data to models.learned JSONB column
// This builds institutional knowledge: which models excel at what, struggle with what.
// Over time the router uses best_for_task_types / avoid_for_task_types for intelligent routing.
func (h *TaskHandler) recordModelLearning(ctx context.Context, modelID, taskType, failureClass, failureDetail string) {
	if modelID == "" {
		return
	}
	category := runtime.CategorizeFailure(failureClass)

	// Update learned.failure_rate_by_type and learned.avoid_for_task_types
	// The RPC will merge into the existing JSONB
	h.database.RPC(ctx, "update_model_learning", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_outcome":          "failure",
		"p_failure_class":    failureClass,
		"p_failure_category": category,
		"p_failure_detail":   failureDetail,
	})
}

// recordEvent writes to orchestrator_events for the dashboard timeline.
// The dashboard reads: event_type (maps to type), reason (maps to reasonCode),
// details JSONB, task_id, model_id, created_at.
// event_type "failure" marks task quality as "fail" in deriveQualityMap.
func (h *TaskHandler) recordEvent(ctx context.Context, eventType, taskID, modelID, reason string, details map[string]any) {
	eventDetails := details
	if eventDetails == nil {
		eventDetails = map[string]any{}
	}
	eventDetails["model_id"] = modelID

	_, err := h.database.Insert(ctx, "orchestrator_events", map[string]any{
		"event_type": eventType,
		"task_id":    taskID,
		"model_id":   modelID,
		"reason":     reason,
		"details":    eventDetails,
	})
	if err != nil {
		log.Printf("[recordEvent] Failed to write event: %v", err)
	}
}

func (h *TaskHandler) saveCheckpoint(ctx context.Context, taskID, step string, progress int, output string, files []string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	h.database.RPC(ctx, "save_checkpoint", map[string]any{
		"p_task_id": taskID, "p_step": step, "p_progress": progress, "p_output": output, "p_files": files,
	})
}

func (h *TaskHandler) deleteCheckpoint(ctx context.Context, taskID string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	h.database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
}

type costResult struct{ Theoretical, Actual, Savings float64 }

func (h *TaskHandler) calculateCosts(ctx context.Context, modelID string, tokensIn, tokensOut int) costResult {
	result, err := h.database.RPC(ctx, "calc_run_costs", map[string]any{
		"p_model_id": modelID, "p_tokens_in": tokensIn, "p_tokens_out": tokensOut, "p_courier_cost_usd": 0,
	})
	if err != nil {
		return costResult{}
	}
	var costs struct {
		TheoreticalCostUsd float64 `json:"theoretical_cost_usd"`
		ActualCostUsd      float64 `json:"actual_cost_usd"`
		SavingsUsd         float64 `json:"savings_usd"`
	}
	if result != nil {
		json.Unmarshal(result, &costs)
	}
	return costResult{Theoretical: costs.TheoreticalCostUsd, Actual: costs.ActualCostUsd, Savings: costs.SavingsUsd}
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "quota exceeded")
}

func setupTaskHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	checkpointMgr *core.CheckpointManager,
	leakDetector *security.LeakDetector,
	usageTracker *runtime.UsageTracker,
	worktreeMgr *gitree.WorktreeManager,
	courierRunner *connectors.CourierRunner,
	v vaultProvider,
	contextBuilder *runtime.ContextBuilder,
) {
	handler := NewTaskHandler(database, factory, pool, connRouter, git, checkpointMgr, leakDetector, cfg, usageTracker, worktreeMgr, courierRunner, v)
	handler.SetContextBuilder(contextBuilder)
	handler.Register(router)
}

// unlockDependentsByTaskNumber finds tasks in "pending" status whose dependencies
// contain the given task number (e.g. "T001") and transitions them to "available"
// once ALL their dependencies are complete.
func unlockDependentsByTaskNumber(ctx context.Context, database db.Database, completedTaskNumber string) {
	if completedTaskNumber == "" || database == nil {
		return
	}

	// Find all pending tasks
	raw, err := database.Query(ctx, "tasks", map[string]any{
		"status": "eq.pending",
		"select": "id,task_number,dependencies",
	})
	if err != nil || len(raw) == 0 {
		return
	}

	var pendingTasks []map[string]any
	if json.Unmarshal(raw, &pendingTasks) != nil {
		return
	}

	for _, pt := range pendingTasks {
		pendingID, _ := pt["id"].(string)
		pendingNum, _ := pt["task_number"].(string)
		deps, _ := pt["dependencies"].([]any)

		if len(deps) == 0 {
			continue
		}

		// Check if this pending task depends on the completed task
		dependsOnCompleted := false
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok && depStr == completedTaskNumber {
				dependsOnCompleted = true
				break
			}
		}
		if !dependsOnCompleted {
			continue
		}

		// Check if ALL dependencies are now complete
		allComplete := true
		for _, dep := range deps {
			depNum, _ := dep.(string)
			if depNum == "" {
				continue
			}
			depRaw, err := database.Query(ctx, "tasks", map[string]any{
				"task_number": depNum,
				"select":      "status",
			})
			if err != nil || len(depRaw) == 0 {
				allComplete = false
				break
			}
			var depTasks []map[string]any
			if json.Unmarshal(depRaw, &depTasks) != nil || len(depTasks) == 0 {
				allComplete = false
				break
			}
			depStatus, _ := depTasks[0]["status"].(string)
			if depStatus != "merged" && depStatus != "complete" && depStatus != "completed" {
				allComplete = false
				break
			}
		}

		if allComplete {
			log.Printf("[DependencyUnlock] Task %s: all dependencies complete, transitioning to available", pendingNum)
			database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    pendingID,
				"p_new_status": "available",
			})
		} else {
			log.Printf("[DependencyUnlock] Task %s: still has unmet dependencies", pendingNum)
		}
	}
}

// getRecoveryCooldown returns the cooldown minutes for a model from config.
func (h *TaskHandler) getRecoveryCooldown(modelID string) int {
	if h.usageTracker != nil {
		cooldown := h.usageTracker.GetModelCooldownMinutes(modelID)
		if cooldown > 0 {
			return cooldown
		}
	}
	return 5
}
