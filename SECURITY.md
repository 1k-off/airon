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
- Bound `runtime.max_tool_rounds` and `runtime.max_call_depth` in production jobs.
- Prefer `airon check --config … --strict` in deploy pipelines before scheduling `airon run`.
