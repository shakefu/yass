/*
 * check_yaml.c - YAML well-formedness check per cli.validate.CheckYAML
 */
#include "check_yaml.h"
#include "yaml_parse.h"
#include "error.h"
#include "utf8.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/*
 * Check YAML well-formedness of a file.
 * On success, populates result with parsed documents.
 * On error, emits one ErrorLine and returns non-zero.
 *
 * Priority order per spec:
 * 1. yass.yaml.not_utf8
 * 2. yass.yaml.has_bom
 * 3. yass.yaml.empty_file
 * 4. yass.yaml.malformed
 * 5. yass.yaml.duplicate_key
 * 6. yass.yaml.anchor_or_alias
 */
int check_yaml(const char *file, yaml_parse_result_t *result) {
    memset(result, 0, sizeof(yaml_parse_result_t));

    /* Open and read the file */
    FILE *f = fopen(file, "rb");
    if (!f) {
        error_emit(file, 0, EC_YAML_MALFORMED, "YAML well-formedness error");
        return 1;
    }

    fseek(f, 0, SEEK_END);
    long file_size = ftell(f);
    fseek(f, 0, SEEK_SET);

    if (file_size < 0) {
        fclose(f);
        error_emit(file, 0, EC_YAML_MALFORMED, "YAML well-formedness error");
        return 1;
    }

    size_t len = (size_t)file_size;

    /* Priority 3: empty file */
    if (len == 0) {
        fclose(f);
        error_emit(file, 0, EC_YAML_EMPTY_FILE, "empty file");
        return 1;
    }

    /* Read file contents */
    unsigned char *data = malloc(len + 1);
    if (!data) {
        fclose(f);
        error_emit(file, 0, EC_YAML_MALFORMED, "YAML well-formedness error");
        return 1;
    }

    size_t read_len = fread(data, 1, len, f);
    fclose(f);

    if (read_len != len) {
        free(data);
        error_emit(file, 0, EC_YAML_MALFORMED, "YAML well-formedness error");
        return 1;
    }

    data[len] = '\0';

    /* Priority 1: not valid UTF-8 */
    if (!utf8_is_valid(data, len)) {
        free(data);
        error_emit(file, 0, EC_YAML_NOT_UTF8, "file is not valid UTF-8");
        return 1;
    }

    /* Priority 2: UTF-8 BOM */
    if (utf8_has_bom(data, len)) {
        free(data);
        error_emit(file, 0, EC_YAML_HAS_BOM, "file begins with a UTF-8 BOM");
        return 1;
    }

    /* Parse the YAML buffer */
    const char *error_code = NULL;
    char *error_message = NULL;
    int error_line = 0;

    int rc = yaml_parse_buffer(data, len, result, &error_code, &error_message, &error_line);
    free(data);

    if (rc != 0) {
        /*
         * yaml_parse_buffer returned an error. It may be malformed, duplicate_key,
         * or anchor_or_alias. We need to apply priority ordering.
         *
         * When yaml_parse_buffer detects anchors or duplicate keys, it still
         * populates the result struct before returning -1. We can inspect
         * result->has_duplicate_keys to determine if duplicate_key should
         * take priority over anchor_or_alias.
         */

        /* Check if this is a genuine parse failure (malformed) */
        if (error_code && strcmp(error_code, EC_YAML_MALFORMED) == 0) {
            /* Priority 4: malformed YAML */
            error_emit(file, error_line, EC_YAML_MALFORMED,
                       "YAML well-formedness error");
            free(error_message);
            yaml_parse_result_free(result);
            return 1;
        }

        /* Priority 5: duplicate key (takes precedence over anchor) */
        if (result->has_duplicate_keys) {
            char msg[256];
            snprintf(msg, sizeof(msg), "duplicate mapping key: %s",
                     result->duplicate_key_name ? result->duplicate_key_name : "(unknown)");
            error_emit(file, result->duplicate_key_line, EC_YAML_DUPLICATE_KEY, msg);
            free(error_message);
            yaml_parse_result_free(result);
            return 1;
        }

        /* Priority 6: anchor or alias */
        if (result->has_anchors) {
            error_emit(file, 0, EC_YAML_ANCHOR_OR_ALIAS,
                       "YAML anchors, aliases, and explicit tags are not allowed");
            free(error_message);
            yaml_parse_result_free(result);
            return 1;
        }

        /* Some other error from the parser (e.g. empty_stream) */
        error_emit(file, error_line, error_code ? error_code : EC_YAML_MALFORMED,
                   error_message ? error_message : "YAML well-formedness error");
        free(error_message);
        yaml_parse_result_free(result);
        return 1;
    }

    /* Parse succeeded - check post-parse conditions with priority ordering */

    /* Priority 5: duplicate key */
    if (result->has_duplicate_keys) {
        char msg[256];
        snprintf(msg, sizeof(msg), "duplicate mapping key: %s",
                 result->duplicate_key_name ? result->duplicate_key_name : "(unknown)");
        error_emit(file, result->duplicate_key_line, EC_YAML_DUPLICATE_KEY, msg);
        yaml_parse_result_free(result);
        return 1;
    }

    /* Priority 6: anchor or alias */
    if (result->has_anchors) {
        error_emit(file, 0, EC_YAML_ANCHOR_OR_ALIAS,
                   "YAML anchors, aliases, and explicit tags are not allowed");
        yaml_parse_result_free(result);
        return 1;
    }

    /* Success - result is populated */
    return 0;
}
