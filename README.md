# engram

engram is a unified memory system for agents.

It is designed to be consumed as a **Go library**, a **CLI tool**, or an **MCP server** — allowing agents to remember, relate, and retrieve information across sessions, tasks, and contexts.

## License

AGPL-3.0

## What is engram?

Most memory systems for agents force you to choose:

- **Key-value stores** give fast lookups but no narrative or semantic context.
- **Vector databases** give similarity search but no structure or relationships.
- **Graph databases** give rich relationships but no temporal or contextual ordering.
- **Event streams** give chronological replay but no random access or relevance ranking.

engram rejects this forced choice. It provides a **single, unified memory model** where every memory can be looked up through **four complementary dimensions** simultaneously:

1. **Context** — semantic metadata (agent, project, file, tags)
2. **Similarity** — vector-based semantic nearness
3. **Relationship** — graph traversal via unidirected links
4. **Time** — temporal ordering, recency, and decay

A query can mix any or all of these. Results are ranked by a **composite relevance score** that weights each dimension according to the query's intent.

## The Memory Model

There is **one** `Memory` type. Not `Event`, `Fact`, `Note`, `Goal` — just `Memory`.

Every memory has the same four dimensions:

```
Memory
├── id
├── content           // The payload (text, JSON, blob — opaque to the store)
├── context           // Semantic metadata: {agent: "X", file: "Y", project: "Z"}
├── embedding         // Vector for similarity search (generated internally)
├── relationships     // Unidirected links to other memories
└── temporal          // created_at, updated_at, accessed_at
```

This unified model is intentional. The *meaning* of a memory is not encoded in its type but in how it is **retrieved** — through the four dimensions and the operational **focus**.

### Why Unified?

Typed memory systems ("this is a Fact, this is an Event") hard-code ontologies that break down when agents operate across domains. A coding agent's "note about a bug" is simultaneously an event (it happened), a fact (it describes system state), and a goal (it implies work to do). engram stores it once and lets retrieval define its role in the current context.

## The Four Dimensions of Lookup

### 1. Context

Context is key-value metadata attached to every memory. It answers: *"Whose memory is this? What file? What project? What topic?"*

Examples:
- `{agent: "support-bot", session: "sess-42", topic: "payments"}`
- `{agent: "coder", project: "engram", file: "internal/store/sqlite.go"}`

Queries filter by exact key-value matches (AND semantics). Context provides **fast, deterministic narrowing** before similarity or graph traversal kicks in.

### 2. Similarity

Similarity is vector-based semantic search. engram generates embeddings internally (configurable model) and indexes memories for nearest-neighbour lookup.

This answers: *"What memories are about the same *kind* of thing?"* — even when the exact words differ.

Example: querying for *"how do I persist data?"* retrieves memories about SQLite, WAL mode, and config overrides without exact keyword matches.

### 3. Relationship (Links)

Relationships are **unidirected** links between memories. Each link has a type (`relates_to`, `part_of`, `depends_on`, etc.) and a target memory ID.

Unidirectionality is intentional: links are traversed symmetrically in queries (if A links to B, querying from A finds B and vice versa), but the *declaration* is one-way. This matches how agents naturally form associations — *"this reminds me of that"* — without requiring bilateral agreement.

Relationship queries answer: *"What is connected to this memory?"* — with configurable traversal depth.

Example: from a memory about "auth middleware", depth-2 traversal finds the storage backend it depends on, and the config it relates to.

### 4. Time

Every memory carries temporal markers. Queries can:

- Restrict to a time range (`after: "24h ago"`, `before: "7d ago"`)
- Order by recency, creation time, or relevance
- Apply recency decay to ranking (memories fade unless reinforced)

Time answers: *"What happened recently?"* and provides temporal narrative when other dimensions are underspecified.

> **Open question: Agent-centric time.** Wall-clock time is independent of agents — an agent suspended for a week experiences no passage, yet its memories age. A more agent-centric model would track time as a linear progression of tokens or operations: the more tokens that have passed since a memory was stored, the less relevant it becomes. We do not yet know how to get agents to track this, but it is a more honest temporal model than calendar time.

## Primitives

### Focus — The Agent's Operational Stance

Focus is engram's most distinctive primitive. It represents the **agent's current operational stance** — what it is doing, where it is, what it cares about *right now* — and multiplies the relevance of matching memories without changing the query itself.

Focus is **agent-managed**, not store-managed. The agent maintains its own focus state and passes it explicitly with each `Query`. This design is intentional:

- **Separation of concerns**: The memory system is stateless with respect to focus. It computes scores; the agent controls attention.
- **Agent owns lifecycle**: The agent decides when focus changes (file switch, task shift, explicit clear). It can implement decay, auto-clear on idle, or per-conversation scopes without the memory system second-guessing it.
- **No cross-process conflicts**: Multiple agents or processes using the same store cannot clobber each other's focus.

#### Focus vs. Context Filter

Passing context filters in a `Query` (`--context agent=coder`) is **exclusive** — it removes non-matching memories from results. Focus is **reordering** — it keeps all memories visible but floats matching ones higher. An agent debugging a memory issue may want to *see* chatbot memories (broad awareness) while having its own coding memories surface first.

Focus contains:
- **Context overlap**: memories sharing Focus keys/values get a relevance boost (up to +50%)
- **Embedding proximity**: memories near the Focus embedding (representing the current task/goal) get a boost

The boost is a **multiplier**, not a filter.

#### Example Workflow

```go
// Agent maintains its own focus state
focus := engram.Focus{Context: map[string]string{"agent": "coder", "project": "engram"}}

// Issue 50 varied queries; focus warms each one without repetition
results, _ := store.Query(ctx, engram.Query{
    Similarity: &engram.SimilarityQuery{Text: "how do I configure"},
    Focus:      &focus,
})

// Agent switches to debugging — updates its own focus
focus = engram.Focus{Context: map[string]string{"agent": "support-bot"}}

// Same query text, different focus → different ranking
results, _ = store.Query(ctx, engram.Query{
    Similarity: &engram.SimilarityQuery{Text: "how do I configure"},
    Focus:      &focus,
})
```

Or via CLI:

```bash
# Query with focus — agent manages what focus means and when to apply it
engram query --similar "how do I configure" --focus agent=coder --limit 3
engram query --similar "how do I configure" --focus agent=support-bot --limit 3
```

### Query — Composite Retrieval

A Query combines any subset of the four dimensions:

```go
type Query struct {
    ContextFilter  *ContextFilter   // Exact metadata matches
    Similarity     *SimilarityQuery // Vector search
    Relationship   *RelationshipQuery // Graph traversal
    Temporal       *TemporalQuery   // Time range / ordering
    Limit          int              // Max results
}
```

Results are scored across all active dimensions and ranked by composite relevance. A query with only `ContextFilter` behaves like a database query. A query with only `Similarity` behaves like semantic search. A query with all four is **contextually grounded, semantically relevant, structurally connected, and temporally aware**.

### Link — Association

`Link(from, to, type)` creates a unidirected relationship. Links are the graph dimension: they allow agents to build associative webs of knowledge rather than isolated facts.

Links are particularly powerful when combined with Focus. A memory linked from the current focus context inherits relevance through graph proximity.

## Interfaces

engram is designed for **three modes of consumption**:

### 1. Go Library (`pkg/engram`)

Import and embed directly in agent applications:

```go
store := engram.NewStore(cfg)
mem, _ := store.Put(ctx, engram.Memory{
    Content: []byte("User prefers dark mode"),
    Context: map[string]string{"agent": "ui-bot", "topic": "preference"},
})

focus := engram.Focus{Context: map[string]string{"agent": "ui-bot"}}
results, _ := store.Query(ctx, engram.Query{
    Similarity: &engram.SimilarityQuery{Text: "dark mode settings"},
    Focus:      &focus,
    Limit:      5,
})
```

### 2. CLI

Command-line tool for direct memory operations, debugging, and scripting:

```bash
engram store --content "SQLite WAL mode recommended" --context agent=coder --context topic=storage
engram query --similar "database performance" --limit 3
engram query --similar "database performance" --focus agent=coder --limit 3
engram link --from code-1 --to code-2 --type depends_on
```

### 3. MCP Server

Model Context Protocol server exposing engram as tools to LLM agents:

- `memory_store` — store a memory
- `memory_query` — query across all four dimensions (including optional focus)
- `memory_link` — create relationships

This allows any MCP-compatible agent (Claude, Cursor, etc.) to use engram as its memory backend without code changes.

## Design Principles

1. **Unified over typed**. One memory type, flexible retrieval. Meaning emerges from context, not ontology.

2. **Retrieval by composition**. Any combination of the four dimensions. No single dimension is privileged.

3. **Focus as agent-managed warm-up**. The memory system is stateless with respect to focus. The agent maintains its own operational stance and passes it per-query. The agent — not the store — owns focus lifecycle (set, clear, decay, switch).

4. **Unidirected links**. Graph traversal is symmetric; link creation is unilateral. Matches associative cognition.

5. **Internal embeddings**. engram owns embedding generation. Callers provide text; the system handles vectors, indexing, and model config.

6. **Pluggable storage**. SQLite by default. PostgreSQL, Redis, or custom backends via interface.

7. **Active writes, passive reads**. Memory writes are a complex, agentic process that may involve enrichment, decomposition, linking, and abstraction hierarchy construction. Memory reads are simple and stable — a `Query` across four dimensions. The memory system provides primitives (`Put`, `Link`) but does not prescribe how writes are enriched. Search behavior (scoring weights, traversal depth, boost curves) is internal and evolving — not part of the public API contract.

## Current State

engram is in early design and prototype phase.

- **Core types and interfaces** are defined (`pkg/engram`)
- **FakeStore** provides a query-capable in-memory implementation with rich sample data for API exploration
- **CLI commands** exist: `query`, `store`, `link`, `version`
- **Focus** is agent-managed, passed per-query (no store-level focus state)
- **MCP server** is implemented with stdio and HTTP/SSE transports
- **Persistent storage** (SQLite) is the next major milestone

The FakeStore is intentionally simple — it exists to let us **feel** the API, iterate on scoring weights, and validate that the unified model works for real use cases before committing to storage and indexing infrastructure.

## What's Next

### Stable Primitives (Ready to Build Against)

These types and interfaces are the public API contract. They will not change without a major version bump:

- `Memory`, `Query`, `Focus`, `Link` types
- `Store` interface (`Put`, `Query`, `Link`)
- Four-dimensional retrieval model

### Evolving Internals (Will Change)

These are prototype implementations that will be replaced or reimplemented:

- `Score()` — weights and boost curves are placeholders
- `FakeStore` — throwaway in-memory prototype
- Token-based similarity — replaced by real embedding model
- Recency decay formula — tunable in future releases

### Future Interfaces (Not Yet Implemented)

- Persistent SQLite backend with vector search (sqlite-vec)
- Real embedding model integration (local sentence-transformers, OpenAI)
- Observability (OpenTelemetry spans per Store operation)
- Multi-agent shared memory (cross-agent context namespaces)

## Getting Started (Prototype)

```bash
# Clone and build
git clone https://github.com/andrewhowdencom/engram
cd engram
go build ./cmd/engram

# Run with sample data
./engram query --context agent=coder --limit 5
./engram query --similar "embeddings" --limit 3
./engram query --similar "how do I configure" --focus agent=coder --limit 3
./engram query --similar "how do I configure" --focus agent=support-bot --limit 3

# Run MCP server (stdio for Claude Desktop, Cursor, etc.)
./engram mcp stdio

# Run MCP server over HTTP/SSE
./engram mcp http --port 8080
```

The prototype persists stored memories and links to `~/.local/share/engram/fake-store.json` between runs. Focus is agent-managed and not persisted by the store.

---

*engram is named after the neuropsychological term for a memory trace — the physical or chemical change in the brain that encodes a memory. The system aims to be the computational equivalent: a persistent, retrievable trace of an agent's experience.*
