package server

import (
	"testing"

	pb "KoraDB/api/gen/KoraDBv1"
)

func TestToFilterRejectsUnsetFilterNode(t *testing.T) {
	if _, err := toFilter(&pb.Filter{}); err == nil {
		t.Fatal("expected unset filter node to be rejected")
	}
}

func TestToFilterAllowsExplicitMatchAll(t *testing.T) {
	filter, err := toFilter(nil)
	if err != nil {
		t.Fatalf("nil filter: %v", err)
	}
	if filter != nil {
		t.Fatalf("nil filter = %T, want nil", filter)
	}
}
