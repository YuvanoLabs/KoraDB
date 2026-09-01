// Package query is Layer 4 of KoraDB: a small but real query engine.
//
// A query is an AST of predicates over a collection's fields. The executor
// evaluates it against documents decoded through the collection's schema. When
// a query contains an equality predicate on an indexed field, the executor
// seeds its candidate set from that index (a prefix scan) instead of reading
// every document; otherwise it falls back to a full collection scan. Either
// way the complete filter is re-checked against each candidate, so results are
// always correct regardless of which path was taken.
package query

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/YuvanoLabs/KoraDB/internal/engine"
	"github.com/YuvanoLabs/KoraDB/internal/storage"
)

// Op is a comparison operator.
type Op int

const (
	Eq Op = iota
	Ne
	Gt
	Gte
	Lt
	Lte
)

func (o Op) String() string {
	switch o {
	case Eq:
		return "=="
	case Ne:
		return "!="
	case Gt:
		return ">"
	case Gte:
		return ">="
	case Lt:
		return "<"
	case Lte:
		return "<="
	default:
		return "?"
	}
}

// Filter is a node in the query AST.
type Filter interface{ isFilter() }

// Cmp compares a field against a literal (given in string form, typed against
// the field at evaluation time).
type Cmp struct {
	Field string
	Op    Op
	Value string
}

// And is satisfied when all sub-filters are satisfied. An empty And matches all.
type And struct{ Filters []Filter }

// Or is satisfied when any sub-filter is satisfied.
type Or struct{ Filters []Filter }

func (Cmp) isFilter() {}
func (And) isFilter() {}
func (Or) isFilter()  {}

// Result is one matched document.
type Result struct {
	ID   string
	JSON []byte
}

// DefaultResultLimit bounds the documents materialized by the current unary
// query API. Pagination must replace this fixed ceiling before a stable SDK
// promises traversal of arbitrarily large result sets.
const DefaultResultLimit = 1000

// Page is one bounded, ordered query result page. NextPageToken is opaque to
// callers. Supplying it to ExecutePage with the same collection and filter
// resumes after the final result in Results. An empty token means the page is
// the last page.
type Page struct {
	Results       []Result
	NextPageToken string
}

// ErrInvalidPageToken is returned when a continuation token is malformed or
// does not belong to the collection and filter used for the request.
var ErrInvalidPageToken = errors.New("query: invalid page token")

type pageCursor struct {
	Version    int    `json:"v"`
	Collection string `json:"c"`
	FilterHash string `json:"f"`
	LastKey    []byte `json:"k"`
}

// ResultLimitError reports that a query matched more documents than the caller
// allowed to materialize in one response.
type ResultLimitError struct{ Limit int }

func (e *ResultLimitError) Error() string {
	return fmt.Sprintf("query: result limit of %d exceeded", e.Limit)
}

// Execute runs filter against the named collection with the default bounded
// result limit. A nil filter matches every document.
func Execute(db *engine.DB, coll string, filter Filter) ([]Result, error) {
	return ExecuteWithLimit(db, coll, filter, DefaultResultLimit)
}

// ExecuteWithLimit runs filter against the named collection and materializes at
// most limit results. It returns ResultLimitError rather than a partial result
// when the limit is exceeded.
func ExecuteWithLimit(db *engine.DB, coll string, filter Filter, limit int) ([]Result, error) {
	return ExecuteWithLimitContext(context.Background(), db, coll, filter, limit)
}

// ExecuteWithLimitContext is ExecuteWithLimit with cancellation support.
func ExecuteWithLimitContext(ctx context.Context, db *engine.DB, coll string, filter Filter, limit int) ([]Result, error) {
	page, err := ExecutePageContext(ctx, db, coll, filter, limit, "")
	if err != nil {
		return nil, err
	}
	if page.NextPageToken != "" {
		// A caller of the legacy unary API must never receive a partial set
		// without having opted into the continuation-token contract.
		return nil, &ResultLimitError{Limit: limit}
	}
	return page.Results, nil
}

// ExecutePage runs filter against coll and materializes at most pageSize
// documents. Results are ordered by the collection's raw primary-key ordering.
// The continuation token binds to the collection and an exact structural hash
// of filter, preventing a token from one query from being reused for another.
//
// A continuation token provides forward-only traversal. It does not create a
// long-lived database snapshot: callers should expect concurrent writes after
// a page boundary to follow normal single-node read semantics.
func ExecutePage(db *engine.DB, coll string, filter Filter, pageSize int, pageToken string) (Page, error) {
	return ExecutePageContext(context.Background(), db, coll, filter, pageSize, pageToken)
}

// ExecutePageContext is ExecutePage with cancellation support. The context is
// checked before a page begins and as documents are scanned or materialized.
// It cannot interrupt an individual storage transaction, but it prevents a
// cancelled service request from continuing through a long collection scan.
func ExecutePageContext(ctx context.Context, db *engine.DB, coll string, filter Filter, pageSize int, pageToken string) (Page, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	if pageSize <= 0 {
		return Page{}, fmt.Errorf("query: page size must be positive")
	}
	filterHash, err := filterHash(filter)
	if err != nil {
		return Page{}, err
	}
	cursor, err := parsePageToken(pageToken, coll, filterHash)
	if err != nil {
		return Page{}, err
	}
	meta, err := db.GetCollection(coll)
	if err != nil {
		return Page{}, err
	}
	md, err := db.Registry().MessageDescriptor(meta.MessageType)
	if err != nil {
		return Page{}, err
	}

	page := Page{}
	var lastKey []byte
	more := false
	appendResult := func(key []byte, msg *dynamicpb.Message) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(page.Results) >= pageSize {
			more = true
			return errPageComplete
		}
		j, err := db.RenderMessage(msg)
		if err != nil {
			return err
		}
		id, err := db.IDFromStorageKey(meta, key)
		if err != nil {
			return err
		}
		page.Results = append(page.Results, Result{ID: id, JSON: j})
		lastKey = append(lastKey[:0], key...)
		return nil
	}
	err = db.Store().View(func(tx *storage.Txn) error {
		seedKeys, seeded := indexSeed(db, tx, meta, filter)
		if seeded {
			sort.Slice(seedKeys, func(i, j int) bool { return bytes.Compare(seedKeys[i], seedKeys[j]) < 0 })
			for _, key := range seedKeys {
				if err := ctx.Err(); err != nil {
					return err
				}
				if len(cursor.LastKey) > 0 && bytes.Compare(key, cursor.LastKey) <= 0 {
					continue
				}
				msg, err := db.FetchByKey(tx, meta, key)
				if err == engine.ErrNotFound {
					continue
				}
				if err != nil {
					return err
				}
				ok, err := eval(md, msg, filter)
				if err != nil {
					return err
				}
				if ok {
					if err := appendResult(key, msg); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Full scan fallback.
		return db.Each(tx, meta, func(key []byte, msg *dynamicpb.Message) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(cursor.LastKey) > 0 && bytes.Compare(key, cursor.LastKey) <= 0 {
				return nil
			}
			ok, err := eval(md, msg, filter)
			if err != nil {
				return err
			}
			if ok {
				return appendResult(key, msg)
			}
			return nil
		})
	})
	if err != nil && !errors.Is(err, errPageComplete) {
		return Page{}, err
	}
	if more {
		token, err := encodePageToken(pageCursor{
			Version:    1,
			Collection: coll,
			FilterHash: filterHash,
			LastKey:    lastKey,
		})
		if err != nil {
			return Page{}, err
		}
		page.NextPageToken = token
	}
	return page, nil
}

var errPageComplete = errors.New("query: page complete")

func encodePageToken(cursor pageCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("query: encode page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func parsePageToken(token, collection, expectedFilterHash string) (pageCursor, error) {
	if token == "" {
		return pageCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return pageCursor{}, fmt.Errorf("%w: malformed encoding", ErrInvalidPageToken)
	}
	var cursor pageCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return pageCursor{}, fmt.Errorf("%w: malformed body", ErrInvalidPageToken)
	}
	if cursor.Version != 1 || len(cursor.LastKey) == 0 || cursor.Collection != collection || cursor.FilterHash != expectedFilterHash {
		return pageCursor{}, fmt.Errorf("%w: query does not match", ErrInvalidPageToken)
	}
	return cursor, nil
}

func filterHash(filter Filter) (string, error) {
	var signature strings.Builder
	if err := writeFilterSignature(&signature, filter); err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(signature.String()))
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func writeFilterSignature(out *strings.Builder, filter Filter) error {
	switch f := filter.(type) {
	case nil:
		out.WriteString("nil;")
	case Cmp:
		fmt.Fprintf(out, "cmp:%d:%s:%d:%d:%s;", len(f.Field), f.Field, int(f.Op), len(f.Value), f.Value)
	case And:
		fmt.Fprintf(out, "and:%d:[", len(f.Filters))
		for _, sub := range f.Filters {
			if err := writeFilterSignature(out, sub); err != nil {
				return err
			}
		}
		out.WriteString("];")
	case Or:
		fmt.Fprintf(out, "or:%d:[", len(f.Filters))
		for _, sub := range f.Filters {
			if err := writeFilterSignature(out, sub); err != nil {
				return err
			}
		}
		out.WriteString("];")
	default:
		return fmt.Errorf("query: unknown filter type %T", filter)
	}
	return nil
}

// indexSeed looks for an equality predicate on an indexed field that can seed
// the candidate set, returning the matching primary keys. It inspects the
// top-level filter and the members of a top-level And (the common case). Or
// nodes are not seeded — a missing branch could drop valid matches — so those
// fall back to a full scan.
func indexSeed(db *engine.DB, tx *storage.Txn, meta *engine.CollectionMeta, filter Filter) ([][]byte, bool) {
	var candidates []Cmp
	switch f := filter.(type) {
	case Cmp:
		candidates = []Cmp{f}
	case And:
		for _, sub := range f.Filters {
			if c, ok := sub.(Cmp); ok {
				candidates = append(candidates, c)
			}
		}
	}
	for _, c := range candidates {
		if c.Op == Eq && meta.HasIndex(c.Field) {
			keys, err := db.LookupByIndex(tx, meta, c.Field, c.Value)
			if err == nil {
				return keys, true
			}
		}
	}
	return nil, false
}

// eval reports whether msg satisfies filter.
func eval(md protoreflect.MessageDescriptor, msg protoreflect.Message, filter Filter) (bool, error) {
	switch f := filter.(type) {
	case nil:
		return true, nil

	case And:
		for _, sub := range f.Filters {
			ok, err := eval(md, msg, sub)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil

	case Or:
		for _, sub := range f.Filters {
			ok, err := eval(md, msg, sub)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case Cmp:
		return evalCmp(md, msg, f)

	default:
		return false, fmt.Errorf("query: unknown filter type %T", filter)
	}
}

func evalCmp(md protoreflect.MessageDescriptor, msg protoreflect.Message, c Cmp) (bool, error) {
	fd := md.Fields().ByName(protoreflect.Name(c.Field))
	if fd == nil {
		return false, fmt.Errorf("query: field %q not in %s", c.Field, md.FullName())
	}
	if fd.IsList() || fd.IsMap() || fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return false, fmt.Errorf("query: cannot compare repeated/map/message field %q", c.Field)
	}
	literal, err := engine.ScalarValueFromString(fd, c.Value)
	if err != nil {
		return false, err
	}
	cmp := engine.CompareValues(fd, msg.Get(fd), literal)
	switch c.Op {
	case Eq:
		return cmp == 0, nil
	case Ne:
		return cmp != 0, nil
	case Gt:
		return cmp > 0, nil
	case Gte:
		return cmp >= 0, nil
	case Lt:
		return cmp < 0, nil
	case Lte:
		return cmp <= 0, nil
	default:
		return false, fmt.Errorf("query: unknown operator %v", c.Op)
	}
}
