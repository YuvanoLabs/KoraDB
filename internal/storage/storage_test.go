package storage

import (
	"path/filepath"
	"testing"
)

func TestPutGetDelete(t *testing.T) {
	st := openTemp(t)

	bucket := []byte("users")
	if err := st.Update(func(tx *Txn) error {
		return tx.Put(bucket, []byte("u1"), []byte("alice"))
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	var got []byte
	if err := st.View(func(tx *Txn) error {
		v, err := tx.Get(bucket, []byte("u1"))
		got = v
		return err
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "alice" {
		t.Fatalf("got %q, want alice", got)
	}

	// Missing key -> ErrNotFound.
	err := st.View(func(tx *Txn) error {
		_, err := tx.Get(bucket, []byte("missing"))
		return err
	})
	if err != ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}

	// Delete then confirm gone.
	if err := st.Update(func(tx *Txn) error {
		return tx.Delete(bucket, []byte("u1"))
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	err = st.View(func(tx *Txn) error {
		_, err := tx.Get(bucket, []byte("u1"))
		return err
	})
	if err != ErrNotFound {
		t.Fatalf("after delete got %v, want ErrNotFound", err)
	}
}

func TestScanOrderAndPrefix(t *testing.T) {
	st := openTemp(t)
	bucket := []byte("k")
	pairs := map[string]string{"a:1": "x", "a:2": "y", "b:1": "z"}
	if err := st.Update(func(tx *Txn) error {
		for k, v := range pairs {
			if err := tx.Put(bucket, []byte(k), []byte(v)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var keys []string
	if err := st.View(func(tx *Txn) error {
		return tx.Scan(bucket, func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0] != "a:1" || keys[1] != "a:2" || keys[2] != "b:1" {
		t.Fatalf("scan order wrong: %v", keys)
	}

	var prefixed []string
	if err := st.View(func(tx *Txn) error {
		return tx.PrefixScan(bucket, []byte("a:"), func(k, v []byte) error {
			prefixed = append(prefixed, string(k))
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	if len(prefixed) != 2 {
		t.Fatalf("prefix scan got %v, want 2 keys", prefixed)
	}
}

func TestRollbackOnError(t *testing.T) {
	st := openTemp(t)
	bucket := []byte("k")
	sentinel := errInjected
	err := st.Update(func(tx *Txn) error {
		if err := tx.Put(bucket, []byte("k1"), []byte("v1")); err != nil {
			return err
		}
		return sentinel // abort: k1 must NOT be committed
	})
	if err != sentinel {
		t.Fatalf("got %v, want injected error", err)
	}
	err = st.View(func(tx *Txn) error {
		_, err := tx.Get(bucket, []byte("k1"))
		return err
	})
	if err != ErrNotFound {
		t.Fatalf("rolled-back write leaked: got %v, want ErrNotFound", err)
	}
}

var errInjected = &injErr{}

type injErr struct{}

func (*injErr) Error() string { return "injected" }

func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
