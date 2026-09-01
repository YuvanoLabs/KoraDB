# KoraDB Documentation

KoraDB is the protobuf-native embedded and single-node service database from
YuvanoLabs. These documents describe the product, how to operate it, and how to
integrate it into an application.

## Start here

1. [Root README](../README.md) - deployment modes and quick start.
2. [Product architecture and adoption](PRODUCT_ASSESSMENT.md) - product model,
   best-fit workloads, and operating boundaries.
3. [Architecture](ARCHITECTURE.md) - storage, schemas, documents, indexes, and
   query execution.
4. [Schema evolution](SCHEMA_EVOLUTION.md) - compatible and incompatible schema
   changes.
5. [Security](SECURITY.md) - TLS, authentication, authorization, and audit
   behavior.
6. [Offline restore](RESTORE.md) - snapshot validation, restore, and rollback
   safety.
7. [Language integrations](INTEGRATIONS.md) - embedded, native ABI, and gRPC
   integration guidance.
8. [Installation](INSTALLATION.md) - verified package/DLL and operator archive
   installation model.
9. [Operations runbook](OPERATIONS.md) - single-node service deployment,
   metrics, backup/restore, rotation, and upgrade procedures.
10. [Production release plan](PRODUCTION_RELEASE_PLAN.md) - current release
   blockers, owners' evidence, and separate launch gates for package/DLL and
   installer delivery variants.
11. [Production-readiness criteria](ENTERPRISE_READINESS.md) - non-negotiable
   criteria for production claims.
12. [Product roadmap](ROADMAP.md) - product delivery programs and priorities.

## Public repository policies

- [License](../LICENSE) and [notice](../NOTICE)
- [Contributing](../CONTRIBUTING.md) and [code of conduct](../CODE_OF_CONDUCT.md)
- [Security reporting](../SECURITY.md), [support](../SUPPORT.md), and
  [privacy](../PRIVACY.md)
- [Governance](../GOVERNANCE.md) and [release lifecycle](../LIFECYCLE.md)
- [Public-release owner checklist](../PUBLIC_REPOSITORY_CHECKLIST.md)

## Product identity

- **Product:** KoraDB
- **Publisher:** YuvanoLabs
- **Deployment:** embedded local database or secured single-node gRPC service

## Documentation standards

Product documentation describes implemented behavior and operational
responsibilities. Package availability, support commitments, and release
artifacts are stated only when the corresponding package or artifact exists.
Internal release coordination is maintained separately from product guidance.
