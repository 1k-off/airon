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
  # max_tokens: 4096        # optional completion cap

workspace:
  path: ./workspace         # sandboxed root for built-in fs tools

runtime:
  max_tool_rounds: 20       # per prompt step
  max_call_depth: 3         # nesting: step→agent→skill fits in 3
  timeout: 45m              # optional whole-run deadline (Go duration)
  stream: false
  save_outputs: true        # persist each step output to workspace/outputs/<step_id>.md
  log_inputs: true          # set false to omit input map from run JSON (secrets still redacted)
  # outcome: pass_fail      # FAIL / missing PASS → non-zero exit + healthchecks /fail

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
    timeout: 45m                # HTTP client timeout (default 60s — raise for long tools)
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
| `stdio` (default) | `command`, optional `args`, `env`, `env_passthrough` | Local process MCP server |
| `streamable_http` | `url`, optional `headers`, `timeout` | Long-running remote MCP (e.g. `backup-check-mcp` on Docker) |
| `sse` | `url`, optional `headers`, `timeout` | Legacy SSE endpoints |

Example — remote HTTP MCP protected by a bearer token:

```yaml
mcp_servers:
  - name: backup
    transport: streamable_http
    url: ${BACKUP_CHECK_MCP_URL}   # e.g. http://localhost:8080/mcp
    timeout: 45m
    headers:
      Authorization: Bearer ${MCP_TOKEN}
```

Every tool the server advertises becomes available under the namespace `backup__<tool>` (double underscore). Reference specific tools in a prompt's `tools:` array — unknown tool names fail loudly at load time.

### Concrete pattern: backup-check

The `util/backup-check` project in onsite-infrastructure runs an HTTP MCP server (`backup-check-mcp`) in Docker/Podman. A typical airon job:

1. Sets `notify.healthchecks.url` and `runtime.outcome: pass_fail` (agent must end with `PASS:` / `FAIL:`).
2. Points `streamable_http` at the MCP (`url` + `timeout: 45m` — large bacpac restores exceed the 60s default).
3. Uses one prompt + `backup-verifier` skill with high `max_tool_rounds` (e.g. 40).
4. Workflow: orient (`pick_next` or `catalog_get`) → download → `extract_archive` when nested → restore → inspect/query → report → registry → cleanup.
5. Guarantees one database per run; never touches non-`verify_*` databases. Cleanup may no-op when the MCP sets `CLEANUP_ENABLED=false` (onsite gate).

See `util/backup-check/job/airon.yaml` and `util/backup-check/RUNBOOK.md` in onsite-infrastructure.

## 4. Template variables

- `{{input}}` — CLI `--input "text"` (or `Options.Input["input"]` when embedding). **Required** if referenced.
- `{{input?}}` — same key, but optional (expands to `""` when omitted). Use for jobs that also support empty input / `pick_next`.
- `{{step_id}}` — output text from a prior prompt step.
- `{{var}}` / `{{var?}}` — anything supplied via CLI `--var key=value` (or `Options.Input`).
- `{{file:relative/path}}` — inline file contents resolved under the sandboxed `workspace.path` (not the config directory).

Missing **required** template variables fail the run with a clear error. That is why `depends_on` matters: it prevents a step from running before its upstream step has produced output.

## 5. Testing your job

Unit-testable pieces:

- **Config load** — run `go test ./internal/config/...` after any parsing changes.
- **Job dry-run** — invoke with a fake input and `--no-healthchecks`, e.g. `airon run --config ./job/airon.yaml --no-healthchecks --input "test"`.
- **Offline gate** — `airon check --config ./job/airon.yaml` (add `--strict` in CI).
- **Embedded** — the `pkg/airon` public API takes an `Options` struct with the same knobs and never calls `os.Exit`, which makes it easy to unit-test the wrapper in isolation.

For live MCP integration, run the MCP server locally first (e.g. `docker compose -f util/backup-check/docker/docker-compose.yml up mssql backup-check-mcp`) and point the job URL at `http://localhost:8080/mcp` before running airon.

## 6. Common pitfalls

- **Unknown tool** — `prompts[*].tools` must reference either a built-in, a loaded skill, an agent id, or an MCP `<server>__<tool>` symbol. Anything else fails at load.
- **Cyclic prompts** — `depends_on` must be a DAG; cycles fail at load.
- **Env expansion missing** — a `${VAR}` reference with no environment value fails immediately. This is intentional so bad configs do not silently ping the wrong URL.
- **MCP client timeout** — HTTP tools that run longer than `mcp_servers[*].timeout` (default 60s) fail with `Client.Timeout exceeded`. Raise the per-server timeout; `runtime.timeout` is a separate whole-run deadline.
- **Long tool loops** — bump `runtime.max_tool_rounds` if the LLM legitimately needs more back-and-forth. Keep `max_call_depth` conservative.
- **Required `{{input}}` without `--input`** — use `{{input?}}` when the job must also run with empty input.
- **Skill/system prompt drift** — skills are just files; changes take effect on the next run without a rebuild.
