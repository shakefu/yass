/*
 * check_spec.c - Spec document validation per cli.validate.CheckSpec
 *
 * Validates each non-first document (spec documents) in a parsed YAML result.
 * Emits one error per failing rule per obligation.
 */
#include "check_spec.h"
#include "yaml_parse.h"
#include "yass.h"
#include "error.h"

#include <string.h>
#include <ctype.h>

/*
 * Check if a character is in the allowed set for spec names: [A-Za-z0-9._-]
 */
static bool is_name_char(char c)
{
    if (c >= 'A' && c <= 'Z') return true;
    if (c >= 'a' && c <= 'z') return true;
    if (c >= '0' && c <= '9') return true;
    if (c == '.' || c == '_' || c == '-') return true;
    return false;
}

/*
 * Check if a spec name contains only allowed characters.
 * Returns true if all characters are in [A-Za-z0-9._-].
 */
static bool name_has_valid_chars(const char *name)
{
    for (const char *p = name; *p; p++) {
        if (!is_name_char(*p)) return false;
    }
    return true;
}

/*
 * Check if a spec name matches the composition regex:
 *   ^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$
 *
 * This means:
 *   - Must not begin or end with '.'
 *   - Must not contain consecutive '.' characters
 *   - Each segment between dots must be non-empty and contain only [A-Za-z0-9_-]
 */
static bool name_has_valid_form(const char *name)
{
    size_t len = strlen(name);
    if (len == 0) return false;

    /* Must not begin or end with '.' */
    if (name[0] == '.' || name[len - 1] == '.') return false;

    /* Check segments */
    const char *p = name;
    while (*p) {
        if (*p == '.') {
            p++;
            /* Consecutive dots or dot at end (already checked) */
            if (*p == '.' || *p == '\0') return false;
        } else {
            /* Must be [A-Za-z0-9_-] */
            char c = *p;
            if (!((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
                  (c >= '0' && c <= '9') || c == '_' || c == '-')) {
                return false;
            }
            p++;
        }
    }
    return true;
}

/*
 * Check if a key is a recognized obligation-level keyword.
 * Returns: 'n' for normativity, 'r' for reference, 'g' for guard, 'u' for unknown.
 */
static char classify_obligation_key(const char *key)
{
    /* Check normativity keywords (case-sensitive exact match first) */
    if (yass_is_keyword(key, NORMATIVITY_KEYWORDS))
        return 'n';

    /* Check reference keywords */
    if (yass_is_keyword(key, REFERENCE_KEYWORDS))
        return 'r';

    /* Check guard keyword */
    if (strcmp(key, GUARD_KEYWORD) == 0)
        return 'g';

    return 'u';
}

/*
 * Check if a key looks like a normativity keyword (for unknown normativity check).
 * A key is considered a "normativity-like" key if it is all uppercase, possibly
 * with hyphens. We consider it normativity-like if it's not a slot, not a
 * reference keyword, not the guard keyword, and not a recognized normativity.
 *
 * Actually per the spec: we only flag yass.normativity.unknown for unknown
 * obligation keys that look like normativity. But the spec says to check
 * "when a Normativity keyword is outside the recognized set" - this means
 * the key must be uppercased like a normativity keyword to be flagged as
 * normativity.unknown rather than just unknown.
 *
 * Looking more carefully: the spec says we need to classify every key in an
 * obligation. Valid keys are: normativity keywords, reference keywords, WHEN.
 * If a key is none of those, we need to determine if it looks like it was
 * intended to be a normativity keyword (emit normativity.unknown) or a
 * reference relation (emit reference.unknown_relation).
 *
 * Per the spec structure: normativity keywords are all-caps with possible
 * hyphens. Reference keywords are also all-caps. The distinction in practice
 * is: there is no generic "unknown key in obligation" error. Instead:
 * - yass.normativity.unknown = a key that looks like a normativity keyword
 *   but isn't recognized
 * - yass.reference.unknown_relation = a key that looks like a reference
 *   relation but isn't recognized
 *
 * Since we can't know the user's intent, we should probably look at what
 * kind of keyword it resembles. However, looking at the error codes again:
 *
 * The spec says:
 *   - "yass.normativity.unknown when a Normativity keyword is outside the
 *     recognized set"
 *   - "yass.reference.unknown_relation when a Reference relation key is
 *     outside the recognized set"
 *
 * This implies the key was already classified as one or the other. Since
 * we don't have that context, we treat any unrecognized key in an obligation
 * as an unknown normativity (since normativity keywords are the primary
 * "verb" of an obligation).
 *
 * Actually, re-reading the spec more carefully: the classify step is done
 * by us. A key in an obligation that isn't WHEN, isn't a recognized
 * normativity keyword, and isn't a recognized reference keyword is unknown.
 * We don't have a generic "unknown key in obligation" error code. The two
 * possible codes are normativity.unknown and reference.unknown_relation.
 *
 * Since there's no way to distinguish intent, and the spec lists these as
 * separate checks, the most likely interpretation is:
 * - If a key is all-uppercase (like normativity/reference keywords are),
 *   check if it could be normativity or reference. If neither, pick one.
 * - The key should be flagged as normativity.unknown since normativity is
 *   the default classification for unknown obligation keys.
 *
 * For simplicity: any unknown key is emitted as normativity.unknown.
 */

/*
 * Validate a single obligation (a list item under a slot).
 * The obligation must be a YAML mapping. Returns number of errors emitted.
 */
static int check_obligation(const char *file, yass_yaml_node_t *node)
{
    int errors = 0;

    /* Obligation must be a mapping */
    if (node->type != 2) {
        error_emit(file, node->line, EC_OBLIGATION_BAD_VALUE,
                   "obligation is not a mapping");
        return 1;
    }

    int norm_count = 0;
    int ref_count = 0;
    int guard_count = 0;

    /* Track seen reference keywords for duplicate detection */
    const char *seen_refs[16];
    int seen_ref_count = 0;

    for (int i = 0; i < node->mapping_count; i++) {
        yaml_kv_t *kv = &node->mapping_values[i];
        char kind = classify_obligation_key(kv->key);

        switch (kind) {
        case 'n':
            norm_count++;
            /* Check value is scalar (not mapping, sequence, or null) */
            if (kv->value_type != 0) {
                error_emit(file, kv->key_line, EC_OBLIGATION_BAD_VALUE,
                           "normativity value must be a scalar string");
                errors++;
            }
            break;

        case 'r':
            ref_count++;
            /* Check for duplicate reference of same relation */
            for (int j = 0; j < seen_ref_count; j++) {
                if (strcmp(seen_refs[j], kv->key) == 0) {
                    error_emit(file, kv->key_line, EC_OBLIGATION_DUP_REF,
                               "duplicate reference relation");
                    errors++;
                    goto ref_recorded;
                }
            }
            if (seen_ref_count < 16) {
                seen_refs[seen_ref_count++] = kv->key;
            }
ref_recorded:
            /* Check value is scalar */
            if (kv->value_type != 0) {
                error_emit(file, kv->key_line, EC_OBLIGATION_BAD_VALUE,
                           "reference value must be a scalar string");
                errors++;
            }
            break;

        case 'g':
            guard_count++;
            /* Check value is scalar */
            if (kv->value_type != 0) {
                error_emit(file, kv->key_line, EC_OBLIGATION_BAD_VALUE,
                           "WHEN guard value must be a scalar string");
                errors++;
            }
            break;

        case 'u':
            /* Unknown key - emit as unknown normativity */
            error_emit(file, kv->key_line, EC_NORMATIVITY_UNKNOWN,
                       "unknown normativity keyword");
            errors++;
            break;
        }
    }

    /* Check: no normativity AND no reference -> missing */
    if (norm_count == 0 && ref_count == 0) {
        error_emit(file, node->line, EC_OBLIGATION_MISSING_NORM,
                   "obligation has neither a normativity keyword nor a reference");
        errors++;
    }

    /* Check: guard without normativity */
    if (guard_count > 0 && norm_count == 0) {
        error_emit(file, node->line, EC_OBLIGATION_GUARD_NO_NORM,
                   "WHEN guard present without a normativity keyword");
        errors++;
    }

    /* Check: duplicate normativity */
    if (norm_count > 1) {
        error_emit(file, node->line, EC_OBLIGATION_DUP_NORM,
                   "obligation has more than one normativity keyword");
        errors++;
    }

    return errors;
}

/*
 * Validate a single spec document (non-first document).
 * Returns number of errors emitted.
 */
static int check_spec_doc(const char *file, yaml_doc_t *doc)
{
    int errors = 0;

    /* 1. Check for "spec" key */
    yaml_kv_t *spec_kv = yaml_doc_find_key(doc, "spec");
    if (!spec_kv) {
        error_emit(file, doc->start_line, EC_SPEC_NO_NAME,
                   "spec document missing 'spec' key");
        errors++;
        /* Continue checking other keys even without spec key */
    }

    /* 2. If spec key exists, validate the name */
    if (spec_kv) {
        if (spec_kv->value_type != 0) {
            /* Not a scalar string */
            error_emit(file, spec_kv->key_line, EC_SPEC_NAME_NOT_STRING,
                       "spec name is not a string");
            errors++;
        } else if (spec_kv->scalar_value == NULL ||
                   strlen(spec_kv->scalar_value) == 0) {
            /* Empty string */
            error_emit(file, spec_kv->key_line, EC_SPEC_NAME_EMPTY,
                       "spec name is empty");
            errors++;
        } else {
            const char *name = spec_kv->scalar_value;

            /* Check for bad characters */
            if (!name_has_valid_chars(name)) {
                error_emit(file, spec_kv->key_line, EC_SPEC_NAME_BAD_CHARS,
                           "spec name contains characters outside [A-Za-z0-9._-]");
                errors++;
            }
            /* Check form (regex pattern) */
            else if (!name_has_valid_form(name)) {
                error_emit(file, spec_kv->key_line, EC_SPEC_NAME_BAD_FORM,
                           "spec name does not match composition pattern");
                errors++;
            }

            /* Check reserved keywords (case-insensitive) */
            if (yass_is_keyword_ci(name, RESERVED_KEYWORDS)) {
                error_emit(file, spec_kv->key_line, EC_SPEC_NAME_RESERVED,
                           "spec name is a reserved keyword");
                errors++;
            }
        }
    }

    /* 3. Check other keys */
    for (int i = 0; i < doc->kv_count; i++) {
        yaml_kv_t *kv = &doc->kvs[i];

        /* Skip the "spec" key itself */
        if (strcmp(kv->key, "spec") == 0) continue;

        /* Check if it's a valid slot key */
        if (!yass_is_keyword(kv->key, SLOT_KEYWORDS)) {
            error_emit(file, kv->key_line, EC_SPEC_UNKNOWN_KEY,
                       "unknown key in spec document");
            errors++;
            continue;
        }

        /* Valid slot key - check that value is a list (sequence) */
        if (kv->value_type != 1) {
            error_emit(file, kv->key_line, EC_SLOT_VALUE_NOT_LIST,
                       "slot value is not a list");
            errors++;
            continue;
        }

        /* 4. Check each obligation in the slot */
        for (int j = 0; j < kv->sequence_count; j++) {
            errors += check_obligation(file, &kv->sequence_values[j]);
        }
    }

    return errors;
}

/*
 * Check spec document structure for all non-first documents.
 * Returns the number of errors found.
 */
int check_spec(const char *file, yaml_parse_result_t *result)
{
    int errors = 0;

    /* Process each document at index >= 1 (skip the preamble at index 0) */
    for (int i = 1; i < result->doc_count; i++) {
        errors += check_spec_doc(file, &result->docs[i]);
    }

    return errors;
}
