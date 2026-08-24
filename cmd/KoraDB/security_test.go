package main

import "testing"

func TestOpenBackendRejectsTokenWithoutTLS(t *testing.T) {
	if _, err := openBackend("", "127.0.0.1:50051", "kdb_token", nil); err == nil {
		t.Fatal("expected plaintext token connection to be rejected")
	}
}

