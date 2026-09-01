# Build the Windows C shared library and exercise its public header from a C
# consumer. This validates the DLL/package delivery path, not merely Go source
# compilation. A GNU C compiler is required (for example MinGW-w64's gcc).
param(
  [string]$OutputDirectory = ".cache/native-abi"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$compiler = if ($env:CC) { $env:CC } elseif (Get-Command gcc -ErrorAction SilentlyContinue) { (Get-Command gcc).Source } else { $null }
if ([string]::IsNullOrWhiteSpace($compiler) -or -not (Test-Path -LiteralPath $compiler -PathType Leaf)) {
  throw "A GNU C compiler is required. Set CC to gcc.exe or install MinGW-w64."
}

$go = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "C:\Program Files\Go\bin\go.exe" }
if (-not (Get-Command $go -ErrorAction SilentlyContinue) -and -not (Test-Path -LiteralPath $go -PathType Leaf)) {
  throw "Go toolchain not found. Install Go or put go on PATH."
}

$absoluteOutput = [System.IO.Path]::GetFullPath((Join-Path $root $OutputDirectory))
New-Item -ItemType Directory -Force -Path $absoluteOutput | Out-Null
$originalCC = $env:CC
$originalCGOEnabled = $env:CGO_ENABLED
$originalGoCache = $env:GOCACHE
try {
  $env:CC = $compiler
  $env:CGO_ENABLED = "1"
  if ([string]::IsNullOrWhiteSpace($env:GOCACHE)) {
    $env:GOCACHE = Join-Path $root ".cache\go-build"
  }
  $library = Join-Path $absoluteOutput "KoraDB-native.dll"
  & $go build -trimpath -buildvcs=false -buildmode=c-shared -o $library ./sdk/native/cshared
  if ($LASTEXITCODE -ne 0) { throw "Native DLL build failed." }

  $smoke = Join-Path $absoluteOutput "native-abi-smoke.exe"
  & $compiler -std=c11 "-I$(Join-Path $root 'sdk\native\include')" (Join-Path $root "sdk\native\testdata\abi_smoke.c") $library -o $smoke
  if ($LASTEXITCODE -ne 0) { throw "Native ABI consumer compilation failed." }

  $oldPath = $env:PATH
  try {
    $env:PATH = "$absoluteOutput;$env:PATH"
    $dbPath = Join-Path $absoluteOutput "native-abi-smoke.db"
    if (Test-Path -LiteralPath $dbPath) { Remove-Item -LiteralPath $dbPath -Force }
    & $smoke $dbPath
    if ($LASTEXITCODE -ne 0) { throw "Native ABI smoke program failed." }
  } finally {
    $env:PATH = $oldPath
  }
} finally {
  $env:CC = $originalCC
  $env:CGO_ENABLED = $originalCGOEnabled
  $env:GOCACHE = $originalGoCache
}
