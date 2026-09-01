package query

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/YuvanoLabs/KoraDB/internal/engine"
)

const personProto = `
syntax = "proto3";
package example;
message Person {
  string name = 1;
  int32 age = 2;
  string city = 3;
}`

func setup(t *testing.T, indexes ...string) *engine.DB {
	t.Helper()
	db, err := engine.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.RegisterSchema(context.Background(), "person.proto", personProto); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCollection("people", "example.Person", &engine.CollectionOptions{Indexes: indexes}); err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{
		`{"name":"Alice","age":30,"city":"NYC"}`,
		`{"name":"Bob","age":25,"city":"LA"}`,
		`{"name":"Carol","age":40,"city":"NYC"}`,
		`{"name":"Dave","age":30,"city":"SF"}`,
	} {
		if _, err := db.Insert("people", []byte(doc)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func names(t *testing.T, rs []Result) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, r := range rs {
		var m map[string]any
		if err := json.Unmarshal(r.JSON, &m); err != nil {
			t.Fatal(err)
		}
		out[m["name"].(string)] = true
	}
	return out
}

func TestEqualityFullScan(t *testing.T) {
	db := setup(t) // no indexes -> full scan path
	rs, err := Execute(db, "people", Cmp{Field: "city", Op: Eq, Value: "NYC"})
	if err != nil {
		t.Fatal(err)
	}
	got := names(t, rs)
	if len(got) != 2 || !got["Alice"] || !got["Carol"] {
		t.Fatalf("city==NYC returned %v", got)
	}
}

func TestEqualityIndexSeeded(t *testing.T) {
	db := setup(t, "city") // indexed -> index-seeded path
	rs, err := Execute(db, "people", Cmp{Field: "city", Op: Eq, Value: "NYC"})
	if err != nil {
		t.Fatal(err)
	}
	got := names(t, rs)
	if len(got) != 2 || !got["Alice"] || !got["Carol"] {
		t.Fatalf("indexed city==NYC returned %v", got)
	}
}

func TestRangeAndComposite(t *testing.T) {
	db := setup(t, "city")
	// city == NYC  AND  age > 30   -> only Carol (40); Alice is exactly 30.
	rs, err := Execute(db, "people", And{Filters: []Filter{
		Cmp{Field: "city", Op: Eq, Value: "NYC"},
		Cmp{Field: "age", Op: Gt, Value: "30"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := names(t, rs)
	if len(got) != 1 || !got["Carol"] {
		t.Fatalf("composite query returned %v, want {Carol}", got)
	}
}

func TestOr(t *testing.T) {
	db := setup(t)
	// age < 26  OR  city == SF  -> Bob (25), Dave (SF)
	rs, err := Execute(db, "people", Or{Filters: []Filter{
		Cmp{Field: "age", Op: Lt, Value: "26"},
		Cmp{Field: "city", Op: Eq, Value: "SF"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := names(t, rs)
	if len(got) != 2 || !got["Bob"] || !got["Dave"] {
		t.Fatalf("OR query returned %v", got)
	}
}

func TestMatchAll(t *testing.T) {
	db := setup(t)
	rs, err := Execute(db, "people", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 4 {
		t.Fatalf("nil filter returned %d docs, want 4", len(rs))
	}
}

// TestIndexAndScanAgree proves the optimizer is sound: the same query returns
// the same results whether or not an index is used to seed it.
func TestIndexAndScanAgree(t *testing.T) {
	indexed := setup(t, "city")
	scanned := setup(t)
	q := Cmp{Field: "city", Op: Eq, Value: "NYC"}
	a, err := Execute(indexed, "people", q)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Execute(scanned, "people", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(names(t, a)) != len(names(t, b)) {
		t.Fatalf("index path (%d) and scan path (%d) disagree", len(a), len(b))
	}
}
