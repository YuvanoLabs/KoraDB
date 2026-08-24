package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ErrSnapshotTooLarge is returned when a snapshot exceeds the caller's
// explicit validation limit.
var ErrSnapshotTooLarge = errors.New("snapshot exceeds validation size limit")

// ValidateSnapshot checks that r contains a readable bbolt snapshot without
// modifying a live database. maxBytes must be a positive upper bound chosen by
// the caller. The returned byte count includes all bytes read during validation.
func ValidateSnapshot(r io.Reader, maxBytes int64) (int64, error) {
	if maxBytes <= 0 {
		return 0, fmt.Errorf("snapshot validation size limit must be positive")
	}

	temporaryFile, err := os.CreateTemp("", "koradb-snapshot-validation-*.db")
	if err != nil {
		return 0, fmt.Errorf("create temporary snapshot file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	limited := io.LimitReader(r, maxBytes+1)
	bytesRead, copyErr := io.Copy(temporaryFile, limited)
	closeErr := temporaryFile.Close()
	if copyErr != nil {
		return bytesRead, fmt.Errorf("copy snapshot for validation: %w", copyErr)
	}
	if closeErr != nil {
		return bytesRead, fmt.Errorf("close temporary snapshot file: %w", closeErr)
	}
	if bytesRead > maxBytes {
		return bytesRead, fmt.Errorf("%w: %d bytes is greater than %d bytes", ErrSnapshotTooLarge, bytesRead, maxBytes)
	}

	database, err := bolt.Open(temporaryPath, 0o600, &bolt.Options{
		ReadOnly: true,
		Timeout:  time.Second,
	})
	if err != nil {
		return bytesRead, fmt.Errorf("open snapshot for validation: %w", err)
	}
	defer database.Close()

	if err := database.View(func(tx *bolt.Tx) error {
		for checkErr := range tx.Check() {
			return checkErr
		}
		return nil
	}); err != nil {
		return bytesRead, fmt.Errorf("verify snapshot integrity: %w", err)
	}

	return bytesRead, nil
}
