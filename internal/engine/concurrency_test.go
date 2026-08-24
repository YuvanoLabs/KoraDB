package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestConcurrentInsertAndSchemaEvolve guards against the lock-order inversion
// between the schema registry mutex and the storage write transaction. Insert
// holds a bbolt write txn and then takes the registry read lock; Register must
// NOT hold the registry lock while opening a write txn, or these deadlock.
//
// The test runs many inserts concurrently with repeated schema re-registration
// and fails (via timeout) if the system hangs. A single-threaded suite cannot
// catch this.
func TestConcurrentInsertAndSchemaEvolve(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "conc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.RegisterSchema(context.Background(), "user.proto", userV1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCollection("users", "example.User", nil); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup

		// Writers: many concurrent inserts.
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					doc := fmt.Sprintf(`{"name":"u%d-%d","email":"u%d-%d@x.com"}`, w, i, w, i)
					if _, err := db.Insert("users", []byte(doc)); err != nil {
						t.Errorf("insert: %v", err)
						return
					}
				}
			}(w)
		}

		// Schema churn: re-register a compatible evolution repeatedly. Once the
		// field in userV2 exists, rolling back to userV1 would be an intentional
		// schema-compatibility violation.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if _, err := db.RegisterSchema(context.Background(), "user.proto", userV2); err != nil {
					t.Errorf("register: %v", err)
					return
				}
			}
		}()

		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// completed without deadlock
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK: concurrent Insert + PutSchema did not finish within 15s")
	}
}
