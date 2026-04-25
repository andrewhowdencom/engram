# Design Principles

This document explains the reasoning behind engram's core design decisions.

## Unified over Typed

### Why one memory type?

Typed memory systems hard-code ontologies. A "bug report" is an event, a fact, and a goal simultaneously. Forcing the agent to choose one type at write time creates misclassification that breaks retrieval later.

The unified model says: store the memory once, let retrieval define its role in the current context.

### Trade-off: No type safety for "events vs facts"

If your application truly needs strict typing (e.g., compliance audit trails where events must never be modified), engram may not be the right fit. The unified model is optimized for flexible, associative retrieval over strict categorical boundaries.

## Agent-Managed Focus

### Why doesn't the store manage focus?

If the memory system stored focus as persistent state, it would:

- Second-guess the agent's attention lifecycle
- Create conflicts when multiple agents share a store
- Make debugging harder ("why did my focus change?")

By making focus agent-managed and per-query, the agent owns its operational stance. The memory system just computes scores.

### Trade-off: Agent must manage its own state

The agent (or its orchestrator) is responsible for updating focus on task switches, clearing it when idle, and implementing any decay or scope logic. This is more work for the agent builder but gives full control.

## Unidirected Links

### Why one-way declarations?

Bilateral link creation ("A relates_to B AND B relates_to A") requires coordination. In a system where memories are created by autonomous agents, this coordination may not exist.

Unidirected links say: "I declare this association from my perspective." The system traverses it symmetrically in queries.

### Trade-off: No directed graph semantics

You cannot express "A depends on B but B does not depend on A" in a meaningful way for queries. If your use case requires strict dependency DAGs, engram's link model is too permissive.

## Active Writes, Passive Reads

### Why separate the complexity?

Memory writes are where intelligence lives — extracting concepts, building hierarchies, cross-referencing. Memory reads must be simple and deterministic so agents can build reliable retrieval logic.

By making the write path open-ended and the read path stable, the memory system can evolve its internals without breaking consumers.

### Trade-off: Write-time enrichment is not guaranteed

An agent that stores a raw memory without enriching it will have a sparser, less connected memory graph. The system does not enforce enrichment — it provides primitives and lets the agent decide how to use them.

## Internal Embeddings

### Why does engram generate embeddings?

If callers provided embeddings directly, every agent would need to manage its own embedding model, dimensionality, and normalization. By owning embedding generation, engram ensures:

- Consistent vector space across all memories
- Swappable models without caller changes
- Indexing optimization (the system knows the model and can precompute)

### Trade-off: Model lock-in at the store level

You cannot mix memories from different embedding models in the same store. If you need to migrate models, you must re-embed all memories.

## See Also

- [What is engram?](what-is-engram.md) — The unified memory model
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — System design and component map
- [README.md](../../README.md) — Quick start and overview
