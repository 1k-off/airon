# Contributing to airon

Thanks for helping improve airon. Keep changes focused and tested.

## Development

```bash
git clone https://github.com/1k-off/airon.git
cd airon
go test ./...
make ci          # Linux/macOS: unit + race + 80% coverage + build
```

On Windows: `powershell -File scripts/ci.ps1` (race needs a CGO-capable toolchain).

## Guidelines

- Public surface lives in `pkg/airon` and the `airon` CLI. Prefer extending those over exporting new `internal/` packages.
- Domain-specific job logic (backups, business workflows, etc.) belongs in consumer repos — not in airon.
- Add or update tests with every behavior change. Prefer fakes over live network calls.
- Live provider E2E (`make test-integration`) is opt-in and requires API keys; do not make CI depend on it by default.
- Run `gofmt` / `go test ./...` before opening a PR.

## Commit style

Short imperative subject; explain *why* in the body when the change is non-obvious.
