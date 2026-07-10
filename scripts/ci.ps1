param(
    [switch]$SkipRace
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

function Write-Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Test-RaceSupported {
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        $gcc = Get-Command gcc -ErrorAction SilentlyContinue
        if (-not $gcc) {
            return $false
        }
    }
    return $true
}

Write-Step "Unit tests"
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (-not $SkipRace) {
    Write-Step "Race detector"
    if (-not (Test-RaceSupported)) {
        Write-Host "SKIP race tests: gcc not found on Windows." -ForegroundColor Yellow
        Write-Host "Install MinGW-w64 (gcc) or run race checks in GitHub Actions CI." -ForegroundColor Yellow
    } else {
        $env:CGO_ENABLED = "1"
        go test -race ./... -count=1
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

Write-Step "Coverage gate (80%)"
& "$PSScriptRoot\check-coverage.ps1" -Threshold 80
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Step "Build"
New-Item -ItemType Directory -Force -Path "$repoRoot\bin" | Out-Null
go build -o "$repoRoot\bin\airon.exe" ./cmd/airon/
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "CI checks passed." -ForegroundColor Green
