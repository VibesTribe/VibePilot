package visualqa

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

type CaptureResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	Error   string `json:"error"`
}

func (v *VisualQA) captureScreenshot(ctx context.Context, url string, outputPath string, width int) (CaptureResult, error) {
	timeout := 60
	if v.config.CaptureTimeoutSeconds > 0 {
		timeout = v.config.CaptureTimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	scriptPath := v.config.RepoPath + "/governor/scripts/vqa_capture.py"
	cmd := exec.CommandContext(ctx, "python3", scriptPath, url, outputPath, strconv.Itoa(width))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CaptureResult{Success: false, Error: fmt.Sprintf("Python script execution failed: %v, Output: %s", err, string(output))},
			fmt.Errorf("[VisualQA] Screenshot capture failed for %s: %v, Output: %s", url, err, string(output))
	}

	var result CaptureResult
	if err := json.Unmarshal(output, &result); err != nil {
		return CaptureResult{Success: false, Error: fmt.Sprintf("Failed to parse Python script output: %v, Output: %s", err, string(output))},
			fmt.Errorf("[VisualQA] Failed to parse Python script output for %s: %v, Output: %s", url, err, string(output))
	}

	if !result.Success {
		return result, fmt.Errorf("[VisualQA] Python script reported error for %s: %s", url, result.Error)
	}

	return result, nil
}
