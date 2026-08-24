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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

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

// schemaHistoryBucket stores immutable records for every schema version
// accepted after version history was introduced. schemaBucket remains the
// active-version catalog used when opening a database.
var schemaHistoryBucket = []byte("__schema_history__")

// record is what we persist for each registered schema: enough to rebuild the
// descriptors on restart and to show the user what they registered.
type record struct {
	Name        string `json:"name"`        // logical schema name (the .proto filename)
	Source      string `json:"source"`      // original .proto text, kept for inspection
	Version     int    `json:"version"`     // bumped each time this name is re-registered
	FDSet       []byte `json:"fdset"`       // serialized FileDescriptorSet (with deps)
	Digest      string `json:"digest"`      // SHA-256 of FDSet for reproducibility
	CreatedUnix int64  `json:"createdUnix"` // acceptance time; zero for legacy records
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
	// Registration is serialized so user-schema imports, compatibility checks,
	// version allocation, persistence, and activation all see one coherent
	// catalog state.
	r.regMu.Lock()
	defer r.regMu.Unlock()

	sources, err := r.activeSources()
	if err != nil {
		return 0, err
	}
	sources[name] = protoSource

	// Compile with standard imports available so schemas may import the
	// well-known types (timestamp.proto, etc.) without shipping those files.
	// The active user-schema catalog is included too, enabling imports by their
	// registered logical name while preserving the exact source in the catalog.
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(sources),
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

	r.mu.RLock()
	candidateProtos := cloneFileProtos(r.fileProtos)
	activeFiles := r.files
	r.mu.RUnlock()
	_, replacesActiveSchema := candidateProtos[name]
	for path, fp := range collected {
		candidateProtos[path] = fp
	}
	candidateFiles, err := buildFiles(candidateProtos)
	if err != nil {
		return 0, err
	}
	if replacesActiveSchema {
		if err := ensureFileCompatible(activeFiles, candidateFiles, name); err != nil {
			return 0, err
		}
	}

	// Determine the next version and persist only after the complete candidate
	// descriptor set has been validated.
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

	rec := record{
		Name:        name,
		Source:      protoSource,
		Version:     version,
		FDSet:       fdsetBytes,
		Digest:      descriptorDigest(fdsetBytes),
		CreatedUnix: time.Now().UTC().Unix(),
	}
	recBytes, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	if err := r.store.Update(func(tx *storage.Txn) error {
		if err := tx.Put(schemaBucket, []byte(name), recBytes); err != nil {
			return err
		}
		return tx.Put(schemaHistoryBucket, schemaHistoryKey(name, version), recBytes)
	}); err != nil {
		return 0, err
	}

	// Activation cannot fail: candidateFiles was built before the durable commit.
	// Hold r.mu only for the pointer swap; no bbolt transaction is opened here.
	r.mu.Lock()
	r.fileProtos = candidateProtos
	r.files = candidateFiles
	r.mu.Unlock()
	return version, nil
}

// rebuildLocked reconstructs the protoregistry.Files from the accumulated set of
// FileDescriptorProtos. Caller must hold r.mu.
func (r *Registry) rebuildLocked() error {
	files, err := buildFiles(r.fileProtos)
	if err != nil {
		return err
	}
	r.files = files
	return nil
}

func buildFiles(fileProtos map[string]*descriptorpb.FileDescriptorProto) (*protoregistry.Files, error) {
	set := &descriptorpb.FileDescriptorSet{File: topoSort(fileProtos)}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, fmt.Errorf("schema: rebuild descriptor set: %w", err)
	}
	return files, nil
}

func cloneFileProtos(in map[string]*descriptorpb.FileDescriptorProto) map[string]*descriptorpb.FileDescriptorProto {
	out := make(map[string]*descriptorpb.FileDescriptorProto, len(in))
	for path, fp := range in {
		out[path] = fp
	}
	return out
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
			out = append(out, schemaInfoFromRecord(rec))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SchemaHistory returns all immutable versions retained for name, oldest
// first. Legacy schemas created before history support may expose only their
// versions accepted after this feature was introduced.
func (r *Registry) SchemaHistory(name string) ([]SchemaInfo, error) {
	var out []SchemaInfo
	err := r.store.View(func(tx *storage.Txn) error {
		return tx.PrefixScan(schemaHistoryBucket, schemaHistoryPrefix(name), func(_, value []byte) error {
			var rec record
			if err := json.Unmarshal(value, &rec); err != nil {
				return fmt.Errorf("schema: corrupt schema history record: %w", err)
			}
			out = append(out, schemaInfoFromRecord(rec))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// SchemaInfo is a user-facing summary of a registered schema.
type SchemaInfo struct {
	Name        string
	Version     int
	Source      string
	Digest      string
	CreatedUnix int64
}

func schemaInfoFromRecord(rec record) SchemaInfo {
	return SchemaInfo{
		Name:        rec.Name,
		Version:     rec.Version,
		Source:      rec.Source,
		Digest:      rec.Digest,
		CreatedUnix: rec.CreatedUnix,
	}
}

func (r *Registry) activeSources() (map[string]string, error) {
	sources := make(map[string]string)
	err := r.store.View(func(tx *storage.Txn) error {
		return tx.Scan(schemaBucket, func(_, value []byte) error {
			var rec record
			if err := json.Unmarshal(value, &rec); err != nil {
				return fmt.Errorf("schema: corrupt schema record: %w", err)
			}
			sources[rec.Name] = rec.Source
			return nil
		})
	})
	return sources, err
}

func schemaHistoryPrefix(name string) []byte { return []byte(name + "\x00") }

func schemaHistoryKey(name string, version int) []byte {
	return []byte(fmt.Sprintf("%s\x00%020d", name, version))
}

func descriptorDigest(fdset []byte) string {
	sum := sha256.Sum256(fdset)
	return hex.EncodeToString(sum[:])
}

// ensureFileCompatible applies KoraDB's initial strict in-place evolution
// policy: existing messages, fields, field names, wire kinds, cardinality,
// presence, oneof membership, and message/enum references must remain stable.
// New fields and new types are allowed. Destructive changes require a new
// collection plus an explicit migration until a richer migration workflow is
// introduced.
func ensureFileCompatible(current, candidate *protoregistry.Files, path string) error {
	if current == nil {
		return nil
	}
	oldFile, err := current.FindFileByPath(path)
	if err != nil {
		return fmt.Errorf("schema %q: resolve active descriptor: %w", path, err)
	}
	newFile, err := candidate.FindFileByPath(path)
	if err != nil {
		return fmt.Errorf("schema %q: resolve candidate descriptor: %w", path, err)
	}
	if err := ensureMessagesCompatible(oldFile.Messages(), newFile.Messages()); err != nil {
		return fmt.Errorf("schema %q: incompatible evolution: %w", path, err)
	}
	return ensureEnumsCompatible(oldFile.Enums(), newFile.Enums())
}

func ensureMessagesCompatible(oldMessages, newMessages protoreflect.MessageDescriptors) error {
	for i := 0; i < oldMessages.Len(); i++ {
		oldMessage := oldMessages.Get(i)
		newMessage := newMessages.ByName(oldMessage.Name())
		if newMessage == nil {
			return fmt.Errorf("message %q was removed or renamed", oldMessage.FullName())
		}
		if err := ensureMessageCompatible(oldMessage, newMessage); err != nil {
			return err
		}
	}
	return nil
}

func ensureMessageCompatible(oldMessage, newMessage protoreflect.MessageDescriptor) error {
	oldFields := oldMessage.Fields()
	for i := 0; i < oldFields.Len(); i++ {
		oldField := oldFields.Get(i)
		newField := newMessage.Fields().ByNumber(oldField.Number())
		if newField == nil {
			return fmt.Errorf("field %q (number %d) was removed", oldField.FullName(), oldField.Number())
		}
		if oldField.Name() != newField.Name() ||
			oldField.Kind() != newField.Kind() ||
			oldField.Cardinality() != newField.Cardinality() ||
			oldField.HasPresence() != newField.HasPresence() ||
			oldField.IsMap() != newField.IsMap() ||
			oneofName(oldField) != oneofName(newField) {
			return fmt.Errorf("field %q (number %d) changed name, kind, cardinality, presence, map, or oneof contract", oldField.FullName(), oldField.Number())
		}
		if oldField.Kind() == protoreflect.MessageKind || oldField.Kind() == protoreflect.GroupKind {
			if oldField.Message().FullName() != newField.Message().FullName() {
				return fmt.Errorf("field %q changed referenced message type", oldField.FullName())
			}
		}
		if oldField.Kind() == protoreflect.EnumKind && oldField.Enum().FullName() != newField.Enum().FullName() {
			return fmt.Errorf("field %q changed referenced enum type", oldField.FullName())
		}
	}
	if err := ensureMessagesCompatible(oldMessage.Messages(), newMessage.Messages()); err != nil {
		return err
	}
	return ensureEnumsCompatible(oldMessage.Enums(), newMessage.Enums())
}

func ensureEnumsCompatible(oldEnums, newEnums protoreflect.EnumDescriptors) error {
	for i := 0; i < oldEnums.Len(); i++ {
		oldEnum := oldEnums.Get(i)
		newEnum := newEnums.ByName(oldEnum.Name())
		if newEnum == nil {
			return fmt.Errorf("enum %q was removed or renamed", oldEnum.FullName())
		}
		oldValues := oldEnum.Values()
		for j := 0; j < oldValues.Len(); j++ {
			oldValue := oldValues.Get(j)
			newValue := newEnum.Values().ByNumber(oldValue.Number())
			if newValue == nil || newValue.Name() != oldValue.Name() {
				return fmt.Errorf("enum value %q (number %d) changed or was removed", oldValue.FullName(), oldValue.Number())
			}
		}
	}
	return nil
}

func oneofName(field protoreflect.FieldDescriptor) protoreflect.Name {
	if oneof := field.ContainingOneof(); oneof != nil {
		return oneof.Name()
	}
	return ""
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
