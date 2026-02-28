package runtime

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"strings"
	"time"
)

type PRDWatcherConfig struct {
	Enabled   bool          `json:"enabled"`
	RepoPath  string        `json:"repo_path"`
	Branch    string        `json:"branch"`
	Directory string        `json:"directory"`
	Interval  time.Duration `json:"interval"`
}

type PRDWatcher struct {
	db       RPCQuerier
	cfg      PRDWatcherConfig
	lastSeen map[string]string
	stop     chan struct{}
}

func NewPRDWatcher(db RPCQuerier, cfg PRDWatcherConfig) *PRDWatcher {
	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Directory == "" {
		cfg.Directory = "docs/prd"
	}

	return &PRDWatcher{
		db:       db,
		cfg:      cfg,
		lastSeen: make(map[string]string),
		stop:     make(chan struct{}),
	}
}

func (w *PRDWatcher) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		log.Println("[PRDWatcher] Disabled in config")
		return
	}

	log.Printf("[PRDWatcher] Starting (branch: %s, directory: %s, interval: %v)", w.cfg.Branch, w.cfg.Directory, w.cfg.Interval)

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	w.checkForNewPRDs(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[PRDWatcher] Stopped by context")
			return
		case <-w.stop:
			log.Println("[PRDWatcher] Stopped")
			return
		case <-ticker.C:
			w.checkForNewPRDs(ctx)
		}
	}
}

func (w *PRDWatcher) checkForNewPRDs(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "-r", w.cfg.Branch, "--name-only")
	cmd.Dir = w.cfg.RepoPath
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[PRDWatcher] Failed to list files: %v", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, w.cfg.Directory) {
			continue
		}
		if !strings.HasSuffix(line, ".md") {
			continue
		}

		shaCmd := exec.CommandContext(ctx, "git", "rev-parse", w.cfg.Branch+":"+line)
		shaCmd.Dir = w.cfg.RepoPath
		shaOutput, err := shaCmd.Output()
		if err != nil {
			continue
		}
		sha := strings.TrimSpace(string(shaOutput))

		if w.lastSeen[line] == sha {
			continue
		}
		w.lastSeen[line] = sha

		if w.planExistsForPRD(ctx, line) {
			continue
		}

		log.Printf("[PRDWatcher] New PRD detected: %s", line)
		w.createPlan(ctx, line)
	}
}

func (w *PRDWatcher) planExistsForPRD(ctx context.Context, prdPath string) bool {
	result, err := w.db.Query(ctx, "plans", map[string]any{
		"prd_path": prdPath,
		"limit":    1,
	})
	if err != nil {
		return false
	}

	var plans []map[string]any
	if err := json.Unmarshal(result, &plans); err != nil {
		return false
	}

	return len(plans) > 0
}

func (w *PRDWatcher) createPlan(ctx context.Context, prdPath string) {
	_, err := w.db.RPC(ctx, "create_plan", map[string]any{
		"p_project_id": nil,
		"p_prd_path":   prdPath,
		"p_plan_path":  nil,
	})
	if err != nil {
		log.Printf("[PRDWatcher] Failed to create plan for %s: %v", prdPath, err)
		return
	}
	log.Printf("[PRDWatcher] Created plan for %s", prdPath)
}

func (w *PRDWatcher) Stop() {
	close(w.stop)
}
