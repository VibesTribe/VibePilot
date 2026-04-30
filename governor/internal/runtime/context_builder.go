package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RPCQuerier interface {
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	RPC(ctx context.Context, name string, params map[string]any) ([]byte, error)
}

// MCPToolLister provides discovered MCP tool info for agent context.
// Implemented by internal/mcp.Registry.
type MCPToolLister interface {
	ListToolInfo() []MCPToolInfo
}

// MCPToolInfo describes a single MCP tool for agent context injection.
type MCPToolInfo struct {
	Name        string
	Description string
	ServerName  string
}

type ContextBuilder struct {
	db       RPCQuerier
	mcpTools MCPToolLister
	repoPath string
	cfg      *CodeMapConfig

	mu           sync.RWMutex
	codeMapCache string
	codeMapLoaded time.Time
}

func NewContextBuilder(db RPCQuerier, repoPath string, cfg *CodeMapConfig) *ContextBuilder {
	if cfg == nil {
		cfg = DefaultCodeMapConfig()
	}
	return &ContextBuilder{db: db, repoPath: repoPath, cfg: cfg}
}

// SetMCPRegistry injects the MCP tool registry for context building.
func (b *ContextBuilder) SetMCPRegistry(registry MCPToolLister) {
	b.mcpTools = registry
}

// loadCodeMap reads the code map from disk with TTL-based caching.
// After TTL expires, next call re-reads from disk.
func (b *ContextBuilder) loadCodeMap() (string, error) {
	ttl := time.Duration(b.cfg.CacheTTLMins) * time.Minute

	b.mu.RLock()
	if b.codeMapCache != "" && time.Since(b.codeMapLoaded) < ttl {
		cached := b.codeMapCache
		b.mu.RUnlock()
		return cached, nil
	}
	b.mu.RUnlock()

	// Cache expired or empty -- reload
	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check after acquiring write lock
	if b.codeMapCache != "" && time.Since(b.codeMapLoaded) < ttl {
		return b.codeMapCache, nil
	}

	mapPath := filepath.Join(b.repoPath, b.cfg.Path)
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return "", fmt.Errorf("read code map: %w", err)
	}

	b.codeMapCache = string(data)
	b.codeMapLoaded = time.Now()
	return b.codeMapCache, nil
}

// InvalidateCache forces a reload on next access (called after jcodemunch refresh).
func (b *ContextBuilder) InvalidateCache() {
	b.mu.Lock()
	b.codeMapCache = ""
	b.codeMapLoaded = time.Time{}
	b.mu.Unlock()
}

// loadFileTree extracts just the file headers from map.md (lightweight for supervisor/council).
func (b *ContextBuilder) loadFileTree() (string, error) {
	fullMap, err := b.loadCodeMap()
	if err != nil {
		return "", err
	}
	var lines []string
	for _, line := range strings.Split(fullMap, "\n") {
		if strings.HasPrefix(line, "## ") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// ReadFileContent reads a specific file from the repo. Used for targeted task context.
// Returns the file content or an error message string (never blocks execution).
func (b *ContextBuilder) ReadFileContent(relPath string) (string, bool) {
	fullPath := filepath.Join(b.repoPath, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "[FILE DOES NOT YET EXIST - CREATE IT]", false
		}
		return fmt.Sprintf("[ERROR READING FILE: %v]", err), false
	}
	return string(data), true
}

// BuildBaseContext returns the codebase file tree. Agents with file_tree policy get this.
func (b *ContextBuilder) BuildBaseContext() string {
	fileTree, err := b.loadFileTree()
	if err != nil {
		return fmt.Sprintf("<!-- File tree unavailable: %v -->\n", err)
	}
	if fileTree == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Codebase Files\n\n")
	sb.WriteString("These are the ONLY files in this codebase. Do NOT reference files not listed here.\n\n")
	sb.WriteString(fileTree)
	sb.WriteString("\n")
	return sb.String()
}

// BuildTargetedContext reads specific files for task runner context.
// Returns file contents formatted for executor prompt injection.
func (b *ContextBuilder) BuildTargetedContext(targetFiles []string) string {
	if len(targetFiles) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Files You Will Modify\n\n")
	for _, f := range targetFiles {
		content, _ := b.ReadFileContent(f)
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", f, content))
	}
	return sb.String()
}

func (b *ContextBuilder) BuildPlannerContext(ctx context.Context, projectType string) (string, error) {
	var contextBuilder strings.Builder

	// Query incomplete slices for task numbering context
	slices, err := b.db.RPC(ctx, "get_slice_task_info", nil)
	if err == nil {
		var sliceList []map[string]any
		if err := json.Unmarshal(slices, &sliceList); err == nil && len(sliceList) > 0 {
			contextBuilder.WriteString("## Incomplete Slices\n\n")
			contextBuilder.WriteString("If your PRD continues an existing slice, use that slice_id and continue numbering from the last task.\n")
			contextBuilder.WriteString("Otherwise, create a new slice_id and start at T001.\n\n")
			for _, s := range sliceList {
				sliceID, _ := s["slice_id"].(string)
				lastTask, _ := s["last_task_number"].(string)
				count, _ := s["task_count"].(float64)
				if sliceID != "" {
					nextNum := int(count) + 1
					contextBuilder.WriteString(fmt.Sprintf("- %s: %d tasks, last %s → continue at T%03d\n", sliceID, int(count), lastTask, nextNum))
				}
			}
			contextBuilder.WriteString("\n")
		}
	}

	rules, err := b.db.RPC(ctx, "get_planner_rules", map[string]any{
		"p_applies_to": projectType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				source, _ := rule["source"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s (from %s)\n", ruleText, source))
			}
		}
	}

	failures, err := b.db.RPC(ctx, "get_recent_failures", map[string]any{
		"p_task_type": projectType,
		"p_since":     "NOW() - INTERVAL '7 days'",
	})
	if err == nil {
		var failureList []map[string]any
		if err := json.Unmarshal(failures, &failureList); err == nil && len(failureList) > 0 {
			contextBuilder.WriteString("\n## Recent Failures to Avoid\n\n")
			for _, f := range failureList {
				failureType, _ := f["failure_type"].(string)
				modelID, _ := f["model_id"].(string)
				count, _ := f["failure_count"].(float64)
				if modelID != "" {
					contextBuilder.WriteString(fmt.Sprintf("- %s on %s (%d occurrences)\n", failureType, modelID, int(count)))
				} else {
					contextBuilder.WriteString(fmt.Sprintf("- %s (%d occurrences)\n", failureType, int(count)))
				}
			}
		}
	}

	// Inject available MCP tools from approved servers
	if b.mcpTools != nil {
		tools := b.mcpTools.ListToolInfo()
		if len(tools) > 0 {
			contextBuilder.WriteString("\n## Available MCP Tools\n\n")
			contextBuilder.WriteString("The following external tools are available from approved MCP servers:\n\n")
			for _, t := range tools {
				contextBuilder.WriteString(fmt.Sprintf("- **%s** (via %s): %s\n", t.Name, t.ServerName, t.Description))
			}
		}
	}

	// Inject full code map so planner references real files and follows existing patterns
	if codeMap, err := b.loadCodeMap(); err == nil && codeMap != "" {
		contextBuilder.WriteString("\n## Codebase Map\n\n")
		contextBuilder.WriteString("The following is the COMPLETE file listing with symbols for this codebase.\n")
		contextBuilder.WriteString("Reference ONLY these files. Do NOT invent file paths.\n")
		contextBuilder.WriteString("Follow existing patterns (function signatures, import styles, struct names).\n\n")
		contextBuilder.WriteString(codeMap)
	} else if err != nil {
		contextBuilder.WriteString(fmt.Sprintf("\n<!-- Code map unavailable: %v -->\n", err))
	}

	return contextBuilder.String(), nil
}

func (b *ContextBuilder) BuildSupervisorContext(ctx context.Context, taskType string) (string, error) {
	var contextBuilder strings.Builder

	// Inject file tree so supervisor can verify plan references real files
	if fileTree, err := b.loadFileTree(); err == nil && fileTree != "" {
		contextBuilder.WriteString("## Codebase File Tree\n\n")
		contextBuilder.WriteString("When reviewing plans, verify that ALL file references in the plan match files listed below.\n")
		contextBuilder.WriteString("REJECT plans that reference files not in this list.\n\n")
		contextBuilder.WriteString(fileTree)
		contextBuilder.WriteString("\n\n")
	} else if err != nil {
		contextBuilder.WriteString(fmt.Sprintf("<!-- File tree unavailable: %v -->\n\n", err))
	}

	rules, err := b.db.RPC(ctx, "get_supervisor_rules", map[string]any{
		"p_applies_to": taskType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Review Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", ruleText))
			}
		}
	}

	return contextBuilder.String(), nil
}

// BuildCouncilContext returns context for council members reviewing plans.
func (b *ContextBuilder) BuildCouncilContext(ctx context.Context, taskType string) (string, error) {
	var sb strings.Builder

	// File tree so council can verify plan references real files
	if fileTree, err := b.loadFileTree(); err == nil && fileTree != "" {
		sb.WriteString("## Codebase File Tree\n\n")
		sb.WriteString("When voting on plans, verify that ALL file references match files listed below.\n")
		sb.WriteString("Vote to REJECT plans that reference files not in this list.\n\n")
		sb.WriteString(fileTree)
		sb.WriteString("\n\n")
	} else if err != nil {
		sb.WriteString(fmt.Sprintf("<!-- File tree unavailable: %v -->\n\n", err))
	}

	return sb.String(), nil
}

func (b *ContextBuilder) BuildTesterContext(ctx context.Context, taskType string) (string, error) {
	var contextBuilder strings.Builder

	rules, err := b.db.RPC(ctx, "get_tester_rules", map[string]any{
		"p_applies_to": taskType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Testing Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", ruleText))
			}
		}
	}

	return contextBuilder.String(), nil
}

func (b *ContextBuilder) GetRoutingHeuristic(ctx context.Context, taskType string) (modelID string, action map[string]any) {
	result, err := b.db.RPC(ctx, "get_heuristic", map[string]any{
		"p_task_type": taskType,
		"p_condition": map[string]any{},
	})
	if err != nil {
		return "", nil
	}

	var heuristics []map[string]any
	if err := json.Unmarshal(result, &heuristics); err != nil || len(heuristics) == 0 {
		return "", nil
	}

	h := heuristics[0]
	modelID, _ = h["preferred_model"].(string)
	action, _ = h["action"].(map[string]any)
	return modelID, action
}

func (b *ContextBuilder) GetProblemSolution(ctx context.Context, failureType, taskType string) (solutionType string, solutionModel string, details map[string]any) {
	result, err := b.db.RPC(ctx, "get_problem_solution", map[string]any{
		"p_failure_type": failureType,
		"p_task_type":    taskType,
		"p_keywords":     []string{},
	})
	if err != nil {
		return "", "", nil
	}

	var solutions []map[string]any
	if err := json.Unmarshal(result, &solutions); err != nil || len(solutions) == 0 {
		return "", "", nil
	}

	s := solutions[0]
	solutionType, _ = s["solution_type"].(string)
	solutionModel, _ = s["solution_model"].(string)
	details, _ = s["solution_details"].(map[string]any)
	return solutionType, solutionModel, details
}
