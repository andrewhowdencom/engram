# Integrate engram as a Go Library

> **Status**: How-to stub — content to be written when API stabilizes.

Import `github.com/andrewhowdencom/engram/pkg/engram` and use the `Store` interface to manage agent memory.

## Basic Setup

```go
import "github.com/andrewhowdencom/engram/pkg/engram"

// Initialize a store (FakeStore for prototyping)
store := engram.NewFakeStore()
```

## Store a Memory

```go
mem, err := store.Put(ctx, engram.Memory{
    Content: []byte("User prefers dark mode"),
    Context: map[string]string{"agent": "ui-bot", "topic": "preference"},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println("Stored:", mem.ID)
```

## Query with Focus

```go
focus := engram.Focus{Context: map[string]string{"agent": "ui-bot"}}

results, err := store.Query(ctx, engram.Query{
    Similarity: &engram.SimilarityQuery{Text: "dark mode settings"},
    Focus:      &focus,
    Limit:      5,
})
```

## Link Memories

```go
err := store.Link(ctx, "mem-123", "mem-456", "relates_to")
```

## See Also

- [API Reference](../../reference/api.md) — Complete type and interface documentation
- [What is engram?](../../explanation/what-is-engram.md) — Conceptual background
