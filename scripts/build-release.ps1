# Cross-compile the KoraDB-server and KoraDB CLI for all supported platforms.
# Produces standalone static binaries (CGO_ENABLED=0) under dist/ — each one has
# zero external/runtime dependencies and needs nothing installed on the target.

param(
  [switch]$Development,
  [string]$Version = "dev",
  [string]$Commit = "unknown"
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
$buildTime = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-X KoraDB/internal/buildinfo.Version=$Version -X KoraDB/internal/buildinfo.Commit=$Commit -X KoraDB/internal/buildinfo.BuildTime=$buildTime"

$targets = @(
  @{os = "linux";   arch = "amd64"; ext = "" },
  @{os = "linux";   arch = "arm64"; ext = "" },
  @{os = "darwin";  arch = "amd64"; ext = "" },
  @{os = "darwin";  arch = "arm64"; ext = "" },
  @{os = "windows"; arch = "amd64"; ext = ".exe" }
)

New-Item -ItemType Directory -Force -Path dist | Out-Null
$artifacts = @()
foreach ($t in $targets) {
  $env:GOOS = $t.os; $env:GOARCH = $t.arch
  foreach ($cmd in @("KoraDB-server", "KoraDB")) {
    $out = "dist\$cmd-$($t.os)-$($t.arch)$($t.ext)"
    & $go build -ldflags $ldflags -o $out "./cmd/$cmd"
    $artifacts += $out
    Write-Host "built $out"
  }
}

$checksums = foreach ($artifact in $artifacts) {
  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $artifact
  "{0}  {1}" -f $hash.Hash.ToLowerInvariant(), [System.IO.Path]::GetFileName($artifact)
}
$checksumPath = "dist\checksums.txt"
$checksums | Sort-Object | Set-Content -LiteralPath $checksumPath -NoNewline:$false
Write-Host "wrote $checksumPath"
Write-Host "`nAll binaries are static (CGO_ENABLED=0) — verify checksums before copying one to the target."
