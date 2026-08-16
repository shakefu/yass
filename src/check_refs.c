/*
 * check_refs.c - Reference resolution check per cli.validate.CheckRefs
 */
#include "check_refs.h"
#include "yaml_parse.h"
#include "error.h"
#include "path.h"
#include "yass.h"

#include <string.h>
#include <stdlib.h>
#include <sys/stat.h>

/*
 * Match ref target against the grammar:
 *   ^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$
 *
 * Returns 1 if matches, 0 if not.
 * On match, sets:
 *   *path_out   = malloc'd path portion (or NULL if no @)
 *   *spec_out   = malloc'd spec name
 *   *slot_out   = malloc'd slot name without :: (or NULL if no ::)
 */
static int parse_ref_target(const char *target,
                            char **path_out, char **spec_out, char **slot_out)
{
    *path_out = NULL;
    *spec_out = NULL;
    *slot_out = NULL;

    if (!target || *target == '\0')
        return 0;

    const char *p = target;
    const char *at = NULL;
    const char *dcolon = NULL;

    /* Find @ and :: positions */
    for (const char *s = p; *s; s++) {
        if (*s == '@' && !at)
            at = s;
        if (s[0] == ':' && s[1] == ':' && !dcolon)
            dcolon = s;
    }

    /* Determine the boundaries of path, spec, slot */
    const char *spec_start;
    const char *spec_end;

    if (at) {
        /* Path portion: target[0..at-1] */
        size_t path_len = (size_t)(at - target);
        if (path_len == 0)
            return 0;
        /* Validate path chars: [A-Za-z0-9._/-] */
        for (size_t i = 0; i < path_len; i++) {
            char c = target[i];
            if (!((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
                  (c >= '0' && c <= '9') || c == '.' || c == '_' ||
                  c == '/' || c == '-'))
                return 0;
        }
        *path_out = malloc(path_len + 1);
        memcpy(*path_out, target, path_len);
        (*path_out)[path_len] = '\0';
        spec_start = at + 1;
    } else {
        spec_start = target;
    }

    if (dcolon) {
        /* dcolon must be after spec_start */
        if (dcolon < spec_start)
            goto fail;
        spec_end = dcolon;
        const char *slot_start = dcolon + 2;
        size_t slot_len = strlen(slot_start);
        if (slot_len == 0)
            goto fail;
        /* Validate slot chars: [A-Z-] */
        for (size_t i = 0; i < slot_len; i++) {
            char c = slot_start[i];
            if (!((c >= 'A' && c <= 'Z') || c == '-'))
                goto fail;
        }
        *slot_out = malloc(slot_len + 1);
        memcpy(*slot_out, slot_start, slot_len);
        (*slot_out)[slot_len] = '\0';
    } else {
        spec_end = target + strlen(target);
    }

    /* Spec portion */
    size_t spec_len = (size_t)(spec_end - spec_start);
    if (spec_len == 0)
        goto fail;
    /* Validate spec chars: [A-Za-z0-9._-] */
    for (size_t i = 0; i < spec_len; i++) {
        char c = spec_start[i];
        if (!((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
              (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'))
            goto fail;
    }
    *spec_out = malloc(spec_len + 1);
    memcpy(*spec_out, spec_start, spec_len);
    (*spec_out)[spec_len] = '\0';

    return 1;

fail:
    free(*path_out);
    free(*spec_out);
    free(*slot_out);
    *path_out = NULL;
    *spec_out = NULL;
    *slot_out = NULL;
    return 0;
}

/*
 * Check if a spec name exists in a parse result (searching doc indices >= 1).
 * Returns a pointer to the doc if found, NULL otherwise.
 */
static yaml_doc_t *find_spec_in_result(yaml_parse_result_t *result,
                                       const char *spec_name)
{
    for (int i = 1; i < result->doc_count; i++) {
        const char *name = yaml_doc_get_string(&result->docs[i], "spec");
        if (name && strcmp(name, spec_name) == 0)
            return &result->docs[i];
    }
    return NULL;
}

/*
 * Check if a doc declares a given slot key.
 */
static bool doc_has_slot(yaml_doc_t *doc, const char *slot)
{
    if (!doc || !slot)
        return false;
    return yaml_doc_find_key(doc, slot) != NULL;
}

/*
 * Tracking structure for deduplicated file errors.
 * We emit at most one file_not_found or file_not_parseable
 * per (referencing-file, referenced-file) pair.
 */
typedef struct {
    char **paths;
    int count;
    int capacity;
} file_error_set_t;

static void file_error_set_init(file_error_set_t *set)
{
    set->paths = NULL;
    set->count = 0;
    set->capacity = 0;
}

static void file_error_set_free(file_error_set_t *set)
{
    for (int i = 0; i < set->count; i++)
        free(set->paths[i]);
    free(set->paths);
}

static bool file_error_set_contains(file_error_set_t *set, const char *path)
{
    for (int i = 0; i < set->count; i++) {
        if (strcmp(set->paths[i], path) == 0)
            return true;
    }
    return false;
}

static void file_error_set_add(file_error_set_t *set, const char *path)
{
    if (set->count >= set->capacity) {
        set->capacity = set->capacity == 0 ? 4 : set->capacity * 2;
        set->paths = realloc(set->paths, (size_t)set->capacity * sizeof(char *));
    }
    set->paths[set->count++] = strdup(path);
}

/*
 * Check if a file exists (regular file).
 */
static bool file_exists(const char *path)
{
    struct stat st;
    if (stat(path, &st) != 0)
        return false;
    return S_ISREG(st.st_mode);
}

/*
 * Process a single ref target value.
 * Returns the number of errors emitted.
 */
static int check_one_ref(const char *file, int line, const char *target,
                         yaml_parse_result_t *result,
                         const char *base_dir, const char *project_root,
                         file_error_set_t *file_errors)
{
    int errors = 0;
    char *ref_path = NULL;
    char *ref_spec = NULL;
    char *ref_slot = NULL;

    /* Step 1: Parse the ref target against the grammar */
    if (!parse_ref_target(target, &ref_path, &ref_spec, &ref_slot)) {
        error_emit(file, line, EC_REF_MALFORMED,
                   target);
        return 1;
    }

    /* Step 2: If ::SLOT is present, check it is a valid slot keyword */
    if (ref_slot) {
        if (!yass_is_keyword(ref_slot, SLOT_KEYWORDS)) {
            error_emit(file, line, EC_REF_UNKNOWN_SLOT,
                       target);
            errors++;
            /* Continue to check other aspects */
        }
    }

    /* Step 3: Resolve the ref */
    if (!ref_path) {
        /* Same-file ref */
        yaml_doc_t *found = find_spec_in_result(result, ref_spec);
        if (!found) {
            error_emit(file, line, EC_REF_SPEC_NOT_FOUND_SAME,
                       target);
            errors++;
        } else if (ref_slot && !errors) {
            /* Step 4: Check slot is declared on the found spec */
            if (yass_is_keyword(ref_slot, SLOT_KEYWORDS) &&
                !doc_has_slot(found, ref_slot)) {
                error_emit(file, line, EC_REF_SLOT_NOT_DECLARED,
                           target);
                errors++;
            }
        }
    } else {
        /* Cross-file ref */
        /* Build the resolved file path */
        char *resolved_path = NULL;
        char *file_with_ext = malloc(strlen(ref_path) + strlen(".yass.yaml") + 1);
        sprintf(file_with_ext, "%s.yass.yaml", ref_path);

        if ((ref_path[0] == '.' && ref_path[1] == '/') ||
            (ref_path[0] == '.' && ref_path[1] == '.' && ref_path[2] == '/')) {
            /* Relative to base_dir */
            resolved_path = path_join(base_dir, file_with_ext);
        } else {
            /* Relative to project_root */
            resolved_path = path_join(project_root, file_with_ext);
        }
        free(file_with_ext);

        /* Check dedup: only one file_not_found or file_not_parseable per pair */
        if (file_error_set_contains(file_errors, resolved_path)) {
            /* Already emitted a file-level error for this path, skip */
            free(resolved_path);
            free(ref_path);
            free(ref_spec);
            free(ref_slot);
            return errors;
        }

        /* Check file exists */
        if (!file_exists(resolved_path)) {
            error_emit(file, line, EC_REF_FILE_NOT_FOUND,
                       target);
            errors++;
            file_error_set_add(file_errors, resolved_path);
            free(resolved_path);
            free(ref_path);
            free(ref_spec);
            free(ref_slot);
            return errors;
        }

        /* Try to parse the referenced file */
        yaml_parse_result_t ref_result;
        const char *parse_err_code = NULL;
        char *parse_err_msg = NULL;
        int parse_err_line = 0;
        int parse_rc = yaml_parse_file(resolved_path, &ref_result,
                                       &parse_err_code, &parse_err_msg,
                                       &parse_err_line);
        if (parse_rc != 0) {
            error_emit(file, line, EC_REF_FILE_NOT_PARSEABLE,
                       target);
            errors++;
            file_error_set_add(file_errors, resolved_path);
            free(parse_err_msg);
            free(resolved_path);
            free(ref_path);
            free(ref_spec);
            free(ref_slot);
            return errors;
        }

        /* Find the spec in the parsed file */
        yaml_doc_t *found = find_spec_in_result(&ref_result, ref_spec);
        if (!found) {
            error_emit(file, line, EC_REF_SPEC_NOT_FOUND_OTHER,
                       target);
            errors++;
        } else if (ref_slot) {
            /* Step 4: Check slot is declared on the found spec */
            if (yass_is_keyword(ref_slot, SLOT_KEYWORDS) &&
                !doc_has_slot(found, ref_slot)) {
                error_emit(file, line, EC_REF_SLOT_NOT_DECLARED,
                           target);
                errors++;
            }
        }

        yaml_parse_result_free(&ref_result);
        free(resolved_path);
    }

    free(ref_path);
    free(ref_spec);
    free(ref_slot);
    return errors;
}

/*
 * Check reference targets in all specs of a file.
 * Iterates over spec docs (index >= 1), for each slot, for each obligation,
 * checks CONFORMS, USES, and SEE ref targets.
 */
int check_refs(const char *file, yaml_parse_result_t *result,
               const char *base_dir, const char *project_root)
{
    int errors = 0;
    file_error_set_t file_errors;
    file_error_set_init(&file_errors);

    /* Iterate over all spec documents (index >= 1) */
    for (int di = 1; di < result->doc_count; di++) {
        yaml_doc_t *doc = &result->docs[di];

        /* For each top-level key (potential slot) */
        for (int ki = 0; ki < doc->kv_count; ki++) {
            yaml_kv_t *slot_kv = &doc->kvs[ki];

            /* Skip the "spec" key */
            if (strcmp(slot_kv->key, "spec") == 0)
                continue;

            /* Only process valid slot keys that have sequence values */
            if (!yass_is_keyword(slot_kv->key, SLOT_KEYWORDS))
                continue;
            if (slot_kv->value_type != 1) /* not a sequence */
                continue;

            /* For each obligation in the slot */
            for (int oi = 0; oi < slot_kv->sequence_count; oi++) {
                yass_yaml_node_t *obl = &slot_kv->sequence_values[oi];

                /* Obligations must be mappings */
                if (obl->type != 2)
                    continue;

                /* Check each reference key in the obligation */
                for (int ri = 0; ri < obl->mapping_count; ri++) {
                    yaml_kv_t *ref_kv = &obl->mapping_values[ri];

                    /* Only check reference keys: CONFORMS, USES, SEE */
                    if (!yass_is_keyword(ref_kv->key, REFERENCE_KEYWORDS))
                        continue;

                    /* The value should be a scalar (ref target string) */
                    if (ref_kv->value_type != 0 || !ref_kv->scalar_value)
                        continue;

                    errors += check_one_ref(file, ref_kv->key_line,
                                            ref_kv->scalar_value,
                                            result, base_dir, project_root,
                                            &file_errors);
                }
            }
        }
    }

    file_error_set_free(&file_errors);
    return errors;
}
