# What is engram?

engram is a unified memory system for agents.

## The Problem with Typed Memory

Most memory systems force you to choose a type for every memory:

- **Events** for things that happened
- **Facts** for things that are true
- **Notes** for observations
- **Goals** for things to achieve

This breaks down quickly. A coding agent's "note about a bug" is simultaneously an event (it happened), a fact (it describes system state), and a goal (it implies work to do). Typed systems force premature classification that often conflicts across contexts.

## The Unified Alternative

engram stores **one** memory type. The meaning of a memory emerges from how it is retrieved — through four complementary dimensions — not from its declared type.

## The Four Dimensions

Every memory in engram can be looked up through:

1. **Context** — metadata tags (agent, project, file, topic)
2. **Similarity** — vector-based semantic nearness
3. **Relationship** — graph links to other memories
4. **Time** — temporal markers and recency

A query can combine any subset of these. Results are ranked by composite relevance.

## Focus: The Agent's Operational Stance

Focus is the agent's current context — what it is doing, where it is, what it cares about right now. It is agent-managed and passed per-query. Focus multiplies the relevance of matching memories without hiding non-matching ones.

This allows an agent to maintain broad awareness while foregrounding what is most relevant *right now*.

## Active Memory

Memory in engram is not a passive database. The write path is an active, agentic process — memories may be decomposed, linked into abstraction hierarchies, enriched with cross-references, and connected to existing concepts. The read path remains simple and stable.

See [ARCHITECTURE.md](../ARCHITECTURE.md) for the full system design and API stability contract.
