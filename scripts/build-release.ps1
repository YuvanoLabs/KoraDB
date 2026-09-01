# Cross-compile the KoraDB server, CLI, and restore utility for all supported
# platforms, then produce verified portable installer archives.
# Produces standalone static binaries (CGO_ENABLED=0) under dist/ — each one has
# zero external/runtime dependencies and needs nothing installed on the target.

param(
  [switch]$Development,
  [string]$Version = "dev",
  [string]$Commit = "unknown",
  [string]$BuildTime = "",
  [string]$OutputDirectory = "dist",
  [string]$NativeCompiler = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if ($Development) {
  Write-Warning "Development override enabled. These artifacts are NOT approved for public release."
} else {
	if ($Version -eq "dev" -or $Commit -eq "unknown" -or [string]::IsNullOrWhiteSpace($BuildTime)) {
		throw "Public releases require explicit -Version, -Commit, and -BuildTime values."
	}
  & (Join-Path $PSScriptRoot "check-release-identity.ps1")
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
}

$go = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "C:\Program Files\Go\bin\go.exe" }
if (-not (Get-Command $go -ErrorAction SilentlyContinue) -and -not (Test-Path -LiteralPath $go -PathType Leaf)) {
  throw "Go toolchain not found. Install Go or put go on PATH."
}
$originalCgoEnabled = $env:CGO_ENABLED
$originalCC = $env:CC
$originalGoCache = $env:GOCACHE
$env:CGO_ENABLED = "0"
if ([string]::IsNullOrWhiteSpace($env:GOCACHE)) {
  $env:GOCACHE = Join-Path $root ".cache\go-build"
}
$originalGoos = $env:GOOS
$originalGoarch = $env:GOARCH
try {
  if ([string]::IsNullOrWhiteSpace($BuildTime)) {
    $BuildTime = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
  } else {
    $BuildTime = [DateTime]::Parse($BuildTime).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  }

$ldflags = "-X github.com/YuvanoLabs/KoraDB/internal/buildinfo.Version=$Version -X github.com/YuvanoLabs/KoraDB/internal/buildinfo.Commit=$Commit -X github.com/YuvanoLabs/KoraDB/internal/buildinfo.BuildTime=$buildTime"

$targets = @(
  @{os = "linux";   arch = "amd64"; ext = "" },
  @{os = "linux";   arch = "arm64"; ext = "" },
  @{os = "darwin";  arch = "amd64"; ext = "" },
  @{os = "darwin";  arch = "arm64"; ext = "" },
  @{os = "windows"; arch = "amd64"; ext = ".exe" }
)

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$artifacts = @()
foreach ($t in $targets) {
  $env:GOOS = $t.os; $env:GOARCH = $t.arch
  $targetName = "$($t.os)-$($t.arch)"
  $targetDirectory = Join-Path $OutputDirectory $targetName
  New-Item -ItemType Directory -Force -Path $targetDirectory | Out-Null
  foreach ($cmd in @("KoraDB-server", "KoraDB", "KoraDB-restore")) {
    $out = Join-Path $targetDirectory "$cmd$($t.ext)"
    & $go build -trimpath -buildvcs=false -ldflags $ldflags -o $out "./cmd/$cmd"
    if ($LASTEXITCODE -ne 0) {
      throw "Build failed for $cmd on $targetName."
    }
    $artifacts += $out
    Write-Host "built $out"
  }
}

# The developer package/DLL delivery path is produced on its native Windows
# builder. Cross-compiling a c-shared Go library is not supported by the Go
# toolchain, so other native targets must be built and verified on matching
# runners before they are declared supported.
$nativeStage = $null
if ($env:OS -eq "Windows_NT") {
  $compiler = $NativeCompiler
  if ([string]::IsNullOrWhiteSpace($compiler)) { $compiler = $env:CC }
  if ([string]::IsNullOrWhiteSpace($compiler) -and (Get-Command gcc -ErrorAction SilentlyContinue)) {
    $compiler = (Get-Command gcc).Source
  }
  if ([string]::IsNullOrWhiteSpace($compiler) -or -not (Test-Path -LiteralPath $compiler -PathType Leaf)) {
    throw "Native DLL packaging requires a GNU C compiler. Set -NativeCompiler or CC to gcc.exe."
  }
  $nativeStage = Join-Path $OutputDirectory "native-windows-amd64"
  New-Item -ItemType Directory -Force -Path $nativeStage | Out-Null
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "1"; $env:CC = $compiler
  $nativeLibrary = Join-Path $nativeStage "KoraDB-native.dll"
  & $go build -trimpath -buildvcs=false -buildmode=c-shared -ldflags $ldflags -o $nativeLibrary ./sdk/native/cshared
  if ($LASTEXITCODE -ne 0) {
    throw "Native DLL build failed."
  }
  $generatedNativeHeader = Join-Path $nativeStage "KoraDB-native.h"
  if (Test-Path -LiteralPath $generatedNativeHeader) {
    Remove-Item -LiteralPath $generatedNativeHeader -Force
  }
  Copy-Item -LiteralPath (Join-Path $root "sdk\native\include\koradb.h") -Destination (Join-Path $nativeStage "koradb.h") -Force
  $artifacts += $nativeLibrary
  $artifacts += (Join-Path $nativeStage "koradb.h")
  Write-Host "built $nativeLibrary and public header"
}

$checksums = foreach ($artifact in $artifacts) {
  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $artifact
  "{0}  {1}" -f $hash.Hash.ToLowerInvariant(), [System.IO.Path]::GetFileName($artifact)
}
$checksumPath = Join-Path $OutputDirectory "checksums.txt"
$checksums | Sort-Object | Set-Content -LiteralPath $checksumPath -NoNewline:$false
Write-Host "wrote $checksumPath"

$manifest = [ordered]@{
  product = "KoraDB"
  version = $Version
  commit = $Commit
  build_time = $BuildTime
  format_version = 1
  artifacts = @(
    foreach ($artifact in $artifacts | Sort-Object) {
      $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $artifact
      [ordered]@{
        path = $artifact.Substring($OutputDirectory.Length).TrimStart([char[]]@('\', '/'))
        sha256 = $hash.Hash.ToLowerInvariant()
      }
    }
  )
}
$manifestPath = Join-Path $OutputDirectory "release-manifest.json"
$manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -NoNewline:$false

$archives = @()
foreach ($t in $targets) {
  $targetName = "$($t.os)-$($t.arch)"
  $stage = Join-Path $OutputDirectory $targetName
  $archive = Join-Path $OutputDirectory "KoraDB-$Version-$targetName.zip"
  if (Test-Path -LiteralPath $archive) {
    Remove-Item -LiteralPath $archive -Force
  }
  $archiveInputs = @(
    Get-ChildItem -LiteralPath $stage -File | ForEach-Object { $_.FullName }
  )
  $archiveInputs += $manifestPath
  Compress-Archive -LiteralPath $archiveInputs -DestinationPath $archive -CompressionLevel Optimal
  $archives += $archive
  Write-Host "packaged $archive"
}
if ($null -ne $nativeStage) {
  $nativeArchive = Join-Path $OutputDirectory "KoraDB-native-$Version-windows-amd64.zip"
  if (Test-Path -LiteralPath $nativeArchive) {
    Remove-Item -LiteralPath $nativeArchive -Force
  }
  $nativeArchiveInputs = @(
    Get-ChildItem -LiteralPath $nativeStage -File | ForEach-Object { $_.FullName }
  )
  $nativeArchiveInputs += $manifestPath
  Compress-Archive -LiteralPath $nativeArchiveInputs -DestinationPath $nativeArchive -CompressionLevel Optimal
  $archives += $nativeArchive
  Write-Host "packaged $nativeArchive"
}

$archiveChecksums = foreach ($archive in $archives) {
  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $archive
  "{0}  {1}" -f $hash.Hash.ToLowerInvariant(), [System.IO.Path]::GetFileName($archive)
}
$archiveChecksums | Sort-Object | Set-Content -LiteralPath (Join-Path $OutputDirectory "archive-checksums.txt") -NoNewline:$false
Write-Host "`nAll binaries are static (CGO_ENABLED=0). Verify archive-checksums.txt before installation."
} finally {
  $env:GOOS = $originalGoos
  $env:GOARCH = $originalGoarch
  $env:CGO_ENABLED = $originalCgoEnabled
  $env:CC = $originalCC
  $env:GOCACHE = $originalGoCache
}
