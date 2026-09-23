# airon

`airon` is an **AI-powered cron** — a Go runtime for autonomously running AI jobs on a schedule or on demand.

You give it a config: LLM **provider** and **model**, a pipeline of **prompts**, optional **skills** and **agents**, and **MCP servers** for external tools. It executes the pipeline, calls tools as needed, and returns the **result** (step outputs, workspace artifacts, and a JSON run log).

Under the hood it is a lightweight orchestrator for prompt pipelines with tool calling, skill delegation, and MCP integration — designed to be wired into cron, CI, or any job runner that needs a deterministic `run` → `output` contract.

## Quick Start

1. Export your API key (match `provider.type` in `airon.yaml`):
   - OpenAI — PowerShell: `$env:OPENAI_API_KEY="your-key"` / Bash: `export OPENAI_API_KEY="your-key"`
   - Anthropic — PowerShell: `$env:ANTHROPIC_API_KEY="your-key"` / Bash: `export ANTHROPIC_API_KEY="your-key"`
2. Pick an example (see [examples/README.md](./examples/README.md)):
   - **simple** — skills + agent: `go run ./cmd/airon run --config ./examples/simple/airon.yaml --input "Go testing best practices"`
   - **custom-skills** — multiple custom `SKILL.md` tools
   - **multi-phase** — four-step pipeline with `depends_on`
   - **mcp-stdio** — local MCP over stdio (filesystem server)
   - **mcp-http** — public HTTP MCP (FMHY dataset, no auth)
   - **full** — skills + stdio MCP + HTTP MCP + multi-phase prompts
3. Check run logs:
   - JSON: `examples/<name>/workspace/logs/<run_id>.json`
   - Stderr prints each step as it finishes (`step`, `rounds`, `tool_calls`, `duration`, token counts)
4. Optional: stream assistant text to stdout:
   - add `--stream` to any `run` command

### Run log usage

Each step and the full run record token usage (no cost estimates):

```json
"usage": {
  "prompt_tokens": 12500,
  "completion_tokens": 3200,
  "total_tokens": 15700,
  "by_model": [
    { "model": "gpt-4o-mini", "prompt_tokens": 10000, "completion_tokens": 2500, "total_tokens": 12500 }
  ]
}
```

Nested skill/agent calls are included in the step that invoked them.

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
- `log.level`: stderr log level (`debug`, `info`, `warn`, `error`; default `info`).
- `notify.healthchecks.url`: optional healthchecks.io-compatible ping URL. See [Notifications](#notifications).

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
airon run --config ./airon.yaml [--input "text"] [--stream] [--no-healthchecks] [--var key=value]
```

Flags:
- `--config` — required. Path to the airon YAML config.
- `--input "text"` — sets the `input` template variable.
- `--var key=value` — sets an additional template variable (repeatable).
- `--stream` — stream assistant text to stdout even if the config disables streaming.
- `--no-healthchecks` — disable the healthchecks.io notifier for this run. The environment variable `AIRON_HEALTHCHECKS` set to `0` / `false` / `no` / `off` has the same effect.

Exit codes:
- `0`: success
- `1`: runtime error
- `2`: usage error

## Notifications

`notify.healthchecks.url` accepts a healthchecks.io-compatible ping URL. When set (and not disabled) airon issues:

1. `GET <url>/start` immediately before the executor begins the run.
2. `GET <url>` on success, or `POST <url>/fail` with the primary error text on failure.

The notifier is a no-op when the URL is empty, when `--no-healthchecks` is passed, or when `AIRON_HEALTHCHECKS=0` (also accepts `false`, `no`, `off`). Notifier HTTP errors are logged to stderr but never override or mask the primary run error — a healthchecks outage will never fail an otherwise-successful run.

```yaml
notify:
  healthchecks:
    url: https://hc-ping.com/${HC_UUID}   # env expansion is supported
```

## Embedding airon as a library (`pkg/airon`)

Cron wrappers, other services, or tests can drive airon directly instead of shelling out to the CLI. The public surface is intentionally small:

```go
import "github.com/1node/airon/pkg/airon"

res, err := airon.Run(ctx, airon.Options{
    ConfigPath:     "./airon.yaml",
    Input:          map[string]string{"input": "hello"},
    NoHealthchecks: true, // optional
    Stream:         false,
})
if err != nil {
    log.Fatal(err)
}
log.Printf("run_id=%s success=%t steps=%d", res.RunID, res.Success, len(res.Steps))
```

`airon.Run` never calls `os.Exit`. It returns the primary run error alongside a `Result` summarizing the run (`RunID`, `Success`, per-step outputs, token usage, timestamps). Bootstrap failures (bad config, MCP handshake, etc.) surface as errors with a zero-valued `Result`.

### Embedding in cron

Prefer a tiny Go wrapper over invoking the binary — it lets you keep secrets in-process, react to `Result` fields, and skip the process spawn cost. A typical wrapper:

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    res, err := airon.Run(ctx, airon.Options{ConfigPath: "/etc/airon/job.yaml"})
    switch {
    case err != nil:
        log.Printf("airon failed: %v", err)
        os.Exit(1)
    case !res.Success:
        log.Printf("airon completed with soft failure: %s", res.Error)
        os.Exit(1)
    default:
        log.Printf("airon ok run_id=%s", res.RunID)
    }
}
```

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

# Full local gate (unit + race + 80% coverage + build)
make ci                    # Linux/macOS
powershell -File scripts/ci.ps1   # Windows (race needs gcc/CGO)

# Live provider E2E (requires API keys)
make test-integration
powershell -File scripts/test-integration.ps1

# Optional overrides for E2E models (defaults: gpt-4o-mini, claude-haiku-4-5)
export OPENAI_E2E_MODEL=gpt-4o-mini
export ANTHROPIC_E2E_MODEL=claude-haiku-4-5
```

### CI (GitHub Actions)

| Workflow | Trigger | What it runs |
|---|---|---|
| [ci.yml](./.github/workflows/ci.yml) | push/PR to `main`/`master`, manual | unit tests, `-race`, 80% coverage gate, build |
| [integration.yml](./.github/workflows/integration.yml) | manual only | live OpenAI + Anthropic E2E (`gpt-4o-mini`, `claude-haiku-4-5`) |

Integration workflow expects repository secrets `OPENAI_API_KEY` and `ANTHROPIC_API_KEY`.
