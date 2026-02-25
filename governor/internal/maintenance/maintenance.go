package maintenance

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/pkg/types"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ChangeType string

const (
	ChangeTypeConfig     ChangeType = "config"
	ChangeTypeDependency ChangeType = "dependency"
	ChangeTypeCode       ChangeType = "code"
	ChangeTypePrompt     ChangeType = "prompt"
	ChangeTypeSchema     ChangeType = "schema"
)

type Change struct {
	ID         string
	Type       ChangeType
	Target     string
	Action     string
	Content    []byte
	Reason     string
	BackupPath string
}

type ExecutionResult struct {
	Success      bool
	Output       string
	TestsPassed  bool
	RollbackPath string
	Error        string
}

type Maintenance struct {
	db         *db.DB
	gitree     *gitree.Gitree
	repoPath   string
	sandboxDir string
}

type Config struct {
	RepoPath   string
	SandboxDir string
}

func New(cfg *Config, database *db.DB, git *gitree.Gitree) *Maintenance {
	repoPath := cfg.RepoPath
	if repoPath == "" {
		repoPath, _ = os.Getwd()
	}

	sandboxDir := cfg.SandboxDir
	if sandboxDir == "" {
		sandboxDir = filepath.Join(os.TempDir(), "vibepilot-sandbox")
	}

	return &Maintenance{
		db:         database,
		gitree:     git,
		repoPath:   repoPath,
		sandboxDir: sandboxDir,
	}
}

func (m *Maintenance) ClassifyRisk(change *Change) RiskLevel {
	switch change.Type {
	case ChangeTypeConfig:
		if change.Action == "create" {
			return RiskLow
		}
		return RiskMedium
	case ChangeTypePrompt:
		return RiskLow
	case ChangeTypeDependency:
		return RiskMedium
	case ChangeTypeCode:
		return RiskHigh
	case ChangeTypeSchema:
		return RiskCritical
	default:
		return RiskHigh
	}
}

func (m *Maintenance) RequiresSandbox(change *Change) bool {
	risk := m.ClassifyRisk(change)
	return risk == RiskHigh || risk == RiskCritical
}

func (m *Maintenance) Execute(ctx context.Context, task *types.Task, packet *types.PromptPacket, output interface{}) (*ExecutionResult, error) {
	change, err := m.parseOutputToChange(output, task)
	if err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	if m.RequiresSandbox(change) {
		return m.executeWithSandbox(ctx, task, packet, change)
	}

	return m.executeDirect(ctx, task, packet, change)
}

func (m *Maintenance) executeDirect(ctx context.Context, task *types.Task, packet *types.PromptPacket, change *Change) (*ExecutionResult, error) {
	result := &ExecutionResult{}

	backupPath, err := m.Backup(change.Target)
	if err != nil {
		log.Printf("Maintenance: backup warning for %s: %v", change.Target, err)
	}
	change.BackupPath = backupPath

	if err := m.applyChange(change); err != nil {
		result.Error = err.Error()
		if backupPath != "" {
			if rerr := m.Rollback(backupPath, change.Target); rerr != nil {
				log.Printf("Maintenance: rollback failed: %v", rerr)
			}
		}
		return result, fmt.Errorf("apply change: %w", err)
	}

	result.Success = true
	result.TestsPassed = true

	m.auditLog(change, result)

	return result, nil
}

func (m *Maintenance) executeWithSandbox(ctx context.Context, task *types.Task, packet *types.PromptPacket, change *Change) (*ExecutionResult, error) {
	result := &ExecutionResult{}

	sandboxPath, err := m.CreateSandbox()
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	defer m.CleanupSandbox(sandboxPath)

	backupPath, err := m.Backup(change.Target)
	if err != nil {
		log.Printf("Maintenance: backup warning for %s: %v", change.Target, err)
	}
	change.BackupPath = backupPath

	if err := m.ApplyToSandbox(sandboxPath, change); err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("apply to sandbox: %w", err)
	}

	testResult, err := m.TestInSandbox(sandboxPath)
	if err != nil || !testResult.Passed {
		result.Error = fmt.Sprintf("sandbox tests failed: %v", testResult.Failures)
		return result, fmt.Errorf("sandbox tests failed")
	}

	if err := m.applyChange(change); err != nil {
		result.Error = err.Error()
		if backupPath != "" {
			if rerr := m.Rollback(backupPath, change.Target); rerr != nil {
				log.Printf("Maintenance: rollback failed: %v", rerr)
			}
		}
		return result, fmt.Errorf("apply change: %w", err)
	}

	result.Success = true
	result.TestsPassed = true
	result.RollbackPath = backupPath

	m.auditLog(change, result)

	return result, nil
}

func (m *Maintenance) applyChange(change *Change) error {
	if change.Target == "" {
		return fmt.Errorf("change target is empty")
	}

	targetPath := filepath.Join(m.repoPath, change.Target)

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	var writeErr error
	if change.Action == "delete" {
		writeErr = os.Remove(targetPath)
		if os.IsNotExist(writeErr) {
			writeErr = nil
		}
	} else {
		writeErr = os.WriteFile(targetPath, change.Content, 0644)
	}

	if writeErr != nil {
		return fmt.Errorf("write file: %w", writeErr)
	}

	return nil
}

func (m *Maintenance) parseOutputToChange(output interface{}, task *types.Task) (*Change, error) {
	change := &Change{
		ID:     task.ID,
		Reason: task.Title,
	}

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("output is not a map")
	}

	if changeType, ok := outputMap["change_type"].(string); ok {
		change.Type = ChangeType(changeType)
	} else {
		change.Type = ChangeTypeCode
	}

	if target, ok := outputMap["target"].(string); ok {
		change.Target = target
	}

	if action, ok := outputMap["action"].(string); ok {
		change.Action = action
	} else {
		change.Action = "update"
	}

	if content, ok := outputMap["content"].(string); ok {
		change.Content = []byte(content)
	} else if contentBytes, ok := outputMap["content"].([]byte); ok {
		change.Content = contentBytes
	}

	return change, nil
}

func (m *Maintenance) auditLog(change *Change, result *ExecutionResult) {
	log.Printf("Maintenance: change %s type=%s target=%s action=%s success=%v tests=%v",
		change.ID, change.Type, change.Target, change.Action, result.Success, result.TestsPassed)

	if m.db != nil {
		m.db.RPC(context.Background(), "log_orchestrator_event", map[string]interface{}{
			"p_event_type": "maintenance_change",
			"p_task_id":    change.ID,
			"p_message":    fmt.Sprintf("type=%s target=%s action=%s", change.Type, change.Target, change.Action),
			"p_details": map[string]interface{}{
				"success":      result.Success,
				"tests_passed": result.TestsPassed,
				"backup_path":  change.BackupPath,
			},
		})
	}
}

func (m *Maintenance) CheckApprovalChain(ctx context.Context, change *Change) error {
	if m.db == nil {
		return nil
	}

	var approvals []map[string]interface{}
	err := m.db.CallRPCInto(ctx, "get_change_approvals", map[string]any{"p_change_id": change.ID}, &approvals)
	if err != nil {
		return fmt.Errorf("get approvals: %w", err)
	}

	approvalMap := make(map[string]bool)
	for _, a := range approvals {
		if approver, ok := a["approver"].(string); ok {
			if approved, ok := a["approved"].(bool); ok {
				approvalMap[approver] = approved
			}
		}
	}

	required := m.requiredApprovals(change)
	for _, req := range required {
		if !approvalMap[req] {
			return fmt.Errorf("missing required approval: %s", req)
		}
	}

	return nil
}

func (m *Maintenance) requiredApprovals(change *Change) []string {
	switch m.ClassifyRisk(change) {
	case RiskLow:
		return []string{"supervisor"}
	case RiskMedium:
		return []string{"supervisor", "council"}
	case RiskHigh:
		return []string{"supervisor", "council", "human"}
	case RiskCritical:
		return []string{"supervisor", "council", "human"}
	default:
		return []string{"supervisor", "council", "human"}
	}
}

func (m *Maintenance) IsSystemChange(change *Change) bool {
	systemPaths := []string{
		"governor/",
		"config/",
		"scripts/",
		"docs/supabase-schema/",
	}

	for _, prefix := range systemPaths {
		if strings.HasPrefix(change.Target, prefix) {
			return true
		}
	}

	switch change.Type {
	case ChangeTypeConfig, ChangeTypeSchema:
		return true
	default:
		return false
	}
}

func (m *Maintenance) RepoPath() string {
	return m.repoPath
}
