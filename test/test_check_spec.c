/*
 * test_check_spec.c - Tests for CheckSpec per cli.validate.CheckSpec spec
 */
#include "tinytest.h"
#include "../src/check_spec.h"
#include "../src/yaml_parse.h"
#include "../src/error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* --- helpers --- */

/*
 * Write a string to a temp file and return the path.
 * The caller must unlink the file when done.
 */
static const char *write_temp_file(const char *name, const char *content) {
    static char path[256];
    snprintf(path, sizeof(path), "/tmp/yass_test_check_spec_%s.yass.yaml", name);
    FILE *f = fopen(path, "w");
    if (!f) return NULL;
    fputs(content, f);
    fclose(f);
    return path;
}

/*
 * Parse a temp file and return the parse result.
 * Returns 0 on success, non-zero on parse failure.
 */
static int parse_temp(const char *path, yaml_parse_result_t *result) {
    const char *err_code = NULL;
    char *err_msg = NULL;
    int err_line = 0;
    int rc = yaml_parse_file(path, result, &err_code, &err_msg, &err_line);
    free(err_msg);
    return rc;
}

/* --- valid spec document --- */

TEST(check_spec_valid) {
    const char *path = write_temp_file("valid",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\n"
        "INPUT:\n- MUST: accept something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- missing spec key --- */

TEST(check_spec_no_name) {
    const char *path = write_temp_file("no_name",
        "---\ndescription: test\nversion: v0\n"
        "---\nINPUT:\n- MUST: do something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Missing spec key -> yass.spec.no_name */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- spec name not a string --- */

TEST(check_spec_name_not_string) {
    const char *path = write_temp_file("name_not_string",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec:\n  nested: value\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* spec name is a mapping, not a string -> name_not_string */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- empty spec name --- */

TEST(check_spec_name_empty) {
    const char *path = write_temp_file("name_empty",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: \"\"\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Empty string spec name -> name_empty */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- bad characters in name --- */

TEST(check_spec_name_bad_chars) {
    const char *path = write_temp_file("name_bad_chars",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: \"my spec!\"\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Space and ! are not in [A-Za-z0-9._-] -> name_bad_chars */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- malformed name starting with dot --- */

TEST(check_spec_name_bad_form_leading_dot) {
    const char *path = write_temp_file("name_bad_form_leading",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: .foo.bar\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Starts with dot -> name_bad_form */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- malformed name ending with dot --- */

TEST(check_spec_name_bad_form_trailing_dot) {
    const char *path = write_temp_file("name_bad_form_trailing",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar.\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Ends with dot -> name_bad_form */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- consecutive dots in name --- */

TEST(check_spec_name_bad_form_consecutive_dots) {
    const char *path = write_temp_file("name_bad_form_consec",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo..bar\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Consecutive dots -> name_bad_form */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- reserved keyword name (uppercase) --- */

TEST(check_spec_name_reserved_upper) {
    const char *path = write_temp_file("name_reserved_upper",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: INPUT\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* INPUT is a reserved keyword -> name_reserved */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- reserved keyword name (exact uppercase MUST) --- */

TEST(check_spec_name_reserved_must) {
    const char *path = write_temp_file("name_reserved_must",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: MUST\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* MUST is a reserved keyword -> name_reserved */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- reserved keyword name (case-insensitive: "must") --- */

TEST(check_spec_name_reserved_lower) {
    const char *path = write_temp_file("name_reserved_lower",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: must\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* "must" case-insensitively matches MUST -> name_reserved */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- unknown key in spec document --- */

TEST(check_spec_unknown_key) {
    const char *path = write_temp_file("unknown_key",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nFOOBAR: something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* FOOBAR is not a valid slot keyword -> unknown_key */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- slot value not a list --- */

TEST(check_spec_slot_value_not_list) {
    const char *path = write_temp_file("slot_not_list",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT: not a list\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* INPUT value is scalar, not a list -> value_not_list */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- obligation not a mapping (bad value shape) --- */

TEST(check_spec_obligation_bad_value_shape) {
    const char *path = write_temp_file("bad_value_shape",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- just a string\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Obligation is a scalar string, not a mapping -> bad_value_shape */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- obligation with no normativity or ref --- */

TEST(check_spec_obligation_missing_normativity_or_ref) {
    const char *path = write_temp_file("missing_norm_ref",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- WHEN: something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Only WHEN guard, no normativity or reference -> missing_normativity_or_ref
     * Also triggers guard_without_normativity */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- WHEN guard without normativity --- */

TEST(check_spec_guard_without_normativity) {
    const char *path = write_temp_file("guard_no_norm",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- WHEN: condition\n  CONFORMS: other.spec\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* WHEN guard with CONFORMS ref but no normativity keyword ->
     * guard_without_normativity */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- duplicate normativity keyword --- */

TEST(check_spec_duplicate_normativity) {
    const char *path = write_temp_file("dup_norm",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- MUST: do one thing\n  SHOULD: do another\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Two normativity keywords in one obligation -> duplicate_normativity */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- valid obligation with CONFORMS ref only (reference-only) --- */

TEST(check_spec_conforms_ref_only) {
    const char *path = write_temp_file("conforms_only",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- CONFORMS: other.spec\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* CONFORMS ref without normativity is valid -> 0 errors */
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- unknown normativity keyword --- */

TEST(check_spec_normativity_unknown) {
    const char *path = write_temp_file("norm_unknown",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- SHANT: do something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* SHANT is not a recognized normativity keyword -> normativity_unknown
     * Also triggers missing_normativity_or_ref since no valid norm/ref found */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- first document is skipped (preamble) --- */

TEST(check_spec_skips_preamble) {
    /* Only one document (the preamble) - no spec docs to check */
    const char *path = write_temp_file("preamble_only",
        "---\ndescription: test\nversion: v0\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* No spec documents -> 0 errors */
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- multiple spec documents, one valid one invalid --- */

TEST(check_spec_multiple_docs) {
    const char *path = write_temp_file("multi_docs",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: good.spec\nINPUT:\n- MUST: do something\n"
        "---\nINPUT:\n- MUST: do something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Second spec doc (doc index 2) missing spec key -> at least 1 error */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- valid spec with WHEN guard and normativity --- */

TEST(check_spec_valid_with_guard) {
    const char *path = write_temp_file("valid_guard",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- WHEN: condition\n  MUST: do something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* Valid: WHEN guard with normativity keyword -> 0 errors */
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- duplicate reference (manually constructed) --- */

TEST(check_spec_duplicate_reference) {
    /* YAML parser rejects duplicate keys, so we construct the data manually */
    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));

    result.doc_count = 2;
    result.docs = calloc(2, sizeof(yass_yaml_doc_t));

    /* Doc 0: preamble */
    result.docs[0].start_line = 1;

    /* Doc 1: spec with obligation that has duplicate CONFORMS */
    result.docs[1].start_line = 4;
    result.docs[1].kv_count = 2;
    result.docs[1].kvs = calloc(2, sizeof(yass_yaml_kv_t));

    result.docs[1].kvs[0].key = strdup("spec");
    result.docs[1].kvs[0].key_line = 5;
    result.docs[1].kvs[0].value_type = 0;
    result.docs[1].kvs[0].scalar_value = strdup("my.spec");

    result.docs[1].kvs[1].key = strdup("INPUT");
    result.docs[1].kvs[1].key_line = 6;
    result.docs[1].kvs[1].value_type = 1;  /* sequence */
    result.docs[1].kvs[1].sequence_count = 1;
    result.docs[1].kvs[1].sequence_values = calloc(1, sizeof(yass_yaml_node_t));

    yass_yaml_node_t *obl = &result.docs[1].kvs[1].sequence_values[0];
    obl->type = 2;  /* mapping */
    obl->line = 7;
    obl->mapping_count = 3;
    obl->mapping_values = calloc(3, sizeof(yass_yaml_kv_t));

    obl->mapping_values[0].key = strdup("MUST");
    obl->mapping_values[0].key_line = 7;
    obl->mapping_values[0].value_type = 0;
    obl->mapping_values[0].scalar_value = strdup("do something");

    obl->mapping_values[1].key = strdup("CONFORMS");
    obl->mapping_values[1].key_line = 8;
    obl->mapping_values[1].value_type = 0;
    obl->mapping_values[1].scalar_value = strdup("some.ref");

    obl->mapping_values[2].key = strdup("CONFORMS");
    obl->mapping_values[2].key_line = 9;
    obl->mapping_values[2].value_type = 0;
    obl->mapping_values[2].scalar_value = strdup("other.ref");

    int errors = check_spec("test.yass.yaml", &result);
    ASSERT(errors >= 1);  /* duplicate_reference */

    yaml_parse_result_free(&result);
}

/* --- bad value shape: normativity with mapping value --- */

TEST(check_spec_bad_value_shape_norm_mapping) {
    const char *path = write_temp_file("bvs_norm_map",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- MUST:\n    nested: value\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* MUST value is a mapping -> bad_value_shape */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- bad value shape: normativity with null value (manually constructed) --- */

TEST(check_spec_bad_value_shape_norm_null) {
    /* libyaml treats bare "MUST:" as empty scalar, not null.
     * Construct manually to test the null value_type path. */
    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));

    result.doc_count = 2;
    result.docs = calloc(2, sizeof(yass_yaml_doc_t));

    result.docs[0].start_line = 1;

    result.docs[1].start_line = 4;
    result.docs[1].kv_count = 2;
    result.docs[1].kvs = calloc(2, sizeof(yass_yaml_kv_t));

    result.docs[1].kvs[0].key = strdup("spec");
    result.docs[1].kvs[0].key_line = 5;
    result.docs[1].kvs[0].value_type = 0;
    result.docs[1].kvs[0].scalar_value = strdup("foo.bar");

    result.docs[1].kvs[1].key = strdup("INPUT");
    result.docs[1].kvs[1].key_line = 6;
    result.docs[1].kvs[1].value_type = 1;
    result.docs[1].kvs[1].sequence_count = 1;
    result.docs[1].kvs[1].sequence_values = calloc(1, sizeof(yass_yaml_node_t));

    yass_yaml_node_t *obl = &result.docs[1].kvs[1].sequence_values[0];
    obl->type = 2;  /* mapping */
    obl->line = 7;
    obl->mapping_count = 1;
    obl->mapping_values = calloc(1, sizeof(yass_yaml_kv_t));

    obl->mapping_values[0].key = strdup("MUST");
    obl->mapping_values[0].key_line = 7;
    obl->mapping_values[0].value_type = 3;  /* null */
    obl->mapping_values[0].scalar_value = NULL;

    int errors = check_spec("test.yass.yaml", &result);
    /* MUST value is null -> bad_value_shape */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
}

/* --- bad value shape: WHEN guard with mapping value --- */

TEST(check_spec_bad_value_shape_when_mapping) {
    const char *path = write_temp_file("bvs_when_map",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- WHEN:\n    nested: val\n  MUST: something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* WHEN value is a mapping -> bad_value_shape */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- bad value shape: reference with sequence value --- */

TEST(check_spec_bad_value_shape_ref_sequence) {
    const char *path = write_temp_file("bvs_ref_seq",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- MUST: something\n  CONFORMS:\n    - a\n    - b\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* CONFORMS value is a sequence -> bad_value_shape */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- unknown reference relation (unknown obligation key) --- */

TEST(check_spec_reference_unknown) {
    const char *path = write_temp_file("ref_unknown",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nINPUT:\n- MUST: something\n  EXTENDS: some.ref\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* EXTENDS is not a recognized keyword -> normativity.unknown error */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- reserved keyword name: CONFORMS --- */

TEST(check_spec_name_reserved_conforms) {
    const char *path = write_temp_file("name_reserved_conforms",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: CONFORMS\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* CONFORMS is a reserved keyword -> name_reserved */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- reserved keyword name: when (case-insensitive) --- */

TEST(check_spec_name_reserved_when) {
    const char *path = write_temp_file("name_reserved_when",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: when\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* "when" case-insensitively matches WHEN -> name_reserved */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- all slot keywords accepted --- */

TEST(check_spec_all_slots) {
    const char *path = write_temp_file("all_slots",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\n"
        "INPUT:\n- MUST: input thing\n"
        "RETURN:\n- MUST: return thing\n"
        "ERROR:\n- MUST: error thing\n"
        "SIDE-EFFECT:\n- MUST: side effect\n"
        "INVARIANT:\n- MUST: invariant thing\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- valid spec name with diverse allowed characters --- */

TEST(check_spec_name_valid_chars) {
    const char *path = write_temp_file("name_valid_chars",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: My-Spec_v2.sub.Component\nINPUT:\n- MUST: something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- multiple errors in one document --- */

TEST(check_spec_multiple_errors) {
    const char *path = write_temp_file("multi_errors",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec: foo.bar\nBOGUS: something\nINPUT: not-a-list\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* BOGUS -> unknown_key, INPUT scalar -> value_not_list */
    ASSERT(errors >= 2);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- spec: with null value (next key follows) --- */

TEST(check_spec_name_null_value) {
    const char *path = write_temp_file("name_null",
        "---\ndescription: test\nversion: v0\n"
        "---\nspec:\nINPUT:\n- MUST: something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = parse_temp(path, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_spec(path, &result);
    /* spec key has null value -> name_not_string */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- Suite runner --- */

void run_suite_check_spec(void) {
    /* valid spec */
    RUN_TEST(check_spec_valid);

    /* missing spec key */
    RUN_TEST(check_spec_no_name);

    /* spec name validation */
    RUN_TEST(check_spec_name_not_string);
    RUN_TEST(check_spec_name_empty);
    RUN_TEST(check_spec_name_bad_chars);
    RUN_TEST(check_spec_name_bad_form_leading_dot);
    RUN_TEST(check_spec_name_bad_form_trailing_dot);
    RUN_TEST(check_spec_name_bad_form_consecutive_dots);

    /* reserved keyword names */
    RUN_TEST(check_spec_name_reserved_upper);
    RUN_TEST(check_spec_name_reserved_must);
    RUN_TEST(check_spec_name_reserved_lower);

    /* unknown key in spec */
    RUN_TEST(check_spec_unknown_key);

    /* slot value not a list */
    RUN_TEST(check_spec_slot_value_not_list);

    /* obligation checks */
    RUN_TEST(check_spec_obligation_bad_value_shape);
    RUN_TEST(check_spec_obligation_missing_normativity_or_ref);
    RUN_TEST(check_spec_guard_without_normativity);
    RUN_TEST(check_spec_duplicate_normativity);

    /* valid reference-only obligation */
    RUN_TEST(check_spec_conforms_ref_only);

    /* unknown normativity */
    RUN_TEST(check_spec_normativity_unknown);

    /* preamble skipping */
    RUN_TEST(check_spec_skips_preamble);

    /* multiple documents */
    RUN_TEST(check_spec_multiple_docs);

    /* valid with guard */
    RUN_TEST(check_spec_valid_with_guard);

    /* duplicate reference (manually constructed) */
    RUN_TEST(check_spec_duplicate_reference);

    /* bad value shape variants */
    RUN_TEST(check_spec_bad_value_shape_norm_mapping);
    RUN_TEST(check_spec_bad_value_shape_norm_null);
    RUN_TEST(check_spec_bad_value_shape_when_mapping);
    RUN_TEST(check_spec_bad_value_shape_ref_sequence);

    /* unknown reference relation */
    RUN_TEST(check_spec_reference_unknown);

    /* reserved keywords: CONFORMS, when */
    RUN_TEST(check_spec_name_reserved_conforms);
    RUN_TEST(check_spec_name_reserved_when);

    /* all slot keywords */
    RUN_TEST(check_spec_all_slots);

    /* valid name with diverse chars */
    RUN_TEST(check_spec_name_valid_chars);

    /* multiple errors in one doc */
    RUN_TEST(check_spec_multiple_errors);

    /* null spec value */
    RUN_TEST(check_spec_name_null_value);
}
