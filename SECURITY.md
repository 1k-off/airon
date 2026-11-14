# Security Policy

## Supported versions

Security fixes land on the latest commit of the default branch and on the most recent tagged release when tags exist.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security problems.

Email the maintainer via the contact listed on [github.com/1k-off](https://github.com/1k-off) with:

- a description of the issue and impact
- steps to reproduce (PoC welcome)
- affected commit / tag if known

You should receive an acknowledgement within a few days.

## Hardening notes for operators

- Keep LLM API keys and MCP credentials in environment variables or a secrets manager — never in committed YAML.
- Treat skills and MCP servers as trusted code: they can influence tool use and file writes inside `workspace.path`.
- Bound `runtime.max_tool_rounds`, `runtime.max_call_depth`, and `runtime.timeout` in production jobs.
- Prefer `airon check --config … --strict` in deploy pipelines before scheduling `airon run`.
- Stdio MCP servers inherit the host environment by default (so child processes see `OPENAI_API_KEY`, etc.). Set `mcp_servers[].env_passthrough: false` and pass only the vars you need under `env:` when the server is less trusted.
- Run logs redact input keys matching `api_key` / `secret` / `password` / `token` patterns. Set `runtime.log_inputs: false` to omit the input map entirely.
