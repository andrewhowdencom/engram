# ARCHITECTURE

This document describes engram's system design, component map, data flows, and API stability contract. It is intended for developers and architects building against or extending the system.

## Component Map

```
┌─────────────────────────────────────────────────────────────┐
│                      engram Component Map                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────┐    ┌──────────────────┐              │
│  │   Go Library     │    │      CLI         │              │
│  │  (pkg/engram)    │    │   (cmd/engram)   │              │
│  │                  │    │                  │              │
│  │  Store interface │    │  query           │              │
│  │  Memory type     │    │  store           │              │
│  │  Query type      │    │  link            │              │
│  │  Focus type      │    │  focus (agent-   │              │
│  │  Link type       │    │      managed)    │              │
│  └────────┬─────────┘    └────────┬─────────┘              │
│           │                       │                        │
│           └───────────┬───────────┘                        │
│                       │                                     │
│              ┌────────┴─────────┐                            │
│              │   Store Interface  │                            │
│              │  Put() Query()    │                            │
│              │  Link()            │                            │
│              └────────┬─────────┘                            │
│                       │                                     │
│           ┌───────────┼───────────┐                        │
│           │           │           │                        │
│    ┌──────┴──────┐ ┌──┴──────┐ ┌──┴──────┐                 │
│    │  FakeStore  │ │ SQLite  │ │  Future │                 │
│    │ (prototype) │ │(planned)│ │ Backends│                 │
│    │             │ │         │ │         │                 │
│    │ In-memory   │ │sqlite-vec│ │PostgreSQL│                │
│    │ JSON persist│ │vectors  │ │ Redis   │                 │
│    │             │ │         │ │  etc.   │                 │
│    └─────────────┘ └─────────┘ └─────────┘                 │
│                                                              │
│  ┌──────────────────────────────────────────┐               │
│  │         Internal Scoring Layer            │               │
│  │  (internal, implementation not stable)    │               │
│  │                                           │               │
│  │  Score() — composite relevance            │               │
│  │  focusScore() — agent-managed warm-up     │               │
│  │  contextScore() — exact metadata match    │               │
│  │  similarityScore() — vector proximity   │               │
│  │  temporalScore() — recency decay          │               │
│  └──────────────────────────────────────────┘               │
│                                                              │
│  ┌──────────────────────────────────────────┐               │
│  │         Enrichment Layer (future)         │               │
│  │  (internal, not yet implemented)            │               │
│  │                                           │               │
│  │  Concept extraction                         │               │
│  │  Abstraction hierarchy linking              │               │
│  │  Cross-reference suggestions               │               │
│  └──────────────────────────────────────────┘               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## API Stability Contract

### Public API (Stable — Major Version Bump Required)

| Type/Interface | Location | Stability |
|---|---|---|
| `Memory` struct | `pkg/engram` | **Stable** |
| `Query` struct | `pkg/engram` | **Stable** |
| `Focus` struct | `pkg/engram` | **Stable** |
| `Link` struct | `pkg/engram` | **Stable** |
| `Store` interface | `pkg/engram` | **Stable** |
| `ContextFilter` | `pkg/engram` | **Stable** |
| `SimilarityQuery` | `pkg/engram` | **Stable** |
| `RelationshipQuery` | `pkg/engram` | **Stable** |
| `TemporalQuery` | `pkg/engram` | **Stable** |

These types define the contract that agents and applications build against. They encode the unified memory model (one type, four dimensions, agent-managed focus) and will not change incompatibly without a major version increment.

### Internal Implementation (Evolving — May Change Anytime)

| Component | Location | Stability |
|---|---|---|
| `Score()` and all scoring helpers | `pkg/engram` | **Internal** |
| `FakeStore` | `pkg/engram` | **Internal / Throwaway** |
| Token-based similarity approximation | `pkg/engram` | **Internal** |
| Recency decay formula | `pkg/engram` | **Internal** |
| Focus boost curve | `pkg/engram` | **Internal** |
| Embedding generation | (not yet impl) | **Internal** |
| Storage backend internals | (not yet impl) | **Internal** |

These are prototype-quality implementations that exist to validate the API shape. They will be replaced by real embedding models, persistent storage backends, and tunable scoring without API breakage.

### Future Interfaces (Not Yet Implemented)

| Interface | Status |
|---|---|
| MCP server (`memory_query`, `memory_store`, `memory_link`) | Planned |
| SQLite persistent backend | Planned |
| PostgreSQL backend | Future |
| Multi-agent context namespaces | Future |
| OpenTelemetry observability | Future |
| Configurable scoring weights | Future |

## Data Flow

### Write Path (Active)

```
Agent ──Put()──→ Store ──→ Storage Backend
                │
                └─→ (future) Enrichment Pipeline
                    ├── Concept extraction
                    ├── Abstraction hierarchy linking
                    └── Cross-reference suggestions
```

The write path is **active** — it may involve complex agentic processes. The `Store` interface provides the primitive (`Put`), but the agent or a background enrichment process decides how to decompose, link, and abstract memories. This is not prescribed by the memory system.

### Read Path (Passive)

```
Agent ──Query()──→ Store ──→ Scoring Layer ──→ Ranked Results
         │
         └─Focus (optional, agent-managed)
```

The read path is **passive and stable** — a `Query` with optional `Focus` always returns ranked `[]Memory`. The agent does not need to understand scoring internals. How the composite relevance is computed (weights, traversal depth, boost curves) is an internal implementation detail.

## Design Decisions

### Unified Memory Type

There is one `Memory` type, not `Event`, `Fact`, `Note`, `Goal`. The meaning of a memory emerges from how it is retrieved — through the four dimensions — not from its declared type. This avoids hard-coded ontologies that break down across agent domains.

### Agent-Managed Focus

Focus is passed per-query, not stored in the memory system. The agent maintains its own operational stance and passes it explicitly. This gives the agent full control over focus lifecycle (set, clear, decay, switch) and avoids cross-process conflicts.

### Unidirected Links

Links are declared one-way but traversed symmetrically. This matches associative cognition — "this reminds me of that" — without requiring bilateral agreement between memories.

### Active Writes, Passive Reads

The write path is open-ended and agentic. The read path is simple and stable. This separation means the memory system can evolve its internals (scoring, enrichment, storage) without breaking consumers.

### No AGENTS.md

engram itself replaces the need for static agent context files. Future agents working on this repository should query the project's own memory store for design decisions, conventions, and history rather than reading human-maintained documentation.

## Future Evolution Points

1. **Scoring weights** will become configurable (per-agent, per-query-type, per-context-key)
2. **Enrichment pipeline** will be pluggable — agents can provide custom enrichment strategies
3. **Storage backends** will implement the `Store` interface — SQLite first, then PostgreSQL, Redis, etc.
4. **Embedding models** will be swappable — local sentence-transformers, OpenAI, or custom
5. **MCP server** will expose the `Store` interface as Model Context Protocol tools
6. **Multi-agent namespaces** will allow shared and private memory scopes
7. **Agent-centric time** — Replace wall-clock timestamps with token- or operation-based temporal metrics. An agent suspended for a week experiences no passage; its memories should not age. Tracking "how many tokens since this memory" is more honest than calendar time, though we do not yet know how to instrument agents to report this.

None of these changes require breaking the public API contract.
