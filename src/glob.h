/*
 * glob.h - Glob expansion per cli.ExpandGlob spec
 */
#ifndef YASS_GLOB_H
#define YASS_GLOB_H

#include <stddef.h>

typedef struct {
    char **paths;
    size_t count;
} glob_result_t;

/*
 * Expand a glob pattern per cli.ExpandGlob spec.
 * - * matches any chars except /
 * - ? matches one char except /
 * - [...] is a POSIX bracket expression
 * - ** matches zero or more path segments
 * - No brace expansion
 * - No tilde/env expansion
 * - No hidden files/dirs (starting with .)
 * - Case-sensitive
 * - No symlink following
 *
 * Returns 0 on success, exit code on error.
 */
int glob_expand(const char *pattern, glob_result_t *result);

/*
 * Free a glob result.
 */
void glob_result_free(glob_result_t *result);

#endif /* YASS_GLOB_H */
