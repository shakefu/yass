/*
 * yaml_parse.h - YAML parsing wrapper
 */
#ifndef YASS_YAML_PARSE_H
#define YASS_YAML_PARSE_H

#include <stdbool.h>
#include <stddef.h>

/*
 * Use yass_ prefix to avoid collision with libyaml's yaml_node_t.
 */

/*
 * A key-value pair in a YAML mapping.
 */
typedef struct yass_yaml_kv {
    char *key;
    int key_line;       /* 1-based line number of the key */
    int value_type;     /* 0=scalar, 1=sequence, 2=mapping, 3=null */
    char *scalar_value; /* only if value_type == 0 */
    struct yass_yaml_kv *mapping_values;   /* only if value_type == 2 */
    int mapping_count;
    struct yass_yaml_node *sequence_values; /* only if value_type == 1 */
    int sequence_count;
} yass_yaml_kv_t;

/*
 * A YAML node (used for sequence items).
 */
typedef struct yass_yaml_node {
    int type;           /* 0=scalar, 1=sequence, 2=mapping, 3=null */
    int line;           /* 1-based line number */
    char *scalar_value; /* only if type == 0 */
    yass_yaml_kv_t *mapping_values;   /* only if type == 2 */
    int mapping_count;
    struct yass_yaml_node *sequence_values; /* only if type == 1 */
    int sequence_count;
} yass_yaml_node_t;

/*
 * A parsed YAML document (a mapping).
 */
typedef struct {
    yass_yaml_kv_t *kvs;
    int kv_count;
    int start_line;     /* 1-based line number of the document start */
} yass_yaml_doc_t;

/*
 * Result of parsing a YAML file.
 */
typedef struct {
    yass_yaml_doc_t *docs;
    int doc_count;
    bool has_anchors;
    bool has_duplicate_keys;
    char *duplicate_key_name;
    int duplicate_key_line;
} yass_yaml_parse_result_t;

/*
 * Parse a YAML file into a multi-document structure.
 * Returns 0 on success, or an error code.
 * On error, sets error_code and error_message.
 */
int yaml_parse_file(const char *path, yass_yaml_parse_result_t *result,
                    const char **error_code, char **error_message, int *error_line);

/*
 * Parse YAML from a buffer.
 */
int yaml_parse_buffer(const unsigned char *data, size_t len,
                      yass_yaml_parse_result_t *result,
                      const char **error_code, char **error_message, int *error_line);

/*
 * Free a parse result.
 */
void yaml_parse_result_free(yass_yaml_parse_result_t *result);

/*
 * Find a key in a document.
 */
yass_yaml_kv_t *yaml_doc_find_key(yass_yaml_doc_t *doc, const char *key);

/*
 * Get the string value of a key, or NULL if not found or not a scalar.
 */
const char *yaml_doc_get_string(yass_yaml_doc_t *doc, const char *key);

/*
 * Backward-compatible type aliases.
 * NOTE: yaml_node_t is NOT aliased because libyaml defines the same name.
 * Use yass_yaml_node_t directly, or use the aliases below for the others.
 */
typedef yass_yaml_kv_t yaml_kv_t;
typedef yass_yaml_doc_t yaml_doc_t;
typedef yass_yaml_parse_result_t yaml_parse_result_t;

#endif /* YASS_YAML_PARSE_H */
