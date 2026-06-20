/*
 * test_check_uniqueness.c - Tests for spec name uniqueness check
 */
#include "tinytest.h"
#include "check_uniqueness.h"
#include "yaml_parse.h"
#include "error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* Helper: write content to a temporary file and return the path.
 * The caller must unlink the file when done. */
static char *write_tmp(const char *suffix, const char *content) {
    static int counter = 0;
    char *path = malloc(256);
    snprintf(path, 256, "/tmp/yass_test_cu_%d_%d%s", (int)getpid(), counter++, suffix);
    FILE *f = fopen(path, "wb");
    if (!f) { free(path); return NULL; }
    fwrite(content, 1, strlen(content), f);
    fclose(f);
    return path;
}

/* Helper: parse a YAML string into a result, returns the temp path (caller frees/unlinks) */
static char *parse_yaml(const char *yaml, yaml_parse_result_t *result) {
    char *path = write_tmp(".yass.yaml", yaml);
    if (!path) return NULL;

    const char *ec = NULL;
    char *em = NULL;
    int el = 0;
    int rc = yaml_parse_file(path, result, &ec, &em, &el);
    free(em);
    if (rc != 0) {
        unlink(path);
        free(path);
        return NULL;
    }
    return path;
}

/* --- All unique names: no errors --- */

TEST(cu_all_unique) {
    const char *yaml =
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: Alpha\n"
        "---\n"
        "spec: Beta\n"
        "---\n"
        "spec: Gamma\n";

    yaml_parse_result_t result;
    char *path = parse_yaml(yaml, &result);
    ASSERT_NOT_NULL(path);

    int errors = check_uniqueness(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
    free(path);
}

/* --- One duplicate name: one error --- */

TEST(cu_one_duplicate) {
    const char *yaml =
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: Alpha\n"
        "---\n"
        "spec: Beta\n"
        "---\n"
        "spec: Alpha\n";

    yaml_parse_result_t result;
    char *path = parse_yaml(yaml, &result);
    ASSERT_NOT_NULL(path);

    int errors = check_uniqueness(path, &result);
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
    unlink(path);
    free(path);
}

/* --- Same name appears 3 times: two errors --- */

TEST(cu_triple_duplicate) {
    const char *yaml =
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: Alpha\n"
        "---\n"
        "spec: Alpha\n"
        "---\n"
        "spec: Alpha\n";

    yaml_parse_result_t result;
    char *path = parse_yaml(yaml, &result);
    ASSERT_NOT_NULL(path);

    int errors = check_uniqueness(path, &result);
    ASSERT_EQ(errors, 2);

    yaml_parse_result_free(&result);
    unlink(path);
    free(path);
}

/* --- Different names: no errors --- */

TEST(cu_different_names) {
    const char *yaml =
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "---\n"
        "spec: Bar\n"
        "---\n"
        "spec: Baz\n"
        "---\n"
        "spec: Qux\n";

    yaml_parse_result_t result;
    char *path = parse_yaml(yaml, &result);
    ASSERT_NOT_NULL(path);

    int errors = check_uniqueness(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
    free(path);
}

/* --- Case-sensitive: Foo vs foo are different --- */

TEST(cu_case_sensitive) {
    const char *yaml =
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "---\n"
        "spec: foo\n"
        "---\n"
        "spec: FOO\n";

    yaml_parse_result_t result;
    char *path = parse_yaml(yaml, &result);
    ASSERT_NOT_NULL(path);

    int errors = check_uniqueness(path, &result);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    unlink(path);
    free(path);
}

void run_suite_check_uniqueness(void) {
    RUN_TEST(cu_all_unique);
    RUN_TEST(cu_one_duplicate);
    RUN_TEST(cu_triple_duplicate);
    RUN_TEST(cu_different_names);
    RUN_TEST(cu_case_sensitive);
}
