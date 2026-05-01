package main

import (
	"context"

	"github.com/vibepilot/governor/internal/kb"
	"github.com/vibepilot/governor/internal/runtime"
)

// kbAdapter wraps internal/kb.KB to satisfy runtime.KBProvider.
// Converts between the two packages' identical-but-distinct types.
type kbAdapter struct {
	inner *kb.KB
}

// convertSymbol converts a kb.Symbol to a runtime.KBSymbol.
func convertSymbol(s kb.Symbol) runtime.KBSymbol {
	return runtime.KBSymbol{
		ID:            s.ID,
		QualifiedName: s.QualifiedName,
		Kind:          s.Kind,
		Name:          s.Name,
		Summary:       s.Summary,
		FileID:        s.FileID,
		LineStart:     s.LineStart,
		LineEnd:       s.LineEnd,
	}
}

func (a *kbAdapter) SearchSymbols(ctx context.Context, query string, filterKind, filterRepo *string, limit int) ([]runtime.KBSymbol, error) {
	symbols, err := a.inner.SearchSymbols(ctx, query, filterKind, filterRepo, limit)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.KBSymbol, len(symbols))
	for i, s := range symbols {
		out[i] = convertSymbol(s)
	}
	return out, nil
}

func (a *kbAdapter) GetFileSymbols(ctx context.Context, fileID string, limit int) ([]runtime.KBSymbol, error) {
	symbols, err := a.inner.GetFileSymbols(ctx, fileID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.KBSymbol, len(symbols))
	for i, s := range symbols {
		out[i] = convertSymbol(s)
	}
	return out, nil
}

func (a *kbAdapter) SearchDocs(ctx context.Context, query string, filterRepo *string, limit int) ([]runtime.KBDocSection, error) {
	docs, err := a.inner.SearchDocs(ctx, query, filterRepo, limit)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.KBDocSection, len(docs))
	for i, d := range docs {
		out[i] = runtime.KBDocSection{
			ID:      d.ID,
			Title:   d.Title,
			DocPath: d.DocPath,
			Level:   d.Level,
			Summary: d.Summary,
			RepoID:  d.RepoID,
		}
	}
	return out, nil
}

func (a *kbAdapter) SearchKnowledge(ctx context.Context, query string, filterType *string, limit int) ([]runtime.KBKnowledgeItem, error) {
	items, err := a.inner.SearchKnowledge(ctx, query, filterType, limit)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.KBKnowledgeItem, len(items))
	for i, k := range items {
		out[i] = runtime.KBKnowledgeItem{
			ID:       k.ID,
			ItemType: k.ItemType,
			Name:     k.Name,
			Title:    k.Title,
			Summary:  k.Summary,
			Priority: k.Priority,
			Status:   k.Status,
		}
	}
	return out, nil
}

func (a *kbAdapter) Stats(ctx context.Context) ([]runtime.KBStatsEntry, error) {
	stats, err := a.inner.Stats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.KBStatsEntry, len(stats))
	for i, s := range stats {
		out[i] = runtime.KBStatsEntry{
			TableName: s.TableName,
			RowCount:  s.RowCount,
		}
	}
	return out, nil
}
