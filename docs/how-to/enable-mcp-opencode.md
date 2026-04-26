# Enable the engram MCP Server in OpenCode

This guide shows you how to connect OpenCode to engram via the Model Context Protocol (MCP) so that OpenCode can store and query memories using the `memory_store`, `memory_query`, and `memory_link` tools.

## Prerequisites

- engram is installed. The `task install:binary` target copies it to `$HOME/.local/bin/engram`. Ensure `$HOME/.local/bin` is on your `PATH`, or use the absolute path in the config below.
- OpenCode is installed. See the [OpenCode installation docs](https://github.com/opencode-ai/opencode) or run `brew install opencode-ai/tap/opencode`.

## 1. Locate the OpenCode Config

OpenCode looks for configuration in the following locations (first match wins):

1. `$HOME/.opencode.json`
2. `$XDG_CONFIG_HOME/opencode/.opencode.json`
3. `./.opencode.json` (local directory)

If none exist, create `$HOME/.opencode.json`.

## 2. Add the engram Server Entry

Open the config file and add an `engram` entry under the `mcp` key:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "engram": {
      "type": "local",
      "command": ["engram", "mcp", "stdio"],
      "enabled": true,
      "env": []
    }
  }
}
```

- `command` — the engram binary and its arguments as an array. If `$HOME/.local/bin` is not on your `PATH`, use the absolute path for the first element (e.g. `"/home/you/.local/bin/engram"` or `"/Users/you/.local/bin/engram"`).
- `type` must be `"local"` for a locally-installed binary.
- `enabled` must be `true` for the server to be active.
- `env` is optional; leave it empty unless you need to pass environment variables.

## 3. Restart OpenCode

Fully quit OpenCode and reopen it so that the new MCP server is registered and its tools are discovered.

## 4. Verify the Tools Are Available

When you start a new session, the AI assistant can now invoke MCP tools. Ask it to store a memory:

```
store a memory that I prefer dark mode in all UIs
```

OpenCode should prompt you to approve the `memory_store` tool call. Once approved, the memory is written to engram's store.

If the tools do not appear, run OpenCode in debug mode to inspect MCP loading:

```bash
opencode -d
```

## 5. Tag Memories with the Agent Context

When OpenCode stores a memory, encourage it to set the `agent` context so that queries later filter to the right agent:

```json
{
  "content": "Use the OpenCode custom-commands directory for shared prompts",
  "context": {
    "agent": "opencode",
    "project": "engram"
  }
}
```

You can also set a default focus in queries so that OpenCode's own memories surface first.

## See Also

- [MCP Server Reference](../../reference/mcp.md) — Full tool schema and design notes
- [Integrate as a Go Library](./integrate-as-library.md) — Using engram directly in code
