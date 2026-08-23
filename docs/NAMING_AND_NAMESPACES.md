# Product Naming and Namespace Launch Review

> Review date: 2026-07-19  
> Scope: product name, preliminary trademark risk, domains, source repository, language-package
> coordinates, executable names, and container registries.  
> Decision: **public release is blocked** pending a final product-name decision, formal trademark
> clearance, legal-owner confirmation, reservations, and source namespace migration.

This is a commercial and technical risk screen, not a legal opinion or a guarantee that a name is
registrable. Search results and availability can change at any moment. A qualified trademark
professional must clear the final mark in every market where the product will be offered.

> **Commercial caution:** this document names domains and account coordinates that appeared
> unreserved during a point-in-time screen. Do not push this review or `product.identity.yaml` to
> a public repository before the owner attempts the priority reservations; disclosure can invite
> squatting.

## Executive decision

| Candidate or decision | Status | Reason |
|---|---|---|
| `KoraDB` as the unqualified public product name | Do not launch | The name is already used by multiple software/database projects, including a closely related protobuf store. It is descriptive of “protobuf database,” making differentiation and searchability weak. |
| `YuvanoLabs KoraDB` | Working name only | The company qualifier improves attribution, but the embedded `KoraDB` term remains crowded. It needs formal clearance and an explicit owner decision. |
| A new distinctive coined product name | Preferred public-launch path | A unique mark offers better searchability, package availability, defensibility, and future product-family expansion. It still requires a full clearance search. |
| Current source identifiers (`module KoraDB`, `KoraDB.v1`) | Development only | They are not globally qualified and would create avoidable Go/protobuf compatibility debt if published. |
| Public release | Blocked | Required approvals and reservations in `product.identity.yaml` are pending. |

The repository may continue using **KoraDB as a development codename**. Documentation must call
`YuvanoLabs KoraDB` a working name until the final decision is recorded. Do not use `®`, claim
registration, publish packages, or promote public container images under these coordinates before
clearance.

## What the preliminary screen found

### Product-name collision

The unqualified name is not a clean launch candidate:

- [linka-cloud/koradb](https://github.com/linka-cloud/koradb) is a public Go project in the same
  problem space and describes in-process, server, and replicated protobuf storage.
- [ygrpc/koradb](https://github.com/ygrpc/koradb) uses the same repository name for a protobuf
  database/ORM project.
- The npm package
  [proto-database](https://www.npmjs.com/package/proto-database) historically exposes a
  `KoraDB` command.
- A GitHub repository-name API search returned dozens of repositories named `KoraDB` at the
  review time.

This does not by itself decide legal rights, but it creates practical risks:

- customer and search-engine confusion;
- mistaken support requests and security reports;
- collisions in shell commands, repositories, imports, and package metadata;
- weak word-of-mouth discoverability;
- possible opposition or rebrand cost after adoption.

### Preliminary trademark screen

No relevant exact-match result for **YuvanoLabs** was found in the public web and repository screen.
No conclusive exact registered-mark result for **KoraDB** was established. Those observations are
not clearance: trademark databases require structured similarity, owner, goods/services, status,
territory, and common-law searches.

Before approval, counsel should search at least:

- exact, phonetic, visual, prefix/suffix, and translation variants of `YuvanoLabs`, `KoraDB`, and
  the selected combined or replacement mark;
- downloadable database/client software in Nice Class 9;
- database SaaS/PaaS, hosted software, and software-development services in Nice Class 42;
- adjacent classes and common-law uses relevant to the actual sales model;
- company registries, app stores, open-source repositories, package registries, domains, and
  unregistered market uses;
- India, the United States, the European Union, and every intended sales or hosting market.

Authoritative starting points are the
[WIPO Global Brand Database](https://branddb.wipo.int/),
[USPTO trademark search](https://tmsearch.uspto.gov/),
[EUIPO eSearch](https://euipo.europa.eu/eSearch/), and
[India IP trademark public search](https://tmrsearch.ipindia.gov.in/tmrpublicsearch/).
USPTO explains that a comprehensive clearance search spans multiple sources and that confusing
similarity depends on both the marks and related goods/services:
[search guidance](https://www.uspto.gov/trademarks/search) and
[likelihood of confusion](https://www.uspto.gov/trademarks/search/likelihood-confusion).
WIPO's Nice Classification places downloadable software in
[Class 9](https://nclpub.wipo.int/enfr/?basic_numbers=show&class_number=9) and SaaS/PaaS services
in [Class 42](https://nclpub.wipo.int/enfr/?basic_numbers=show&class_number=42).

`KoraDB` also reads as a shortened description of the product category. Descriptive wording can
be harder to register or enforce; the
[USPTO descriptiveness guidance](https://www.uspto.gov/trademarks/laws/merely-descriptive-refusal)
explains that merely descriptive marks may be refused.

### Domain screen

The following point-in-time observations were made on 2026-07-19 using registry RDAP and DNS:

| Domain | Observation at review time | Decision |
|---|---|---|
| `KoraDB.com` | Registered record existed and showed redemption-period status. | Do not depend on it; monitor only if desired. A redemption or drop process is unpredictable. |
| `KoraDB.io` | DNS was delegated to name servers. | Treat as owned by another party. |
| `KoraDB.dev` | No current registry record was returned. | Unreserved is not guaranteed; optional defensive registration only after final-name decision. |
| `yuvanolabs.com` | No current registry record was returned. | Highest-priority owner action if YuvanoLabs is the approved company/house mark. |
| `yuvanolabs.dev` | No current registry record was returned. | Optional defensive registration after `yuvanolabs.com`. |
| `yuvanolabs.io` | No registry record or DNS delegation was observed. | Optional defensive registration after `yuvanolabs.com`. |

The `KoraDB.com` observation can be rechecked through
[Verisign RDAP](https://rdap.verisign.com/com/v1/domain/KoraDB.com). A “not found” response is
not a reservation, ownership right, registrar quote, or guarantee that registration will
succeed. Domain checks should be repeated and purchased in the same controlled session.

Use `yuvanolabs.com/KoraDB` as the planned product URL rather than building the brand around a
second product domain. After the primary domain is secured, `docs.yuvanolabs.com/KoraDB` or an
equivalent documentation route can be introduced.

### Repository and package screen

Exact public records were not found at review time for these planned coordinates:

| Ecosystem | Provisional coordinate |
|---|---|
| GitHub | `yuvanolabs/KoraDB` |
| Go | `github.com/yuvanolabs/KoraDB` |
| npm | `@yuvanolabs/KoraDB` |
| Python/PyPI | `yuvanolabs-KoraDB` |
| Maven Central | `com.yuvanolabs:KoraDB` |
| NuGet | `YuvanoLabs.KoraDB` |
| crates.io | `yuvanolabs-KoraDB` |
| GitHub Container Registry | `ghcr.io/yuvanolabs/KoraDB` |
| Docker Hub | `docker.io/yuvanolabs/KoraDB` |

The same API pass did not return exact public packages named `KoraDB` on npm, PyPI, crates.io, or
NuGet, and it did not return an official Docker Hub `library/KoraDB` image. That does **not** make
the unqualified identity acceptable: the product/repository collision still exists, registry
names can be reserved or claimed between checks, and publishing an unqualified package would
increase dependency-confusion and mistaken-identity risk.

The checks used public APIs for
[GitHub](https://api.github.com/repos/yuvanolabs/KoraDB),
[npm](https://registry.npmjs.org/%40yuvanolabs%2FKoraDB),
[PyPI](https://pypi.org/pypi/yuvanolabs-KoraDB/json),
[Maven Central](https://search.maven.org/solrsearch/select?q=g%3A%22com.yuvanolabs%22%20AND%20a%3A%22KoraDB%22&rows=20&wt=json),
[NuGet](https://api.nuget.org/v3-flatcontainer/yuvanolabs.KoraDB/index.json),
[crates.io](https://crates.io/api/v1/crates/yuvanolabs-KoraDB), and
[Docker Hub](https://hub.docker.com/v2/repositories/yuvanolabs/KoraDB/).
An HTTP “not found” result only means no public record was returned; it does not prove the name can
be claimed or that the account/namespace is unreserved.

## Provisional namespace policy

The full machine-readable plan is in [`product.identity.yaml`](../product.identity.yaml).
If counsel and the owner approve **YuvanoLabs KoraDB**, use one company-controlled root across
ecosystems:

| Surface | Planned value | Policy |
|---|---|---|
| House domain | `yuvanolabs.com` | Company-owned registrar account, auto-renew, registry lock where available |
| Repository | `github.com/yuvanolabs/KoraDB` | Organization-owned; never release from an employee's personal namespace |
| Go module | `github.com/yuvanolabs/KoraDB` | Migrate before the first public version |
| Protobuf package | `yuvanolabs.KoraDB.v1` | Company-qualified and API-major-versioned |
| Protobuf Go package | `github.com/yuvanolabs/KoraDB/api/gen/KoraDBv1` | Must match the canonical Go module |
| npm | `@yuvanolabs/KoraDB` | Use a verified organization/scope with publishing protection |
| Python | `yuvanolabs-KoraDB` | Distribution name may contain hyphens; document the eventual import name separately |
| Maven | `com.yuvanolabs:KoraDB` | Use only after control of the corresponding company domain is established |
| NuGet | `YuvanoLabs.KoraDB` | Sign releases and reserve related SDK package names as real packages are published |
| Rust | `yuvanolabs-KoraDB` | Publish only if Rust becomes a supported SDK |
| OCI image | `ghcr.io/yuvanolabs/KoraDB` | Primary image beside source; sign by digest |
| Docker Hub mirror | `docker.io/yuvanolabs/KoraDB` | Optional controlled mirror, not an independent release source |
| Executables | `yuvanolabs-KoraDB`, `yuvanolabs-KoraDB-server` | Avoid the already-used unqualified command name in public installers |

If a new coined product mark is selected, replace the `KoraDB` segment everywhere in one
pre-release migration. Do not partially rename marketing while leaving a conflicting permanent
module, protobuf, or container identity.

## Reservation and account-control checklist

Complete these actions from company-controlled accounts. A public “placeholder” package with no
real artifact is not recommended; reserve names through legitimate signed preview packages when
each registry's policy permits.

1. Confirm YuvanoLabs's exact legal entity, address, authorized registrant, and ownership of the
   product IP.
2. Engage trademark counsel, define target territories and goods/services, perform comprehensive
   clearance, and record the written decision.
3. Decide and approve either a distinctive new mark or the qualified working name.
4. Register the primary domain and essential defensive variants. Enable multi-factor
   authentication, auto-renewal, recovery contacts, transfer lock, and DNS change controls.
5. Create the GitHub organization under company custody. Require at least two organization owners,
   hardware-backed MFA where possible, protected branches/tags, and non-personal recovery.
6. Claim the matching container organization and npm organization/scope. Configure least-privilege
   publishers, short-lived CI credentials or trusted publishing, immutable release tags, and
   signing.
7. Create legitimate initial packages in supported ecosystems only. Enable MFA/trusted publishing,
   provenance/signing, reserved maintainers, and recovery procedures.
8. Migrate the Go module, protobuf package, generated stubs, Buf module option, command paths,
   artifact names, documentation, and examples before `v0.1.0`.
9. Re-run collision searches immediately before announcement and archive dated evidence,
   receipts, account owners, and counsel approval.
10. Change all approvals in `product.identity.yaml`, then run
    `scripts/check-release-identity.ps1`. A normal release build must remain blocked until it
    passes.

## Repository enforcement

`scripts/build-release.ps1` now calls the identity gate before creating public release artifacts.
While the name remains unresolved, an engineer can use:

```powershell
.\scripts\build-release.ps1 -Development
```

That override is for internal test binaries only. It does not approve publication, package upload,
container push, announcement, or customer distribution. The explicit bypass keeps local
development moving while making the release decision visible and auditable.

The current Go module and protobuf namespace have deliberately not been changed yet. Changing
them now would present provisional coordinates as if they were secured and would require
regenerating committed API stubs. Perform that migration once, immediately after the final name,
owner, and repository are approved.

