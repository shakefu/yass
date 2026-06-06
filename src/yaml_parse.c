/*
 * yaml_parse.c - YAML parsing layer using libyaml
 */

#include "yaml_parse.h"
#include <yaml.h>
#include "utf8.h"
#include "error.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Forward declarations for recursive parsing */
static int parse_mapping(yaml_parser_t *parser, yaml_kv_t **kvs, int *kv_count,
                         bool *has_anchors, bool *has_duplicate_keys,
                         char **dup_key_name, int *dup_key_line);
static int parse_sequence(yaml_parser_t *parser, yass_yaml_node_t **nodes, int *node_count,
                          bool *has_anchors, bool *has_duplicate_keys,
                          char **dup_key_name, int *dup_key_line);

/* Free helpers */
static void free_kvs(yaml_kv_t *kvs, int count);
static void free_nodes(yass_yaml_node_t *nodes, int count);

/*
 * Check if a tag is non-default (explicit).
 * Default tags are NULL, or start with "!" or "!!".
 * Actually, libyaml resolves default tags to full URIs like
 * "tag:yaml.org,2002:str". We consider any non-NULL tag as
 * potentially explicit, but libyaml sets tag to NULL when no
 * explicit tag is given and the tag is resolved implicitly.
 * When an explicit tag like !!str is used, libyaml sets the tag
 * to "tag:yaml.org,2002:str" but also sets plain_implicit=0
 * or quoted_implicit=0.
 *
 * For simplicity: if the scalar has a tag AND both plain_implicit
 * and quoted_implicit are 0, it's an explicit tag.
 * For mapping/sequence: if anchor is set we already track it.
 * For tags on mapping/sequence: if tag is non-NULL, flag it.
 */
static bool is_explicit_tag_scalar(yaml_event_t *event) {
    if (event->type != YAML_SCALAR_EVENT) return false;
    yaml_char_t *tag = event->data.scalar.tag;
    if (tag == NULL) return false;
    /* If neither implicit flag is set, the tag was explicitly specified */
    if (!event->data.scalar.plain_implicit &&
        !event->data.scalar.quoted_implicit) {
        return true;
    }
    return false;
}

static bool is_explicit_tag_mapping(yaml_event_t *event) {
    if (event->type != YAML_MAPPING_START_EVENT) return false;
    yaml_char_t *tag = event->data.mapping_start.tag;
    if (tag == NULL) return false;
    if (!event->data.mapping_start.implicit) return true;
    return false;
}

static bool is_explicit_tag_sequence(yaml_event_t *event) {
    if (event->type != YAML_SEQUENCE_START_EVENT) return false;
    yaml_char_t *tag = event->data.sequence_start.tag;
    if (tag == NULL) return false;
    if (!event->data.sequence_start.implicit) return true;
    return false;
}

/*
 * Check for anchors on any event type that supports them.
 */
static bool has_anchor(yaml_event_t *event) {
    switch (event->type) {
        case YAML_SCALAR_EVENT:
            return event->data.scalar.anchor != NULL;
        case YAML_MAPPING_START_EVENT:
            return event->data.mapping_start.anchor != NULL;
        case YAML_SEQUENCE_START_EVENT:
            return event->data.sequence_start.anchor != NULL;
        default:
            return false;
    }
}

/*
 * Check for any anchor/alias/tag issues on an event.
 */
static void check_event_flags(yaml_event_t *event, bool *has_anchors_flag) {
    if (event->type == YAML_ALIAS_EVENT) {
        *has_anchors_flag = true;
        return;
    }
    if (has_anchor(event)) {
        *has_anchors_flag = true;
    }
    if (is_explicit_tag_scalar(event) ||
        is_explicit_tag_mapping(event) ||
        is_explicit_tag_sequence(event)) {
        *has_anchors_flag = true;
    }
}

/*
 * Parse the value portion after a key in a mapping.
 * Returns 0 on success, -1 on error.
 * The next event determines the value type.
 */
static int parse_value(yaml_parser_t *parser, yaml_kv_t *kv,
                       bool *has_anchors, bool *has_duplicate_keys,
                       char **dup_key_name, int *dup_key_line) {
    yaml_event_t event;
    if (!yaml_parser_parse(parser, &event)) return -1;

    check_event_flags(&event, has_anchors);

    switch (event.type) {
        case YAML_SCALAR_EVENT: {
            kv->value_type = 0; /* scalar */
            kv->scalar_value = strdup((const char *)event.data.scalar.value);
            yaml_event_delete(&event);
            return 0;
        }
        case YAML_MAPPING_START_EVENT: {
            kv->value_type = 2; /* mapping */
            yaml_event_delete(&event);
            return parse_mapping(parser, &kv->mapping_values, &kv->mapping_count,
                                has_anchors, has_duplicate_keys,
                                dup_key_name, dup_key_line);
        }
        case YAML_SEQUENCE_START_EVENT: {
            kv->value_type = 1; /* sequence */
            yaml_event_delete(&event);
            return parse_sequence(parser, &kv->sequence_values, &kv->sequence_count,
                                 has_anchors, has_duplicate_keys,
                                 dup_key_name, dup_key_line);
        }
        default: {
            /* Null or unexpected - treat as null */
            kv->value_type = 3; /* null */
            yaml_event_delete(&event);
            return 0;
        }
    }
}

/*
 * Parse a YAML mapping (between MAPPING_START and MAPPING_END).
 * The MAPPING_START event has already been consumed.
 */
static int parse_mapping(yaml_parser_t *parser, yaml_kv_t **kvs, int *kv_count,
                         bool *has_anchors, bool *has_duplicate_keys,
                         char **dup_key_name, int *dup_key_line) {
    *kvs = NULL;
    *kv_count = 0;

    int capacity = 0;

    while (1) {
        yaml_event_t event;
        if (!yaml_parser_parse(parser, &event)) return -1;

        check_event_flags(&event, has_anchors);

        if (event.type == YAML_MAPPING_END_EVENT) {
            yaml_event_delete(&event);
            break;
        }

        if (event.type != YAML_SCALAR_EVENT) {
            /* Keys must be scalars; skip non-scalar keys */
            yaml_event_delete(&event);
            continue;
        }

        /* We have a key */
        const char *key_str = (const char *)event.data.scalar.value;
        int key_line = (int)event.start_mark.line + 1;

        /* Check for duplicate keys in this mapping */
        if (!*has_duplicate_keys) {
            for (int i = 0; i < *kv_count; i++) {
                if (strcmp((*kvs)[i].key, key_str) == 0) {
                    *has_duplicate_keys = true;
                    if (*dup_key_name == NULL) {
                        *dup_key_name = strdup(key_str);
                        *dup_key_line = key_line;
                    }
                    break;
                }
            }
        }

        /* Grow array if needed */
        if (*kv_count >= capacity) {
            capacity = capacity == 0 ? 4 : capacity * 2;
            *kvs = realloc(*kvs, (size_t)capacity * sizeof(yaml_kv_t));
        }

        yaml_kv_t *kv = &(*kvs)[*kv_count];
        memset(kv, 0, sizeof(yaml_kv_t));
        kv->key = strdup(key_str);
        kv->key_line = key_line;

        yaml_event_delete(&event);

        /* Parse the value */
        if (parse_value(parser, kv, has_anchors, has_duplicate_keys,
                        dup_key_name, dup_key_line) != 0) {
            return -1;
        }

        (*kv_count)++;
    }

    return 0;
}

/*
 * Parse a YAML sequence (between SEQUENCE_START and SEQUENCE_END).
 * The SEQUENCE_START event has already been consumed.
 */
static int parse_sequence(yaml_parser_t *parser, yass_yaml_node_t **nodes, int *node_count,
                          bool *has_anchors, bool *has_duplicate_keys,
                          char **dup_key_name, int *dup_key_line) {
    *nodes = NULL;
    *node_count = 0;

    int capacity = 0;

    while (1) {
        /* Peek at next event to check for SEQUENCE_END */
        yaml_event_t peek;
        if (!yaml_parser_parse(parser, &peek)) return -1;

        check_event_flags(&peek, has_anchors);

        if (peek.type == YAML_SEQUENCE_END_EVENT) {
            yaml_event_delete(&peek);
            break;
        }

        /* Grow array if needed */
        if (*node_count >= capacity) {
            capacity = capacity == 0 ? 4 : capacity * 2;
            *nodes = realloc(*nodes, (size_t)capacity * sizeof(yass_yaml_node_t));
        }

        yass_yaml_node_t *node = &(*nodes)[*node_count];
        memset(node, 0, sizeof(yass_yaml_node_t));
        node->line = (int)peek.start_mark.line + 1;

        /* Process this event as the node value */
        switch (peek.type) {
            case YAML_SCALAR_EVENT: {
                node->type = 0;
                node->scalar_value = strdup((const char *)peek.data.scalar.value);
                yaml_event_delete(&peek);
                break;
            }
            case YAML_MAPPING_START_EVENT: {
                node->type = 2;
                yaml_event_delete(&peek);
                if (parse_mapping(parser, &node->mapping_values, &node->mapping_count,
                                  has_anchors, has_duplicate_keys,
                                  dup_key_name, dup_key_line) != 0) {
                    return -1;
                }
                break;
            }
            case YAML_SEQUENCE_START_EVENT: {
                node->type = 1;
                yaml_event_delete(&peek);
                if (parse_sequence(parser, &node->sequence_values, &node->sequence_count,
                                   has_anchors, has_duplicate_keys,
                                   dup_key_name, dup_key_line) != 0) {
                    return -1;
                }
                break;
            }
            default: {
                node->type = 3; /* null */
                yaml_event_delete(&peek);
                break;
            }
        }

        (*node_count)++;
    }

    return 0;
}

/*
 * Free an array of yaml_kv_t.
 */
static void free_kvs(yaml_kv_t *kvs, int count) {
    if (!kvs) return;
    for (int i = 0; i < count; i++) {
        free(kvs[i].key);
        free(kvs[i].scalar_value);
        free_kvs(kvs[i].mapping_values, kvs[i].mapping_count);
        free_nodes(kvs[i].sequence_values, kvs[i].sequence_count);
    }
    free(kvs);
}

/*
 * Free an array of yass_yaml_node_t.
 */
static void free_nodes(yass_yaml_node_t *nodes, int count) {
    if (!nodes) return;
    for (int i = 0; i < count; i++) {
        free(nodes[i].scalar_value);
        free_kvs(nodes[i].mapping_values, nodes[i].mapping_count);
        free_nodes(nodes[i].sequence_values, nodes[i].sequence_count);
    }
    free(nodes);
}

/*
 * Parse YAML from a buffer.
 */
int yaml_parse_buffer(const unsigned char *data, size_t len,
                      yaml_parse_result_t *result,
                      const char **error_code, char **error_message, int *error_line) {
    memset(result, 0, sizeof(yaml_parse_result_t));

    yaml_parser_t parser;
    if (!yaml_parser_initialize(&parser)) {
        *error_code = EC_YAML_MALFORMED;
        *error_message = strdup("failed to initialize YAML parser");
        *error_line = 0;
        return -1;
    }

    yaml_parser_set_input_string(&parser, data, len);

    bool has_anchors = false;
    bool has_duplicate_keys = false;
    char *dup_key_name = NULL;
    int dup_key_line_num = 0;

    int doc_capacity = 0;
    bool in_stream = false;
    bool done = false;
    bool had_documents = false;

    while (!done) {
        yaml_event_t event;
        if (!yaml_parser_parse(&parser, &event)) {
            /* Parse error */
            *error_code = EC_YAML_MALFORMED;
            if (parser.problem) {
                size_t msg_len = strlen(parser.problem) + 64;
                *error_message = malloc(msg_len);
                snprintf(*error_message, msg_len, "%s", parser.problem);
            } else {
                *error_message = strdup("malformed YAML");
            }
            *error_line = (int)parser.problem_mark.line + 1;

            /* Clean up any partially-built result */
            for (int i = 0; i < result->doc_count; i++) {
                free_kvs(result->docs[i].kvs, result->docs[i].kv_count);
            }
            free(result->docs);
            memset(result, 0, sizeof(yaml_parse_result_t));
            free(dup_key_name);

            yaml_parser_delete(&parser);
            return -1;
        }

        check_event_flags(&event, &has_anchors);

        switch (event.type) {
            case YAML_STREAM_START_EVENT:
                in_stream = true;
                break;

            case YAML_STREAM_END_EVENT:
                done = true;
                break;

            case YAML_DOCUMENT_START_EVENT: {
                had_documents = true;
                int doc_line = (int)event.start_mark.line + 1;

                yaml_event_delete(&event);

                /* Peek at next event to determine document content type */
                yaml_event_t next;
                if (!yaml_parser_parse(&parser, &next)) {
                    *error_code = EC_YAML_MALFORMED;
                    *error_message = strdup("malformed YAML");
                    *error_line = (int)parser.problem_mark.line + 1;
                    for (int i = 0; i < result->doc_count; i++) {
                        free_kvs(result->docs[i].kvs, result->docs[i].kv_count);
                    }
                    free(result->docs);
                    memset(result, 0, sizeof(yaml_parse_result_t));
                    free(dup_key_name);
                    yaml_parser_delete(&parser);
                    return -1;
                }

                check_event_flags(&next, &has_anchors);

                /* Grow docs array if needed */
                if (result->doc_count >= doc_capacity) {
                    doc_capacity = doc_capacity == 0 ? 4 : doc_capacity * 2;
                    result->docs = realloc(result->docs, (size_t)doc_capacity * sizeof(yaml_doc_t));
                }

                yaml_doc_t *doc = &result->docs[result->doc_count];
                memset(doc, 0, sizeof(yaml_doc_t));
                doc->start_line = doc_line;

                if (next.type == YAML_MAPPING_START_EVENT) {
                    yaml_event_delete(&next);
                    if (parse_mapping(&parser, &doc->kvs, &doc->kv_count,
                                      &has_anchors, &has_duplicate_keys,
                                      &dup_key_name, &dup_key_line_num) != 0) {
                        *error_code = EC_YAML_MALFORMED;
                        *error_message = strdup("malformed YAML");
                        *error_line = (int)parser.problem_mark.line + 1;
                        free_kvs(doc->kvs, doc->kv_count);
                        for (int i = 0; i < result->doc_count; i++) {
                            free_kvs(result->docs[i].kvs, result->docs[i].kv_count);
                        }
                        free(result->docs);
                        memset(result, 0, sizeof(yaml_parse_result_t));
                        free(dup_key_name);
                        yaml_parser_delete(&parser);
                        return -1;
                    }
                } else if (next.type == YAML_DOCUMENT_END_EVENT) {
                    /* Empty document - no content */
                    yaml_event_delete(&next);
                    result->doc_count++;
                    continue;
                } else {
                    /* Non-mapping document (scalar or sequence at top level).
                     * We still need to consume until DOCUMENT_END.
                     * For now, skip events until we reach DOCUMENT_END. */
                    int depth = 0;
                    if (next.type == YAML_SEQUENCE_START_EVENT ||
                        next.type == YAML_MAPPING_START_EVENT) {
                        depth = 1;
                    }
                    yaml_event_delete(&next);

                    while (depth > 0 || 1) {
                        yaml_event_t skip;
                        if (!yaml_parser_parse(&parser, &skip)) {
                            *error_code = EC_YAML_MALFORMED;
                            *error_message = strdup("malformed YAML");
                            *error_line = 0;
                            for (int i = 0; i < result->doc_count; i++) {
                                free_kvs(result->docs[i].kvs, result->docs[i].kv_count);
                            }
                            free(result->docs);
                            memset(result, 0, sizeof(yaml_parse_result_t));
                            free(dup_key_name);
                            yaml_parser_delete(&parser);
                            return -1;
                        }
                        check_event_flags(&skip, &has_anchors);
                        if (skip.type == YAML_MAPPING_START_EVENT ||
                            skip.type == YAML_SEQUENCE_START_EVENT) {
                            depth++;
                        } else if (skip.type == YAML_MAPPING_END_EVENT ||
                                   skip.type == YAML_SEQUENCE_END_EVENT) {
                            depth--;
                        } else if (skip.type == YAML_DOCUMENT_END_EVENT) {
                            yaml_event_delete(&skip);
                            break;
                        }
                        yaml_event_delete(&skip);
                    }

                    result->doc_count++;
                    continue;
                }

                result->doc_count++;

                /* Consume the DOCUMENT_END event */
                {
                    yaml_event_t doc_end;
                    if (!yaml_parser_parse(&parser, &doc_end)) {
                        /* Not fatal - we already parsed the doc */
                    } else {
                        yaml_event_delete(&doc_end);
                    }
                }
                continue;
            }

            default:
                break;
        }

        yaml_event_delete(&event);
    }

    (void)in_stream;

    yaml_parser_delete(&parser);

    /* Check post-parse conditions */
    if (!had_documents) {
        *error_code = EC_YAML_EMPTY_STREAM;
        *error_message = strdup("YAML stream contains no documents");
        *error_line = 0;
        yaml_parse_result_free(result);
        return -1;
    }

    result->has_anchors = has_anchors;
    result->has_duplicate_keys = has_duplicate_keys;
    result->duplicate_key_name = dup_key_name;
    result->duplicate_key_line = dup_key_line_num;

    if (has_anchors) {
        *error_code = EC_YAML_ANCHOR_OR_ALIAS;
        *error_message = strdup("YAML anchors, aliases, or explicit tags are not allowed");
        *error_line = 0;
        return -1;
    }

    if (has_duplicate_keys) {
        *error_code = EC_YAML_DUPLICATE_KEY;
        size_t msg_len = 128 + (dup_key_name ? strlen(dup_key_name) : 0);
        *error_message = malloc(msg_len);
        snprintf(*error_message, msg_len, "duplicate key: %s",
                 dup_key_name ? dup_key_name : "(unknown)");
        *error_line = dup_key_line_num;
        return -1;
    }

    return 0;
}

/*
 * Parse a YAML file into a multi-document structure.
 */
int yaml_parse_file(const char *path, yaml_parse_result_t *result,
                    const char **error_code, char **error_message, int *error_line) {
    memset(result, 0, sizeof(yaml_parse_result_t));

    FILE *f = fopen(path, "rb");
    if (!f) {
        *error_code = EC_PATH_UNREADABLE;
        size_t msg_len = strlen(path) + 64;
        *error_message = malloc(msg_len);
        snprintf(*error_message, msg_len, "cannot open file: %s", path);
        *error_line = 0;
        return -1;
    }

    /* Get file size */
    fseek(f, 0, SEEK_END);
    long file_size = ftell(f);
    fseek(f, 0, SEEK_SET);

    if (file_size < 0) {
        fclose(f);
        *error_code = EC_PATH_UNREADABLE;
        *error_message = strdup("cannot determine file size");
        *error_line = 0;
        return -1;
    }

    size_t len = (size_t)file_size;

    /* Check empty file */
    if (len == 0) {
        fclose(f);
        *error_code = EC_YAML_EMPTY_FILE;
        *error_message = strdup("file is empty");
        *error_line = 0;
        return -1;
    }

    /* Read file contents */
    unsigned char *data = malloc(len + 1);
    if (!data) {
        fclose(f);
        *error_code = EC_PATH_UNREADABLE;
        *error_message = strdup("out of memory");
        *error_line = 0;
        return -1;
    }

    size_t read_len = fread(data, 1, len, f);
    fclose(f);

    if (read_len != len) {
        free(data);
        *error_code = EC_PATH_UNREADABLE;
        *error_message = strdup("failed to read file");
        *error_line = 0;
        return -1;
    }

    data[len] = '\0';

    /* Check UTF-8 validity */
    if (!utf8_is_valid(data, len)) {
        free(data);
        *error_code = EC_YAML_NOT_UTF8;
        *error_message = strdup("file is not valid UTF-8");
        *error_line = 0;
        return -1;
    }

    /* Check BOM */
    if (utf8_has_bom(data, len)) {
        free(data);
        *error_code = EC_YAML_HAS_BOM;
        *error_message = strdup("file starts with a UTF-8 BOM");
        *error_line = 0;
        return -1;
    }

    /* Parse the buffer */
    int rc = yaml_parse_buffer(data, len, result, error_code, error_message, error_line);
    free(data);
    return rc;
}

/*
 * Free a parse result.
 */
void yaml_parse_result_free(yaml_parse_result_t *result) {
    if (!result) return;
    for (int i = 0; i < result->doc_count; i++) {
        free_kvs(result->docs[i].kvs, result->docs[i].kv_count);
    }
    free(result->docs);
    free(result->duplicate_key_name);
    memset(result, 0, sizeof(yaml_parse_result_t));
}

/*
 * Find a key in a document.
 */
yaml_kv_t *yaml_doc_find_key(yaml_doc_t *doc, const char *key) {
    if (!doc || !key) return NULL;
    for (int i = 0; i < doc->kv_count; i++) {
        if (strcmp(doc->kvs[i].key, key) == 0) {
            return &doc->kvs[i];
        }
    }
    return NULL;
}

/*
 * Get the string value of a key, or NULL if not found or not a scalar.
 */
const char *yaml_doc_get_string(yaml_doc_t *doc, const char *key) {
    yaml_kv_t *kv = yaml_doc_find_key(doc, key);
    if (!kv || kv->value_type != 0) return NULL;
    return kv->scalar_value;
}
