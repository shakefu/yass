/*
 * test_emit.c - Tests for YAML emitter per cli.query.OutputProfile
 */
#include "tinytest.h"
#include "../src/emit.h"

#include <stdlib.h>
#include <string.h>

/* ------------------------------------------------------------------ */
/*  Helper: capture emit_spec_fragment output into a string            */
/* ------------------------------------------------------------------ */

static char *capture_fragment(yaml_doc_t *doc) {
    char *buf = NULL;
    size_t len = 0;
    FILE *f = open_memstream(&buf, &len);
    if (!f) return NULL;
    emit_spec_fragment(f, doc);
    fclose(f);
    return buf;
}

/* ------------------------------------------------------------------ */
/*  emit_needs_quoting tests                                           */
/* ------------------------------------------------------------------ */

TEST(emit_quoting_plain) {
    ASSERT_EQ(emit_needs_quoting("hello"), 0);
    ASSERT_EQ(emit_needs_quoting("foo.bar"), 0);
    ASSERT_EQ(emit_needs_quoting("spec"), 0);
    ASSERT_EQ(emit_needs_quoting("some plain text"), 0);
}

TEST(emit_quoting_colon_space) {
    ASSERT_EQ(emit_needs_quoting("key: value"), 1);
    ASSERT_EQ(emit_needs_quoting("a: b: c"), 1);
    /* colon without space is OK */
    ASSERT_EQ(emit_needs_quoting("key:value"), 0);
}

TEST(emit_quoting_leading_special) {
    ASSERT_EQ(emit_needs_quoting("*anchor"), 1);
    ASSERT_EQ(emit_needs_quoting("?question"), 1);
    ASSERT_EQ(emit_needs_quoting("-dash"), 1);
    ASSERT_EQ(emit_needs_quoting("&ref"), 1);
    ASSERT_EQ(emit_needs_quoting("!tag"), 1);
    ASSERT_EQ(emit_needs_quoting("|literal"), 1);
    ASSERT_EQ(emit_needs_quoting(">folded"), 1);
    ASSERT_EQ(emit_needs_quoting("%directive"), 1);
    ASSERT_EQ(emit_needs_quoting("@at"), 1);
}

TEST(emit_quoting_bool_tokens) {
    ASSERT_EQ(emit_needs_quoting("true"), 1);
    ASSERT_EQ(emit_needs_quoting("false"), 1);
    ASSERT_EQ(emit_needs_quoting("null"), 1);
    ASSERT_EQ(emit_needs_quoting("TRUE"), 1);
    ASSERT_EQ(emit_needs_quoting("False"), 1);
    ASSERT_EQ(emit_needs_quoting("NULL"), 1);
    ASSERT_EQ(emit_needs_quoting("Null"), 1);
}

TEST(emit_quoting_yes_no) {
    ASSERT_EQ(emit_needs_quoting("yes"), 1);
    ASSERT_EQ(emit_needs_quoting("no"), 1);
    ASSERT_EQ(emit_needs_quoting("YES"), 1);
    ASSERT_EQ(emit_needs_quoting("NO"), 1);
    ASSERT_EQ(emit_needs_quoting("Yes"), 1);
    ASSERT_EQ(emit_needs_quoting("No"), 1);
    ASSERT_EQ(emit_needs_quoting("on"), 1);
    ASSERT_EQ(emit_needs_quoting("off"), 1);
    ASSERT_EQ(emit_needs_quoting("ON"), 1);
    ASSERT_EQ(emit_needs_quoting("OFF"), 1);
}

TEST(emit_quoting_numeric) {
    ASSERT_EQ(emit_needs_quoting("42"), 1);
    ASSERT_EQ(emit_needs_quoting("3.14"), 1);
    ASSERT_EQ(emit_needs_quoting("0"), 1);
    ASSERT_EQ(emit_needs_quoting("0x1f"), 1);
    ASSERT_EQ(emit_needs_quoting("0o17"), 1);
    ASSERT_EQ(emit_needs_quoting(".inf"), 1);
    ASSERT_EQ(emit_needs_quoting(".nan"), 1);
    ASSERT_EQ(emit_needs_quoting("1e10"), 1);
}

TEST(emit_quoting_empty_string) {
    ASSERT_EQ(emit_needs_quoting(""), 1);
    ASSERT_EQ(emit_needs_quoting(NULL), 1);
}

TEST(emit_quoting_leading_trailing_space) {
    ASSERT_EQ(emit_needs_quoting(" leading"), 1);
    ASSERT_EQ(emit_needs_quoting("trailing "), 1);
    ASSERT_EQ(emit_needs_quoting(" both "), 1);
    ASSERT_EQ(emit_needs_quoting("\ttab"), 1);
}

/* ------------------------------------------------------------------ */
/*  emit_spec_fragment tests                                           */
/* ------------------------------------------------------------------ */

TEST(emit_fragment_simple) {
    /*
     * Build:  spec: my.spec
     *         description: a simple spec
     */
    yaml_kv_t kvs[2];
    memset(kvs, 0, sizeof(kvs));

    kvs[0].key = "spec";
    kvs[0].value_type = 0;
    kvs[0].scalar_value = "my.spec";

    kvs[1].key = "description";
    kvs[1].value_type = 0;
    kvs[1].scalar_value = "a simple spec";

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 2;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result,
        "---\n"
        "spec: my.spec\n"
        "description: a simple spec\n"
    );
    free(result);
}

TEST(emit_fragment_quoted_scalar) {
    /*
     * Build:  spec: my.spec
     *         value: "true"
     */
    yaml_kv_t kvs[2];
    memset(kvs, 0, sizeof(kvs));

    kvs[0].key = "spec";
    kvs[0].value_type = 0;
    kvs[0].scalar_value = "my.spec";

    kvs[1].key = "value";
    kvs[1].value_type = 0;
    kvs[1].scalar_value = "true";

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 2;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result,
        "---\n"
        "spec: my.spec\n"
        "value: \"true\"\n"
    );
    free(result);
}

TEST(emit_fragment_sequence_of_mappings) {
    /*
     * Build:
     *   spec: my.spec
     *   RETURN:
     *   - MUST: do something
     *     USES: other.spec
     *   - WHEN: condition holds
     *     MUST: do another thing
     */
    yass_yaml_kv_t item0_kvs[2];
    memset(item0_kvs, 0, sizeof(item0_kvs));
    item0_kvs[0].key = "MUST";
    item0_kvs[0].value_type = 0;
    item0_kvs[0].scalar_value = "do something";
    item0_kvs[1].key = "USES";
    item0_kvs[1].value_type = 0;
    item0_kvs[1].scalar_value = "other.spec";

    yass_yaml_kv_t item1_kvs[2];
    memset(item1_kvs, 0, sizeof(item1_kvs));
    item1_kvs[0].key = "WHEN";
    item1_kvs[0].value_type = 0;
    item1_kvs[0].scalar_value = "condition holds";
    item1_kvs[1].key = "MUST";
    item1_kvs[1].value_type = 0;
    item1_kvs[1].scalar_value = "do another thing";

    yass_yaml_node_t items[2];
    memset(items, 0, sizeof(items));
    items[0].type = 2; /* mapping */
    items[0].mapping_values = item0_kvs;
    items[0].mapping_count = 2;
    items[1].type = 2; /* mapping */
    items[1].mapping_values = item1_kvs;
    items[1].mapping_count = 2;

    yaml_kv_t top_kvs[2];
    memset(top_kvs, 0, sizeof(top_kvs));
    top_kvs[0].key = "spec";
    top_kvs[0].value_type = 0;
    top_kvs[0].scalar_value = "my.spec";
    top_kvs[1].key = "RETURN";
    top_kvs[1].value_type = 1; /* sequence */
    top_kvs[1].sequence_values = items;
    top_kvs[1].sequence_count = 2;

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = top_kvs;
    doc.kv_count = 2;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result,
        "---\n"
        "spec: my.spec\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: other.spec\n"
        "- WHEN: condition holds\n"
        "  MUST: do another thing\n"
    );
    free(result);
}

TEST(emit_fragment_starts_with_doc_marker) {
    yaml_kv_t kvs[1];
    memset(kvs, 0, sizeof(kvs));
    kvs[0].key = "spec";
    kvs[0].value_type = 0;
    kvs[0].scalar_value = "x";

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 1;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    /* Must start with --- */
    ASSERT(strncmp(result, "---\n", 4) == 0);
    /* Must NOT contain trailing ... */
    ASSERT(strstr(result, "...") == NULL);
    /* Must end with exactly one LF */
    size_t len = strlen(result);
    ASSERT(len > 0);
    ASSERT(result[len - 1] == '\n');
    if (len > 1) ASSERT(result[len - 2] != '\n');
    free(result);
}

TEST(emit_fragment_null_args) {
    int rc = emit_spec_fragment(NULL, NULL);
    ASSERT_EQ(rc, 1);
}

TEST(emit_fragment_escape_sequences) {
    /*
     * Test double-quote escaping: backslash, double-quote, colon-space
     */
    yaml_kv_t kvs[1];
    memset(kvs, 0, sizeof(kvs));
    kvs[0].key = "msg";
    kvs[0].value_type = 0;
    kvs[0].scalar_value = "has: colon and \"quotes\"";

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 1;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_CONTAINS(result, "\"has: colon and \\\"quotes\\\"\"");
    free(result);
}

TEST(emit_fragment_colon_space_value) {
    yaml_kv_t kvs[1];
    memset(kvs, 0, sizeof(kvs));
    kvs[0].key = "MUST";
    kvs[0].value_type = 0;
    kvs[0].scalar_value = "emit key: value pairs";

    yaml_doc_t doc;
    memset(&doc, 0, sizeof(doc));
    doc.kvs = kvs;
    doc.kv_count = 1;

    char *result = capture_fragment(&doc);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result,
        "---\n"
        "MUST: \"emit key: value pairs\"\n"
    );
    free(result);
}

/* ------------------------------------------------------------------ */
/*  Suite runner                                                       */
/* ------------------------------------------------------------------ */

void run_suite_emit(void) {
    RUN_TEST(emit_quoting_plain);
    RUN_TEST(emit_quoting_colon_space);
    RUN_TEST(emit_quoting_leading_special);
    RUN_TEST(emit_quoting_bool_tokens);
    RUN_TEST(emit_quoting_yes_no);
    RUN_TEST(emit_quoting_numeric);
    RUN_TEST(emit_quoting_empty_string);
    RUN_TEST(emit_quoting_leading_trailing_space);
    RUN_TEST(emit_fragment_simple);
    RUN_TEST(emit_fragment_quoted_scalar);
    RUN_TEST(emit_fragment_sequence_of_mappings);
    RUN_TEST(emit_fragment_starts_with_doc_marker);
    RUN_TEST(emit_fragment_null_args);
    RUN_TEST(emit_fragment_escape_sequences);
    RUN_TEST(emit_fragment_colon_space_value);
}
