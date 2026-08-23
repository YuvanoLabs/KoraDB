// Package schema is Layer 1 of KoraDB: the schema registry.
//
// This layer is what makes KoraDB a *protobuf* database rather than a key/value
// store that happens to hold protobuf bytes. Protobuf wire bytes are not
// self-describing — the bytes 0x08 0x96 0x01 mean nothing without the .proto
// that says "field 1 is an int32 named age". So the registry compiles .proto
// source at runtime (pure Go, via bufbuild/protocompile — no protoc binary),
// persists the resulting descriptors alongside the data, and hands out
// MessageDescriptors and dynamic messages that every higher layer uses to
// encode, decode, validate, and read fields out of stored documents.
package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"KoraDB/internal/storage"
)

// schemaBucket holds one record per registered schema. The leading "__"
// marks it as reserved; user collections never use this name.
var schemaBucket = []byte("__schemas__")

// record is what we persist for each registered schema: enough to rebuild the
// descriptors on restart and to show the user what they registered.
type record struct {
	Name    string `json:"name"`    // logical schema name (the .proto filename)
	Source  string `json:"source"`  // original .proto text, kept for inspection
	Version int    `json:"version"` // bumped each time this name is re-registered
	FDSet   []byte `json:"fdset"`   // serialized FileDescriptorSet (with deps)
}

// Registry compiles, stores, and serves protobuf schemas. It keeps an in-memory
// view (files) of every loaded descriptor for fast lookup, rebuilt whenever a
// schema is registered or loaded from disk.
type Registry struct {
	store *storage.Store

	// regMu serializes Register calls (version numbering + persistence) without
	// holding mu, so we never open a bbolt transaction while holding mu. Insert
	// holds a bbolt write transaction and then takes mu.RLock (via
	// MessageDescriptor); if Register held mu while opening a write txn the two
	// lock orders would invert and deadlock.
	regMu sync.Mutex

	mu    sync.RWMutex
	files *protoregistry.Files
	// fileProtos is the accumulated set of every FileDescriptorProto across all
	// registered schemas, keyed by path. It is the source of truth from which
	// files is rebuilt (protodesc needs the full dependency closure together).
	fileProtos map[string]*descriptorpb.FileDescriptorProto
}

// NewRegistry opens a registry backed by store and loads any schemas already
// persisted from previous runs.
func NewRegistry(store *storage.Store) (*Registry, error) {
	r := &Registry{
		store:      store,
		files:      new(protoregistry.Files),
		fileProtos: make(map[string]*descriptorpb.FileDescriptorProto),
	}
	if err := r.loadFromStore(); err != nil {
		return nil, err
	}
	return r, nil
}

// loadFromStore reads every persisted schema record and rebuilds the in-memory
// descriptor view. Called once at open.
func (r *Registry) loadFromStore() error {
	err := r.store.View(func(tx *storage.Txn) error {
		return tx.Scan(schemaBucket, func(_, v []byte) error {
			var rec record
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("schema: corrupt schema record: %w", err)
			}
			var fdset descriptorpb.FileDescriptorSet
			if err := protoUnmarshal(rec.FDSet, &fdset); err != nil {
				return fmt.Errorf("schema %q: decode descriptors: %w", rec.Name, err)
			}
			for _, fp := range fdset.File {
				r.fileProtos[fp.GetName()] = fp
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	// Rebuild the resolvable descriptor view from everything just loaded.
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.fileProtos) == 0 {
		return nil
	}
	return r.rebuildLocked()
}

// Register compiles protoSource (a complete .proto file), persists its
// descriptors, and makes its message types available for use. name is the
// logical .proto filename (e.g. "user.proto"). Re-registering the same name is
// how schema evolution works: the new version is stored and used for future
// encode/decode, while existing documents on disk remain readable because the
// protobuf wire format is forward/backward compatible by field number.
//
// It returns the new version number for the schema.
func (r *Registry) Register(ctx context.Context, name, protoSource string) (int, error) {
	// Compile with standard imports available so schemas may import the
	// well-known types (timestamp.proto, etc.) without shipping those files.
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(map[string]string{
				name: protoSource,
			}),
		}),
	}
	compiled, err := compiler.Compile(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("schema %q: compile failed: %w", name, err)
	}
	if len(compiled) == 0 {
		return 0, fmt.Errorf("schema %q: produced no files", name)
	}

	// Collect this file and its full transitive import closure as protos.
	collected := make(map[string]*descriptorpb.FileDescriptorProto)
	collectFiles(compiled[0], collected)

	fdset := &descriptorpb.FileDescriptorSet{File: topoSort(collected)}
	fdsetBytes, err := protoMarshal(fdset)
	if err != nil {
		return 0, fmt.Errorf("schema %q: encode descriptors: %w", name, err)
	}

	// Persist outside r.mu (see regMu doc): never hold r.mu across a bbolt txn.
	r.regMu.Lock()
	defer r.regMu.Unlock()

	// Determine the next version and persist the record.
	version := 1
	if err := r.store.View(func(tx *storage.Txn) error {
		v, err := tx.Get(schemaBucket, []byte(name))
		if err == storage.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		var prev record
		if err := json.Unmarshal(v, &prev); err == nil {
			version = prev.Version + 1
		}
		return nil
	}); err != nil {
		return 0, err
	}

	rec := record{Name: name, Source: protoSource, Version: version, FDSet: fdsetBytes}
	recBytes, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	if err := r.store.Update(func(tx *storage.Txn) error {
		return tx.Put(schemaBucket, []byte(name), recBytes)
	}); err != nil {
		return 0, err
	}

	// Merge into the in-memory descriptor view and rebuild. Hold r.mu only for
	// this in-memory work — no bbolt transaction is opened here.
	r.mu.Lock()
	for _, fp := range collected {
		r.fileProtos[fp.GetName()] = fp
	}
	err = r.rebuildLocked()
	r.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return version, nil
}

// rebuildLocked reconstructs the protoregistry.Files from the accumulated set of
// FileDescriptorProtos. Caller must hold r.mu.
func (r *Registry) rebuildLocked() error {
	set := &descriptorpb.FileDescriptorSet{File: topoSort(r.fileProtos)}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return fmt.Errorf("schema: rebuild descriptor set: %w", err)
	}
	r.files = files
	return nil
}

// MessageDescriptor returns the descriptor for a fully-qualified message name
// (e.g. "example.User"). Higher layers bind a collection to such a name.
func (r *Registry) MessageDescriptor(fqName string) (protoreflect.MessageDescriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.files == nil {
		return nil, fmt.Errorf("schema: %q not found (no schemas registered)", fqName)
	}
	d, err := r.files.FindDescriptorByName(protoreflect.FullName(fqName))
	if err != nil {
		return nil, fmt.Errorf("schema: message %q not found: %w", fqName, err)
	}
	md, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("schema: %q is not a message type", fqName)
	}
	return md, nil
}

// NewMessage returns a fresh dynamic message for the named type, ready to be
// populated and marshaled, or to receive an Unmarshal of stored bytes.
func (r *Registry) NewMessage(fqName string) (*dynamicpb.Message, error) {
	md, err := r.MessageDescriptor(fqName)
	if err != nil {
		return nil, err
	}
	return dynamicpb.NewMessage(md), nil
}

// ListSchemas returns the persisted record for every registered schema name.
func (r *Registry) ListSchemas() ([]SchemaInfo, error) {
	var out []SchemaInfo
	err := r.store.View(func(tx *storage.Txn) error {
		return tx.Scan(schemaBucket, func(_, v []byte) error {
			var rec record
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			out = append(out, SchemaInfo{Name: rec.Name, Version: rec.Version, Source: rec.Source})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SchemaInfo is a user-facing summary of a registered schema.
type SchemaInfo struct {
	Name    string
	Version int
	Source  string
}

// collectFiles walks fd and all its imports, recording each as a
// FileDescriptorProto keyed by path. Idempotent across the shared map.
func collectFiles(fd protoreflect.FileDescriptor, out map[string]*descriptorpb.FileDescriptorProto) {
	if _, seen := out[fd.Path()]; seen {
		return
	}
	out[fd.Path()] = protodesc.ToFileDescriptorProto(fd)
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		collectFiles(imports.Get(i).FileDescriptor, out)
	}
}

// topoSort orders files so that every file appears after all of its imports,
// which is what protodesc.NewFiles needs to resolve dependencies.
func topoSort(m map[string]*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	var ordered []*descriptorpb.FileDescriptorProto
	visited := make(map[string]bool)
	var visit func(path string)
	visit = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true
		fp, ok := m[path]
		if !ok {
			return // dependency outside our set (resolved elsewhere)
		}
		for _, dep := range fp.GetDependency() {
			visit(dep)
		}
		ordered = append(ordered, fp)
	}
	// Visit in a stable order for deterministic output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		visit(k)
	}
	return ordered
}
