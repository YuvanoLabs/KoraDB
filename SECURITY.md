# Security policy

## Supported status

KoraDB v1.0.0 is a Community general-availability product. It has no production
SLA, guaranteed response time, or regulated-workload certification. Security
fixes apply to the complete KoraDB product, including its embedded SDK, native
ABI, CLI, server, and installer artifacts; no feature is withheld by product
tier.

The server is secure by default: TLS and an administrator key are required.
`--insecure` is restricted to numeric loopback development addresses. The
database file, backups, audit logs, TLS material, and metrics endpoint remain
the deploying operator's responsibility. See [docs/SECURITY.md](docs/SECURITY.md)
for the technical security model.

## Report a vulnerability privately

Do not put vulnerability details, credentials, customer data, or exploits in a
public issue, discussion, or pull request.

Use the repository's **Security → Report a vulnerability** flow. The
repository owner must enable GitHub private vulnerability reporting before
public launch. If that flow is unavailable, email
[smartbytecoder@gmail.com](mailto:smartbytecoder@gmail.com) with the subject
`KoraDB security contact request` and no technical details; a maintainer will
establish a private reporting channel.

Include the affected version or commit, impact, prerequisites, a minimal
synthetic reproduction, and suggested mitigation when possible. Do not test
against systems or data you do not own.

## Response process

Once a monitored private channel is active, maintainers target acknowledgement
within three business days, severity assessment, coordinated remediation, and
disclosure after a fix is available. Reporters are credited unless anonymity
is requested. No bug bounty or payment is implied.
