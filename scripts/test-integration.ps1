param(
    [switch]$Stream
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$run = "TestE2E_OpenAI"
if ($Stream) {
    $run += "|TestE2E_OpenAI_streaming"
}

if ($env:OPENAI_API_KEY) {
    Write-Host "==> OpenAI E2E" -ForegroundColor Cyan
    go test -tags=integration ./internal/e2e/... -run $run -count=1 -v
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
    Write-Host "SKIP OpenAI E2E: OPENAI_API_KEY not set" -ForegroundColor Yellow
}

$run = "TestE2E_Anthropic"
if ($Stream) {
    $run += "|TestE2E_Anthropic_streaming"
}

if ($env:ANTHROPIC_API_KEY) {
    Write-Host "==> Anthropic E2E" -ForegroundColor Cyan
    go test -tags=integration ./internal/e2e/... -run $run -count=1 -v
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
    Write-Host "SKIP Anthropic E2E: ANTHROPIC_API_KEY not set" -ForegroundColor Yellow
}
