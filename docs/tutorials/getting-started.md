# Getting Started

> **Status**: Tutorial stub — content to be written when API stabilizes.

This tutorial walks through installing engram, storing your first memory, querying it, and understanding how focus affects retrieval.

## Prerequisites

- Go 1.26 or later
- Git

## Installation

```bash
git clone https://github.com/andrewhowdencom/engram
cd engram
go build ./cmd/engram
```

## Step 1: Explore Sample Memories

The prototype comes with pre-loaded sample memories. Query them to understand the four dimensions:

```bash
# Query by context
./engram query --context agent=coder --limit 5

# Query by similarity
./engram query --similar "how do I configure" --limit 3

# Query by relationship traversal
./engram query --rel-from code-1 --rel-depth 2
```

## Step 2: Store Your First Memory

```bash
./engram store --content "My first memory about engram" --context agent=me --context topic=learning
```

## Step 3: Query with Focus

```bash
# Without focus — all results ranked by relevance
./engram query --similar "my first"

# With focus — your memories float higher
./engram query --similar "my first" --focus agent=me
```

## Step 4: Link Memories

```bash
./engram link --from <your-memory-id> --to code-1 --type relates_to
```

## Step 5: Run the MCP Server

Expose engram to any MCP-compatible agent:

```bash
# Stdio transport (Claude Desktop, Cursor, etc.)
./engram mcp stdio

# HTTP streamable transport
./engram mcp http --port 8080
```

The server exposes three tools: `memory_store`, `memory_query`, and `memory_link`.

See the [MCP Server Reference](../reference/mcp.md) for full details.

## What's Next

- Read [What is engram?](../explanation/what-is-engram.md) for the conceptual background
- Read [Design Principles](../explanation/design-principles.md) for design trade-offs
- Check the [API Reference](../reference/api.md) for Go integration
