package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

type TaskData struct {
	TaskNumber       string
	Title            string
	Type             string
	Confidence       float64
	Category         string
	Dependencies     []string
	RequiresCodebase bool
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

func createTasksFromApprovedPlan(ctx context.Context, database *db.DB, plan map[string]any, cfg *runtime.ValidationConfig, repoPath string) error {
	planID, _ := plan["id"].(string)
	planPath, _ := plan["plan_path"].(string)
	if planPath == "" {
		return fmt.Errorf("plan has no plan_path")
	}

	fullPath := filepath.Join(repoPath, planPath)
	planContent, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read plan file %s: %w", fullPath, err)
	}

	tasks, err := parseTasksFromPlanMarkdown(string(planContent))
	if err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no valid tasks found in plan")
	}

	// Check if tasks already exist for this plan (prevents duplicates from multiple events)
	existingData, err := database.REST(ctx, "GET",
		fmt.Sprintf("/rest/v1/tasks?plan_id=eq.%s&select=id", planID), nil)
	if err == nil && len(existingData) > 2 {
		var existing []map[string]interface{}
		if parseErr := json.Unmarshal(existingData, &existing); parseErr == nil && len(existing) > 0 {
			log.Printf("[createTasksFromApprovedPlan] Plan %s already has %d tasks, skipping duplicate creation",
				truncateID(planID), len(existing))
			return nil
		}
	}

	log.Printf("[createTasksFromApprovedPlan] Found %d tasks in plan %s", len(tasks), truncateID(planID))

	if validationErr := validateTasks(tasks, cfg); validationErr != nil {
		log.Printf("[createTasksFromApprovedPlan] Validation failed: %v", validationErr)
		return validationErr
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

		status := "available"
		if len(task.Dependencies) > 0 {
			status = "pending"
		}

		maxAttempts := 3
		if cfg != nil && cfg.DefaultMaxAttempts > 0 {
			maxAttempts = cfg.DefaultMaxAttempts
		}
		taskID, err := database.RPC(ctx, "create_task_with_packet", map[string]any{
			"p_plan_id":             planID,
			"p_task_number":         task.TaskNumber,
			"p_title":               task.Title,
			"p_type":                task.Type,
			"p_status":              status,
			"p_priority":            5,
			"p_confidence":          task.Confidence,
			"p_category":            task.Category,
			"p_routing_flag":        routingFlag,
			"p_routing_flag_reason": routingReason,
			"p_dependencies":        task.Dependencies,
			"p_prompt":              task.PromptPacket,
			"p_expected_output":     task.ExpectedOutput,
			"p_context":             map[string]any{"source": "plan_approval"},
			"p_max_attempts":        maxAttempts,
		})
		if err != nil {
			log.Printf("[createTasksFromApprovedPlan] Failed to create task %s: %v", task.TaskNumber, err)
			continue
		}

		var taskIDStr string
		if len(taskID) > 0 {
			json.Unmarshal(taskID, &taskIDStr)
		}
		log.Printf("[createTasksFromApprovedPlan] Created task %s: %s (id: %s)", task.TaskNumber, task.Title, truncateID(taskIDStr))
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
			task.Dependencies = strings.Fields(depsStr)
		}
	}

	catMatch := regexp.MustCompile(`\*\*Category:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(catMatch) > 1 {
		task.Category = strings.TrimSpace(catMatch[1])
	}

	typeMatch := regexp.MustCompile(`\*\*Type:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(typeMatch) > 1 {
		task.Type = strings.TrimSpace(typeMatch[1])
	}

	codebaseMatch := regexp.MustCompile(`\*\*Requires Codebase:\*\*\s*(true|false)`).FindStringSubmatch(body)
	if len(codebaseMatch) > 1 {
		task.RequiresCodebase = strings.ToLower(codebaseMatch[1]) == "true"
	}

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
