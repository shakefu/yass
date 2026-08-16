/*
 * path.c - Path utilities
 */
#include "path.h"
#include "utf8.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static const char YASS_SUFFIX[] = ".yass.yaml";
static const size_t YASS_SUFFIX_LEN = 10; /* strlen(".yass.yaml") */

bool path_has_yass_suffix(const char *path)
{
    if (!path)
        return false;
    size_t len = strlen(path);
    if (len < YASS_SUFFIX_LEN)
        return false;
    if (memcmp(path + len - YASS_SUFFIX_LEN, YASS_SUFFIX, YASS_SUFFIX_LEN) != 0)
        return false;

    /* The basename must have a non-empty prefix before .yass.yaml.
     * i.e. bare ".yass.yaml" (as the basename) must NOT match. */
    const char *base = path_basename(path);
    size_t base_len = strlen(base);
    if (base_len <= YASS_SUFFIX_LEN)
        return false;
    return true;
}

bool path_has_colon(const char *path)
{
    if (!path)
        return false;
    return strchr(path, ':') != NULL;
}

const char *path_basename(const char *path)
{
    if (!path)
        return path;
    const char *slash = strrchr(path, '/');
    if (slash)
        return slash + 1;
    return path;
}

char *path_dirname(const char *path)
{
    if (!path)
        return strdup(".");
    const char *slash = strrchr(path, '/');
    if (!slash)
        return strdup(".");
    if (slash == path)
        return strdup("/");
    size_t len = (size_t)(slash - path);
    char *dir = malloc(len + 1);
    if (!dir)
        return NULL;
    memcpy(dir, path, len);
    dir[len] = '\0';
    return dir;
}

char *path_join(const char *dir, const char *file)
{
    if (!dir || !file)
        return NULL;
    size_t dlen = strlen(dir);
    size_t flen = strlen(file);
    int need_slash = (dlen > 0 && dir[dlen - 1] != '/') ? 1 : 0;
    size_t total = dlen + need_slash + flen + 1;
    char *result = malloc(total);
    if (!result)
        return NULL;
    memcpy(result, dir, dlen);
    if (need_slash)
        result[dlen] = '/';
    memcpy(result + dlen + need_slash, file, flen);
    result[total - 1] = '\0';
    return result;
}

char *path_absolute(const char *path)
{
    if (!path)
        return NULL;
    if (path[0] == '/')
        return strdup(path);
    char cwd[4096];
    if (!getcwd(cwd, sizeof(cwd)))
        return NULL;
    return path_join(cwd, path);
}

char *path_relative_to_cwd(const char *path)
{
    if (!path)
        return NULL;

    char cwd[4096];
    if (!getcwd(cwd, sizeof(cwd)))
        return strdup(path);

    /* Get lexical absolute path (no realpath / symlink resolution). */
    char *abs = path_absolute(path);
    if (!abs)
        return strdup(path);

    size_t cwd_len = strlen(cwd);

    /* Check if abs starts with cwd + "/" */
    if (strncmp(abs, cwd, cwd_len) == 0) {
        if (abs[cwd_len] == '/') {
            /* Return the relative portion after cwd/ */
            char *rel = strdup(abs + cwd_len + 1);
            free(abs);
            return rel;
        }
        if (abs[cwd_len] == '\0') {
            /* The path IS the cwd itself */
            free(abs);
            return strdup(".");
        }
    }

    /* Not under cwd: return the absolute path */
    return abs;
}

bool path_starts_with(const char *path, const char *prefix)
{
    if (!path || !prefix)
        return false;
    size_t plen = strlen(prefix);
    size_t pathlen = strlen(path);
    if (pathlen < plen)
        return false;
    if (strncmp(path, prefix, plen) != 0)
        return false;
    /* Exact match or path continues with '/' */
    if (pathlen == plen)
        return true;
    if (path[plen] == '/')
        return true;
    return false;
}

int path_compare_nfc(const char *a, const char *b)
{
    char *na = utf8_nfc_normalize(a);
    char *nb = utf8_nfc_normalize(b);
    int result = strcmp(na ? na : a, nb ? nb : b);
    free(na);
    free(nb);
    return result;
}

bool path_has_glob_chars(const char *path)
{
    if (!path)
        return false;
    for (const char *p = path; *p; p++) {
        if (*p == '*' || *p == '?' || *p == '[')
            return true;
    }
    return false;
}
