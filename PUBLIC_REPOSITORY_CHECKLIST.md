# KoraDB public-release checklist

This checklist records actions that only a YuvanoLabs repository owner or
release operator can perform. It does not itself publish an artifact, change a
GitHub setting, reserve a name, or deploy infrastructure.

## Completed in source

- [x] YuvanoLabs canonical Go module and protobuf package migration.
- [x] Apache-2.0 license, notice, contribution, conduct, governance, support,
  security, privacy, and lifecycle policies.
- [x] Release workflow, archive checksums, manifest, SBOM/provenance steps,
  native DLL validation, installer scripts, and operations runbook.
- [x] Automated unit, integration, race, vet, protocol-lint, native ABI, and
  development release-archive verification.

## Owner actions before public release

- [x] Retain formal approval for the **KoraDB** product name.
- [x] Use the `github.com/YuvanoLabs/KoraDB` canonical source coordinate.
- [ ] Push this exact namespace-migrated source and enable Issues,
  Discussions, and private vulnerability reporting.
- [ ] Protect the default branch; require review, passing checks, and resolved
  conversations; restrict force pushes/deletion and administrator bypass.
- [ ] Confirm the shared `smartbytecoder@gmail.com` security/conduct contact is
  monitored and has account-recovery controls.
- [ ] Configure release permissions, repository secrets, artifact-signing
  policy, and release-environment protection; test the tag workflow from a
  release candidate.
- [ ] Publish the canonical YuvanoLabs KoraDB repository/documentation URL.
  Do not publish package, container, or website links until they exist.

## Evidence before a production/GA claim

- [ ] Review the released archives, SBOM, provenance, and signatures from the
  real protected release workflow.
- [ ] Run and retain capacity/load, filesystem, disk-full, corruption,
  power-loss, upgrade, and recovery tests on every supported OS/filesystem.
- [ ] Define and test backup retention, off-host encryption, RPO/RTO, and a
  restore/rollback drill with the production operator.
- [ ] Complete an independent security review, dependency/vulnerability
  process, incident exercise, certificate/key rotation rehearsal, and owner
  approval of the support matrix.
- [ ] Publish only the support boundaries proven by that evidence. Do not
  claim HA, replication, at-rest encryption, multi-tenancy, external identity,
  or language packages that have not been released and supported.
