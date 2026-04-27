package store

import (
	"github.com/andrewhowdencom/engram/pkg/engram"
)

// Option configures a Store implementation.
type Option func(*storeConfig)

type storeConfig struct {
	embedder engram.Embedder
}

// WithEmbedder sets the embedder used to auto-generate embeddings on Put.
func WithEmbedder(e engram.Embedder) Option {
	return func(c *storeConfig) {
		c.embedder = e
	}
}
