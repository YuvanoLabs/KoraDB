# KoraDB Native ABI

`include/koradb.h` defines KoraDB's pre-release C ABI for serverless embedded
language bindings. The implementation opens the same local database file as the
Go SDK; it does not start a KoraDB service or use gRPC.

Build the shared library on each supported target with a C toolchain:

```sh
go build -buildmode=c-shared -o koradb-native.<ext> ./sdk/native/cshared
```

The ABI uses opaque handles and caller-owned C strings. Every returned error or
output string must be released with `KoraDBFreeString`.

This is an ABI foundation, not a published compatibility promise. It must gain
conformance tests, cancellation, pagination, backup/verification calls,
structured error codes, platform packaging, and an ABI-version policy before it
is consumed by the Python, Node.js, or .NET packages.
