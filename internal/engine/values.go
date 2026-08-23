package engine

import (
	"fmt"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// idStringFromValue renders a scalar field value as the canonical user-facing
// id string. It is the inverse of encodeKey's parsing, so a value extracted on
// insert and re-encoded on lookup round-trips exactly. We do NOT use
// protoreflect.Value.String() here: its output for non-string scalars is
// explicitly undefined ("for debugging only").
func idStringFromValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) (string, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return v.String(), nil
	case protoreflect.BoolKind:
		return strconv.FormatBool(v.Bool()), nil
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind,
		protoreflect.Sint64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return strconv.FormatInt(v.Int(), 10), nil
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return strconv.FormatUint(v.Uint(), 10), nil
	default:
		return "", fmt.Errorf("engine: field of kind %s cannot be a primary key", fd.Kind())
	}
}

// ScalarValueFromString is the exported form used by the query layer to type a
// query literal against the field it is compared with.
func ScalarValueFromString(fd protoreflect.FieldDescriptor, s string) (protoreflect.Value, error) {
	return scalarValueFromString(fd, s)
}

// scalarValueFromString converts a user-supplied string into a protoreflect
// Value typed to match fd. Used by index lookups and query predicates so that
// a literal in a query ("42", "true", "alice") is compared in the field's own
// type, not as text.
func scalarValueFromString(fd protoreflect.FieldDescriptor, s string) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(s), nil

	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not a bool: %w", s, err)
		}
		return protoreflect.ValueOfBool(b), nil

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not an int32: %w", s, err)
		}
		return protoreflect.ValueOfInt32(int32(n)), nil

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not an int64: %w", s, err)
		}
		return protoreflect.ValueOfInt64(n), nil

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not a uint32: %w", s, err)
		}
		return protoreflect.ValueOfUint32(uint32(n)), nil

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not a uint64: %w", s, err)
		}
		return protoreflect.ValueOfUint64(n), nil

	case protoreflect.FloatKind:
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not a float: %w", s, err)
		}
		return protoreflect.ValueOfFloat32(float32(f)), nil

	case protoreflect.DoubleKind:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("engine: %q is not a double: %w", s, err)
		}
		return protoreflect.ValueOfFloat64(f), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("engine: cannot compare field of kind %s", fd.Kind())
	}
}

// CompareValues orders two values of the same field kind, returning -1, 0, or
// +1. Used by query range predicates (gt/lt). It assumes both values come from
// the same field descriptor.
func CompareValues(fd protoreflect.FieldDescriptor, a, b protoreflect.Value) int {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return strings_Compare(a.String(), b.String())
	case protoreflect.BoolKind:
		ai, bi := boolToInt(a.Bool()), boolToInt(b.Bool())
		return cmpInt64(int64(ai), int64(bi))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return cmpInt64(a.Int(), b.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return cmpUint64(a.Uint(), b.Uint())
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return cmpFloat64(a.Float(), b.Float())
	default:
		return 0
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpUint64(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpFloat64(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// strings_Compare avoids importing strings just for one call site elsewhere.
func strings_Compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
