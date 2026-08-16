/*
 * path.h - Path utilities
 */
#ifndef YASS_PATH_H
#define YASS_PATH_H

#include <stdbool.h>
#include <stddef.h>

/*
 * Check if a path has the .yass.yaml suffix.
 */
bool path_has_yass_suffix(const char *path);

/*
 * Check if a path contains a colon character.
 */
bool path_has_colon(const char *path);

/*
 * Get the basename of a path (pointer into the original string).
 */
const char *path_basename(const char *path);

/*
 * Get the directory portion of a path.
 * Returns a malloc'd string. Caller must free.
 */
char *path_dirname(const char *path);

/*
 * Join two path components.
 * Returns a malloc'd string. Caller must free.
 */
char *path_join(const char *dir, const char *file);

/*
 * Make a path relative to cwd per ErrorLine spec.
 * Returns a malloc'd string. Caller must free.
 */
char *path_relative_to_cwd(const char *path);

/*
 * Get the lexical absolute path (no realpath, no symlink resolution).
 * Returns a malloc'd string. Caller must free.
 */
char *path_absolute(const char *path);

/*
 * Check if a path begins with a given prefix directory.
 */
bool path_starts_with(const char *path, const char *prefix);

/*
 * Compare two paths for NFC Unicode code-point order sorting.
 */
int path_compare_nfc(const char *a, const char *b);

/*
 * Check if a string contains glob metacharacters.
 */
bool path_has_glob_chars(const char *path);

#endif /* YASS_PATH_H */
