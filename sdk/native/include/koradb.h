#ifndef KORADB_H
#define KORADB_H

#include <stdint.h>

#ifdef _WIN32
#define KORADB_API __declspec(dllimport)
#else
#define KORADB_API
#endif

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Pre-release native ABI for embedded language bindings.
 *
 * A null return value means success. A non-null return value is an allocated
 * UTF-8 error string and must be released with KoraDBFreeString. Every non-null
 * output string follows the same ownership rule. Database handles are opaque
 * and are valid until KoraDBClose succeeds.
 */
typedef uint64_t KoraDBHandle;

KORADB_API const char *KoraDBVersion(void);
KORADB_API void KoraDBFreeString(char *value);

KORADB_API const char *KoraDBOpen(const char *path, KoraDBHandle *out_handle);
KORADB_API const char *KoraDBClose(KoraDBHandle handle);

KORADB_API const char *KoraDBRegisterSchema(
    KoraDBHandle handle,
    const char *name,
    const char *proto_source,
    int32_t *out_version);

/* indexes_json is a JSON string array, for example: ["city","status"]. */
KORADB_API const char *KoraDBCreateCollection(
    KoraDBHandle handle,
    const char *name,
    const char *message_type,
    const char *key_field,
    const char *indexes_json);

KORADB_API const char *KoraDBInsertJSON(
    KoraDBHandle handle,
    const char *collection,
    const char *document_json,
    char **out_id);
KORADB_API const char *KoraDBGetJSON(
    KoraDBHandle handle,
    const char *collection,
    const char *id,
    char **out_document_json);
KORADB_API const char *KoraDBUpdateJSON(
    KoraDBHandle handle,
    const char *collection,
    const char *id,
    const char *document_json);
KORADB_API const char *KoraDBDelete(
    KoraDBHandle handle,
    const char *collection,
    const char *id);

/* out_results_json is a JSON array of {"id":"...","json":{...}}. */
KORADB_API const char *KoraDBQueryJSON(
    KoraDBHandle handle,
    const char *collection,
    const char *field,
    const char *operator_name,
    const char *value,
    char **out_results_json);

#ifdef __cplusplus
}
#endif

#endif
