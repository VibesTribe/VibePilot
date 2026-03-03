package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

func getRecoveryConfig(cfg *runtime.Config) RecoveryConfig {
	recovery := RecoveryConfig{
		OrphanThresholdSeconds: 300,
		MaxTaskAttempts:        3,
		ModelFailureThreshold:  3,
	}

	if cfg.System != nil && cfg.System.Recovery != nil {
		if v := cfg.System.Recovery["orphan_threshold_seconds"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.OrphanThresholdSeconds = int(val)
			case int:
				recovery.OrphanThresholdSeconds = val
			}
		}
		if v := cfg.System.Recovery["max_task_attempts"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.MaxTaskAttempts = int(val)
			case int:
				recovery.MaxTaskAttempts = val
			}
		}
		if v := cfg.System.Recovery["model_failure_threshold"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.ModelFailureThreshold = int(val)
			case int:
				recovery.ModelFailureThreshold = val
			}
		}
	}

	return recovery
}

func runStartupRecovery(ctx context.Context, database *db.DB, cfg RecoveryConfig) {
	log.Println("Running startup recovery...")

	orphans, err := database.RPC(ctx, "find_orphaned_sessions", map[string]interface{}{
		"p_orphan_threshold_seconds": cfg.OrphanThresholdSeconds,
	})
	if err != nil {
		log.Printf("[Recovery] Warning: Could not check for orphans: %v", err)
		return
	}

	var orphanList []map[string]interface{}
	if err := json.Unmarshal(orphans, &orphanList); err != nil {
		log.Printf("[Recovery] Warning: Could not parse orphan list: %v", err)
		return
	}

	if len(orphanList) == 0 {
		log.Println("[Recovery] No orphaned sessions found")
		return
	}

	log.Printf("[Recovery] Found %d orphaned session(s)", len(orphanList))

	for _, orphan := range orphanList {
		sessionID, _ := orphan["id"].(string)
		taskID, _ := orphan["task_id"].(string)
		secondsSince, _ := orphan["seconds_since_heartbeat"].(float64)

		log.Printf("[Recovery] Recovering orphan session %s (task %s, %d seconds since heartbeat)",
			truncateID(sessionID), truncateID(taskID), int(secondsSince))

		_, err := database.RPC(ctx, "recover_orphaned_session", map[string]interface{}{
			"p_session_id": sessionID,
			"p_reason":     "startup_recovery",
		})
		if err != nil {
			log.Printf("[Recovery] Failed to recover session %s: %v", truncateID(sessionID), err)
		}
	}

	log.Printf("[Recovery] Recovery complete - %d session(s) recovered", len(orphanList))
}

func runProcessingRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config) {
	timeout := cfg.GetProcessingTimeoutSeconds()
	interval := time.Duration(cfg.GetProcessingRecoveryIntervalSeconds()) * time.Second

	if interval == 0 {
		interval = 60 * time.Second
	}
	if timeout == 0 {
		timeout = 300
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[ProcessingRecovery] Starting (interval: %v, timeout: %ds)", interval, timeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("[ProcessingRecovery] Stopped")
			return
		case <-ticker.C:
			recoverStaleProcessing(ctx, database, "plans", timeout)
			recoverStaleProcessing(ctx, database, "tasks", timeout)
			recoverStaleProcessing(ctx, database, "test_results", timeout)
			recoverStaleProcessing(ctx, database, "research_suggestions", timeout)
			recoverStaleProcessing(ctx, database, "maintenance_commands", timeout)
		}
	}
}

func recoverStaleProcessing(ctx context.Context, database *db.DB, table string, timeout int) {
	stale, err := database.RPC(ctx, "find_stale_processing", map[string]any{
		"p_table":           table,
		"p_timeout_seconds": timeout,
	})
	if err != nil {
		log.Printf("[ProcessingRecovery] Failed to find stale %s: %v", table, err)
		return
	}

	var staleItems []map[string]any
	if err := json.Unmarshal(stale, &staleItems); err != nil {
		return
	}

	if len(staleItems) == 0 {
		return
	}

	for _, item := range staleItems {
		id, _ := item["id"].(string)
		processingBy, _ := item["processing_by"].(string)
		secondsStale, _ := item["seconds_stale"].(float64)

		log.Printf("[ProcessingRecovery] Recovering stale %s %s (processing_by: %s, stale for %ds)",
			table[:len(table)-1], truncateID(id), processingBy, int(secondsStale))

		_, err := database.RPC(ctx, "recover_stale_processing", map[string]any{
			"p_table":  table,
			"p_id":     id,
			"p_reason": fmt.Sprintf("timeout_recovery (%ds)", int(secondsStale)),
		})
		if err != nil {
			log.Printf("[ProcessingRecovery] Failed to recover %s %s: %v", table[:len(table)-1], truncateID(id), err)
		}
	}

	log.Printf("[ProcessingRecovery] Recovered %d stale %s", len(staleItems), table)
}

func runCheckpointRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config, checkpointMgr *core.CheckpointManager) {
	coreCfg := cfg.GetCoreConfig()
	if !coreCfg.IsRecoveryEnabled() {
		log.Println("[CheckpointRecovery] Recovery disabled, skipping")
		return
	}

	log.Println("[CheckpointRecovery] Checking for tasks with checkpoints...")

	result, err := database.RPC(ctx, "find_tasks_with_checkpoints", map[string]any{
		"p_statuses": []string{"in_progress", "review", "testing"},
	})
	if err != nil {
		log.Printf("[CheckpointRecovery] Warning: Could not query checkpoints: %v", err)
		return
	}

	var tasks []map[string]any
	if err := json.Unmarshal(result, &tasks); err != nil {
		log.Printf("[CheckpointRecovery] Warning: Could not parse checkpoint list: %v", err)
		return
	}

	if len(tasks) == 0 {
		log.Println("[CheckpointRecovery] No tasks with checkpoints found")
		return
	}

	log.Printf("[CheckpointRecovery] Found %d task(s) with checkpoints", len(tasks))

	for _, task := range tasks {
		taskID, _ := task["task_id"].(string)
		taskNumber, _ := task["task_number"].(string)
		status, _ := task["status"].(string)
		step, _ := task["step"].(string)
		progress, _ := task["progress"].(float64)

		log.Printf("[CheckpointRecovery] Task %s (status: %s, step: %s, progress: %d%%)",
			taskNumber, status, step, int(progress))

		switch step {
		case "execution":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to reset task %s: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Reset task %s to available for re-execution", taskNumber)
				database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
			}

		case "review":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "review",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to set task %s to review: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Task %s will be picked up for review", taskNumber)
			}

		case "testing":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "testing",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to set task %s to testing: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Task %s will be picked up for testing", taskNumber)
			}

		default:
			log.Printf("[CheckpointRecovery] Unknown step '%s' for task %s, resetting to available", step, taskNumber)
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			if err == nil {
				database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
			}
		}
	}

	log.Printf("[CheckpointRecovery] Recovery complete - processed %d task(s)", len(tasks))
}
