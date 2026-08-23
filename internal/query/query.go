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
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"KoraDB/internal/engine"
	"KoraDB/internal/storage"
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

// Execute runs filter against the named collection and returns matching
// documents as JSON. A nil filter matches every document.
func Execute(db *engine.DB, coll string, filter Filter) ([]Result, error) {
	meta, err := db.GetCollection(coll)
	if err != nil {
		return nil, err
	}
	md, err := db.Registry().MessageDescriptor(meta.MessageType)
	if err != nil {
		return nil, err
	}

	var results []Result
	err = db.Store().View(func(tx *storage.Txn) error {
		seedKeys, seeded := indexSeed(db, tx, meta, filter)
		if seeded {
			for _, key := range seedKeys {
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
					j, err := db.RenderMessage(msg)
					if err != nil {
						return err
					}
					results = append(results, Result{ID: string(key), JSON: j})
				}
			}
			return nil
		}

		// Full scan fallback.
		return db.Each(tx, meta, func(key []byte, msg *dynamicpb.Message) error {
			ok, err := eval(md, msg, filter)
			if err != nil {
				return err
			}
			if ok {
				j, err := db.RenderMessage(msg)
				if err != nil {
					return err
				}
				results = append(results, Result{ID: string(key), JSON: j})
			}
			return nil
		})
	})
	return results, err
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
