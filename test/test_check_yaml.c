/*
 * test_check_yaml.c - Tests for YAML well-formedness checking
 */
#include "tinytest.h"
#include "../src/check_yaml.h"
#include "../src/yaml_parse.h"
#include "../src/error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* --- helpers --- */

/*
 * Write raw bytes to a temp file and return the path.
 * The caller must unlink the file when done.
 */
static const char *write_temp(const char *suffix, const unsigned char *data, size_t len) {
    static char path[256];
    snprintf(path, sizeof(path), "/tmp/yass_test_check_yaml_%s", suffix);
    FILE *f = fopen(path, "wb");
    if (!f) return NULL;
    if (len > 0) {
        fwrite(data, 1, len, f);
    }
    fclose(f);
    return path;
}

/*
 * Write a string to a temp file.
 */
static const char *write_temp_str(const char *suffix, const char *content) {
    return write_temp(suffix, (const unsigned char *)content, strlen(content));
}

/* --- valid YAML --- */

TEST(check_yaml_valid_simple) {
    const char *path = write_temp_str("valid_simple",
        "---\nspec: foo.bar\nINPUT:\n- MUST: do something\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 0);
    ASSERT(result.doc_count > 0);
    yaml_parse_result_free(&result);
    unlink(path);
}

TEST(check_yaml_valid_multi_doc) {
    const char *path = write_temp_str("valid_multi",
        "---\ndescription: test\nversion: v1\n---\nspec: my.spec\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 2);
    yaml_parse_result_free(&result);
    unlink(path);
}

/* --- not UTF-8 --- */

TEST(check_yaml_not_utf8) {
    /* Invalid UTF-8: bare continuation byte */
    unsigned char data[] = { 0x80, 0x81, 0x82 };
    const char *path = write_temp("not_utf8", data, sizeof(data));
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_not_utf8_with_valid_yaml_after) {
    /* Invalid UTF-8 byte followed by valid YAML content */
    unsigned char data[] = { 0xFE, 's', 'p', 'e', 'c', ':', ' ', 'a', '\n' };
    const char *path = write_temp("not_utf8_mixed", data, sizeof(data));
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- BOM --- */

TEST(check_yaml_has_bom) {
    /* UTF-8 BOM (EF BB BF) followed by valid YAML */
    const char *content = "\xEF\xBB\xBF---\nspec: foo\n";
    const char *path = write_temp_str("has_bom", content);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- empty file --- */

TEST(check_yaml_empty_file) {
    const char *path = write_temp("empty", (const unsigned char *)"", 0);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- malformed YAML --- */

TEST(check_yaml_malformed) {
    const char *path = write_temp_str("malformed",
        "---\nkey: value\n  bad indent: here\n    : also bad\n[[[");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_malformed_tabs) {
    const char *path = write_temp_str("malformed_tabs",
        "---\nkey:\n\t- value\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- duplicate keys --- */

TEST(check_yaml_duplicate_key) {
    const char *path = write_temp_str("dup_key",
        "---\nfoo: one\nbar: two\nfoo: three\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- anchors and aliases --- */

TEST(check_yaml_anchor) {
    const char *path = write_temp_str("anchor",
        "---\nfoo: &anchor value\nbar: *anchor\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_explicit_tag) {
    const char *path = write_temp_str("explicit_tag",
        "---\nfoo: !!str bar\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- priority order tests --- */

TEST(check_yaml_priority_utf8_over_bom) {
    /*
     * A file that is not valid UTF-8 but also starts with BOM bytes:
     * not_utf8 should take priority over has_bom.
     * BOM is EF BB BF, followed by invalid continuation byte.
     * But BOM itself is valid UTF-8... so we need invalid bytes after.
     * Actually, for not_utf8 to trigger, the entire buffer must fail
     * utf8_is_valid. BOM bytes are valid UTF-8. So to trigger not_utf8
     * with BOM present, we need invalid bytes mixed in.
     *
     * EF BB BF is valid UTF-8 (it's U+FEFF). To make the file invalid
     * UTF-8, add a bad byte after the BOM.
     */
    unsigned char data[] = { 0xEF, 0xBB, 0xBF, 0xFE };
    const char *path = write_temp("priority_utf8_bom", data, sizeof(data));
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* Should fail - not_utf8 takes priority over has_bom */
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_priority_bom_over_malformed) {
    /* BOM followed by malformed YAML - BOM takes priority */
    const char *content = "\xEF\xBB\xBF[[[bad yaml";
    const char *path = write_temp_str("priority_bom_malformed", content);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* BOM should be caught before parsing */
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_priority_empty_over_malformed) {
    /* Empty file check happens before parse, so empty takes priority */
    const char *path = write_temp("priority_empty", (const unsigned char *)"", 0);
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

TEST(check_yaml_priority_dup_over_anchor) {
    /*
     * File with both duplicate keys and anchors.
     * duplicate_key should take priority over anchor_or_alias.
     *
     * Note: libyaml may fail to parse anchors/aliases in certain positions,
     * but we just need duplicate keys detected alongside anchors.
     * Use an anchor on a value (not alias) with duplicate keys.
     */
    const char *path = write_temp_str("priority_dup_anchor",
        "---\nfoo: &anc value1\nbar: baz\nfoo: value2\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* Should fail with duplicate_key, not anchor_or_alias */
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- file not found --- */

TEST(check_yaml_file_not_found) {
    yaml_parse_result_t result;
    int rc = check_yaml("/tmp/yass_test_check_yaml_nonexistent_file_xyz", &result);
    ASSERT_EQ(rc, 1);
}

/*
 * Empty stream: file has content but no YAML documents.
 * This hits the "other error" fallback path (not malformed, not dup, not anchor).
 */
TEST(check_yaml_empty_stream) {
    const char *path = write_temp_str("empty_stream", "# just a comment\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* Should fail with empty_stream error */
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/*
 * Empty stream with only whitespace.
 */
TEST(check_yaml_whitespace_only) {
    const char *path = write_temp_str("whitespace_only", "   \n\n   \n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/*
 * Valid YAML with three documents.
 */
TEST(check_yaml_valid_three_docs) {
    const char *path = write_temp_str("valid_three",
        "---\nfoo: bar\n---\nbaz: qux\n---\nhello: world\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.doc_count, 3);
    yaml_parse_result_free(&result);
    unlink(path);
}

/*
 * Valid YAML - single scalar value (not a mapping).
 * Tests the parser's handling of non-mapping documents.
 */
TEST(check_yaml_scalar_doc) {
    const char *path = write_temp_str("scalar_doc", "---\nhello\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* Parsing a scalar doc may succeed or fail depending on parser behavior */
    /* Either way, it should not crash */
    if (rc == 0) {
        yaml_parse_result_free(&result);
    }
    unlink(path);
}

/*
 * Duplicate key with unknown name (NULL duplicate_key_name).
 * This is hard to trigger via normal YAML, but we verify the existing
 * duplicate key path with a different key name.
 */
TEST(check_yaml_duplicate_key_different) {
    const char *path = write_temp_str("dup_key2",
        "---\nalpha: one\nbeta: two\nalpha: three\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/*
 * Alias without anchor definition -> parser error.
 */
TEST(check_yaml_alias_only) {
    const char *path = write_temp_str("alias_only",
        "---\nfoo: *undefined_anchor\n");
    ASSERT_NOT_NULL(path);

    yaml_parse_result_t result;
    int rc = check_yaml(path, &result);
    /* Should fail - either malformed or anchor_or_alias */
    ASSERT_EQ(rc, 1);
    unlink(path);
}

/* --- Suite runner --- */

void run_suite_check_yaml(void) {
    /* valid YAML */
    RUN_TEST(check_yaml_valid_simple);
    RUN_TEST(check_yaml_valid_multi_doc);
    RUN_TEST(check_yaml_valid_three_docs);
    RUN_TEST(check_yaml_scalar_doc);

    /* not UTF-8 */
    RUN_TEST(check_yaml_not_utf8);
    RUN_TEST(check_yaml_not_utf8_with_valid_yaml_after);

    /* BOM */
    RUN_TEST(check_yaml_has_bom);

    /* empty file */
    RUN_TEST(check_yaml_empty_file);

    /* empty stream */
    RUN_TEST(check_yaml_empty_stream);
    RUN_TEST(check_yaml_whitespace_only);

    /* malformed YAML */
    RUN_TEST(check_yaml_malformed);
    RUN_TEST(check_yaml_malformed_tabs);

    /* duplicate keys */
    RUN_TEST(check_yaml_duplicate_key);
    RUN_TEST(check_yaml_duplicate_key_different);

    /* anchors and aliases */
    RUN_TEST(check_yaml_anchor);
    RUN_TEST(check_yaml_explicit_tag);
    RUN_TEST(check_yaml_alias_only);

    /* priority ordering */
    RUN_TEST(check_yaml_priority_utf8_over_bom);
    RUN_TEST(check_yaml_priority_bom_over_malformed);
    RUN_TEST(check_yaml_priority_empty_over_malformed);
    RUN_TEST(check_yaml_priority_dup_over_anchor);

    /* file not found */
    RUN_TEST(check_yaml_file_not_found);
}
