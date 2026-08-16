/*
 * test_glob.c - Tests for glob expansion per cli.ExpandGlob spec
 */
#include "tinytest.h"
#include "../src/glob.h"

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

/* Helper: create a directory (recursive). */
static void mkdirp(const char *path)
{
    char tmp[1024];
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

/* Helper: create an empty file. */
static void touch(const char *path)
{
    FILE *f = fopen(path, "w");
    if (f)
        fclose(f);
}

/* Helper: recursively remove a directory tree. */
static void rmrf(const char *path)
{
    char cmd[1100];
    snprintf(cmd, sizeof(cmd), "rm -rf '%s'", path);
    (void)system(cmd);
}

/* Unique base for each test run. */
static char test_base[256];
static int test_base_counter = 0;

static void setup_test_dir(void)
{
    snprintf(test_base, sizeof(test_base),
             "/tmp/yass_test_glob_%d_%d", (int)getpid(), test_base_counter++);
    rmrf(test_base);
    mkdirp(test_base);
}

static void teardown_test_dir(void)
{
    rmrf(test_base);
}

/* ---- Literal path (no glob chars) returned unchanged ---- */

TEST(glob_literal_path)
{
    glob_result_t result;
    int rc = glob_expand("some/plain/path.txt", &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 1);
    ASSERT_STR_EQ(result.paths[0], "some/plain/path.txt");
    glob_result_free(&result);
}

TEST(glob_literal_absolute)
{
    glob_result_t result;
    int rc = glob_expand("/absolute/literal/path", &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 1);
    ASSERT_STR_EQ(result.paths[0], "/absolute/literal/path");
    glob_result_free(&result);
}

/* ---- Simple * pattern matching ---- */

TEST(glob_star_pattern)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/startest", test_base);
    mkdirp(dir);

    char f1[512], f2[512], f3[512];
    snprintf(f1, sizeof(f1), "%s/alpha.txt", dir);
    snprintf(f2, sizeof(f2), "%s/beta.txt", dir);
    snprintf(f3, sizeof(f3), "%s/gamma.log", dir);
    touch(f1);
    touch(f2);
    touch(f3);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/startest/*.txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 2);
    /* Should be sorted: alpha.txt before beta.txt */
    ASSERT_STR_CONTAINS(result.paths[0], "alpha.txt");
    ASSERT_STR_CONTAINS(result.paths[1], "beta.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

TEST(glob_star_all_files)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/allfiles", test_base);
    mkdirp(dir);

    char f1[512], f2[512];
    snprintf(f1, sizeof(f1), "%s/one", dir);
    snprintf(f2, sizeof(f2), "%s/two", dir);
    touch(f1);
    touch(f2);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/allfiles/*", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 2);
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- ** doublestar pattern ---- */

TEST(glob_doublestar)
{
    setup_test_dir();

    char d1[512], d2[512];
    snprintf(d1, sizeof(d1), "%s/ds/sub1", test_base);
    snprintf(d2, sizeof(d2), "%s/ds/sub2/deep", test_base);
    mkdirp(d1);
    mkdirp(d2);

    char f1[512], f2[512], f3[512];
    snprintf(f1, sizeof(f1), "%s/ds/top.yass.yaml", test_base);
    snprintf(f2, sizeof(f2), "%s/ds/sub1/mid.yass.yaml", test_base);
    snprintf(f3, sizeof(f3), "%s/ds/sub2/deep/bot.yass.yaml", test_base);
    touch(f1);
    touch(f2);
    touch(f3);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/ds/**/*.yass.yaml", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    /* ** matches zero or more segments, finding all three .yass.yaml files.
     * Sorted by full path: sub1/mid < sub2/deep/bot < top */
    ASSERT_EQ(result.count, 3);
    ASSERT_STR_CONTAINS(result.paths[0], "sub1/mid.yass.yaml");
    ASSERT_STR_CONTAINS(result.paths[1], "sub2/deep/bot.yass.yaml");
    ASSERT_STR_CONTAINS(result.paths[2], "top.yass.yaml");
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- ? single char matching ---- */

TEST(glob_question_mark)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/qmark", test_base);
    mkdirp(dir);

    char f1[512], f2[512], f3[512];
    snprintf(f1, sizeof(f1), "%s/a1.txt", dir);
    snprintf(f2, sizeof(f2), "%s/a2.txt", dir);
    snprintf(f3, sizeof(f3), "%s/ab.txt", dir);
    touch(f1);
    touch(f2);
    touch(f3);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/qmark/a?.txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 3);
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- Hidden files skipped ---- */

TEST(glob_hidden_files_skipped)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/hidtest", test_base);
    mkdirp(dir);

    char f1[512], f2[512], f3[512];
    snprintf(f1, sizeof(f1), "%s/visible.txt", dir);
    snprintf(f2, sizeof(f2), "%s/.hidden.txt", dir);
    snprintf(f3, sizeof(f3), "%s/.secret", dir);
    touch(f1);
    touch(f2);
    touch(f3);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/hidtest/*", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 1);
    ASSERT_STR_CONTAINS(result.paths[0], "visible.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

TEST(glob_hidden_dirs_skipped)
{
    setup_test_dir();

    char d1[512], d2[512];
    snprintf(d1, sizeof(d1), "%s/hd/visible", test_base);
    snprintf(d2, sizeof(d2), "%s/hd/.hidden", test_base);
    mkdirp(d1);
    mkdirp(d2);

    char f1[512], f2[512];
    snprintf(f1, sizeof(f1), "%s/hd/visible/file.txt", d1);
    snprintf(f2, sizeof(f2), "%s/hd/.hidden/file.txt", d2);
    /* Fix: use the actual dirs */
    snprintf(f1, sizeof(f1), "%s/file.txt", d1);
    snprintf(f2, sizeof(f2), "%s/file.txt", d2);
    touch(f1);
    touch(f2);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/hd/**/*.txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 1);
    ASSERT_STR_CONTAINS(result.paths[0], "visible/file.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- No match -> error ---- */

TEST(glob_no_match)
{
    setup_test_dir();

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/nonexistent_dir_xyz/*.nothing", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 2);
    ASSERT_EQ(result.count, 0);
    ASSERT_NULL(result.paths);

    teardown_test_dir();
}

/* ---- Sorted results ---- */

TEST(glob_sorted_results)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/sorted", test_base);
    mkdirp(dir);

    /* Create files in non-alphabetical order. */
    char f1[512], f2[512], f3[512], f4[512];
    snprintf(f1, sizeof(f1), "%s/delta.txt", dir);
    snprintf(f2, sizeof(f2), "%s/alpha.txt", dir);
    snprintf(f3, sizeof(f3), "%s/charlie.txt", dir);
    snprintf(f4, sizeof(f4), "%s/bravo.txt", dir);
    touch(f1);
    touch(f2);
    touch(f3);
    touch(f4);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/sorted/*.txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 4);
    ASSERT_STR_CONTAINS(result.paths[0], "alpha.txt");
    ASSERT_STR_CONTAINS(result.paths[1], "bravo.txt");
    ASSERT_STR_CONTAINS(result.paths[2], "charlie.txt");
    ASSERT_STR_CONTAINS(result.paths[3], "delta.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- No symlink following ---- */

TEST(glob_no_symlink_follow)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/symtest", test_base);
    mkdirp(dir);

    char real_file[512], link_file[512];
    snprintf(real_file, sizeof(real_file), "%s/real.txt", dir);
    snprintf(link_file, sizeof(link_file), "%s/link.txt", dir);
    touch(real_file);
    symlink(real_file, link_file);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/symtest/*.txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    /* Only real.txt should appear; link.txt is a symlink and should be skipped. */
    ASSERT_EQ(result.count, 1);
    ASSERT_STR_CONTAINS(result.paths[0], "real.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- Bracket expression ---- */

TEST(glob_bracket_expression)
{
    setup_test_dir();

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/bracket", test_base);
    mkdirp(dir);

    char f1[512], f2[512], f3[512], f4[512];
    snprintf(f1, sizeof(f1), "%s/file1.txt", dir);
    snprintf(f2, sizeof(f2), "%s/file2.txt", dir);
    snprintf(f3, sizeof(f3), "%s/file3.txt", dir);
    snprintf(f4, sizeof(f4), "%s/fileA.txt", dir);
    touch(f1);
    touch(f2);
    touch(f3);
    touch(f4);

    char pattern[512];
    snprintf(pattern, sizeof(pattern), "%s/bracket/file[1-2].txt", test_base);

    glob_result_t result;
    int rc = glob_expand(pattern, &result);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(result.count, 2);
    ASSERT_STR_CONTAINS(result.paths[0], "file1.txt");
    ASSERT_STR_CONTAINS(result.paths[1], "file2.txt");
    glob_result_free(&result);

    teardown_test_dir();
}

/* ---- glob_result_free on empty ---- */

TEST(glob_result_free_empty)
{
    glob_result_t result;
    result.paths = NULL;
    result.count = 0;
    /* Should not crash. */
    glob_result_free(&result);
    ASSERT_NULL(result.paths);
    ASSERT_EQ(result.count, 0);
}

TEST(glob_result_free_null)
{
    /* Should not crash. */
    glob_result_free(NULL);
    ASSERT_TRUE(1);
}

/* ---- Suite registration ---- */

void run_suite_glob(void)
{
    /* literal */
    RUN_TEST(glob_literal_path);
    RUN_TEST(glob_literal_absolute);

    /* star */
    RUN_TEST(glob_star_pattern);
    RUN_TEST(glob_star_all_files);

    /* doublestar */
    RUN_TEST(glob_doublestar);

    /* question mark */
    RUN_TEST(glob_question_mark);

    /* hidden */
    RUN_TEST(glob_hidden_files_skipped);
    RUN_TEST(glob_hidden_dirs_skipped);

    /* no match */
    RUN_TEST(glob_no_match);

    /* sorted */
    RUN_TEST(glob_sorted_results);

    /* symlinks */
    RUN_TEST(glob_no_symlink_follow);

    /* bracket */
    RUN_TEST(glob_bracket_expression);

    /* free */
    RUN_TEST(glob_result_free_empty);
    RUN_TEST(glob_result_free_null);
}
