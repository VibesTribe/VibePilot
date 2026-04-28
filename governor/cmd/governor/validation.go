package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

type TaskData struct {
	TaskNumber       string
	Title            string
	Type             string
	Confidence       float64
	Category         string
	SliceID          string
	Dependencies     []string
	RequiresCodebase bool
	TargetFiles      []string
	PromptPacket     string
	ExpectedOutput   string
}

type ValidationError struct {
	TaskNumber string
	Issue      string
	Severity   string
}

func (e *ValidationError) Error() string {
	return "task " + e.TaskNumber + ": " + e.Issue
}

type ValidationFailedError struct {
	Errors []ValidationError
}

func (e *ValidationFailedError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return e.Errors[0].Issue + " (" + e.Errors[0].TaskNumber + ")"
}

func validateTasks(tasks []TaskData, cfg *runtime.ValidationConfig) *ValidationFailedError {
	if cfg == nil {
		cfg = &runtime.ValidationConfig{
			MinTaskConfidence:     0.0,
			RequirePromptPacket:   true,
			RequireCategory:       true,
			RequireExpectedOutput: true,
		}
	}

	var errors []ValidationError
	for _, task := range tasks {
		if task.Confidence < cfg.MinTaskConfidence {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      fmt.Sprintf("confidence %.2f below minimum %.2f", task.Confidence, cfg.MinTaskConfidence),
				Severity:   "high",
			})
		}
		if cfg.RequirePromptPacket && task.PromptPacket == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "empty prompt packet",
				Severity:   "critical",
			})
		}
		if cfg.RequireCategory && task.Category == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing category",
				Severity:   "medium",
			})
		}
		if cfg.RequireExpectedOutput && task.ExpectedOutput == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing expected output",
				Severity:   "medium",
			})
		}
	}

	if len(errors) > 0 {
		return &ValidationFailedError{Errors: errors}
	}
	return nil
}

func createTasksFromApprovedPlan(ctx context.Context, database db.Database, plan map[string]any, cfg *runtime.ValidationConfig, repoPath string, git *gitree.Gitree) error {
	planID, _ := plan["id"].(string)
	planPath, _ := plan["plan_path"].(string)
	if planPath == "" {
		return fmt.Errorf("plan has no plan_path")
	}

	// Fetch plan content
	planContent, err := fetchContent(ctx, repoPath, planPath)
	if err != nil {
		return fmt.Errorf("fetch plan: %w", err)
	}

	tasks, err := parseTasksFromPlanMarkdown(string(planContent))
	if err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no valid tasks found in plan")
	}

	log.Printf("[createTasksFromApprovedPlan] Found %d tasks in plan %s", len(tasks), truncateID(planID))

	// Deduplication guard: if tasks already exist for this plan, skip creation.
	// This prevents duplicate tasks when pgnotify fires the review handler twice.
	existingRaw, _ := database.Query(ctx, "tasks", map[string]any{
		"plan_id": planID,
		"select":  "id",
	})
	if len(existingRaw) > 4 { // Query returns at least "[]" (2 chars)
		var existing []map[string]any
		if json.Unmarshal(existingRaw, &existing) == nil && len(existing) > 0 {
			log.Printf("[createTasksFromApprovedPlan] Skipping: %d tasks already exist for plan %s", len(existing), truncateID(planID))
			return nil
		}
	}

	if validationErr := validateTasks(tasks, cfg); validationErr != nil {
		log.Printf("[createTasksFromApprovedPlan] Validation failed: %v", validationErr)
		return validationErr
	}

	sliceIDs := make(map[string]bool)
	for _, task := range tasks {
		if task.SliceID != "" {
			sliceIDs[task.SliceID] = true
		}
	}

	if git != nil {
		for sliceID := range sliceIDs {
			if err := git.CreateModuleBranch(ctx, sliceID); err != nil {
				log.Printf("[createTasksFromApprovedPlan] Warning: failed to create module branch %s: %v", sliceID, err)
			} else {
				log.Printf("[createTasksFromApprovedPlan] Created module branch: module/%s", sliceID)
			}
		}
	}

	createdCount := 0
	for _, task := range tasks {
		var routingFlag string
		var routingReason string

		if task.RequiresCodebase || len(task.Dependencies) > 0 {
			routingFlag = "internal"
			if task.RequiresCodebase {
				routingReason = "requires codebase access"
			} else {
				routingReason = fmt.Sprintf("has %d dependencies", len(task.Dependencies))
			}
		}

		status := "pending"

		maxAttempts := 3
		if cfg != nil && cfg.DefaultMaxAttempts > 0 {
			maxAttempts = cfg.DefaultMaxAttempts
		}

		// Get slice-specific sequential task number to prevent collisions
		taskNumber := task.TaskNumber
		if task.SliceID != "" {
			result, err := database.RPC(ctx, "get_next_task_number_for_slice", map[string]any{
				"p_slice_id": task.SliceID,
			})
			if err != nil {
				// Non-fatal: use the plan's task number instead of aborting
				log.Printf("[createTasksFromApprovedPlan] Warning: failed to get task number for slice %s (using plan number %s): %v", task.SliceID, task.TaskNumber, err)
			} else {
				// Parse RPC result - function returns TEXT directly, not wrapped
				if len(result) > 0 {
					// Try parsing as direct string response
					var directResult string
					if err := json.Unmarshal(result, &directResult); err == nil && directResult != "" {
						taskNumber = directResult
						log.Printf("[createTasksFromApprovedPlan] Assigned task number %s for slice %s (was %s)", taskNumber, task.SliceID, task.TaskNumber)
					} else {
						// Fallback: try parsing as array of objects
						var rpcResult []map[string]any
						if err := json.Unmarshal(result, &rpcResult); err == nil && len(rpcResult) > 0 {
							if num, ok := rpcResult[0]["get_next_task_number_for_slice"].(string); ok {
								taskNumber = num
								log.Printf("[createTasksFromApprovedPlan] Assigned task number %s for slice %s (was %s) [fallback]", taskNumber, task.SliceID, task.TaskNumber)
							}
						}
					}
				}
			}
		}

		taskData := map[string]any{
			"plan_id":             planID,
			"task_number":         taskNumber,
			"title":               task.Title,
			"type":                task.Type,
			"status":              status,
			"priority":            5,
			"confidence":          task.Confidence,
			"category":            task.Category,
			"slice_id":            task.SliceID,
			"routing_flag":        routingFlag,
			"routing_flag_reason": routingReason,
			"dependencies":        task.Dependencies,
			"result": map[string]any{
				"prompt_packet":   task.PromptPacket,
				"expected_output": task.ExpectedOutput,
				"target_files":    task.TargetFiles,
			},
			"max_attempts": maxAttempts,
		}

		result, err := database.Insert(ctx, "tasks", taskData)
		if err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "duplicate") {
				log.Printf("[createTasksFromApprovedPlan] Task %s already exists, skipping", task.TaskNumber)
				createdCount++
				continue
			}
			log.Printf("[createTasksFromApprovedPlan] Failed to create task %s: %v", task.TaskNumber, err)
			continue
		}

		var createdTasks []map[string]any
		if len(result) > 0 {
			json.Unmarshal(result, &createdTasks)
		}
		var taskIDStr string
		if len(createdTasks) > 0 {
			if id, ok := createdTasks[0]["id"].(string); ok {
				taskIDStr = id
			}
		}
		log.Printf("[createTasksFromApprovedPlan] Created task %s: %s (id: %s)", task.TaskNumber, task.Title, truncateID(taskIDStr))

		// Write prompt_packet to dedicated task_packets table so it survives
		// result JSONB overwrites during execution. GetTaskPacket already queries
		// this table first and falls back to tasks.result JSONB.
		if taskIDStr != "" && task.PromptPacket != "" {
			_, pktErr := database.Insert(ctx, "task_packets", map[string]any{
				"task_id":         taskIDStr,
				"prompt":          task.PromptPacket,
				"expected_output": task.ExpectedOutput,
				"context": map[string]any{
					"target_files": task.TargetFiles,
					"task_number":  task.TaskNumber,
					"slice_id":     task.SliceID,
				},
				"version": 1,
			})
			if pktErr != nil {
				log.Printf("[createTasksFromApprovedPlan] Warning: failed to insert task_packet for %s: %v", task.TaskNumber, pktErr)
			} else {
				log.Printf("[createTasksFromApprovedPlan] Wrote task_packet for task %s (id: %s)", task.TaskNumber, truncateID(taskIDStr))
			}
		}

		createdCount++
	}

	log.Printf("[createTasksFromApprovedPlan] Created %d/%d tasks for plan %s", createdCount, len(tasks), truncateID(planID))
	return nil
}

func parseTasksFromPlanMarkdown(content string) ([]TaskData, error) {
	var tasks []TaskData

	taskHeaderRegex := regexp.MustCompile(`(?m)^### (T\d+):`)
	matches := taskHeaderRegex.FindAllStringSubmatchIndex(content, -1)

	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		start := match[0]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		section := content[start:end]
		task, err := parseTaskSection(section)
		if err != nil {
			log.Printf("[parseTasksFromPlanMarkdown] Failed to parse task section: %v", err)
			continue
		}

		if task.TaskNumber != "" && task.Title != "" && task.PromptPacket != "" {
			tasks = append(tasks, task)
		} else {
			log.Printf("[parseTasksFromPlanMarkdown] Task %s incomplete: title=%q prompt_len=%d", task.TaskNumber, task.Title, len(task.PromptPacket))
		}
	}

	return tasks, nil
}

func parseTaskSection(section string) (TaskData, error) {
	var task TaskData
	task.Type = "feature"
	task.Category = "coding"
	task.SliceID = "general"
	task.Confidence = 0.95
	task.Dependencies = []string{}

	headerEnd := strings.Index(section, "\n")
	if headerEnd == -1 {
		return task, fmt.Errorf("no newline in section")
	}

	header := section[3:headerEnd]
	body := section[headerEnd+1:]

	parts := strings.SplitN(header, ":", 2)
	if len(parts) < 2 {
		return task, fmt.Errorf("invalid header format: %s", header)
	}

	task.TaskNumber = strings.TrimSpace(parts[0])
	task.Title = strings.TrimSpace(parts[1])

	confidenceMatch := regexp.MustCompile(`\*\*Confidence:\*\*\s*([\d.]+)`).FindStringSubmatch(body)
	if len(confidenceMatch) > 1 {
		task.Confidence, _ = strconv.ParseFloat(confidenceMatch[1], 64)
	}

	depsMatch := regexp.MustCompile(`\*\*Dependencies:\*\*\s*(.+)`).FindStringSubmatch(body)
	if len(depsMatch) > 1 {
		depsStr := strings.TrimSpace(depsMatch[1])
		if depsStr != "none" && depsStr != "-" {
			if strings.HasPrefix(depsStr, "[") {
				// Try direct JSON parse
				var parsed []string
				if err := json.Unmarshal([]byte(depsStr), &parsed); err == nil {
					task.Dependencies = parsed
				} else {
					// Planner may output escaped quotes: [\"T001\"] — unescape and retry
					unescaped := strings.ReplaceAll(depsStr, `\"`, `"`)
					if err := json.Unmarshal([]byte(unescaped), &parsed); err == nil {
						task.Dependencies = parsed
					} else {
						// Final fallback: strip all brackets, quotes, backslashes and split
						cleaned := strings.Trim(depsStr, "[]\"\\")
						task.Dependencies = strings.Fields(cleaned)
					}
				}
			} else {
				task.Dependencies = strings.Fields(depsStr)
			}
		}
	}

	catMatch := regexp.MustCompile(`\*\*Category:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(catMatch) > 1 {
		task.Category = strings.TrimSpace(catMatch[1])
	}

	sliceMatch := regexp.MustCompile(`\*\*Slice:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(sliceMatch) > 1 {
		task.SliceID = strings.TrimSpace(sliceMatch[1])
	}

	typeMatch := regexp.MustCompile(`\*\*Type:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(typeMatch) > 1 {
		task.Type = strings.TrimSpace(typeMatch[1])
	}

	codebaseMatch := regexp.MustCompile(`\*\*Requires Codebase:\*\*\s*(true|false)`).FindStringSubmatch(body)
	if len(codebaseMatch) > 1 {
		task.RequiresCodebase = strings.ToLower(codebaseMatch[1]) == "true"
	}

	// Parse target files from "Target Files" or "Deliverables" section
	targetFilesMatch := regexp.MustCompile(`\*\*Target Files:\*\*\s*(.+)`).FindStringSubmatch(body)
	if len(targetFilesMatch) > 1 {
		filesStr := strings.TrimSpace(targetFilesMatch[1])
		if filesStr != "none" && filesStr != "-" && filesStr != "N/A" {
			if strings.HasPrefix(filesStr, "[") {
				var parsed []string
				if err := json.Unmarshal([]byte(filesStr), &parsed); err == nil {
					task.TargetFiles = parsed
				}
			} else {
				// Comma-separated or space-separated
				task.TargetFiles = strings.Fields(strings.ReplaceAll(filesStr, ",", " "))
			}
		}
	}
	task.TargetFiles = sanitizeFilePaths(task.TargetFiles)

	task.PromptPacket = extractSection(body, "#### Prompt Packet")
	task.ExpectedOutput = extractSection(body, "#### Expected Output")

	return task, nil
}

func extractSection(body, heading string) string {
	start := strings.Index(body, heading)
	if start == -1 {
		return ""
	}

	content := body[start+len(heading):]
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```") {
		newlineIdx := strings.Index(content, "\n")
		if newlineIdx == -1 {
			return content
		}
		content = content[newlineIdx+1:]
		endMarker := "\n```"
		endIdx := strings.Index(content, endMarker)
		if endIdx == -1 {
			return content
		}
		return strings.TrimSpace(content[:endIdx])
	}

	nextSection := strings.Index(content, "\n#### ")
	if nextSection != -1 {
		return strings.TrimSpace(content[:nextSection])
	}

	return strings.TrimSpace(content)
}

// sanitizeFilePaths cleans up file paths extracted from planner output.
// Strips quotes, backticks, and path traversal attempts.
func sanitizeFilePaths(paths []string) []string {
	var clean []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"`'")
		p = strings.TrimPrefix(p, "./")
		// Reject path traversal
		if strings.Contains(p, "..") {
			continue
		}
		// Reject empty or obviously non-file strings
		if p == "" || len(p) < 3 || strings.Contains(p, " ") {
			continue
		}
		clean = append(clean, p)
	}
	return clean
}
