package search

import (
	"context"
	"testing"
	"time"

	"github.com/andrewhowdencom/engram/internal/store"
	"github.com/andrewhowdencom/engram/pkg/engram"
)

func TestSearcherRecencyOrder(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(dir+"/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	s := NewSearcher(st, nil)
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

func TestSearcherLimitsResults(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(dir + "/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	s := NewSearcher(st, nil)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_, _ = s.Put(ctx, engram.Memory{Content: []byte("item"), Context: map[string]string{"tag": "bulk"}})
	}

	results, err := s.Query(ctx, engram.Query{
		ContextFilter: &engram.ContextFilter{Pairs: map[string]string{"tag": "bulk"}},
		Limit:         5,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

// fakeEmbedder returns deterministic embeddings for testing.
type fakeEmbedder struct{}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	// Return a 3-dim embedding based on text content.
	emb := make([]float32, 3)
	for i, c := range text {
		emb[i%3] += float32(c)
	}
	// Normalise roughly.
	var sum float32
	for _, v := range emb {
		sum += v * v
	}
	if sum > 0 {
		inv := 1.0 / float32(sum)
		for i := range emb {
			emb[i] *= inv
		}
	}
	return emb, nil
}

func TestSearcherSimilarityWithEmbeddings(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(dir + "/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	em := &fakeEmbedder{}
	s := NewSearcher(st, em)
	ctx := context.Background()

	// Store two memories with different content.
	_, _ = s.Put(ctx, engram.Memory{Content: []byte("apple banana cherry")})
	_, _ = s.Put(ctx, engram.Memory{Content: []byte("dog elephant fox")})

	// Query for something closer to the first memory.
	results, err := s.Query(ctx, engram.Query{
		Similarity: &engram.SimilarityQuery{Text: "fruit apple"},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// With our fake embedder, "fruit apple" should score higher against
	// "apple banana cherry" because of shared characters.
	if string(results[0].Content) != "apple banana cherry" {
		t.Fatalf("expected first result to be 'apple banana cherry', got %q", results[0].Content)
	}
}

func TestSearcherAutoEmbedOnPut(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(dir + "/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	em := &fakeEmbedder{}
	s := NewSearcher(st, em)
	ctx := context.Background()

	m, err := s.Put(ctx, engram.Memory{Content: []byte("hello world")})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if len(m.Embedding) == 0 {
		t.Fatal("expected embedding to be auto-generated")
	}
	if len(m.Embedding) != 3 {
		t.Fatalf("expected 3-dim embedding, got %d", len(m.Embedding))
	}
}
