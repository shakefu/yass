/*
 * test_check_refs.c - Tests for reference resolution checking
 */
#include "tinytest.h"
#include "../src/check_refs.h"
#include "../src/yaml_parse.h"
#include "../src/error.h"
#include "../src/path.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>
#include <dirent.h>

/* ---- helpers ---- */

static void rmtree(const char *path)
{
    DIR *dp = opendir(path);
    if (!dp)
        return;
    struct dirent *ent;
    while ((ent = readdir(dp)) != NULL) {
        if (strcmp(ent->d_name, ".") == 0 || strcmp(ent->d_name, "..") == 0)
            continue;
        char child[4096];
        snprintf(child, sizeof(child), "%s/%s", path, ent->d_name);
        struct stat st;
        if (lstat(child, &st) == 0) {
            if (S_ISDIR(st.st_mode))
                rmtree(child);
            else
                unlink(child);
        }
    }
    closedir(dp);
    rmdir(path);
}

static void mkdirp(const char *path)
{
    char tmp[4096];
    snprintf(tmp, sizeof(tmp), "%s", path);
    for (char *p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = '\0';
            mkdir(tmp, 0755);
            *p = '/';
        }
    }
    mkdir(tmp, 0755);
}

static void write_file(const char *path, const char *content)
{
    FILE *f = fopen(path, "w");
    if (f) {
        fputs(content, f);
        fclose(f);
    }
}

/*
 * Parse a YAML buffer into a result for testing.
 * Returns 0 on success.
 */
static int parse_yaml(const char *yaml, yaml_parse_result_t *result)
{
    const char *err_code = NULL;
    char *err_msg = NULL;
    int err_line = 0;
    int rc = yaml_parse_buffer((const unsigned char *)yaml, strlen(yaml),
                               result, &err_code, &err_msg, &err_line);
    free(err_msg);
    return rc;
}

/* ---- Valid same-file ref -> no error ---- */

TEST(check_refs_valid_same_file)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: Foo\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
}

/* ---- Same-file ref not found -> error ---- */

TEST(check_refs_same_file_not_found)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: NonExistent\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
}

/* ---- Valid cross-file ref (relative) -> no error ---- */

TEST(check_refs_valid_cross_file_relative)
{
    const char *base = "/tmp/yass_test_check_refs_cross";
    rmtree(base);
    mkdirp(base);

    /* Create a referenced file with a spec named Baz */
    char ref_file[4096];
    snprintf(ref_file, sizeof(ref_file), "%s/other.yass.yaml", base);
    write_file(ref_file,
        "---\n"
        "description: other\n"
        "version: v1\n"
        "---\n"
        "spec: Baz\n"
        "RETURN:\n"
        "- MUST: do something\n");

    /* Create the referencing file content */
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: ./other@Baz\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, base, base);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Cross-file ref file not found -> error ---- */

TEST(check_refs_cross_file_not_found)
{
    const char *base = "/tmp/yass_test_check_refs_no_file";
    rmtree(base);
    mkdirp(base);

    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: ./missing@Bar\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, base, base);
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Cross-file ref spec not found -> error ---- */

TEST(check_refs_cross_file_spec_not_found)
{
    const char *base = "/tmp/yass_test_check_refs_no_spec";
    rmtree(base);
    mkdirp(base);

    /* Create a referenced file without the expected spec */
    char ref_file[4096];
    snprintf(ref_file, sizeof(ref_file), "%s/other.yass.yaml", base);
    write_file(ref_file,
        "---\n"
        "description: other\n"
        "version: v1\n"
        "---\n"
        "spec: WrongName\n"
        "RETURN:\n"
        "- MUST: do something\n");

    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: ./other@RightName\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, base, base);
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Slot ref on valid spec -> no error ---- */

TEST(check_refs_valid_slot_ref)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  CONFORMS: Foo::RETURN\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
}

/* ---- Slot ref on spec without that slot -> slot_not_declared ---- */

TEST(check_refs_slot_not_declared)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  CONFORMS: Foo::ERROR\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
}

/* ---- Malformed ref target -> malformed ---- */

TEST(check_refs_malformed)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: \"not a valid ref!!\"\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
}

/* ---- Unknown slot in ref -> unknown_slot ---- */

TEST(check_refs_unknown_slot)
{
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: Bar\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: Foo::BOGUS\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, "/tmp", "/tmp");
    /* Should get unknown_slot error */
    ASSERT(errors >= 1);

    yaml_parse_result_free(&result);
}

/* ---- Cross-file ref with root-relative path ---- */

TEST(check_refs_root_relative)
{
    const char *base = "/tmp/yass_test_check_refs_root_rel";
    rmtree(base);
    mkdirp(base);

    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/sub", base);
    mkdirp(subdir);

    /* Create a referenced file at project root */
    char ref_file[4096];
    snprintf(ref_file, sizeof(ref_file), "%s/lib.yass.yaml", base);
    write_file(ref_file,
        "---\n"
        "description: lib\n"
        "version: v1\n"
        "---\n"
        "spec: Widget\n"
        "RETURN:\n"
        "- MUST: do something\n");

    /* Reference uses root-relative path (no leading dot) */
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  USES: lib@Widget\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    /* base_dir is the subdir, project_root is base */
    int errors = check_refs("test.yass.yaml", &result, subdir, base);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Deduplicate file-level errors ---- */

TEST(check_refs_dedup_file_errors)
{
    const char *base = "/tmp/yass_test_check_refs_dedup";
    rmtree(base);
    mkdirp(base);

    /* Two refs to the same missing file should produce only one error */
    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: first ref\n"
        "  USES: ./missing@A\n"
        "- MUST: second ref\n"
        "  USES: ./missing@B\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, base, base);
    /* Only one file_not_found error, second ref is silently skipped */
    ASSERT_EQ(errors, 1);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Cross-file slot ref valid ---- */

TEST(check_refs_cross_file_slot_valid)
{
    const char *base = "/tmp/yass_test_check_refs_xslot";
    rmtree(base);
    mkdirp(base);

    char ref_file[4096];
    snprintf(ref_file, sizeof(ref_file), "%s/other.yass.yaml", base);
    write_file(ref_file,
        "---\n"
        "description: other\n"
        "version: v1\n"
        "---\n"
        "spec: Target\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "ERROR:\n"
        "- MUST: handle errors\n");

    const char *yaml =
        "---\n"
        "description: test\n"
        "version: v1\n"
        "---\n"
        "spec: Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "  CONFORMS: ./other@Target::RETURN\n";

    yaml_parse_result_t result;
    int rc = parse_yaml(yaml, &result);
    ASSERT_EQ(rc, 0);

    int errors = check_refs("test.yass.yaml", &result, base, base);
    ASSERT_EQ(errors, 0);

    yaml_parse_result_free(&result);
    rmtree(base);
}

/* ---- Suite runner ---- */

void run_suite_check_refs(void)
{
    RUN_TEST(check_refs_valid_same_file);
    RUN_TEST(check_refs_same_file_not_found);
    RUN_TEST(check_refs_valid_cross_file_relative);
    RUN_TEST(check_refs_cross_file_not_found);
    RUN_TEST(check_refs_cross_file_spec_not_found);
    RUN_TEST(check_refs_valid_slot_ref);
    RUN_TEST(check_refs_slot_not_declared);
    RUN_TEST(check_refs_malformed);
    RUN_TEST(check_refs_unknown_slot);
    RUN_TEST(check_refs_root_relative);
    RUN_TEST(check_refs_dedup_file_errors);
    RUN_TEST(check_refs_cross_file_slot_valid);
}
