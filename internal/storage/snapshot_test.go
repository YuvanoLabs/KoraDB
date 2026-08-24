package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotProducesReadableConsistentDatabase(t *testing.T) {
	st := openTemp(t)
	if err := st.Update(func(tx *Txn) error {
		return tx.Put([]byte("users"), []byte("ada"), []byte("durable"))
	}); err != nil {
		t.Fatal(err)
	}

	var image bytes.Buffer
	if err := st.Snapshot(&image); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := os.WriteFile(path, image.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	t.Cleanup(func() { restored.Close() })
	if err := restored.View(func(tx *Txn) error {
		value, err := tx.Get([]byte("users"), []byte("ada"))
		if err != nil {
			return err
		}
		if string(value) != "durable" {
			t.Fatalf("snapshot value = %q, want durable", value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
