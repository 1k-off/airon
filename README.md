# airon

`airon` is a lightweight Go runtime for orchestrating prompt pipelines with tools, skills, agents, and optional MCP servers.

## Quick Start

1. Export your API key:
   - PowerShell: `$env:OPENAI_API_KEY="your-key"`
   - Bash: `export OPENAI_API_KEY="your-key"`
2. Run the example:
   - `go run ./cmd/airon run --config ./examples/simple/airon.yaml --input "Go testing best practices"`
3. Grafana + MCP (query metrics over HTTP):
   - See `examples/grafana/airon-stdio.yaml`
4. Google Ads MCP (HTTP + Bearer auth):
   - See `examples/google-ads/airon.yaml`
5. Check run logs:
   - `examples/simple/workspace/logs/<run_id>.json`
4. Optional: stream assistant text to stdout:
   - `go run ./cmd/airon run --config ./examples/simple/airon.yaml --input "topic" --stream`

## Config Summary

- `version`: runtime config version (`"1"`).
- `provider`: model provider settings (`type`, `model`, `api_key`, optional `base_url`, `temperature`).
  - Supported providers: `openai`, `anthropic`.
- `workspace.path`: sandbox root used by built-in filesystem tools.
- `runtime`: execution options:
  - `max_tool_rounds` (default 10)
  - `max_call_depth` (default 3)
  - `stream` — enable streaming in config (overridden by CLI `--stream`)
  - `save_outputs` — write each step output to `workspace/outputs/<step_id>.md`
- `skills`: list of skill file or directory paths.
- `agents`: optional agent tools with system prompt and tool scope.
- `mcp_servers`: optional MCP servers (`stdio`, `streamable_http`, `sse`), each namespaced as `<server>__<tool>`.
- `prompts`: ordered DAG of prompt steps (`id`, `content`, `tools`, `depends_on`).

### MCP server transports

| Transport | Required fields | Example |
|---|---|---|
| `stdio` (default) | `command`, optional `args`, `env` | Local process MCP server |
| `streamable_http` | `url`, optional `headers` | Remote HTTP MCP endpoint |
| `sse` | `url`, optional `headers` | Legacy SSE MCP endpoint |

```yaml
mcp_servers:
  - name: local
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "./workspace"]

  - name: remote
    transport: streamable_http
    url: http://localhost:8080/mcp
    headers:
      Authorization: Bearer ${MCP_TOKEN}
```

### Template variables

- `{{input}}` — set via CLI `--input`
- `{{step_id}}` — output from a prior prompt step
- `{{var}}` — additional vars via `--var key=value`
- `{{file:relative/path}}` — inline file contents from workspace (resolved relative to config directory)

## CLI

```bash
airon run --config ./airon.yaml [--input "text"] [--stream] [--var key=value]
```

Exit codes:
- `0`: success
- `1`: runtime error
- `2`: usage error

## Security Notes

- Keep API keys in environment variables; do not commit secrets to config files.
- File operations are sandboxed to `workspace.path` via `workspace.Manager`.
- Symlink escape attempts on write are rejected.
- Use trusted skill files and MCP servers only; they can influence runtime behavior.
- Set conservative `max_tool_rounds` and `max_call_depth` to prevent runaway loops.

## Build & Test

```bash
go test ./...
go build -o bin/airon ./cmd/airon/
```
