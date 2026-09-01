// Package koradb is the public embedded Go API for KoraDB.
//
// It opens a local KoraDB file directly in the caller's process. It never
// starts a server or listens on a network port. Service-mode clients will be a
// separate provider once the remote API and SDK compatibility contract are
// stabilized.
package koradb

import (
	"context"
	"fmt"
	"io"

	"github.com/YuvanoLabs/KoraDB/internal/buildinfo"
	"github.com/YuvanoLabs/KoraDB/internal/engine"
	"github.com/YuvanoLabs/KoraDB/internal/query"
)

var (
	// ErrNotFound is returned when a collection or document does not exist.
	ErrNotFound = engine.ErrNotFound
	// ErrCollectionExists is returned when a collection name is already in use.
	ErrCollectionExists = engine.ErrExists
	// ErrDuplicateKey is returned when a field-keyed insert would overwrite an
	// existing document.
	ErrDuplicateKey = engine.ErrDuplicateKey
)

// Version is the build identity of the embedded KoraDB core.
func Version() string { return buildinfo.String() }

// Options configures an embedded database. It is intentionally empty in the
// first pre-release API so future explicit durability, locking, and telemetry
// options can be added without changing Open's shape.
type Options struct{}

// Database is an open embedded KoraDB database file.
type Database struct {
	db *engine.DB
}

// Open opens or creates a KoraDB database at path in embedded mode.
func Open(path string, _ Options) (*Database, error) {
	db, err := engine.Open(path)
	if err != nil {
		return nil, err
	}
	return &Database{db: db}, nil
}

// Close flushes and releases the database file lock.
func (db *Database) Close() error {
	if db == nil || db.db == nil {
		return nil
	}
	return db.db.Close()
}

// Backup writes a consistent embedded-database snapshot to w. The caller owns
// durable storage, encryption, retention, and restore workflow.
func (db *Database) Backup(ctx context.Context, w io.Writer) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	return db.db.Backup(w)
}

// Verify checks the current database file's storage structure without mutating
// it. It does not validate all documents against protobuf schemas.
func (db *Database) Verify(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	return db.db.Verify()
}

// RegisterSchema compiles and registers a protobuf source file. name is the
// logical .proto path used by the schema registry.
func (db *Database) RegisterSchema(ctx context.Context, name, protoSource string) (int, error) {
	if err := contextError(ctx); err != nil {
		return 0, err
	}
	return db.db.RegisterSchema(ctx, name, protoSource)
}

// SchemaInfo describes the active version of a registered schema.
type SchemaInfo struct {
	Name    string
	Version int
	Source  string
}

// ListSchemas returns active schemas in deterministic name order.
func (db *Database) ListSchemas(ctx context.Context) ([]SchemaInfo, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	schemas, err := db.db.Registry().ListSchemas()
	if err != nil {
		return nil, err
	}
	out := make([]SchemaInfo, 0, len(schemas))
	for _, schema := range schemas {
		out = append(out, SchemaInfo{Name: schema.Name, Version: schema.Version, Source: schema.Source})
	}
	return out, nil
}

// CollectionSpec defines a collection and its primary-key/index policy.
type CollectionSpec struct {
	Name        string
	MessageType string
	KeyField    string
	Indexes     []string
}

// CollectionInfo describes a collection's persisted configuration.
type CollectionInfo struct {
	Name        string
	MessageType string
	KeyKind     string
	KeyField    string
	Indexes     []string
}

// CreateCollection persists a collection definition and returns its embedded
// handle. Each collection is bound to one registered protobuf message type.
func (db *Database) CreateCollection(ctx context.Context, spec CollectionSpec) (*Collection, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if spec.Name == "" || spec.MessageType == "" {
		return nil, fmt.Errorf("koradb: collection name and message type are required")
	}
	if _, err := db.db.CreateCollection(spec.Name, spec.MessageType, &engine.CollectionOptions{
		KeyField: spec.KeyField,
		Indexes:  append([]string(nil), spec.Indexes...),
	}); err != nil {
		return nil, err
	}
	return &Collection{db: db, name: spec.Name}, nil
}

// Collection opens an existing collection by name.
func (db *Database) Collection(ctx context.Context, name string) (*Collection, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if _, err := db.db.GetCollection(name); err != nil {
		return nil, err
	}
	return &Collection{db: db, name: name}, nil
}

// ListCollections returns collection metadata in deterministic name order.
func (db *Database) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	collections, err := db.db.ListCollections()
	if err != nil {
		return nil, err
	}
	out := make([]CollectionInfo, 0, len(collections))
	for _, collection := range collections {
		out = append(out, CollectionInfo{
			Name:        collection.Name,
			MessageType: collection.MessageType,
			KeyKind:     string(collection.KeyKind),
			KeyField:    collection.KeyField,
			Indexes:     append([]string(nil), collection.Indexes...),
		})
	}
	return out, nil
}

// Collection is a handle to a single collection in an embedded database.
type Collection struct {
	db   *Database
	name string
}

// Name returns the collection's persisted name.
func (c *Collection) Name() string { return c.name }

// InsertJSON validates a ProtoJSON document and returns its canonical ID.
func (c *Collection) InsertJSON(ctx context.Context, document []byte) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	return c.db.db.Insert(c.name, document)
}

// GetJSON returns a ProtoJSON document by its canonical ID.
func (c *Collection) GetJSON(ctx context.Context, id string) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return c.db.db.Get(c.name, id)
}

// UpdateJSON replaces a document while preserving the collection's primary-key
// invariant.
func (c *Collection) UpdateJSON(ctx context.Context, id string, document []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	return c.db.db.Update(c.name, id, document)
}

// Delete removes a document. Deleting a missing ID is idempotent.
func (c *Collection) Delete(ctx context.Context, id string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	return c.db.db.Delete(c.name, id)
}

// ComparisonOperator selects a scalar comparison for QueryJSON.
type ComparisonOperator string

const (
	Equal              ComparisonOperator = "eq"
	NotEqual           ComparisonOperator = "ne"
	GreaterThan        ComparisonOperator = "gt"
	GreaterThanOrEqual ComparisonOperator = "gte"
	LessThan           ComparisonOperator = "lt"
	LessThanOrEqual    ComparisonOperator = "lte"
)

// Document is a document returned by QueryJSON. ID is always in the same
// canonical format accepted by GetJSON, UpdateJSON, and Delete.
type Document struct {
	ID   string
	JSON []byte
}

// QueryPage is one bounded page returned by QueryPageJSON. NextPageToken is
// opaque; provide it unchanged to the next call with the same query to resume
// after the final document in Documents. An empty token marks the last page.
type QueryPage struct {
	Documents     []Document
	NextPageToken string
}

// QueryJSON runs one scalar predicate. More complex filter trees, pagination,
// projections, sorting, and typed query values are intentionally withheld from
// this pre-release API until their compatibility and resource-limit contracts
// are complete.
func (c *Collection) QueryJSON(ctx context.Context, field string, op ComparisonOperator, value string) ([]Document, error) {
	page, err := c.QueryPageJSON(ctx, field, op, value, query.DefaultResultLimit, "")
	if err != nil {
		return nil, err
	}
	if page.NextPageToken != "" {
		return nil, &query.ResultLimitError{Limit: query.DefaultResultLimit}
	}
	return page.Documents, nil
}

// QueryPageJSON runs one scalar predicate and returns at most pageSize
// documents. pageToken must be empty for the first page or the opaque token
// from a previous call with the same collection and predicate.
func (c *Collection) QueryPageJSON(ctx context.Context, field string, op ComparisonOperator, value string, pageSize int, pageToken string) (QueryPage, error) {
	if err := contextError(ctx); err != nil {
		return QueryPage{}, err
	}
	engineOp, err := toEngineOp(op)
	if err != nil {
		return QueryPage{}, err
	}
	page, err := query.ExecutePage(c.db.db, c.name, query.Cmp{Field: field, Op: engineOp, Value: value}, pageSize, pageToken)
	if err != nil {
		return QueryPage{}, err
	}
	out := QueryPage{Documents: make([]Document, 0, len(page.Results)), NextPageToken: page.NextPageToken}
	for _, result := range page.Results {
		out.Documents = append(out.Documents, Document{ID: result.ID, JSON: append([]byte(nil), result.JSON...)})
	}
	return out, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func toEngineOp(op ComparisonOperator) (query.Op, error) {
	switch op {
	case Equal:
		return query.Eq, nil
	case NotEqual:
		return query.Ne, nil
	case GreaterThan:
		return query.Gt, nil
	case GreaterThanOrEqual:
		return query.Gte, nil
	case LessThan:
		return query.Lt, nil
	case LessThanOrEqual:
		return query.Lte, nil
	default:
		return 0, fmt.Errorf("koradb: unsupported comparison operator %q", op)
	}
}
