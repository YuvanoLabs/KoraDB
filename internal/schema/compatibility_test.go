package schema

import (
	"context"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestRegisterRejectsBreakingFieldChange(t *testing.T) {
	r, _ := newReg(t)
	if _, err := r.Register(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}
	const incompatible = `
syntax = "proto3";
package example;
message User {
  string id = 1;
  string name = 2;
  string email = 3;
}`
	if _, err := r.Register(context.Background(), "user.proto", incompatible); err == nil {
		t.Fatal("expected field-kind change to be rejected")
	}
	message, err := r.MessageDescriptor("example.User")
	if err != nil {
		t.Fatal(err)
	}
	if got := message.Fields().ByName("id").Kind(); got != protoreflect.Uint64Kind {
		t.Fatalf("failed registration changed active descriptor to %s", got)
	}
}

func TestSchemaHistoryRetainsAcceptedVersions(t *testing.T) {
	r, _ := newReg(t)
	if _, err := r.Register(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register(context.Background(), "user.proto", userV2); err != nil {
		t.Fatal(err)
	}
	history, err := r.SchemaHistory("user.proto")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Version != 1 || history[1].Version != 2 {
		t.Fatalf("history = %#v, want versions 1 and 2", history)
	}
	if history[0].Digest == "" || history[1].Digest == "" {
		t.Fatalf("history does not include descriptor digests: %#v", history)
	}
}

func TestRegisterResolvesImportsFromActiveSchemaCatalog(t *testing.T) {
	r, _ := newReg(t)
	const common = `
syntax = "proto3";
package common;
message Address { string city = 1; }
`
	const user = `
syntax = "proto3";
package example;
import "common.proto";
message User { common.Address address = 1; }
`
	if _, err := r.Register(context.Background(), "common.proto", common); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register(context.Background(), "user.proto", user); err != nil {
		t.Fatalf("registering schema with a catalog import: %v", err)
	}
	if _, err := r.MessageDescriptor("example.User"); err != nil {
		t.Fatal(err)
	}
}
