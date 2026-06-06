/*
 * discover.c - File discovery and project root finding
 */
#include "discover.h"
#include "error.h"
#include "glob.h"
#include "path.h"
#include "utf8.h"

#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

/* ---- helpers ---- */

/*
 * Check if a directory contains a .git entry (file or directory).
 * Uses stat() so both .git dirs and .git files (worktrees) are detected.
 */
static bool dir_has_git(const char *dir)
{
    char *git_path = path_join(dir, ".git");
    if (!git_path)
        return false;
    struct stat st;
    bool found = (stat(git_path, &st) == 0);
    free(git_path);
    return found;
}

/*
 * Check if a directory contains any file matching *.yass.yaml.
 * Uses opendir/readdir to scan entries.
 */
static bool dir_has_yass_yaml(const char *dir)
{
    DIR *dp = opendir(dir);
    if (!dp)
        return false;
    struct dirent *ent;
    while ((ent = readdir(dp)) != NULL) {
        if (path_has_yass_suffix(ent->d_name)) {
            closedir(dp);
            return true;
        }
    }
    closedir(dp);
    return false;
}

/*
 * Get the parent directory of an absolute path.
 * Returns NULL if path is already "/".
 * Returns a malloc'd string.
 */
static char *parent_dir(const char *absdir)
{
    if (!absdir || strcmp(absdir, "/") == 0)
        return NULL;
    /* Strip trailing slashes (except root) */
    size_t len = strlen(absdir);
    while (len > 1 && absdir[len - 1] == '/')
        len--;
    /* Find last slash */
    size_t i = len;
    while (i > 0 && absdir[i - 1] != '/')
        i--;
    if (i == 0)
        return NULL; /* should not happen for absolute paths */
    /* i points just after the last slash */
    if (i == 1)
        return strdup("/");
    char *result = malloc(i);
    if (!result)
        return NULL;
    memcpy(result, absdir, i - 1);
    result[i - 1] = '\0';
    return result;
}

/* ---- dynamic path list ---- */

typedef struct {
    char **items;
    size_t count;
    size_t capacity;
} path_list_t;

static void path_list_init(path_list_t *pl)
{
    pl->items = NULL;
    pl->count = 0;
    pl->capacity = 0;
}

static bool path_list_push(path_list_t *pl, char *path)
{
    if (pl->count >= pl->capacity) {
        size_t newcap = pl->capacity == 0 ? 16 : pl->capacity * 2;
        char **newitems = realloc(pl->items, newcap * sizeof(char *));
        if (!newitems)
            return false;
        pl->items = newitems;
        pl->capacity = newcap;
    }
    pl->items[pl->count++] = path;
    return true;
}

static void path_list_free_contents(path_list_t *pl)
{
    for (size_t i = 0; i < pl->count; i++)
        free(pl->items[i]);
    free(pl->items);
    pl->items = NULL;
    pl->count = 0;
    pl->capacity = 0;
}

static int path_list_cmp(const void *a, const void *b)
{
    const char *pa = *(const char **)a;
    const char *pb = *(const char **)b;
    return path_compare_nfc(pa, pb);
}

static void path_list_sort(path_list_t *pl)
{
    if (pl->count > 1)
        qsort(pl->items, pl->count, sizeof(char *), path_list_cmp);
}

/* ---- find_project_root ---- */

char *find_project_root(const char *start_dir)
{
    char *abs = NULL;

    if (!start_dir) {
        char cwd[4096];
        if (!getcwd(cwd, sizeof(cwd)))
            return NULL;
        abs = strdup(cwd);
    } else {
        abs = path_absolute(start_dir);
    }

    if (!abs)
        return NULL;

    /* Pass 1: search upward for .git */
    {
        char *cur = strdup(abs);
        if (!cur) {
            free(abs);
            return NULL;
        }
        for (;;) {
            if (dir_has_git(cur)) {
                free(abs);
                return cur;
            }
            if (strcmp(cur, "/") == 0)
                break;
            char *up = parent_dir(cur);
            free(cur);
            cur = up;
            if (!cur)
                break;
        }
        free(cur);
    }

    /* Pass 2: search upward for any .yass.yaml file */
    {
        char *cur = strdup(abs);
        if (!cur) {
            free(abs);
            return NULL;
        }
        for (;;) {
            if (dir_has_yass_yaml(cur)) {
                free(abs);
                return cur;
            }
            if (strcmp(cur, "/") == 0)
                break;
            char *up = parent_dir(cur);
            free(cur);
            cur = up;
            if (!cur)
                break;
        }
        free(cur);
    }

    free(abs);
    return NULL;
}

/* ---- recursive traversal ---- */

/*
 * Recursively collect .yass.yaml files from dir_path.
 * dir_path must be an absolute path.
 * is_top: true if this is the top-level directory argument.
 */
static int recurse_dir(const char *dir_path, path_list_t *pl,
                       int *error_count, bool is_top)
{
    DIR *dp = opendir(dir_path);
    if (!dp) {
        if (is_top) {
            error_emit(dir_path, 0, EC_PATH_UNREADABLE,
                       "cannot read directory");
            return 2;
        }
        /* During recursion, emit and continue */
        error_emit(dir_path, 0, EC_DISCOVER_DIR_UNREADABLE,
                   "cannot read directory during traversal");
        (*error_count)++;
        return 0;
    }

    struct dirent *ent;
    while ((ent = readdir(dp)) != NULL) {
        const char *name = ent->d_name;

        /* Skip . and .. */
        if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0)
            continue;

        /* Skip hidden entries (name starts with .) */
        if (name[0] == '.')
            continue;

        char *full = path_join(dir_path, name);
        if (!full)
            continue;

        struct lstat_buf;
        struct stat lst;
        if (lstat(full, &lst) != 0) {
            free(full);
            continue;
        }

        /* Skip symlinks during recursion */
        if (S_ISLNK(lst.st_mode)) {
            free(full);
            continue;
        }

        if (S_ISDIR(lst.st_mode)) {
            /* Recurse into subdirectory */
            recurse_dir(full, pl, error_count, false);
        } else if (S_ISREG(lst.st_mode)) {
            /* Check if it's a .yass.yaml file */
            if (path_has_yass_suffix(name)) {
                char *display = path_relative_to_cwd(full);
                if (display)
                    path_list_push(pl, display);
            }
        }

        free(full);
    }

    closedir(dp);
    return 0;
}

/* ---- discover_spec_files ---- */

int discover_spec_files(const char *path, discover_result_t *result)
{
    result->paths = NULL;
    result->count = 0;
    result->error_count = 0;

    const char *effective_path = path;
    char *root = NULL;

    /* If no path given, use project root */
    if (!effective_path) {
        root = find_project_root(NULL);
        if (!root) {
            error_emit(NULL, 0, EC_FINDROOT_NO_MARKER,
                       "no project root found (no .git or .yass.yaml marker)");
            return 2;
        }
        effective_path = root;
    }

    /* Get absolute path (no realpath) */
    char *abs = path_absolute(effective_path);
    if (!abs) {
        free(root);
        return 2;
    }

    /* Stat the path -- use lstat to detect symlinks */
    struct stat lst;
    if (lstat(abs, &lst) != 0) {
        error_emit(effective_path, 0, EC_PATH_NOT_FOUND,
                   "path does not exist");
        free(abs);
        free(root);
        return 2;
    }

    /* If it's a symlink, stat through it to get target type */
    struct stat st;
    bool is_symlink = S_ISLNK(lst.st_mode);
    if (is_symlink) {
        if (stat(abs, &st) != 0) {
            error_emit(effective_path, 0, EC_PATH_NOT_FOUND,
                       "symlink target does not exist");
            free(abs);
            free(root);
            return 2;
        }
    } else {
        st = lst;
    }

    if (S_ISREG(st.st_mode)) {
        /* Single file */
        if (!path_has_yass_suffix(abs)) {
            error_emit(effective_path, 0, EC_PATH_BAD_EXTENSION,
                       "file does not have .yass.yaml extension");
            free(abs);
            free(root);
            return 2;
        }
        char *display = path_relative_to_cwd(abs);
        if (!display)
            display = strdup(abs);
        result->paths = malloc(sizeof(char *));
        if (result->paths) {
            result->paths[0] = display;
            result->count = 1;
        } else {
            free(display);
        }
        free(abs);
        free(root);
        return 0;
    }

    if (S_ISDIR(st.st_mode)) {
        path_list_t pl;
        path_list_init(&pl);

        int rc = recurse_dir(abs, &pl, &result->error_count, true);
        if (rc != 0) {
            path_list_free_contents(&pl);
            free(abs);
            free(root);
            return rc;
        }

        path_list_sort(&pl);

        result->paths = pl.items;
        result->count = pl.count;
        /* Don't free pl.items -- ownership transferred */
        free(abs);
        free(root);
        return 0;
    }

    /* Not a file or directory */
    error_emit(effective_path, 0, EC_PATH_INVALID_TYPE,
               "path is not a regular file or directory");
    free(abs);
    free(root);
    return 2;
}

/* ---- discover_result_free ---- */

void discover_result_free(discover_result_t *result)
{
    if (!result)
        return;
    for (size_t i = 0; i < result->count; i++)
        free(result->paths[i]);
    free(result->paths);
    result->paths = NULL;
    result->count = 0;
    result->error_count = 0;
}

/* ---- discover_from_args ---- */

/*
 * Check if abs_path is already in the dedup list.
 */
static bool dedup_contains(char **dedup, size_t dedup_count, const char *abs)
{
    for (size_t i = 0; i < dedup_count; i++) {
        if (strcmp(dedup[i], abs) == 0)
            return true;
    }
    return false;
}

int discover_from_args(char **paths, int count, discover_result_t *result)
{
    result->paths = NULL;
    result->count = 0;
    result->error_count = 0;

    path_list_t combined;
    path_list_init(&combined);

    /* Track absolute paths for dedup */
    path_list_t dedup;
    path_list_init(&dedup);

    int final_rc = 0;

    for (int i = 0; i < count; i++) {
        const char *arg = paths[i];

        /* Check for glob metacharacters */
        if (path_has_glob_chars(arg)) {
            glob_result_t gr;
            int rc = glob_expand(arg, &gr);
            if (rc != 0) {
                final_rc = rc;
                continue;
            }
            /* Discover each expanded path */
            for (size_t g = 0; g < gr.count; g++) {
                discover_result_t dr;
                rc = discover_spec_files(gr.paths[g], &dr);
                if (rc != 0) {
                    if (final_rc == 0)
                        final_rc = rc;
                    discover_result_free(&dr);
                    continue;
                }
                result->error_count += dr.error_count;
                for (size_t j = 0; j < dr.count; j++) {
                    char *abspath = path_absolute(dr.paths[j]);
                    if (abspath && !dedup_contains(dedup.items, dedup.count,
                                                   abspath)) {
                        path_list_push(&dedup, abspath);
                        path_list_push(&combined, strdup(dr.paths[j]));
                    } else {
                        free(abspath);
                    }
                }
                discover_result_free(&dr);
            }
            glob_result_free(&gr);
        } else {
            discover_result_t dr;
            int rc = discover_spec_files(arg, &dr);
            if (rc != 0) {
                if (final_rc == 0)
                    final_rc = rc;
                discover_result_free(&dr);
                continue;
            }
            result->error_count += dr.error_count;
            for (size_t j = 0; j < dr.count; j++) {
                char *abspath = path_absolute(dr.paths[j]);
                if (abspath && !dedup_contains(dedup.items, dedup.count,
                                               abspath)) {
                    path_list_push(&dedup, abspath);
                    path_list_push(&combined, strdup(dr.paths[j]));
                } else {
                    free(abspath);
                }
            }
            discover_result_free(&dr);
        }
    }

    path_list_sort(&combined);

    result->paths = combined.items;
    result->count = combined.count;

    path_list_free_contents(&dedup);

    return final_rc;
}
