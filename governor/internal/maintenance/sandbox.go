package maintenance

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Maintenance) CreateSandbox() (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	sandboxPath := filepath.Join(m.sandboxDir, timestamp)

	if err := os.MkdirAll(sandboxPath, 0755); err != nil {
		return "", fmt.Errorf("create sandbox dir: %w", err)
	}

	if err := m.copyDir(m.repoPath, sandboxPath); err != nil {
		os.RemoveAll(sandboxPath)
		return "", fmt.Errorf("copy repo to sandbox: %w", err)
	}

	log.Printf("Maintenance: created sandbox at %s", sandboxPath)
	return sandboxPath, nil
}

func (m *Maintenance) ApplyToSandbox(sandboxPath string, change *Change) error {
	targetPath := filepath.Join(sandboxPath, change.Target)

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sandbox dir: %w", err)
	}

	if change.Action == "delete" {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete in sandbox: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(targetPath, change.Content, 0644); err != nil {
		return fmt.Errorf("write sandbox file: %w", err)
	}

	return nil
}

type SandboxTestResult struct {
	Passed   bool
	Failures []string
}

func (m *Maintenance) TestInSandbox(sandboxPath string) (SandboxTestResult, error) {
	result := SandboxTestResult{Passed: true}

	if _, err := exec.LookPath("go"); err == nil {
		cmd := exec.CommandContext(context.Background(), "go", "build", "./...")
		cmd.Dir = sandboxPath
		if output, err := cmd.CombinedOutput(); err != nil {
			result.Passed = false
			result.Failures = append(result.Failures, "go build: "+string(output))
		}

		cmd = exec.CommandContext(context.Background(), "go", "vet", "./...")
		cmd.Dir = sandboxPath
		if output, err := cmd.CombinedOutput(); err != nil {
			result.Passed = false
			result.Failures = append(result.Failures, "go vet: "+string(output))
		}
	}

	if _, err := exec.LookPath("pytest"); err == nil {
		cmd := exec.CommandContext(context.Background(), "pytest", "--tb=short", "-q")
		cmd.Dir = sandboxPath
		if output, err := cmd.CombinedOutput(); err != nil {
			result.Passed = false
			result.Failures = append(result.Failures, "pytest: "+string(output))
		}
	}

	return result, nil
}

func (m *Maintenance) CleanupSandbox(sandboxPath string) error {
	if sandboxPath == "" || sandboxPath == m.repoPath {
		return nil
	}

	if err := os.RemoveAll(sandboxPath); err != nil {
		log.Printf("Maintenance: warning - failed to cleanup sandbox: %v", err)
		return err
	}

	log.Printf("Maintenance: cleaned up sandbox at %s", sandboxPath)
	return nil
}

func (m *Maintenance) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			if skipDir(info.Name()) {
				return filepath.SkipDir
			}
			return os.MkdirAll(dstPath, info.Mode())
		}

		if skipFile(info.Name()) {
			return nil
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func skipDir(name string) bool {
	skip := []string{".git", "node_modules", "__pycache__", ".venv", "venv", "vendor"}
	for _, s := range skip {
		if name == s {
			return true
		}
	}
	return false
}

func skipFile(name string) bool {
	skip := []string{".DS_Store", "Thumbs.db"}
	for _, s := range skip {
		if name == s {
			return true
		}
	}
	return false
}
