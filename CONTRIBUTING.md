# Contributing to KoraDB

Thank you for improving KoraDB. Focused bug reports, documentation fixes,
reproducible recovery evidence, and small tested changes are especially
valuable while the product is in engineering preview.

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Before opening work

1. Search existing issues and the [roadmap](docs/ROADMAP.md).
2. Open an issue before a large change, dependency, public API, file-format,
   schema, storage, release, or security change.
3. Do not include credentials, database files, customer documents, private
   incident records, or sensitive audit logs in an issue or pull request.

Small documentation corrections may go directly to a pull request.

## Change standards

- Add or update tests and documentation with every behavioral change.
- Preserve the documented single-file, single-node, atomicity, recovery, and
  security contracts.
- Public Go, C ABI, gRPC, file-format, schema, and authentication changes
  require compatibility and migration analysis.
- New dependencies require a purpose, license review, maintenance assessment,
  and vulnerability review.
- Keep release artifacts reproducible and do not commit local databases,
  credentials, generated release output, or tool caches.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
buf lint
```

Native ABI and release-archive checks are documented in
[Installation](docs/INSTALLATION.md) and the
[Production release plan](docs/PRODUCTION_RELEASE_PLAN.md).

## Security

Do not report vulnerabilities in public issues or pull requests. Follow
[SECURITY.md](SECURITY.md).
