/*
 * glob.c - Glob expansion per cli.ExpandGlob spec
 */
#include "glob.h"
#include "path.h"
#include "error.h"
#include "utf8.h"

#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

/* Dynamic array for collecting paths. */
typedef struct {
    char **items;
    size_t count;
    size_t capacity;
} path_list_t;

static void path_list_init(path_list_t *list)
{
    list->items = NULL;
    list->count = 0;
    list->capacity = 0;
}

static int path_list_push(path_list_t *list, const char *path)
{
    if (list->count >= list->capacity) {
        size_t new_cap = list->capacity == 0 ? 16 : list->capacity * 2;
        char **new_items = realloc(list->items, new_cap * sizeof(char *));
        if (!new_items)
            return -1;
        list->items = new_items;
        list->capacity = new_cap;
    }
    list->items[list->count] = strdup(path);
    if (!list->items[list->count])
        return -1;
    list->count++;
    return 0;
}

static void path_list_free(path_list_t *list)
{
    for (size_t i = 0; i < list->count; i++)
        free(list->items[i]);
    free(list->items);
    list->items = NULL;
    list->count = 0;
    list->capacity = 0;
}

/* Check if a name is hidden (starts with '.'). */
static int is_hidden(const char *name)
{
    return name[0] == '.';
}

/*
 * Match a single segment (filename component) against a glob pattern segment.
 * Handles *, ?, and [...] bracket expressions.
 * Returns 1 on match, 0 otherwise.
 */
static int glob_match_segment(const char *pattern, const char *str)
{
    while (*pattern && *str) {
        if (*pattern == '*') {
            /* Skip consecutive stars within segment. */
            while (*pattern == '*')
                pattern++;
            /* Star at end of pattern matches rest of string. */
            if (!*pattern)
                return 1;
            /* Try matching the rest at each position. */
            while (*str) {
                if (glob_match_segment(pattern, str))
                    return 1;
                str++;
            }
            return glob_match_segment(pattern, str);
        } else if (*pattern == '?') {
            /* ? matches exactly one character (not /). */
            if (*str == '/')
                return 0;
            pattern++;
            str++;
        } else if (*pattern == '[') {
            /* POSIX bracket expression. */
            pattern++; /* skip '[' */
            int negate = 0;
            if (*pattern == '!' || *pattern == '^') {
                negate = 1;
                pattern++;
            }
            int matched = 0;
            int first = 1;
            while (*pattern && (*pattern != ']' || first)) {
                first = 0;
                char lo = *pattern;
                pattern++;
                if (*pattern == '-' && *(pattern + 1) && *(pattern + 1) != ']') {
                    pattern++; /* skip '-' */
                    char hi = *pattern;
                    pattern++;
                    if ((unsigned char)*str >= (unsigned char)lo &&
                        (unsigned char)*str <= (unsigned char)hi)
                        matched = 1;
                } else {
                    if (*str == lo)
                        matched = 1;
                }
            }
            if (*pattern == ']')
                pattern++;
            if (negate)
                matched = !matched;
            if (!matched)
                return 0;
            str++;
        } else {
            /* Literal character match (case-sensitive). */
            if (*pattern != *str)
                return 0;
            pattern++;
            str++;
        }
    }

    /* Handle trailing stars in pattern. */
    while (*pattern == '*')
        pattern++;

    return *pattern == '\0' && *str == '\0';
}

/*
 * Split a pattern into segments by '/'.
 * Returns a malloc'd array of malloc'd strings.
 * Sets *count to the number of segments.
 * Caller must free each segment and the array.
 */
static char **split_segments(const char *pattern, size_t *count)
{
    *count = 0;
    if (!pattern || !*pattern)
        return NULL;

    /* Count segments. */
    size_t n = 1;
    for (const char *p = pattern; *p; p++) {
        if (*p == '/')
            n++;
    }

    char **segs = malloc(n * sizeof(char *));
    if (!segs)
        return NULL;

    size_t idx = 0;
    const char *start = pattern;
    for (const char *p = pattern; ; p++) {
        if (*p == '/' || *p == '\0') {
            size_t len = (size_t)(p - start);
            segs[idx] = malloc(len + 1);
            if (!segs[idx]) {
                for (size_t i = 0; i < idx; i++)
                    free(segs[i]);
                free(segs);
                return NULL;
            }
            memcpy(segs[idx], start, len);
            segs[idx][len] = '\0';
            idx++;
            if (*p == '\0')
                break;
            start = p + 1;
        }
    }
    *count = idx;
    return segs;
}

static void free_segments(char **segs, size_t count)
{
    for (size_t i = 0; i < count; i++)
        free(segs[i]);
    free(segs);
}

/*
 * Check if an entry is a directory (using lstat, no symlink following).
 */
static int is_directory(const char *path)
{
    struct stat st;
    if (lstat(path, &st) != 0)
        return 0;
    return S_ISDIR(st.st_mode);
}

/*
 * Check if a path is a symlink.
 */
static int is_symlink(const char *path)
{
    struct stat st;
    if (lstat(path, &st) != 0)
        return 0;
    return S_ISLNK(st.st_mode);
}

/*
 * Recursively expand glob segments starting from base_dir.
 * segments[seg_idx..seg_count-1] are the remaining segments to match.
 * Collects matching paths into list.
 */
static int glob_expand_segments(const char *base_dir,
                                 char **segments, size_t seg_idx, size_t seg_count,
                                 path_list_t *list)
{
    if (seg_idx >= seg_count) {
        /* All segments consumed; base_dir is a match.
         * But we need to verify the path actually exists. */
        struct stat st;
        if (lstat(base_dir, &st) == 0) {
            /* Skip symlinks. */
            if (S_ISLNK(st.st_mode))
                return 0;
            return path_list_push(list, base_dir);
        }
        return 0;
    }

    const char *seg = segments[seg_idx];

    /* Handle ** (doublestar) segment. */
    if (strcmp(seg, "**") == 0) {
        /* ** matches zero or more path segments. */

        /* Zero segments: skip ** and continue with next segment. */
        if (glob_expand_segments(base_dir, segments, seg_idx + 1, seg_count, list) < 0)
            return -1;

        /* One or more segments: descend into subdirectories. */
        DIR *dir = opendir(base_dir);
        if (!dir)
            return 0;

        struct dirent *entry;
        while ((entry = readdir(dir)) != NULL) {
            const char *name = entry->d_name;

            /* Skip . and .. */
            if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0)
                continue;

            /* Skip hidden files/dirs. */
            if (is_hidden(name))
                continue;

            char *child = path_join(base_dir, name);
            if (!child) {
                closedir(dir);
                return -1;
            }

            /* Skip symlinks. */
            if (is_symlink(child)) {
                free(child);
                continue;
            }

            if (is_directory(child)) {
                /* Recurse: ** can still match more levels from child. */
                if (glob_expand_segments(child, segments, seg_idx, seg_count, list) < 0) {
                    free(child);
                    closedir(dir);
                    return -1;
                }
            }

            free(child);
        }
        closedir(dir);
        return 0;
    }

    /* Non-doublestar segment: check if it contains glob metacharacters. */
    int has_meta = path_has_glob_chars(seg);

    if (!has_meta) {
        /* Literal segment: just append and continue. */
        char *child = path_join(base_dir, seg);
        if (!child)
            return -1;

        /* Skip symlinks. */
        if (is_symlink(child)) {
            free(child);
            return 0;
        }

        int rc = glob_expand_segments(child, segments, seg_idx + 1, seg_count, list);
        free(child);
        return rc;
    }

    /* Glob segment: list directory and match. */
    DIR *dir = opendir(base_dir);
    if (!dir)
        return 0;

    struct dirent *entry;
    while ((entry = readdir(dir)) != NULL) {
        const char *name = entry->d_name;

        /* Skip . and .. */
        if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0)
            continue;

        /* Skip hidden files/dirs. */
        if (is_hidden(name))
            continue;

        /* Match against the glob pattern segment. */
        if (!glob_match_segment(seg, name))
            continue;

        char *child = path_join(base_dir, name);
        if (!child) {
            closedir(dir);
            return -1;
        }

        /* Skip symlinks. */
        if (is_symlink(child)) {
            free(child);
            continue;
        }

        /* If there are more segments, child must be a directory. */
        if (seg_idx + 1 < seg_count) {
            if (!is_directory(child)) {
                free(child);
                continue;
            }
        }

        if (glob_expand_segments(child, segments, seg_idx + 1, seg_count, list) < 0) {
            free(child);
            closedir(dir);
            return -1;
        }

        free(child);
    }
    closedir(dir);
    return 0;
}

/* qsort comparison for NFC-sorted paths. */
static int compare_paths(const void *a, const void *b)
{
    const char *pa = *(const char **)a;
    const char *pb = *(const char **)b;
    return path_compare_nfc(pa, pb);
}

int glob_expand(const char *pattern, glob_result_t *result)
{
    result->paths = NULL;
    result->count = 0;

    if (!pattern || !*pattern) {
        error_emit(NULL, 0, EC_GLOB_NO_MATCH, "empty glob pattern");
        return 2;
    }

    /* If no glob metacharacters, return the literal path unchanged. */
    if (!path_has_glob_chars(pattern)) {
        result->paths = malloc(sizeof(char *));
        if (!result->paths)
            return 2;
        result->paths[0] = strdup(pattern);
        if (!result->paths[0]) {
            free(result->paths);
            result->paths = NULL;
            return 2;
        }
        result->count = 1;
        return 0;
    }

    /* Split pattern into segments. */
    size_t seg_count = 0;
    char **segments = split_segments(pattern, &seg_count);
    if (!segments || seg_count == 0) {
        error_emit(NULL, 0, EC_GLOB_NO_MATCH, "invalid glob pattern");
        return 2;
    }

    /* Determine the base directory: find the leading literal segments. */
    size_t first_glob = 0;
    while (first_glob < seg_count && !path_has_glob_chars(segments[first_glob])
           && strcmp(segments[first_glob], "**") != 0) {
        first_glob++;
    }

    /* Build base_dir from leading literal segments. */
    char *base_dir;
    if (pattern[0] == '/') {
        /* Absolute pattern. */
        if (first_glob == 0) {
            base_dir = strdup("/");
        } else {
            /* Reconstruct absolute base from literal segments. */
            base_dir = strdup("/");
            for (size_t i = 0; i < first_glob && base_dir; i++) {
                /* Skip empty segment from leading / split. */
                if (segments[i][0] == '\0')
                    continue;
                char *tmp = path_join(base_dir, segments[i]);
                free(base_dir);
                base_dir = tmp;
            }
        }
    } else {
        /* Relative pattern. */
        if (first_glob == 0) {
            base_dir = strdup(".");
        } else {
            base_dir = strdup(segments[0]);
            for (size_t i = 1; i < first_glob && base_dir; i++) {
                char *tmp = path_join(base_dir, segments[i]);
                free(base_dir);
                base_dir = tmp;
            }
        }
    }

    if (!base_dir) {
        free_segments(segments, seg_count);
        error_emit(NULL, 0, EC_GLOB_NO_MATCH, "memory allocation failure");
        return 2;
    }

    /* Adjust segment index to skip leading empty segment for absolute paths. */
    size_t start_seg = first_glob;
    if (pattern[0] == '/' && seg_count > 0 && segments[0][0] == '\0') {
        /* The split produces an empty first segment for absolute paths.
         * If first_glob was 0, start_seg stays 0, but we skip the empty segment. */
        if (start_seg == 0)
            start_seg = 1;
        /* Also, if first_glob included the empty segment in literal prefix,
         * it was already handled. */
    }

    path_list_t list;
    path_list_init(&list);

    int rc = glob_expand_segments(base_dir, segments, start_seg, seg_count, &list);
    free(base_dir);
    free_segments(segments, seg_count);

    if (rc < 0) {
        path_list_free(&list);
        error_emit(NULL, 0, EC_GLOB_NO_MATCH, "glob expansion failed");
        return 2;
    }

    if (list.count == 0) {
        path_list_free(&list);
        /* Build error message with the pattern. */
        size_t msg_len = strlen("no matches for pattern: ") + strlen(pattern) + 1;
        char *msg = malloc(msg_len);
        if (msg) {
            snprintf(msg, msg_len, "no matches for pattern: %s", pattern);
            error_emit(NULL, 0, EC_GLOB_NO_MATCH, msg);
            free(msg);
        } else {
            error_emit(NULL, 0, EC_GLOB_NO_MATCH, "no matches");
        }
        return 2;
    }

    /* Sort results by NFC Unicode code-point order. */
    qsort(list.items, list.count, sizeof(char *), compare_paths);

    result->paths = list.items;
    result->count = list.count;
    return 0;
}

void glob_result_free(glob_result_t *result)
{
    if (!result)
        return;
    for (size_t i = 0; i < result->count; i++)
        free(result->paths[i]);
    free(result->paths);
    result->paths = NULL;
    result->count = 0;
}
