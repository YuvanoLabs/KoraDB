# YuvanoLabs KoraDB Schema Evolution Contract (working name)

KoraDB stores protobuf wire bytes, so compatible protobuf changes can be read without rewriting
every existing document. That is a major benefit, but it applies only when field-number, wire-type,
collection-key, and index rules are enforced.

The current implementation demonstrates an additive change: new fields with new numbers are added
to a message and old documents remain readable. It does **not yet enforce** the complete contract
described below.

## What “no document rewrite” should mean

An accepted compatible schema version becomes the active descriptor for new reads and writes.
Existing document bytes are not eagerly rewritten. Missing newly added fields read according to
protobuf presence/default rules.

It must not mean:

- every `.proto` edit is safe;
- no index rebuild is ever needed;
- every client can upgrade independently without compatibility checks;
- old schema versions can be discarded;
- application semantics cannot change.

## Proposed compatibility classes

### Automatically allowed

- Add a field using a number and name never used before.
- Add an enum value without changing existing numeric values.
- Add a new message or enum type without conflicting symbols.
- Mark a field deprecated without changing its number or wire type.

### Allowed with an explicit plan

- Remove a field after its name and number are reserved.
- Rename a field when binary compatibility is sufficient and ProtoJSON/API compatibility impact
  is accepted.
- Make a wire-compatible but potentially lossy type change.
- Change presence/cardinality semantics.
- Change an indexed field in a way that requires rebuilding and validating the index.
- Change application validation rules or defaults.

### Rejected for an in-place collection schema

- Change an existing field number.
- Reuse a deleted or reserved field number.
- Change a field to a wire-incompatible type.
- Change the collection primary-key field, number, kind, or cardinality.
- Remove the active primary-key field.
- Remove or make composite an indexed field without an index migration.
- Introduce duplicate fully-qualified symbols across registered files.

When an incompatible change is genuinely needed, create a new message/collection version and run
an explicit, observable data migration.

## Required registration workflow

```text
submit source/module
       |
       v
compile full import graph
       |
       v
compare with active descriptor + collection contracts
       |
       +--> reject with structured compatibility report
       |
       v
stage descriptor version and required index plan
       |
       v
atomically persist catalog version
       |
       v
activate for new operations
```

The registry must validate and build the complete candidate registry **before** committing it. A
failed compile, dependency resolution, compatibility check, or registry rebuild must leave both
durable and in-memory state unchanged.

## Version records

Retain immutable records for every accepted version:

- schema/module name;
- semantic or monotonic version;
- source files and full descriptor closure;
- content digest;
- parent version;
- timestamp and principal;
- compatibility policy and report;
- collection/key/index impact;
- activation status.

The active version should be a separate pointer. Rollback must be allowed only when the target
descriptor is compatible with all data written since it was active, or through an explicit
migration.

## Document envelope

Raw protobuf bytes alone do not say which accepted schema version wrote them. A storage-format
envelope should include at least:

- format version;
- schema or message fingerprint;
- payload length;
- protobuf payload;
- optional revision/etag for optimistic concurrency;
- optional checksum if the selected storage-integrity model requires it.

This enables deterministic diagnostics, migration, export, and future mixed-version reads.

## Imports and modules

Production schema registration should accept a module or file set, not just one source string.
Imports must resolve from:

1. files in the submitted module;
2. version-pinned registered dependencies;
3. approved standard imports.

Dependency versions and digests must be recorded. Mutable “latest dependency” resolution makes
reproducibility and rollback unsafe.

## Collections, keys, and indexes

Every schema change must be checked against every bound collection.

### Primary keys

- Only explicitly supported scalar kinds may be selected.
- The key must be present and non-default if the collection policy requires it.
- Replacement updates must preserve the key or use a dedicated atomic rekey operation.
- Key encoding must round-trip all supported values, including negative signed integers.

### Indexes

- The indexed field must continue to exist with compatible kind and cardinality.
- Changes affecting encoding require a versioned index rebuild.
- Rebuild should occur from a consistent snapshot.
- Activation should switch atomically from old to verified new index.
- Operators need progress, cancellation, disk-capacity guidance, and verification.

## ProtoJSON considerations

The current client API sends documents as ProtoJSON. Binary-wire compatibility alone is not
enough:

- renaming a field changes its JSON name unless an explicit `json_name` policy is used;
- 64-bit integer JSON representation differs from ordinary JavaScript number expectations;
- bytes and enum values have specific ProtoJSON representations;
- implicit scalar presence can conflate “unset” and the zero value;
- parsing and re-emitting JSON can discard information not represented by the active JSON shape.

Until a binary document API exists, use a compatibility policy that protects both binary and JSON
forms.

## Tooling and CI

- Run `buf lint` on the service API and registered-schema fixtures.
- Run `buf breaking` against the last released API descriptor.
- For persisted user schemas, apply at least binary and ProtoJSON compatibility rules plus
  KoraDB-specific key/index rules.
- Test additive, removal/reservation, rename, type-change, dependency, rollback, and reindex cases.
- Fuzz descriptor registration and document decoding.

Useful references:

- [Protobuf: updating a message type](https://protobuf.dev/programming-guides/proto3/#updating)
- [Buf breaking-change categories](https://buf.build/docs/breaking/rules/)

