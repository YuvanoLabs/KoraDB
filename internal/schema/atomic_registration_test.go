package schema

import (
	"context"
	"testing"
)

func TestRegisterRejectsInvalidCandidateWithoutPersistingIt(t *testing.T) {
	r, st := newReg(t)
	if _, err := r.Register(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}

	const conflictingSchema = `
syntax = "proto3";
package example;
message User { string conflicting = 1; }
`
	if _, err := r.Register(context.Background(), "conflicting.proto", conflictingSchema); err == nil {
		t.Fatal("expected duplicate message symbol to make candidate registry invalid")
	}

	schemas, err := r.ListSchemas()
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 1 || schemas[0].Name != "user.proto" {
		t.Fatalf("failed registration was persisted: %#v", schemas)
	}
	if _, err := r.MessageDescriptor("example.User"); err != nil {
		t.Fatalf("active in-memory registry changed after failed registration: %v", err)
	}

	if _, err := NewRegistry(st); err != nil {
		t.Fatalf("failed registration made the database unreopenable: %v", err)
	}
}
