/*
 * discover.h - File discovery and project root finding
 */
#ifndef YASS_DISCOVER_H
#define YASS_DISCOVER_H

#include <stdbool.h>
#include <stddef.h>

/*
 * Result of file discovery.
 */
typedef struct {
    char **paths;      /* malloc'd array of malloc'd strings */
    size_t count;
    int error_count;   /* errors encountered during discovery */
} discover_result_t;

/*
 * Find the project root by searching upward from start_dir.
 * Algorithm:
 * 1. Search upward for .git (deepest match)
 * 2. If no .git found, search upward for any .yass.yaml file
 *
 * Returns a malloc'd absolute path, or NULL on failure.
 * Caller must free.
 */
char *find_project_root(const char *start_dir);

/*
 * Discover .yass.yaml files.
 * - If path is a file, returns that single file (after extension check)
 * - If path is a directory, recursively searches for .yass.yaml files
 * - If path is NULL, discovers from project root
 *
 * Emits ErrorLines to stderr for errors.
 * Returns 0 on success, exit code on fatal error.
 */
int discover_spec_files(const char *path, discover_result_t *result);

/*
 * Free a discover result.
 */
void discover_result_free(discover_result_t *result);

/*
 * Discover from multiple paths (for validate positional args).
 * Deduplicates by absolute path.
 */
int discover_from_args(char **paths, int count, discover_result_t *result);

#endif /* YASS_DISCOVER_H */
