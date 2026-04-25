# API Reference

## Core Types

### Memory

The unified memory type. One type for all memories — events, facts, notes, goals.

```go
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
```

- `Content` — opaque payload (text, JSON, binary blob)
- `Context` — semantic metadata for deterministic filtering
- `Embedding` — vector for similarity search (generated internally by the store)
- `Links` — unidirected relationships to other memories
- `CreatedAt`, `UpdatedAt`, `AccessedAt` — temporal markers

### Query

Retrieves memories across any combination of four dimensions. All fields are optional; omitting a dimension means "do not filter on this dimension."

```go
type Query struct {
    ContextFilter *ContextFilter
    Similarity    *SimilarityQuery
    Relationship  *RelationshipQuery
    Temporal      *TemporalQuery
    Focus         *Focus
    Limit         int
}
```

### Focus

Agent-managed operational context that warms up retrieval. Passed per-query, not stored in the memory system.

```go
type Focus struct {
    Context   map[string]string
    Embedding []float32
}
```

- `Context` — key-value overlap with memory context provides a relevance boost
- `Embedding` — vector proximity to memory embeddings provides a relevance boost

The boost is a multiplier (up to +50%), not a filter. Non-matching memories remain visible but are deprioritised.

### Link

Unidirected relationship between two memories.

```go
type Link struct {
    To   string // Target memory ID
    Type string // e.g. "relates_to", "part_of", "depends_on"
}
```

Declared one-way, traversed symmetrically in relationship queries.

## Store Interface

```go
type Store interface {
    Put(ctx context.Context, m Memory) (Memory, error)
    Query(ctx context.Context, q Query) ([]Memory, error)
    Link(ctx context.Context, from, to string, linkType string) error
}
```

### Put

Stores a new memory. If `m.ID` is empty, the store assigns one. Returns the stored memory with its assigned ID and timestamps.

### Query

Retrieves memories matching the query constraints, ranked by composite relevance across all active dimensions. An optional `Focus` in the query warms up results toward the agent's current operational stance.

### Link

Creates a unidirected relationship from memory `from` to memory `to` with the given link type. The relationship is traversed symmetrically in queries.

## Sub-Query Types

### ContextFilter

Exact key-value matching on `Memory.Context`. All specified pairs must match (AND semantics).

```go
type ContextFilter struct {
    Pairs map[string]string
}
```

### SimilarityQuery

Vector-based semantic search. The store generates an embedding from the query text and finds nearest neighbours.

```go
type SimilarityQuery struct {
    Text      string
    Threshold float32 // 0.0 - 1.0
}
```

### RelationshipQuery

Graph traversal from a starting memory.

```go
type RelationshipQuery struct {
    FromID string
    Type   string // empty = any type
    Depth  int    // 1 = direct neighbours
}
```

### TemporalQuery

Time range restriction and ordering.

```go
type TemporalQuery struct {
    After   *time.Time
    Before  *time.Time
    OrderBy string // "recency", "relevance", "created"
}
```

## Stability

The types and interfaces documented above are the **public API contract**. They change only with major version bumps.

Scoring implementations (`Score()`, `FakeStore`) are internal prototypes and will evolve without API breakage.

See [ARCHITECTURE.md](../../ARCHITECTURE.md) for the full stability contract.
