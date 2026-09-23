# Runbook: Operating airon

Operational playbook for running airon in production — secrets, log paths, notifications, and failure triage. Companion to [writing-a-job.md](./writing-a-job.md).

## 1. Secrets & environment

airon reads every secret from the environment. It never accepts secrets in code and never persists them in the workspace.

| Variable | Consumer | Notes |
|---|---|---|
| `OPENAI_API_KEY` | `provider.type: openai` (via `${OPENAI_API_KEY}` in config) | Required if using OpenAI. |
| `ANTHROPIC_API_KEY` | `provider.type: anthropic` | Required if using Anthropic. |
| `AIRON_HEALTHCHECKS` | Runtime | `0` / `false` / `no` / `off` disables the healthchecks.io notifier (same as `--no-healthchecks`). |
| Job-specific | Anything referenced with `${VAR}` in the YAML config | e.g. `HC_UUID`, `BACKUP_CHECK_MCP_URL`, `MCP_TOKEN`. |

Provisioning tips:

- On Linux systemd cron units, load secrets via `EnvironmentFile=/etc/airon/<job>.env` (mode `0600`, owned by the airon user).
- On Windows Task Scheduler, load secrets in a wrapper `.ps1` (`$env:OPENAI_API_KEY = Get-Secret openai`) and `exec` airon from that script — do not put secrets in the scheduled task action.
- For local dev, use a `.env` file that is already git-ignored, and load it with `direnv`, `dotenv`, or an equivalent.

## 2. Filesystem layout

Everything airon reads and writes is either the config directory or the sandboxed workspace.

```
<job-dir>/
├── airon.yaml                           # your job
├── skills/<name>/SKILL.md               # optional
└── workspace/                           # sandboxed runtime area
    ├── logs/<run_id>.json               # one JSON per run (see "Run logs")
    └── outputs/<step_id>.md             # per-step outputs when save_outputs: true
```

The workspace path comes from `workspace.path` in the config and is resolved relative to the config file. Built-in `fs_*` tools cannot escape it (symlink escapes are rejected).

## 3. Run logs

Every `Run` writes `workspace/logs/<run_id>.json` with:

- `run_id`, `config_path`, `started_at`, `finished_at`
- Rendered input variables
- Every step: id, rendered prompt, output, tool calls (with duration + result/error), rounds, per-step token usage
- Aggregate token usage (`prompt_tokens`, `completion_tokens`, `total_tokens`, `by_model`)
- `error` (populated on failure — contains the primary run error)

Stderr also emits one structured `slog` line per completed step (`step`, `rounds`, `tool_calls`, `duration`, tokens) — perfect for log shippers.

Retention is entirely up to you: nothing in airon prunes the workspace. A daily `find <workspace>/logs -mtime +30 -delete` job is common.

## 4. Notifications (healthchecks.io)

Enable per job by setting `notify.healthchecks.url` in the YAML config. airon then:

1. `GET <url>/start` before the executor runs.
2. `GET <url>` on success, or `POST <url>/fail` with the primary error text on failure.

Disable in three ways:

- Leave `notify.healthchecks.url` empty (default).
- Pass `--no-healthchecks` to the CLI.
- Set `AIRON_HEALTHCHECKS=0` (or `false` / `no` / `off`) in the environment.

**HTTP ping failures never override the run outcome.** A healthchecks.io outage will not fail an otherwise-successful run; the notifier error is logged at `WARN` and the primary result is preserved. This means alerts stay accurate even if the check-in service is down.

### Alerting recipe

- Create one healthchecks.io check per job. Grace period ≥ expected run time + 20%.
- Route notifications to whichever pager/chat integration you use.
- For jobs that run multiple times per day, use the same UUID — healthchecks tracks the schedule.
- When you disable the notifier (e.g. during a maintenance window), remember to pause the check on the healthchecks side so the missing pings do not fire spurious alerts.

## 5. Failure triage

When a run fails, follow this order:

1. **Read stderr.** airon prints the primary error last (`runtime error: ...`). CLI exit code disambiguates:
   - `0` — success.
   - `1` — runtime error (config load, provider handshake, MCP dial, executor error).
   - `2` — usage error (bad flags).
2. **Open the run log.** `workspace/logs/<run_id>.json` has the full history: which step failed, what the LLM said, what tools were called, and any provider error text.
3. **Check tool call errors.** In the step record, individual `tool_calls[*].error` entries pinpoint MCP or skill failures.
4. **Reproduce with `--no-healthchecks`.** Rerunning locally with the notifier off avoids polluting the healthchecks history while you iterate.
5. **Stream output for tricky flows.** Add `--stream` to see assistant deltas in real time when a step hangs or loops.

### Common failure patterns

| Symptom | Likely cause | Fix |
|---|---|---|
| `config: missing environment variable X` | Job references `${X}` but env is not set | Populate env in the systemd unit / scheduled task / `.env`. |
| `runtime error: dial tcp ...: connection refused` | MCP server is down | Restart the MCP container / process, or fix the URL. |
| `unknown tool "foo"` at load | `prompts[*].tools` references a missing tool | Verify skill loaded, agent id matches, or MCP `<server>__<tool>` namespace is correct. |
| Step never finishes / hits `max_tool_rounds` | LLM caught in a tool loop | Tighten the system prompt, reduce tool scope for that step, or lower `max_tool_rounds` while debugging. |
| Healthchecks flapping / paged missed run | Job crashed before `Start` ping, or notifier disabled | Check exit code + logs. If disabled intentionally, pause the healthchecks check. |
| Provider 429 / rate-limit | Provider throttling | Add retry/backoff at the wrapper level, or reduce concurrency across jobs. |

### Cleaning up after a bad run

- Remove `workspace/outputs/<step_id>.md` if a corrupted output is being picked up by a downstream template on the next run.
- Do **not** delete `workspace/logs/*.json` while triaging — you need them.
- If the workspace itself is compromised (external tool wrote outside the sandbox), airon rejects the write; still, delete and recreate the workspace directory to be safe.

## 6. Upgrading & rollbacks

- airon is a single Go binary. Ship it via your existing pipeline (`go build -o bin/airon ./cmd/airon/`) and swap the binary atomically.
- Configs are backward compatible on the `version: "1"` schema; new fields default to safe values.
- To roll back, restore the previous binary and (if needed) the previous config from your VCS.

## 7. Cron embedding cheat sheet

- **Shell + binary**: `airon run --config /etc/airon/job.yaml --no-healthchecks` when running from CI/manual, drop the flag in real cron so pings happen.
- **Embedded Go wrapper**: prefer this in production. See "Embedding airon as a library" in the [README](../../README.md#embedding-airon-as-a-library-pkgairon). Advantages: keeps secrets in-process, lets you branch on `Result.Success` and `Result.Steps[*].Error`, avoids the fork/exec cost.
- **Windows service**: wrap the embedded Go binary as a Windows service via `nssm` or `sc.exe create` — do not put `airon.exe` directly in a scheduled task if you also want structured secrets handling.
