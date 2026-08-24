package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestRestoreFilePublishesValidatedSnapshot(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := writeSnapshot(t, dir, "source.db", []byte("new"))
	destination := filepath.Join(dir, "restored.db")

	bytesWritten, err := RestoreFile(snapshotPath, destination, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if bytesWritten == 0 {
		t.Fatal("expected restore to write snapshot data")
	}
	if got := readValue(t, destination); string(got) != "new" {
		t.Fatalf("restored value = %q, want %q", got, "new")
	}
}

func TestRestoreFileRequiresExplicitRollbackForOverwrite(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := writeSnapshot(t, dir, "source.db", []byte("new"))
	destination := writeDatabase(t, dir, "destination.db", []byte("old"))

	_, err := RestoreFile(snapshotPath, destination, Options{MaxBytes: 1 << 20})
	if !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("RestoreFile error = %v, want ErrDestinationExists", err)
	}
	if got := readValue(t, destination); string(got) != "old" {
		t.Fatalf("destination changed after refused restore: got %q", got)
	}
}

func TestRestoreFileRetainsRollbackDatabase(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := writeSnapshot(t, dir, "source.db", []byte("new"))
	destination := writeDatabase(t, dir, "destination.db", []byte("old"))
	rollbackPath := filepath.Join(dir, "destination-before-restore.db")

	_, err := RestoreFile(snapshotPath, destination, Options{
		MaxBytes:     1 << 20,
		Overwrite:    true,
		RollbackPath: rollbackPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readValue(t, destination); string(got) != "new" {
		t.Fatalf("restored value = %q, want %q", got, "new")
	}
	if got := readValue(t, rollbackPath); string(got) != "old" {
		t.Fatalf("rollback value = %q, want %q", got, "old")
	}
}

func writeSnapshot(t *testing.T, dir, name string, value []byte) string {
	t.Helper()
	databasePath := writeDatabase(t, dir, name, value)
	database, err := bolt.Open(databasePath, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(dir, name+".snapshot")
	snapshot, err := os.Create(snapshotPath)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	err = database.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(snapshot)
		return err
	})
	closeErr := snapshot.Close()
	database.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return snapshotPath
}

func writeDatabase(t *testing.T, dir, name string, value []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	database, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = database.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("test"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("value"), value)
	})
	closeErr := database.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return path
}

func readValue(t *testing.T, path string) []byte {
	t.Helper()
	database, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var value []byte
	if err := database.View(func(tx *bolt.Tx) error {
		value = append(value, tx.Bucket([]byte("test")).Get([]byte("value"))...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return value
}
