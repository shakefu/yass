/*
 * test_discover.c - Tests for file discovery and project root finding
 */
#include "tinytest.h"
#include "../src/discover.h"
#include "../src/path.h"

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

static void touch(const char *path)
{
    FILE *f = fopen(path, "w");
    if (f)
        fclose(f);
}

/* ---- find_project_root tests ---- */

TEST(root_git_dir)
{
    const char *base = "/tmp/yass_test_discover_root_git";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/a/b/c", base);
    mkdirp(subdir);

    char *root = find_project_root(subdir);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, base);
    free(root);

    rmtree(base);
}

TEST(root_git_in_start_dir)
{
    const char *base = "/tmp/yass_test_discover_root_git_start";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* Starting dir itself contains .git */
    char *root = find_project_root(base);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, base);
    free(root);

    rmtree(base);
}

TEST(root_yass_yaml_fallback)
{
    const char *base = "/tmp/yass_test_discover_root_yass";
    rmtree(base);
    mkdirp(base);

    /* No .git, but a .yass.yaml file */
    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/foo.yass.yaml", base);
    touch(spec);

    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/sub", base);
    mkdirp(subdir);

    char *root = find_project_root(subdir);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, base);
    free(root);

    rmtree(base);
}

TEST(root_yass_yaml_in_start_dir)
{
    const char *base = "/tmp/yass_test_discover_root_yass_start";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/thing.yass.yaml", base);
    touch(spec);

    char *root = find_project_root(base);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, base);
    free(root);

    rmtree(base);
}

TEST(root_no_marker)
{
    const char *base = "/tmp/yass_test_discover_root_none";
    rmtree(base);
    mkdirp(base);

    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/empty/nest", base);
    mkdirp(subdir);

    /*
     * This will search all the way up to /.  If the test machine itself
     * has a .git at some ancestor this may not return NULL.  We accept
     * that; the important property is that when there really is no
     * marker the function returns NULL.  We test in a deep /tmp path to
     * make it unlikely.
     */
    char *root = find_project_root(subdir);
    /*
     * We cannot guarantee NULL on every machine (host may have .git above /tmp).
     * Just verify it doesn't crash and returns either NULL or a valid string.
     */
    if (root) {
        /* If something was found, it should be an absolute path */
        ASSERT(root[0] == '/');
        free(root);
    }

    rmtree(base);
}

TEST(root_git_takes_priority_over_yass)
{
    const char *base = "/tmp/yass_test_discover_root_prio";
    rmtree(base);
    mkdirp(base);

    /* .git at base */
    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    /* .yass.yaml in a deeper dir */
    char subdir[4096];
    snprintf(subdir, sizeof(subdir), "%s/deep", base);
    mkdirp(subdir);
    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/deep/x.yass.yaml", subdir);
    touch(spec);

    /* Should find the .git root, not the yass.yaml one */
    char deep2[4096];
    snprintf(deep2, sizeof(deep2), "%s/deep", base);
    char *root = find_project_root(deep2);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, base);
    free(root);

    rmtree(base);
}

TEST(root_null_start_dir)
{
    /* Should use cwd without crashing */
    save_cwd();
    const char *base = "/tmp/yass_test_discover_root_null_start";
    rmtree(base);
    mkdirp(base);

    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    chdir(base);
    /* getcwd may resolve symlinks (e.g. /tmp -> /private/tmp on macOS) */
    char real_cwd[4096];
    getcwd(real_cwd, sizeof(real_cwd));

    char *root = find_project_root(NULL);
    ASSERT_NOT_NULL(root);
    ASSERT_STR_EQ(root, real_cwd);
    free(root);

    restore_cwd();
    rmtree(base);
}

/* ---- discover_spec_files tests ---- */

TEST(discover_single_file)
{
    const char *base = "/tmp/yass_test_discover_single";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/my.yass.yaml", base);
    touch(spec);

    discover_result_t dr;
    int rc = discover_spec_files(spec, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 1);
    /* The path should contain the filename */
    ASSERT_NOT_NULL(strstr(dr.paths[0], "my.yass.yaml"));
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_directory_recursive)
{
    const char *base = "/tmp/yass_test_discover_dir";
    rmtree(base);
    mkdirp(base);

    char sub1[4096];
    snprintf(sub1, sizeof(sub1), "%s/a", base);
    mkdirp(sub1);
    char sub2[4096];
    snprintf(sub2, sizeof(sub2), "%s/b", base);
    mkdirp(sub2);

    char f1[4096], f2[4096], f3[4096];
    snprintf(f1, sizeof(f1), "%s/top.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/alpha.yass.yaml", sub1);
    snprintf(f3, sizeof(f3), "%s/beta.yass.yaml", sub2);
    touch(f1);
    touch(f2);
    touch(f3);

    /* Also put a non-matching file */
    char f4[4096];
    snprintf(f4, sizeof(f4), "%s/readme.txt", base);
    touch(f4);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 3);
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_skip_hidden_dirs)
{
    const char *base = "/tmp/yass_test_discover_hidden_dir";
    rmtree(base);
    mkdirp(base);

    char hidden[4096];
    snprintf(hidden, sizeof(hidden), "%s/.hidden", base);
    mkdirp(hidden);

    char visible[4096];
    snprintf(visible, sizeof(visible), "%s/visible", base);
    mkdirp(visible);

    char f1[4096], f2[4096];
    snprintf(f1, sizeof(f1), "%s/.hidden/secret.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/visible/found.yass.yaml", base);
    touch(f1);
    touch(f2);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 1);
    ASSERT_NOT_NULL(strstr(dr.paths[0], "found.yass.yaml"));
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_skip_hidden_files)
{
    const char *base = "/tmp/yass_test_discover_hidden_file";
    rmtree(base);
    mkdirp(base);

    /* A hidden file that ends with .yass.yaml */
    char f1[4096];
    snprintf(f1, sizeof(f1), "%s/.hidden.yass.yaml", base);
    touch(f1);

    /* A visible file */
    char f2[4096];
    snprintf(f2, sizeof(f2), "%s/visible.yass.yaml", base);
    touch(f2);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 1);
    ASSERT_NOT_NULL(strstr(dr.paths[0], "visible.yass.yaml"));
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_bad_extension)
{
    const char *base = "/tmp/yass_test_discover_bad_ext";
    rmtree(base);
    mkdirp(base);

    char f1[4096];
    snprintf(f1, sizeof(f1), "%s/plain.yaml", base);
    touch(f1);

    discover_result_t dr;
    int rc = discover_spec_files(f1, &dr);
    ASSERT_EQ(rc, 2);
    ASSERT_EQ(dr.count, 0);
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_not_found)
{
    discover_result_t dr;
    int rc = discover_spec_files("/tmp/yass_test_discover_no_such_path_xyz", &dr);
    ASSERT_EQ(rc, 2);
    ASSERT_EQ(dr.count, 0);
    discover_result_free(&dr);
}

TEST(discover_sort_order)
{
    const char *base = "/tmp/yass_test_discover_sort";
    rmtree(base);
    mkdirp(base);

    save_cwd();
    chdir(base);

    char dz[4096], da[4096], dm[4096];
    snprintf(dz, sizeof(dz), "%s/z_dir", base);
    snprintf(da, sizeof(da), "%s/a_dir", base);
    snprintf(dm, sizeof(dm), "%s/m_dir", base);
    mkdirp(dz);
    mkdirp(da);
    mkdirp(dm);

    char f1[4096], f2[4096], f3[4096];
    snprintf(f1, sizeof(f1), "%s/z_dir/z.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/a_dir/a.yass.yaml", base);
    snprintf(f3, sizeof(f3), "%s/m_dir/m.yass.yaml", base);
    touch(f1);
    touch(f2);
    touch(f3);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 3);

    /* Should be sorted: a_dir/a < m_dir/m < z_dir/z */
    ASSERT_NOT_NULL(strstr(dr.paths[0], "a_dir"));
    ASSERT_NOT_NULL(strstr(dr.paths[1], "m_dir"));
    ASSERT_NOT_NULL(strstr(dr.paths[2], "z_dir"));

    discover_result_free(&dr);
    restore_cwd();
    rmtree(base);
}

TEST(discover_bare_yass_yaml_skipped)
{
    const char *base = "/tmp/yass_test_discover_bare";
    rmtree(base);
    mkdirp(base);

    /* .yass.yaml with no prefix should NOT match */
    char f1[4096];
    snprintf(f1, sizeof(f1), "%s/.yass.yaml", base);
    touch(f1);

    /* But one with a prefix should */
    char f2[4096];
    snprintf(f2, sizeof(f2), "%s/valid.yass.yaml", base);
    touch(f2);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 1);
    ASSERT_NOT_NULL(strstr(dr.paths[0], "valid.yass.yaml"));
    discover_result_free(&dr);

    rmtree(base);
}

/* ---- discover_from_args tests ---- */

TEST(discover_dedup)
{
    const char *base = "/tmp/yass_test_discover_dedup";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/dup.yass.yaml", base);
    touch(spec);

    /* Pass the same file twice */
    char *args[2];
    args[0] = spec;
    args[1] = spec;

    discover_result_t dr;
    int rc = discover_from_args(args, 2, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 1);
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_from_args_multiple)
{
    const char *base = "/tmp/yass_test_discover_multi";
    rmtree(base);
    mkdirp(base);

    char f1[4096], f2[4096];
    snprintf(f1, sizeof(f1), "%s/one.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/two.yass.yaml", base);
    touch(f1);
    touch(f2);

    char *args[2];
    args[0] = f1;
    args[1] = f2;

    discover_result_t dr;
    int rc = discover_from_args(args, 2, &dr);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(dr.count, 2);
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_skip_symlinks_in_recursion)
{
    const char *base = "/tmp/yass_test_discover_symlink_recurse";
    rmtree(base);
    mkdirp(base);

    /* Create a real directory with a spec file */
    char real_dir[4096];
    snprintf(real_dir, sizeof(real_dir), "%s/real", base);
    mkdirp(real_dir);

    char real_file[4096];
    snprintf(real_file, sizeof(real_file), "%s/real/real.yass.yaml", base);
    touch(real_file);

    /* Create a symlink to a directory within the tree */
    char link_dir[4096];
    snprintf(link_dir, sizeof(link_dir), "%s/link_to_real", base);
    symlink(real_dir, link_dir);

    /* Create a symlink to a file within the tree */
    char link_file[4096];
    snprintf(link_file, sizeof(link_file), "%s/link.yass.yaml", base);
    symlink(real_file, link_file);

    discover_result_t dr;
    int rc = discover_spec_files(base, &dr);
    ASSERT_EQ(rc, 0);
    /* Should find only the real file, not the symlinks */
    ASSERT_EQ(dr.count, 1);
    ASSERT_NOT_NULL(strstr(dr.paths[0], "real.yass.yaml"));
    discover_result_free(&dr);

    rmtree(base);
}

TEST(discover_result_free_null_safe)
{
    /* Should not crash */
    discover_result_free(NULL);

    discover_result_t dr;
    dr.paths = NULL;
    dr.count = 0;
    dr.error_count = 0;
    discover_result_free(&dr);
}

/* ---- Suite registration ---- */

void run_suite_discover(void)
{
    /* find_project_root */
    RUN_TEST(root_git_dir);
    RUN_TEST(root_git_in_start_dir);
    RUN_TEST(root_yass_yaml_fallback);
    RUN_TEST(root_yass_yaml_in_start_dir);
    RUN_TEST(root_no_marker);
    RUN_TEST(root_git_takes_priority_over_yass);
    RUN_TEST(root_null_start_dir);

    /* discover_spec_files */
    RUN_TEST(discover_single_file);
    RUN_TEST(discover_directory_recursive);
    RUN_TEST(discover_skip_hidden_dirs);
    RUN_TEST(discover_skip_hidden_files);
    RUN_TEST(discover_bad_extension);
    RUN_TEST(discover_not_found);
    RUN_TEST(discover_sort_order);
    RUN_TEST(discover_bare_yass_yaml_skipped);
    RUN_TEST(discover_skip_symlinks_in_recursion);

    /* discover_from_args */
    RUN_TEST(discover_dedup);
    RUN_TEST(discover_from_args_multiple);

    /* discover_result_free */
    RUN_TEST(discover_result_free_null_safe);
}
