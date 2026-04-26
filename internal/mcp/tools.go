// Package mcp implements the Model Context Protocol server for engram.
//
// It exposes engram's Store interface as three MCP tools:
//   - memory_store: store a new memory
//   - memory_query: query memories across four dimensions
//   - memory_link:  create a unidirected relationship between two memories
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/andrewhowdencom/engram/internal/timeutil"
	"github.com/andrewhowdencom/engram/pkg/engram"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Tool inputs
// ---------------------------------------------------------------------------

// MemoryStoreInput is the argument schema for the memory_store tool.
type MemoryStoreInput struct {
	Content string            `json:"content" jsonschema:"The memory content to store (required)"`
	Context map[string]string `json:"context,omitempty" jsonschema:"Optional key-value context metadata"`
}

// MemoryStoreOutput is the result schema for the memory_store tool.
type MemoryStoreOutput struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// MemoryQueryInput is the argument schema for the memory_query tool.
type MemoryQueryInput struct {
	ContextFilter    map[string]string `json:"context_filter,omitempty" jsonschema:"Exact key-value context filters (AND semantics)"`
	Similar          string            `json:"similar,omitempty" jsonschema:"Text for semantic similarity search"`
	SimilarThreshold float32           `json:"similar_threshold,omitempty" jsonschema:"Minimum similarity score (0.0-1.0)"`
	RelFrom          string            `json:"rel_from,omitempty" jsonschema:"Relationship origin memory ID"`
	RelType          string            `json:"rel_type,omitempty" jsonschema:"Relationship type filter"`
	RelDepth         int               `json:"rel_depth,omitempty" jsonschema:"Traversal depth (default 1)"`
	After            string            `json:"after,omitempty" jsonschema:"Only memories newer than duration (e.g. 24h, 7d)"`
	Before           string            `json:"before,omitempty" jsonschema:"Only memories older than duration"`
	Order            string            `json:"order,omitempty" jsonschema:"Ordering: relevance, recency, created (default relevance)"`
	Limit            int               `json:"limit,omitempty" jsonschema:"Maximum number of results (default 10)"`
	Focus            map[string]string `json:"focus,omitempty" jsonschema:"Agent-managed focus context (warms up ranking)"`
}

// LinkResult represents a single relationship in query output.
type LinkResult struct {
	Type string `json:"type"`
	To   string `json:"to"`
}

// MemoryResult is a single memory returned by memory_query.
type MemoryResult struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Context   map[string]string `json:"context"`
	Links     []LinkResult      `json:"links"`
	CreatedAt string            `json:"created_at"`
}

// MemoryQueryOutput is the result schema for the memory_query tool.
type MemoryQueryOutput struct {
	Memories []MemoryResult `json:"memories"`
}

// MemoryLinkInput is the argument schema for the memory_link tool.
type MemoryLinkInput struct {
	From string `json:"from" jsonschema:"Source memory ID (required)"`
	To   string `json:"to" jsonschema:"Target memory ID (required)"`
	Type string `json:"type,omitempty" jsonschema:"Relationship type (default: relates_to)"`
}

// MemoryLinkOutput is the result schema for the memory_link tool.
type MemoryLinkOutput struct {
	Success bool `json:"success"`
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

// MemoryStore returns a handler bound to the given store.
func MemoryStore(store engram.Store) func(context.Context, *sdkmcp.CallToolRequest, MemoryStoreInput) (*sdkmcp.CallToolResult, MemoryStoreOutput, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input MemoryStoreInput) (*sdkmcp.CallToolResult, MemoryStoreOutput, error) {
		m := engram.Memory{
			Content: []byte(input.Content),
			Context: input.Context,
		}
		stored, err := store.Put(ctx, m)
		if err != nil {
			return nil, MemoryStoreOutput{}, fmt.Errorf("failed to store memory: %w", err)
		}
		return nil, MemoryStoreOutput{
			ID:        stored.ID,
			CreatedAt: stored.CreatedAt.Format(time.RFC3339),
		}, nil
	}
}

// MemoryQuery returns a handler bound to the given store.
func MemoryQuery(store engram.Store) func(context.Context, *sdkmcp.CallToolRequest, MemoryQueryInput) (*sdkmcp.CallToolResult, MemoryQueryOutput, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input MemoryQueryInput) (*sdkmcp.CallToolResult, MemoryQueryOutput, error) {
		q := engram.Query{Limit: input.Limit}

		if len(input.ContextFilter) > 0 {
			q.ContextFilter = &engram.ContextFilter{Pairs: input.ContextFilter}
		}

		if input.Similar != "" {
			q.Similarity = &engram.SimilarityQuery{
				Text:      input.Similar,
				Threshold: input.SimilarThreshold,
			}
		}

		if input.RelFrom != "" {
			q.Relationship = &engram.RelationshipQuery{
				FromID: input.RelFrom,
				Type:   input.RelType,
				Depth:  input.RelDepth,
			}
		}

		tq := &engram.TemporalQuery{OrderBy: input.Order}
		if input.After != "" {
			d, err := timeutil.ParseDuration(input.After)
			if err != nil {
				return nil, MemoryQueryOutput{}, fmt.Errorf("invalid after duration %q: %w", input.After, err)
			}
			t := time.Now().Add(-d)
			tq.After = &t
		}
		if input.Before != "" {
			d, err := timeutil.ParseDuration(input.Before)
			if err != nil {
				return nil, MemoryQueryOutput{}, fmt.Errorf("invalid before duration %q: %w", input.Before, err)
			}
			t := time.Now().Add(-d)
			tq.Before = &t
		}
		q.Temporal = tq

		if len(input.Focus) > 0 {
			q.Focus = &engram.Focus{Context: input.Focus}
		}

		results, err := store.Query(ctx, q)
		if err != nil {
			return nil, MemoryQueryOutput{}, fmt.Errorf("query failed: %w", err)
		}

		memories := make([]MemoryResult, len(results))
		for i, m := range results {
			links := make([]LinkResult, len(m.Links))
			for j, l := range m.Links {
				links[j] = LinkResult{Type: l.Type, To: l.To}
			}
			memories[i] = MemoryResult{
				ID:        m.ID,
				Content:   string(m.Content),
				Context:   m.Context,
				Links:     links,
				CreatedAt: m.CreatedAt.Format(time.RFC3339),
			}
		}

		return nil, MemoryQueryOutput{Memories: memories}, nil
	}
}

// MemoryLink returns a handler bound to the given store.
func MemoryLink(store engram.Store) func(context.Context, *sdkmcp.CallToolRequest, MemoryLinkInput) (*sdkmcp.CallToolResult, MemoryLinkOutput, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input MemoryLinkInput) (*sdkmcp.CallToolResult, MemoryLinkOutput, error) {
		linkType := input.Type
		if linkType == "" {
			linkType = "relates_to"
		}
		if err := store.Link(ctx, input.From, input.To, linkType); err != nil {
			return nil, MemoryLinkOutput{}, fmt.Errorf("failed to link memories: %w", err)
		}
		return nil, MemoryLinkOutput{Success: true}, nil
	}
}

// ---------------------------------------------------------------------------
// Error helper
// ---------------------------------------------------------------------------

// ToolError creates a CallToolResult that signals an error to the MCP client.
func ToolError(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: msg},
		},
	}
}

// JSONString marshals v as indented JSON for tool result text content.
func JSONString(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
