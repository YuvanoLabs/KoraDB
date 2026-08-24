# YuvanoLabs KoraDB Security Model

This describes the security of the KoraDB **gRPC server**. (The in-process engine and local-file
CLI run in the caller's own process and trust boundary; OS file permissions on the `.db` file are
the control there.)

## Principles

- **Secure by default / fail-closed.** `KoraDB-server serve` refuses to start unless TLS is
  configured *and* at least one API key exists ? unless you explicitly pass `--insecure`
  (intended only for explicit loopback development; it prints a loud warning and disables both
  TLS and auth).
- **Secured server mode couples authentication to TLS.** The authenticated server configuration
  cannot run as plaintext. The CLI refuses to attach a supplied token to a plaintext connection.
- **Default-deny authorization.** A gRPC method with no entry in the RBAC policy is denied to
  everyone, including admins. Adding an RPC and forgetting to classify it fails closed.
- **Audit without exposure.** Every request is logged with who/method/outcome/peer/latency and
  **never** the document JSON or query values (which may contain PII / classified data).

## Transport: TLS and mTLS

- `--tls-cert` / `--tls-key`: server certificate and key (TLS ? 1.2). Required unless `--insecure`.
- `--tls-client-ca`: when set, the server **requires and verifies client certificates** (mTLS)
  signed by that CA.
- `gencert` produces a dev CA + server cert. **Production should use your organization's CA/PKI**,
  not `gencert`.

`gencert` does not produce a separately managed client identity. Use organizational PKI or add a
dedicated development client-certificate workflow before relying on mTLS for multiple principals.

## Authentication: API keys

- Tokens look like `kdb_<keyID>_<secret>` ? 64 bits of key id + 256 bits of secret. The `kdb_`
  prefix lets secret scanners flag leaks.
- Only `SHA-256(secret)` is stored, never the secret. Verification is a constant-time compare.
  A fast hash is correct here: a 256-bit random secret cannot be brute-forced, so the password
  work-factor that bcrypt/argon2 provide adds latency without security benefit.
- The token is shown **once** at creation and cannot be recovered.
- The CLI presents tokens in the gRPC `authorization` metadata as `Bearer <token>`.
- The server currently also accepts a raw token in that metadata field. Tightening this to the
  documented Bearer form is recommended before a stable release.

### Bootstrapping the first key

bbolt holds an **exclusive file lock** while the server runs, so a second process cannot open the
database to add a key. Therefore:

- The **first** admin key is created with the server **stopped** and bootstrap requires the
  `admin` role: `KoraDB-server bootstrap --db data.db --role admin`.
- All **subsequent** key management happens over the running server via admin-only RPCs
  (`CreateKey` / `ListKeys` / `RevokeKey`, exposed as `KoraDB key create|list|revoke`).
  **Revocation takes effect immediately** ? no restart.

## Authorization: roles (RBAC)

Coarse, ordered roles; higher includes lower:

| Role | Allowed methods |
|---|---|
| `readonly` | Get, Query, ListSchemas, ListCollections |
| `readwrite` | + Insert, Update, Delete |
| `admin` | + PutSchema, CreateCollection, CreateKey, ListKeys, RevokeKey |

The policy lives in `internal/auth/rbac.go` and is consulted fail-closed.

## Audit log

One structured line per request, e.g.:

```
audit method=Insert principal=reporting/readonly peer=10.0.0.4:55512 code=PermissionDenied dur=0s
```

Includes failed authentication/authorization attempts. Contains **no** request/response payloads.
(Forwarding this to a SIEM / structured sink is a roadmap item.)

## What is verified by tests

`internal/auth` and `test/security_test.go` assert the **denials**, which are the actual feature:

- unauthenticated request ? `Unauthenticated`
- invalid / tampered / unknown token ? `Unauthenticated`
- revoked key ? `Unauthenticated` (immediately)
- `readonly` calling a write or admin method ? `PermissionDenied`
- unmapped method ? denied even for admin
- plaintext client against a TLS server ? connection refused
- mTLS server + client with no client certificate ? rejected

## Not yet covered (roadmap ? do not assume these exist)

- Key **rotation** / expiry; per-key scopes; per-collection ACLs (only coarse roles today).
- Secret management integration (Vault/KMS); at-rest encryption of the `.db` file.
- Rate limiting / quota; request size limits; brute-force lockout.
- Audit forwarding to a SIEM; tamper-evident audit storage.
- Stream/`Watch` RPC interceptors (no streaming RPCs exist yet).
- Validation/escaping of principal names before line-oriented audit output.
- Certificate hot reload and documented certificate-rotation procedures.
- Limits for document size, query result count/depth, concurrent connections, and execution time.

## Known prototype hazards

- `--insecure` disables both TLS and authentication. It is restricted to explicit numeric loopback
  addresses such as `--addr 127.0.0.1:50051`; firewall development systems as well.
- The CLI refuses to attach a token to a plaintext remote connection. Use no token for an insecure
  local-development server, or configure TLS for authenticated service access.
- Bootstrap must create an admin key, and KoraDB refuses to revoke the final admin key.
- The database file contains documents, schemas, indexes, and API-key hashes but is not encrypted
  by KoraDB. Protect it with local-disk permissions and approved disk/volume encryption.
- The API has no pagination. Unary queries are capped at 1,000 matching documents; gRPC requests
  and responses are capped at 4 MiB; filter trees are limited to 32 levels and 64 predicates.
  Full connection, rate, and execution-time controls remain pending, so do not expose it to
  untrusted tenants.

**Until these land, treat KoraDB as appropriate for trusted/internal networks with TLS, not as a
hardened internet-facing service.** Have it security-reviewed against your organization's
requirements before production use, and do not place classified/Restricted data behind it without
that review.

