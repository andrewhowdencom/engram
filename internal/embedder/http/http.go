// Package http provides a generic HTTP-based embedder that can target
// OpenAI, Gemini, Ollama, or any compatible text-embedding endpoint.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Embedder sends text to a remote HTTP endpoint and extracts embedding vectors
// from the JSON response.
type Embedder struct {
	client       *http.Client
	endpoint     string
	headers      map[string]string
	inputField   string   // JSON path to the input text in the request body
	outputPath   []string // nested JSON keys leading to the embedding array
	model        string   // model identifier (optional, included in request body)
	dimensions   int      // requested dimensions (optional, OpenAI supports this)
}

// Option configures an Embedder.
type Option func(*Embedder)

// WithClient sets a custom HTTP client.
func WithClient(c *http.Client) Option {
	return func(e *Embedder) { e.client = c }
}

// WithHeader adds a custom HTTP header.
func WithHeader(key, value string) Option {
	return func(e *Embedder) { e.headers[key] = value }
}

// WithInputField sets the JSON field name for the input text.
func WithInputField(field string) Option {
	return func(e *Embedder) { e.inputField = field }
}

// WithOutputPath sets the nested JSON path to the embedding array.
func WithOutputPath(path ...string) Option {
	return func(e *Embedder) { e.outputPath = path }
}

// WithModel sets the model identifier included in the request body.
func WithModel(model string) Option {
	return func(e *Embedder) { e.model = model }
}

// WithDimensions sets the requested output dimensions (OpenAI-specific).
func WithDimensions(d int) Option {
	return func(e *Embedder) { e.dimensions = d }
}

// NewEmbedder creates a generic HTTP embedder.
// endpoint is the full URL (e.g. "https://api.openai.com/v1/embeddings").
func NewEmbedder(endpoint string, opts ...Option) *Embedder {
	e := &Embedder{
		client:     &http.Client{Timeout: 30 * time.Second},
		endpoint:   endpoint,
		headers:    make(map[string]string),
		inputField: "input",
		outputPath: []string{"data", "0", "embedding"},
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Embed sends text to the configured endpoint and returns the embedding.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]any{e.inputField: text}
	if e.model != "" {
		payload["model"] = e.model
	}
	if e.dimensions > 0 {
		payload["dimensions"] = e.dimensions
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return extractFloatSlice(result, e.outputPath)
}

// Pre-configured constructors for popular services.

// NewOpenAIEmbedder creates an embedder for OpenAI's embeddings API.
func NewOpenAIEmbedder(apiKey, model string) *Embedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return NewEmbedder(
		"https://api.openai.com/v1/embeddings",
		WithHeader("Authorization", "Bearer "+apiKey),
		WithModel(model),
		WithInputField("input"),
		WithOutputPath("data", "0", "embedding"),
	)
}

// NewGeminiEmbedder creates an embedder for the Gemini embeddings API.
func NewGeminiEmbedder(apiKey, model string) *Embedder {
	if model == "" {
		model = "models/embedding-001"
	}
	return NewEmbedder(
		"https://generativelanguage.googleapis.com/v1beta/"+model+":embedContent?key="+apiKey,
		WithInputField("content"),
		WithOutputPath("embedding", "values"),
	)
}

// NewOllamaEmbedder creates an embedder for a local Ollama instance.
func NewOllamaEmbedder(baseURL, model string) *Embedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return NewEmbedder(
		baseURL+"/api/embeddings",
		WithModel(model),
		WithInputField("prompt"),
		WithOutputPath("embedding"),
	)
}

// extractFloatSlice walks a nested map and returns a []float32.
func extractFloatSlice(m map[string]any, path []string) ([]float32, error) {
	var v any = m
	for _, key := range path {
		switch cur := v.(type) {
		case map[string]any:
			v = cur[key]
		case []any:
			idx, err := parseIndex(key)
			if err != nil {
				return nil, fmt.Errorf("expected array index %q: %w", key, err)
			}
			if idx < 0 || idx >= len(cur) {
				return nil, fmt.Errorf("index %d out of range", idx)
			}
			v = cur[idx]
		default:
			return nil, fmt.Errorf("unexpected type %T at key %q", v, key)
		}
	}

	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array at end of path, got %T", v)
	}

	out := make([]float32, len(arr))
	for i, e := range arr {
		switch n := e.(type) {
		case float64:
			out[i] = float32(n)
		case float32:
			out[i] = n
		case int:
			out[i] = float32(n)
		default:
			return nil, fmt.Errorf("non-numeric value at index %d: %T", i, e)
		}
	}
	return out, nil
}

func parseIndex(s string) (int, error) {
	var idx int
	_, err := fmt.Sscanf(s, "%d", &idx)
	return idx, err
}
