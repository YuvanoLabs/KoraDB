# Offline Restore

KoraDB snapshots are database-file images. Restoring one is an offline
operation: stop every process that uses the destination database first.

The internal `recovery.RestoreFile` primitive validates the staged snapshot
with bbolt before publishing it. It requires a positive maximum size and, when
overwriting a database, requires a new rollback path in the same directory.
The existing database is moved to that rollback path before the validated
snapshot is put in place.

Use the dedicated offline command:

```text
KoraDB-restore --max-bytes <bytes> <snapshot.db> <destination.db>
KoraDB-restore --max-bytes <bytes> --overwrite --rollback <previous.db> <snapshot.db> <destination.db>
```

This command is intentionally not a general server RPC. A remote restore must
be introduced only with an authenticated maintenance mode, operator audit
trail, backup retention policy, and tested service shutdown workflow. A
documented, versioned disaster-recovery runbook remains release work; do not
treat a raw snapshot alone as a complete production backup-and-restore
solution.
