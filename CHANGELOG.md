# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-11-08

First public release of **airon** — an AI-powered cron / job runner for prompt pipelines with skills, agents, and MCP tools.

### Added
- CLI: `airon run`, `airon check [--strict]`, `airon version` (ldflags-stamped build info)
- Config v1: provider (`openai` / `anthropic`), workspace sandbox, skills, agents, MCP servers, prompt DAG
- MCP transports: `stdio`, `streamable_http`, `sse` (tools namespaced as `<server>__<tool>`)
- `mcp_servers[].timeout` for HTTP/SSE clients (default 60s); `env_passthrough` for stdio children
- `runtime.timeout`, `max_tool_rounds`, `max_call_depth`, `save_outputs`, `stream`, `log_inputs`
- `runtime.outcome: pass_fail` — agent `FAIL` / missing `PASS` → non-zero exit and Healthchecks `/fail`
- Optional template variables with `?` (`{{input?}}`) so jobs can run without `--input`
- `{{file:path}}` inlining from the sandboxed workspace; `${ENV}` expansion in config strings
- Healthchecks.io-compatible notifier (`notify.healthchecks.url`, `--no-healthchecks`, `AIRON_HEALTHCHECKS`)
- Embeddable library API: `pkg/airon` (`Run`, `Check`, injectable provider I/O, `Result.LogPath`)
- JSON run logs under `workspace/logs/` with per-step tool calls and token usage
- Operator runbooks, examples (simple / multi-phase / MCP stdio+HTTP / full), CI and coverage gate

### Security
- Workspace sandbox with symlink-escape rejection on writes
- Sensitive input keys redacted from run logs; optional `log_inputs: false`
- Stdio MCP host-env inheritance documented; opt out via `env_passthrough: false`

### Fixed
- Tool call errors recorded in run JSON (`ToolCallResult.Error`)
- Agent nesting no longer double-increments `max_call_depth` (step → agent → skill works at default 3)
- Healthchecks success/fail pings use a context that survives run cancellation
- MCP HTTP clients always apply a client timeout
- Config-relative paths resolve from the config directory (portable jobs)
- Validation errors name the offending version / provider / prompt id

[0.1.0]: https://github.com/1k-off/airon/releases/tag/v0.1.0
