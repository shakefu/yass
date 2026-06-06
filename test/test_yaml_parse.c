/*
 * test_yaml_parse.c - Tests for YAML parsing layer
 */
#include "tinytest.h"
#include "yaml_parse.h"
#include "error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* Helper: write content to a temporary file and return the path.
 * The caller must unlink the file when done. */
static char *write_tmp(const char *suffix, const void *data, size_t len) {
    static int counter = 0;
    char *path = malloc(256);
    snprintf(path, 256, "/tmp/yass_test_%d_%d%s", (int)getpid(), counter++, suffix);
    FILE *f = fopen(path, "wb");
    if (!f) { free(path); return NULL; }
    fwrite(data, 1, len, f);
    fclose(f);
    return path;
}

static char *write_tmp_str(const char *suffix, const char *content) {
    return write_tmp(suffix, content, strlen(content));
}

/* --- Parse simple single-document YAML --- */

TEST(yp_simple_single_doc) {
    const char *yaml =
        "name: hello\n"
        "version: \"1.0\"\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);
    ASSERT_EQ(result.docs[0].kv_count, 2);
    ASSERT_STR_EQ(result.docs[0].kvs[0].key, "name");
    ASSERT_EQ(result.docs[0].kvs[0].value_type, 0);
    ASSERT_STR_EQ(result.docs[0].kvs[0].scalar_value, "hello");
    ASSERT_STR_EQ(result.docs[0].kvs[1].key, "version");
    ASSERT_STR_EQ(result.docs[0].kvs[1].scalar_value, "1.0");

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Parse multi-document YAML --- */

TEST(yp_multi_doc) {
    const char *yaml =
        "---\n"
        "name: first\n"
        "---\n"
        "name: second\n"
        "value: two\n"
        "...\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 2);
    ASSERT_STR_EQ(result.docs[0].kvs[0].key, "name");
    ASSERT_STR_EQ(result.docs[0].kvs[0].scalar_value, "first");
    ASSERT_EQ(result.docs[1].kv_count, 2);
    ASSERT_STR_EQ(result.docs[1].kvs[0].key, "name");
    ASSERT_STR_EQ(result.docs[1].kvs[0].scalar_value, "second");

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Parse file with sequences --- */

TEST(yp_sequence) {
    const char *yaml =
        "items:\n"
        "  - alpha\n"
        "  - beta\n"
        "  - gamma\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);
    ASSERT_EQ(result.docs[0].kv_count, 1);

    yaml_kv_t *kv = &result.docs[0].kvs[0];
    ASSERT_STR_EQ(kv->key, "items");
    ASSERT_EQ(kv->value_type, 1); /* sequence */
    ASSERT_EQ(kv->sequence_count, 3);
    ASSERT_EQ(kv->sequence_values[0].type, 0); /* scalar */
    ASSERT_STR_EQ(kv->sequence_values[0].scalar_value, "alpha");
    ASSERT_STR_EQ(kv->sequence_values[1].scalar_value, "beta");
    ASSERT_STR_EQ(kv->sequence_values[2].scalar_value, "gamma");

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Parse nested mappings --- */

TEST(yp_nested_mapping) {
    const char *yaml =
        "outer:\n"
        "  inner_key: inner_val\n"
        "  nested:\n"
        "    deep: value\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);
    ASSERT_EQ(result.docs[0].kv_count, 1);

    yaml_kv_t *outer = &result.docs[0].kvs[0];
    ASSERT_STR_EQ(outer->key, "outer");
    ASSERT_EQ(outer->value_type, 2); /* mapping */
    ASSERT_EQ(outer->mapping_count, 2);
    ASSERT_STR_EQ(outer->mapping_values[0].key, "inner_key");
    ASSERT_STR_EQ(outer->mapping_values[0].scalar_value, "inner_val");

    yaml_kv_t *nested = &outer->mapping_values[1];
    ASSERT_STR_EQ(nested->key, "nested");
    ASSERT_EQ(nested->value_type, 2);
    ASSERT_EQ(nested->mapping_count, 1);
    ASSERT_STR_EQ(nested->mapping_values[0].key, "deep");
    ASSERT_STR_EQ(nested->mapping_values[0].scalar_value, "value");

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Detect non-UTF-8 content --- */

TEST(yp_not_utf8) {
    /* Invalid UTF-8 byte sequence */
    unsigned char bad[] = { 0xFF, 0xFE, 0x00, 0x6E, 0x61, 0x6D, 0x65 };
    char *path = write_tmp(".yaml", bad, sizeof(bad));
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_NOT_UTF8);
    free(em);
}

/* --- Detect BOM --- */

TEST(yp_has_bom) {
    /* UTF-8 BOM followed by valid YAML */
    unsigned char bom_yaml[] = {
        0xEF, 0xBB, 0xBF,
        'n', 'a', 'm', 'e', ':', ' ', 'x', '\n'
    };
    char *path = write_tmp(".yaml", bom_yaml, sizeof(bom_yaml));
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_HAS_BOM);
    free(em);
}

/* --- Detect empty file --- */

TEST(yp_empty_file) {
    char *path = write_tmp_str(".yaml", "");
    /* write_tmp_str writes 0 bytes for empty string, but write_tmp
     * uses strlen which is 0 - we need an actual empty file */
    ASSERT_NOT_NULL(path);

    /* Rewrite as truly empty */
    FILE *f = fopen(path, "wb");
    fclose(f);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_EMPTY_FILE);
    free(em);
}

/* --- Detect malformed YAML --- */

TEST(yp_malformed) {
    const char *yaml =
        "name: [\n"
        "  broken:\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_MALFORMED);
    ASSERT_NOT_NULL(em);
    free(em);
}

/* --- Detect duplicate keys --- */

TEST(yp_duplicate_keys) {
    const char *yaml =
        "name: first\n"
        "value: something\n"
        "name: second\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_DUPLICATE_KEY);
    ASSERT_TRUE(result.has_duplicate_keys);
    ASSERT_STR_EQ(result.duplicate_key_name, "name");
    ASSERT_EQ(result.duplicate_key_line, 3);

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Detect anchors/aliases --- */

TEST(yp_anchor_alias) {
    const char *yaml =
        "defaults: &defaults\n"
        "  adapter: postgres\n"
        "production:\n"
        "  <<: *defaults\n";
    char *path = write_tmp_str(".yaml", yaml);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path);
    free(path);

    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_ANCHOR_OR_ALIAS);
    ASSERT_TRUE(result.has_anchors);

    yaml_parse_result_free(&result);
    free(em);
}

/* --- yaml_doc_find_key --- */

TEST(yp_find_key) {
    const char *yaml = "alpha: 1\nbeta: 2\ngamma: 3\n";
    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);

    yaml_kv_t *kv = yaml_doc_find_key(&result.docs[0], "beta");
    ASSERT_NOT_NULL(kv);
    ASSERT_STR_EQ(kv->key, "beta");
    ASSERT_STR_EQ(kv->scalar_value, "2");

    /* Key not found */
    yaml_kv_t *missing = yaml_doc_find_key(&result.docs[0], "delta");
    ASSERT_NULL(missing);

    /* NULL args */
    ASSERT_NULL(yaml_doc_find_key(NULL, "beta"));
    ASSERT_NULL(yaml_doc_find_key(&result.docs[0], NULL));

    yaml_parse_result_free(&result);
    free(em);
}

/* --- yaml_doc_get_string --- */

TEST(yp_get_string) {
    const char *yaml =
        "name: hello\n"
        "nested:\n"
        "  key: val\n";
    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);

    /* Scalar value */
    const char *val = yaml_doc_get_string(&result.docs[0], "name");
    ASSERT_NOT_NULL(val);
    ASSERT_STR_EQ(val, "hello");

    /* Non-scalar (mapping) returns NULL */
    const char *nested = yaml_doc_get_string(&result.docs[0], "nested");
    ASSERT_NULL(nested);

    /* Missing key returns NULL */
    const char *miss = yaml_doc_get_string(&result.docs[0], "missing");
    ASSERT_NULL(miss);

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Free without crash --- */

TEST(yp_free_no_crash) {
    /* Free a zeroed result */
    yaml_parse_result_t result;
    memset(&result, 0, sizeof(result));
    yaml_parse_result_free(&result);

    /* Free NULL */
    yaml_parse_result_free(NULL);

    /* Parse, then free */
    const char *yaml = "key: value\n";
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);
    yaml_parse_result_free(&result);
    free(em);

    /* Verify the result is zeroed after free */
    ASSERT_EQ(result.doc_count, 0);
    ASSERT_NULL(result.docs);
}

/* --- Key line numbers are 1-based --- */

TEST(yp_key_line_numbers) {
    const char *yaml =
        "first: a\n"
        "second: b\n"
        "third: c\n";
    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.docs[0].kvs[0].key_line, 1);
    ASSERT_EQ(result.docs[0].kvs[1].key_line, 2);
    ASSERT_EQ(result.docs[0].kvs[2].key_line, 3);

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Parse buffer with explicit tags --- */

TEST(yp_explicit_tag) {
    const char *yaml = "name: !!str hello\n";
    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_NEQ(rc, 0);
    ASSERT_STR_EQ(ec, EC_YAML_ANCHOR_OR_ALIAS);
    ASSERT_TRUE(result.has_anchors);

    yaml_parse_result_free(&result);
    free(em);
}

/* --- Sequence with nested mappings --- */

TEST(yp_sequence_with_mappings) {
    const char *yaml =
        "items:\n"
        "  - name: one\n"
        "    value: 1\n"
        "  - name: two\n"
        "    value: 2\n";
    yaml_parse_result_t result;
    const char *ec = NULL;
    char *em = NULL;
    int el = 0;

    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);

    yaml_kv_t *items = &result.docs[0].kvs[0];
    ASSERT_EQ(items->value_type, 1); /* sequence */
    ASSERT_EQ(items->sequence_count, 2);

    /* First item is a mapping */
    ASSERT_EQ(items->sequence_values[0].type, 2); /* mapping */
    ASSERT_EQ(items->sequence_values[0].mapping_count, 2);
    ASSERT_STR_EQ(items->sequence_values[0].mapping_values[0].key, "name");
    ASSERT_STR_EQ(items->sequence_values[0].mapping_values[0].scalar_value, "one");

    /* Second item */
    ASSERT_EQ(items->sequence_values[1].type, 2);
    ASSERT_STR_EQ(items->sequence_values[1].mapping_values[0].scalar_value, "two");

    yaml_parse_result_free(&result);
    free(em);
}

/* Test null/empty yaml_doc_find_key */
TEST(yp_find_key_edge) {
    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    yaml_kv_t *kv = yaml_doc_find_key(&doc, NULL);
    ASSERT_NULL(kv);
    kv = yaml_doc_find_key(NULL, "foo");
    ASSERT_NULL(kv);
    ASSERT_NULL(yaml_doc_get_string(&doc, "foo"));
}

/* Test parsing a document with null/empty values */
TEST(yp_null_values) {
    char *path = write_tmp_str(".yaml", "---\nfoo:\nbar: \"\"\nbaz: ~\n");
    ASSERT_NOT_NULL(path);
    yaml_parse_result_t result;
    const char *ec = NULL; char *em = NULL; int el = 0;
    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path); free(path);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);
    yaml_kv_t *foo = yaml_doc_find_key(&result.docs[0], "foo");
    ASSERT_NOT_NULL(foo);
    const char *bar = yaml_doc_get_string(&result.docs[0], "bar");
    ASSERT_NOT_NULL(bar);
    ASSERT_STR_EQ(bar, "");
    yaml_parse_result_free(&result);
    free(em);
}

/* Test parsing a file with nested sequences */
TEST(yp_nested_sequences) {
    char *path = write_tmp_str(".yaml", "---\nfoo:\n- - a\n  - b\n- - c\n");
    ASSERT_NOT_NULL(path);
    yaml_parse_result_t result;
    const char *ec = NULL; char *em = NULL; int el = 0;
    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path); free(path);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 1);
    yaml_kv_t *foo = yaml_doc_find_key(&result.docs[0], "foo");
    ASSERT_NOT_NULL(foo);
    ASSERT_EQ(foo->value_type, 1);
    ASSERT(foo->sequence_count >= 1);
    yaml_parse_result_free(&result);
    free(em);
}

/* Test yaml_parse_buffer with valid multi-doc content */
TEST(yp_buffer_direct) {
    const char *content = "---\nfoo: bar\n---\nbaz: qux\n";
    yaml_parse_result_t result;
    const char *ec = NULL; char *em = NULL; int el = 0;
    int rc = yaml_parse_buffer((const unsigned char *)content, strlen(content),
                                &result, &ec, &em, &el);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 2);
    ASSERT_STR_EQ(yaml_doc_get_string(&result.docs[0], "foo"), "bar");
    ASSERT_STR_EQ(yaml_doc_get_string(&result.docs[1], "baz"), "qux");
    yaml_parse_result_free(&result);
    free(em);
}

/* Test parsing a scalar-only document (non-mapping top level) */
TEST(yp_scalar_doc) {
    char *path = write_tmp_str(".yaml", "---\njust a string\n---\nfoo: bar\n");
    ASSERT_NOT_NULL(path);
    yaml_parse_result_t result;
    const char *ec = NULL; char *em = NULL; int el = 0;
    int rc = yaml_parse_file(path, &result, &ec, &em, &el);
    unlink(path); free(path);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.docs[0].kv_count, 0);
    ASSERT_EQ(result.docs[1].kv_count, 1);
    yaml_parse_result_free(&result);
    free(em);
}

/* --- Suite runner --- */

void run_suite_yaml_parse(void) {
    RUN_TEST(yp_simple_single_doc);
    RUN_TEST(yp_multi_doc);
    RUN_TEST(yp_sequence);
    RUN_TEST(yp_nested_mapping);
    RUN_TEST(yp_not_utf8);
    RUN_TEST(yp_has_bom);
    RUN_TEST(yp_empty_file);
    RUN_TEST(yp_malformed);
    RUN_TEST(yp_duplicate_keys);
    RUN_TEST(yp_anchor_alias);
    RUN_TEST(yp_find_key);
    RUN_TEST(yp_get_string);
    RUN_TEST(yp_free_no_crash);
    RUN_TEST(yp_key_line_numbers);
    RUN_TEST(yp_explicit_tag);
    RUN_TEST(yp_sequence_with_mappings);
    RUN_TEST(yp_find_key_edge);
    RUN_TEST(yp_null_values);
    RUN_TEST(yp_nested_sequences);
    RUN_TEST(yp_buffer_direct);
    RUN_TEST(yp_scalar_doc);
}
