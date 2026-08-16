/*
 * check_uniqueness.c - Spec name uniqueness check per cli.validate.CheckUniqueness
 */
#include "check_uniqueness.h"
#include "yaml_parse.h"
#include "error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/*
 * Check that all spec names are unique within a file.
 * Emits one error per duplicate-after-the-first occurrence.
 * Returns the number of errors found.
 */
int check_uniqueness(const char *file, yaml_parse_result_t *result)
{
    if (!result || result->doc_count < 2)
        return 0;

    int errors = 0;

    /* Collect spec names from docs at index >= 1 (skip preamble) */
    int capacity = result->doc_count - 1;
    const char **names = malloc(sizeof(const char *) * capacity);
    int *lines = malloc(sizeof(int) * capacity);
    int name_count = 0;

    if (!names || !lines) {
        free(names);
        free(lines);
        return 0;
    }

    for (int i = 1; i < result->doc_count; i++) {
        yaml_doc_t *doc = &result->docs[i];
        const char *spec_name = yaml_doc_get_string(doc, "spec");
        if (!spec_name)
            continue;

        /* Determine the line: prefer the line of the "spec" key, fall back
         * to the document start_line */
        int line = doc->start_line;
        yaml_kv_t *spec_kv = yaml_doc_find_key(doc, "spec");
        if (spec_kv)
            line = spec_kv->key_line;

        /* Check if this name was already seen */
        int is_duplicate = 0;
        for (int j = 0; j < name_count; j++) {
            if (strcmp(names[j], spec_name) == 0) {
                is_duplicate = 1;
                break;
            }
        }

        if (is_duplicate) {
            /* Build error message */
            size_t msg_len = strlen("duplicate spec name in file: ") +
                             strlen(spec_name) + 1;
            char *msg = malloc(msg_len);
            if (msg) {
                snprintf(msg, msg_len, "duplicate spec name in file: %s",
                         spec_name);
                error_emit(file, line, EC_SPEC_DUPLICATE_NAME, msg);
                free(msg);
            }
            errors++;
        } else {
            /* Record the name as seen */
            names[name_count] = spec_name;
            lines[name_count] = line;
            name_count++;
        }
    }

    free(names);
    free(lines);
    return errors;
}
