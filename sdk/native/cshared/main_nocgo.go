//go:build !cgo

// This placeholder keeps ordinary `go build ./...` checks valid on hosts
// without a C toolchain. The native ABI itself is built only with CGO enabled
// and `-buildmode=c-shared`.
package main

func main() {}
