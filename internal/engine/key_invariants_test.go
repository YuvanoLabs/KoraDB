package engine

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNegativeIntegerFieldKeyRoundTrips(t *testing.T) {
	db := openAccounts(t, &CollectionOptions{KeyField: "account_id"})
	id, err := db.Insert("accounts", []byte(`{"account_id":-1009,"owner":"Eve"}`))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id != "-1009" {
		t.Fatalf("id = %q, want -1009", id)
	}
	got, err := db.Get("accounts", id)
	if err != nil {
		t.Fatalf("get by negative key: %v", err)
	}
	if field(t, got, "owner") != "Eve" {
		t.Fatalf("round-trip failed: %s", got)
	}
}

func TestUpdateRejectsPrimaryKeyMutation(t *testing.T) {
	db := openAccounts(t, &CollectionOptions{KeyField: "account_id"})
	if _, err := db.Insert("accounts", []byte(`{"account_id":1,"owner":"A"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Update("accounts", "1", []byte(`{"account_id":2,"owner":"B"}`)); err == nil {
		t.Fatal("expected primary-key mutation to be rejected")
	}
	got, err := db.Get("accounts", "1")
	if err != nil {
		t.Fatalf("original document was not preserved: %v", err)
	}
	if field(t, got, "owner") != "A" {
		t.Fatalf("failed update changed the document: %s", got)
	}
}

func TestUnsupportedPrimaryKeyKindRejectedAtCollectionCreate(t *testing.T) {
	const schemaSource = `
syntax = "proto3";
package example;
message Metric { double value = 1; }
`
	db, err := Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.RegisterSchema(context.Background(), "metric.proto", schemaSource); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCollection("metrics", "example.Metric", &CollectionOptions{KeyField: "value"}); err == nil {
		t.Fatal("expected unsupported double key field to be rejected")
	}
}
