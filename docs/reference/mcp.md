# MCP Server Reference

engram exposes its memory operations as Model Context Protocol (MCP) tools, allowing any MCP-compatible agent to use engram as its memory backend.

## Running the Server

### Stdio (JSON-RPC)

The standard transport for local MCP clients such as Claude Desktop and Cursor:

```bash
engram mcp stdio
```

### HTTP (Streamable)

For remote or browser-based clients, using the modern streamable HTTP transport (MCP spec 2025-11-25):

```bash
engram mcp http --port 8080
```

Binds to `localhost` by default. Use `--port` / `-p` to change the port.

## Tools

### memory_store

Store a new memory in the agent's memory.

**Parameters:**
- `content` (string, required) — Memory content
- `context` (object, optional) — Key-value context metadata

**Returns:**
- `id` (string) — Assigned memory ID
- `created_at` (string) — ISO 8601 timestamp

### memory_query

Query memories across all four dimensions with optional focus.

**Parameters:**
- `context_filter` (object, optional) — Key-value context filters
- `similar` (string, optional) — Text for semantic similarity
- `similar_threshold` (float, optional) — Minimum similarity score
- `rel_from` (string, optional) — Relationship origin memory ID
- `rel_type` (string, optional) — Relationship type filter
- `rel_depth` (int, optional) — Traversal depth (default: 1)
- `after` (string, optional) — Temporal filter (e.g. "24h")
- `before` (string, optional) — Temporal filter
- `order` (string, optional) — Ordering: relevance, recency, created
- `limit` (int, optional) — Max results (default: 10)
- `focus` (object, optional) — Agent-managed focus context

**Returns:**
- `memories` (array) — Ranked memory results, each with `id`, `content`, `context`, `links`, `created_at`

### memory_link

Create a unidirected relationship between two memories.

**Parameters:**
- `from` (string, required) — Source memory ID
- `to` (string, required) — Target memory ID
- `type` (string, optional) — Relationship type (default: "relates_to")

**Returns:**
- `success` (boolean)

## Design Notes

- Focus is passed per-query as a parameter, not stored server-side. The MCP client (the agent) maintains its own focus state.
- All tools map directly to the `Store` interface methods.
- The stdio and HTTP transports share the same persistence path as the CLI (`~/.local/share/engram/fake-store.json` in the prototype).

See [ARCHITECTURE.md](../../ARCHITECTURE.md) for the system design and API stability contract.
