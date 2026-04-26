package cli

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/andrewhowdencom/engram/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPHTTPCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "http",
		Short: "Run the MCP server over HTTP (streamable)",
		Long: `http runs the engram MCP server over HTTP using the streamable
HTTP transport (MCP spec 2025-11-25). This is the modern HTTP transport
that replaces the older SSE-based approach.

The server exposes three tools:
  memory_store  — store a new memory
  memory_query  — query across four dimensions
  memory_link   — create relationships between memories`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr := "localhost:" + strconv.Itoa(port)

			// StreamableHTTPHandler creates a new MCP server per request,
			// all backed by the shared store. This implements the
			// 2025-11-25 spec streamable HTTP transport.
			handler := sdkmcp.NewStreamableHTTPHandler(
				func(_ *http.Request) *sdkmcp.Server {
					return mcp.NewServer(store)
				},
				&sdkmcp.StreamableHTTPOptions{},
			)

			mux := http.NewServeMux()
			mux.Handle("/", handler)

			server := &http.Server{Addr: addr, Handler: mux}

			fmt.Fprintf(cmd.ErrOrStderr(), "engram MCP streamable HTTP server starting on http://%s\n", addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("http server error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP server port")

	return cmd
}
