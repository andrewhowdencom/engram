// Package engram provides the core library for managing agent memory.
//
// API Stability: The types Memory, Query, Focus, Link, and the Store interface
// are the public API contract. They change only with major version bumps.
//
// Implementation Stability: Score() and all scoring helpers are
// prototype implementations. They will change as real embedding models,
// persistent storage, and tunable scoring are introduced without breaking the
// public API.
package engram

import (
	"context"
	"math"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Embedder interface
// ---------------------------------------------------------------------------

// Embedder converts text into a dense vector representation.
// Implementations are responsible for their own model lifecycle,
// tokenisation, and normalisation. engram stores the resulting
// vectors but does not prescribe their dimensionality or scale.
type Embedder interface {
	// Embed converts text into a dense vector.
	// The returned slice must be safe for the caller to retain.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// Memory is the unified memory type. Every memory has four dimensions:
// context, similarity (embedding), relationship (links), and temporal.
type Memory struct {
	ID         string
	Content    []byte
	Context    map[string]string
	Embedding  []float32
	Links      []Link
	CreatedAt  time.Time
	UpdatedAt  time.Time
	AccessedAt time.Time
}

// Link is a unidirected relationship between two memories.
type Link struct {
	To   string // Memory ID
	Type string // e.g. "relates_to", "part_of", "depends_on"
}

// Query defines how to retrieve memories across all four dimensions.
// Focus is optional agent-managed operational context that warms up
// retrieval for this specific query.
type Query struct {
	ContextFilter *ContextFilter
	Similarity    *SimilarityQuery
	Relationship  *RelationshipQuery
	Temporal      *TemporalQuery
	Focus         *Focus // agent-managed, not persisted by the store
	Limit         int
}

// ContextFilter matches key-value pairs in Memory.Context.
// All specified keys must match (AND semantics). Values support
// exact match only in this prototype.
type ContextFilter struct {
	Pairs map[string]string
}

// SimilarityQuery finds memories whose content is semantically
// similar to the provided text. In the real implementation this
// would use an embedding model; here we use a rough approximation.
type SimilarityQuery struct {
	Text      string
	Threshold float32 // 0.0 - 1.0
}

// RelationshipQuery traverses unidirected links from a starting memory.
type RelationshipQuery struct {
	FromID string
	Type   string // empty = any type
	Depth  int    // 1 = direct neighbours
}

// TemporalQuery restricts results by time.
type TemporalQuery struct {
	After   *time.Time
	Before  *time.Time
	OrderBy string // "recency", "relevance", "created"
}

// Focus is persistent operational context that warms up retrieval.
// It multiplies the relevance of memories whose Context overlaps
// with the Focus Context and whose Embeddings are near Focus.Embedding.
type Focus struct {
	Context   map[string]string
	Embedding []float32
}

// ---------------------------------------------------------------------------
// Store interface
// ---------------------------------------------------------------------------

// Store is the primary abstraction for agent memory.
type Store interface {
	// Put stores a new memory and returns it with its assigned ID.
	Put(ctx context.Context, m Memory) (Memory, error)

	// Query retrieves memories matching the query, ranked by composite relevance.
	// The caller may supply an optional Focus in the Query to warm up results.
	Query(ctx context.Context, q Query) ([]Memory, error)

	// Link creates a unidirected relationship between two memories.
	Link(ctx context.Context, from, to string, linkType string) error
}

// ---------------------------------------------------------------------------
// Scoring helpers (used by implementations)
// ---------------------------------------------------------------------------

// Score computes a composite relevance score for a memory against a query
// and optional agent-managed focus. Higher is better. The score is in the range [0, 1].
//
// Score uses the legacy token-overlap similarity. For real vector similarity,
// use ScoreWithEmbedding with a pre-computed query embedding.
func Score(m Memory, q Query, focus *Focus) float64 {
	return ScoreWithEmbedding(m, q, focus, nil)
}

// ScoreWithEmbedding computes composite relevance using real vector similarity
// when queryEmb is non-nil. If queryEmb is nil or the memory has no embedding,
// it falls back to the token-overlap approximation.
func ScoreWithEmbedding(m Memory, q Query, focus *Focus, queryEmb []float32) float64 {
	var score float64

	// Context match component (0 - 0.3)
	if q.ContextFilter != nil && len(q.ContextFilter.Pairs) > 0 {
		score += contextScore(m, q.ContextFilter) * 0.3
	}

	// Similarity component (0 - 0.3)
	if q.Similarity != nil {
		if len(queryEmb) > 0 && len(m.Embedding) > 0 {
			score += float64(CosineSimilarity(queryEmb, m.Embedding)) * 0.3
		} else {
			score += similarityScore(m, q.Similarity) * 0.3
		}
	}

	// Relationship proximity (0 - 0.2)
	if q.Relationship != nil {
		score += relationshipScore(m, q.Relationship) * 0.2
	}

	// Temporal relevance (0 - 0.2)
	if q.Temporal != nil {
		score += temporalScore(m, q.Temporal) * 0.2
	} else {
		// Default recency boost when no temporal filter given.
		score += recencyScore(m) * 0.2
	}

	// Focus multiplier: boost when focus context overlaps.
	focusBoost := focusScore(m, focus)
	if focusBoost > 0 {
		score = score * (1.0 + focusBoost)
		if score > 1.0 {
			score = 1.0
		}
	}

	return score
}

func contextScore(m Memory, f *ContextFilter) float64 {
	if len(f.Pairs) == 0 {
		return 1.0
	}
	matches := 0
	for k, v := range f.Pairs {
		if m.Context[k] == v {
			matches++
		}
	}
	return float64(matches) / float64(len(f.Pairs))
}

// similarityScore is a crude stand-in for real vector similarity.
// In the prototype we tokenise the query and memory content and count overlap.
func similarityScore(m Memory, sq *SimilarityQuery) float64 {
	queryTokens := tokenize(string(sq.Text))
	memTokens := tokenize(string(m.Content))
	if len(queryTokens) == 0 {
		return 0
	}
	matches := 0
	for _, qt := range queryTokens {
		for _, mt := range memTokens {
			if qt == mt {
				matches++
				break
			}
		}
	}
	ratio := float64(matches) / float64(len(queryTokens))
	if ratio < float64(sq.Threshold) {
		return 0
	}
	return ratio
}

func relationshipScore(m Memory, rq *RelationshipQuery) float64 {
	// In a full implementation this would inspect graph distance.
	// For the prototype we give a small boost if the memory is linked
	// from the origin and the type matches.
	if rq.FromID == "" {
		return 1.0 // no origin specified
	}
	// This is a simplified placeholder; real graph scoring would
	// be done during traversal, not per-memory.
	return 0.5
}

func temporalScore(m Memory, tq *TemporalQuery) float64 {
	score := 1.0
	if tq.After != nil && m.CreatedAt.Before(*tq.After) {
		return 0
	}
	if tq.Before != nil && m.CreatedAt.After(*tq.Before) {
		return 0
	}
	// Recency decay
	age := time.Since(m.CreatedAt).Hours()
	score *= math.Exp(-age / 168.0) // half-life of one week
	return score
}

func recencyScore(m Memory) float64 {
	age := time.Since(m.CreatedAt).Hours()
	return math.Exp(-age / 168.0)
}

func focusScore(m Memory, f *Focus) float64 {
	if f == nil || (len(f.Context) == 0 && len(f.Embedding) == 0) {
		return 0
	}
	var contextBoost float64
	if len(f.Context) > 0 {
		matches := 0
		for k, v := range f.Context {
			if m.Context[k] == v {
				matches++
			}
		}
		contextBoost = float64(matches) / float64(len(f.Context))
	}
	var embeddingBoost float64
	if len(f.Embedding) > 0 && len(m.Embedding) > 0 {
		embeddingBoost = float64(CosineSimilarity(f.Embedding, m.Embedding))
	}
	// Combine: max of the two, so either context overlap OR embedding
	// similarity can provide a boost.
	boost := contextBoost
	if embeddingBoost > boost {
		boost = embeddingBoost
	}
	return boost * 0.5 // focus adds up to +50% relevance
}

func tokenize(s string) []string {
	lower := strings.ToLower(s)
	// naive tokenisation
	lower = strings.ReplaceAll(lower, ".", " ")
	lower = strings.ReplaceAll(lower, ",", " ")
	lower = strings.ReplaceAll(lower, "?", " ")
	lower = strings.ReplaceAll(lower, "!", " ")
	fields := strings.Fields(lower)
	// deduplicate stop-words
	var out []string
	stop := map[string]bool{"the": true, "a": true, "an": true, "is": true, "to": true, "and": true, "of": true}
	for _, f := range fields {
		if !stop[f] {
			out = append(out, f)
		}
	}
	return out
}

// CosineSimilarity returns the cosine similarity of two vectors.
// Vectors must have the same length; otherwise the result is 0.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

var _ Store = (interface {
	Put(ctx context.Context, m Memory) (Memory, error)
	Query(ctx context.Context, q Query) ([]Memory, error)
	Link(ctx context.Context, from, to, linkType string) error
})(nil)
