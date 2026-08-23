package auth

import (
	"path/filepath"
	"testing"

	"KoraDB/internal/storage"
)

func tempStore(t *testing.T) *storage.Store {
	t.Helper()
	st, err := storage.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestKeyRoundTrip(t *testing.T) {
	st := tempStore(t)
	token, keyID, err := CreateKey(st, "svc", RoleReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Authenticate(st, token)
	if err != nil {
		t.Fatalf("authenticate valid token: %v", err)
	}
	if p.KeyID != keyID || p.Name != "svc" || p.Role != RoleReadWrite {
		t.Fatalf("principal mismatch: %+v", p)
	}
}

func TestAuthenticateRejectsBadTokens(t *testing.T) {
	st := tempStore(t)
	good, _, _ := CreateKey(st, "svc", RoleAdmin)
	cases := []string{
		"",
		"garbage",
		"pdb_short",
		"pdb_" + "00000000000000ff" + "_" + "tooShortSecret",
		good + "x",         // tampered secret length
		good[:len(good)-1], // truncated
	}
	for _, tok := range cases {
		if _, err := Authenticate(st, tok); err == nil {
			t.Fatalf("expected rejection for token %q", tok)
		}
	}
	// A well-formed token for a non-existent key must also fail.
	fake := "pdb_" + "0011223344556677" + "_" + repeat("a", 64)
	if _, err := Authenticate(st, fake); err != ErrInvalidToken {
		t.Fatalf("unknown key: got %v, want ErrInvalidToken", err)
	}
}

func TestRevoke(t *testing.T) {
	st := tempStore(t)
	token, keyID, _ := CreateKey(st, "svc", RoleReadOnly)
	if _, err := Authenticate(st, token); err != nil {
		t.Fatal(err)
	}
	if err := Revoke(st, keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(st, token); err != ErrInvalidToken {
		t.Fatalf("after revoke: got %v, want ErrInvalidToken", err)
	}
}

func TestRBACFailClosed(t *testing.T) {
	ro := &Principal{Role: RoleReadOnly}
	rw := &Principal{Role: RoleReadWrite}
	admin := &Principal{Role: RoleAdmin}

	// Readonly can read but not write or admin.
	mustAllow(t, ro, "/KoraDB.v1.KoraDB/Get")
	mustDeny(t, ro, "/KoraDB.v1.KoraDB/Insert")
	mustDeny(t, ro, "/KoraDB.v1.KoraDB/CreateKey")

	// Readwrite can read+write but not admin.
	mustAllow(t, rw, "/KoraDB.v1.KoraDB/Insert")
	mustAllow(t, rw, "/KoraDB.v1.KoraDB/Get")
	mustDeny(t, rw, "/KoraDB.v1.KoraDB/PutSchema")

	// Admin can do everything mapped.
	mustAllow(t, admin, "/KoraDB.v1.KoraDB/PutSchema")
	mustAllow(t, admin, "/KoraDB.v1.KoraDB/RevokeKey")

	// Fail-closed: an UNMAPPED method is denied even for admin.
	mustDeny(t, admin, "/KoraDB.v1.KoraDB/SomeFutureMethod")
	// A nil principal is denied.
	if (*Principal)(nil).Can("/KoraDB.v1.KoraDB/Get") {
		t.Fatal("nil principal must be denied")
	}
}

func mustAllow(t *testing.T, p *Principal, method string) {
	t.Helper()
	if !p.Can(method) {
		t.Errorf("role %s should be allowed %s", p.Role, method)
	}
}
func mustDeny(t *testing.T, p *Principal, method string) {
	t.Helper()
	if p.Can(method) {
		t.Errorf("role %s should be DENIED %s", p.Role, method)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s[0])
	}
	return string(out)
}
