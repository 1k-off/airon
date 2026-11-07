# Design: airon framework hardening + autonomous backup verification

**Date:** 2026-09-22  
**Status:** Approved (brainstorm decisions locked; remaining sections compacted for delivery)  
**Repos:** `airon` (framework), `onsite-infrastructure/util/backup-check` (first job)  
**Immutable:** `util/configgen` — read-only consumer; no edits in v1

## Decisions (locked)

| Topic | Choice |
|-------|--------|
| Framework | Evolve existing airon (A→B): keep runtime, stabilize public API |
| Job location | `onsite-infrastructure/util/backup-check/` |
| Verification depth | Selective DB restore + journal/registry |
| Operation | Fully autonomous `airon run` (external cron) |
| Catalog | Parse generated configgen artifacts + overlay; future optional configgen inventory exporter |
| AI role | LLM orchestrator; MCP tools deterministic; AI adapts restore/SQL on errors |
| Runtime topology | MCP + SQL Server in Docker; airon runs externally over HTTP MCP |
| Reporting | Run JSON/MD + `registry.json` + Healthchecks ping; `--no-healthchecks` / env disable for tests |
| Secrets | Prod: MCP container env; Dev: optional overrides from configgen artifacts |
| Batch | Exactly **1 database per run** |

## Architecture

```
[cron / operator]
       │
       ▼
   airon run  ──HTTP MCP──►  backup-check-mcp  ──►  SQL Server (Docker)
       │                         │
       │                         ├── S3 / restic download
       │                         ├── decrypt / restore (bacpac|bak|bak_zip)
       │                         └── schema inspect + SQL
       ▼
  workspace + reports/runs + registry.json
       │
       └── Healthchecks ping (optional)
```

**Approach:** Thin job + fat MCP. Domain logic lives in `util/backup-check`. airon stays a universal AI job runner.

## Part A — airon (framework)

### Goals

1. Keep current YAML DAG / skills / agents / MCP / workspace / runlog behavior.
2. Expose a small **public** Go API for embed and tests (`pkg/airon`).
3. First-class optional Healthchecks notification with hard disable for tests.
4. Docs + runbooks for writing and operating AI jobs.

### Public API (`pkg/airon`)

```go
package airon

type Options struct {
    ConfigPath     string
    Input          map[string]string
    Stream         bool
    NoHealthchecks bool // also honored if AIRON_HEALTHCHECKS=0
}

type Result struct {
    RunID      string
    Success    bool
    Err        error
    // step summaries / usage mirrored from executor
}

func Run(ctx context.Context, opts Options) (Result, error)
```

CLI remains thin wrapper:

```bash
airon run --config PATH [--input TEXT] [--stream] [--var k=v] [--no-healthchecks]
```

Exit codes unchanged: `0` success, `1` runtime/job fail, `2` usage.

### Config addition

```yaml
notify:
  healthchecks:
    url: ${BACKUP_CHECK_HEALTHCHECKS_URL}  # empty => no-op
```

Ping semantics: `/start` before run (optional), success URL on ok, `/fail` on error. Disabled when `--no-healthchecks`, `AIRON_HEALTHCHECKS=0`, or URL empty.

### Non-goals (airon v1)

- Built-in cron/daemon
- Parallel DAG steps
- Skills with nested tools
- Backup-domain code inside airon

### Docs / runbooks (airon)

- README updates: Job contract, public API, notify block, disable flag
- `docs/runbooks/writing-a-job.md` — how to author YAML + skills + MCP
- `docs/runbooks/operating-airon.md` — cron, secrets, exit codes, logs
- Note: if `docs/` was gitignored, allow `docs/superpowers/` and `docs/runbooks/`

## Part B — backup-check (first job)

### Layout

```
util/backup-check/
  README.md
  RUNBOOK.md
  catalog/
    databases.yaml          # generated
    overlay.yaml            # hand: skip, priority, notes
    schemas/catalog.schema.json
  cmd/catalog-gen/          # parses util/configgen/configs/{restic,gobackup}
  mcp/                      # Go module: HTTP MCP server
  docker/                   # compose: mssql + mcp
  job/
    airon.yaml
    skills/...
    prompts/...
  reports/
    registry.json
    runs/
    schemas/
  docs/
```

### Catalog

- **Source:** generated restic.yml + gobackup docker-compose.yml under `util/configgen/configs/` (must run configgen first).
- **Unit:** one database / restoreable artifact identity.
- **Fields (minimum):** `id`, `project`, `provider` (`restic`|`gobackup`), `format` (`bacpac`|`bak`|`bak_zip`|`archive`), repo/prefix, include path, schedule mode, secret key refs, `encrypted`.
- **Overlay:** skip list, priority, human notes — merged by `catalog-gen`.
- **Future:** configgen may emit inventory natively; keep catalog schema stable so switch is drop-in.

### Registry / journal

`reports/registry.json`:

```json
{
  "updated_at": "RFC3339",
  "databases": {
    "<catalog_id>": {
      "last_verified_at": "RFC3339",
      "last_run_id": "...",
      "status": "ok|fail",
      "error": ""
    }
  }
}
```

Selection rule for `pick_next`: prefer never-verified; then oldest `last_verified_at`; respect overlay skip; one ID only.

### MCP tools

| Tool | Behavior |
|------|----------|
| `registry_get` | Read registry |
| `catalog_list` | List catalog entries (filterable) |
| `pick_next` | Deterministic next candidate |
| `list_snapshots` | Restic snapshots or S3 list under prefix |
| `download_backup` | Download (+ decrypt if needed) into workspace |
| `restore_backup` | Restore into ephemeral SQL DB name `verify_<id>_<ts>` |
| `inspect_schema` | Tables, columns, approximate counts |
| `query_sql` | Read-only SQL (guard: block DROP/ALTER of non-verify DBs) |
| `execute_sql` | Limited writes for restore fixes inside verify DB only |
| `cleanup` | Drop verify DBs + wipe workspace files for run |
| `write_report` | Persist run JSON (+ optional MD) |
| `update_registry` | Upsert registry entry |

Secrets resolution: env on MCP (`S3_*`, `RESTIC_PASSWORD_<KEY>`, `GOBACKUP_DECRYPT_<KEY>`, SQL sa). Tool args may pass password overrides for local/dev only.

### AI job workflow (`job/airon.yaml`)

Single autonomous pipeline (conceptually one agent-heavy step or short DAG):

1. Load catalog + registry via MCP; call `pick_next` (override only with written reason).
2. `list_snapshots` → choose newest suitable artifact.
3. `download_backup` → `restore_backup`. On failure: read error, adapt (retry params, alternate format path, pre-SQL fixes) up to `runtime.max_tool_rounds`.
4. `inspect_schema` → craft DB-specific smoke checks (not universal SQL). Persist check list in report.
5. `write_report` + `update_registry` + `cleanup` (cleanup always, even on fail — agent instructed; MCP may also auto-cleanup on disconnect best-effort).
6. Job exit non-zero if verification failed after retries → Healthchecks fail ping.

Default batch size: **1**.

### Docker

- `mssql` container (Developer/Express acceptable for verify).
- `backup-check-mcp` exposing streamable HTTP (e.g. `:8080/mcp`).
- Shared volume for workspace artifacts; reports may bind-mount repo `reports/`.
- Resource note: sequential restores; target ~8–16 GB host RAM.

### Testing

| Layer | What |
|-------|------|
| MCP unit | fakes for S3/restic/sql/exec; no live cloud |
| catalog-gen | fixtures from sample restic/gobackup YAML |
| airon notify | URL empty / `--no-healthchecks` no-op; mock HTTP for ping |
| Integration | docker compose + synthetic bacpac/bak fixtures |
| E2E (manual/CI opt-in) | live S3 + LLM — tagged, not default CI |

### Runbooks (`util/backup-check`)

1. **Bootstrap:** generate configgen configs, place secrets into MCP env, `docker compose up`.
2. **Cron:** schedule `airon run --config util/backup-check/job/airon.yaml` from host that can reach MCP URL; LLM API key on airon host.
3. **Dev:** `--no-healthchecks`; local MCP; overlay skip for large DBs.
4. **Failure triage:** read latest `reports/runs/*.json`, MCP logs, SQL error text; re-run same id via overlay priority / var.
5. **Coverage:** track `registry.json` until catalog exhausted; then re-verify by age.

## Error handling

- Restore/SQL failures are **expected** agent-recovery paths, not instant job abort — until round budget exhausted.
- After budget: mark registry `fail`, write report with error, cleanup, non-zero exit.
- Never mutate production servers or production backup objects (read-only S3/restic download).
- SQL guards: only touch `verify_*` databases.

## Success criteria

- `airon` public API + healthchecks disable covered by tests; docs/runbooks present.
- `catalog-gen` builds catalog from generated configs without editing configgen.
- Autonomous single-DB verify loop works against Docker stack with fixtures.
- Reports + registry updated each run; Healthchecks optional.
- Operator can follow RUNBOOK end-to-end without reading source.

## Out of scope (v1)

- Editing `util/configgen` source
- Multi-DB per run
- Full schema diff vs production
- UI / dashboard
- Automatic configgen inventory exporter (documented as future)
- Slack/email (Healthchecks only)
