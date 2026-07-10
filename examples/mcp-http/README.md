# MCP HTTP example

Uses the public **[FMHY MCP](https://fmhy-mcp.ken.tools/)** server:

- URL: `https://fmhy-mcp.ken.tools/mcp`
- Transport: `streamable_http`
- **No API key or auth** required
- Read-only access to the [FMHY](https://fmhy.net) link dataset

## Tools (namespaced as `fmhy__*`)

| Tool | Purpose |
|------|---------|
| `fmhy__fmhy_list_sections` | List dataset sections |
| `fmhy__fmhy_search` | Full-text search (`query` parameter) |
| `fmhy__fmhy_get_section` | Fetch a section by id |
| `fmhy__fmhy_get_links` | Links inside a section |
| `fmhy__fmhy_refresh_cache` | Refresh server cache (usually not needed) |

## Run

```powershell
$env:OPENAI_API_KEY = "sk-..."
go run ./cmd/airon run --config ./examples/mcp-http/airon.yaml --input "Go programming tutorials"
```

No local MCP server to start — the endpoint is on the public internet.

## Alternative public servers

Other no-auth streamable HTTP MCP servers that work with airon:

| Server | URL | Notes |
|--------|-----|-------|
| [turva-mcp](https://mcp.turva.dev/) | `https://mcp.turva.dev/mcp` | Read-only turva.dev catalog & security evidence |
| FMHY (this example) | `https://fmhy-mcp.ken.tools/mcp` | Searchable FMHY resource index |

Swap `mcp_servers` and `tools:` in `airon.yaml` to try another server.
