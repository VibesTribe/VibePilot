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

func validateTasks(tasks []TaskData, validationCfg *runtime.ValidationConfig) *ValidationFailedError {
	var errors []ValidationError

	if validationCfg == nil {
		validationCfg = &runtime.ValidationConfig{
			MinTaskConfidence:     0.95,
			RequirePromptPacket:   true,
			RequireCategory:       true,
			RequireExpectedOutput: true,
		}
	}

	for _, task := range tasks {
		if task.Confidence < validationCfg.MinTaskConfidence {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      fmt.Sprintf("confidence %.2f below minimum %.2f - task must be split further", task.Confidence, validationCfg.MinTaskConfidence),
				Severity:   "high",
			})
		}
		if validationCfg.RequirePromptPacket && task.PromptPacket == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "empty prompt packet - task has no instructions",
				Severity:   "critical",
			})
		}
		if validationCfg.RequireCategory && task.Category == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing category - needed for routing to appropriate model",
				Severity:   "medium",
			})
		}
		if validationCfg.RequireExpectedOutput && task.ExpectedOutput == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing expected output - supervisor cannot verify completion",
				Severity:   "medium",
			})
		}
	}

	if len(errors) > 0 {
		return &ValidationFailedError{Errors: errors}
	}
	return nil
}

func createTasksFromApprovedPlan(ctx context.Context, database *db.DB, plan map[string]any, validationCfg *runtime.ValidationConfig, repoPath string) error {
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

	log.Printf("[createTasksFromApprovedPlan] Found %d tasks in plan %s", len(tasks), truncateID(planID))

	if validationErr := validateTasks(tasks, validationCfg); validationErr != nil {
		log.Printf("[createTasksFromApprovedPlan] Validation failed for plan %s: %v", truncateID(planID), validationErr)
		return validationErr
	}

	createdCount := 0
	for _, task := range tasks {
		routingFlag := "web"
		if task.RequiresCodebase {
			routingFlag = "internal"
		}

		// Determine status based on dependencies
		status := "available"
		if len(task.Dependencies) > 0 {
			// Task has dependencies - starts as pending until deps complete
			status = "pending"
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
			"p_routing_flag_reason": fmt.Sprintf("From plan: %s", planPath),
			"p_dependencies":        task.Dependencies,
			"p_prompt":              task.PromptPacket,
			"p_expected_output":     task.ExpectedOutput,
			"p_context":             map[string]any{"source": "plan_approval"},
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

func findMatchingCodeBlockEnd(content string) int {
	depth := 1
	i := 0
	for i < len(content) {
		if strings.HasPrefix(content[i:], "```") {
			if i > 0 && content[i-1] == '\n' {
				depth++
			}
			i += 3
			continue
		}
		if strings.HasPrefix(content[i:], "\n```") {
			depth--
			if depth == 0 {
				return i
			}
			i += 4
			continue
		}
		i++
	}
	return -1
}

func parseTaskSection(section string) (TaskData, error) {
	var task TaskData
	task.Type = "feature"
	task.Category = "coding"

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

	categoryMatch := regexp.MustCompile(`\*\*Category:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(categoryMatch) > 1 {
		task.Category = strings.TrimSpace(categoryMatch[1])
	}

	typeMatch := regexp.MustCompile(`\*\*Type:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(typeMatch) > 1 {
		task.Type = strings.TrimSpace(typeMatch[1])
	}

	codebaseMatch := regexp.MustCompile(`\*\*Requires Codebase:\*\*\s*(true|false)`).FindStringSubmatch(body)
	if len(codebaseMatch) > 1 {
		task.RequiresCodebase = strings.ToLower(codebaseMatch[1]) == "true"
	}

	ppStart := strings.Index(body, "#### Prompt Packet")
	if ppStart != -1 {
		ppContent := body[ppStart+19:]
		ppContent = strings.TrimSpace(ppContent)

		codeStart := strings.Index(ppContent, "```")
		if codeStart != -1 && codeStart < 10 {
			ppContent = ppContent[codeStart+3:]
			if strings.HasPrefix(ppContent, "json") || strings.HasPrefix(ppContent, "markdown") {
				ppContent = ppContent[strings.Index(ppContent, "\n")+1:]
			} else if strings.HasPrefix(ppContent, "\n") {
				ppContent = ppContent[1:]
			}
			ppEnd := findMatchingCodeBlockEnd(ppContent)
			if ppEnd != -1 {
				task.PromptPacket = strings.TrimSpace(ppContent[:ppEnd])
			}
		} else {
			nextSection := strings.Index(ppContent, "\n#### ")
			if nextSection != -1 {
				task.PromptPacket = strings.TrimSpace(ppContent[:nextSection])
			} else {
				task.PromptPacket = strings.TrimSpace(ppContent)
			}
		}
	}

	eoStart := strings.Index(body, "#### Expected Output")
	if eoStart != -1 {
		eoContent := body[eoStart+19:]
		eoContent = strings.TrimSpace(eoContent)

		codeStart := strings.Index(eoContent, "```")
		if codeStart != -1 && codeStart < 10 {
			eoContent = eoContent[codeStart+3:]
			if strings.HasPrefix(eoContent, "json") || strings.HasPrefix(eoContent, "markdown") {
				eoContent = eoContent[strings.Index(eoContent, "\n")+1:]
			} else if strings.HasPrefix(eoContent, "\n") {
				eoContent = eoContent[1:]
			}
			eoEnd := findMatchingCodeBlockEnd(eoContent)
			if eoEnd != -1 {
				task.ExpectedOutput = strings.TrimSpace(eoContent[:eoEnd])
			}
		} else {
			nextSection := strings.Index(eoContent, "\n#### ")
			if nextSection != -1 {
				task.ExpectedOutput = strings.TrimSpace(eoContent[:nextSection])
			} else {
				task.ExpectedOutput = strings.TrimSpace(eoContent)
			}
		}
	}

	return task, nil
}
