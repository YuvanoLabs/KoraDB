# KoraDB Product Architecture and Adoption Guide

## Product definition

KoraDB is a protobuf-native document database for applications that need durable
local data without requiring a separate database service. It stores documents as
protobuf wire data, retains schema descriptors with the database, and exposes
the same document model through embedded and secured service deployments.

KoraDB is published by YuvanoLabs.

## Deployment choices

| Deployment | Best for | Operating model |
|---|---|---|
| Embedded | Desktop software, tools, appliances, tests, and local-first services | The application owns one database file and its lifecycle. |
| Secured service | Shared access, centralized credentials, and network operations | KoraDB-server owns the file and provides gRPC over TLS. |

The embedded model never starts a hidden server. The service model is explicit
and should be selected whenever more than one process needs access to the same
database.

## Where KoraDB fits best

- Edge gateways, industrial appliances, and air-gapped deployments.
- Protobuf and gRPC applications that want one schema vocabulary for APIs and
  persistence.
- Developer tools, automation, agents, desktop products, and test harnesses.
- Control-plane and configuration workloads with durable local state, many
  reads, and modest write concurrency.
- On-prem software where a single deployable binary and local database file
  simplify installation and support.

## Data and schema model

KoraDB accepts protobuf source at runtime, compiles it without a target-side
`protoc` dependency, and records schema metadata alongside document data.
Collections bind to a protobuf message type and may define a primary-key field
and secondary indexes.

Schema registration is atomic. Existing field numbers, types, cardinality,
presence, maps, oneofs, referenced types, messages, and enum values are
protected against incompatible replacement. Compatible additive changes retain
readability of existing data. See [Schema evolution](SCHEMA_EVOLUTION.md).

## Operational model

KoraDB provides consistent snapshots, storage integrity verification, and an
offline restore command that validates a staged snapshot before publication.
An overwrite requires an explicit rollback destination for the previous
database. See [Offline restore](RESTORE.md).

A database file uses the single-writer model of its bbolt storage engine. Keep
it on a local, reliable filesystem and do not share the file over network
filesystems or between processes. Use KoraDB-server for shared access.

## Security model

The service uses TLS by default, can require mTLS, authenticates API keys, and
applies readonly, readwrite, and administrator roles. It provides a health
endpoint, request and response size limits, bounded filter complexity, and
audit logging that excludes document contents. See [Security](SECURITY.md) for
configuration and operational responsibilities.

## Product boundaries

KoraDB intentionally does not claim distributed consensus, automatic failover,
replication, sharding, SQL joins, analytics, full-text search, mobile sync, or
multi-region operation. These are different product categories with different
correctness and operational contracts.

For language packaging and deployment guidance, see
[Language integrations](INTEGRATIONS.md). For delivery priorities, see the
[Product roadmap](ROADMAP.md).
