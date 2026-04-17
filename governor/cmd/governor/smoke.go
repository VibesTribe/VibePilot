package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// SmokeTest runs an end-to-end pipeline test and reports timing for each stage.
// Usage: governor -smoke-test
// Or: go run ./cmd/governor/ -smoke-test

func runSmokeTest(dbURL, dbKey string) {
	log.Println("[Smoke] Starting end-to-end pipeline test")
	log.Println("[Smoke] Supabase:", dbURL)

	// 1. Clean all data
	log.Println("[Smoke] Step 1: Cleaning all data...")
	cleanTables := []string{"task_runs", "orchestrator_events", "tasks", "plans"}
	for _, table := range cleanTables {
		if _, err := smokeREST(dbURL, dbKey, "DELETE", table+"?id=not.is.null", nil); err != nil {
			log.Printf("[Smoke] WARNING: failed to clean %s: %v", table, err)
		}
	}
	log.Println("[Smoke] Step 1: Clean ✓")

	// 2. Insert a simple test plan
	log.Println("[Smoke] Step 2: Inserting test plan...")
	start := time.Now()
	planResult, err := smokeREST(dbURL, dbKey, "POST", "plans", map[string]any{
		"prd_path": "docs/prd/hello-world-endpoint.md",
		"status":   "draft",
	})
	if err != nil {
		log.Fatalf("[Smoke] FAIL: Could not insert plan: %v", err)
	}
	var plans []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(planResult, &plans); err != nil || len(plans) == 0 {
		log.Fatalf("[Smoke] FAIL: Could not parse plan response: %s", string(planResult))
	}
	planID := plans[0].ID
	log.Printf("[Smoke] Step 2: Plan inserted ✓ (id=%s, took %s)", planID[:8], time.Since(start))

	// 3. Poll for pipeline stages
	log.Println("[Smoke] Step 3: Watching pipeline stages...")

	type stageCheck struct {
		name      string
		check     func() (bool, string, error)
		timeout   time.Duration
	}

	planChecked := false
	tasksCreated := false
	taskCount := 0

	stages := []stageCheck{
		{
			name: "planner picks up plan",
			timeout: 120 * time.Second,
			check: func() (bool, string, error) {
				result, err := smokeREST(dbURL, dbKey, "GET", "plans?select=status,plan_path&id=eq."+planID, nil)
				if err != nil {
					return false, "", err
				}
				var ps []struct {
					Status   string `json:"status"`
					PlanPath string `json:"plan_path"`
				}
				json.Unmarshal(result, &ps)
				if len(ps) == 0 {
					return false, "no plan found", nil
				}
				if ps[0].Status != "draft" && !planChecked {
					planChecked = true
					return true, fmt.Sprintf("status=%s plan_path=%s", ps[0].Status, ps[0].PlanPath), nil
				}
				if planChecked {
					return true, fmt.Sprintf("status=%s", ps[0].Status), nil
				}
				return false, fmt.Sprintf("status=%s", ps[0].Status), nil
			},
		},
		{
			name: "supervisor approves plan",
			timeout: 120 * time.Second,
			check: func() (bool, string, error) {
				result, err := smokeREST(dbURL, dbKey, "GET", "plans?select=status&id=eq."+planID, nil)
				if err != nil {
					return false, "", err
				}
				var ps []struct {
					Status string `json:"status"`
				}
				json.Unmarshal(result, &ps)
				if len(ps) == 0 {
					return false, "no plan", nil
				}
				if ps[0].Status == "approved" || ps[0].Status == "executing" {
					return true, ps[0].Status, nil
				}
				return false, ps[0].Status, nil
			},
		},
		{
			name: "tasks created",
			timeout: 60 * time.Second,
			check: func() (bool, string, error) {
				result, err := smokeREST(dbURL, dbKey, "GET", "tasks?select=id,status,title,task_number&plan_id=eq."+planID, nil)
				if err != nil {
					return false, "", err
				}
				var tasks []struct {
					ID         string `json:"id"`
					Status     string `json:"status"`
					Title      string `json:"title"`
					TaskNumber int    `json:"task_number"`
				}
				json.Unmarshal(result, &tasks)
				if len(tasks) > 0 && !tasksCreated {
					tasksCreated = true
					taskCount = len(tasks)
					return true, fmt.Sprintf("%d tasks: %v", len(tasks), taskSummaries(tasks)), nil
				}
				if tasksCreated {
					return true, fmt.Sprintf("%d tasks", taskCount), nil
				}
				return false, "no tasks yet", nil
			},
		},
		{
			name: "task runner picks up task",
			timeout: 120 * time.Second,
			check: func() (bool, string, error) {
				result, err := smokeREST(dbURL, dbKey, "GET", "tasks?select=status,assigned_to&plan_id=eq."+planID, nil)
				if err != nil {
					return false, "", err
				}
				var tasks []struct {
					Status     string `json:"status"`
					AssignedTo string `json:"assigned_to"`
				}
				json.Unmarshal(result, &tasks)
				for _, t := range tasks {
					if t.Status == "in_progress" || t.Status == "review" || t.Status == "completed" {
						return true, fmt.Sprintf("%s (assigned: %s)", t.Status, t.AssignedTo), nil
					}
				}
				statuses := make([]string, len(tasks))
				for i, t := range tasks {
					statuses[i] = t.Status
				}
				return false, fmt.Sprintf("task statuses: %v", statuses), nil
			},
		},
	}

	stageStart := time.Now()
	for _, stage := range stages {
		stageStart = time.Now()
		log.Printf("[Smoke]   Waiting for: %s (timeout: %s)", stage.name, stage.timeout)

		done := false
		for elapsed := 0 * time.Second; elapsed < stage.timeout; elapsed += 3 * time.Second {
			ok, info, err := stage.check()
			if err != nil {
				log.Printf("[Smoke]   WARNING: check error: %v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			if ok {
				log.Printf("[Smoke]   ✓ %s: %s (took %s)", stage.name, info, time.Since(stageStart))
				done = true
				break
			}
			time.Sleep(3 * time.Second)
		}

		if !done {
			log.Printf("[Smoke]   ✗ %s: TIMED OUT after %s", stage.name, stage.timeout)
			log.Println("[Smoke] FAILED")
			os.Exit(1)
		}
	}

	// Summary
	log.Println("[Smoke] ==============================")
	log.Printf("[Smoke] ALL STAGES PASSED in %s", time.Since(start))
	log.Printf("[Smoke] Plan: %s", planID[:8])
	log.Printf("[Smoke] Tasks created: %d", taskCount)
	log.Println("[Smoke] ==============================")

	// Clean up
	log.Println("[Smoke] Cleaning up test data...")
	for _, table := range cleanTables {
		smokeREST(dbURL, dbKey, "DELETE", table+"?id=not.is.null", nil)
	}
	log.Println("[Smoke] Done.")
}

func taskSummaries(tasks []struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	TaskNumber int    `json:"task_number"`
}) string {
	sums := make([]string, len(tasks))
	for i, t := range tasks {
		sums[i] = fmt.Sprintf("T%03d: %s (%s)", t.TaskNumber, t.Title, t.Status)
	}
	return fmt.Sprintf("%v", sums)
}

func smokeREST(dbURL, dbKey, method, path string, body map[string]any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	url := dbURL + "/rest/v1/" + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", dbKey)
	req.Header.Set("Authorization", "Bearer "+dbKey)
	req.Header.Set("Content-Type", "application/json")
	if method == "POST" {
		req.Header.Set("Prefer", "return=representation")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s → %d: %s", method, path, resp.StatusCode, string(data))
	}

	return data, nil
}
