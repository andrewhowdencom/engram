# CLI Reference

## Commands

### query

Retrieve memories across all four dimensions, ranked by composite relevance.

```bash
engram query [flags]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--context` | stringArray | — | Context key=value filter (repeatable) |
| `--similar` | string | — | Text for semantic similarity search |
| `--similar-threshold` | float32 | 0.0 | Minimum similarity score (0-1) |
| `--rel-from` | string | — | Relationship origin memory ID |
| `--rel-type` | string | — | Relationship type filter |
| `--rel-depth` | int | 1 | Traversal depth |
| `--after` | string | — | Only memories newer than duration (e.g. 24h, 7d) |
| `--before` | string | — | Only memories older than duration |
| `--order` | string | relevance | Result ordering: relevance, recency, created |
| `--limit` | int | 10 | Maximum results |
| `--focus` | stringArray | — | Focus context key=value (repeatable, agent-managed) |

### store

Store a new memory.

```bash
engram store [flags]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--content` | string | — | Memory content (required) |
| `--context` | stringArray | — | Context key=value (repeatable) |

### link

Create a unidirected relationship between two memories.

```bash
engram link [flags]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--from` | string | — | Source memory ID (required) |
| `--to` | string | — | Target memory ID (required) |
| `--type` | string | relates_to | Relationship type |

### version

Print version information.

```bash
engram version
```

## Global Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--log-level` | string | info | Log level: debug, info, warn, error |

## Examples

```bash
# Query by context
engram query --context agent=coder --context project=engram --limit 5

# Query by similarity with focus
engram query --similar "how do I configure embeddings" --focus agent=coder --limit 3

# Store a memory
engram store --content "SQLite WAL mode recommended" --context agent=coder --context topic=storage

# Link memories
engram link --from code-1 --to code-2 --type depends_on
```
