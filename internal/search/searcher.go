// Package search provides a composite search layer on top of an engram Store.
//
// The Searcher adds embedding-based similarity scoring and ranking to the
// hard-filtered results returned by a Store implementation.
package search

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrewhowdencom/engram/pkg/engram"
)

// Searcher wraps a Store and an optional Embedder to provide
// embedding-aware ranking.
type Searcher struct {
	store    engram.Store
	embedder engram.Embedder
}

// NewSearcher creates a Searcher backed by the given store.
// If embedder is nil, similarity queries fall back to token overlap.
func NewSearcher(store engram.Store, embedder engram.Embedder) *Searcher {
	return &Searcher{store: store, embedder: embedder}
}

// Query retrieves memories matching the query, ranked by composite relevance.
// It delegates hard filtering to the underlying Store, then scores and
// ranks the candidates in Go.
func (s *Searcher) Query(ctx context.Context, q engram.Query) ([]engram.Memory, error) {
	// Step 1: fetch candidates (hard-filtered, unranked) from the store.
	candidates, err := s.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store query: %w", err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Step 2: generate query embedding if similarity search is requested.
	var queryEmb []float32
	if q.Similarity != nil && q.Similarity.Text != "" && s.embedder != nil {
		queryEmb, err = s.embedder.Embed(ctx, q.Similarity.Text)
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
	}

	// Step 3: score and rank.
	var results []scoredMemory
	for _, mem := range candidates {
		score := engram.ScoreWithEmbedding(mem, q, q.Focus, queryEmb)
		if score > 0 {
			results = append(results, scoredMemory{mem: mem, score: score})
		}
	}

	order := "relevance"
	if q.Temporal != nil && q.Temporal.OrderBy != "" {
		order = q.Temporal.OrderBy
	}
	sort.Slice(results, func(i, j int) bool {
		switch order {
		case "recency":
			return results[i].mem.CreatedAt.After(results[j].mem.CreatedAt)
		case "created":
			return results[i].mem.CreatedAt.Before(results[j].mem.CreatedAt)
		default:
			return results[i].score > results[j].score
		}
	})

	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]engram.Memory, len(results))
	for i, sm := range results {
		out[i] = sm.mem
	}
	return out, nil
}

// Put stores a new memory, automatically generating an embedding if the
// Searcher has an Embedder and the memory does not already contain one.
func (s *Searcher) Put(ctx context.Context, m engram.Memory) (engram.Memory, error) {
	if s.embedder != nil && len(m.Embedding) == 0 && len(m.Content) > 0 {
		emb, err := s.embedder.Embed(ctx, string(m.Content))
		if err != nil {
			return m, fmt.Errorf("embed memory: %w", err)
		}
		m.Embedding = emb
	}
	return s.store.Put(ctx, m)
}

// Link delegates to the underlying store.
func (s *Searcher) Link(ctx context.Context, from, to, linkType string) error {
	return s.store.Link(ctx, from, to, linkType)
}

type scoredMemory struct {
	mem   engram.Memory
	score float64
}

// Compile-time interface check.
var _ engram.Store = (*Searcher)(nil)
