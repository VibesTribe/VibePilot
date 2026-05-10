package designpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Config holds the design preview configuration.
type Config struct {
	Enabled                 bool     `json:"enabled"`
	TriggerCategories       []string `json:"trigger_categories"`
	TriggerTags             []string `json:"trigger_tags"`
	ConnectorID             string   `json:"connector_id"`
	Model                   string   `json:"model"`
	DesignOutputDir         string   `json:"design_output_dir"`
	ManifestFile            string   `json:"manifest_file"`
	MaxIterations           int      `json:"max_iterations"`
	GitCommitDesigns        bool     `json:"git_commit_designs"`
	IncludeBaselineScreenshot bool     `json:"include_baseline_screenshot"`
	GeminiVaultKey          string   `json:"gemini_vault_key"`
	RepoPath                string   `json:"repo_path"`
}

// DesignReview represents a design review record from the database.
type DesignReview struct {
	ID                  string  `json:"id"`
	TaskID              string  `json:"task_id"`
	Version             int     `json:"version"`
	Status              string  `json:"status"`
	MockupHTMLPath      string  `json:"mockup_html_path"`
	MockupScreenshotPath string  `json:"mockup_screenshot_path"`
	DesignPrompt        string  `json:"design_prompt"`
	ModelID             string  `json:"model_id"`
	TokensIn            int     `json:"tokens_in"`
	TokensOut           int     `json:"tokens_out"`
	HumanFeedback       string  `json:"human_feedback"`
	ReviewedAt          *time.Time `json:"reviewed_at"`
	ReviewedBy          string  `json:"reviewed_by"`
	CreatedAt           time.Time `json:"created_at"`
}

// DB is the interface the design preview package needs for persistence.
type DB interface {
	Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
	Query(ctx context.Context, query string, args ...interface{}) (json.RawMessage, error)
}

// DesignPreviewService is the main design preview engine.
type DesignPreviewService struct {
	config    Config
	apiKey    string
	db        DB
	generator *Generator
}

// NewDesignPreviewService creates a new DesignPreviewService instance.
func NewDesignPreviewService(config Config, apiKey string, db DB) *DesignPreviewService {
	return &DesignPreviewService{
		config:    config,
		apiKey:    apiKey,
		db:        db,
		generator: NewGenerator(config, apiKey),
	}
}

// GetEnabled returns whether the service is enabled.
func (s *DesignPreviewService) GetEnabled() bool {
	return s != nil && s.config.Enabled
}

// GenerateDesign generates an HTML mockup for a task and persists it.
func (s *DesignPreviewService) GenerateDesign(ctx context.Context, taskID, taskTitle, taskDescription string) (*DesignReview, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("[DesignPreview] Service is disabled")
	}

	// Check if a design review already exists for this task
	existing, _ := s.db.Query(ctx, "SELECT id, version FROM design_reviews WHERE task_id = $1 ORDER BY version DESC LIMIT 1", taskID)
	if existing != nil {
		var reviews []struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		}
		if json.Unmarshal(existing, &reviews) == nil && len(reviews) > 0 {
			if reviews[0].Version >= s.config.MaxIterations {
				return nil, fmt.Errorf("[DesignPreview] Max iterations (%d) reached for task %s", s.config.MaxIterations, taskID)
			}
		}
	}

	// Generate HTML mockup
	htmlContent, tokensIn, tokensOut, err := s.generator.GenerateHTMLMockup(ctx, taskTitle, taskDescription)
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to generate mockup: %w", err)
	}

	// Persist the HTML file to disk
	version := 1
	mockupHTMLPath, err := s.generator.SaveHTML(s.config.RepoPath, s.config.DesignOutputDir, taskID, version, htmlContent)
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to save HTML: %w", err)
	}

	// Insert design review record into DB
	designPrompt := fmt.Sprintf("Generate UI mockup for: %s - %s", taskTitle, taskDescription)
	var dr DesignReview
	_, err = s.db.Exec(ctx, `
		INSERT INTO design_reviews (task_id, version, status, mockup_html_path, design_prompt, model_id, tokens_in, tokens_out)
		VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7)
		RETURNING id, task_id, version, status, mockup_html_path, design_prompt, model_id, tokens_in, tokens_out, created_at
	`, taskID, version, mockupHTMLPath, designPrompt, s.config.Model, tokensIn, tokensOut)
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to insert design_review: %w", err)
	}

	dr = DesignReview{
		TaskID:         taskID,
		Version:        version,
		Status:         "pending",
		MockupHTMLPath: mockupHTMLPath,
		DesignPrompt:   designPrompt,
		ModelID:        s.config.Model,
		TokensIn:       tokensIn,
		TokensOut:      tokensOut,
	}

	fmt.Printf("[DesignPreview] Created design review for task %s (version %d)\n", taskID, version)
	return &dr, nil
}

// ShouldTriggerDesign returns true if a task should go through design preview
// based on its category or tags matching the trigger configuration.
func (s *DesignPreviewService) ShouldTriggerDesign(category string, tags []string) bool {
	if !s.config.Enabled {
		return false
	}

	for _, tc := range s.config.TriggerCategories {
		if tc == category {
			return true
		}
	}

	for _, tag := range tags {
		for _, tt := range s.config.TriggerTags {
			if tag == tt {
				return true
			}
		}
	}

	return false
}
