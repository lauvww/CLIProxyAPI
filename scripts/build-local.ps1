$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$versionFile = Join-Path $repoRoot 'VERSION'

if (-not (Test-Path $versionFile)) {
  throw "VERSION file not found: $versionFile"
}

$version = (Get-Content -Raw $versionFile).Trim()
if (-not $version) {
  throw "VERSION file is empty: $versionFile"
}

$commit = 'none'
try {
  $commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
} catch {
  $commit = 'none'
}

$buildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$output = Join-Path $repoRoot 'cli-proxy-api.exe'

Push-Location $repoRoot
try {
  go build -ldflags "-s -w -X main.Version=$version -X main.Commit=$commit -X main.BuildDate=$buildDate" -o $output ./cmd/server
  Write-Host "Built $output"
  Write-Host "Version: $version"
  Write-Host "Commit: $commit"
  Write-Host "BuildDate: $buildDate"
} finally {
  Pop-Location
}
