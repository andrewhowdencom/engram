package mcp

import (
	"github.com/andrewhowdencom/engram/pkg/engram"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server exposing engram's Store as three tools.
func NewServer(store engram.Store) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "engram",
		Version: "0.1.0",
	}, nil)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "memory_store",
		Description: "Store a new memory in the agent's unified memory system. Returns the assigned memory ID and creation timestamp.",
	}, MemoryStore(store))

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "memory_query",
		Description: "Query memories across all four dimensions (context, similarity, relationship, time) with optional agent-managed focus. Results are ranked by composite relevance.",
	}, MemoryQuery(store))

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "memory_link",
		Description: "Create a unidirected relationship between two memories. The relationship is traversed symmetrically in queries but declared one-way.",
	}, MemoryLink(store))

	return server
}
