// Package storage is Layer 0 of KoraDB: the durable storage spine.
//
// It is a thin wrapper over bbolt (a crash-safe, single-writer/many-reader
// B+tree). Everything above this layer — schemas, documents, indexes — is just
// bytes-by-key inside named buckets. Durability and crash-safety are provided
// by bbolt: every read-write transaction is fsync'd on commit, and the
// copy-on-write page layout and commit protocol provide the underlying
// crash-recovery model. KoraDB still needs platform/filesystem fault testing
// for the exact durability guarantees it publishes.
//
// The whole database is a single file on disk.
package storage

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ErrNotFound is returned by Get when a key does not exist.
var ErrNotFound = errors.New("storage: key not found")

// Store is a handle to an open database file.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the database file at path. It blocks up to a short
// timeout if another process holds the file lock, rather than hanging forever.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("storage: open %q: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Close flushes and releases the database file.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the on-disk path of the database file.
func (s *Store) Path() string {
	return s.db.Path()
}

// Update runs fn inside a single read-write transaction. If fn returns nil the
// transaction is committed atomically and durably (fsync); if fn returns an
// error or panics, the transaction is rolled back and the file is unchanged.
// Only one Update runs at a time (single writer).
func (s *Store) Update(fn func(*Txn) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return fn(&Txn{tx: tx, writable: true})
	})
}

// View runs fn inside a read-only transaction. Many Views run concurrently with
// each other and with a single Update, each seeing a consistent snapshot.
func (s *Store) View(fn func(*Txn) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		return fn(&Txn{tx: tx, writable: false})
	})
}

// Txn is a transaction handle scoped to a set of named buckets. All mutating
// methods are only valid inside Update; calling them inside View returns an
// error from the underlying engine.
type Txn struct {
	tx       *bolt.Tx
	writable bool
}

// Put stores val under key in bucket, creating the bucket if necessary.
func (t *Txn) Put(bucket, key, val []byte) error {
	b, err := t.tx.CreateBucketIfNotExists(bucket)
	if err != nil {
		return fmt.Errorf("storage: bucket %q: %w", bucket, err)
	}
	return b.Put(key, val)
}

// Get returns a copy of the value stored under key in bucket. It returns
// ErrNotFound if the bucket or key does not exist. The returned slice is owned
// by the caller and remains valid after the transaction ends.
func (t *Txn) Get(bucket, key []byte) ([]byte, error) {
	b := t.tx.Bucket(bucket)
	if b == nil {
		return nil, ErrNotFound
	}
	v := b.Get(key)
	if v == nil {
		return nil, ErrNotFound
	}
	// bbolt values are only valid for the life of the transaction; copy out.
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

// Has reports whether key exists in bucket.
func (t *Txn) Has(bucket, key []byte) (bool, error) {
	b := t.tx.Bucket(bucket)
	if b == nil {
		return false, nil
	}
	return b.Get(key) != nil, nil
}

// Delete removes key from bucket. Deleting a missing key is not an error.
func (t *Txn) Delete(bucket, key []byte) error {
	b := t.tx.Bucket(bucket)
	if b == nil {
		return nil
	}
	return b.Delete(key)
}

// Scan calls fn for every key/value pair in bucket, in key order. The slices
// passed to fn are only valid for the duration of the call; copy if retaining.
// If the bucket does not exist, Scan returns nil without calling fn.
func (t *Txn) Scan(bucket []byte, fn func(key, val []byte) error) error {
	b := t.tx.Bucket(bucket)
	if b == nil {
		return nil
	}
	return b.ForEach(fn)
}

// PrefixScan calls fn for every key in bucket that begins with prefix, in key
// order. Used by the index layer, where keys are composite (value + id).
func (t *Txn) PrefixScan(bucket, prefix []byte, fn func(key, val []byte) error) error {
	b := t.tx.Bucket(bucket)
	if b == nil {
		return nil
	}
	c := b.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

// NextSequence returns a monotonically increasing uint64 unique within bucket.
// Used to mint auto-generated document IDs.
func (t *Txn) NextSequence(bucket []byte) (uint64, error) {
	b, err := t.tx.CreateBucketIfNotExists(bucket)
	if err != nil {
		return 0, err
	}
	return b.NextSequence()
}

// DeleteBucket removes an entire bucket and all its contents. Used when dropping
// a collection or rebuilding an index.
func (t *Txn) DeleteBucket(bucket []byte) error {
	err := t.tx.DeleteBucket(bucket)
	if err != nil && !errors.Is(err, bolt.ErrBucketNotFound) {
		return err
	}
	return nil
}

// BucketExists reports whether a bucket exists.
func (t *Txn) BucketExists(bucket []byte) bool {
	return t.tx.Bucket(bucket) != nil
}
