# KoraDB privacy notice

KoraDB is self-hosted software, not a hosted data service. KoraDB does not
operate a customer account, collect telemetry to YuvanoLabs, or transmit a
database file, document, schema, API key, backup, or audit record to
YuvanoLabs by default.

In embedded mode, application data stays in the database file controlled by
the application operator. In service mode, document JSON crosses only between
the configured gRPC client and the operator's KoraDB server. Operators choose
the network, TLS certificates, identity model, backups, log retention, and
storage encryption.

The server writes payload-free JSON audit records locally and offers a
loopback-only Prometheus endpoint containing request method, outcome, count,
latency, and in-flight work. Operators decide whether to collect, retain, or
export those records. Do not configure external log or metrics systems to
receive confidential information without an approved privacy and security
review.

This notice covers the KoraDB software distribution. A company website,
package registry, support portal, or managed service may have additional
privacy terms when one is published.
