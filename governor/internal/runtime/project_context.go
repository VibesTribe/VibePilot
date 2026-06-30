package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectContext holds the loaded context for a PIF-isolated project.
// When a task belongs to a non-vibepilot project, this context is assembled
// from the project's ~/projects/{slug}/ directory and injected into the
// agent's prompt — replacing the vibepilot codebase context entirely.
//
// This is Phase B of the Project Isolation Framework. It ensures:
// - The agent sees the PROJECT'S file tree, not vibepilot's
// - The agent reads the PROJECT'S .hermes.md rules, not vibepilot's
// - The agent knows which skills exist in the PROJECT'S skills/ directory
// - The agent has NO visibility into other projects or vibepilot internals
type ProjectContext struct {
	// Slug identifies the project (e.g., "sealed")
	Slug string

	// ProjectDir is ~/projects/{slug}/
	ProjectDir string

	// RepoDir is ~/projects/{slug}/repo/
	RepoDir string

	// HermesMD is the contents of the project's .hermes.md (agent rules)
	HermesMD string

	// Skills lists skill names found in the project's skills/ directory
	Skills []string

	// FileTree is a lightweight listing of files in the project's repo/
	FileTree string

	// VibepilotTOML is the parsed manifest (key fields, not raw TOML)
	Manifest *ProjectManifest
}

// ProjectManifest holds the key fields from vibepilot.toml that the agent
// needs to know about (agent runtime, deploy target, database config, etc.)
type ProjectManifest struct {
	AgentRuntime   string // "hermes" | "claude-code" | "opencode" | etc.
	AgentProfile   string // Hermes profile name
	DeployTarget   string // "cloudflare" | "vercel" | "docker" | "none"
	DatabaseType   string // "sqlite" | "postgres" | "supabase" | "none"
	DatabasePath   string // path to SQLite file if applicable
	Description    string
	DisplayName    string
	EgressAllow    []string
	ApprovalReq    bool
}

// projectsBaseDir returns ~/projects/
func projectsBaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "projects")
}

// LoadProjectContext assembles the full project context from disk.
// If the project directory doesn't exist or is incomplete, it returns nil
// (non-fatal — the task will fall back to vibepilot context).
func LoadProjectContext(slug string) *ProjectContext {
	if slug == "" || slug == "vibepilot" {
		return nil // vibepilot uses its own context builder
	}

	projectDir := filepath.Join(projectsBaseDir(), slug)
	if _, err := os.Stat(projectDir); err != nil {
		return nil // project directory doesn't exist (not scaffolded)
	}

	ctx := &ProjectContext{
		Slug:       slug,
		ProjectDir: projectDir,
		RepoDir:    filepath.Join(projectDir, "repo"),
	}

	// Load .hermes.md (agent rules for this project)
	ctx.HermesMD = loadFileIfExists(filepath.Join(projectDir, ".hermes.md"))

	// Load vibepilot.toml manifest fields
	ctx.Manifest = loadManifest(projectDir, slug)

	// List skills
	ctx.Skills = listSkills(filepath.Join(projectDir, "skills"))

	// Build file tree from repo/
	ctx.FileTree = buildProjectFileTree(ctx.RepoDir)

	return ctx
}

// AssemblePrompt injects the project context into a prompt string.
// This REPLACES the vibepilot context entirely — the agent only sees
// this project's files, rules, and skills.
func (pc *ProjectContext) AssemblePrompt() string {
	var sb strings.Builder

	sb.WriteString("\n\n--- PROJECT CONTEXT (PIF ISOLATED) ---\n\n")

	if pc.Manifest != nil {
		sb.WriteString(fmt.Sprintf("## Project: %s\n\n", pc.Manifest.DisplayName))
		if pc.Manifest.Description != "" {
			sb.WriteString(pc.Manifest.Description)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("**Agent Runtime:** %s\n", pc.Manifest.AgentRuntime))
		sb.WriteString(fmt.Sprintf("**Deploy Target:** %s\n", pc.Manifest.DeployTarget))
		sb.WriteString(fmt.Sprintf("**Database:** %s", pc.Manifest.DatabaseType))
		if pc.Manifest.DatabasePath != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", pc.Manifest.DatabasePath))
		}
		sb.WriteString("\n")
		if pc.Manifest.ApprovalReq {
			sb.WriteString("**Approval Required:** Yes — human must approve before deploy/merge\n")
		}
		if len(pc.Manifest.EgressAllow) > 0 {
			sb.WriteString(fmt.Sprintf("**Network Egress:** %s\n", strings.Join(pc.Manifest.EgressAllow, ", ")))
		} else {
			sb.WriteString("**Network Egress:** No external access allowed\n")
		}
		sb.WriteString("\n")
	}

	// Project agent rules
	if pc.HermesMD != "" {
		sb.WriteString("## Project Rules (.hermes.md)\n\n")
		sb.WriteString(pc.HermesMD)
		sb.WriteString("\n\n")
	}

	// Available skills
	if len(pc.Skills) > 0 {
		sb.WriteString("## Available Skills (project-specific)\n\n")
		for _, s := range pc.Skills {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// File tree
	if pc.FileTree != "" {
		sb.WriteString("## Codebase Files\n\n")
		sb.WriteString("These are the ONLY files in this project's codebase.\n")
		sb.WriteString("Do NOT reference files not listed here.\n\n")
		sb.WriteString(pc.FileTree)
		sb.WriteString("\n")
	} else {
		sb.WriteString("## Codebase Files\n\n")
		sb.WriteString("(Repository is empty — this is a new project)\n\n")
	}

	sb.WriteString("--- END PROJECT CONTEXT ---\n")

	return sb.String()
}

// loadFileIfExists reads a file, returning empty string if it doesn't exist.
func loadFileIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// loadManifest reads key fields from vibepilot.toml.
// Uses simple line parsing (no TOML library dependency needed).
func loadManifest(projectDir, slug string) *ProjectManifest {
	m := &ProjectManifest{
		AgentRuntime: "hermes",   // defaults
		AgentProfile: slug,
		DeployTarget: "none",
		DatabaseType: "none",
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "vibepilot.toml"))
	if err != nil {
		return m
	}

	lines := strings.Split(string(data), "\n")
	currentSection := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed
			continue
		}

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Parse key = "value" or key = value
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip inline comments (everything after # that's outside quotes)
		// TOML values are always quoted in our manifest, so we can safely
		// find the closing quote and cut everything after it.
		if strings.HasPrefix(val, "\"") {
			endQuote := strings.Index(val[1:], "\"")
			if endQuote >= 0 {
				val = val[1 : 1+endQuote]
			} else {
				val = strings.Trim(val, "\"")
			}
		} else {
			// Unquoted value — strip anything after #
			if idx := strings.Index(val, "#"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
			}
		}

		switch currentSection {
		case "[project]":
			if key == "display_name" {
				m.DisplayName = val
			}
			if key == "description" {
				m.Description = val
			}
		case "[agent]":
			if key == "runtime" {
				m.AgentRuntime = val
			}
			if key == "profile" {
				m.AgentProfile = val
			}
		case "[deploy]":
			if key == "target" {
				m.DeployTarget = val
			}
		case "[database]":
			if key == "type" {
				m.DatabaseType = val
			}
			if key == "edge_path" {
				m.DatabasePath = val
			}
		case "[network]":
			if key == "egress_allow" && val != "[]" {
				// Parse ["domain1", "domain2"]
				val = strings.Trim(val, "[]")
				for _, d := range strings.Split(val, ",") {
					d = strings.Trim(strings.TrimSpace(d), "\"")
					if d != "" {
						m.EgressAllow = append(m.EgressAllow, d)
					}
				}
			}
		case "[execution]":
			if key == "approval_required" && val == "true" {
				m.ApprovalReq = true
			}
		}
	}

	return m
}

// listSkills returns skill directory names in the project's skills/ folder.
func listSkills(skillsDir string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	var skills []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			skills = append(skills, entry.Name())
		}
	}
	sort.Strings(skills)
	return skills
}

// buildProjectFileTree creates a lightweight file listing from the project's repo/.
// Shows directories and source files (not .git, node_modules, etc.).
func buildProjectFileTree(repoDir string) string {
	if _, err := os.Stat(repoDir); err != nil {
		return ""
	}

	var lines []string
	excludeDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"__pycache__":  true,
	}

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, _ := filepath.Rel(repoDir, path)
		if rel == "." {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		for _, p := range parts {
			if excludeDirs[p] {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			lines = append(lines, fmt.Sprintf("## %s/", rel))
		} else {
			lines = append(lines, fmt.Sprintf("  %s", rel))
		}
		return nil
	})

	if err != nil || len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}
