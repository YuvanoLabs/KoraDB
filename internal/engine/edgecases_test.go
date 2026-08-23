package engine

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

const accountProto = `
syntax = "proto3";
package example;
message Account {
  int64 account_id = 1;
  string owner = 2;
  bool active = 3;
  repeated string roles = 4;
}`

func openAccounts(t *testing.T, opts *CollectionOptions) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "acc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.RegisterSchema(context.Background(), "account.proto", accountProto); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCollection("accounts", "example.Account", opts); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestIntegerFieldKey covers the integer primary-key path (previously untested):
// insert with an int64 key, then fetch it back by the same id string.
func TestIntegerFieldKey(t *testing.T) {
	db := openAccounts(t, &CollectionOptions{KeyField: "account_id"})
	id, err := db.Insert("accounts", []byte(`{"account_id":1009,"owner":"Eve","active":true}`))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id != "1009" {
		t.Fatalf("integer key id = %q, want 1009", id)
	}
	got, err := db.Get("accounts", "1009")
	if err != nil {
		t.Fatalf("get by integer key: %v", err)
	}
	if field(t, got, "owner") != "Eve" {
		t.Fatalf("round-trip failed: %s", got)
	}
}

// TestDuplicateKeyRejected confirms field-keyed inserts are create-only.
func TestDuplicateKeyRejected(t *testing.T) {
	db := openAccounts(t, &CollectionOptions{KeyField: "account_id"})
	if _, err := db.Insert("accounts", []byte(`{"account_id":1,"owner":"A"}`)); err != nil {
		t.Fatal(err)
	}
	_, err := db.Insert("accounts", []byte(`{"account_id":1,"owner":"B"}`))
	if !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("duplicate insert err = %v, want ErrDuplicateKey", err)
	}
	// The original must be untouched.
	got, _ := db.Get("accounts", "1")
	if field(t, got, "owner") != "A" {
		t.Fatalf("duplicate insert clobbered original: %s", got)
	}
}

// TestRepeatedFieldNotIndexable rejects an index on a repeated field at create
// time rather than silently writing garbage index entries.
func TestRepeatedFieldNotIndexable(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "acc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.RegisterSchema(context.Background(), "account.proto", accountProto); err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateCollection("accounts", "example.Account", &CollectionOptions{Indexes: []string{"roles"}})
	if err == nil {
		t.Fatal("expected error indexing a repeated field")
	}
}

// TestRepeatedFieldKeyRejected rejects a repeated field as the primary key.
func TestRepeatedFieldKeyRejected(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "acc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.RegisterSchema(context.Background(), "account.proto", accountProto); err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateCollection("accounts", "example.Account", &CollectionOptions{KeyField: "roles"})
	if err == nil {
		t.Fatal("expected error using a repeated field as primary key")
	}
}

// TestBoolFieldKey covers the bool branch of the key round-trip.
func TestBoolFieldKey(t *testing.T) {
	db := openAccounts(t, &CollectionOptions{KeyField: "active"})
	id, err := db.Insert("accounts", []byte(`{"owner":"X","active":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if id != "true" {
		t.Fatalf("bool key id = %q, want true", id)
	}
	if _, err := db.Get("accounts", "true"); err != nil {
		t.Fatalf("get by bool key: %v", err)
	}
}
