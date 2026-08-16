/*
 * query.c - Query subcommand per cli.query spec
 */
#include "query.h"
#include "emit.h"
#include "yaml_parse.h"
#include "discover.h"
#include "error.h"
#include "path.h"
#include "yass.h"
#include "utf8.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <ctype.h>

/* A matched spec entry */
typedef struct {
    char *file_path;
    int doc_index;
    char *spec_name;
    char *description;
} query_match_t;

/*
 * Check if query matches a spec name.
 * Rules:
 * - Exact match (case-sensitive byte comparison)
 * - Trailing dot-aligned suffix match:
 *   query equals spec_name with zero or more leading dot-separated
 *   components removed.
 *   e.g. "ExtractFragment" matches "cli.query.ExtractFragment"
 *   e.g. "query.ExtractFragment" matches "cli.query.ExtractFragment"
 * - NOT partial substring: "Extract" does NOT match
 *   "cli.query.ExtractFragment"
 */
static bool name_matches(const char *spec_name, const char *query) {
    /* Exact match */
    if (strcmp(spec_name, query) == 0) return true;

    /* Trailing suffix match: spec_name must end with "." + query */
    size_t slen = strlen(spec_name);
    size_t qlen = strlen(query);
    if (qlen == 0 || qlen >= slen) return false;

    /* Check that spec_name[slen - qlen - 1] == '.' */
    size_t prefix_end = slen - qlen - 1;
    if (spec_name[prefix_end] != '.') return false;

    /* Check that the suffix matches */
    if (strcmp(spec_name + prefix_end + 1, query) != 0) return false;

    return true;
}

/*
 * Check if a string contains any whitespace.
 */
static bool has_whitespace(const char *s) {
    for (; *s; s++) {
        if (isspace((unsigned char)*s)) return true;
    }
    return false;
}

/*
 * Get preamble description from a parse result.
 */
static char *get_description(yaml_parse_result_t *result) {
    if (result->doc_count < 1) return strdup("");
    yaml_doc_t *preamble = &result->docs[0];
    const char *desc = yaml_doc_get_string(preamble, "description");
    if (!desc) return strdup("");
    char *norm = utf8_normalize_whitespace(desc);
    return norm;
}

/*
 * Parse a ref target into path, spec, slot components.
 * Grammar: [path@]SpecName[::SLOT]
 * Returns 0 on success, -1 on malformed.
 */
static int parse_ref_target(const char *target, char **out_path,
                            char **out_spec, char **out_slot) {
    *out_path = NULL;
    *out_spec = NULL;
    *out_slot = NULL;

    const char *at = strchr(target, '@');
    const char *dcolon = strstr(target, "::");

    const char *spec_start;
    const char *spec_end;

    if (at) {
        /* path@SpecName[::SLOT] */
        *out_path = strndup(target, (size_t)(at - target));
        spec_start = at + 1;
    } else {
        spec_start = target;
    }

    if (dcolon && dcolon > spec_start) {
        spec_end = dcolon;
        *out_slot = strdup(dcolon + 2);
    } else {
        spec_end = target + strlen(target);
    }

    if (spec_end <= spec_start) {
        free(*out_path);
        free(*out_slot);
        *out_path = NULL;
        *out_slot = NULL;
        return -1;
    }

    *out_spec = strndup(spec_start, (size_t)(spec_end - spec_start));
    return 0;
}

/*
 * Resolve a ref target file path.
 * For ./ or ../ prefixed paths, resolve relative to base_dir.
 * Otherwise resolve relative to project_root.
 */
static char *resolve_ref_file(const char *ref_path, const char *base_dir,
                              const char *project_root) {
    char *file_with_ext;
    size_t plen = strlen(ref_path);

    /* Build path + .yass.yaml */
    file_with_ext = malloc(plen + 11);
    snprintf(file_with_ext, plen + 11, "%s.yass.yaml", ref_path);

    char *resolved;
    if (ref_path[0] == '.' && (ref_path[1] == '/' ||
        (ref_path[1] == '.' && ref_path[2] == '/'))) {
        /* Relative to base_dir */
        resolved = path_join(base_dir, file_with_ext);
    } else {
        /* Relative to project root */
        resolved = path_join(project_root, file_with_ext);
    }
    free(file_with_ext);
    return resolved;
}

/*
 * Find a spec in a parse result by name. Returns the doc or NULL.
 */
static yaml_doc_t *find_spec_in_result(yaml_parse_result_t *result,
                                       const char *spec_name) {
    for (int i = 1; i < result->doc_count; i++) {
        const char *name = yaml_doc_get_string(&result->docs[i], "spec");
        if (name && strcmp(name, spec_name) == 0) {
            return &result->docs[i];
        }
    }
    return NULL;
}

/*
 * Deep-copy a yaml_kv_t array.
 */
static yaml_kv_t *deep_copy_kvs(const yaml_kv_t *src, int count);
static yass_yaml_node_t *deep_copy_nodes(const yass_yaml_node_t *src, int count);

static yaml_kv_t *deep_copy_kvs(const yaml_kv_t *src, int count) {
    if (!src || count <= 0) return NULL;
    yaml_kv_t *dst = calloc((size_t)count, sizeof(yaml_kv_t));
    for (int i = 0; i < count; i++) {
        dst[i].key = src[i].key ? strdup(src[i].key) : NULL;
        dst[i].key_line = src[i].key_line;
        dst[i].value_type = src[i].value_type;
        dst[i].scalar_value = src[i].scalar_value ? strdup(src[i].scalar_value) : NULL;
        dst[i].mapping_values = deep_copy_kvs(src[i].mapping_values, src[i].mapping_count);
        dst[i].mapping_count = src[i].mapping_count;
        dst[i].sequence_values = deep_copy_nodes(src[i].sequence_values, src[i].sequence_count);
        dst[i].sequence_count = src[i].sequence_count;
    }
    return dst;
}

static yass_yaml_node_t *deep_copy_nodes(const yass_yaml_node_t *src, int count) {
    if (!src || count <= 0) return NULL;
    yass_yaml_node_t *dst = calloc((size_t)count, sizeof(yass_yaml_node_t));
    for (int i = 0; i < count; i++) {
        dst[i].type = src[i].type;
        dst[i].line = src[i].line;
        dst[i].scalar_value = src[i].scalar_value ? strdup(src[i].scalar_value) : NULL;
        dst[i].mapping_values = deep_copy_kvs(src[i].mapping_values, src[i].mapping_count);
        dst[i].mapping_count = src[i].mapping_count;
        dst[i].sequence_values = deep_copy_nodes(src[i].sequence_values, src[i].sequence_count);
        dst[i].sequence_count = src[i].sequence_count;
    }
    return dst;
}

/*
 * Free a deep-copied node array.
 */
static void free_copied_nodes(yass_yaml_node_t *nodes, int count);

/*
 * Free a deep-copied kv array.
 */
static void free_copied_kvs(yaml_kv_t *kvs, int count) {
    if (!kvs) return;
    for (int i = 0; i < count; i++) {
        free(kvs[i].key);
        free(kvs[i].scalar_value);
        free_copied_kvs(kvs[i].mapping_values, kvs[i].mapping_count);
        free_copied_nodes(kvs[i].sequence_values, kvs[i].sequence_count);
    }
    free(kvs);
}

static void free_copied_nodes(yass_yaml_node_t *nodes, int count) {
    if (!nodes) return;
    for (int i = 0; i < count; i++) {
        free(nodes[i].scalar_value);
        free_copied_kvs(nodes[i].mapping_values, nodes[i].mapping_count);
        free_copied_nodes(nodes[i].sequence_values, nodes[i].sequence_count);
    }
    free(nodes);
}

/* Provenance comment storage */
typedef struct {
    int slot_index;
    int obligation_index;
    char *comment;
} provenance_t;

typedef struct {
    provenance_t *items;
    int count;
    int capacity;
} provenance_list_t;

static void provenance_init(provenance_list_t *pl) {
    memset(pl, 0, sizeof(*pl));
}

static void provenance_add(provenance_list_t *pl, int slot_idx, int obl_idx,
                           const char *comment) {
    if (pl->count >= pl->capacity) {
        pl->capacity = pl->capacity == 0 ? 8 : pl->capacity * 2;
        pl->items = realloc(pl->items, (size_t)pl->capacity * sizeof(provenance_t));
    }
    pl->items[pl->count].slot_index = slot_idx;
    pl->items[pl->count].obligation_index = obl_idx;
    pl->items[pl->count].comment = strdup(comment);
    pl->count++;
}

static void provenance_free(provenance_list_t *pl) {
    for (int i = 0; i < pl->count; i++) free(pl->items[i].comment);
    free(pl->items);
    memset(pl, 0, sizeof(*pl));
}

/*
 * Emit a spec document as YAML with provenance comments.
 * This is a specialized emitter that handles CONFORMS provenance.
 */
static void emit_scalar_q(FILE *out, const char *value) {
    if (emit_needs_quoting(value)) {
        fputc('"', out);
        for (const char *p = value; *p; p++) {
            switch (*p) {
                case '"': fputs("\\\"", out); break;
                case '\\': fputs("\\\\", out); break;
                case '\n': fputs("\\n", out); break;
                case '\t': fputs("\\t", out); break;
                default: fputc(*p, out); break;
            }
        }
        fputc('"', out);
    } else {
        fputs(value, out);
    }
}

static void emit_query_fragment(FILE *out, yaml_doc_t *doc,
                                provenance_list_t *prov) {
    fprintf(out, "---\n");

    for (int ki = 0; ki < doc->kv_count; ki++) {
        yaml_kv_t *kv = &doc->kvs[ki];

        if (kv->value_type == 0) {
            /* Scalar value */
            fprintf(out, "%s: ", kv->key);
            emit_scalar_q(out, kv->scalar_value);
            fprintf(out, "\n");
        } else if (kv->value_type == 1) {
            /* Sequence (list of obligations) */
            fprintf(out, "%s:\n", kv->key);

            /* Find the slot index for provenance lookup */
            int slot_idx = ki;

            for (int si = 0; si < kv->sequence_count; si++) {
                yass_yaml_node_t *node = &kv->sequence_values[si];

                /* Check for provenance comment */
                for (int pi = 0; pi < prov->count; pi++) {
                    if (prov->items[pi].slot_index == slot_idx &&
                        prov->items[pi].obligation_index == si) {
                        fprintf(out, "%s\n", prov->items[pi].comment);
                    }
                }

                if (node->type == 2 && node->mapping_count > 0) {
                    /* Mapping obligation */
                    for (int mi = 0; mi < node->mapping_count; mi++) {
                        yaml_kv_t *mkv = &node->mapping_values[mi];
                        if (mi == 0) {
                            fprintf(out, "- %s: ", mkv->key);
                        } else {
                            fprintf(out, "  %s: ", mkv->key);
                        }
                        if (mkv->value_type == 0 && mkv->scalar_value) {
                            emit_scalar_q(out, mkv->scalar_value);
                        }
                        fprintf(out, "\n");
                    }
                } else if (node->type == 0 && node->scalar_value) {
                    fprintf(out, "- ");
                    emit_scalar_q(out, node->scalar_value);
                    fprintf(out, "\n");
                }
            }
        }
    }
}

/*
 * Inline CONFORMS refs in a deep-copied doc.
 * file_result is the cached parse result for the file containing this spec.
 * Returns 0 on success, exit code on error.
 */
static int inline_conforms(yaml_doc_t *doc, const char *file_path,
                           yaml_parse_result_t *file_result,
                           provenance_list_t *prov) {
    char *base_dir = path_dirname(file_path);
    char *project_root = find_project_root(base_dir);
    int errors = 0;

    for (int ki = 0; ki < doc->kv_count; ki++) {
        yaml_kv_t *slot_kv = &doc->kvs[ki];
        if (strcmp(slot_kv->key, "spec") == 0) continue;
        if (slot_kv->value_type != 1) continue;

        /* New sequence to build */
        yass_yaml_node_t *new_seq = NULL;
        int new_count = 0;
        int new_cap = 0;

        for (int oi = 0; oi < slot_kv->sequence_count; oi++) {
            yass_yaml_node_t *obl = &slot_kv->sequence_values[oi];
            if (obl->type != 2) {
                /* Non-mapping, keep as-is */
                if (new_count >= new_cap) {
                    new_cap = new_cap == 0 ? 8 : new_cap * 2;
                    new_seq = realloc(new_seq, (size_t)new_cap * sizeof(yass_yaml_node_t));
                }
                new_seq[new_count] = *obl;
                obl->scalar_value = NULL;
                obl->mapping_values = NULL;
                obl->sequence_values = NULL;
                new_count++;
                continue;
            }

            /* Check for CONFORMS ref */
            int conforms_idx = -1;
            char *conforms_target = NULL;
            bool has_normativity = false;
            char *carrier_when = NULL;

            for (int mi = 0; mi < obl->mapping_count; mi++) {
                if (strcmp(obl->mapping_values[mi].key, "CONFORMS") == 0) {
                    conforms_idx = mi;
                    if (obl->mapping_values[mi].value_type == 0)
                        conforms_target = obl->mapping_values[mi].scalar_value;
                }
                if (yass_is_keyword(obl->mapping_values[mi].key, NORMATIVITY_KEYWORDS))
                    has_normativity = true;
                if (strcmp(obl->mapping_values[mi].key, "WHEN") == 0 &&
                    obl->mapping_values[mi].value_type == 0)
                    carrier_when = obl->mapping_values[mi].scalar_value;
            }

            if (conforms_idx < 0 || !conforms_target) {
                /* No CONFORMS ref, keep as-is */
                if (new_count >= new_cap) {
                    new_cap = new_cap == 0 ? 8 : new_cap * 2;
                    new_seq = realloc(new_seq, (size_t)new_cap * sizeof(yass_yaml_node_t));
                }
                new_seq[new_count] = *obl;
                obl->scalar_value = NULL;
                obl->mapping_values = NULL;
                obl->sequence_values = NULL;
                new_count++;
                continue;
            }

            /* Parse the CONFORMS target */
            char *ref_path_str = NULL, *ref_spec = NULL, *ref_slot = NULL;
            if (parse_ref_target(conforms_target, &ref_path_str, &ref_spec, &ref_slot) != 0) {
                error_emit(file_path, 0, EC_QUERY_CONFORMS_UNRESOLVED,
                           conforms_target);
                errors++;
                free(ref_path_str); free(ref_spec); free(ref_slot);
                goto cleanup_seq;
            }

            /* Must have ::SLOT suffix */
            if (!ref_slot || strlen(ref_slot) == 0) {
                char msg[512];
                snprintf(msg, sizeof(msg), "CONFORMS ref must address a slot in v1: %s",
                         conforms_target);
                error_emit(file_path, 0, EC_QUERY_CONFORMS_NO_SLOT, msg);
                errors++;
                free(ref_path_str); free(ref_spec); free(ref_slot);
                goto cleanup_seq;
            }

            /* Resolve the ref */
            yaml_parse_result_t ref_result;
            memset(&ref_result, 0, sizeof(ref_result));
            bool resolved = false;
            bool is_cross_file = (ref_path_str != NULL);
            yaml_doc_t *ref_doc = NULL;

            if (!is_cross_file) {
                /* Same-file ref - use cached file_result */
                ref_doc = find_spec_in_result(file_result, ref_spec);
                if (ref_doc)
                    resolved = true;
            } else {
                /* Cross-file ref */
                char *resolved_path = resolve_ref_file(ref_path_str, base_dir, project_root);
                if (resolved_path) {
                    const char *ec; char *em; int el;
                    if (yaml_parse_file(resolved_path, &ref_result, &ec, &em, &el) == 0) {
                        ref_doc = find_spec_in_result(&ref_result, ref_spec);
                        if (ref_doc) resolved = true;
                    } else {
                        free(em);
                    }
                }
                free(resolved_path);
            }

            if (!resolved || !ref_doc) {
                char msg[512];
                snprintf(msg, sizeof(msg), "unresolvable CONFORMS ref: %s", conforms_target);
                error_emit(file_path, 0, EC_QUERY_CONFORMS_UNRESOLVED, msg);
                errors++;
                free(ref_path_str); free(ref_spec); free(ref_slot);
                if (is_cross_file && ref_result.docs)
                    yaml_parse_result_free(&ref_result);
                goto cleanup_seq;
            }

            /* Find the referenced slot */
            yaml_kv_t *ref_slot_kv = yaml_doc_find_key(ref_doc, ref_slot);
            if (!ref_slot_kv || ref_slot_kv->value_type != 1) {
                char msg[512];
                snprintf(msg, sizeof(msg), "unresolvable CONFORMS ref: %s", conforms_target);
                error_emit(file_path, 0, EC_QUERY_CONFORMS_UNRESOLVED, msg);
                errors++;
                free(ref_path_str); free(ref_spec); free(ref_slot);
                if (is_cross_file)
                    yaml_parse_result_free(&ref_result);
                goto cleanup_seq;
            }

            /* If carrier is normative, keep it (with CONFORMS stripped) and
             * append inlined obligations after it. */
            if (has_normativity) {
                if (new_count >= new_cap) {
                    new_cap = new_cap == 0 ? 8 : new_cap * 2;
                    new_seq = realloc(new_seq, (size_t)new_cap * sizeof(yass_yaml_node_t));
                }
                /* Deep copy the carrier, then strip CONFORMS */
                yass_yaml_node_t *copies = deep_copy_nodes(obl, 1);
                yass_yaml_node_t *kept = &new_seq[new_count];
                *kept = copies[0];
                free(copies);

                /* Strip CONFORMS key from the kept mapping */
                for (int mi = 0; mi < kept->mapping_count; mi++) {
                    if (strcmp(kept->mapping_values[mi].key, "CONFORMS") == 0) {
                        free(kept->mapping_values[mi].key);
                        free(kept->mapping_values[mi].scalar_value);
                        free_copied_kvs(kept->mapping_values[mi].mapping_values,
                                       kept->mapping_values[mi].mapping_count);
                        free_copied_nodes(kept->mapping_values[mi].sequence_values,
                                         kept->mapping_values[mi].sequence_count);
                        if (mi < kept->mapping_count - 1) {
                            memmove(&kept->mapping_values[mi],
                                    &kept->mapping_values[mi + 1],
                                    (size_t)(kept->mapping_count - mi - 1) * sizeof(yaml_kv_t));
                        }
                        kept->mapping_count--;
                        break;
                    }
                }
                new_count++;
            }
            /* If reference-only (no normativity), don't keep the carrier -
             * it gets replaced by the inlined obligations. */

            /* Inline obligations from the referenced slot */
            for (int ri = 0; ri < ref_slot_kv->sequence_count; ri++) {
                yass_yaml_node_t *ref_obl = &ref_slot_kv->sequence_values[ri];
                if (ref_obl->type != 2) continue;

                if (new_count >= new_cap) {
                    new_cap = new_cap == 0 ? 8 : new_cap * 2;
                    new_seq = realloc(new_seq, (size_t)new_cap * sizeof(yass_yaml_node_t));
                }

                /* Deep copy the inlined obligation */
                yass_yaml_node_t *copies = deep_copy_nodes(ref_obl, 1);
                yass_yaml_node_t *inlined = &new_seq[new_count];
                *inlined = copies[0];
                free(copies);

                /* If carrier had a WHEN guard, combine with inlined
                 * obligation's WHEN guard. */
                if (carrier_when) {
                    yaml_kv_t *inlined_when = NULL;
                    for (int m = 0; m < inlined->mapping_count; m++) {
                        if (strcmp(inlined->mapping_values[m].key, "WHEN") == 0) {
                            inlined_when = &inlined->mapping_values[m];
                            break;
                        }
                    }
                    if (inlined_when && inlined_when->value_type == 0 &&
                        inlined_when->scalar_value) {
                        /* Combine: outer + " and " + inner */
                        size_t clen = strlen(carrier_when) + 5 +
                                     strlen(inlined_when->scalar_value) + 1;
                        char *combined = malloc(clen);
                        snprintf(combined, clen, "%s and %s",
                                carrier_when, inlined_when->scalar_value);
                        free(inlined_when->scalar_value);
                        inlined_when->scalar_value = combined;
                    } else {
                        /* Add a WHEN guard to the inlined obligation */
                        inlined->mapping_count++;
                        inlined->mapping_values = realloc(inlined->mapping_values,
                            (size_t)inlined->mapping_count * sizeof(yaml_kv_t));
                        /* Find where WHEN should go (after normativity,
                         * before refs) per obligation key order */
                        int when_pos = 0;
                        for (int m = 0; m < inlined->mapping_count - 1; m++) {
                            if (yass_is_keyword(inlined->mapping_values[m].key,
                                               NORMATIVITY_KEYWORDS)) {
                                when_pos = m + 1;
                                break;
                            }
                        }
                        if (when_pos < inlined->mapping_count - 1) {
                            memmove(&inlined->mapping_values[when_pos + 1],
                                    &inlined->mapping_values[when_pos],
                                    (size_t)(inlined->mapping_count - 1 - when_pos) *
                                    sizeof(yaml_kv_t));
                        }
                        memset(&inlined->mapping_values[when_pos], 0, sizeof(yaml_kv_t));
                        inlined->mapping_values[when_pos].key = strdup("WHEN");
                        inlined->mapping_values[when_pos].value_type = 0;
                        inlined->mapping_values[when_pos].scalar_value = strdup(carrier_when);
                        inlined->mapping_values[when_pos].key_line = 0;
                    }
                }

                /* Strip any CONFORMS keys from the inlined obligation
                 * (we don't recurse into nested CONFORMS) */
                for (int m = 0; m < inlined->mapping_count; m++) {
                    if (strcmp(inlined->mapping_values[m].key, "CONFORMS") == 0) {
                        free(inlined->mapping_values[m].key);
                        free(inlined->mapping_values[m].scalar_value);
                        free_copied_kvs(inlined->mapping_values[m].mapping_values,
                                       inlined->mapping_values[m].mapping_count);
                        free_copied_nodes(inlined->mapping_values[m].sequence_values,
                                         inlined->mapping_values[m].sequence_count);
                        if (m < inlined->mapping_count - 1) {
                            memmove(&inlined->mapping_values[m],
                                    &inlined->mapping_values[m + 1],
                                    (size_t)(inlined->mapping_count - m - 1) *
                                    sizeof(yaml_kv_t));
                        }
                        inlined->mapping_count--;
                        m--;
                    }
                }

                /* Add provenance comment */
                char prov_comment[1024];
                snprintf(prov_comment, sizeof(prov_comment),
                         "# CONFORMS: %s", conforms_target);
                provenance_add(prov, ki, new_count, prov_comment);

                new_count++;
            }

            free(ref_path_str);
            free(ref_spec);
            free(ref_slot);
            if (is_cross_file)
                yaml_parse_result_free(&ref_result);
        }

        /* Replace the slot's sequence with the new one */
        free_copied_nodes(slot_kv->sequence_values, slot_kv->sequence_count);
        slot_kv->sequence_values = new_seq;
        slot_kv->sequence_count = new_count;
        continue;

cleanup_seq:
        /* On error, free what we built and abort */
        free_copied_nodes(new_seq, new_count);
        free(base_dir);
        free(project_root);
        return 1;
    }

    free(base_dir);
    free(project_root);
    return errors > 0 ? 1 : 0;
}

/*
 * Run the query subcommand.
 */
int cmd_query(int argc, char **argv) {
    /* Parse arguments: spec_name [scope] */
    if (argc < 1) {
        error_emit(NULL, 0, EC_QUERY_NAME_MISSING, "missing spec name");
        return 2;
    }

    const char *spec_name = argv[0];
    const char *scope = argc > 1 ? argv[1] : NULL;

    /* Check blank name: empty string or whitespace-only -> name_blank error.
     * Names containing whitespace mixed with non-whitespace characters are
     * treated as a no-match condition, not as a blank error. */
    {
        size_t name_len = strlen(spec_name);
        if (name_len == 0) {
            error_emit(NULL, 0, EC_QUERY_NAME_BLANK,
                       "spec name is blank or contains whitespace");
            return 2;
        }
        bool all_ws = true;
        for (size_t i = 0; i < name_len; i++) {
            if (!isspace((unsigned char)spec_name[i])) {
                all_ws = false;
                break;
            }
        }
        if (all_ws) {
            error_emit(NULL, 0, EC_QUERY_NAME_BLANK,
                       "spec name is blank or contains whitespace");
            return 2;
        }
    }

    /* Check scope for colon */
    if (scope && path_has_colon(scope)) {
        char msg[512];
        snprintf(msg, sizeof(msg), "path contains an unsupported colon character: %s", scope);
        error_emit(NULL, 0, EC_PATH_COLON_IN_PATH, msg);
        return 2;
    }

    /* Validate scope BEFORE name lookup */
    if (scope) {
        struct stat st;
        if (stat(scope, &st) != 0) {
            char msg[512];
            snprintf(msg, sizeof(msg), "scope path does not exist: %s", scope);
            error_emit(NULL, 0, EC_QUERY_SCOPE_NOT_FOUND, msg);
            return 2;
        }
    }

    /* Discover files in scope */
    discover_result_t disc;
    memset(&disc, 0, sizeof(disc));
    int disc_rc;

    if (scope) {
        disc_rc = discover_spec_files(scope, &disc);
    } else {
        disc_rc = discover_spec_files(NULL, &disc);
    }

    if (disc_rc != 0) {
        discover_result_free(&disc);
        return disc_rc;
    }

    if (disc.count == 0) {
        if (scope) {
            char msg[512];
            snprintf(msg, sizeof(msg), "no .yass.yaml files found in scope: %s", scope);
            error_emit(NULL, 0, EC_QUERY_SCOPE_EMPTY, msg);
        }
        discover_result_free(&disc);
        return 2;
    }

    /* Name lookup - find all matching specs.
     * Cache parse results so we don't re-parse for single match. */
    query_match_t *matches = NULL;
    int match_count = 0;
    int match_cap = 0;

    typedef struct {
        char *file_path;
        yaml_parse_result_t result;
        bool valid;
    } parsed_file_t;

    parsed_file_t *parsed_files = calloc(disc.count ? disc.count : 1,
                                         sizeof(parsed_file_t));
    int parsed_count = 0;

    /* If name contains whitespace, it's a no-match (not blank error) */
    bool name_has_ws = has_whitespace(spec_name);

    for (size_t fi = 0; fi < disc.count && !name_has_ws; fi++) {
        yaml_parse_result_t result;
        const char *ec; char *em; int el;

        if (yaml_parse_file(disc.paths[fi], &result, &ec, &em, &el) != 0) {
            free(em);
            continue;
        }

        /* Cache the parse result */
        parsed_files[parsed_count].file_path = strdup(disc.paths[fi]);
        parsed_files[parsed_count].result = result;
        parsed_files[parsed_count].valid = true;

        char *desc = get_description(&result);

        for (int di = 1; di < result.doc_count; di++) {
            const char *sname = yaml_doc_get_string(&result.docs[di], "spec");
            if (!sname) continue;

            if (name_matches(sname, spec_name)) {
                if (match_count >= match_cap) {
                    match_cap = match_cap == 0 ? 4 : match_cap * 2;
                    matches = realloc(matches, (size_t)match_cap * sizeof(query_match_t));
                }
                matches[match_count].file_path = strdup(disc.paths[fi]);
                matches[match_count].doc_index = di;
                matches[match_count].spec_name = strdup(sname);
                matches[match_count].description = strdup(desc);
                match_count++;
            }
        }

        free(desc);
        parsed_count++;
    }

    discover_result_free(&disc);

    /* Dispatch on match count */
    int exit_code;

    if (match_count == 0) {
        char msg[512];
        snprintf(msg, sizeof(msg), "no spec matches: %s", spec_name);
        error_emit(NULL, 0, EC_QUERY_NO_MATCH, msg);
        exit_code = 1;
    } else if (match_count > 1) {
        /* Multi-match: emit disambiguation rows in list format.
         * No truncation regardless of TTY.
         * Files lex-sorted by path, specs in document order. */
        for (int i = 0; i < match_count; i++) {
            char *rel_path = path_relative_to_cwd(matches[i].file_path);
            printf("%s\t%s\t%s\n", rel_path, matches[i].spec_name,
                   matches[i].description);
            free(rel_path);
        }
        exit_code = 0;
    } else {
        /* Single match: extract and emit with CONFORMS inlining */
        char *matched_file = matches[0].file_path;
        int matched_idx = matches[0].doc_index;

        /* Find the cached parse result for this match */
        yaml_parse_result_t *match_result = NULL;
        for (int i = 0; i < parsed_count; i++) {
            if (parsed_files[i].valid &&
                strcmp(parsed_files[i].file_path, matched_file) == 0) {
                match_result = &parsed_files[i].result;
                break;
            }
        }

        if (!match_result) {
            error_emit(NULL, 0, EC_INTERNAL_UNCAUGHT,
                       "internal error: matched file not in parse cache");
            exit_code = 1;
        } else {
            /* Deep-copy the matched doc */
            yaml_doc_t copy;
            copy.start_line = match_result->docs[matched_idx].start_line;
            copy.kv_count = match_result->docs[matched_idx].kv_count;
            copy.kvs = deep_copy_kvs(match_result->docs[matched_idx].kvs,
                                     match_result->docs[matched_idx].kv_count);

            /* Resolve absolute path for CONFORMS resolution */
            char *abs_file = path_absolute(matched_file);

            /* Inline CONFORMS */
            provenance_list_t prov;
            provenance_init(&prov);
            int inline_rc = inline_conforms(&copy,
                                            abs_file ? abs_file : matched_file,
                                            match_result, &prov);

            if (inline_rc != 0) {
                exit_code = 1;
            } else {
                /* Emit the fragment */
                emit_query_fragment(stdout, &copy, &prov);
                exit_code = 0;
            }

            free_copied_kvs(copy.kvs, copy.kv_count);
            provenance_free(&prov);
            free(abs_file);
        }
    }

    /* Cleanup */
    for (int i = 0; i < match_count; i++) {
        free(matches[i].file_path);
        free(matches[i].spec_name);
        free(matches[i].description);
    }
    free(matches);
    for (int i = 0; i < parsed_count; i++) {
        if (parsed_files[i].valid)
            yaml_parse_result_free(&parsed_files[i].result);
        free(parsed_files[i].file_path);
    }
    free(parsed_files);
    return exit_code;
}
