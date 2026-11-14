param(
    [int]$Threshold = 80
)

$ErrorActionPreference = "Stop"

$output = go test -cover ./internal/... 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host $output
    exit 1
}

$failed = $false
$checked = 0

foreach ($line in $output) {
    $text = [string]$line
    if ($text -notmatch 'coverage:\s+([\d.]+)%') {
        continue
    }

    $pct = [double]$Matches[1]
    $pkg = ($text -split '\s+')[1]
    $checked++

    if ($pct -lt $Threshold) {
        Write-Host "FAIL $pkg ${pct}% < ${Threshold}%"
        $failed = $true
    } else {
        Write-Host "OK   $pkg ${pct}%"
    }
}

if ($checked -eq 0) {
    Write-Host "Coverage gate failed: no packages measured"
    Write-Host $output
    exit 1
}

if ($failed) {
    Write-Host "Coverage gate failed (threshold ${Threshold}%)"
    exit 1
}

Write-Host "Coverage gate passed (threshold ${Threshold}%)"
