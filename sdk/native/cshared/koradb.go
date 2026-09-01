//go:build cgo

// KoraDB's pre-release C ABI. Build this package with:
//
//	go build -buildmode=c-shared -o koradb-native.<ext> ./sdk/native/cshared
package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/YuvanoLabs/KoraDB/internal/buildinfo"
	"github.com/YuvanoLabs/KoraDB/internal/engine"
	"github.com/YuvanoLabs/KoraDB/internal/query"
)

const nativeAPIVersion = "0.2.0-pre"

var nativeDatabases sync.Map // map[uint64]*engine.DB
var nativeNextHandle atomic.Uint64

type abiDocument struct {
	ID   string          `json:"id"`
	JSON json.RawMessage `json:"json"`
}

//export KoraDBVersion
func KoraDBVersion() *C.char {
	return C.CString(buildinfo.String() + "; native-abi=" + nativeAPIVersion)
}

//export KoraDBFreeString
func KoraDBFreeString(value *C.char) {
	if value != nil {
		C.free(unsafe.Pointer(value))
	}
}

//export KoraDBOpen
func KoraDBOpen(path *C.char, outHandle *C.uint64_t) *C.char {
	value, err := requiredCString(path, "path")
	if err != nil {
		return nativeError(err)
	}
	if outHandle == nil {
		return nativeError(fmt.Errorf("koradb native: out_handle is required"))
	}
	db, err := engine.Open(value)
	if err != nil {
		return nativeError(err)
	}
	handle := nativeNextHandle.Add(1)
	nativeDatabases.Store(handle, db)
	*outHandle = C.uint64_t(handle)
	return nil
}

//export KoraDBClose
func KoraDBClose(handle C.uint64_t) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	if err := db.Close(); err != nil {
		return nativeError(err)
	}
	nativeDatabases.Delete(uint64(handle))
	return nil
}

//export KoraDBRegisterSchema
func KoraDBRegisterSchema(handle C.uint64_t, name, protoSource *C.char, outVersion *C.int32_t) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	logicalName, err := requiredCString(name, "schema name")
	if err != nil {
		return nativeError(err)
	}
	source, err := requiredCString(protoSource, "proto source")
	if err != nil {
		return nativeError(err)
	}
	if outVersion == nil {
		return nativeError(fmt.Errorf("koradb native: out_version is required"))
	}
	version, err := db.RegisterSchema(context.Background(), logicalName, source)
	if err != nil {
		return nativeError(err)
	}
	*outVersion = C.int32_t(version)
	return nil
}

//export KoraDBCreateCollection
func KoraDBCreateCollection(handle C.uint64_t, name, messageType, keyField, indexesJSON *C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	collectionName, err := requiredCString(name, "collection name")
	if err != nil {
		return nativeError(err)
	}
	message, err := requiredCString(messageType, "message type")
	if err != nil {
		return nativeError(err)
	}
	indexes, err := indexesFromJSON(indexesJSON)
	if err != nil {
		return nativeError(err)
	}
	_, err = db.CreateCollection(collectionName, message, &engine.CollectionOptions{
		KeyField: optionalCString(keyField),
		Indexes:  indexes,
	})
	if err != nil {
		return nativeError(err)
	}
	return nil
}

//export KoraDBInsertJSON
func KoraDBInsertJSON(handle C.uint64_t, collection, documentJSON *C.char, outID **C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	if outID == nil {
		return nativeError(fmt.Errorf("koradb native: out_id is required"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	document, err := requiredCString(documentJSON, "document JSON")
	if err != nil {
		return nativeError(err)
	}
	id, err := db.Insert(collectionName, []byte(document))
	if err != nil {
		return nativeError(err)
	}
	*outID = C.CString(id)
	return nil
}

//export KoraDBGetJSON
func KoraDBGetJSON(handle C.uint64_t, collection, id *C.char, outDocumentJSON **C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	if outDocumentJSON == nil {
		return nativeError(fmt.Errorf("koradb native: out_document_json is required"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	documentID, err := requiredCString(id, "id")
	if err != nil {
		return nativeError(err)
	}
	document, err := db.Get(collectionName, documentID)
	if err != nil {
		return nativeError(err)
	}
	*outDocumentJSON = C.CString(string(document))
	return nil
}

//export KoraDBUpdateJSON
func KoraDBUpdateJSON(handle C.uint64_t, collection, id, documentJSON *C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	documentID, err := requiredCString(id, "id")
	if err != nil {
		return nativeError(err)
	}
	document, err := requiredCString(documentJSON, "document JSON")
	if err != nil {
		return nativeError(err)
	}
	if err := db.Update(collectionName, documentID, []byte(document)); err != nil {
		return nativeError(err)
	}
	return nil
}

//export KoraDBDelete
func KoraDBDelete(handle C.uint64_t, collection, id *C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	documentID, err := requiredCString(id, "id")
	if err != nil {
		return nativeError(err)
	}
	if err := db.Delete(collectionName, documentID); err != nil {
		return nativeError(err)
	}
	return nil
}

//export KoraDBQueryJSON
func KoraDBQueryJSON(handle C.uint64_t, collection, field, operatorName, value *C.char, outResultsJSON **C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	if outResultsJSON == nil {
		return nativeError(fmt.Errorf("koradb native: out_results_json is required"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	fieldName, err := requiredCString(field, "field")
	if err != nil {
		return nativeError(err)
	}
	operator, err := requiredCString(operatorName, "operator")
	if err != nil {
		return nativeError(err)
	}
	literal, err := requiredCString(value, "value")
	if err != nil {
		return nativeError(err)
	}
	op, err := nativeQueryOp(operator)
	if err != nil {
		return nativeError(err)
	}
	results, err := query.Execute(db, collectionName, query.Cmp{Field: fieldName, Op: op, Value: literal})
	if err != nil {
		return nativeError(err)
	}
	out := make([]abiDocument, 0, len(results))
	for _, result := range results {
		out = append(out, abiDocument{ID: result.ID, JSON: result.JSON})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nativeError(err)
	}
	*outResultsJSON = C.CString(string(encoded))
	return nil
}

//export KoraDBQueryPageJSON
func KoraDBQueryPageJSON(handle C.uint64_t, collection, field, operatorName, value *C.char, pageSize C.int32_t, pageToken *C.char, outResultsJSON, outNextPageToken **C.char) *C.char {
	db, ok := nativeDatabase(handle)
	if !ok {
		return nativeError(fmt.Errorf("koradb native: invalid database handle"))
	}
	if outResultsJSON == nil || outNextPageToken == nil {
		return nativeError(fmt.Errorf("koradb native: output pointers are required"))
	}
	collectionName, err := requiredCString(collection, "collection")
	if err != nil {
		return nativeError(err)
	}
	fieldName, err := requiredCString(field, "field")
	if err != nil {
		return nativeError(err)
	}
	operator, err := requiredCString(operatorName, "operator")
	if err != nil {
		return nativeError(err)
	}
	literal, err := requiredCString(value, "value")
	if err != nil {
		return nativeError(err)
	}
	op, err := nativeQueryOp(operator)
	if err != nil {
		return nativeError(err)
	}
	page, err := query.ExecutePage(db, collectionName, query.Cmp{Field: fieldName, Op: op, Value: literal}, int(pageSize), optionalCString(pageToken))
	if err != nil {
		return nativeError(err)
	}
	out := make([]abiDocument, 0, len(page.Results))
	for _, result := range page.Results {
		out = append(out, abiDocument{ID: result.ID, JSON: result.JSON})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nativeError(err)
	}
	*outResultsJSON = C.CString(string(encoded))
	*outNextPageToken = C.CString(page.NextPageToken)
	return nil
}

func main() {}

func nativeDatabase(handle C.uint64_t) (*engine.DB, bool) {
	value, ok := nativeDatabases.Load(uint64(handle))
	if !ok {
		return nil, false
	}
	db, ok := value.(*engine.DB)
	return db, ok
}

func requiredCString(value *C.char, field string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("koradb native: %s is required", field)
	}
	text := C.GoString(value)
	if text == "" {
		return "", fmt.Errorf("koradb native: %s must not be empty", field)
	}
	return text, nil
}

func optionalCString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func indexesFromJSON(value *C.char) ([]string, error) {
	raw := optionalCString(value)
	if raw == "" {
		return nil, nil
	}
	var indexes []string
	if err := json.Unmarshal([]byte(raw), &indexes); err != nil {
		return nil, fmt.Errorf("koradb native: indexes_json must be a JSON string array: %w", err)
	}
	return indexes, nil
}

func nativeQueryOp(value string) (query.Op, error) {
	switch value {
	case "eq":
		return query.Eq, nil
	case "ne":
		return query.Ne, nil
	case "gt":
		return query.Gt, nil
	case "gte":
		return query.Gte, nil
	case "lt":
		return query.Lt, nil
	case "lte":
		return query.Lte, nil
	default:
		return 0, fmt.Errorf("koradb native: unsupported comparison operator %q", value)
	}
}

func nativeError(err error) *C.char {
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}
