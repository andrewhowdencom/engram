// Package store provides implementations of the engram.Store interface.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/andrewhowdencom/engram/pkg/engram"
)

// ---------------------------------------------------------------------------
// Fake implementation
// ---------------------------------------------------------------------------

// FakeStore is a query-capable in-memory store pre-loaded with rich sample data.
// Writes (Put, Link) are saved to a JSON file if persistPath is set.
type FakeStore struct {
	memories    map[string]engram.Memory
	persistPath string
}

// NewFakeStore creates a FakeStore loaded with sample memories.
// Data is ephemeral (lost on process exit) unless you also call
// fs.SetPersistPath and the store was not loaded from disk.
func NewFakeStore() *FakeStore {
	fs := &FakeStore{
		memories: make(map[string]engram.Memory),
	}
	fs.seed()
	return fs
}

// NewFakeStoreWithPath creates a FakeStore that attempts to load existing
// state from path. If the file does not exist it seeds sample data.
func NewFakeStoreWithPath(path string) (*FakeStore, error) {
	fs := &FakeStore{
		memories:    make(map[string]engram.Memory),
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
	m := func(id string, content string, ctx map[string]string, links []engram.Link, created time.Time) engram.Memory {
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
		return engram.Memory{
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
		[]engram.Link{},
		now.Add(-2*time.Hour))

	fs.memories["chat-2"] = m("chat-2",
		"User reported a bug in the checkout flow on mobile Safari. Need to investigate viewport issues.",
		map[string]string{"agent": "support-bot", "session": "sess-42", "topic": "bug"},
		[]engram.Link{{To: "chat-1", Type: "relates_to"}},
		now.Add(-90*time.Minute))

	fs.memories["chat-3"] = m("chat-3",
		"User wants to integrate Stripe payments. Asked about webhook security best practices.",
		map[string]string{"agent": "support-bot", "session": "sess-43", "topic": "payments"},
		[]engram.Link{},
		now.Add(-30*time.Minute))

	// --- Coding agent memories ---
	fs.memories["code-1"] = m("code-1",
		"The auth middleware uses JWT tokens with HS256. Secret is loaded from ENGRAM_JWT_SECRET env var.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/auth/jwt.go", "topic": "security"},
		[]engram.Link{},
		now.Add(-24*time.Hour))

	fs.memories["code-2"] = m("code-2",
		"SQLite is the default storage backend. Connection string can be overridden via config. WAL mode recommended for concurrency.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/store/sqlite.go", "topic": "storage"},
		[]engram.Link{{To: "code-1", Type: "depends_on"}},
		now.Add(-20*time.Hour))

	fs.memories["code-3"] = m("code-3",
		"Embedding model is configured in config.embedder. Supports local sentence-transformers and OpenAI text-embedding-3.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/embedder/config.go", "topic": "ml"},
		[]engram.Link{{To: "code-2", Type: "relates_to"}},
		now.Add(-18*time.Hour))

	fs.memories["code-4"] = m("code-4",
		"MCP server exposes a single memory_query tool. All four dimensions (context, similarity, relationship, time) are parameters.",
		map[string]string{"agent": "coder", "project": "engram", "file": "internal/mcp/server.go", "topic": "integration"},
		[]engram.Link{{To: "code-3", Type: "part_of"}},
		now.Add(-12*time.Hour))

	// --- Cross-project / global memories ---
	fs.memories["global-1"] = m("global-1",
		"Go 1.26 adds new iter package utilities. Worth reviewing for our query pipeline.",
		map[string]string{"agent": "coder", "project": "global", "topic": "golang"},
		[]engram.Link{},
		now.Add(-72*time.Hour))

	fs.memories["global-2"] = m("global-2",
		"Observability: every Store operation should emit an OpenTelemetry span. Include query dimensions as attributes.",
		map[string]string{"agent": "coder", "project": "global", "topic": "observability"},
		[]engram.Link{{To: "global-1", Type: "relates_to"}},
		now.Add(-48*time.Hour))

	fs.memories["global-3"] = m("global-3",
		"Focus acts as a warm-up context. When an agent switches tasks, updating Focus re-ranks memory relevance without changing the query.",
		map[string]string{"agent": "coder", "project": "engram", "topic": "design"},
		[]engram.Link{},
		now.Add(-6*time.Hour))

	fs.memories["global-4"] = m("global-4",
		"Performance note: vector search with 10k 768-dim embeddings in SQLite via sqlite-vec is sub-10ms on M3 Mac.",
		map[string]string{"agent": "coder", "project": "engram", "topic": "performance"},
		[]engram.Link{{To: "code-2", Type: "relates_to"}},
		now.Add(-4*time.Hour))
}

// Put stores a memory in the fake store and persists if a path is set.
func (fs *FakeStore) Put(_ context.Context, m engram.Memory) (engram.Memory, error) {
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
func (fs *FakeStore) Query(_ context.Context, q engram.Query) ([]engram.Memory, error) {
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
		score := engram.Score(mem, q, q.Focus)
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

	out := make([]engram.Memory, len(results))
	for i, sm := range results {
		out[i] = sm.mem
	}
	return out, nil
}

type scoredMemory struct {
	mem   engram.Memory
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
	m.Links = append(m.Links, engram.Link{To: to, Type: linkType})
	fs.memories[from] = m
	return fs.save()
}

// ---------------------------------------------------------------------------
// Persistence helpers (JSON file)
// ---------------------------------------------------------------------------

type fakeStoreSnapshot struct {
	Memories map[string]engram.Memory `json:"memories"`
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

var _ engram.Store = (*FakeStore)(nil)
