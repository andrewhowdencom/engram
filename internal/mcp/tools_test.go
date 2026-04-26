package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewhowdencom/engram/internal/store"
	"github.com/andrewhowdencom/engram/pkg/engram"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMemoryStore(t *testing.T) {
	store := store.NewFakeStore()
	handler := MemoryStore(store)

	req := &sdkmcp.CallToolRequest{}
	input := MemoryStoreInput{
		Content: "User prefers dark mode",
		Context: map[string]string{
			"agent": "ui-bot",
			"topic": "preference",
		},
	}

	_, output, err := handler(context.Background(), req, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if output.CreatedAt == "" {
		t.Fatal("expected non-empty CreatedAt")
	}

	// Verify the memory was actually stored by querying.
	results, err := store.Query(context.Background(), engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"agent": "ui-bot"}},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	found := false
	for _, m := range results {
		if m.ID == output.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("stored memory not found in query results")
	}
}

func TestMemoryQuery(t *testing.T) {
	store := store.NewFakeStore()
	// Seed a memory directly.
	_, err := store.Put(context.Background(), engram.Memory{
		Content: []byte("SQLite WAL mode recommended for concurrency"),
		Context: map[string]string{"agent": "coder", "topic": "storage"},
	})
	if err != nil {
		t.Fatalf("seed error: %v", err)
	}

	handler := MemoryQuery(store)
	req := &sdkmcp.CallToolRequest{}

	// Query by context.
	_, output, err := handler(context.Background(), req, MemoryQueryInput{
		ContextFilter: map[string]string{"agent": "coder"},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Memories) == 0 {
		t.Fatal("expected at least one result")
	}

	// Query by similarity.
	_, output, err = handler(context.Background(), req, MemoryQueryInput{
		Similar: "database concurrency",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, m := range output.Memories {
		if strings.Contains(m.Content, "WAL") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected similarity query to find WAL memory")
	}

	// Query with focus.
	_, output, err = handler(context.Background(), req, MemoryQueryInput{
		Similar: "database concurrency",
		Focus:   map[string]string{"agent": "coder"},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Memories) == 0 {
		t.Fatal("expected focus query to return results")
	}
}

func TestMemoryQueryTemporal(t *testing.T) {
	store := store.NewFakeStore()
	// Seed an old memory.
	oldMem := engram.Memory{
		Content: []byte("Old memory"),
		Context: map[string]string{"agent": "tester"},
	}
	// Manually adjust timestamps after Put.
	_, err := store.Put(context.Background(), oldMem)
	if err != nil {
		t.Fatalf("seed error: %v", err)
	}
	// FakeStore does not expose memories map, so we rely on the default
	// seed data having a spread of ages. Just verify parsing works.

	handler := MemoryQuery(store)
	req := &sdkmcp.CallToolRequest{}

	_, _, err = handler(context.Background(), req, MemoryQueryInput{
		After:   "1d",
		ContextFilter: map[string]string{"agent": "tester"},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error with after filter: %v", err)
	}

	_, _, err = handler(context.Background(), req, MemoryQueryInput{
		Before:  "1h",
		ContextFilter: map[string]string{"agent": "tester"},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error with before filter: %v", err)
	}

	// Invalid duration should error.
	_, _, err = handler(context.Background(), req, MemoryQueryInput{
		After: "not-a-duration",
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestMemoryLink(t *testing.T) {
	store := store.NewFakeStore()
	// Seed two memories.
	m1, err := store.Put(context.Background(), engram.Memory{
		Content: []byte("First"),
		Context: map[string]string{"agent": "tester"},
	})
	if err != nil {
		t.Fatalf("seed error: %v", err)
	}
	m2, err := store.Put(context.Background(), engram.Memory{
		Content: []byte("Second"),
		Context: map[string]string{"agent": "tester"},
	})
	if err != nil {
		t.Fatalf("seed error: %v", err)
	}

	handler := MemoryLink(store)
	req := &sdkmcp.CallToolRequest{}

	_, output, err := handler(context.Background(), req, MemoryLinkInput{
		From: m1.ID,
		To:   m2.ID,
		Type: "depends_on",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Fatal("expected success")
	}

	// Verify the link exists by querying with relationship.
	results, err := store.Query(context.Background(), engram.Query{
		Relationship: &engram.RelationshipQuery{
			FromID: m1.ID,
			Depth:  1,
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	found := false
	for _, m := range results {
		if m.ID == m2.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected linked memory to be reachable")
	}
}

func TestMemoryLinkDefaultType(t *testing.T) {
	store := store.NewFakeStore()
	m1, _ := store.Put(context.Background(), engram.Memory{Content: []byte("A")})
	m2, _ := store.Put(context.Background(), engram.Memory{Content: []byte("B")})

	handler := MemoryLink(store)
	req := &sdkmcp.CallToolRequest{}

	_, output, err := handler(context.Background(), req, MemoryLinkInput{
		From: m1.ID,
		To:   m2.ID,
		// Type omitted — should default to "relates_to"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Fatal("expected success")
	}
}

func TestMemoryLinkMissingMemory(t *testing.T) {
	store := store.NewFakeStore()
	handler := MemoryLink(store)
	req := &sdkmcp.CallToolRequest{}

	_, _, err := handler(context.Background(), req, MemoryLinkInput{
		From: "non-existent-id",
		To:   "also-non-existent",
	})
	if err == nil {
		t.Fatal("expected error linking non-existent memories")
	}
}

func TestMemoryQueryEmptyResults(t *testing.T) {
	store := store.NewFakeStore()
	handler := MemoryQuery(store)
	req := &sdkmcp.CallToolRequest{}

	// Impossible temporal range: after 1h ago AND before 2h ago.
	_, output, err := handler(context.Background(), req, MemoryQueryInput{
		After:  "1h",
		Before: "2h",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Memories) != 0 {
		t.Fatalf("expected 0 results, got %d", len(output.Memories))
	}
}

func TestServerInitialization(t *testing.T) {
	store := store.NewFakeStore()
	server := NewServer(store)
	if server == nil {
		t.Fatal("expected non-nil server")
	}

	// The SDK does not expose a simple way to list registered tools,
	// but server creation itself exercises AddTool wiring.
	// We verify the server can be created and connected to an in-memory transport.
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Run(ctx, serverTransport)
	}()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect error: %v", err)
	}
	defer func() { _ = session.Close() }()

	// List tools to verify registration.
	toolsResult, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools error: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range []string{"memory_store", "memory_query", "memory_link"} {
		if !toolNames[name] {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}

	cancel()
	<-errChan
}
