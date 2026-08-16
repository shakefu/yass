/*
 * test_validate.c - Tests for the validate subcommand
 */
#include "tinytest.h"
#include "../src/validate.h"
#include "../src/error.h"

#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

/* ---- helpers ---- */

static char saved_cwd[4096];

static void save_cwd(void)
{
    getcwd(saved_cwd, sizeof(saved_cwd));
}

static void restore_cwd(void)
{
    if (saved_cwd[0])
        chdir(saved_cwd);
}

/*
 * Recursively remove a directory tree.
 */
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
        if (content)
            fputs(content, f);
        fclose(f);
    }
}

/*
 * Capture stdout into a buffer during a cmd_validate call.
 * Returns the exit code from cmd_validate.
 * The caller must free *out_buf if it is non-NULL.
 */
static int run_validate_capture(int argc, char **argv,
                                char **out_buf, size_t *out_len)
{
    /* Flush and redirect stdout to a temp file */
    fflush(stdout);
    int saved_stdout = dup(STDOUT_FILENO);
    char tmppath[] = "/tmp/yass_test_validate_stdout_XXXXXX";
    int tmpfd = mkstemp(tmppath);
    dup2(tmpfd, STDOUT_FILENO);

    int rc = cmd_validate(argc, argv);

    /* Restore stdout */
    fflush(stdout);
    dup2(saved_stdout, STDOUT_FILENO);
    close(saved_stdout);

    /* Read captured output */
    lseek(tmpfd, 0, SEEK_SET);
    struct stat st;
    fstat(tmpfd, &st);
    size_t len = (size_t)st.st_size;
    char *buf = malloc(len + 1);
    if (buf) {
        ssize_t n = read(tmpfd, buf, len);
        if (n < 0) n = 0;
        buf[n] = '\0';
        *out_buf = buf;
        *out_len = (size_t)n;
    } else {
        *out_buf = NULL;
        *out_len = 0;
    }
    close(tmpfd);
    unlink(tmppath);

    return rc;
}

/* ---- valid file -> 0 errors, exit 0 ---- */

TEST(validate_valid_file)
{
    const char *base = "/tmp/yass_test_validate_valid";
    rmtree(base);
    mkdirp(base);

    /* Create .git marker for project root */
    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a valid .yass.yaml file */
    char specfile[4096];
    snprintf(specfile, sizeof(specfile), "%s/valid.yass.yaml", base);
    write_file(specfile,
        "---\n"
        "description: A valid spec file\n"
        "version: v1\n"
        "---\n"
        "spec: my.valid.Spec\n"
        "INPUT:\n"
        "- MUST: do something useful\n");

    save_cwd();
    chdir(base);

    char *args[] = { specfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 1 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- invalid YAML file -> 1 error, exit 1 ---- */

TEST(validate_invalid_yaml)
{
    const char *base = "/tmp/yass_test_validate_invalid_yaml";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a malformed YAML file */
    char specfile[4096];
    snprintf(specfile, sizeof(specfile), "%s/bad.yass.yaml", base);
    write_file(specfile, "---\nkey:\n\t- bad indent\n");

    save_cwd();
    chdir(base);

    char *args[] = { specfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 1);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 1 files, found 1 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- missing preamble -> error ---- */

TEST(validate_missing_preamble)
{
    const char *base = "/tmp/yass_test_validate_missing_preamble";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a file with no preamble (just a spec doc) */
    char specfile[4096];
    snprintf(specfile, sizeof(specfile), "%s/nopreamble.yass.yaml", base);
    write_file(specfile,
        "---\n"
        "spec: some.Spec\n"
        "INPUT:\n"
        "- MUST: do something\n");

    save_cwd();
    chdir(base);

    char *args[] = { specfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    /* Should have at least one error (missing preamble or has_spec_key) */
    ASSERT_EQ(rc, 1);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 1 files");
    /* M should be >= 1 */
    ASSERT(strstr(out, "found 0 errors") == NULL);

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- bad spec name -> error ---- */

TEST(validate_bad_spec_name)
{
    const char *base = "/tmp/yass_test_validate_bad_spec_name";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a file with a valid preamble but a bad spec name */
    char specfile[4096];
    snprintf(specfile, sizeof(specfile), "%s/badname.yass.yaml", base);
    write_file(specfile,
        "---\n"
        "description: test file\n"
        "version: v1\n"
        "---\n"
        "spec: \"\"\n"
        "INPUT:\n"
        "- MUST: do something\n");

    save_cwd();
    chdir(base);

    char *args[] = { specfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    /* Should have errors for the empty spec name */
    ASSERT_EQ(rc, 1);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 1 files");
    ASSERT(strstr(out, "found 0 errors") == NULL);

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- summary format ---- */

TEST(validate_summary_format)
{
    const char *base = "/tmp/yass_test_validate_summary_fmt";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create two valid files */
    char f1[4096], f2[4096];
    snprintf(f1, sizeof(f1), "%s/alpha.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/beta.yass.yaml", base);
    write_file(f1,
        "---\n"
        "description: first file\n"
        "version: v1\n"
        "---\n"
        "spec: alpha.Spec\n"
        "INPUT:\n"
        "- MUST: do alpha things\n");
    write_file(f2,
        "---\n"
        "description: second file\n"
        "version: v1\n"
        "---\n"
        "spec: beta.Spec\n"
        "INPUT:\n"
        "- MUST: do beta things\n");

    save_cwd();
    chdir(base);

    char *args[] = { f1, f2 };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(2, args, &out, &outlen);

    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 2 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- no files found -> exit 2 ---- */

TEST(validate_no_files_found)
{
    const char *base = "/tmp/yass_test_validate_no_files";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Empty directory - no .yass.yaml files */
    save_cwd();
    chdir(base);

    /* Discover with no args from a project root that has no spec files */
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(0, NULL, &out, &outlen);

    ASSERT_EQ(rc, 2);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 0 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- colon in path -> exit 2 ---- */

TEST(validate_colon_in_path)
{
    const char *base = "/tmp/yass_test_validate_colon";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    save_cwd();
    chdir(base);

    char *args[] = { "foo:bar.yass.yaml" };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 2);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 0 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- bad extension (non-glob file arg) -> exit 2 ---- */

TEST(validate_bad_extension)
{
    const char *base = "/tmp/yass_test_validate_bad_ext";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a file with wrong extension */
    char badfile[4096];
    snprintf(badfile, sizeof(badfile), "%s/notaspec.yaml", base);
    write_file(badfile, "key: value\n");

    save_cwd();
    chdir(base);

    char *args[] = { badfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    /* Non-glob file arg without .yass.yaml extension -> exit 2 */
    ASSERT_EQ(rc, 2);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 0 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- path not found -> exit 2 ---- */

TEST(validate_path_not_found)
{
    const char *base = "/tmp/yass_test_validate_notfound";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    save_cwd();
    chdir(base);

    char *args[] = { "/tmp/yass_test_validate_notfound/nonexistent.yass.yaml" };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 2);

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- multiple files with mixed errors ---- */

TEST(validate_mixed_errors)
{
    const char *base = "/tmp/yass_test_validate_mixed";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* One valid file */
    char f1[4096];
    snprintf(f1, sizeof(f1), "%s/good.yass.yaml", base);
    write_file(f1,
        "---\n"
        "description: good file\n"
        "version: v1\n"
        "---\n"
        "spec: good.Spec\n"
        "INPUT:\n"
        "- MUST: work correctly\n");

    /* One file with bad YAML (empty file -> exactly 1 error) */
    char f2[4096];
    snprintf(f2, sizeof(f2), "%s/bad.yass.yaml", base);
    write_file(f2, "");

    save_cwd();
    chdir(base);

    char *args[] = { f1, f2 };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(2, args, &out, &outlen);

    ASSERT_EQ(rc, 1);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 2 files");
    /* Should have at least 1 error from the empty file */
    ASSERT(strstr(out, "found 0 errors") == NULL);

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- directory discovery ---- */

TEST(validate_directory_discovery)
{
    const char *base = "/tmp/yass_test_validate_dir_disc";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/specs", base);
    mkdirp(subdir);

    char f1[4096];
    snprintf(f1, sizeof(f1), "%s/specs/one.yass.yaml", base);
    write_file(f1,
        "---\n"
        "description: spec one\n"
        "version: v1\n"
        "---\n"
        "spec: one.Spec\n"
        "INPUT:\n"
        "- MUST: do one thing\n");

    save_cwd();
    chdir(base);

    char *args[] = { subdir };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "checked 1 files, found 0 errors");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- summary line exact format ---- */

TEST(validate_summary_line_format)
{
    const char *base = "/tmp/yass_test_validate_sumfmt";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Create a valid .yass.yaml file */
    char specfile[4096];
    snprintf(specfile, sizeof(specfile), "%s/fmt.yass.yaml", base);
    write_file(specfile,
        "---\n"
        "description: format test\n"
        "version: v1\n"
        "---\n"
        "spec: fmt.Spec\n"
        "INPUT:\n"
        "- MUST: be formatted\n");

    save_cwd();
    chdir(base);

    char *args[] = { specfile };
    char *out = NULL;
    size_t outlen = 0;
    int rc = run_validate_capture(1, args, &out, &outlen);

    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Exact format: "checked N files, found M errors\n" */
    ASSERT_STR_EQ(out, "checked 1 files, found 0 errors\n");

    free(out);
    restore_cwd();
    rmtree(base);
}

/* ---- Suite registration ---- */

void run_suite_validate(void)
{
    RUN_TEST(validate_valid_file);
    RUN_TEST(validate_invalid_yaml);
    RUN_TEST(validate_missing_preamble);
    RUN_TEST(validate_bad_spec_name);
    RUN_TEST(validate_summary_format);
    RUN_TEST(validate_no_files_found);
    RUN_TEST(validate_colon_in_path);
    RUN_TEST(validate_bad_extension);
    RUN_TEST(validate_path_not_found);
    RUN_TEST(validate_mixed_errors);
    RUN_TEST(validate_directory_discovery);
    RUN_TEST(validate_summary_line_format);
}
