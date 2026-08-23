package schema

import "google.golang.org/protobuf/proto"

// Thin aliases so the registry's intent reads clearly and the proto dependency
// is isolated to one place.

func protoMarshal(m proto.Message) ([]byte, error) { return proto.Marshal(m) }

func protoUnmarshal(b []byte, m proto.Message) error { return proto.Unmarshal(b, m) }
