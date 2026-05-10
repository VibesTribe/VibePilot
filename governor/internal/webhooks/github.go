package webhooks

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"

	"github.com/vibepilot/governor/internal/db"
)

type GitHubWebhookHandler struct {
	db     db.Database
	prdDir string
}

type GitHubPushPayload struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []GitHubCommit `json:"commits"`
}

type GitHubCommit struct {
	ID        string   `json:"id"`
	Message   string   `json:"message"`
	Timestamp string   `json:"timestamp"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
}

func NewGitHubWebhookHandler(database db.Database, prdDir string) *GitHubWebhookHandler {
	return &GitHubWebhookHandler{
		db:     database,
		prdDir: prdDir,
	}
}

func (h *GitHubWebhookHandler) HandlePush(ctx context.Context, body []byte) {
	var payload GitHubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[GitHub Webhooks] Failed to parse push payload: %v", err)
		return
	}

	log.Printf("[GitHub Webhooks] Processing push to %s (%d commits)", payload.Repository.FullName, len(payload.Commits))

	for _, commit := range payload.Commits {
		h.processCommit(ctx, commit, payload.Repository.FullName)
	}
}

func (h *GitHubWebhookHandler) processCommit(ctx context.Context, commit GitHubCommit, repoName string) {
	for _, file := range commit.Added {
		h.checkAndCreatePRD(ctx, file, repoName, "added")
		h.checkAndCreateResearch(ctx, file, repoName, "added")
	}

	for _, file := range commit.Removed {
		if h.isPRD(file) {
			log.Printf("[GitHub Webhooks] PRD removed: %s in %s", file, repoName)
		}
		if h.isResearchFile(file) {
			log.Printf("[GitHub Webhooks] Research file removed: %s in %s", file, repoName)
		}
	}

	for _, file := range commit.Modified {
		h.checkAndCreatePRD(ctx, file, repoName, "modified")
		h.checkAndCreateResearch(ctx, file, repoName, "modified")
	}
}

// --- PRD handling (existing) ---

func (h *GitHubWebhookHandler) checkAndCreatePRD(ctx context.Context, file, repoName, action string) {
	if !h.isPRD(file) {
		return
	}

	exists, err := h.prdExists(ctx, file)
	if err != nil {
		log.Printf("[GitHub Webhooks] Error checking PRD existence: %v", err)
		return
	}

	if exists {
		log.Printf("[GitHub Webhooks] PRD already tracked (%s): %s", action, file)
		return
	}

	log.Printf("[GitHub Webhooks] New PRD detected (%s): %s in %s", action, file, repoName)

	// Record webhook event for dashboard timeline
	h.db.Insert(ctx, "orchestrator_events", map[string]any{
		"event_type": "prd_committed",
		"task_id":    file,
		"model_id":   "",
		"reason":     "",
		"details": map[string]any{
			"prd_path":  file,
			"repo_name": repoName,
			"action":    action,
			"source":    "webhook",
		},
	})

	h.createPlanForPRD(ctx, file)
}

func (h *GitHubWebhookHandler) isPRD(file string) bool {
	if !strings.HasPrefix(file, "docs/prd/") || !strings.HasSuffix(file, ".md") {
		return false
	}
	// Exclude subfolders — only match docs/prd/*.md directly
	sub := strings.TrimPrefix(file, "docs/prd/")
	return !strings.Contains(sub, "/")
}

func (h *GitHubWebhookHandler) createPlanForPRD(ctx context.Context, prdPath string) {
	_, err := h.db.RPC(ctx, "create_plan", map[string]any{
		"p_project_id": nil,
		"p_prd_path":   prdPath,
		"p_plan_path":  nil,
	})
	if err != nil {
		log.Printf("[GitHub Webhooks] Failed to create plan for %s: %v", prdPath, err)
		return
	}
	log.Printf("[GitHub Webhooks] Created plan for PRD: %s", prdPath)
}

func (h *GitHubWebhookHandler) prdExists(ctx context.Context, prdPath string) (bool, error) {
	// Check plans table first
	result, err := h.db.Query(ctx, "plans", map[string]any{
		"prd_path": prdPath,
		"limit":    1,
	})
	if err != nil {
		return false, err
	}

	var plans []map[string]any
	if err := json.Unmarshal(result, &plans); err != nil {
		return false, err
	}

	if len(plans) > 0 {
		return true, nil
	}

	// Also check orchestrator_events for existing prd_committed — prevents
	// duplicate events when a force-push or rebase causes git to list old
	// PRD files as "added" again
	events, err := h.db.Query(ctx, "orchestrator_events", map[string]any{
		"event_type": "prd_committed",
		"task_id":    prdPath,
		"limit":      1,
	})
	if err != nil {
		return false, err
	}

	var existing []map[string]any
	if err := json.Unmarshal(events, &existing); err != nil {
		return false, err
	}

	return len(existing) > 0, nil
}

// --- Research file handling (new) ---

// isResearchFile returns true for files like research/some-topic.md
// Must live directly under research/ (no nested subfolders).
func (h *GitHubWebhookHandler) isResearchFile(file string) bool {
	if !strings.HasPrefix(file, "research/") || !strings.HasSuffix(file, ".md") {
		return false
	}
	sub := strings.TrimPrefix(file, "research/")
	// Only direct children, not nested paths
	return !strings.Contains(sub, "/")
}

func (h *GitHubWebhookHandler) checkAndCreateResearch(ctx context.Context, file, repoName, action string) {
	if !h.isResearchFile(file) {
		return
	}

	// Only process research files from the knowledgebase repo
	if repoName != "VibesTribe/knowledgebase" {
		log.Printf("[GitHub Webhooks] Research file from non-KB repo, skipping: %s in %s", file, repoName)
		return
	}

	// Check if this research file is already tracked
	exists, err := h.researchSuggestionExists(ctx, file)
	if err != nil {
		log.Printf("[GitHub Webhooks] Error checking research existence: %v", err)
		return
	}
	if exists {
		log.Printf("[GitHub Webhooks] Research already tracked (%s): %s", action, file)
		return
	}

	// Extract title from filename: research/my-topic-slug.md -> My Topic Slug
	base := strings.TrimSuffix(filepath.Base(file), ".md")
	title := strings.ReplaceAll(base, "-", " ")
	// Capitalize first letter of each word
	titleParts := strings.Split(title, " ")
	for i, part := range titleParts {
		if len(part) > 0 {
			titleParts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	title = strings.Join(titleParts, " ")

	log.Printf("[GitHub Webhooks] New research detected (%s): %s in %s", action, file, repoName)

	// Insert into research_suggestions
	details := map[string]any{
		"repo_name":    repoName,
		"action":       action,
		"commit_file":  file,
		"source":       "knowledgebase_push",
		"auto_created": true,
	}

	_, err = h.db.Insert(ctx, "research_suggestions", map[string]any{
		"title":         title,
		"type":          "architecture",
		"complexity":    "complex",
		"summary":       "Research report auto-detected from knowledgebase push: " + file,
		"findings_path": file,
		"status":        "pending",
		"source":        "knowledgebase_push",
		"details":       details,
	})
	if err != nil {
		log.Printf("[GitHub Webhooks] Failed to insert research suggestion for %s: %v", file, err)
		return
	}

	log.Printf("[GitHub Webhooks] Created research suggestion for: %s", file)

	// Record event for dashboard timeline
	h.db.Insert(ctx, "orchestrator_events", map[string]any{
		"event_type": "research_committed",
		"task_id":    file,
		"model_id":   "",
		"reason":     "",
		"details": map[string]any{
			"research_path": file,
			"repo_name":     repoName,
			"action":        action,
			"source":        "webhook",
		},
	})
}

func (h *GitHubWebhookHandler) researchSuggestionExists(ctx context.Context, findingsPath string) (bool, error) {
	result, err := h.db.Query(ctx, "research_suggestions", map[string]any{
		"findings_path": findingsPath,
		"limit":         1,
	})
	if err != nil {
		return false, err
	}

	var rows []map[string]any
	if err := json.Unmarshal(result, &rows); err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}
