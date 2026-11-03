# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- `airon check [--strict]` offline config validation
- `airon version` / `--version` with ldflags-stamped build info
- `pkg/airon.Check`, injectable `Provider`, `Stdout`/`Stderr`, `Result.LogPath` / `WorkspaceRoot`
- `runtime.timeout`, `provider.max_tokens`, `mcp_servers[].timeout`
- `mcp_servers[].env_passthrough` to stop leaking host secrets into stdio MCP children
- `runtime.log_inputs` plus automatic redaction of sensitive input keys in run logs

### Fixed
- Tool call errors are recorded in `ToolCallResult.Error` / run JSON
- Default `max_call_depth` no longer breaks step → agent → skill (agent no longer double-increments depth)
- Healthchecks `/fail` and `/success` pings survive a canceled run context
- MCP HTTP clients always have a timeout (default 60s)
- Config relative paths resolve from the config directory first (portable jobs)
- Validation errors include the offending version / provider / prompt id

### Security
- Documented stdio MCP env inheritance; opt-out via `env_passthrough: false`
- Sensitive template input values redacted from persisted run logs
