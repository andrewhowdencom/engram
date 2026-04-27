// Package onnx provides a local ONNX Runtime embedder for sentence-transformer models.
package onnx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/andrewhowdencom/engram/pkg/engram"
)

// Embedder runs a local ONNX model to produce text embeddings.
type Embedder struct {
	modelPath string
	vocabPath string

	session     any // *ort.AdvancedSession — stored as any to avoid import in header
	tokenizer   *tokenizer
	maxLen      int
	hiddenSize  int
	modelLoaded bool
	mu          sync.Mutex
}

// Option configures an Embedder.
type Option func(*Embedder)

// WithMaxLen sets the maximum sequence length (default 128).
func WithMaxLen(n int) Option {
	return func(e *Embedder) { e.maxLen = n }
}

// WithHiddenSize sets the hidden dimension (default 384 for all-MiniLM-L6-v2).
func WithHiddenSize(n int) Option {
	return func(e *Embedder) { e.hiddenSize = n }
}

// NewEmbedder creates an ONNX embedder using the given model and vocab files.
// Both paths must exist. Use DownloadModel to fetch them first.
func NewEmbedder(modelPath, vocabPath string, opts ...Option) (*Embedder, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not found at %q: %w", modelPath, err)
	}
	if _, err := os.Stat(vocabPath); err != nil {
		return nil, fmt.Errorf("vocab file not found at %q: %w", vocabPath, err)
	}

	tok, err := newTokenizer(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	e := &Embedder{
		modelPath: modelPath,
		vocabPath: vocabPath,
		tokenizer: tok,
		maxLen:    128,
		hiddenSize: 384,
	}
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

// Embed converts text into a dense vector via the ONNX model.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.modelLoaded {
		if err := e.loadModel(); err != nil {
			return nil, fmt.Errorf("load onnx model: %w", err)
		}
	}

	return e.embed(ctx, text)
}

// Ensure Embedder implements the interface.
var _ engram.Embedder = (*Embedder)(nil)

// DefaultModelDir returns the default directory for downloaded models.
func DefaultModelDir() string {
	// Prefer XDG_DATA_HOME, fall back to ~/.local/share/engram/models
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(dir, "engram", "models")
}
