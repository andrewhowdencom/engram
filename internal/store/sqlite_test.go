package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andrewhowdencom/engram/pkg/engram"
)

func TestSQLiteStorePutAndQuery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Store a memory.
	m := engram.Memory{
		Content: []byte("SQLite WAL mode recommended for concurrency"),
		Context: map[string]string{"agent": "coder", "topic": "storage"},
	}
	stored, err := s.Put(ctx, m)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if stored.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// Query by context.
	results, err := s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"agent": "coder"}},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if string(results[0].Content) != string(m.Content) {
		t.Fatalf("content mismatch: %q", results[0].Content)
	}

	// Query with no matches.
	results, err = s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"agent": "ghost"}},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteStoreSimilarityQuery(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	_, err = s.Put(ctx, engram.Memory{
		Content: []byte("SQLite WAL mode recommended for concurrency"),
		Context: map[string]string{"agent": "coder", "topic": "storage"},
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	results, err := s.Query(ctx, engram.Query{
		Similarity: &engram.SimilarityQuery{Text: "database concurrency", Threshold: 0},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

func TestSQLiteStoreTemporalFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	_, err = s.Put(ctx, engram.Memory{
		Content: []byte("Old memory"),
		Context: map[string]string{"agent": "tester"},
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	after := time.Now().Add(-24 * time.Hour)
	results, err := s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"agent": "tester"}},
		Temporal:      &engram.TemporalQuery{After: &after},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	before := time.Now().Add(-time.Minute)
	results, err = s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"agent": "tester"}},
		Temporal:      &engram.TemporalQuery{Before: &before},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteStoreLinkAndRelationshipQuery(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	m1, err := s.Put(ctx, engram.Memory{Content: []byte("First"), Context: map[string]string{"agent": "tester"}})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	m2, err := s.Put(ctx, engram.Memory{Content: []byte("Second"), Context: map[string]string{"agent": "tester"}})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}

	if err := s.Link(ctx, m1.ID, m2.ID, "depends_on"); err != nil {
		t.Fatalf("link: %v", err)
	}

	results, err := s.Query(ctx, engram.Query{
		Relationship: &engram.RelationshipQuery{FromID: m1.ID, Type: "depends_on", Depth: 1},
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == m2.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected linked memory to be reachable")
	}
}

func TestSQLiteStoreLinkMissingMemory(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	err = s.Link(ctx, "non-existent", "also-non-existent", "relates_to")
	if err == nil {
		t.Fatal("expected error linking non-existent memories")
	}
}

func TestSQLiteStoreContextNeverNull(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Store with nil context.
	stored, err := s.Put(ctx, engram.Memory{Content: []byte("no context")})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	results, err := s.Query(ctx, engram.Query{Limit: 10})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	for _, r := range results {
		if r.ID == stored.ID {
			if r.Context == nil {
				t.Fatal("context should never be nil, expected empty map")
			}
			break
		}
	}
}

func TestSQLiteStorePersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	ctx := context.Background()

	// First store instance.
	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 1: %v", err)
	}
	stored, err := s1.Put(ctx, engram.Memory{Content: []byte("persist me"), Context: map[string]string{"key": "val"}})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	s1.Close()

	// Reopen the same file.
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 2: %v", err)
	}
	defer s2.Close()

	results, err := s2.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"key": "val"}},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 || results[0].ID != stored.ID {
		t.Fatalf("expected persisted memory %q, got %v", stored.ID, results)
	}
}

func TestSQLiteStoreReopenWithForeignKeys(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fk.db")

	ctx := context.Background()

	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 1: %v", err)
	}

	m1, _ := s1.Put(ctx, engram.Memory{Content: []byte("A")})
	m2, _ := s1.Put(ctx, engram.Memory{Content: []byte("B")})
	_ = s1.Link(ctx, m1.ID, m2.ID, "relates_to")
	s1.Close()

	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 2: %v", err)
	}
	defer s2.Close()

	results, err := s2.Query(ctx, engram.Query{
		Relationship: &engram.RelationshipQuery{FromID: m1.ID, Depth: 1},
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == m2.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected link to survive reopen")
	}
}

func TestSQLiteStoreRecreatesSchemaOnExistingDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "schema.db")

	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 1: %v", err)
	}
	s1.Close()

	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store 2: %v", err)
	}
	defer s2.Close()

	ctx := context.Background()
	_, err = s2.Put(ctx, engram.Memory{Content: []byte("schema ok")})
	if err != nil {
		t.Fatalf("put after reopen: %v", err)
	}
}

func TestSQLiteStoreOrderByRecency(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "recency.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	m1, _ := s.Put(ctx, engram.Memory{Content: []byte("older"), Context: map[string]string{"tag": "x"}})
	time.Sleep(50 * time.Millisecond)
	m2, _ := s.Put(ctx, engram.Memory{Content: []byte("newer"), Context: map[string]string{"tag": "x"}})

	results, err := s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"tag": "x"}},
		Temporal:      &engram.TemporalQuery{OrderBy: "recency"},
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != m2.ID {
		t.Fatalf("expected newest first, got %s before %s", results[0].ID, m2.ID)
	}
	if results[1].ID != m1.ID {
		t.Fatalf("expected older second, got %s before %s", results[1].ID, m1.ID)
	}
}

func TestSQLiteStoreEmptyContextIsEmptyObject(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(filepath.Join(dir, "ctx.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	stored, err := s.Put(ctx, engram.Memory{Content: []byte("no ctx")})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	results, err := s.Query(ctx, engram.Query{Limit: 10})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for _, r := range results {
		if r.ID == stored.ID {
			if r.Context == nil || len(r.Context) != 0 {
				t.Fatalf("expected empty non-nil context, got %v", r.Context)
			}
			return
		}
	}
	t.Fatal("stored memory not found")
}

func TestSQLiteStoreDataDirCreated(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "deep", "path")
	dbPath := filepath.Join(sub, "engram.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("expected database file to be created")
	}
}
