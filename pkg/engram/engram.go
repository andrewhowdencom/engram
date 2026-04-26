// Package engram provides the core library for managing agent memory.
//
// API Stability: The types Memory, Query, Focus, Link, and the Store interface
// are the public API contract. They change only with major version bumps.
//
// Implementation Stability: Score(), FakeStore, and all scoring helpers are
// prototype implementations. They will change as real embedding models,
// persistent storage, and tunable scoring are introduced without breaking the
// public API.
package engram

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
func Score(m Memory, q Query, focus *Focus) float64 {
	var score float64

	// Context match component (0 - 0.3)
	if q.ContextFilter != nil && len(q.ContextFilter.Pairs) > 0 {
		score += contextScore(m, q.ContextFilter) * 0.3
	}

	// Similarity component (0 - 0.3)
	if q.Similarity != nil {
		score += similarityScore(m, q.Similarity) * 0.3
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
		embeddingBoost = float64(cosineSimilarity(f.Embedding, m.Embedding))
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

func cosineSimilarity(a, b []float32) float32 {
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

// ---------------------------------------------------------------------------
// Fake implementation
// ---------------------------------------------------------------------------

// FakeStore is a query-capable in-memory store pre-loaded with rich sample data.
// Writes (Put, Link) are saved to a JSON file if persistPath is set.
type FakeStore struct {
	memories    map[string]Memory
	persistPath string
}

// NewFakeStore creates a FakeStore loaded with sample memories.
// Data is ephemeral (lost on process exit) unless you also call
// fs.SetPersistPath and the store was not loaded from disk.
func NewFakeStore() *FakeStore {
	fs := &FakeStore{
		memories: make(map[string]Memory),
	}
	fs.seed()
	return fs
}

// NewFakeStoreWithPath creates a FakeStore that attempts to load existing
// state from path. If the file does not exist it seeds sample data.
func NewFakeStoreWithPath(path string) (*FakeStore, error) {
	fs := &FakeStore{
		memories:    make(map[string]Memory),
		persistPath: path,
	}
	if err := fs.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load fake store: %w", err)
		}
		// File does not exist — seed with sample data.
		fs.seed()
		if err := fs.save(); err != nil {
			return nil, fmt.Errorf("failed to save seeded fake store: %w", err)
		}
	}
	return fs, nil
}

// SetPersistPath enables JSON file persistence at the given path.
func (fs *FakeStore) SetPersistPath(path string) {
	fs.persistPath = path
}

func (fs *FakeStore) seed() {
	now := time.Now()
	// Helper to build a memory
	m := func(id string, content string, ctx map[string]string, links []Link, created time.Time) Memory {
		// Generate a fake 8-dim embedding based on content length & sum of bytes.
		// This gives deterministic but distinct vectors for each memory.
		emb := make([]float32, 8)
		var sum int
		for i := range content {
			sum += int(content[i])
		}
		for i := range emb {
			emb[i] = float32(float64(sum*(i+1)) / 100000.0)
		}
		// normalise roughly
		var norm float64
		for _, v := range emb {
			norm += float64(v * v)
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for i := range emb {
				emb[i] = float32(float64(emb[i]) / norm)
			}
		}
		return Memory{
			ID:         id,
			Content:    []byte(content),
			Context:    ctx,
			Embedding:  emb,
			Links:      links,
			CreatedAt:  created,
			UpdatedAt:  created,
			AccessedAt: created,
		}
	}

	// --- Chatbot memories ---
	fs.memories["chat-1"] = m("chat-1",
		"User asked how to parse JSON in Go. Suggested encoding/json.",
		map[string]string{"agent": "support-bot", "session": "sess-42", "topic": "golang"},
		[]Link{},
		now.Add(-2*time.Hour))

	fs.memories["chat-2"] = m("chat-2",
		"User reported a bug in the checkout flow on mobile Safari. Need to investigate viewport issues.",
		map[string]string{"agent": "support-bot", "session": "sess-42", "topic": "bug"},
		[]Link{{To: "chat-1", Type: "relates_to"}},
		now.Add(-90*time.Minute))

	fs.memories["chat-3"] = m("chat-3",
		"User wants to integrate Stripe payments. Asked about webhook security best practices.",
		map[string]string{"agent": "support-bot", "session": "sess-43", "topic": "payments"},
		[]Link{},
		now.Add(-30*time.Minute))

	// --- Coding agent memories ---
	fs.memories["code-1"] = m("code-1",
		"The auth middleware uses JWT tokens with HS256. Secret is loaded from ENGRAM_JWT_SECRET env var.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/auth/jwt.go", "topic": "security"},
		[]Link{},
		now.Add(-24*time.Hour))

	fs.memories["code-2"] = m("code-2",
		"SQLite is the default storage backend. Connection string can be overridden via config. WAL mode recommended for concurrency.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/store/sqlite.go", "topic": "storage"},
		[]Link{{To: "code-1", Type: "depends_on"}},
		now.Add(-20*time.Hour))

	fs.memories["code-3"] = m("code-3",
		"Embedding model is configured in config.embedder. Supports local sentence-transformers and OpenAI text-embedding-3.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/embedder/config.go", "topic": "ml"},
		[]Link{{To: "code-2", Type: "relates_to"}},
		now.Add(-18*time.Hour))

	fs.memories["code-4"] = m("code-4",
		"MCP server exposes a single memory_query tool. All four dimensions (context, similarity, relationship, time) are parameters.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/mcp/server.go", "topic": "integration"},
		[]Link{{To: "code-3", Type: "part_of"}},
		now.Add(-12*time.Hour))

	// --- Cross-project / global memories ---
	fs.memories["global-1"] = m("global-1",
		"Go 1.26 adds new iter package utilities. Worth reviewing for our query pipeline.",
		map[string]string{"agent": "coder", "project": "global", "topic": "golang"},
		[]Link{},
		now.Add(-72*time.Hour))

	fs.memories["global-2"] = m("global-2",
		"Observability: every Store operation should emit an OpenTelemetry span. Include query dimensions as attributes.",
		map[string]string{"agent": "coder", "project": "global", "topic": "observability"},
		[]Link{{To: "global-1", Type: "relates_to"}},
		now.Add(-48*time.Hour))

	fs.memories["global-3"] = m("global-3",
		"Focus acts as a warm-up context. When an agent switches tasks, updating Focus re-ranks memory relevance without changing the query.",
		map[string]string{"agent": "coder", "project": "engram", "topic": "design"},
		[]Link{},
		now.Add(-6*time.Hour))

	fs.memories["global-4"] = m("global-4",
		"Performance note: vector search with 10k 768-dim embeddings in SQLite via sqlite-vec is sub-10ms on M3 Mac.",
		map[string]string{"agent": "coder", "project": "engram", "topic": "performance"},
		[]Link{{To: "code-2", Type: "relates_to"}},
		now.Add(-4*time.Hour))
}

// Put stores a memory in the fake store and persists if a path is set.
func (fs *FakeStore) Put(_ context.Context, m Memory) (Memory, error) {
	if m.ID == "" {
		m.ID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = m.CreatedAt
	m.AccessedAt = m.CreatedAt
	fs.memories[m.ID] = m
	if err := fs.save(); err != nil {
		return m, fmt.Errorf("store failed to persist: %w", err)
	}
	return m, nil
}

// Query retrieves memories from the fake store, ranked by composite relevance.
func (fs *FakeStore) Query(_ context.Context, q Query) ([]Memory, error) {
	var results []scoredMemory
	for _, mem := range fs.memories {
		// Relationship filter: if a relationship query is set, we only
		// include memories reachable within Depth from FromID.
		if q.Relationship != nil && q.Relationship.FromID != "" {
			if !fs.reachable(mem.ID, q.Relationship.FromID, q.Relationship.Type, q.Relationship.Depth) {
				continue
			}
		}
		// Temporal hard filters
		if q.Temporal != nil {
			if q.Temporal.After != nil && mem.CreatedAt.Before(*q.Temporal.After) {
				continue
			}
			if q.Temporal.Before != nil && mem.CreatedAt.After(*q.Temporal.Before) {
				continue
			}
		}
		score := Score(mem, q, q.Focus)
		if score > 0 {
			results = append(results, scoredMemory{mem: mem, score: score})
		}
	}

	// Sort by score descending
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

	out := make([]Memory, len(results))
	for i, sm := range results {
		out[i] = sm.mem
	}
	return out, nil
}

type scoredMemory struct {
	mem   Memory
	score float64
}

func (fs *FakeStore) reachable(target, origin, linkType string, depth int) bool {
	if depth <= 0 {
		return false
	}
	visited := make(map[string]bool)
	queue := []string{origin}
	for d := 0; d < depth && len(queue) > 0; d++ {
		nextQueue := []string{}
		for _, id := range queue {
			if visited[id] {
				continue
			}
			visited[id] = true
			m, ok := fs.memories[id]
			if !ok {
				continue
			}
			for _, l := range m.Links {
				if linkType != "" && l.Type != linkType {
					continue
				}
				if l.To == target {
					return true
				}
				nextQueue = append(nextQueue, l.To)
			}
		}
		queue = nextQueue
	}
	return false
}

// Link creates a unidirected relationship between two memories.
func (fs *FakeStore) Link(_ context.Context, from, to, linkType string) error {
	m, ok := fs.memories[from]
	if !ok {
		return fmt.Errorf("memory %q not found", from)
	}
	m.Links = append(m.Links, Link{To: to, Type: linkType})
	fs.memories[from] = m
	return fs.save()
}

// ---------------------------------------------------------------------------
// Persistence helpers (JSON file)
// ---------------------------------------------------------------------------

type fakeStoreSnapshot struct {
	Memories map[string]Memory `json:"memories"`
}

func (fs *FakeStore) save() error {
	if fs.persistPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(fs.persistPath), 0o750); err != nil {
		return err
	}
	snap := fakeStoreSnapshot{
		Memories: fs.memories,
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fs.persistPath, b, 0o600)
}

func (fs *FakeStore) load() error {
	if fs.persistPath == "" {
		return os.ErrNotExist
	}
	b, err := os.ReadFile(fs.persistPath)
	if err != nil {
		return err
	}
	var snap fakeStoreSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return err
	}
	fs.memories = snap.Memories
	return nil
}

var _ Store = (*FakeStore)(nil)
