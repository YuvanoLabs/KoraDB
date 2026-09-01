package engine

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/YuvanoLabs/KoraDB/internal/index"
	"github.com/YuvanoLabs/KoraDB/internal/storage"
)

// indexInsert writes index entries for every indexed field of msg, in the
// caller's transaction. A no-op when the collection has no indexes.
func (db *DB) indexInsert(tx *storage.Txn, meta *CollectionMeta, primaryKey []byte, msg *dynamicpb.Message) error {
	return db.eachIndexedValue(meta, msg, func(field string, enc []byte) error {
		return tx.Put(index.Bucket(meta.Name, field), index.EntryKey(enc, primaryKey), nil)
	})
}

// indexDelete removes the index entries that msg previously contributed.
func (db *DB) indexDelete(tx *storage.Txn, meta *CollectionMeta, primaryKey []byte, msg *dynamicpb.Message) error {
	return db.eachIndexedValue(meta, msg, func(field string, enc []byte) error {
		return tx.Delete(index.Bucket(meta.Name, field), index.EntryKey(enc, primaryKey))
	})
}

// eachIndexedValue calls fn with the encoded value of each indexed field.
func (db *DB) eachIndexedValue(meta *CollectionMeta, msg *dynamicpb.Message, fn func(field string, enc []byte) error) error {
	if len(meta.Indexes) == 0 {
		return nil
	}
	fields := msg.Descriptor().Fields()
	for _, field := range meta.Indexes {
		fd := fields.ByName(protoreflect.Name(field))
		if fd == nil {
			return fmt.Errorf("engine: indexed field %q not in message", field)
		}
		enc, err := index.EncodeValue(fd, msg.Get(fd))
		if err != nil {
			return err
		}
		if err := fn(field, enc); err != nil {
			return err
		}
	}
	return nil
}

// LookupByIndex returns the primary keys of documents whose indexed field
// equals value (value given as its user-facing string form). Used by the query
// executor for equality predicates on indexed fields.
func (db *DB) LookupByIndex(tx *storage.Txn, meta *CollectionMeta, field, value string) ([][]byte, error) {
	md, err := db.reg.MessageDescriptor(meta.MessageType)
	if err != nil {
		return nil, err
	}
	fd := md.Fields().ByName(protoreflect.Name(field))
	if fd == nil {
		return nil, fmt.Errorf("engine: field %q not in message", field)
	}
	pv, err := scalarValueFromString(fd, value)
	if err != nil {
		return nil, err
	}
	enc, err := index.EncodeValue(fd, pv)
	if err != nil {
		return nil, err
	}
	prefix := index.LookupPrefix(enc)
	var keys [][]byte
	err = tx.PrefixScan(index.Bucket(meta.Name, field), prefix, func(k, _ []byte) error {
		pk := index.PrimaryKeyFromEntry(k, len(enc))
		cp := make([]byte, len(pk))
		copy(cp, pk)
		keys = append(keys, cp)
		return nil
	})
	return keys, err
}

// HasIndex reports whether the collection has an index on field.
func (m *CollectionMeta) HasIndex(field string) bool {
	for _, f := range m.Indexes {
		if f == field {
			return true
		}
	}
	return false
}
