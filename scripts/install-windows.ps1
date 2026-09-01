param(
  [Parameter(Mandatory = $true)]
  [string]$PackagePath,
  [Parameter(Mandatory = $true)]
  [string]$SHA256,
  [string]$Destination = (Join-Path $env:ProgramFiles "KoraDB"),
  [switch]$Force
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$package = [System.IO.Path]::GetFullPath($PackagePath)
if (-not (Test-Path -LiteralPath $package -PathType Leaf)) {
  throw "Installer package not found: $package"
}
if ([System.IO.Path]::GetExtension($package) -ne ".zip") {
  throw "Installer package must be a .zip archive: $package"
}

$actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $package).Hash.ToLowerInvariant()
if ($actual -ne $SHA256.ToLowerInvariant()) {
  throw "SHA-256 mismatch for $package. Expected $SHA256, got $actual."
}

$destinationPath = [System.IO.Path]::GetFullPath($Destination)
$parent = Split-Path -Parent $destinationPath
if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
  New-Item -ItemType Directory -Force -Path $parent | Out-Null
}
if (Test-Path -LiteralPath $destinationPath) {
  if (-not $Force) {
    throw "Destination already exists: $destinationPath. Re-run with -Force only after taking a verified database backup."
  }
  $backup = "$destinationPath.previous-" + [DateTime]::UtcNow.ToString("yyyyMMddHHmmss")
  Move-Item -LiteralPath $destinationPath -Destination $backup
  Write-Host "Moved previous installation to $backup"
}

$stage = Join-Path $parent (".KoraDB-install-" + [guid]::NewGuid().ToString("N"))
try {
  Expand-Archive -LiteralPath $package -DestinationPath $stage -Force
  foreach ($required in @("KoraDB.exe", "KoraDB-server.exe", "KoraDB-restore.exe", "release-manifest.json")) {
    if (-not (Test-Path -LiteralPath (Join-Path $stage $required) -PathType Leaf)) {
      throw "Installer package is missing $required"
    }
  }
  Move-Item -LiteralPath $stage -Destination $destinationPath
  Write-Host "Installed KoraDB to $destinationPath"
  Write-Host "Add $destinationPath to PATH to use KoraDB commands."
  Write-Host "Configure TLS and an administrator key before registering KoraDB-server as a Windows service."
} catch {
  if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
  }
  throw
}
