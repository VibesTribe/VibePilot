package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

type Maintenance struct {
	repoPath string
	db       *db.DB
	agentID  string
}

type Config struct {
	RepoPath string
}

func New(cfg *Config) *Maintenance {
	if cfg == nil || cfg.RepoPath == "" {
		cwd, _ := os.Getwd()
		return &Maintenance{repoPath: cwd}
	}
	return &Maintenance{repoPath: cfg.RepoPath}
}

func (m *Maintenance) SetDB(database *db.DB) {
	m.db = database
}

func (m *Maintenance) SetAgentID(id string) {
	m.agentID = id
}

func (m *Maintenance) Run(ctx context.Context, pollInterval time.Duration) {
	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}

	log.Printf("Maintenance: started polling (interval=%v)", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Maintenance: shutting down")
			return
		case <-ticker.C:
			m.pollAndExecute(ctx)
		}
	}
}

func (m *Maintenance) pollAndExecute(ctx context.Context) {
	if m.db == nil {
		return
	}

	agentID := m.agentID
	if agentID == "" {
		agentID = "maintenance-governor"
	}

	cmd, err := m.db.ClaimNextCommand(ctx, agentID)
	if err != nil {
		log.Printf("Maintenance: claim error: %v", err)
		return
	}

	if cmd == nil {
		return
	}

	log.Printf("Maintenance: executing %s command %s", cmd.CommandType, cmd.ID[:8])

	result, execErr := m.executeCommand(ctx, cmd)

	if execErr != nil {
		log.Printf("Maintenance: command %s failed: %v", cmd.ID[:8], execErr)
		m.db.CompleteCommand(ctx, cmd.ID, false, nil, execErr.Error())
		return
	}

	log.Printf("Maintenance: command %s completed", cmd.ID[:8])
	m.db.CompleteCommand(ctx, cmd.ID, true, result, "")
}

func (m *Maintenance) executeCommand(ctx context.Context, cmd *db.MaintenanceCommand) (map[string]interface{}, error) {
	switch cmd.CommandType {
	case "create_branch":
		return m.executeCreateBranch(ctx, cmd.Payload)
	case "commit_code":
		return m.executeCommitCode(ctx, cmd.Payload)
	case "merge_branch":
		return m.executeMergeBranch(ctx, cmd.Payload)
	case "delete_branch":
		return m.executeDeleteBranch(ctx, cmd.Payload)
	case "tag_release":
		return m.executeTagRelease(ctx, cmd.Payload)
	default:
		return nil, fmt.Errorf("unknown command type: %s", cmd.CommandType)
	}
}

func (m *Maintenance) executeCreateBranch(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	branchName, _ := payload["branch_name"].(string)
	if branchName == "" {
		return nil, fmt.Errorf("branch_name required")
	}

	baseBranch, _ := payload["base_branch"].(string)
	if baseBranch == "" {
		baseBranch = "main"
	}

	if err := m.gitCommand(ctx, "checkout", baseBranch).Run(); err != nil {
		return nil, fmt.Errorf("checkout base: %w", err)
	}

	if err := m.gitCommand(ctx, "pull", "origin", baseBranch).Run(); err != nil {
		log.Printf("Maintenance: pull warning: %v", err)
	}

	var out bytes.Buffer
	createCmd := m.gitCommand(ctx, "checkout", "-b", branchName)
	createCmd.Stdout = &out
	createCmd.Stderr = &out

	if err := createCmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			if err := m.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
				return nil, fmt.Errorf("checkout existing: %w", err)
			}
			return map[string]interface{}{"branch": branchName, "status": "existing"}, nil
		}
		return nil, fmt.Errorf("create branch: %w - %s", err, out.String())
	}

	if err := m.gitCommand(ctx, "push", "-u", "origin", branchName).Run(); err != nil {
		return nil, fmt.Errorf("push branch: %w", err)
	}

	return map[string]interface{}{"branch": branchName, "status": "created"}, nil
}

func (m *Maintenance) executeCommitCode(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	branch, _ := payload["branch"].(string)
	if branch == "" {
		return nil, fmt.Errorf("branch required")
	}

	message, _ := payload["message"].(string)
	if message == "" {
		message = "automated commit"
	}

	if err := m.gitCommand(ctx, "checkout", branch).Run(); err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}

	if files, ok := payload["files"].([]interface{}); ok {
		for _, f := range files {
			file, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			path, _ := file["path"].(string)
			content, _ := file["content"].(string)

			if path == "" {
				continue
			}

			fullPath := filepath.Join(m.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return nil, fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("write file: %w", err)
			}
		}
	}

	if err := m.gitCommand(ctx, "add", ".").Run(); err != nil {
		return nil, fmt.Errorf("git add: %w", err)
	}

	var commitOut bytes.Buffer
	commitCmd := m.gitCommand(ctx, "commit", "-m", message)
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		outStr := commitOut.String()
		if strings.Contains(outStr, "nothing to commit") {
			return map[string]interface{}{"status": "no_changes"}, nil
		}
		return nil, fmt.Errorf("commit: %w - %s", err, outStr)
	}

	if err := m.gitCommand(ctx, "push").Run(); err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}

	var hashOut bytes.Buffer
	hashCmd := m.gitCommand(ctx, "rev-parse", "HEAD")
	hashCmd.Stdout = &hashOut
	commitHash := strings.TrimSpace(hashOut.String())

	return map[string]interface{}{"status": "committed", "hash": commitHash}, nil
}

func (m *Maintenance) executeMergeBranch(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	source, _ := payload["source"].(string)
	if source == "" {
		return nil, fmt.Errorf("source required")
	}

	target, _ := payload["target"].(string)
	if target == "" {
		target = "main"
	}

	deleteSource, _ := payload["delete_source"].(bool)

	if err := m.gitCommand(ctx, "checkout", target).Run(); err != nil {
		return nil, fmt.Errorf("checkout target: %w", err)
	}

	if err := m.gitCommand(ctx, "pull", "origin", target).Run(); err != nil {
		log.Printf("Maintenance: pull warning: %v", err)
	}

	var mergeOut bytes.Buffer
	mergeCmd := m.gitCommand(ctx, "merge", source)
	mergeCmd.Stdout = &mergeOut
	mergeCmd.Stderr = &mergeOut

	if err := mergeCmd.Run(); err != nil {
		return nil, fmt.Errorf("merge: %w - %s", err, mergeOut.String())
	}

	if err := m.gitCommand(ctx, "push", "origin", target).Run(); err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}

	if deleteSource {
		m.gitCommand(ctx, "branch", "-d", source).Run()
		m.gitCommand(ctx, "push", "origin", "--delete", source).Run()
	}

	return map[string]interface{}{"status": "merged", "source": source, "target": target}, nil
}

func (m *Maintenance) executeDeleteBranch(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	branchName, _ := payload["branch_name"].(string)
	if branchName == "" {
		return nil, fmt.Errorf("branch_name required")
	}

	m.gitCommand(ctx, "branch", "-d", branchName).Run()
	m.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()

	return map[string]interface{}{"status": "deleted", "branch": branchName}, nil
}

func (m *Maintenance) executeTagRelease(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	tag, _ := payload["tag"].(string)
	if tag == "" {
		return nil, fmt.Errorf("tag required")
	}

	target, _ := payload["target"].(string)
	if target == "" {
		target = "main"
	}

	message, _ := payload["message"].(string)
	if message == "" {
		message = "release " + tag
	}

	args := []string{"tag", "-a", tag, "-m", message}
	if target != "" {
		args = append(args, target)
	}

	if err := m.gitCommand(ctx, args...).Run(); err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}

	if err := m.gitCommand(ctx, "push", "origin", tag).Run(); err != nil {
		return nil, fmt.Errorf("push tag: %w", err)
	}

	return map[string]interface{}{"status": "tagged", "tag": tag}, nil
}

func (m *Maintenance) CreateBranch(ctx context.Context, branchName string) error {
	var out bytes.Buffer
	cmd := m.gitCommand(ctx, "checkout", "-b", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return m.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create branch: %w - %s", err, out.String())
	}
	return m.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (m *Maintenance) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	if err := m.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		outputMap = make(map[string]interface{})
	}

	if files, ok := outputMap["files"]; ok {
		filesList, ok := files.([]interface{})
		if !ok {
			return fmt.Errorf("files must be an array")
		}
		for _, f := range filesList {
			file, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			path, ok := file["path"].(string)
			if !ok {
				continue
			}
			content, ok := file["content"].(string)
			if !ok {
				continue
			}

			fullPath := filepath.Join(m.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		}
	}

	if output, ok := outputMap["output"]; ok && outputMap["files"] == nil {
		resultPath := filepath.Join(m.repoPath, "task_output.txt")
		content, _ := json.MarshalIndent(output, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	if response, ok := outputMap["response"]; ok {
		resultPath := filepath.Join(m.repoPath, "task_result.json")
		content, _ := json.MarshalIndent(response, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	if err := m.gitCommand(ctx, "add", ".").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	var commitOut bytes.Buffer
	commitCmd := m.gitCommand(ctx, "commit", "-m", "task output")
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		outStr := commitOut.String()
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "no changes added") {
			return fmt.Errorf("task produced no output: no files were created or modified")
		}
		return fmt.Errorf("git commit: %w - %s", err, outStr)
	}

	return m.gitCommand(ctx, "push").Run()
}

func (m *Maintenance) ReadBranchOutput(ctx context.Context, branchName string) ([]string, error) {
	var files []string

	cmd := m.gitCommand(ctx, "diff", "--name-only", "main..."+branchName)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

func (m *Maintenance) MergeBranch(ctx context.Context, sourceBranch, targetBranch string) error {
	m.gitCommand(ctx, "checkout", targetBranch).Run()
	m.gitCommand(ctx, "pull", "origin", targetBranch).Run()

	var out bytes.Buffer
	cmd := m.gitCommand(ctx, "merge", sourceBranch)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge: %w - %s", err, out.String())
	}

	return m.gitCommand(ctx, "push", "origin", targetBranch).Run()
}

func (m *Maintenance) DeleteBranch(ctx context.Context, branchName string) error {
	m.gitCommand(ctx, "branch", "-d", branchName).Run()
	m.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()
	return nil
}

func (m *Maintenance) ExecuteMerge(ctx context.Context, taskID, branchName string) error {
	if err := m.MergeBranch(ctx, branchName, "main"); err != nil {
		return err
	}

	m.DeleteBranch(ctx, branchName)
	return nil
}

func (m *Maintenance) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.repoPath
	return cmd
}
