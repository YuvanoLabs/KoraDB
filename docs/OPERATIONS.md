# KoraDB single-node operations runbook

KoraDB is a single-node product. One process owns each database file: either
the embedding application or `KoraDB-server`. Do not place a live database on
a shared/network filesystem or let an embedded process open the file while the
server owns it.

This runbook describes the v1.0.0 secure single-node service profile. The
release is approved for Community GA with the explicit boundaries in
[RELEASE_DECISION.md](../RELEASE_DECISION.md); target-environment qualification
remains the operator's responsibility.

## Secure service baseline

1. Install a verified operator archive using [Installation](INSTALLATION.md).
2. Create a dedicated operating-system account and a private database
   directory owned by that account.
3. Obtain a server certificate/key from organization-managed PKI. The bundled
   `gencert` command is only for development.
4. With the server stopped, bootstrap the first administrator key. Store the
   displayed token in an approved secret manager; it is never recoverable.
5. Start the service with TLS, explicit database path, and loopback metrics.

```sh
KoraDB-server bootstrap --db /var/lib/koradb/data.db --name bootstrap --role admin

KoraDB-server serve \
  --addr 0.0.0.0:50051 \
  --db /var/lib/koradb/data.db \
  --tls-cert /etc/koradb/server.crt \
  --tls-key /etc/koradb/server.key \
  --metrics-addr 127.0.0.1:9090
```

The service refuses to run without TLS and an administrator key. Never use
`--insecure` outside a numeric loopback development address.

The defaults cap unary requests at 30 seconds, 128 in-flight requests, and a
shared 200 requests/second burstable rate. Tune only from observed capacity:
`--max-request-duration`, `--max-concurrent-requests`,
`--max-requests-per-second`, and `--request-rate-burst`.

## Linux service-manager example

Place the verified binaries in an administrator-owned directory such as
`/opt/koradb`, and create an account named `koradb`. A starting systemd unit
is shown below; adapt paths, certificate access, and hardening to the host.

```ini
[Unit]
Description=KoraDB single-node service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=koradb
Group=koradb
ExecStart=/opt/koradb/KoraDB-server serve --addr 0.0.0.0:50051 --db /var/lib/koradb/data.db --tls-cert /etc/koradb/server.crt --tls-key /etc/koradb/server.key --metrics-addr 127.0.0.1:9090
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/koradb

[Install]
WantedBy=multi-user.target
```

Use the equivalent dedicated service account, locked-down database directory,
and explicit TLS paths with Windows Service Control Manager. The installer
does not register a service automatically, so it cannot accidentally start an
unauthenticated or misconfigured server.

## Health and metrics

- The gRPC health service reports readiness only after the database opens.
- `http://127.0.0.1:9090/metrics` is a loopback-only Prometheus endpoint.
  It contains request method/outcome, count, latency, and in-flight work—no
  tokens, principals, documents, or query values.
- Collect metrics through a local agent or authenticated proxy. Do not bind
  metrics directly to an untrusted network.
- JSON audit records are emitted to standard error. Retain and protect them
  with the platform logging system; they are not tamper-evident or a SIEM sink.

## Backup, verification, and restore

Remote backup/restore is deliberately unsupported. Treat backups as an
offline maintenance operation until an authenticated maintenance workflow is
released.

1. Stop `KoraDB-server` cleanly. It attempts a graceful 10-second shutdown.
2. Verify the offline file and create a new snapshot path (the CLI refuses to
   overwrite a backup).

```sh
KoraDB --db /var/lib/koradb/data.db verify
KoraDB --db /var/lib/koradb/data.db backup /secure-backups/koradb-$(date -u +%Y%m%dT%H%M%SZ).db
```

3. Independently protect the snapshot with approved off-host storage and
   encryption, then record its checksum, retention time, and restore test.
4. For a restore, stop the service, preserve the current database, restore to
   a staged destination with an explicit size limit, verify it, and only then
   make it the service database.

```sh
KoraDB-restore --max-bytes 1073741824 snapshot.db /var/lib/koradb/data-restored.db
KoraDB --db /var/lib/koradb/data-restored.db verify
```

Use `--overwrite --rollback <path>` only when the destination and rollback
path have been verified. See [Offline restore](RESTORE.md) for exact overwrite
and rollback behavior.

## Credential and certificate rotation

Create a replacement key with a short, explicit expiry; move every client to
it; confirm successful requests; then revoke the previous key.

```sh
KoraDB --server db.example:50051 --tls-ca /etc/ssl/corp-ca.crt --token "$KoraDB_TOKEN" \
  key create workload-next readwrite --expires-at=2027-01-01T00:00:00Z
```

The server reads certificates at startup. To rotate TLS, deploy the new key
and certificate with correct OS permissions, validate their chain and name,
restart the service during a planned window, and test a client using the
updated trust store. Certificate hot reload, key automation, and identity
mapping are not implemented.

## Upgrade and rollback

Before an upgrade, record the version, verify the database, create and test a
backup, and retain the previous verified program directory. Installers move an
old program directory aside only with explicit `--force`; they never replace a
database file. Stop the service before changing binaries, then start it and
check health, metrics, and representative client operations. Roll back the
binary first; restore data only through the tested offline restore procedure.

Capacity, RPO/RTO, filesystem qualification, fault-injection evidence,
external audit export, alert thresholds, and a diagnostics bundle remain GA
gates. Do not invent values for them in an operational change record.
