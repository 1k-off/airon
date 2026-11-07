# airon + Backup Verify Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden `airon` into a maintainable AI-job framework (public API + optional Healthchecks) and ship autonomous backup verification in `onsite-infrastructure/util/backup-check` that restores one DB per run via HTTP MCP, journals coverage, and writes reports — without modifying `util/configgen`.

**Architecture:** Thin job + fat MCP. airon runs externally; Docker hosts SQL Server + `backup-check-mcp`. Catalog is generated from configgen *output* artifacts + overlay. LLM orchestrates adaptive restore/SQL; tools are deterministic.

**Tech Stack:** Go 1.22+, YAML, MCP streamable HTTP (`modelcontextprotocol/go-sdk`), Docker Compose, SQL Server container, restic CLI, sqlpackage/sqlcmd, OpenAI/Anthropic via existing airon providers.

## Global Constraints

- Do **not** edit `util/configgen/**` source (read generated `configs/` only).
- Exactly **1** database per airon run.
- Healthchecks must be disableable: `--no-healthchecks` and `AIRON_HEALTHCHECKS=0`.
- Secrets: prod in MCP env; optional per-call overrides for dev.
- SQL mutations only against `verify_*` databases.
- OLD BACKUP CHECKS is inspirational only — reimplement cleanly under `util/backup-check`.
- Prefer TDD; keep packages small and interfaces fakeable.
- Do not commit secrets; keep `.env` / `docker/secrets` gitignored.
- User did not request git commits automatically — create commits only if explicitly asked later; still leave the tree buildable and documented.

**Spec:** `docs/superpowers/specs/2026-09-22-airon-backup-verify-design.md` (mirrored in airon repo).

---

## File map

### airon repo (`D:\work\onsite\airon`)

| Path | Responsibility |
|------|----------------|
| `pkg/airon/airon.go` | Public `Run(ctx, Options) (Result, error)` |
| `pkg/airon/airon_test.go` | Public API tests with fake provider |
| `internal/notify/healthchecks.go` | Ping start/success/fail |
| `internal/notify/healthchecks_test.go` | Disable + HTTP mock tests |
| `internal/config/config.go` | Add `Notify` / healthchecks URL |
| `internal/bootstrap/app.go` | Wire notify around run |
| `cmd/airon/main.go` | `--no-healthchecks` flag |
| `docs/runbooks/*.md` | Framework runbooks |
| `README.md` | Document notify + public API |

### onsite-infrastructure (`D:\work\onsite\onsite-infrastructure`)

| Path | Responsibility |
|------|----------------|
| `util/backup-check/cmd/catalog-gen` | Build catalog from generated configs |
| `util/backup-check/catalog/*` | Schema, overlay, generated yaml |
| `util/backup-check/mcp/` | HTTP MCP server module |
| `util/backup-check/docker/` | Compose stack |
| `util/backup-check/job/` | airon.yaml + skills/prompts |
| `util/backup-check/reports/` | registry + schemas + runs |
| `util/backup-check/README.md` | Overview |
| `util/backup-check/RUNBOOK.md` | Ops runbook |

---

### Task 1: airon — Healthchecks notify package

**Files:**
- Create: `internal/notify/healthchecks.go`
- Create: `internal/notify/healthchecks_test.go`
- Modify: `internal/config/config.go` (add Notify struct)
- Modify: `internal/config/load.go` / validate if needed

**Interfaces:**
- Produces: `notify.Healthchecks` with `Start(ctx)`, `Success(ctx)`, `Fail(ctx, body string)`, constructed via `notify.NewHealthchecks(url string, disabled bool)`

- [ ] **Step 1: Write failing tests** for no-op when URL empty or disabled; success/fail HTTP calls when enabled (httptest).

- [ ] **Step 2: Implement** Healthchecks client using `HEAD`/`POST` compatible with healthchecks.io style URLs (`/start`, base, `/fail`).

- [ ] **Step 3: Extend config**

```go
type NotifyConfig struct {
    Healthchecks HealthchecksConfig `yaml:"healthchecks"`
}
type HealthchecksConfig struct {
    URL string `yaml:"url"`
}
// on Config:
Notify NotifyConfig `yaml:"notify"`
```

- [ ] **Step 4: Run** `go test ./internal/notify/... ./internal/config/...` — expect PASS.

---

### Task 2: airon — Wire notify + CLI flag + public API

**Files:**
- Modify: `internal/bootstrap/app.go`, `options.go`
- Modify: `cmd/airon/main.go`
- Create: `pkg/airon/airon.go`, `pkg/airon/airon_test.go`

**Interfaces:**
- Consumes: `notify.NewHealthchecks`
- Produces: `airon.Run(ctx, airon.Options) (airon.Result, error)`

- [ ] **Step 1: Add** `AppOptions.NoHealthchecks` and env `AIRON_HEALTHCHECKS=0` check in bootstrap; call Start before executor, Success/Fail after.

- [ ] **Step 2: CLI** `--no-healthchecks` passed into options.

- [ ] **Step 3: Public API** `pkg/airon` wrapping bootstrap (same behavior as CLI run path without os.Exit).

- [ ] **Step 4: Tests** — API run with injected mock provider; notify disabled.

- [ ] **Step 5: Run** `go test ./...` (unit, not integration) — PASS; coverage gate still holds.

---

### Task 3: airon — Docs & runbooks

**Files:**
- Modify: `README.md`, `.gitignore` (allow docs/runbooks + docs/superpowers)
- Create: `docs/runbooks/writing-a-job.md`, `docs/runbooks/operating-airon.md`
- Ensure design/plan copies under `docs/superpowers/`

- [ ] **Step 1: Document** notify block, `--no-healthchecks`, `pkg/airon`, exit codes, cron embedding.

- [ ] **Step 2: Writing-a-job runbook** — YAML DAG, skills, MCP HTTP example pointing at backup-check pattern.

- [ ] **Step 3: Operating runbook** — secrets, logs path, failure triage.

---

### Task 4: backup-check — Scaffold + report/registry schemas

**Files:**
- Create tree under `util/backup-check/` as in design
- Create: `reports/schemas/run-report.schema.json`, `registry.schema.json`
- Create: `reports/registry.json` empty seed
- Create: `catalog/schemas/catalog.schema.json`, `catalog/overlay.yaml`
- Create: `README.md` stub

- [ ] **Step 1: Scaffold directories** and schemas adapted from OLD (clean copy, paths updated).

- [ ] **Step 2: Empty registry** `{"updated_at":"","databases":{}}`.

---

### Task 5: backup-check — catalog-gen

**Files:**
- Create: `util/backup-check/cmd/catalog-gen/main.go`
- Create: `util/backup-check/internal/catalog/` (`parse_restic.go`, `parse_gobackup.go`, `merge.go`, tests + fixtures)

**Interfaces:**
- Produces: `catalog/databases.yaml` with entries implementing design fields
- Consumes: `util/configgen/configs/restic/**/restic.yml`, `util/configgen/configs/gobackup/**/docker-compose.yml`, `catalog/overlay.yaml`

- [ ] **Step 1: Fixtures** — minimal restic.yml + gobackup compose snippets under `internal/catalog/testdata/`.

- [ ] **Step 2: TDD parsers** — extract project/service, DB names from run-before scripts / databases sections, formats, prefixes, repo paths.

- [ ] **Step 3: CLI** writes merged YAML; `--configgen-root` and `--out` flags.

- [ ] **Step 4: Document** in README: must run `cd util/configgen && go run .` first (with env creds) so configs exist.

**Note:** If live configs absent in workspace, tests use fixtures only; CLI still works when configs present.

---

### Task 6: backup-check — MCP core (storage/restore/sql) with fakes

**Files:**
- Create module `util/backup-check/mcp/go.mod` (module path e.g. `github.com/onsite/backup-check-mcp` or match org)
- Packages: `internal/config`, `internal/workspace`, `internal/exec`, `internal/storage`, `internal/restore`, `internal/sql`, `internal/service`, `internal/mcpserver`, `cmd/backup-check-mcp`

**Interfaces (service layer):**
```go
ListSnapshots(ctx, req) (ListSnapshotsResult, error)
DownloadBackup(ctx, req) (DownloadBackupResult, error)
RestoreBackup(ctx, req) error
InspectSchema(ctx, db string) (SchemaInfo, error)
QuerySQL(ctx, req) (QueryResult, error)
ExecuteSQL(ctx, req) error
Cleanup(ctx) (CleanupResult, error)
// plus catalog/registry file helpers
```

- [ ] **Step 1: Workspace + exec runner** with tests.

- [ ] **Step 2: Storage interfaces** + fake implementations; restic/S3 real adapters behind interfaces (shell out to `restic` / AWS SDK or minio client).

- [ ] **Step 3: Restore** bacpac (sqlpackage), bak/bak_zip (sqlcmd/7z) — unit-tested with fake runner asserting commands.

- [ ] **Step 4: SQL guard** — reject statements targeting non-`verify_` DBs.

- [ ] **Step 5: Service orchestration** matching tool semantics; unit tests with fakes.

---

### Task 7: backup-check — MCP HTTP server tools

**Files:**
- `internal/mcpserver/server.go` + tests
- `cmd/backup-check-mcp/main.go`

- [ ] **Step 1: Register tools** listed in design (namespaced cleanly).

- [ ] **Step 2: Streamable HTTP** listen addr from env `MCP_ADDR` default `:8080`.

- [ ] **Step 3: Smoke test** tool list / one fake tool call in-process.

---

### Task 8: Docker compose stack

**Files:**
- `docker/docker-compose.yml`, `docker/README.md`, `.env.example`, `secrets/.gitkeep`

- [ ] **Step 1: Services** `mssql` + `backup-check-mcp` with volume mounts for workspace + reports + catalog.

- [ ] **Step 2: Document** required env vars (S3, restic passwords map, SA password, MCP bind).

- [ ] **Step 3: Healthcheck** on SQL ready before MCP starts (`depends_on` + wait script if needed).

---

### Task 9: airon job (YAML + skills) for autonomous verify

**Files:**
- `job/airon.yaml`
- `job/skills/backup-verifier/SKILL.md`
- `job/prompts/*.md` if split

- [ ] **Step 1: Config** provider from env; MCP `streamable_http` to `${BACKUP_CHECK_MCP_URL}`; high `max_tool_rounds` (e.g. 40); `notify.healthchecks.url` from env.

- [ ] **Step 2: Skill/system prompt** — mandate: pick_next → download → restore with on-error repair → inspect_schema → tailored checks → write_report → update_registry → cleanup; batch=1; never touch non-verify DBs.

- [ ] **Step 3: Example dry-run instructions** in README using `--no-healthchecks`.

---

### Task 10: Integration tests + end-to-end docs

**Files:**
- `mcp/internal/...` integration tests tagged `integration`
- `RUNBOOK.md`, polish `README.md`
- Optional: `scripts/run-verify.ps1` / `.sh` wrappers

- [ ] **Step 1: Compose-based test** (or documented manual) with fixture artifact if available; otherwise unit coverage + clear manual E2E checklist.

- [ ] **Step 2: RUNBOOK** — bootstrap, cron example, triage, coverage loop, disable healthchecks for tests.

- [ ] **Step 3: Cross-link** airon runbooks ↔ backup-check runbook.

- [ ] **Step 4: Final verification**
  - `cd airon && go test ./...`
  - `cd util/backup-check/mcp && go test ./...`
  - `cd util/backup-check && go test ./...` (catalog-gen packages)
  - Confirm configgen untouched: `git -C onsite-infrastructure status util/configgen` clean of intentional edits.

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| Public airon API | 2 |
| Healthchecks + disable | 1–2 |
| airon docs/runbooks | 3 |
| util/backup-check location | 4–10 |
| catalog from generated configs + overlay | 5 |
| MCP tools + restore formats | 6–7 |
| Docker SQL+MCP; airon external | 8–9 |
| 1 DB/run autonomous AI loop | 9 |
| registry + reports | 4, 6, 9 |
| secrets hybrid | 8–9 |
| configgen immutable | Global + Task 10 |
| future configgen inventory note | README Task 10 |

## Execution order

Execute Task 1→3 (airon) in parallel with Task 4→5 (scaffold/catalog) once Task 4 exists; then 6→10 sequentially on backup-check. Prefer feature branches `feat/airon-public-api-notify` and `feat/backup-check` if not already on a feature branch.
