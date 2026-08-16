/*
 * test_query.c - Tests for the query subcommand
 */
#include "tinytest.h"
#include "query.h"
#include "yaml_parse.h"
#include "error.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

/* ---- helpers ---- */

static void write_file(const char *path, const char *content) {
    FILE *f = fopen(path, "w");
    if (f) { fputs(content, f); fclose(f); }
}

static void mkdirp(const char *path) {
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

static const char *test_dir = "/tmp/yass_test_query";

static void setup_query_test(void) {
    system("rm -rf /tmp/yass_test_query");
    mkdirp(test_dir);
    mkdirp("/tmp/yass_test_query/.git");

    write_file("/tmp/yass_test_query/sample.yass.yaml",
        "---\n"
        "description: Sample spec file\n"
        "version: v1\n"
        "---\n"
        "spec: pkg.Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: pkg.Bar\n"
        "RETURN:\n"
        "- MUST: do something else\n"
    );
}

static void cleanup_query_test(void) {
    system("rm -rf /tmp/yass_test_query");
}

/*
 * Capture stdout from cmd_query into a buffer.
 * Returns the exit code. Sets *out to a malloc'd string (caller frees).
 */
static int capture_query(int argc, char **argv, char **out) {
    fflush(stdout);

    char tmpf[] = "/tmp/yass_test_query_out_XXXXXX";
    int fd = mkstemp(tmpf);
    if (fd < 0) {
        *out = strdup("");
        return -1;
    }

    int saved_stdout = dup(STDOUT_FILENO);
    dup2(fd, STDOUT_FILENO);
    close(fd);

    int rc = cmd_query(argc, argv);

    fflush(stdout);
    dup2(saved_stdout, STDOUT_FILENO);
    close(saved_stdout);

    FILE *f = fopen(tmpf, "r");
    if (!f) {
        *out = strdup("");
        unlink(tmpf);
        return rc;
    }
    fseek(f, 0, SEEK_END);
    long len = ftell(f);
    fseek(f, 0, SEEK_SET);
    char *buf = malloc((size_t)len + 1);
    if (buf) {
        size_t nread = fread(buf, 1, (size_t)len, f);
        buf[nread] = '\0';
    } else {
        buf = strdup("");
    }
    fclose(f);
    unlink(tmpf);

    *out = buf;
    return rc;
}

/*
 * Capture both stdout and stderr from cmd_query.
 */
static int capture_query_both(int argc, char **argv, char **out_stdout,
                              char **out_stderr) {
    fflush(stdout);
    fflush(stderr);

    char tmpout[] = "/tmp/yass_test_query_out_XXXXXX";
    char tmperr[] = "/tmp/yass_test_query_err_XXXXXX";
    int fd_out = mkstemp(tmpout);
    int fd_err = mkstemp(tmperr);
    if (fd_out < 0 || fd_err < 0) {
        *out_stdout = strdup("");
        *out_stderr = strdup("");
        return -1;
    }

    int saved_stdout = dup(STDOUT_FILENO);
    int saved_stderr = dup(STDERR_FILENO);
    dup2(fd_out, STDOUT_FILENO);
    dup2(fd_err, STDERR_FILENO);
    close(fd_out);
    close(fd_err);

    int rc = cmd_query(argc, argv);

    fflush(stdout);
    fflush(stderr);
    dup2(saved_stdout, STDOUT_FILENO);
    dup2(saved_stderr, STDERR_FILENO);
    close(saved_stdout);
    close(saved_stderr);

    /* Read stdout */
    FILE *f = fopen(tmpout, "r");
    if (f) {
        fseek(f, 0, SEEK_END);
        long len = ftell(f);
        fseek(f, 0, SEEK_SET);
        char *buf = malloc((size_t)len + 1);
        if (buf) {
            size_t nread = fread(buf, 1, (size_t)len, f);
            buf[nread] = '\0';
        } else {
            buf = strdup("");
        }
        fclose(f);
        *out_stdout = buf;
    } else {
        *out_stdout = strdup("");
    }
    unlink(tmpout);

    /* Read stderr */
    f = fopen(tmperr, "r");
    if (f) {
        fseek(f, 0, SEEK_END);
        long len = ftell(f);
        fseek(f, 0, SEEK_SET);
        char *buf = malloc((size_t)len + 1);
        if (buf) {
            size_t nread = fread(buf, 1, (size_t)len, f);
            buf[nread] = '\0';
        } else {
            buf = strdup("");
        }
        fclose(f);
        *out_stderr = buf;
    } else {
        *out_stderr = strdup("");
    }
    unlink(tmperr);

    return rc;
}

/* ---- tests ---- */

/* Query with no name -> yass.query.name_missing, exit 2 */
TEST(query_no_name) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(0, NULL, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.name_missing");
    /* No stdout output */
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query with blank (empty) name -> yass.query.name_blank, exit 2 */
TEST(query_blank_name) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {""};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.name_blank");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query with whitespace-only name -> yass.query.name_blank, exit 2 */
TEST(query_whitespace_only_name) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"   "};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.name_blank");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query with whitespace-containing name -> no-match (exit 1), not blank */
TEST(query_name_with_whitespace) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"foo bar"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 1);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.no_match");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query for non-existent spec -> yass.query.no_match, exit 1 */
TEST(query_no_match) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"NonExistentSpec"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 1);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.no_match");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query with exact match -> emits YAML fragment, exit 0 */
TEST(query_exact_match) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"pkg.Foo"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Must begin with --- */
    ASSERT_STR_CONTAINS(out, "---\n");
    /* Must contain the spec name */
    ASSERT_STR_CONTAINS(out, "spec: pkg.Foo");
    /* Must contain the obligation */
    ASSERT_STR_CONTAINS(out, "MUST: do something");
    /* Must end with exactly one trailing LF */
    size_t len = strlen(out);
    ASSERT(len > 0);
    ASSERT_EQ(out[len - 1], '\n');
    if (len > 1) ASSERT_NEQ(out[len - 2], '\n');

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Query with suffix match: "Foo" matches "pkg.Foo" */
TEST(query_suffix_match) {
    setup_query_test();
    /* Create a file with only one spec matching "Foo" */
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");
    write_file("/tmp/yass_test_query/single.yass.yaml",
        "---\n"
        "description: Single spec\n"
        "version: v1\n"
        "---\n"
        "spec: pkg.Foo\n"
        "RETURN:\n"
        "- MUST: do something\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Foo"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "spec: pkg.Foo");

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Multi-component suffix: "query.Foo" matches "cli.query.Foo" */
TEST(query_multi_component_suffix) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");
    write_file("/tmp/yass_test_query/deep.yass.yaml",
        "---\n"
        "description: Deep spec\n"
        "version: v1\n"
        "---\n"
        "spec: cli.query.Foo\n"
        "RETURN:\n"
        "- MUST: do deep thing\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"query.Foo"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "spec: cli.query.Foo");

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Partial substring must NOT match: "Extract" must NOT match
 * "cli.query.ExtractFragment" */
TEST(query_no_partial_substring) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");
    write_file("/tmp/yass_test_query/partial.yass.yaml",
        "---\n"
        "description: Partial test\n"
        "version: v1\n"
        "---\n"
        "spec: cli.query.ExtractFragment\n"
        "RETURN:\n"
        "- MUST: extract\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Extract"};
    int rc = cmd_query(1, args);
    ASSERT_EQ(rc, 1);  /* no-match */

    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Multiple matches -> disambiguation rows, exit 0 */
TEST(query_multiple_matches) {
    setup_query_test();
    write_file("/tmp/yass_test_query/other.yass.yaml",
        "---\n"
        "description: Other spec file\n"
        "version: v1\n"
        "---\n"
        "spec: other.Foo\n"
        "RETURN:\n"
        "- MUST: do other thing\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Foo"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 0);
    /* Disambiguation rows to stdout */
    ASSERT_STR_CONTAINS(out_stdout, "pkg.Foo");
    ASSERT_STR_CONTAINS(out_stdout, "other.Foo");
    /* Tab-separated format */
    ASSERT_STR_CONTAINS(out_stdout, "\t");
    /* No stderr output for multi-match */
    ASSERT_STR_EQ(out_stderr, "");
    /* No --- (no YAML fragment) */
    ASSERT(strstr(out_stdout, "---") == NULL);

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Scope not found -> yass.query.scope_not_found, exit 2 */
TEST(query_scope_not_found) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Foo", "/tmp/nonexistent_scope_dir_xyz"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(2, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.scope_not_found");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Colon in scope path -> yass.path.colon_in_path, exit 2 */
TEST(query_colon_in_scope) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Foo", "path:with:colon"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(2, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.path.colon_in_path");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Scope exists but has no .yass.yaml files -> yass.query.scope_empty, exit 2 */
TEST(query_scope_empty) {
    setup_query_test();
    mkdirp("/tmp/yass_test_query/empty_scope");

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"Foo", "/tmp/yass_test_query/empty_scope"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(2, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.scope_empty");
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* CONFORMS inlining: reference-only obligation is replaced with
 * inlined obligations from the referenced slot */
TEST(query_conforms_inline) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");

    /* Create the referenced spec file */
    write_file("/tmp/yass_test_query/base.yass.yaml",
        "---\n"
        "description: Base spec\n"
        "version: v1\n"
        "---\n"
        "spec: base.Contract\n"
        "RETURN:\n"
        "- MUST: return valid data\n"
        "- SHOULD: be fast\n"
    );

    /* Create the spec that CONFORMS to the base */
    write_file("/tmp/yass_test_query/derived.yass.yaml",
        "---\n"
        "description: Derived spec\n"
        "version: v1\n"
        "---\n"
        "spec: derived.Impl\n"
        "RETURN:\n"
        "- MUST: do its own thing\n"
        "- CONFORMS: ./base@base.Contract::RETURN\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"derived.Impl"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    /* The output should contain the spec */
    ASSERT_STR_CONTAINS(out, "spec: derived.Impl");
    /* The carrier obligation should be kept */
    ASSERT_STR_CONTAINS(out, "MUST: do its own thing");
    /* Inlined obligations from base.Contract::RETURN */
    ASSERT_STR_CONTAINS(out, "MUST: return valid data");
    ASSERT_STR_CONTAINS(out, "SHOULD: be fast");
    /* Provenance comment */
    ASSERT_STR_CONTAINS(out, "# CONFORMS: ./base@base.Contract::RETURN");
    /* The CONFORMS ref itself should be stripped (not in output) */
    ASSERT(strstr(out, "CONFORMS: ./base@base.Contract::RETURN\n") == NULL ||
           strstr(out, "# CONFORMS:") != NULL);

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* CONFORMS inlining with normative carrier: carrier kept, inlined appended */
TEST(query_conforms_normative_carrier) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");

    write_file("/tmp/yass_test_query/base.yass.yaml",
        "---\n"
        "description: Base spec\n"
        "version: v1\n"
        "---\n"
        "spec: base.Rules\n"
        "RETURN:\n"
        "- MUST: follow rule one\n"
    );

    write_file("/tmp/yass_test_query/user.yass.yaml",
        "---\n"
        "description: User spec\n"
        "version: v1\n"
        "---\n"
        "spec: user.Feature\n"
        "RETURN:\n"
        "- MUST: do feature work\n"
        "  CONFORMS: ./base@base.Rules::RETURN\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"user.Feature"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    /* Carrier obligation should be kept (with CONFORMS stripped) */
    ASSERT_STR_CONTAINS(out, "MUST: do feature work");
    /* Inlined obligation appended after carrier */
    ASSERT_STR_CONTAINS(out, "MUST: follow rule one");
    /* Provenance comment */
    ASSERT_STR_CONTAINS(out, "# CONFORMS: ./base@base.Rules::RETURN");

    /* Verify order: carrier before inlined */
    char *pos_carrier = strstr(out, "do feature work");
    char *pos_inlined = strstr(out, "follow rule one");
    ASSERT_NOT_NULL(pos_carrier);
    ASSERT_NOT_NULL(pos_inlined);
    ASSERT(pos_carrier < pos_inlined);

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* CONFORMS ref without ::SLOT suffix -> yass.query.conforms_no_slot, exit 1 */
TEST(query_conforms_no_slot) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");

    write_file("/tmp/yass_test_query/base.yass.yaml",
        "---\n"
        "description: Base spec\n"
        "version: v1\n"
        "---\n"
        "spec: base.Contract\n"
        "RETURN:\n"
        "- MUST: return valid data\n"
    );

    /* CONFORMS ref without ::SLOT */
    write_file("/tmp/yass_test_query/noslot.yass.yaml",
        "---\n"
        "description: No slot spec\n"
        "version: v1\n"
        "---\n"
        "spec: noslot.Feature\n"
        "RETURN:\n"
        "- CONFORMS: ./base@base.Contract\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"noslot.Feature"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 1);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.conforms_no_slot");
    /* No fragment emitted on failure */
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* CONFORMS with unresolvable ref -> yass.query.conforms_unresolved, exit 1 */
TEST(query_conforms_unresolved) {
    setup_query_test();
    system("rm -f /tmp/yass_test_query/sample.yass.yaml");

    write_file("/tmp/yass_test_query/unresolved.yass.yaml",
        "---\n"
        "description: Unresolved ref spec\n"
        "version: v1\n"
        "---\n"
        "spec: unresolved.Feature\n"
        "RETURN:\n"
        "- CONFORMS: ./nonexistent@nonexistent.Spec::RETURN\n"
    );

    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"unresolved.Feature"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 1);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.conforms_unresolved");
    /* No partial fragment on failure */
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Scope validation happens BEFORE name lookup: invalid scope with
 * would-be-no-match name should show scope error, not name error */
TEST(query_scope_validated_before_name) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"WouldNotMatch", "/tmp/nonexistent_scope_dir_xyz"};
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_query_both(2, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.query.scope_not_found");
    /* Should NOT contain no_match - scope error takes priority */
    ASSERT(strstr(out_stderr, "yass.query.no_match") == NULL);
    ASSERT_STR_EQ(out_stdout, "");

    free(out_stdout);
    free(out_stderr);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* Output starts with --- and ends with exactly one LF */
TEST(query_output_format) {
    setup_query_test();
    char *saved_cwd = getcwd(NULL, 0);
    if (chdir(test_dir) != 0) { free(saved_cwd); return; }

    char *args[] = {"pkg.Foo"};
    char *out = NULL;
    int rc = capture_query(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    /* Starts with --- */
    ASSERT(strncmp(out, "---\n", 4) == 0);

    /* Ends with exactly one trailing LF */
    size_t len = strlen(out);
    ASSERT(len >= 2);
    ASSERT_EQ(out[len - 1], '\n');
    ASSERT_NEQ(out[len - 2], '\n');

    /* No trailing ... */
    ASSERT(strstr(out, "...") == NULL);

    free(out);
    chdir(saved_cwd); free(saved_cwd);
    cleanup_query_test();
}

/* ---- Suite registration ---- */

void run_suite_query(void) {
    RUN_TEST(query_no_name);
    RUN_TEST(query_blank_name);
    RUN_TEST(query_whitespace_only_name);
    RUN_TEST(query_name_with_whitespace);
    RUN_TEST(query_no_match);
    RUN_TEST(query_exact_match);
    RUN_TEST(query_suffix_match);
    RUN_TEST(query_multi_component_suffix);
    RUN_TEST(query_no_partial_substring);
    RUN_TEST(query_multiple_matches);
    RUN_TEST(query_scope_not_found);
    RUN_TEST(query_colon_in_scope);
    RUN_TEST(query_scope_empty);
    RUN_TEST(query_conforms_inline);
    RUN_TEST(query_conforms_normative_carrier);
    RUN_TEST(query_conforms_no_slot);
    RUN_TEST(query_conforms_unresolved);
    RUN_TEST(query_scope_validated_before_name);
    RUN_TEST(query_output_format);
}
