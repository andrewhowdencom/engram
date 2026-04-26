package cli

import (
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the Model Context Protocol server",
		Long: `mcp exposes engram's memory operations as MCP tools,
allowing any MCP-compatible agent to use engram as its memory backend.

Subcommands:
  stdio   Run MCP server over stdio (JSON-RPC)
  http    Run MCP server over HTTP/SSE`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newMCPStdioCmd())
	cmd.AddCommand(newMCPHTTPCmd())

	return cmd
}
