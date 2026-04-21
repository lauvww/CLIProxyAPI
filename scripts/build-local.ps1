param(
  [string]$OutputPath,
  [string]$GoBinary
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$workspaceRoot = Split-Path -Parent $repoRoot
$versionFile = Join-Path $repoRoot 'VERSION'

function Resolve-GoBinary {
  param(
    [string]$ExplicitPath,
    [string]$WorkspaceRoot
  )

  $candidates = @()
  if ($ExplicitPath) {
    $candidates += $ExplicitPath
  }

  if ($WorkspaceRoot) {
    $candidates += (Join-Path $WorkspaceRoot '.tools\go1.26.2\go\bin\go.exe')
  }

  $command = Get-Command go -ErrorAction SilentlyContinue
  if ($command -and $command.Source) {
    $candidates += $command.Source
  }

  foreach ($candidate in $candidates) {
    if (-not $candidate) {
      continue
    }

    $resolved = $candidate
    if (-not [System.IO.Path]::IsPathRooted($resolved)) {
      try {
        $resolved = (Resolve-Path -LiteralPath $resolved).Path
      } catch {
        continue
      }
    }

    if (Test-Path -LiteralPath $resolved) {
      return $resolved
    }
  }

  throw 'Unable to locate go executable. Pass -GoBinary or install Go.'
}

function Resolve-IconSource {
  param(
    [string]$WorkspaceRoot
  )

  $candidates = @(
    (Join-Path $WorkspaceRoot 'image\\logo.ico'),
    (Join-Path $WorkspaceRoot 'image-logo.ico')
  )

  foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path -LiteralPath $candidate)) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }

  return ''
}

function Resolve-RsrcBinary {
  param(
    [string]$GoBinary
  )

  $candidates = @()
  $command = Get-Command rsrc -ErrorAction SilentlyContinue
  if ($command -and $command.Source) {
    $candidates += $command.Source
  }

  try {
    $goPath = (& $GoBinary env GOPATH).Trim()
    if ($goPath) {
      $candidates += (Join-Path $goPath 'bin\\rsrc.exe')
    }
  } catch {
  }

  foreach ($candidate in $candidates) {
    if (-not $candidate) {
      continue
    }
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }

  return ''
}

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
$goExe = Resolve-GoBinary -ExplicitPath $GoBinary -WorkspaceRoot $workspaceRoot
$output = if ($OutputPath) { $OutputPath } else { Join-Path $repoRoot 'cli-proxy-api.exe' }
$iconSource = Resolve-IconSource -WorkspaceRoot $workspaceRoot
$rsrcExe = Resolve-RsrcBinary -GoBinary $goExe

if (-not [System.IO.Path]::IsPathRooted($output)) {
  $output = Join-Path $repoRoot $output
}

$outputDir = Split-Path -Parent $output
if ($outputDir) {
  New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
}

Push-Location $repoRoot
try {
  if ($iconSource) {
    $embeddedIconPath = Join-Path $repoRoot 'cmd\\server\\app.ico'
    Copy-Item -LiteralPath $iconSource -Destination $embeddedIconPath -Force

    if ($rsrcExe) {
      & $rsrcExe -ico $embeddedIconPath -arch amd64 -o (Join-Path $repoRoot 'cmd\\server\\app_windows_amd64.syso')
      Write-Host "IconSource: $iconSource"
      Write-Host "RsrcBinary: $rsrcExe"
    } else {
      Write-Host "IconSource: $iconSource"
      Write-Host "RsrcBinary: not found, keeping existing .syso resource"
    }
  }

  & $goExe build -ldflags "-s -w -X main.Version=$version -X main.Commit=$commit -X main.BuildDate=$buildDate -X main.DefaultLocalModel=true" -o $output ./cmd/server
  Write-Host "Built $output"
  Write-Host "Version: $version"
  Write-Host "Commit: $commit"
  Write-Host "BuildDate: $buildDate"
  Write-Host "GoBinary: $goExe"
} finally {
  Pop-Location
}
