/*
 * check_preamble.c - Preamble validation per cli.validate.CheckPreamble
 *
 * Emits at most one error per file in priority order:
 * (1) yass.preamble.has_spec_key
 * (2) yass.yaml.empty_stream
 * (3) yass.preamble.missing
 * (4) yass.preamble.duplicate
 * (5) yass.preamble.misplaced
 * (6) yass.preamble.missing_description
 * (7) yass.preamble.missing_version
 * (8) yass.preamble.unknown_version
 * (9) yass.preamble.bad_related
 */
#include "check_preamble.h"
#include "yaml_parse.h"
#include "error.h"

#include <stdio.h>
#include <string.h>

/*
 * Return true if the document has a "spec" key.
 */
static int doc_has_spec(yaml_doc_t *doc)
{
    return yaml_doc_find_key(doc, "spec") != NULL;
}

/*
 * Check whether a "related" key value is a valid sequence of strings.
 * Returns 0 if valid or absent, 1 if invalid.
 */
static int check_related(yaml_doc_t *doc)
{
    yaml_kv_t *kv = yaml_doc_find_key(doc, "related");
    if (!kv)
        return 0; /* absent is fine */

    /* Must be a sequence */
    if (kv->value_type != 1)
        return 1;

    /* Every item must be a scalar */
    for (int i = 0; i < kv->sequence_count; i++) {
        if (kv->sequence_values[i].type != 0)
            return 1;
    }

    return 0;
}

int check_preamble(const char *file, yaml_parse_result_t *result)
{
    /* (2) empty_stream: zero documents */
    if (result->doc_count == 0) {
        error_emit(file, 0, EC_YAML_EMPTY_STREAM,
                   "YAML stream contains no documents");
        return 1;
    }

    yaml_doc_t *first = &result->docs[0];

    /* (1) has_spec_key: first document contains a "spec" key */
    if (doc_has_spec(first)) {
        error_emit(file, first->start_line, EC_PREAMBLE_HAS_SPEC_KEY,
                   "first document must be a Preamble, not a Spec");
        return 1;
    }

    /*
     * First doc does not have "spec", so it IS the preamble.
     * Now scan remaining docs for additional preambles (docs without "spec").
     */

    /* (3) missing: no preamble at all.
     * Since the first doc does NOT have "spec", a preamble exists.
     * If we reached here, we have at least one preamble (the first doc).
     * So "missing" only fires when ALL docs have "spec" -- but that requires
     * the first doc to have "spec", which we already handled above.
     * Therefore, "missing" cannot fire at this point.
     *
     * However, there is still a path: if the first doc has "spec" we already
     * emitted has_spec_key and returned.  The "missing" case would apply
     * when there is no doc without "spec" at all -- but has_spec_key has
     * higher priority and catches the first doc having "spec".  If doc_count
     * is 1 and doc[0] has spec, has_spec_key fires.  If doc_count > 1 and
     * all docs have spec, has_spec_key fires on first doc.
     * So "missing" is unreachable past the has_spec_key guard.  But per the
     * priority spec it should still logically be checked.  Since we can't
     * reach it, no code is needed.
     */

    /* Count preamble docs (docs without "spec" key) at positions > 0 */
    int extra_preamble_count = 0;
    int first_extra_preamble_pos = -1;
    for (int i = 1; i < result->doc_count; i++) {
        if (!doc_has_spec(&result->docs[i])) {
            extra_preamble_count++;
            if (first_extra_preamble_pos < 0)
                first_extra_preamble_pos = i;
        }
    }

    /* (4) duplicate: more than one preamble */
    if (extra_preamble_count > 0) {
        /* The first doc is a preamble plus at least one more -> duplicate */
        int line = result->docs[first_extra_preamble_pos].start_line;
        error_emit(file, line, EC_PREAMBLE_DUPLICATE,
                   "more than one Preamble in file");
        return 1;
    }

    /* (5) misplaced: a preamble at non-first position.
     * Since the first doc is a preamble and there are no extras,
     * the preamble is correctly at position 0.  Misplaced is unreachable
     * here (same reasoning as "missing" above).
     */

    /*
     * The first doc is the sole preamble at position 0.  Validate its fields.
     */
    yaml_doc_t *preamble = first;

    /* (6) missing_description */
    if (!yaml_doc_find_key(preamble, "description")) {
        error_emit(file, preamble->start_line, EC_PREAMBLE_MISSING_DESC,
                   "Preamble missing description");
        return 1;
    }

    /* (7) missing_version */
    yaml_kv_t *version_kv = yaml_doc_find_key(preamble, "version");
    if (!version_kv) {
        error_emit(file, preamble->start_line, EC_PREAMBLE_MISSING_VERSION,
                   "Preamble missing version");
        return 1;
    }

    /* (8) unknown_version: version is not exactly "v1" */
    const char *version_str = yaml_doc_get_string(preamble, "version");
    if (!version_str || strcmp(version_str, "v1") != 0) {
        /* Build message with the actual version value */
        const char *display = version_str ? version_str : "(null)";
        char msg[256];
        snprintf(msg, sizeof(msg), "unsupported Preamble version: %s", display);
        error_emit(file, preamble->start_line, EC_PREAMBLE_UNKNOWN_VERSION, msg);
        return 1;
    }

    /* (9) bad_related */
    if (check_related(preamble)) {
        error_emit(file, preamble->start_line, EC_PREAMBLE_BAD_RELATED,
                   "Preamble related must be a sequence of strings");
        return 1;
    }

    /* All checks passed */
    return 0;
}
