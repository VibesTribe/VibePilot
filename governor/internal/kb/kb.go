package kb

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// KB provides typed access to the VibePilot Knowledge Base tables.
// It uses its own pgxpool connection to PostgreSQL.
type KB struct {
	pool *pgxpool.Pool
}

// Symbol represents a code symbol from kb_code_symbols.
type Symbol struct {
	ID            string  `json:"id"`
	QualifiedName string  `json:"qualified_name"`
	Kind          string  `json:"kind"`
	Name          string  `json:"name"`
	Summary       *string `json:"summary,omitempty"`
	FileID        string  `json:"file_id"`
	LineStart     int     `json:"line_start"`
	LineEnd       int     `json:"line_end"`
}

// DocSection represents a documentation section from kb_doc_sections.
type DocSection struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	DocPath string  `json:"doc_path"`
	Level   int     `json:"level"`
	Summary *string `json:"summary,omitempty"`
	RepoID  string  `json:"repo_id"`
}

// Edge represents a code relationship from kb_code_edges.
type Edge struct {
	SourceQualified string  `json:"source_qualified,omitempty"`
	TargetQualified string  `json:"target_qualified,omitempty"`
	Kind            string  `json:"kind"`
	FileID          *string `json:"file_id,omitempty"`
	Line            *int    `json:"line,omitempty"`
}

// KnowledgeItem represents a knowledge item from kb_knowledge_items.
type KnowledgeItem struct {
	ID       string  `json:"id"`
	ItemType string  `json:"item_type"`
	Name     string  `json:"name"`
	Title    string  `json:"title"`
	Summary  *string `json:"summary,omitempty"`
	Priority *string `json:"priority,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// BlastRadiusEntry represents a caller in the transitive call graph.
type BlastRadiusEntry struct {
	QualifiedName string `json:"qualified_name"`
	CallDepth     int    `json:"call_depth"`
	EdgeKind      string `json:"edge_kind"`
}

// Skill represents a Hermes agent skill from kb_skills.
type Skill struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    *string `json:"category,omitempty"`
	Description *string `json:"description,omitempty"`
}

// StatsEntry represents a table row count from kb_stats.
type StatsEntry struct {
	TableName string `json:"table_name"`
	RowCount  int64  `json:"row_count"`
}

// New creates a new KB instance connected to the given PostgreSQL database.
// connString should be a standard pgx connection string (e.g. "dbname=vibepilot").
func New(ctx context.Context, connString string) (*KB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("kb: parse connection string: %w", err)
	}

	// Small pool — KB queries are lightweight, not high-concurrency
	config.MinConns = 1
	config.MaxConns = 3

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("kb: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("kb: ping: %w", err)
	}

	return &KB{pool: pool}, nil
}

// Close releases the connection pool.
func (k *KB) Close() {
	k.pool.Close()
}

// SearchSymbols searches code symbols by name, qualified_name, or summary.
func (k *KB) SearchSymbols(ctx context.Context, query string, filterKind, filterRepo *string, limit int) ([]Symbol, error) {
	sql := `SELECT * FROM kb_search_symbols($1, $2, $3, $4)`
	rows, err := k.pool.Query(ctx, sql, query, limit, filterKind, filterRepo)
	if err != nil {
		return nil, fmt.Errorf("kb: search_symbols: %w", err)
	}
	defer rows.Close()

	var results []Symbol
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.QualifiedName, &s.Kind, &s.Name,
			&s.Summary, &s.FileID, &s.LineStart, &s.LineEnd); err != nil {
			return nil, fmt.Errorf("kb: scan symbol: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// SearchDocs searches documentation sections by title or summary.
func (k *KB) SearchDocs(ctx context.Context, query string, filterRepo *string, limit int) ([]DocSection, error) {
	sql := `SELECT * FROM kb_search_docs($1, $2, $3)`
	rows, err := k.pool.Query(ctx, sql, query, limit, filterRepo)
	if err != nil {
		return nil, fmt.Errorf("kb: search_docs: %w", err)
	}
	defer rows.Close()

	var results []DocSection
	for rows.Next() {
		var d DocSection
		if err := rows.Scan(&d.ID, &d.Title, &d.DocPath, &d.Level,
			&d.Summary, &d.RepoID); err != nil {
			return nil, fmt.Errorf("kb: scan doc: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetCallers returns all functions that call/import/reference a given symbol.
func (k *KB) GetCallers(ctx context.Context, qualifiedName string, limit int) ([]Edge, error) {
	sql := `SELECT * FROM kb_get_callers($1, $2)`
	rows, err := k.pool.Query(ctx, sql, qualifiedName, limit)
	if err != nil {
		return nil, fmt.Errorf("kb: get_callers: %w", err)
	}
	defer rows.Close()

	var results []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.SourceQualified, &e.Kind, &e.FileID, &e.Line); err != nil {
			return nil, fmt.Errorf("kb: scan caller: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// GetCallees returns all functions called/imported/referenced by a given symbol.
func (k *KB) GetCallees(ctx context.Context, qualifiedName string, limit int) ([]Edge, error) {
	sql := `SELECT * FROM kb_get_callees($1, $2)`
	rows, err := k.pool.Query(ctx, sql, qualifiedName, limit)
	if err != nil {
		return nil, fmt.Errorf("kb: get_callees: %w", err)
	}
	defer rows.Close()

	var results []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.TargetQualified, &e.Kind, &e.FileID, &e.Line); err != nil {
			return nil, fmt.Errorf("kb: scan callee: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// GetBlastRadius returns all transitive callers of a symbol (for change impact analysis).
func (k *KB) GetBlastRadius(ctx context.Context, qualifiedName string, maxDepth int) ([]BlastRadiusEntry, error) {
	sql := `SELECT * FROM kb_get_blast_radius($1, $2)`
	rows, err := k.pool.Query(ctx, sql, qualifiedName, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("kb: get_blast_radius: %w", err)
	}
	defer rows.Close()

	var results []BlastRadiusEntry
	for rows.Next() {
		var b BlastRadiusEntry
		if err := rows.Scan(&b.QualifiedName, &b.CallDepth, &b.EdgeKind); err != nil {
			return nil, fmt.Errorf("kb: scan blast radius: %w", err)
		}
		results = append(results, b)
	}
	return results, rows.Err()
}

// GetFileSymbols returns all code symbols defined in a file.
func (k *KB) GetFileSymbols(ctx context.Context, fileID string, limit int) ([]Symbol, error) {
	sql := `SELECT id, kind, name, qualified_name, line_start, line_end, summary FROM kb_get_file_symbols($1, $2)`
	rows, err := k.pool.Query(ctx, sql, fileID, limit)
	if err != nil {
		return nil, fmt.Errorf("kb: get_file_symbols: %w", err)
	}
	defer rows.Close()

	var results []Symbol
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.Kind, &s.Name, &s.QualifiedName,
			&s.LineStart, &s.LineEnd, &s.Summary); err != nil {
			return nil, fmt.Errorf("kb: scan file symbol: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// SearchKnowledge searches knowledge items by type, name, or content.
func (k *KB) SearchKnowledge(ctx context.Context, query string, filterType *string, limit int) ([]KnowledgeItem, error) {
	sql := `SELECT * FROM kb_search_knowledge($1, $2, $3)`
	rows, err := k.pool.Query(ctx, sql, query, filterType, limit)
	if err != nil {
		return nil, fmt.Errorf("kb: search_knowledge: %w", err)
	}
	defer rows.Close()

	var results []KnowledgeItem
	for rows.Next() {
		var ki KnowledgeItem
		if err := rows.Scan(&ki.ID, &ki.ItemType, &ki.Name, &ki.Title,
			&ki.Summary, &ki.Priority, &ki.Status); err != nil {
			return nil, fmt.Errorf("kb: scan knowledge item: %w", err)
		}
		results = append(results, ki)
	}
	return results, rows.Err()
}

// SearchSkills searches Hermes agent skills by name, description, or content.
func (k *KB) SearchSkills(ctx context.Context, query string, filterCategory *string, limit int) ([]Skill, error) {
	sql := `SELECT * FROM kb_search_skills($1, $2, $3)`
	rows, err := k.pool.Query(ctx, sql, query, filterCategory, limit)
	if err != nil {
		return nil, fmt.Errorf("kb: search_skills: %w", err)
	}
	defer rows.Close()

	var results []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description); err != nil {
			return nil, fmt.Errorf("kb: scan skill: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// Stats returns row counts for all kb_ tables.
func (k *KB) Stats(ctx context.Context) ([]StatsEntry, error) {
	sql := `SELECT * FROM kb_stats()`
	rows, err := k.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("kb: stats: %w", err)
	}
	defer rows.Close()

	var results []StatsEntry
	for rows.Next() {
		var s StatsEntry
		if err := rows.Scan(&s.TableName, &s.RowCount); err != nil {
			return nil, fmt.Errorf("kb: scan stats: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// ResolveFileID builds a kb file ID from repo and relative path.
func ResolveFileID(repoID, relativePath string) string {
	return repoID + ":" + relativePath
}

// ParseFileID splits a kb file ID into repo ID and relative path.
func ParseFileID(fileID string) (repoID, relativePath string) {
	parts := strings.SplitN(fileID, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fileID
}

// SemanticResult represents a result from semantic search across any table.
type SemanticResult struct {
	SourceTable string  `json:"source_table"`
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Detail      string  `json:"detail"`
	Similarity  float64 `json:"similarity"`
}

// SearchAllSemantic performs cross-table semantic search using pgvector.
// The embedding vector must be 768-dimensional (nomic-embed-text).
func (k *KB) SearchAllSemantic(ctx context.Context, embedding string, limit int, minSimilarity float64) ([]SemanticResult, error) {
	if limit <= 0 {
		limit = 30
	}
	if minSimilarity <= 0 {
		minSimilarity = 0.5
	}
	rows, err := k.pool.Query(ctx,
		"SELECT source_table, id, title, detail, similarity FROM kb_search_all_semantic($1::vector, $2, $3)",
		embedding, limit, minSimilarity,
	)
	if err != nil {
		return nil, fmt.Errorf("kb: semantic search: %w", err)
	}
	defer rows.Close()

	var results []SemanticResult
	for rows.Next() {
		var r SemanticResult
		if err := rows.Scan(&r.SourceTable, &r.ID, &r.Title, &r.Detail, &r.Similarity); err != nil {
			return nil, fmt.Errorf("kb: scan semantic result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
