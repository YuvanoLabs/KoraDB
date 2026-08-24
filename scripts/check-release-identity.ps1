param(
  [string]$Manifest = (Join-Path (Split-Path -Parent $PSScriptRoot) "product.identity.yaml")
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$manifestPath = [System.IO.Path]::GetFullPath($Manifest)
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
  Write-Error "Release identity manifest not found: $manifestPath"
  exit 1
}

$content = Get-Content -LiteralPath $manifestPath -Raw

function Get-IdentityValue {
  param([Parameter(Mandatory = $true)][string]$Key)

  $pattern = "(?m)^\s*" + [regex]::Escape($Key) + "\s*:\s*(?<value>[^#\r\n]+?)\s*$"
  $match = [regex]::Match($content, $pattern)
  if (-not $match.Success) {
    throw "Required key '$Key' is missing from $manifestPath"
  }

  return $match.Groups["value"].Value.Trim().Trim('"').Trim("'")
}

$requirements = @(
  @{key = "launch_status"; expected = "approved" },
  @{key = "product_name_approval"; expected = "approved" },
  @{key = "legal_owner_confirmation"; expected = "approved" },
  @{key = "trademark_clearance"; expected = "approved" },
  @{key = "domain_reservation"; expected = "reserved" },
  @{key = "github_owner_reservation"; expected = "reserved" },
  @{key = "container_owner_reservation"; expected = "reserved" },
  @{key = "package_scope_reservation"; expected = "reserved" },
  @{key = "namespace_migration"; expected = "complete" }
)

$blockers = @()
foreach ($requirement in $requirements) {
  $actual = Get-IdentityValue -Key $requirement.key
  if ($actual -ne $requirement.expected) {
    $blockers += "$($requirement.key) is '$actual' (required: '$($requirement.expected)')"
  }
}

if ($blockers.Count -gt 0) {
  Write-Host "PUBLIC RELEASE BLOCKED by product.identity.yaml:" -ForegroundColor Red
  foreach ($blocker in $blockers) {
    Write-Host "  - $blocker" -ForegroundColor Red
  }
  Write-Host "See docs/NAMING_AND_NAMESPACES.md for evidence and the owner checklist."
  exit 1
}

# Once the approvals are complete, prevent the release status from getting ahead
# of the actual source namespace migration.
$plannedGoModule = Get-IdentityValue -Key "go_module"
$plannedProtoPackage = Get-IdentityValue -Key "protobuf_package"
$plannedProtoGoPackage = Get-IdentityValue -Key "protobuf_go_package"

$goMod = Get-Content -LiteralPath (Join-Path $root "go.mod") -Raw
$goModuleMatch = [regex]::Match($goMod, "(?m)^module\s+(?<value>\S+)\s*$")
if (-not $goModuleMatch.Success -or $goModuleMatch.Groups["value"].Value -ne $plannedGoModule) {
  $blockers += "go.mod module does not equal '$plannedGoModule'"
}

$proto = Get-Content -LiteralPath (Join-Path $root "api/proto/KoraDB.proto") -Raw
$protoPackagePattern = "(?m)^package\s+" + [regex]::Escape($plannedProtoPackage) + "\s*;"
$protoGoPackagePattern =
  "(?m)^option\s+go_package\s*=\s*""" +
  [regex]::Escape($plannedProtoGoPackage) +
  """\s*;"
if ($proto -notmatch $protoPackagePattern) {
  $blockers += "protobuf package does not equal '$plannedProtoPackage'"
}
if ($proto -notmatch $protoGoPackagePattern) {
  $blockers += "protobuf go_package does not equal '$plannedProtoGoPackage'"
}

$bufConfig = Get-Content -LiteralPath (Join-Path $root "buf.gen.yaml") -Raw
$bufModulePattern = "(?m)^\s*opt:\s*module=" + [regex]::Escape($plannedGoModule) + "\s*$"
if ($bufConfig -notmatch $bufModulePattern) {
  $blockers += "buf.gen.yaml does not generate into module '$plannedGoModule'"
}

if ($blockers.Count -gt 0) {
  Write-Host "PUBLIC RELEASE BLOCKED by incomplete source namespace migration:" -ForegroundColor Red
  foreach ($blocker in $blockers) {
    Write-Host "  - $blocker" -ForegroundColor Red
  }
  exit 1
}

Write-Host "Product identity and namespace release gate passed." -ForegroundColor Green

