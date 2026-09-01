# YuvanoLabs KoraDB Service Security Model

This describes the implemented security controls and release gaps of the
KoraDB **gRPC server**. KoraDB v1.0.0 is approved for Community general
availability; see the [Production release plan](PRODUCTION_RELEASE_PLAN.md)
and [release decision](../RELEASE_DECISION.md) for its published boundaries
and evidence.

The in-process engine and local-file CLI run in the caller's own process and
trust boundary. OS file permissions and approved host/disk encryption protect
the `.db` file in embedded deployments.

## Principles

- **Secure by default / fail-closed.** `KoraDB-server serve` refuses to start
  unless TLS is configured *and* at least one administrator API key exists,
  unless you explicitly pass `--insecure`. That mode is only for explicit
  loopback development; it prints a loud warning and disables TLS and auth.
- **Secured server mode couples authentication to TLS.** The authenticated server configuration
  cannot run as plaintext. The CLI refuses to attach a supplied token to a plaintext connection.
- **Default-deny authorization.** A gRPC method with no entry in the RBAC policy is denied to
  everyone, including admins. Adding an RPC and forgetting to classify it fails closed.
- **Audit without exposure.** Every unary request is logged as a JSON record
  with who, method, outcome, peer, and latency, and **never**
  the document JSON or query values (which may contain PII or classified data).

## Transport: TLS and mTLS

- `--tls-cert` / `--tls-key`: server certificate and key (TLS 1.2 or later).
  Required unless `--insecure`.
- `--tls-client-ca`: when set, the server **requires and verifies client certificates** (mTLS)
  signed by that CA.
- `gencert` produces a dev CA + server cert. **Production should use your organization's CA/PKI**,
  not `gencert`.

`gencert` does not produce a separately managed client identity. Use organizational PKI or add a
dedicated development client-certificate workflow before relying on mTLS for multiple principals.

## Authentication: API keys

- Tokens look like `kdb_<keyID>_<secret>`: 64 bits of key ID plus 256 bits of
  secret. The `kdb_`
  prefix lets secret scanners flag leaks.
- Only `SHA-256(secret)` is stored, never the secret. Verification is a constant-time compare.
  A fast hash is correct here: a 256-bit random secret cannot be brute-forced, so the password
  work-factor that bcrypt/argon2 provide adds latency without security benefit.
- The token is shown **once** at creation and cannot be recovered.
- The CLI presents tokens in the gRPC `authorization` metadata as `Bearer <token>`.
- The server requires exactly one `Bearer <token>` value in the authorization
  metadata. Raw tokens and duplicate authorization values are rejected.

### Bootstrapping the first key

bbolt holds an **exclusive file lock** while the server runs, so a second process cannot open the
database to add a key. Therefore:

- The **first** admin key is created with the server **stopped** and bootstrap requires the
  `admin` role: `KoraDB-server bootstrap --db data.db --role admin`.
- All **subsequent** key management happens over the running server via admin-only RPCs
  (`CreateKey` / `ListKeys` / `RevokeKey`, exposed as `KoraDB key create|list|revoke`).
  **Revocation takes effect immediately**; no restart is required.

The current API supports a manual rotation procedure—create a replacement key,
move the workload to it, then revoke the old key—and each key may carry an
optional UTC expiry. It has no automated rotation deadline, per-key scope, or
external secret-manager integration.

## Authorization: roles (RBAC)

Coarse, ordered roles; higher includes lower:

| Role | Allowed methods |
|---|---|
| `readonly` | Get, Query, ListSchemas, ListCollections |
| `readwrite` | + Insert, Update, Delete |
| `admin` | + PutSchema, CreateCollection, CreateKey, ListKeys, RevokeKey |

The policy lives in `internal/auth/rbac.go` and is consulted fail-closed.

## Audit and resource limits

One JSON record per request, e.g.:

```
{"timestamp":"2026-08-31T12:00:00Z","method":"Insert","principal":"reporting/readonly","peer":"10.0.0.4:55512","code":"PermissionDenied","duration_ms":0}
```

Includes failed authentication and authorization attempts. It contains **no**
request or response payloads. It is a local structured log, not a tamper-evident
audit trail or SIEM integration.

The server enforces these preview safety bounds:

- gRPC receive and send messages are limited to 4 MiB.
- Legacy unary queries materialize at most 1,000 matching documents. On
  overflow they return an error, never a partial result set. Clients can opt
  into opaque continuation-token pages of up to 1,000 documents.
- A query filter is limited to 32 nesting levels and 64 comparison predicates.
- The server defaults to a 30-second unary-request deadline, 128 concurrent
  unary requests, and a shared 200 requests/second token bucket with a burst
  of 400. Operators can tune these with `--max-request-duration`,
  `--max-concurrent-requests`, `--max-requests-per-second`, and
  `--request-rate-burst`; a zero request rate is an explicit development-only
  opt-out.
- Query scans and page materialization observe request cancellation. Storage
  transactions remain atomic and may finish their individual operation before
  a cancelled request returns.

There are no server-side connection limits, per-principal/tenant quotas, or
brute-force-specific controls. The shared rate guard is an overload control,
not multi-tenant isolation.

## Metrics endpoint

By default, the server publishes process-local Prometheus metrics at
`http://127.0.0.1:9090/metrics`. It exports only method, gRPC outcome code,
request count, latency, and in-flight work; it never labels metrics with
credentials, principals, document IDs, or query values. The endpoint accepts
only an explicit numeric loopback address. Set `--metrics-addr=` to disable it
or collect it through a local agent/authenticated proxy; do not expose it
directly to untrusted networks.

## What is verified by tests

`internal/auth` and `test/security_test.go` assert the **denials**, which are the actual feature:

- unauthenticated request returns `Unauthenticated`
- invalid, tampered, or unknown token returns `Unauthenticated`
- a revoked key returns `Unauthenticated` immediately
- `readonly` calling a write or admin method returns `PermissionDenied`
- an unmapped method is denied even for an administrator
- a plaintext client cannot connect to a TLS server
- an mTLS server rejects a client with no certificate

## Required before a production security claim

- Scoped credentials and the approved identity integrations required by the
  supported deployment profile. Roles are coarse today; there are no
  per-collection ACLs or automated rotation workflow.
- Secret-manager integration and an approved at-rest-encryption/key-recovery
  design for the `.db` file.
- Connection limits, per-principal/tenant resource quotas, and brute-force
  controls, plus capacity evidence for the implemented request controls.
- Distributed tracing, alerting, SIEM/audit export, and tamper-evident audit
  storage. Loopback Prometheus request metrics are available today.
- Certificate rotation/reload procedures and mTLS identity-to-authorization
  mapping.
- A formal review of streaming interceptors before a streaming RPC is added.
- An independent threat model, security review, vulnerability-management
  process, incident procedure, and retained test evidence.

## Known prototype hazards

- `--insecure` disables both TLS and authentication. It is restricted to explicit numeric loopback
  addresses such as `--addr 127.0.0.1:50051`; firewall development systems as well.
- The CLI refuses to attach a token to a plaintext remote connection. Use no token for an insecure
  local-development server, or configure TLS for authenticated service access.
- Bootstrap must create an admin key, and KoraDB refuses to revoke the final admin key.
- The database file contains documents, schemas, indexes, and API-key hashes but is not encrypted
  by KoraDB. Protect it with local-disk permissions and approved disk/volume encryption.
- Legacy unary queries are capped at 1,000 matching documents; gRPC requests
  and responses are capped at 4 MiB; filter trees are limited to 32 levels and
  64 predicates. Explicit continuation-token pagination, request deadlines,
  bounded in-flight work, and a shared rate guard are available. Connection
  and tenant-specific controls remain pending, so do not expose it to
  untrusted tenants.

**Until these land, treat KoraDB as appropriate for trusted/internal networks with TLS, not as a
hardened internet-facing service.** Have it security-reviewed against your organization's
requirements before production use, and do not place classified/Restricted data behind it without
that review.

