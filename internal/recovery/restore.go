// Package recovery provides safe, offline snapshot-restore primitives.
package recovery

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	// ErrSnapshotTooLarge is returned when a snapshot exceeds the caller's
	// explicit restore limit.
	ErrSnapshotTooLarge = errors.New("snapshot exceeds restore size limit")
	// ErrDestinationExists is returned unless an existing destination has an
	// explicit overwrite and rollback policy.
	ErrDestinationExists = errors.New("restore destination already exists")
	// ErrRollbackPathRequired prevents an overwrite that would discard the
	// previous database without retaining a rollback copy.
	ErrRollbackPathRequired = errors.New("restore rollback path is required when overwriting a database")
	// ErrRollbackPathExists prevents an accidental overwrite of an existing
	// rollback database.
	ErrRollbackPathExists = errors.New("restore rollback path already exists")
	// ErrSourceMatchesDestination prevents self-restores through RestoreFile.
	ErrSourceMatchesDestination = errors.New("restore source and destination are the same file")
)

// Options controls the safety boundary of Restore and RestoreFile. MaxBytes is
// required so callers do not accept an unbounded snapshot stream. Overwriting
// an existing destination requires RollbackPath in the same directory; the
// original database is atomically moved there before the restored snapshot is
// published.
type Options struct {
	MaxBytes     int64
	Overwrite    bool
	RollbackPath string
}

// RestoreFile restores a bbolt snapshot file to destination. It is an offline
// operation: if destination exists, the function first verifies that it can
// acquire the database lock. A running KoraDB process must be stopped first.
func RestoreFile(snapshotPath, destination string, options Options) (int64, error) {
	sourceInfo, err := os.Stat(snapshotPath)
	if err != nil {
		return 0, fmt.Errorf("inspect restore snapshot %q: %w", snapshotPath, err)
	}
	if sourceInfo.IsDir() {
		return 0, fmt.Errorf("restore snapshot %q is a directory", snapshotPath)
	}
	if destinationInfo, err := os.Stat(destination); err == nil && os.SameFile(sourceInfo, destinationInfo) {
		return 0, ErrSourceMatchesDestination
	} else if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("inspect restore destination %q: %w", destination, err)
	}

	snapshot, err := os.Open(snapshotPath)
	if err != nil {
		return 0, fmt.Errorf("open restore snapshot %q: %w", snapshotPath, err)
	}
	defer snapshot.Close()
	return Restore(snapshot, destination, options)
}

// Restore validates and publishes a bbolt snapshot from r. It writes only a
// staging file until integrity validation succeeds. On an overwrite, the
// previous database is retained at Options.RollbackPath.
func Restore(r io.Reader, destination string, options Options) (int64, error) {
	if r == nil {
		return 0, errors.New("restore snapshot reader is nil")
	}
	if options.MaxBytes <= 0 {
		return 0, errors.New("restore size limit must be positive")
	}
	if destination == "" {
		return 0, errors.New("restore destination is required")
	}

	destination = filepath.Clean(destination)
	destinationInfo, statErr := os.Stat(destination)
	destinationExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return 0, fmt.Errorf("inspect restore destination %q: %w", destination, statErr)
	}
	if destinationExists && destinationInfo.IsDir() {
		return 0, fmt.Errorf("restore destination %q is a directory", destination)
	}
	if destinationExists && !options.Overwrite {
		return 0, fmt.Errorf("%w: %q", ErrDestinationExists, destination)
	}

	rollbackPath, err := validateRollbackPath(destination, destinationExists, options)
	if err != nil {
		return 0, err
	}
	if destinationExists {
		if err := verifyDestinationIsOffline(destination); err != nil {
			return 0, err
		}
	}

	staged, err := os.CreateTemp(filepath.Dir(destination), ".koradb-restore-*")
	if err != nil {
		return 0, fmt.Errorf("create restore staging file: %w", err)
	}
	stagedPath := staged.Name()
	published := false
	closed := false
	defer func() {
		if !closed {
			_ = staged.Close()
		}
		if !published {
			_ = os.Remove(stagedPath)
		}
	}()

	if err := staged.Chmod(0o600); err != nil {
		return 0, fmt.Errorf("secure restore staging file: %w", err)
	}
	bytesWritten, err := io.Copy(staged, io.LimitReader(r, options.MaxBytes+1))
	if err != nil {
		return bytesWritten, fmt.Errorf("stage restore snapshot: %w", err)
	}
	if bytesWritten > options.MaxBytes {
		return bytesWritten, fmt.Errorf("%w: %d bytes is greater than %d bytes", ErrSnapshotTooLarge, bytesWritten, options.MaxBytes)
	}
	if err := staged.Sync(); err != nil {
		return bytesWritten, fmt.Errorf("sync restore staging file: %w", err)
	}
	if err := staged.Close(); err != nil {
		return bytesWritten, fmt.Errorf("close restore staging file: %w", err)
	}
	closed = true
	if err := verifySnapshot(stagedPath); err != nil {
		return bytesWritten, err
	}

	if destinationExists {
		if err := os.Rename(destination, rollbackPath); err != nil {
			return bytesWritten, fmt.Errorf("move existing database to rollback path: %w", err)
		}
		if err := os.Rename(stagedPath, destination); err != nil {
			if rollbackErr := os.Rename(rollbackPath, destination); rollbackErr != nil {
				return bytesWritten, fmt.Errorf("publish restored database: %w; rollback also failed: %v", err, rollbackErr)
			}
			return bytesWritten, fmt.Errorf("publish restored database: %w", err)
		}
		published = true
		return bytesWritten, nil
	}

	if err := os.Rename(stagedPath, destination); err != nil {
		return bytesWritten, fmt.Errorf("publish restored database: %w", err)
	}
	published = true
	return bytesWritten, nil
}

func validateRollbackPath(destination string, destinationExists bool, options Options) (string, error) {
	if !destinationExists {
		if options.RollbackPath != "" {
			return "", errors.New("restore rollback path is only valid when overwriting a database")
		}
		return "", nil
	}
	if !options.Overwrite {
		return "", nil
	}
	if options.RollbackPath == "" {
		return "", ErrRollbackPathRequired
	}

	rollbackPath := filepath.Clean(options.RollbackPath)
	if filepath.Dir(rollbackPath) != filepath.Dir(destination) {
		return "", errors.New("restore rollback path must be in the destination directory")
	}
	if rollbackPath == destination {
		return "", errors.New("restore rollback path must differ from the destination")
	}
	if _, err := os.Lstat(rollbackPath); err == nil {
		return "", fmt.Errorf("%w: %q", ErrRollbackPathExists, rollbackPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect restore rollback path %q: %w", rollbackPath, err)
	}
	return rollbackPath, nil
}

func verifyDestinationIsOffline(path string) error {
	database, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if err != nil {
		return fmt.Errorf("open restore destination %q: stop every KoraDB process before restoring: %w", path, err)
	}
	return database.Close()
}

func verifySnapshot(path string) error {
	database, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if err != nil {
		return fmt.Errorf("open staged restore snapshot: %w", err)
	}
	defer database.Close()
	if err := database.View(func(tx *bolt.Tx) error {
		for checkErr := range tx.Check() {
			return checkErr
		}
		return nil
	}); err != nil {
		return fmt.Errorf("verify staged restore snapshot: %w", err)
	}
	return nil
}
