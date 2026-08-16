/*
 * test_path.c - Tests for path utilities
 */
#include "tinytest.h"
#include "../src/path.h"

#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* ---- path_has_yass_suffix ---- */

TEST(suffix_valid)
{
    ASSERT_TRUE(path_has_yass_suffix("foo.yass.yaml"));
    ASSERT_TRUE(path_has_yass_suffix("dir/bar.yass.yaml"));
    ASSERT_TRUE(path_has_yass_suffix("/absolute/path/baz.yass.yaml"));
    ASSERT_TRUE(path_has_yass_suffix("a.yass.yaml"));
}

TEST(suffix_yaml_only)
{
    ASSERT_FALSE(path_has_yass_suffix("foo.yaml"));
    ASSERT_FALSE(path_has_yass_suffix("foo.yml"));
    ASSERT_FALSE(path_has_yass_suffix("config.yaml"));
}

TEST(suffix_bare)
{
    /* Bare .yass.yaml with no prefix in basename must NOT match */
    ASSERT_FALSE(path_has_yass_suffix(".yass.yaml"));
    ASSERT_FALSE(path_has_yass_suffix("dir/.yass.yaml"));
    ASSERT_FALSE(path_has_yass_suffix("/path/to/.yass.yaml"));
}

TEST(suffix_empty_and_null)
{
    ASSERT_FALSE(path_has_yass_suffix(""));
    ASSERT_FALSE(path_has_yass_suffix(NULL));
    ASSERT_FALSE(path_has_yass_suffix("yass.yaml"));
}

TEST(suffix_case_sensitive)
{
    ASSERT_FALSE(path_has_yass_suffix("foo.YASS.YAML"));
    ASSERT_FALSE(path_has_yass_suffix("foo.Yass.Yaml"));
    ASSERT_FALSE(path_has_yass_suffix("foo.yass.YAML"));
}

/* ---- path_has_colon ---- */

TEST(colon_present)
{
    ASSERT_TRUE(path_has_colon("foo:bar"));
    ASSERT_TRUE(path_has_colon(":leading"));
    ASSERT_TRUE(path_has_colon("trailing:"));
    ASSERT_TRUE(path_has_colon("a:b:c"));
}

TEST(colon_absent)
{
    ASSERT_FALSE(path_has_colon("foobar"));
    ASSERT_FALSE(path_has_colon("/path/to/file"));
    ASSERT_FALSE(path_has_colon(""));
    ASSERT_FALSE(path_has_colon(NULL));
}

/* ---- path_basename ---- */

TEST(basename_with_slash)
{
    ASSERT_STR_EQ(path_basename("/foo/bar/baz.txt"), "baz.txt");
    ASSERT_STR_EQ(path_basename("dir/file.c"), "file.c");
    ASSERT_STR_EQ(path_basename("/file"), "file");
}

TEST(basename_no_slash)
{
    ASSERT_STR_EQ(path_basename("hello.txt"), "hello.txt");
    ASSERT_STR_EQ(path_basename("name"), "name");
}

TEST(basename_null)
{
    ASSERT_NULL(path_basename(NULL));
}

/* ---- path_dirname ---- */

TEST(dirname_with_slash)
{
    char *d1 = path_dirname("/foo/bar/baz.txt");
    ASSERT_STR_EQ(d1, "/foo/bar");
    free(d1);

    char *d2 = path_dirname("dir/file.c");
    ASSERT_STR_EQ(d2, "dir");
    free(d2);
}

TEST(dirname_no_slash)
{
    char *d = path_dirname("file.txt");
    ASSERT_STR_EQ(d, ".");
    free(d);
}

TEST(dirname_root)
{
    char *d = path_dirname("/file");
    ASSERT_STR_EQ(d, "/");
    free(d);
}

TEST(dirname_null)
{
    char *d = path_dirname(NULL);
    ASSERT_STR_EQ(d, ".");
    free(d);
}

/* ---- path_join ---- */

TEST(join_basic)
{
    char *j = path_join("/foo", "bar.txt");
    ASSERT_STR_EQ(j, "/foo/bar.txt");
    free(j);
}

TEST(join_trailing_slash)
{
    char *j = path_join("/foo/", "bar.txt");
    ASSERT_STR_EQ(j, "/foo/bar.txt");
    free(j);
}

TEST(join_empty_dir)
{
    char *j = path_join("", "bar.txt");
    ASSERT_STR_EQ(j, "bar.txt");
    free(j);
}

TEST(join_null)
{
    ASSERT_NULL(path_join(NULL, "bar"));
    ASSERT_NULL(path_join("foo", NULL));
    ASSERT_NULL(path_join(NULL, NULL));
}

/* ---- path_relative_to_cwd ---- */

TEST(relative_under_cwd)
{
    char cwd[4096];
    getcwd(cwd, sizeof(cwd));

    /* Construct an absolute path under cwd */
    char *abs = path_join(cwd, "subdir/file.txt");
    char *rel = path_relative_to_cwd(abs);
    ASSERT_STR_EQ(rel, "subdir/file.txt");
    free(abs);
    free(rel);
}

TEST(relative_is_cwd)
{
    char cwd[4096];
    getcwd(cwd, sizeof(cwd));

    char *rel = path_relative_to_cwd(cwd);
    ASSERT_STR_EQ(rel, ".");
    free(rel);
}

TEST(relative_outside_cwd)
{
    char *rel = path_relative_to_cwd("/some/other/directory/file.txt");
    ASSERT_STR_EQ(rel, "/some/other/directory/file.txt");
    free(rel);
}

TEST(relative_already_relative)
{
    /* A relative input gets joined with cwd, then stripped back to relative */
    char *rel = path_relative_to_cwd("foo/bar.txt");
    ASSERT_STR_EQ(rel, "foo/bar.txt");
    free(rel);
}

TEST(relative_basename_in_cwd)
{
    char cwd[4096];
    getcwd(cwd, sizeof(cwd));

    char *abs = path_join(cwd, "justfile.txt");
    char *rel = path_relative_to_cwd(abs);
    ASSERT_STR_EQ(rel, "justfile.txt");
    free(abs);
    free(rel);
}

/* ---- path_absolute ---- */

TEST(absolute_already)
{
    char *a = path_absolute("/foo/bar");
    ASSERT_STR_EQ(a, "/foo/bar");
    free(a);
}

TEST(absolute_relative)
{
    char cwd[4096];
    getcwd(cwd, sizeof(cwd));

    char *expected = path_join(cwd, "foo.txt");
    char *a = path_absolute("foo.txt");
    ASSERT_STR_EQ(a, expected);
    free(expected);
    free(a);
}

TEST(absolute_null)
{
    ASSERT_NULL(path_absolute(NULL));
}

/* ---- path_starts_with ---- */

TEST(starts_with_exact)
{
    ASSERT_TRUE(path_starts_with("/foo/bar", "/foo/bar"));
}

TEST(starts_with_subdir)
{
    ASSERT_TRUE(path_starts_with("/foo/bar/baz", "/foo/bar"));
    ASSERT_TRUE(path_starts_with("/foo/bar/baz/qux", "/foo"));
}

TEST(starts_with_no_match)
{
    ASSERT_FALSE(path_starts_with("/foo/barbaz", "/foo/bar"));
    ASSERT_FALSE(path_starts_with("/abc", "/def"));
}

TEST(starts_with_null)
{
    ASSERT_FALSE(path_starts_with(NULL, "/foo"));
    ASSERT_FALSE(path_starts_with("/foo", NULL));
}

/* ---- path_has_glob_chars ---- */

TEST(glob_star)
{
    ASSERT_TRUE(path_has_glob_chars("*.yaml"));
    ASSERT_TRUE(path_has_glob_chars("dir/*"));
}

TEST(glob_question)
{
    ASSERT_TRUE(path_has_glob_chars("file?.txt"));
}

TEST(glob_bracket)
{
    ASSERT_TRUE(path_has_glob_chars("file[0-9].txt"));
}

TEST(glob_none)
{
    ASSERT_FALSE(path_has_glob_chars("plain_file.txt"));
    ASSERT_FALSE(path_has_glob_chars("/path/to/file"));
    ASSERT_FALSE(path_has_glob_chars(""));
    ASSERT_FALSE(path_has_glob_chars(NULL));
}

/* ---- Suite registration ---- */

void run_suite_path(void)
{
    /* suffix */
    RUN_TEST(suffix_valid);
    RUN_TEST(suffix_yaml_only);
    RUN_TEST(suffix_bare);
    RUN_TEST(suffix_empty_and_null);
    RUN_TEST(suffix_case_sensitive);

    /* colon */
    RUN_TEST(colon_present);
    RUN_TEST(colon_absent);

    /* basename */
    RUN_TEST(basename_with_slash);
    RUN_TEST(basename_no_slash);
    RUN_TEST(basename_null);

    /* dirname */
    RUN_TEST(dirname_with_slash);
    RUN_TEST(dirname_no_slash);
    RUN_TEST(dirname_root);
    RUN_TEST(dirname_null);

    /* join */
    RUN_TEST(join_basic);
    RUN_TEST(join_trailing_slash);
    RUN_TEST(join_empty_dir);
    RUN_TEST(join_null);

    /* relative */
    RUN_TEST(relative_under_cwd);
    RUN_TEST(relative_is_cwd);
    RUN_TEST(relative_outside_cwd);
    RUN_TEST(relative_already_relative);
    RUN_TEST(relative_basename_in_cwd);

    /* absolute */
    RUN_TEST(absolute_already);
    RUN_TEST(absolute_relative);
    RUN_TEST(absolute_null);

    /* starts_with */
    RUN_TEST(starts_with_exact);
    RUN_TEST(starts_with_subdir);
    RUN_TEST(starts_with_no_match);
    RUN_TEST(starts_with_null);

    /* glob */
    RUN_TEST(glob_star);
    RUN_TEST(glob_question);
    RUN_TEST(glob_bracket);
    RUN_TEST(glob_none);
}
