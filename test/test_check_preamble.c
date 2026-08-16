/*
 * test_check_preamble.c - Tests for CheckPreamble per cli.validate.CheckPreamble spec
 */
#include "tinytest.h"
#include "../src/check_preamble.h"
#include "../src/error.h"

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

/*
 * Helper: build a yaml_kv_t with a string scalar value.
 */
static yaml_kv_t make_scalar_kv(const char *key, const char *value, int line)
{
    yaml_kv_t kv;
    memset(&kv, 0, sizeof(kv));
    kv.key = strdup(key);
    kv.key_line = line;
    kv.value_type = 0; /* scalar */
    kv.scalar_value = value ? strdup(value) : NULL;
    return kv;
}

/*
 * Helper: build a yaml_kv_t with a sequence of scalar strings.
 */
static yaml_kv_t make_string_seq_kv(const char *key, const char **values,
                                    int count, int line)
{
    yaml_kv_t kv;
    memset(&kv, 0, sizeof(kv));
    kv.key = strdup(key);
    kv.key_line = line;
    kv.value_type = 1; /* sequence */
    kv.sequence_count = count;
    kv.sequence_values = calloc((size_t)count, sizeof(yass_yaml_node_t));
    for (int i = 0; i < count; i++) {
        kv.sequence_values[i].type = 0; /* scalar */
        kv.sequence_values[i].line = line;
        kv.sequence_values[i].scalar_value = strdup(values[i]);
    }
    return kv;
}

/*
 * Helper: build a yaml_kv_t with a sequence containing a non-scalar (mapping).
 */
static yaml_kv_t make_bad_seq_kv(const char *key, int line)
{
    yaml_kv_t kv;
    memset(&kv, 0, sizeof(kv));
    kv.key = strdup(key);
    kv.key_line = line;
    kv.value_type = 1; /* sequence */
    kv.sequence_count = 1;
    kv.sequence_values = calloc(1, sizeof(yass_yaml_node_t));
    kv.sequence_values[0].type = 2; /* mapping - not a scalar string */
    kv.sequence_values[0].line = line;
    return kv;
}

/*
 * Helper: build a yaml_kv_t with a non-sequence value for "related" (scalar).
 */
static yaml_kv_t make_scalar_related_kv(const char *key, const char *value,
                                        int line)
{
    return make_scalar_kv(key, value, line);
}

/*
 * Helper: free a doc's kvs.
 */
static void free_doc_kvs(yaml_doc_t *doc)
{
    for (int i = 0; i < doc->kv_count; i++) {
        free(doc->kvs[i].key);
        free(doc->kvs[i].scalar_value);
        for (int j = 0; j < doc->kvs[i].sequence_count; j++) {
            free(doc->kvs[i].sequence_values[j].scalar_value);
        }
        free(doc->kvs[i].sequence_values);
    }
    free(doc->kvs);
}

/*
 * Helper: free an entire result.
 */
static void free_result(yaml_parse_result_t *result)
{
    for (int i = 0; i < result->doc_count; i++) {
        free_doc_kvs(&result->docs[i]);
    }
    free(result->docs);
}

/* --- empty_stream: zero documents --- */

TEST(cp_empty_stream)
{
    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 0;
    result.docs = NULL;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);
    /* error_emit writes to stderr; we verify return code */
}

/* --- has_spec_key: first doc has "spec" --- */

TEST(cp_has_spec_key)
{
    yaml_kv_t kvs[1];
    kvs[0] = make_scalar_kv("spec", "cli.foo", 2);

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 1;
    doc.start_line = 2;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = &doc;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    /* Clean up (don't free docs pointer since it's stack-allocated) */
    free(kvs[0].key);
    free(kvs[0].scalar_value);
}

/* --- valid preamble: description + version v1 --- */

TEST(cp_valid_preamble)
{
    yaml_kv_t *kvs = calloc(2, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v1", 2);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 0);

    free_result(&result);
}

/* --- valid preamble with related sequence of strings --- */

TEST(cp_valid_preamble_with_related)
{
    const char *related[] = {"./foo@bar", "./baz@qux"};

    yaml_kv_t *kvs = calloc(3, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v1", 2);
    kvs[2] = make_string_seq_kv("related", related, 2, 3);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 3;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 0);

    free_result(&result);
}

/* --- valid preamble + spec doc --- */

TEST(cp_valid_preamble_with_spec)
{
    yaml_kv_t *preamble_kvs = calloc(2, sizeof(yaml_kv_t));
    preamble_kvs[0] = make_scalar_kv("description", "A test spec", 1);
    preamble_kvs[1] = make_scalar_kv("version", "v1", 2);

    yaml_kv_t *spec_kvs = calloc(1, sizeof(yaml_kv_t));
    spec_kvs[0] = make_scalar_kv("spec", "cli.foo", 4);

    yaml_doc_t *docs = calloc(2, sizeof(yaml_doc_t));
    docs[0].kvs = preamble_kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;
    docs[1].kvs = spec_kvs;
    docs[1].kv_count = 1;
    docs[1].start_line = 3;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 2;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 0);

    free_result(&result);
}

/* --- missing_description --- */

TEST(cp_missing_description)
{
    yaml_kv_t *kvs = calloc(1, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("version", "v1", 1);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 1;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- missing_version --- */

TEST(cp_missing_version)
{
    yaml_kv_t *kvs = calloc(1, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 1;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- unknown_version: version is not "v1" --- */

TEST(cp_unknown_version)
{
    yaml_kv_t *kvs = calloc(2, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v2", 2);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- unknown_version: version is empty string --- */

TEST(cp_unknown_version_empty)
{
    yaml_kv_t *kvs = calloc(2, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "", 2);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- bad_related: related is a scalar, not a sequence --- */

TEST(cp_bad_related_scalar)
{
    yaml_kv_t *kvs = calloc(3, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v1", 2);
    kvs[2] = make_scalar_related_kv("related", "not-a-sequence", 3);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 3;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- bad_related: sequence contains a non-string (mapping) --- */

TEST(cp_bad_related_non_string_item)
{
    yaml_kv_t *kvs = calloc(3, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v1", 2);
    kvs[2] = make_bad_seq_kv("related", 3);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 3;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- duplicate: two preambles (first + later doc without "spec") --- */

TEST(cp_duplicate_preamble)
{
    yaml_kv_t *preamble_kvs = calloc(2, sizeof(yaml_kv_t));
    preamble_kvs[0] = make_scalar_kv("description", "A test spec", 1);
    preamble_kvs[1] = make_scalar_kv("version", "v1", 2);

    yaml_kv_t *dup_kvs = calloc(2, sizeof(yaml_kv_t));
    dup_kvs[0] = make_scalar_kv("description", "Another preamble", 5);
    dup_kvs[1] = make_scalar_kv("version", "v1", 6);

    yaml_doc_t *docs = calloc(2, sizeof(yaml_doc_t));
    docs[0].kvs = preamble_kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;
    docs[1].kvs = dup_kvs;
    docs[1].kv_count = 2;
    docs[1].start_line = 4;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 2;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- duplicate: three docs, first + third are preambles, second is spec --- */

TEST(cp_duplicate_preamble_with_spec_between)
{
    yaml_kv_t *preamble_kvs = calloc(2, sizeof(yaml_kv_t));
    preamble_kvs[0] = make_scalar_kv("description", "A test spec", 1);
    preamble_kvs[1] = make_scalar_kv("version", "v1", 2);

    yaml_kv_t *spec_kvs = calloc(1, sizeof(yaml_kv_t));
    spec_kvs[0] = make_scalar_kv("spec", "cli.foo", 5);

    yaml_kv_t *dup_kvs = calloc(2, sizeof(yaml_kv_t));
    dup_kvs[0] = make_scalar_kv("description", "Another preamble", 8);
    dup_kvs[1] = make_scalar_kv("version", "v1", 9);

    yaml_doc_t *docs = calloc(3, sizeof(yaml_doc_t));
    docs[0].kvs = preamble_kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;
    docs[1].kvs = spec_kvs;
    docs[1].kv_count = 1;
    docs[1].start_line = 4;
    docs[2].kvs = dup_kvs;
    docs[2].kv_count = 2;
    docs[2].start_line = 7;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 3;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- priority: missing_description before missing_version --- */

TEST(cp_priority_desc_before_version)
{
    /* Preamble has neither description nor version */
    yaml_kv_t *kvs = calloc(1, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("unrelated_key", "value", 1);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 1;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    /* Should fail with missing_description (priority 6) before
     * missing_version (priority 7) */
    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- priority: duplicate before field checks --- */

TEST(cp_priority_duplicate_before_fields)
{
    /* First doc is preamble missing description; second doc is also preamble.
     * Duplicate (4) has higher priority than missing_description (6). */
    yaml_kv_t *kvs1 = calloc(1, sizeof(yaml_kv_t));
    kvs1[0] = make_scalar_kv("version", "v1", 1);

    yaml_kv_t *kvs2 = calloc(1, sizeof(yaml_kv_t));
    kvs2[0] = make_scalar_kv("version", "v1", 4);

    yaml_doc_t *docs = calloc(2, sizeof(yaml_doc_t));
    docs[0].kvs = kvs1;
    docs[0].kv_count = 1;
    docs[0].start_line = 1;
    docs[1].kvs = kvs2;
    docs[1].kv_count = 1;
    docs[1].start_line = 3;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 2;
    result.docs = docs;

    /* Should fail with duplicate (4), not missing_description (6) */
    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- version key present but not a scalar (e.g., null) --- */

TEST(cp_version_not_scalar)
{
    yaml_kv_t *kvs = calloc(2, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    /* version exists but is null type (value_type=3) */
    kvs[1].key = strdup("version");
    kvs[1].key_line = 2;
    kvs[1].value_type = 3; /* null */
    kvs[1].scalar_value = NULL;

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    /* version key present but yaml_doc_get_string returns NULL (not scalar),
     * so unknown_version fires with display "(null)" */
    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free(kvs[0].key);
    free(kvs[0].scalar_value);
    free(kvs[1].key);
    free(docs);
}

/* --- has_spec_key takes priority over all else --- */

TEST(cp_has_spec_key_priority)
{
    /* First doc has both "spec" and "description" -- has_spec_key fires */
    yaml_kv_t *kvs = calloc(2, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("spec", "cli.foo", 2);
    kvs[1] = make_scalar_kv("description", "A test spec", 3);

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 2;
    docs[0].start_line = 2;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 1);

    free_result(&result);
}

/* --- empty related sequence is valid --- */

TEST(cp_valid_related_empty_seq)
{
    yaml_kv_t *kvs = calloc(3, sizeof(yaml_kv_t));
    kvs[0] = make_scalar_kv("description", "A test spec", 1);
    kvs[1] = make_scalar_kv("version", "v1", 2);
    /* Empty sequence for related */
    kvs[2].key = strdup("related");
    kvs[2].key_line = 3;
    kvs[2].value_type = 1; /* sequence */
    kvs[2].sequence_count = 0;
    kvs[2].sequence_values = NULL;

    yaml_doc_t *docs = calloc(1, sizeof(yaml_doc_t));
    docs[0].kvs = kvs;
    docs[0].kv_count = 3;
    docs[0].start_line = 1;

    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    result.doc_count = 1;
    result.docs = docs;

    int rc = check_preamble("test.yass.yaml", &result);
    ASSERT_EQ(rc, 0);

    free(kvs[0].key);
    free(kvs[0].scalar_value);
    free(kvs[1].key);
    free(kvs[1].scalar_value);
    free(kvs[2].key);
    free(docs);
}

/* --- Suite runner --- */

void run_suite_check_preamble(void)
{
    RUN_TEST(cp_empty_stream);
    RUN_TEST(cp_has_spec_key);
    RUN_TEST(cp_valid_preamble);
    RUN_TEST(cp_valid_preamble_with_related);
    RUN_TEST(cp_valid_preamble_with_spec);
    RUN_TEST(cp_missing_description);
    RUN_TEST(cp_missing_version);
    RUN_TEST(cp_unknown_version);
    RUN_TEST(cp_unknown_version_empty);
    RUN_TEST(cp_bad_related_scalar);
    RUN_TEST(cp_bad_related_non_string_item);
    RUN_TEST(cp_duplicate_preamble);
    RUN_TEST(cp_duplicate_preamble_with_spec_between);
    RUN_TEST(cp_priority_desc_before_version);
    RUN_TEST(cp_priority_duplicate_before_fields);
    RUN_TEST(cp_version_not_scalar);
    RUN_TEST(cp_has_spec_key_priority);
    RUN_TEST(cp_valid_related_empty_seq);
}
