# Governance

KoraDB uses maintainer-led, evidence-driven governance. The goal is to grow a
healthy Community product without weakening data safety, security, or user
control.

| Role | Responsibility |
|---|---|
| Contributor | Issues, documentation, code, tests, reviews |
| Reviewer | Technical review in an explicitly assigned area |
| Maintainer | Merge, release, triage, security, roadmap, and compatibility decisions |
| Security responder | Private vulnerability assessment and coordinated disclosure |

- Small fixes use pull-request consensus from the responsible maintainer.
- Public API, schema, package, storage, security, and release changes require
  a written compatibility and risk decision.
- Breaking contract changes require migration and rollback analysis plus
  maintainer approval.
- Security fixes and data-safety corrections are never feature-gated.

Governance changes use a public pull request after the repository is public.
Emergency security response may proceed immediately and is documented after
coordinated disclosure.
