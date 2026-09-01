#!/usr/bin/env sh
# Install a verified KoraDB portable release archive on Linux or macOS.

set -eu

usage() {
  cat <<'EOF'
Usage: install.sh --package FILE.zip --sha256 HEX [--destination DIR] [--force]

The archive must be verified against the SHA-256 value published in
archive-checksums.txt. The installer does not start or register KoraDB-server.
EOF
}

package=""
expected_sha=""
destination="${HOME}/.local/lib/koradb"
force=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --package)
      package=${2:?--package requires a value}
      shift 2
      ;;
    --sha256)
      expected_sha=${2:?--sha256 requires a value}
      shift 2
      ;;
    --destination)
      destination=${2:?--destination requires a value}
      shift 2
      ;;
    --force)
      force=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ -z "$package" ] || [ -z "$expected_sha" ]; then
  echo "error: --package and --sha256 are required" >&2
  usage >&2
  exit 2
fi
if [ ! -f "$package" ]; then
  echo "error: installer package not found: $package" >&2
  exit 1
fi
case "$package" in
  *.zip) ;;
  *) echo "error: installer package must be a .zip archive" >&2; exit 1 ;;
esac
if ! command -v unzip >/dev/null 2>&1; then
  echo "error: unzip is required to install KoraDB" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual_sha=$(sha256sum "$package" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual_sha=$(shasum -a 256 "$package" | awk '{print $1}')
else
  echo "error: sha256sum or shasum is required to verify the release" >&2
  exit 1
fi
if [ "$(printf '%s' "$actual_sha" | tr '[:upper:]' '[:lower:]')" != "$(printf '%s' "$expected_sha" | tr '[:upper:]' '[:lower:]')" ]; then
  echo "error: SHA-256 mismatch for $package" >&2
  exit 1
fi

parent=$(dirname "$destination")
mkdir -p "$parent"
stage=$(mktemp -d "$parent/.koradb-install.XXXXXX")
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT INT TERM

unzip -q "$package" -d "$stage"
for binary in KoraDB KoraDB-server KoraDB-restore; do
  if [ ! -f "$stage/$binary" ]; then
    echo "error: installer package is missing $binary" >&2
    exit 1
  fi
  chmod 0755 "$stage/$binary"
done
if [ ! -f "$stage/release-manifest.json" ]; then
  echo "error: installer package is missing release-manifest.json" >&2
  exit 1
fi

if [ -e "$destination" ]; then
  if [ "$force" -ne 1 ]; then
    echo "error: destination exists: $destination (use --force only after a verified database backup)" >&2
    exit 1
  fi
  backup="${destination}.previous.$(date -u +%Y%m%d%H%M%S)"
  mv "$destination" "$backup"
  echo "Moved previous installation to $backup"
fi
mv "$stage" "$destination"
trap - EXIT INT TERM
echo "Installed KoraDB to $destination"
echo "Add $destination to PATH to use KoraDB commands."
echo "Configure TLS and an administrator key before registering KoraDB-server with your service manager."
