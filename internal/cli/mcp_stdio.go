package cli

import (
	"fmt"

	"github.com/andrewhowdencom/engram/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPStdioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stdio",
		Short: "Run the MCP server over stdio",
		Long: `stdio runs the engram MCP server over standard input/output
using JSON-RPC. This is the standard transport for local MCP clients
such as Claude Desktop and Cursor.

The server exposes three tools:
  memory_store  — store a new memory
  memory_query  — query across four dimensions
  memory_link   — create relationships between memories`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			server := mcp.NewServer(store)
			fmt.Fprintln(cmd.ErrOrStderr(), "engram MCP server starting on stdio...")
			if err := server.Run(cmd.Context(), &sdkmcp.StdioTransport{}); err != nil {
				return fmt.Errorf("mcp server error: %w", err)
			}
			return nil
		},
	}
	return cmd
}
