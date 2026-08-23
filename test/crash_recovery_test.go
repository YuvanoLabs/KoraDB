// Package dbtest holds integration tests that exercise KoraDB across layers.
//
// crash_recovery_test.go is the Layer 0 gate described in the plan: it proves
// the database is durable and crash-safe, not a toy. The strategy is the
// standard Go re-exec pattern — the test binary launches *itself* as a child
// process (CRASH_CHILD=1), the child commits one record and then hard-exits in
// the middle of a second, uncommitted write. The parent then reopens the same
// file and asserts:
//
//  1. the file is not corrupt (it opens cleanly),
//  2. the committed record survived the abrupt termination (durability), and
//  3. the uncommitted record is absent (atomicity — no torn write).
package dbtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"KoraDB/internal/storage"
)

const (
	crashBucket    = "crashtest"
	committedKey   = "committed"
	committedVal   = "durable"
	uncommittedKey = "uncommitted"
	uncommittedVal = "should-not-survive"
)

// TestMain lets this test binary act as its own crash victim. When CRASH_CHILD
// is set, the process runs the child routine and exits hard instead of running
// the normal test suite.
func TestMain(m *testing.M) {
	if os.Getenv("CRASH_CHILD") == "1" {
		runCrashChild(os.Getenv("CRASH_DB_PATH"))
		// runCrashChild never returns; it kills the process mid-write.
		return
	}
	os.Exit(m.Run())
}

// runCrashChild commits one durable record, then begins a second write and
// terminates the process before that transaction can commit.
func runCrashChild(path string) {
	st, err := storage.Open(path)
	if err != nil {
		os.Exit(10)
	}

	// (1) Commit a record. Update returns only after fsync, so this is durable.
	if err := st.Update(func(tx *storage.Txn) error {
		return tx.Put([]byte(crashBucket), []byte(committedKey), []byte(committedVal))
	}); err != nil {
		os.Exit(11)
	}

	// (2) Begin a second write and crash *inside* the transaction, before
	// commit. bbolt's copy-on-write design means this write must leave no trace.
	_ = st.Update(func(tx *storage.Txn) error {
		if err := tx.Put([]byte(crashBucket), []byte(uncommittedKey), []byte(uncommittedVal)); err != nil {
			os.Exit(12)
		}
		// Simulate abrupt process termination inside an uncommitted transaction.
		// This is not a physical power-loss or mid-fsync simulation: the commit
		// path never begins. The file lock is released by the OS on exit.
		os.Exit(99)
		return nil
	})
	os.Exit(13) // unreachable
}

func TestCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "crash.db")

	// Launch ourselves as the crash victim.
	cmd := exec.Command(os.Args[0], "-test.run=TestCrashRecovery")
	cmd.Env = append(os.Environ(), "CRASH_CHILD=1", "CRASH_DB_PATH="+dbPath)
	out, err := cmd.CombinedOutput()

	// We deliberately exit(99) mid-write, so a non-zero exit is expected.
	if exitErr, ok := err.(*exec.ExitError); ok {
		if code := exitErr.ExitCode(); code != 99 {
			t.Fatalf("child exited %d (output: %s); expected the mid-write crash (99)", code, out)
		}
	} else if err != nil {
		t.Fatalf("launching crash child: %v (output: %s)", err, out)
	} else {
		t.Fatalf("child exited cleanly; expected a mid-write crash")
	}

	// Reopen the file the crashed process left behind.
	st, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("database is corrupt — failed to reopen after crash: %v", err)
	}
	defer st.Close()

	err = st.View(func(tx *storage.Txn) error {
		// (1) Durability: the committed record must be intact.
		got, err := tx.Get([]byte(crashBucket), []byte(committedKey))
		if err != nil {
			t.Fatalf("committed record lost after crash: %v", err)
		}
		if string(got) != committedVal {
			t.Fatalf("committed record corrupted: got %q, want %q", got, committedVal)
		}

		// (2) Atomicity: the uncommitted record must NOT exist.
		_, err = tx.Get([]byte(crashBucket), []byte(uncommittedKey))
		if err != storage.ErrNotFound {
			t.Fatalf("uncommitted (torn) write survived the crash: err=%v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log("crash recovery verified: committed data durable, uncommitted write left no trace, file uncorrupted")
}
