package storage

import "testing"

func TestVerifyAcceptsValidDatabase(t *testing.T) {
	st := openTemp(t)
	if err := st.Update(func(tx *Txn) error {
		return tx.Put([]byte("users"), []byte("ada"), []byte("durable"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Verify(); err != nil {
		t.Fatalf("verify valid database: %v", err)
	}
}
