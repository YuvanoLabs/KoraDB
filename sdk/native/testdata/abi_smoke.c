#include "koradb.h"

#include <stdio.h>
#include <string.h>

static int fail(const char *step, const char *error) {
    fprintf(stderr, "%s: %s\n", step, error);
    KoraDBFreeString((char *)error);
    return 1;
}

static int require_output(const char *step, char *value) {
    if (value == NULL || value[0] == '\0') {
        fprintf(stderr, "%s: expected a non-empty result\n", step);
        if (value != NULL) {
            KoraDBFreeString(value);
        }
        return 1;
    }
    KoraDBFreeString(value);
    return 0;
}

int main(int argc, char **argv) {
    const char *database_path = argc == 2 ? argv[1] : "koradb-native-abi-smoke.db";
    const char *schema =
        "syntax = \"proto3\";\n"
        "package example;\n"
        "message Person { string id = 1; string name = 2; string city = 3; }\n";
    const char *error;
    KoraDBHandle handle = 0;
    int32_t version = 0;
    char *id = NULL;
    char *document = NULL;
    char *first_page = NULL;
    char *first_token = NULL;
    char *second_page = NULL;
    char *second_token = NULL;

    if (require_output("KoraDBVersion", (char *)KoraDBVersion()) != 0) return 1;
    if ((error = KoraDBOpen(database_path, &handle)) != NULL) return fail("KoraDBOpen", error);
    if ((error = KoraDBRegisterSchema(handle, "person.proto", schema, &version)) != NULL) return fail("KoraDBRegisterSchema", error);
    if (version != 1) {
        fprintf(stderr, "schema version: got %d, want 1\n", version);
        return 1;
    }
    if ((error = KoraDBCreateCollection(handle, "people", "example.Person", "id", "[\"city\"]")) != NULL) return fail("KoraDBCreateCollection", error);
    if ((error = KoraDBInsertJSON(handle, "people", "{\"id\":\"alice\",\"name\":\"Alice\",\"city\":\"NYC\"}", &id)) != NULL) return fail("KoraDBInsertJSON alice", error);
    if (require_output("KoraDBInsertJSON alice id", id) != 0) return 1;
    id = NULL;
    if ((error = KoraDBInsertJSON(handle, "people", "{\"id\":\"carol\",\"name\":\"Carol\",\"city\":\"NYC\"}", &id)) != NULL) return fail("KoraDBInsertJSON carol", error);
    if (require_output("KoraDBInsertJSON carol id", id) != 0) return 1;
    id = NULL;
    if ((error = KoraDBGetJSON(handle, "people", "alice", &document)) != NULL) return fail("KoraDBGetJSON", error);
    if (require_output("KoraDBGetJSON result", document) != 0) return 1;
    document = NULL;
    if ((error = KoraDBQueryPageJSON(handle, "people", "city", "eq", "NYC", 1, NULL, &first_page, &first_token)) != NULL) return fail("KoraDBQueryPageJSON first", error);
    if (require_output("KoraDBQueryPageJSON first page", first_page) != 0) return 1;
    first_page = NULL;
    if (first_token == NULL || first_token[0] == '\0') {
        fprintf(stderr, "KoraDBQueryPageJSON first token: expected continuation token\n");
        return 1;
    }
    if ((error = KoraDBQueryPageJSON(handle, "people", "city", "eq", "NYC", 1, first_token, &second_page, &second_token)) != NULL) return fail("KoraDBQueryPageJSON second", error);
    KoraDBFreeString(first_token);
    first_token = NULL;
    if (require_output("KoraDBQueryPageJSON second page", second_page) != 0) return 1;
    second_page = NULL;
    if (second_token == NULL || second_token[0] != '\0') {
        fprintf(stderr, "KoraDBQueryPageJSON final token: expected empty token\n");
        if (second_token != NULL) KoraDBFreeString(second_token);
        return 1;
    }
    KoraDBFreeString(second_token);
    if ((error = KoraDBClose(handle)) != NULL) return fail("KoraDBClose", error);
    puts("KoraDB native ABI smoke test passed");
    return 0;
}
