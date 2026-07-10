.PHONY: test test-race test-cover test-integration build ci

test:
	go test ./... -count=1

# Race needs CGO. On Windows also install MinGW-w64 (gcc) or use: powershell -File scripts/ci.ps1
test-race:
	CGO_ENABLED=1 go test -race ./... -count=1

test-cover:
ifeq ($(OS),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check-coverage.ps1 -Threshold 80
else
	bash scripts/check-coverage.sh 80
endif

test-integration:
	go test -tags=integration ./internal/e2e/... -count=1 -v

build:
	go build -o bin/airon ./cmd/airon/

ifeq ($(OS),Windows_NT)
ci:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci.ps1
else
ci: test test-race test-cover build
endif
