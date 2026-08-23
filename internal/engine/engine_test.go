package engine

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"KoraDB/internal/storage"
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
  int32 age = 4;
  repeated string tags = 5;
}`

func openDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "engine.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.RegisterSchema(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}
	return db
}

func field(t *testing.T, jsonDoc []byte, name string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(jsonDoc, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return m[name]
}

func TestInsertGetUpdateDelete_AutoKey(t *testing.T) {
	db := openDB(t)
	if _, err := db.CreateCollection("users", "example.User", nil); err != nil {
		t.Fatal(err)
	}

	id, err := db.Insert("users", []byte(`{"name":"Alice","email":"alice@example.com"}`))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id != "1" {
		t.Fatalf("first auto id = %q, want 1", id)
	}

	got, err := db.Get("users", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if field(t, got, "name") != "Alice" {
		t.Fatalf("got name %v", field(t, got, "name"))
	}

	if err := db.Update("users", id, []byte(`{"name":"Alice B","email":"aliceb@example.com"}`)); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = db.Get("users", id)
	if field(t, got, "name") != "Alice B" {
		t.Fatalf("update not applied: %v", field(t, got, "name"))
	}

	if err := db.Delete("users", id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.Get("users", id); err == nil {
		t.Fatal("expected ErrNotFound after delete")
	}
}

func TestFieldKey(t *testing.T) {
	db := openDB(t)
	if _, err := db.CreateCollection("users", "example.User", &CollectionOptions{KeyField: "email"}); err != nil {
		t.Fatal(err)
	}
	id, err := db.Insert("users", []byte(`{"name":"Bob","email":"bob@example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	if id != "bob@example.com" {
		t.Fatalf("field key id = %q", id)
	}
	if _, err := db.Get("users", "bob@example.com"); err != nil {
		t.Fatalf("get by field key: %v", err)
	}
}

func TestSecondaryIndexLookup(t *testing.T) {
	db := openDB(t)
	if _, err := db.CreateCollection("users", "example.User", &CollectionOptions{Indexes: []string{"name"}}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Alice", "Bob", "Alice"} {
		if _, err := db.Insert("users", []byte(`{"name":"`+n+`"}`)); err != nil {
			t.Fatal(err)
		}
	}
	var keys [][]byte
	err := db.Store().View(func(tx *storage.Txn) error {
		k, err := db.LookupByIndex(tx, mustMeta(t, db, "users"), "name", "Alice")
		keys = k
		return err
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("index lookup for Alice returned %d keys, want 2", len(keys))
	}
}

// TestDocumentSchemaEvolution is the headline test: documents written under
// schema v1 remain readable after the schema evolves to v2 — with no migration,
// no rewrite, no downtime. This is the core advantage over JSON/BSON databases.
func TestDocumentSchemaEvolution(t *testing.T) {
	db := openDB(t)
	if _, err := db.CreateCollection("users", "example.User", nil); err != nil {
		t.Fatal(err)
	}

	// Write a document under v1 (no age / tags fields exist yet).
	oldID, err := db.Insert("users", []byte(`{"name":"Carol","email":"carol@example.com"}`))
	if err != nil {
		t.Fatal(err)
	}

	// Evolve the schema: add fields 4 (age) and 5 (tags). No data migration.
	if _, err := db.RegisterSchema(context.Background(), "user.proto", userV2); err != nil {
		t.Fatalf("evolve schema: %v", err)
	}

	// The OLD document still reads back cleanly; new fields take zero values.
	got, err := db.Get("users", oldID)
	if err != nil {
		t.Fatalf("read pre-evolution doc: %v", err)
	}
	if field(t, got, "name") != "Carol" {
		t.Fatalf("old field lost after evolution: %v", field(t, got, "name"))
	}
	// age absent in stored bytes -> default 0 (protojson omits zero values).
	if v, ok := field(t, got, "age").(float64); ok && v != 0 {
		t.Fatalf("expected default age, got %v", v)
	}

	// A NEW document can use the new fields immediately.
	newID, err := db.Insert("users", []byte(`{"name":"Dave","email":"dave@example.com","age":30,"tags":["admin","ops"]}`))
	if err != nil {
		t.Fatalf("insert evolved doc: %v", err)
	}
	got, err = db.Get("users", newID)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, got, "age").(float64) != 30 {
		t.Fatalf("new field not stored: %v", field(t, got, "age"))
	}
	t.Log("schema evolution verified: v1 documents readable under v2 with no migration; new fields usable immediately")
}

// --- small test helpers ---

func mustMeta(t *testing.T, db *DB, coll string) *CollectionMeta {
	t.Helper()
	m, err := db.GetCollection(coll)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
