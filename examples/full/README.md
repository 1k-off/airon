# Full example

End-to-end **resource guide** pipeline: research public links, write a draft, peer-edit, publish.

Demonstrates **three skills**, **stdio MCP** (filesystem), **HTTP MCP** ([FMHY](https://fmhy-mcp.ken.tools/mcp)), and a **five-phase** `depends_on` chain.

## Scenario

Input: a technical topic (e.g. `"Observability for a Go microservice"`).

| Phase | Purpose |
|-------|---------|
| `discover` | Workspace inventory + FMHY connectivity |
| `research` | FMHY search + `researcher` skill → `notes.md` |
| `draft` | `writer` skill → `draft.md` |
| `edit` | `editor` skill + FMHY links for gaps |
| `publish` | Merge draft + edit brief → `final.md` |

Each step builds on the previous one; FMHY is used where external references matter, not as a fake health check.

## Run

```powershell
$env:OPENAI_API_KEY = "sk-..."
go run ./cmd/airon run --config ./examples/full/airon.yaml --input "Observability for a Go microservice"
```

Requires **Node.js/npx** for the stdio filesystem MCP.
