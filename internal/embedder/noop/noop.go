// Package noop provides a no-op embedder that always returns nil.
// It is used as the default fallback when no embedding backend is configured.
package noop

import "context"

// Embedder is a no-op embedder that returns nil for every input.
type Embedder struct{}

// New returns a new no-op embedder.
func New() *Embedder { return &Embedder{} }

// Embed returns nil, nil — no vector is produced.
func (e *Embedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}
