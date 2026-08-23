// Package index is Layer 3 of KoraDB: persistent secondary indexes.
//
// An index lets the query layer find documents by a field value without
// scanning the whole collection. Each indexed field gets its own bucket whose
// keys are "<encoded field value> 0x00 <primary key>" and whose values are
// empty. Looking up a value is then a prefix scan over that bucket.
//
// Index entries are written and deleted *inside the same storage transaction*
// as the document itself (see the engine), so the data and its indexes can
// never drift apart, even across a crash.
//
// Field-value encoding is order-preserving for fixed-width numeric types, so
// range scans are possible in principle; this version uses indexes for equality
// lookups and leaves ranges to the executor's fallback scan.
package index

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// sep separates the encoded value from the primary key within an index key.
// Embedded NUL bytes in string keys are the one ambiguous case; documented and
// out of scope for this version.
const sep = 0x00

// Bucket returns the bucket name for a collection/field index.
func Bucket(coll, field string) []byte {
	return []byte("idx:" + coll + ":" + field)
}

// EntryKey builds the index key for (field value, primary key).
func EntryKey(encodedValue, primaryKey []byte) []byte {
	k := make([]byte, 0, len(encodedValue)+1+len(primaryKey))
	k = append(k, encodedValue...)
	k = append(k, sep)
	k = append(k, primaryKey...)
	return k
}

// LookupPrefix is the key prefix that selects every entry with the given value.
func LookupPrefix(encodedValue []byte) []byte {
	p := make([]byte, 0, len(encodedValue)+1)
	p = append(p, encodedValue...)
	p = append(p, sep)
	return p
}

// PrimaryKeyFromEntry recovers the primary key from a full index key, given the
// length of the encoded value portion.
func PrimaryKeyFromEntry(entryKey []byte, encodedValueLen int) []byte {
	start := encodedValueLen + 1 // skip value + separator
	if start > len(entryKey) {
		return nil
	}
	return entryKey[start:]
}

// EncodeValue produces an order-preserving byte encoding of a scalar field
// value, suitable for index keys. Supported kinds: integers, unsigned
// integers, booleans, and strings. Other kinds return an error so callers can
// reject indexes on unsupported fields up front.
func EncodeValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) ([]byte, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return []byte(v.String()), nil

	case protoreflect.BoolKind:
		if v.Bool() {
			return []byte{1}, nil
		}
		return []byte{0}, nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, v.Uint())
		return b, nil

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		// Flip the sign bit so negative values sort before positive ones.
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(v.Int())^(1<<63))
		return b, nil

	default:
		return nil, fmt.Errorf("index: field %q has unindexable kind %s", fd.Name(), fd.Kind())
	}
}

// Indexable reports whether a field can be indexed. Repeated fields, maps, and
// message/group fields are rejected: their value is a List/Map/Message, not a
// scalar, so the order-preserving scalar encoding does not apply to them.
func Indexable(fd protoreflect.FieldDescriptor) bool {
	if fd.IsList() || fd.IsMap() || fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return false
	}
	_, err := EncodeValue(fd, defaultValue(fd))
	return err == nil
}

func defaultValue(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString("")
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(false)
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(0)
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(0)
	default:
		return protoreflect.Value{}
	}
}
