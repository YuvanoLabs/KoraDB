package schema

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/YuvanoLabs/KoraDB/internal/storage"
)

const userV1 = `
syntax = "proto3";
package example;
message User {
  uint64 id = 1;
  string name = 2;
  string email = 3;
}`

const userV2 = `
syntax = "proto3";
package example;
message User {
  uint64 id = 1;
  string name = 2;
  string email = 3;
  int32 age = 4;          // new field, additive
  repeated string tags = 5;
}`

func newReg(t *testing.T) (*Registry, *storage.Store) {
	t.Helper()
	st, err := storage.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r, err := NewRegistry(st)
	if err != nil {
		t.Fatal(err)
	}
	return r, st
}

func TestRegisterAndResolve(t *testing.T) {
	r, _ := newReg(t)
	v, err := r.Register(context.Background(), "user.proto", userV1)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if v != 1 {
		t.Fatalf("first version = %d, want 1", v)
	}
	md, err := r.MessageDescriptor("example.User")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if md.Fields().Len() != 3 {
		t.Fatalf("v1 has %d fields, want 3", md.Fields().Len())
	}
}

func TestRegisterRejectsBadProto(t *testing.T) {
	r, _ := newReg(t)
	_, err := r.Register(context.Background(), "bad.proto", "this is not valid proto")
	if err == nil {
		t.Fatal("expected compile error for invalid proto")
	}
}

// TestSchemaEvolution proves the protobuf differentiator at the schema level:
// re-registering a type with added fields bumps the version, and the new
// descriptor exposes the new fields while remaining the same message type.
func TestSchemaEvolution(t *testing.T) {
	r, _ := newReg(t)
	if _, err := r.Register(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}
	v2, err := r.Register(context.Background(), "user.proto", userV2)
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("evolved version = %d, want 2", v2)
	}
	md, err := r.MessageDescriptor("example.User")
	if err != nil {
		t.Fatal(err)
	}
	if md.Fields().Len() != 5 {
		t.Fatalf("v2 has %d fields, want 5", md.Fields().Len())
	}
	if md.Fields().ByName("age") == nil || md.Fields().ByName("tags") == nil {
		t.Fatal("evolved fields not present in descriptor")
	}
}

// TestPersistenceAcrossReopen confirms schemas survive a registry restart,
// rebuilt from the bytes persisted in the storage layer.
func TestPersistenceAcrossReopen(t *testing.T) {
	st, err := storage.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r1, err := NewRegistry(st)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r1.Register(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}

	// Fresh registry over the same store — must reload the schema from disk.
	r2, err := NewRegistry(st)
	if err != nil {
		t.Fatal(err)
	}
	md, err := r2.MessageDescriptor("example.User")
	if err != nil {
		t.Fatalf("schema did not persist across reopen: %v", err)
	}
	if md.Fields().ByName("email").Kind() != protoreflect.StringKind {
		t.Fatal("reloaded descriptor has wrong field kind")
	}
}
