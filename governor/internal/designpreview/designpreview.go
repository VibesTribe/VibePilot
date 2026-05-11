package designpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Config holds design preview configuration.
type Config struct {
	Enabled         bool   `json:"enabled"`
	CoDesignURL     string `json:"codesign_url"`
	GeminiVaultKey  string `json:"gemini_vault_key"`
	GeminiModel     string `json:"gemini_model"`
	OutputDir       string `json:"output_dir"`
	AutoGenerate    bool   `json:"auto_generate"`
	RequireApproval bool   `json:"require_approval"`
	MaxWaitMinutes  int    `json:"max_wait_minutes"`
}

// DB interface for persistence. Uses the governor's DB layer.
type DB interface {
	Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error)
	Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
}

// PreviewStatus represents the state of a design preview.
type PreviewStatus string

const (
	StatusPending  PreviewStatus = "pending_approval"
	StatusApproved PreviewStatus = "approved"
	StatusRejected PreviewStatus = "rejected"
)

// DesignPreview manages the pre-execution design review gate.
// When a UI task reaches the design_review stage, this generates a mockup
// via Gemini, stores it for dashboard viewing, and waits for human approval.
type DesignPreview struct {
	config    Config
	generator *Generator
	db        DB
}

// GenerateRequest is the input for generating a design preview.
type GenerateRequest struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url,omitempty"`
	DesignHints string `json:"design_hints,omitempty"`
}

// GenerateResponse is the output of a design preview generation.
type GenerateResponse struct {
	PreviewID   string `json:"preview_id"`
	HTMLContent string `json:"html_content"`
	FilePath    string `json:"file_path"`
	Model       string `json:"model"`
	TokensUsed  int    `json:"tokens_used"`
}

// NewDesignPreview creates a new DesignPreview instance.
func NewDesignPreview(config Config, apiKey string, db DB) *DesignPreview {
	return &DesignPreview{
		config:    config,
		generator: NewGenerator(config, apiKey),
		db:        db,
	}
}

// Generate creates a design mockup from a task description.
func (dp *DesignPreview) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if !dp.config.Enabled {
		return nil, fmt.Errorf("[DesignPreview] Design preview is disabled")
	}

	previewID := uuid.New().String()

	// Use the Generator to call Gemini and produce HTML
	htmlContent, tokensIn, tokensOut, err := dp.generator.GenerateHTMLMockup(ctx, req.Title, req.Description)
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Gemini generation failed: %w", err)
	}

	// Save the HTML file to the repo
	filePath, err := dp.generator.SaveHTML("", dp.config.OutputDir, req.TaskID, 1, htmlContent)
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to save mockup: %w", err)
	}

	// Persist preview record to DB
	_, err = dp.db.Insert(ctx, "design_previews", map[string]any{
		"id":           previewID,
		"task_id":      req.TaskID,
		"status":       string(StatusPending),
		"prompt":       fmt.Sprintf("%s: %s", req.Title, req.Description),
		"html_content": htmlContent,
		"file_path":    filePath,
		"version":      1,
		"created_at":   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to insert preview record: %w", err)
	}

	return &GenerateResponse{
		PreviewID:   previewID,
		HTMLContent: htmlContent,
		FilePath:    filePath,
		Model:       dp.config.GeminiModel,
		TokensUsed:  tokensIn + tokensOut,
	}, nil
}

// Approve marks a design preview as approved.
func (dp *DesignPreview) Approve(ctx context.Context, previewID, reviewer, notes string) error {
	_, err := dp.db.Update(ctx, "design_previews", previewID, map[string]any{
		"status":       string(StatusApproved),
		"reviewer":     reviewer,
		"review_notes": notes,
		"reviewed_at":  time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("[DesignPreview] Failed to approve preview %s: %w", previewID, err)
	}
	return nil
}

// Reject marks a design preview as rejected with feedback.
func (dp *DesignPreview) Reject(ctx context.Context, previewID, reviewer, notes string) error {
	_, err := dp.db.Update(ctx, "design_previews", previewID, map[string]any{
		"status":       string(StatusRejected),
		"reviewer":     reviewer,
		"review_notes": notes,
		"reviewed_at":  time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("[DesignPreview] Failed to reject preview %s: %w", previewID, err)
	}
	return nil
}

// GetEnabled returns whether design preview is enabled.
func (dp *DesignPreview) GetEnabled() bool {
	return dp != nil && dp.config.Enabled
}

// EnsureDBSchema is a no-op placeholder -- schema created by migration file.
func EnsureDBSchema() error {
	return nil
}

// MarshalJSON implements custom JSON for PreviewStatus.
func (s PreviewStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
