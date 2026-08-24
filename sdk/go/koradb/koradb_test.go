package koradb

import (
	"context"
	"path/filepath"
	"testing"
)

const testUserSchema = `
syntax = "proto3";
package example;
message User {
  string email = 1;
  string city = 2;
}`

func TestEmbeddedWorkflow(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "users.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.RegisterSchema(ctx, "user.proto", testUserSchema); err != nil {
		t.Fatal(err)
	}
	users, err := db.CreateCollection(ctx, CollectionSpec{
		Name: "users", MessageType: "example.User", KeyField: "email", Indexes: []string{"city"},
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := users.InsertJSON(ctx, []byte(`{"email":"ada@example.com","city":"Pune"}`))
	if err != nil {
		t.Fatal(err)
	}
	if id != "ada@example.com" {
		t.Fatalf("id = %q", id)
	}
	results, err := users.QueryJSON(ctx, "city", Equal, "Pune")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != id {
		t.Fatalf("results = %#v", results)
	}
}
