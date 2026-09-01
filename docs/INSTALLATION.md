# Install KoraDB release archives

KoraDB is one Community product with two delivery variants:

- **Developer package/DLL:** language-specific packages and native libraries
  for applications embedding KoraDB.
- **Operator installer:** verified portable archives containing `KoraDB`,
  `KoraDB-server`, and `KoraDB-restore`.

The current repository can build the operator archives. Do not treat
development builds as official artifacts. Use only a protected release tag
whose checksums, SBOM, provenance, and release notes have been published.

## Verify the archive first

Obtain the archive SHA-256 from the release's `archive-checksums.txt`, not from
an untrusted download page. Verify the checksum before extracting or running
anything from the archive.

```powershell
Get-FileHash -Algorithm SHA256 .\KoraDB-vX.Y.Z-windows-amd64.zip
```

```sh
sha256sum KoraDB-vX.Y.Z-linux-amd64.zip
# macOS alternative: shasum -a 256 KoraDB-vX.Y.Z-darwin-arm64.zip
```

The archive contains a `release-manifest.json` that records the product,
version, source commit, build time, and SHA-256 digest of every binary.

## Native DLL package (Windows amd64)

The developer release archive is named
`KoraDB-native-vX.Y.Z-windows-amd64.zip`. Verify it the same way as the
operator archive, then place `KoraDB-native.dll` beside the application or in
an approved native-library directory. Compile against the bundled `koradb.h`.
The DLL uses the embedded database file directly; never open the same file
from a server or another process.

The native ABI remains pre-release until its compatibility policy and
platform-specific package support are published. See the
[native ABI guide](../sdk/native/README.md) for ownership rules and its
consumer smoke test.

## Windows installer

Run the verified installer archive with its expected digest. It expands into a
new directory and refuses to replace an existing installation unless `-Force`
is explicit. A forced replacement preserves the old program directory beside
the new one; it does **not** replace or manipulate a database file.

```powershell
.\scripts\install-windows.ps1 `
  -PackagePath .\KoraDB-vX.Y.Z-windows-amd64.zip `
  -SHA256 <digest-from-archive-checksums.txt> `
  -Destination 'C:\Program Files\KoraDB'
```

## Linux and macOS installer

The POSIX installer verifies the archive before extraction. By default it
installs to `~/.local/lib/koradb`; use `--destination` for a service-owned
directory after validating its ownership and permissions.

```sh
./scripts/install.sh \
  --package KoraDB-vX.Y.Z-linux-amd64.zip \
  --sha256 <digest-from-archive-checksums.txt>
```

Both installers place binaries only. They do not start a server or install a
service automatically. Before a secured-service deployment, use an
organization-managed TLS certificate, bootstrap an administrator key while the
server is stopped, configure a service-manager unit appropriate for the
platform, and complete the operational checks in the
[Production release plan](PRODUCTION_RELEASE_PLAN.md).

`KoraDB-server` exposes Prometheus metrics on `127.0.0.1:9090` by default.
Keep that loopback-only endpoint behind a local collector or authenticated
proxy; use `--metrics-addr=` only when disabling it intentionally.

## Development archive build

For local validation only, build portable archives from the current checkout:

```powershell
.\scripts\build-release.ps1 -Development -Version 0.0.0-dev -Commit local
```

Building the Windows DLL archive also needs a GNU C compiler. Either put
MinGW-w64 `gcc.exe` on `PATH`, set `CC`, or pass `-NativeCompiler` explicitly.

The script writes per-platform binaries, a manifest, and archive checksums to
`dist/`. A public release requires explicit version, source commit, and build
time values and will refuse to proceed until the identity gate passes.
