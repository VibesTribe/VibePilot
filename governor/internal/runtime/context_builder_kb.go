package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// KBProvider is the interface for knowledge base queries.
// Implemented by internal/kb.KB, injected via SetKBProvider.
type KBProvider interface {
	SearchSymbols(ctx context.Context, query string, filterKind, filterRepo *string, limit int) ([]KBSymbol, error)
	GetFileSymbols(ctx context.Context, fileID string, limit int) ([]KBSymbol, error)
	SearchDocs(ctx context.Context, query string, filterRepo *string, limit int) ([]KBDocSection, error)
	SearchKnowledge(ctx context.Context, query string, filterType *string, limit int) ([]KBKnowledgeItem, error)
	Stats(ctx context.Context) ([]KBStatsEntry, error)
}

// KBSymbol represents a code symbol from the knowledge base.
type KBSymbol struct {
	ID            string  `json:"id"`
	QualifiedName string  `json:"qualified_name"`
	Kind          string  `json:"kind"`
	Name          string  `json:"name"`
	Summary       *string `json:"summary,omitempty"`
	FileID        string  `json:"file_id"`
	LineStart     int     `json:"line_start"`
	LineEnd       int     `json:"line_end"`
}

// KBDocSection represents a documentation section from the knowledge base.
type KBDocSection struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	DocPath string  `json:"doc_path"`
	Level   int     `json:"level"`
	Summary *string `json:"summary,omitempty"`
	RepoID  string  `json:"repo_id"`
}

// KBKnowledgeItem represents a knowledge item from the knowledge base.
type KBKnowledgeItem struct {
	ID       string  `json:"id"`
	ItemType string  `json:"item_type"`
	Name     string  `json:"name"`
	Title    string  `json:"title"`
	Summary  *string `json:"summary,omitempty"`
	Priority *string `json:"priority,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// KBStatsEntry represents a table row count from the knowledge base.
type KBStatsEntry struct {
	TableName string `json:"table_name"`
	RowCount  int64  `json:"row_count"`
}

// kbFileGroups collects symbols grouped by file for code map generation.
type kbFileGroup struct {
	fileID   string
	path     string // relative path extracted from fileID
	symbols  []KBSymbol
	lineCount int
}

// SetKBProvider injects the knowledge base provider for context building.
// Falls back to disk-based code map if nil or on errors.
func (b *ContextBuilder) SetKBProvider(kb KBProvider) {
	b.kb = kb
}

// loadCodeMapFromKB builds the code map from the knowledge base database.
// Returns the map text and true if successful, empty string and false if fallback needed.
func (b *ContextBuilder) loadCodeMapFromKB(ctx context.Context) (string, bool) {
	if b.kb == nil {
		return "", false
	}

	// Query all symbols from the vibepilot repo
	vibepilot := "vibepilot"
	symbols, err := b.kb.SearchSymbols(ctx, "", nil, &vibepilot, 5000)
	if err != nil || len(symbols) == 0 {
		return "", false
	}

	// Group symbols by file
	groups := make(map[string]*kbFileGroup)
	var fileOrder []string

	for _, s := range symbols {
		if _, exists := groups[s.FileID]; !exists {
			// Extract relative path from fileID ("vibepilot:path" → "path")
			path := s.FileID
			if idx := strings.Index(s.FileID, ":"); idx >= 0 {
				path = s.FileID[idx+1:]
			}
			groups[s.FileID] = &kbFileGroup{
				fileID: s.FileID,
				path:   path,
			}
			fileOrder = append(fileOrder, s.FileID)
		}
		groups[s.FileID].symbols = append(groups[s.FileID].symbols, s)
	}

	// Sort files by path
	sort.Strings(fileOrder)

	// Build the map in the same format as jcodemunch output
	var sb strings.Builder
	sb.WriteString("# VibePilot Code Map (from Knowledge Base)\n\n")

	for _, fileID := range fileOrder {
		g := groups[fileID]
		sb.WriteString(fmt.Sprintf("## %s\n", g.path))

		// Symbol listing
		for _, s := range g.symbols {
			kind := symbolKindPrefix(s.Kind)
			sig := ""
			if s.Summary != nil && *s.Summary != "" {
				sig = *s.Summary
			}
			if sig != "" {
				sb.WriteString(fmt.Sprintf("  %s %s\n", kind, sig))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s\n", kind, s.Name))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), true
}

// loadFileTreeFromKB extracts just the file headers from the knowledge base.
func (b *ContextBuilder) loadFileTreeFromKB(ctx context.Context) (string, bool) {
	if b.kb == nil {
		return "", false
	}

	vibepilot := "vibepilot"
	symbols, err := b.kb.SearchSymbols(ctx, "", nil, &vibepilot, 5000)
	if err != nil || len(symbols) == 0 {
		return "", false
	}

	// Collect unique files, preserving order
	seen := make(map[string]bool)
	var files []string
	for _, s := range symbols {
		if !seen[s.FileID] {
			seen[s.FileID] = true
			path := s.FileID
			if idx := strings.Index(s.FileID, ":"); idx >= 0 {
				path = s.FileID[idx+1:]
			}
			files = append(files, path)
		}
	}

	sort.Strings(files)

	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("## %s\n", f))
	}
	return sb.String(), true
}

// symbolKindPrefix returns a short prefix for the symbol kind.
func symbolKindPrefix(kind string) string {
	switch strings.ToLower(kind) {
	case "function":
		return "fn"
	case "method":
		return "fn"
	case "type", "struct", "interface":
		return "cl" // class-like
	case "constant":
		return "const"
	case "variable":
		return "var"
	default:
		return kind
	}
}
