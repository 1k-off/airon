# Runbook: Writing an airon job

This runbook walks you through authoring a new airon job (a YAML config plus any accompanying skills / prompts / MCP servers). It complements the top-level [README](../../README.md) — read the "Config Summary" section there first for the field reference.

## 1. Anatomy of an airon job

An airon job is a single YAML file. Everything referenced in it (skills, prompts, MCP servers, workspace) is resolved relative to the file's directory unless an absolute path is given.

```yaml
version: "1"

provider:
  type: openai              # or "anthropic"
  model: gpt-4o-mini
  api_key: ${OPENAI_API_KEY}

workspace:
  path: ./workspace         # sandboxed root for built-in fs tools

runtime:
  max_tool_rounds: 20       # per prompt step
  max_call_depth: 3         # nesting: step→agent→skill fits in 3 after depth fix
  timeout: 45m              # optional whole-run deadline (Go duration)
  stream: false
  save_outputs: true        # persist each step output to workspace/outputs/<step_id>.md
  log_inputs: true          # set false to omit input map from run JSON (secrets still redacted)

log:
  level: info

notify:
  healthchecks:
    url: ${HC_UUID_URL}     # optional healthchecks.io ping URL

skills:
  - path: ./skills/researcher

agents:
  - id: analyst
    description: Summarize findings
    system_prompt: You are a careful analyst.
    tools: [fs_read, researcher]
    max_tool_rounds: 10

mcp_servers:
  - name: backup
    transport: streamable_http
    url: ${BACKUP_CHECK_MCP_URL}
    timeout: 60s                # HTTP client timeout (default 60s)
    # env_passthrough: false    # for stdio: do not inherit host env (API keys)
    headers:
      Authorization: Bearer ${MCP_TOKEN}

prompts:
  - id: gather
    content: "Investigate {{input}} and list relevant sources."
    tools: [researcher, backup__list_snapshots]

  - id: summarize
    content: "Summarize the findings from step gather:\n\n{{gather}}"
    tools: [analyst]
    depends_on: [gather]
```

### Key ideas

- **Prompts form a DAG.** Each entry in `prompts` runs as a distinct executor step. The step's output is exposed to downstream prompts as `{{step_id}}`; `depends_on` establishes ordering when the template does not.
- **Tools are opt-in per step.** A prompt only sees the tools listed in its `tools` array (built-ins like `fs_read` / `fs_write`, plus skills, agents, and namespaced MCP tools `<server>__<tool>`).
- **Skills are self-contained sub-prompts.** They live in a directory with a `SKILL.md` (front-matter + system prompt) and expose themselves as a single callable tool.
- **Agents are inline tool clusters.** They run their own prompt loop with a scoped tool set. Use them when a skill file would be overkill.
- **Env expansion (`${VAR}`) is available everywhere strings are read** — provider keys, MCP URLs, headers, notify URLs, skill paths, prompt content.

## 2. Authoring a skill

1. Create `./skills/my-skill/SKILL.md` (path referenced from `skills:`).
2. Add YAML front matter and a system prompt:

```markdown
---
name: my-skill
description: One-line summary shown to the LLM as the tool description.
tools: [fs_read, fs_write]   # optional: restrict what this skill may call
---

You are the my-skill assistant. Follow these steps:
1. ...
2. ...
```

3. Reference it from a prompt: `tools: [my-skill]`.

Skills nest: a skill may call other skills or agents, bounded by `runtime.max_call_depth`.

## 3. Wiring an MCP server

airon supports three transports:

| Transport | Required fields | When to use |
|---|---|---|
| `stdio` (default) | `command`, optional `args`, `env` | Local process MCP server |
| `streamable_http` | `url`, optional `headers` | Long-running remote MCP (e.g. `backup-check-mcp` on Docker) |
| `sse` | `url`, optional `headers` | Legacy SSE endpoints |

Example — remote HTTP MCP protected by a bearer token:

```yaml
mcp_servers:
  - name: backup
    transport: streamable_http
    url: ${BACKUP_CHECK_MCP_URL}   # e.g. http://localhost:8080/mcp
    headers:
      Authorization: Bearer ${MCP_TOKEN}
```

Every tool the server advertises becomes available under the namespace `backup__<tool>` (double underscore). Reference specific tools in a prompt's `tools:` array — unknown tool names fail loudly at load time.

### Concrete pattern: backup-check

The `util/backup-check` project in the onsite-infrastructure repo runs an HTTP MCP server (`backup-check-mcp`) inside Docker Compose. An airon job that drives it typically:

1. Sets `notify.healthchecks.url` to a per-job UUID.
2. Configures a single `streamable_http` MCP server pointing at `${BACKUP_CHECK_MCP_URL}`.
3. Uses a single prompt with a high `runtime.max_tool_rounds` (e.g. 40) and a system prompt (via a skill) that mandates: `pick_next` → `download` → `restore` → `inspect_schema` → `write_report` → `update_registry` → `cleanup`.
4. Guarantees one database per run (batch = 1), never touches non-`verify_` databases.

See `util/backup-check/job/airon.yaml` for the concrete config once Task 9 lands.

## 4. Template variables

- `{{input}}` — CLI `--input "text"` (or `Options.Input["input"]` when embedding).
- `{{step_id}}` — output text from a prior prompt step.
- `{{var}}` — anything supplied via CLI `--var key=value` (or `Options.Input`).
- `{{file:relative/path}}` — inline file contents resolved relative to the config directory.

Missing template variables fail the run with a clear error. That is why `depends_on` matters: it prevents a step from running before its upstream step has produced output.

## 5. Testing your job

Unit-testable pieces:

- **Config load** — run `go test ./internal/config/...` after any parsing changes.
- **Job dry-run** — invoke with a fake input and `--no-healthchecks`, e.g. `airon run --config ./job/airon.yaml --no-healthchecks --input "test"`.
- **Embedded** — the `pkg/airon` public API takes an `Options` struct with the same knobs and never calls `os.Exit`, which makes it easy to unit-test the wrapper in isolation.

For live MCP integration, run the MCP server locally first (e.g. `docker compose -f util/backup-check/docker/docker-compose.yml up mssql backup-check-mcp`) and set `BACKUP_CHECK_MCP_URL=http://localhost:8080/mcp` before running airon.

## 6. Common pitfalls

- **Unknown tool** — `prompts[*].tools` must reference either a built-in, a loaded skill, an agent id, or an MCP `<server>__<tool>` symbol. Anything else fails at load.
- **Cyclic prompts** — `depends_on` must be a DAG; cycles fail at load.
- **Env expansion missing** — a `${VAR}` reference with no environment value fails immediately. This is intentional so bad configs do not silently ping the wrong URL.
- **Long-running MCP calls** — bump `runtime.max_tool_rounds` if the LLM legitimately needs more back-and-forth. Keep `max_call_depth` conservative.
- **Skill/system prompt drift** — skills are just files; changes take effect on the next run without a rebuild.
