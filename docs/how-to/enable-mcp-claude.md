# Enable the engram MCP Server in Claude Desktop

This guide shows you how to connect Claude Desktop to engram via the Model Context Protocol (MCP) so that Claude can store and query memories using the `memory_store`, `memory_query`, and `memory_link` tools.

## Prerequisites

- engram is installed. The `task install:binary` target copies it to `$HOME/.local/bin/engram`. Ensure `$HOME/.local/bin` is on your `PATH`, or use the absolute path in the config below.
- Claude Desktop is installed. Download it from [claude.ai/download](https://claude.ai/download).

## 1. Locate the Claude Desktop Config

The MCP server list lives in a JSON file whose path depends on your OS:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

If the file does not exist, create it.

## 2. Add the engram Server Entry

Open the config file and add an `engram` entry under the `mcpServers` key:

```json
{
  "mcpServers": {
    "engram": {
      "command": "engram",
      "args": ["mcp", "stdio"],
      "env": {}
    }
  }
}
```

- `command` — name of the engram binary. If `$HOME/.local/bin` is not on your `PATH`, use the absolute path (e.g. `"/home/you/.local/bin/engram"` or `"/Users/you/.local/bin/engram"`).
- `args` must include `mcp` and `stdio` so that Claude spawns the JSON-RPC stdio transport.

## 3. Restart Claude Desktop

Fully quit Claude Desktop and reopen it so that the new MCP server is loaded.

## 4. Verify the Tools Are Available

1. Open a new conversation.
2. Look for the tool-hammer icon in the composer. You should see:
   - `memory_store`
   - `memory_query`
   - `memory_link`

If the tools do not appear, check the MCP server logs:

| OS | Logs path |
|---|---|
| macOS | `~/Library/Logs/Claude/mcp.log` |
| Windows | `%APPDATA%\Claude\Logs\mcp.log` |
| Linux | `~/.config/Claude/Logs/mcp.log` |

## 5. Tag Memories with the Agent Context

When Claude stores a memory, encourage it to set the `agent` context so that queries later filter to the right agent:

```json
{
  "content": "User prefers short variable names",
  "context": {
    "agent": "claude",
    "project": "engram"
  }
}
```

You can also set a default focus in queries so that Claude's own memories surface first.

## See Also

- [MCP Server Reference](../../reference/mcp.md) — Full tool schema and design notes
- [Integrate as a Go Library](./integrate-as-library.md) — Using engram directly in code
