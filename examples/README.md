# airon examples

Teaching configs for common patterns. They intentionally stay small — advanced knobs (`runtime.outcome: pass_fail`, `mcp_servers[].timeout`, `{{input?}}`, Healthchecks) are documented in the [README](../README.md) and [writing-a-job](../docs/runbooks/writing-a-job.md) runbook; production backup-verify lives in onsite-infrastructure `util/backup-check/job/`.

| Example | What it demonstrates |
|---------|----------------------|
| [simple](./simple/) | Minimal pipeline with skills, agent, and `depends_on` |
| [custom-skills](./custom-skills/) | Several custom `SKILL.md` tools with distinct roles |
| [multi-phase](./multi-phase/) | Four-step prompt pipeline (`plan → research → draft → finalize`) |
| [mcp-stdio](./mcp-stdio/) | Local MCP server over **stdio** (`@modelcontextprotocol/server-filesystem`) |
| [mcp-http](./mcp-http/) | Public remote MCP over **streamable_http** ([FMHY](https://fmhy-mcp.ken.tools/mcp), no auth) |
| [full](./full/) | **All together**: multiple skills, stdio + HTTP MCP, multi-phase prompts |

## Prerequisites

- Go 1.25+
- `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` (match `provider.type` in `airon.yaml`)
- Node.js + `npx` for stdio MCP examples (`@modelcontextprotocol/server-filesystem`)

## Run

```powershell
$env:OPENAI_API_KEY = "sk-..."

# Simple
go run ./cmd/airon run --config ./examples/simple/airon.yaml --input "Go testing best practices"

# Custom skills
go run ./cmd/airon run --config ./examples/custom-skills/airon.yaml --input "Structured logging in Go"

# Multi-phase (no --input required if you pass --var topic=...)
go run ./cmd/airon run --config ./examples/multi-phase/airon.yaml --var topic="API rate limiting"

# MCP stdio
go run ./cmd/airon run --config ./examples/mcp-stdio/airon.yaml --input "inventory"

# MCP HTTP (public FMHY server, no local setup)
go run ./cmd/airon run --config ./examples/mcp-http/airon.yaml --input "Go programming tutorials"

# Full stack (stdio + HTTP MCP + skills)
go run ./cmd/airon run --config ./examples/full/airon.yaml --input "Add health checks to a Go service"
```
