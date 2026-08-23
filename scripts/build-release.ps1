# Cross-compile the KoraDB-server and KoraDB CLI for all supported platforms.
# Produces standalone static binaries (CGO_ENABLED=0) under dist/ — each one has
# zero external/runtime dependencies and needs nothing installed on the target.

param(
  [switch]$Development
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if ($Development) {
  Write-Warning "Development override enabled. These artifacts are NOT approved for public release."
} else {
  & (Join-Path $PSScriptRoot "check-release-identity.ps1")
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
}

$go = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "C:\Program Files\Go\bin\go.exe" }
$env:CGO_ENABLED = "0"

$targets = @(
  @{os = "linux";   arch = "amd64"; ext = "" },
  @{os = "linux";   arch = "arm64"; ext = "" },
  @{os = "darwin";  arch = "amd64"; ext = "" },
  @{os = "darwin";  arch = "arm64"; ext = "" },
  @{os = "windows"; arch = "amd64"; ext = ".exe" }
)

New-Item -ItemType Directory -Force -Path dist | Out-Null
foreach ($t in $targets) {
  $env:GOOS = $t.os; $env:GOARCH = $t.arch
  foreach ($cmd in @("KoraDB-server", "KoraDB")) {
    $out = "dist\$cmd-$($t.os)-$($t.arch)$($t.ext)"
    & $go build -o $out "./cmd/$cmd"
    Write-Host "built $out"
  }
}
Write-Host "`nAll binaries are static (CGO_ENABLED=0) — copy one to the target and run it."
