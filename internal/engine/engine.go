// Package engine is Layer 2 of KoraDB: the document engine.
//
// It binds the storage spine (Layer 0) and the schema registry (Layer 1)
// together into the database users actually talk to. A Collection is a named
// set of documents that all share one protobuf message type. Documents enter
// and leave as JSON (parsed/rendered with protojson against the collection's
// schema) but are stored on disk as protobuf wire bytes. This avoids repeating
// field names in every document; the actual size difference versus JSON/BSON
// depends on the document shape and must be measured on representative data.
package engine

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"KoraDB/internal/index"
	"KoraDB/internal/schema"
	"KoraDB/internal/storage"
)

// ErrNotFound is returned when a collection or document does not exist.
var ErrNotFound = errors.New("engine: not found")

// ErrExists is returned when creating a collection that already exists.
var ErrExists = errors.New("engine: collection already exists")

// ErrDuplicateKey is returned when inserting a document whose primary key
// already exists in a field-keyed collection.
var ErrDuplicateKey = errors.New("engine: duplicate primary key")

// collectionsBucket holds one metadata record per collection.
var collectionsBucket = []byte("__collections__")

// dataBucket returns the bucket name holding a collection's documents.
func dataBucket(coll string) []byte { return []byte("data:" + coll) }

// KeyKind describes how a collection's primary key is produced.
type KeyKind string

const (
	// KeyAuto mints a monotonically increasing uint64 id per insert.
	KeyAuto KeyKind = "auto"
	// KeyField uses the value of a named document field as the primary key.
	KeyField KeyKind = "field"
)

// CollectionMeta is the persisted definition of a collection.
type CollectionMeta struct {
	Name        string   `json:"name"`
	MessageType string   `json:"messageType"` // fully-qualified, e.g. "example.User"
	KeyKind     KeyKind  `json:"keyKind"`
	KeyField    string   `json:"keyField,omitempty"` // set when KeyKind == KeyField
	Indexes     []string `json:"indexes,omitempty"`  // field names with a secondary index (Layer 3)
}

// DB is an open KoraDB database: a storage file plus its schema registry.
type DB struct {
	store *storage.Store
	reg   *schema.Registry
}

// Open opens (or creates) a KoraDB database at path.
func Open(path string) (*DB, error) {
	st, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	reg, err := schema.NewRegistry(st)
	if err != nil {
		st.Close()
		return nil, err
	}
	return &DB{store: st, reg: reg}, nil
}

// Close releases the underlying database file.
func (db *DB) Close() error { return db.store.Close() }

// Backup writes a consistent snapshot of this embedded database to w. It does
// not close the database or block readers for the duration of the copy.
func (db *DB) Backup(w io.Writer) error { return db.store.Snapshot(w) }

// Verify checks the underlying storage structure without mutating the
// database. It does not validate every document against its schema.
func (db *DB) Verify() error { return db.store.Verify() }

// Registry exposes the schema registry so callers can register/evolve schemas.
func (db *DB) Registry() *schema.Registry { return db.reg }

// Store exposes the storage layer (used by the index layer, Layer 3).
func (db *DB) Store() *storage.Store { return db.store }

// RegisterSchema compiles and stores a .proto schema. See schema.Registry.
func (db *DB) RegisterSchema(ctx context.Context, name, protoSource string) (int, error) {
	return db.reg.Register(ctx, name, protoSource)
}

// CreateCollection defines a new collection bound to a message type. The
// message type must already be registered. opts may be nil for defaults
// (auto-generated keys, no indexes).
func (db *DB) CreateCollection(name, messageType string, opts *CollectionOptions) (*CollectionMeta, error) {
	if opts == nil {
		opts = &CollectionOptions{}
	}
	md, err := db.reg.MessageDescriptor(messageType)
	if err != nil {
		return nil, err
	}

	meta := &CollectionMeta{
		Name:        name,
		MessageType: messageType,
		KeyKind:     KeyAuto,
		Indexes:     opts.Indexes,
	}
	if opts.KeyField != "" {
		kfd := md.Fields().ByName(protoreflect.Name(opts.KeyField))
		if kfd == nil {
			return nil, fmt.Errorf("engine: key field %q not in message %q", opts.KeyField, messageType)
		}
		if kfd.IsList() || kfd.IsMap() || kfd.Kind() == protoreflect.MessageKind || kfd.Kind() == protoreflect.GroupKind {
			return nil, fmt.Errorf("engine: key field %q must be a scalar (got %s)", opts.KeyField, kfd.Kind())
		}
		if !primaryKeyKindSupported(kfd.Kind()) {
			return nil, fmt.Errorf("engine: key field %q has unsupported kind %s", opts.KeyField, kfd.Kind())
		}
		meta.KeyKind = KeyField
		meta.KeyField = opts.KeyField
	}

	// Validate that every requested index is on a field that exists and whose
	// type can be indexed.
	for _, field := range opts.Indexes {
		fd := md.Fields().ByName(protoreflect.Name(field))
		if fd == nil {
			return nil, fmt.Errorf("engine: index field %q not in message %q", field, messageType)
		}
		if !index.Indexable(fd) {
			return nil, fmt.Errorf("engine: field %q (kind %s) cannot be indexed", field, fd.Kind())
		}
	}

	err = db.store.Update(func(tx *storage.Txn) error {
		if ok, _ := tx.Has(collectionsBucket, []byte(name)); ok {
			return ErrExists
		}
		b, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return tx.Put(collectionsBucket, []byte(name), b)
	})
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// CollectionOptions configures a new collection.
type CollectionOptions struct {
	KeyField string   // if set, use this field's value as the primary key
	Indexes  []string // field names to maintain secondary indexes on
}

// getMeta loads a collection's metadata or returns ErrNotFound.
func (db *DB) getMeta(tx *storage.Txn, coll string) (*CollectionMeta, error) {
	b, err := tx.Get(collectionsBucket, []byte(coll))
	if err == storage.ErrNotFound {
		return nil, fmt.Errorf("engine: collection %q: %w", coll, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	var meta CollectionMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetCollection returns a collection's metadata.
func (db *DB) GetCollection(coll string) (*CollectionMeta, error) {
	var meta *CollectionMeta
	err := db.store.View(func(tx *storage.Txn) error {
		m, err := db.getMeta(tx, coll)
		meta = m
		return err
	})
	return meta, err
}

// ListCollections returns metadata for every collection.
func (db *DB) ListCollections() ([]*CollectionMeta, error) {
	var out []*CollectionMeta
	err := db.store.View(func(tx *storage.Txn) error {
		return tx.Scan(collectionsBucket, func(_, v []byte) error {
			var m CollectionMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return err
			}
			out = append(out, &m)
			return nil
		})
	})
	return out, err
}

// Insert parses jsonDoc against the collection's schema, stores it as protobuf
// bytes, and returns the document's id. Index entries (if any) are written in
// the same transaction so data and indexes can never diverge.
func (db *DB) Insert(coll string, jsonDoc []byte) (string, error) {
	var id string
	err := db.store.Update(func(tx *storage.Txn) error {
		meta, err := db.getMeta(tx, coll)
		if err != nil {
			return err
		}
		msg, err := db.parse(meta, jsonDoc)
		if err != nil {
			return err
		}
		key, idStr, err := db.deriveKey(tx, meta, msg)
		if err != nil {
			return err
		}
		// For field-keyed collections the key is caller-controlled, so guard
		// against silently clobbering an existing document (auto keys are unique
		// by construction). Insert is create-only; use Update to replace.
		if meta.KeyKind == KeyField {
			if exists, err := tx.Has(dataBucket(coll), key); err != nil {
				return err
			} else if exists {
				return fmt.Errorf("engine: document with key %q already exists in %q: %w", idStr, coll, ErrDuplicateKey)
			}
		}
		wire, err := proto.Marshal(msg)
		if err != nil {
			return fmt.Errorf("engine: marshal: %w", err)
		}
		if err := tx.Put(dataBucket(coll), key, wire); err != nil {
			return err
		}
		if err := db.indexInsert(tx, meta, key, msg); err != nil {
			return err
		}
		id = idStr
		return nil
	})
	return id, err
}

// Get returns the document with the given id, rendered as JSON.
func (db *DB) Get(coll, id string) ([]byte, error) {
	var out []byte
	err := db.store.View(func(tx *storage.Txn) error {
		meta, err := db.getMeta(tx, coll)
		if err != nil {
			return err
		}
		key, err := db.encodeKey(meta, id)
		if err != nil {
			return err
		}
		wire, err := tx.Get(dataBucket(coll), key)
		if err == storage.ErrNotFound {
			return fmt.Errorf("engine: document %q in %q: %w", id, coll, ErrNotFound)
		}
		if err != nil {
			return err
		}
		j, err := db.render(meta, wire)
		out = j
		return err
	})
	return out, err
}

// Update replaces the document at id with jsonDoc. The id is preserved; for
// field-keyed collections jsonDoc must carry the same key value.
func (db *DB) Update(coll, id string, jsonDoc []byte) error {
	return db.store.Update(func(tx *storage.Txn) error {
		meta, err := db.getMeta(tx, coll)
		if err != nil {
			return err
		}
		key, err := db.encodeKey(meta, id)
		if err != nil {
			return err
		}
		// Load the prior document so stale index entries can be removed.
		old, err := tx.Get(dataBucket(coll), key)
		if err == storage.ErrNotFound {
			return fmt.Errorf("engine: document %q in %q: %w", id, coll, ErrNotFound)
		}
		if err != nil {
			return err
		}
		oldMsg, err := db.decode(meta, old)
		if err != nil {
			return err
		}
		newMsg, err := db.parse(meta, jsonDoc)
		if err != nil {
			return err
		}
		if meta.KeyKind == KeyField {
			newKey, _, err := db.deriveKey(tx, meta, newMsg)
			if err != nil {
				return err
			}
			if !bytes.Equal(key, newKey) {
				return fmt.Errorf("engine: update of %q in %q changes immutable key field %q", id, coll, meta.KeyField)
			}
		}
		wire, err := proto.Marshal(newMsg)
		if err != nil {
			return err
		}
		if err := db.indexDelete(tx, meta, key, oldMsg); err != nil {
			return err
		}
		if err := tx.Put(dataBucket(coll), key, wire); err != nil {
			return err
		}
		return db.indexInsert(tx, meta, key, newMsg)
	})
}

// Delete removes the document at id. Deleting a missing document is not an error.
func (db *DB) Delete(coll, id string) error {
	return db.store.Update(func(tx *storage.Txn) error {
		meta, err := db.getMeta(tx, coll)
		if err != nil {
			return err
		}
		key, err := db.encodeKey(meta, id)
		if err != nil {
			return err
		}
		old, err := tx.Get(dataBucket(coll), key)
		if err == storage.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		oldMsg, err := db.decode(meta, old)
		if err != nil {
			return err
		}
		if err := db.indexDelete(tx, meta, key, oldMsg); err != nil {
			return err
		}
		return tx.Delete(dataBucket(coll), key)
	})
}

// Each calls fn for every document in the collection, decoded into a dynamic
// message, in primary-key order. Used by the query executor's full scans.
func (db *DB) Each(tx *storage.Txn, meta *CollectionMeta, fn func(key []byte, msg *dynamicpb.Message) error) error {
	md, err := db.reg.MessageDescriptor(meta.MessageType)
	if err != nil {
		return err
	}
	return tx.Scan(dataBucket(meta.Name), func(k, v []byte) error {
		msg := dynamicpb.NewMessage(md)
		if err := proto.Unmarshal(v, msg); err != nil {
			return fmt.Errorf("engine: decode document: %w", err)
		}
		// Copy the key; the slice from Scan is only valid during the callback.
		kc := make([]byte, len(k))
		copy(kc, k)
		return fn(kc, msg)
	})
}

// FetchByKey reads and decodes a single document by its raw storage key, within
// the caller's transaction. Returns ErrNotFound if absent. Used by the query
// executor to materialize index-seeded candidates.
func (db *DB) FetchByKey(tx *storage.Txn, meta *CollectionMeta, key []byte) (*dynamicpb.Message, error) {
	wire, err := tx.Get(dataBucket(meta.Name), key)
	if err == storage.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return db.decode(meta, wire)
}

// RenderMessage renders a dynamic message to JSON (proto field names).
func (db *DB) RenderMessage(msg *dynamicpb.Message) ([]byte, error) {
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
}

// --- helpers: parse / render / decode -------------------------------------

func (db *DB) parse(meta *CollectionMeta, jsonDoc []byte) (*dynamicpb.Message, error) {
	msg, err := db.reg.NewMessage(meta.MessageType)
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(jsonDoc, msg); err != nil {
		return nil, fmt.Errorf("engine: invalid document for %q: %w", meta.MessageType, err)
	}
	return msg, nil
}

func (db *DB) decode(meta *CollectionMeta, wire []byte) (*dynamicpb.Message, error) {
	msg, err := db.reg.NewMessage(meta.MessageType)
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(wire, msg); err != nil {
		return nil, fmt.Errorf("engine: decode document: %w", err)
	}
	return msg, nil
}

func (db *DB) render(meta *CollectionMeta, wire []byte) ([]byte, error) {
	msg, err := db.decode(meta, wire)
	if err != nil {
		return nil, err
	}
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
}

// --- helpers: keys ---------------------------------------------------------

// deriveKey computes the storage key and user-facing id for a new document.
func (db *DB) deriveKey(tx *storage.Txn, meta *CollectionMeta, msg *dynamicpb.Message) (key []byte, id string, err error) {
	if meta.KeyKind == KeyAuto {
		seq, err := tx.NextSequence(dataBucket(meta.Name))
		if err != nil {
			return nil, "", err
		}
		return encodeUint(seq), strconv.FormatUint(seq, 10), nil
	}
	// KeyField: read the field value from the document.
	fd := msg.Descriptor().Fields().ByName(protoreflect.Name(meta.KeyField))
	if fd == nil {
		return nil, "", fmt.Errorf("engine: key field %q missing from message", meta.KeyField)
	}
	idStr, err := idStringFromValue(fd, msg.Get(fd))
	if err != nil {
		return nil, "", err
	}
	k, err := db.encodeKey(meta, idStr)
	if err != nil {
		return nil, "", err
	}
	return k, idStr, nil
}

// encodeKey turns a user-facing id string into the storage key, matching the
// collection's key kind and (for field keys) the field's type.
func (db *DB) encodeKey(meta *CollectionMeta, id string) ([]byte, error) {
	if meta.KeyKind == KeyAuto {
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("engine: invalid auto id %q: %w", id, err)
		}
		return encodeUint(n), nil
	}
	md, err := db.reg.MessageDescriptor(meta.MessageType)
	if err != nil {
		return nil, err
	}
	fd := md.Fields().ByName(protoreflect.Name(meta.KeyField))
	if fd == nil {
		return nil, fmt.Errorf("engine: collection %q key field %q is missing from message %q", meta.Name, meta.KeyField, meta.MessageType)
	}
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind,
		protoreflect.Sint64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		n, err := strconv.ParseInt(id, 10, primaryKeyBitSize(fd.Kind()))
		if err != nil {
			return nil, fmt.Errorf("engine: invalid signed integer key %q: %w", id, err)
		}
		return encodeUint(uint64(n)), nil
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		n, err := strconv.ParseUint(id, 10, primaryKeyBitSize(fd.Kind()))
		if err != nil {
			return nil, fmt.Errorf("engine: invalid unsigned integer key %q: %w", id, err)
		}
		return encodeUint(n), nil
	case protoreflect.BoolKind:
		v, err := strconv.ParseBool(id)
		if err != nil {
			return nil, fmt.Errorf("engine: invalid bool key %q: %w", id, err)
		}
		return []byte(strconv.FormatBool(v)), nil
	case protoreflect.StringKind:
		return []byte(id), nil
	default:
		return nil, fmt.Errorf("engine: key field %q has unsupported kind %s", meta.KeyField, fd.Kind())
	}
}

// IDFromStorageKey renders a raw storage key as the canonical public document
// ID. Query results use this so their IDs can be passed directly to Get,
// Update, and Delete.
func (db *DB) IDFromStorageKey(meta *CollectionMeta, key []byte) (string, error) {
	if meta.KeyKind == KeyAuto {
		if len(key) != 8 {
			return "", fmt.Errorf("engine: invalid auto key length %d", len(key))
		}
		return strconv.FormatUint(binary.BigEndian.Uint64(key), 10), nil
	}
	md, err := db.reg.MessageDescriptor(meta.MessageType)
	if err != nil {
		return "", err
	}
	fd := md.Fields().ByName(protoreflect.Name(meta.KeyField))
	if fd == nil {
		return "", fmt.Errorf("engine: collection %q key field %q is missing from message %q", meta.Name, meta.KeyField, meta.MessageType)
	}
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind,
		protoreflect.Sint64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		if len(key) != 8 {
			return "", fmt.Errorf("engine: invalid signed integer key length %d", len(key))
		}
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(key)), 10), nil
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		if len(key) != 8 {
			return "", fmt.Errorf("engine: invalid unsigned integer key length %d", len(key))
		}
		return strconv.FormatUint(binary.BigEndian.Uint64(key), 10), nil
	case protoreflect.BoolKind, protoreflect.StringKind:
		return string(key), nil
	default:
		return "", fmt.Errorf("engine: key field %q has unsupported kind %s", meta.KeyField, fd.Kind())
	}
}

func primaryKeyKindSupported(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.StringKind, protoreflect.BoolKind,
		protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return true
	default:
		return false
	}
}

func primaryKeyBitSize(kind protoreflect.Kind) int {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return 32
	default:
		return 64
	}
}

// encodeUint encodes a uint64 big-endian so keys sort numerically.
func encodeUint(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}
