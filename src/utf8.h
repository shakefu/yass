/*
 * utf8.h - UTF-8 utilities
 */
#ifndef YASS_UTF8_H
#define YASS_UTF8_H

#include <stdbool.h>
#include <stddef.h>

/*
 * Check if a byte buffer is valid UTF-8.
 */
bool utf8_is_valid(const unsigned char *data, size_t len);

/*
 * Check if a byte buffer starts with a UTF-8 BOM (EF BB BF).
 */
bool utf8_has_bom(const unsigned char *data, size_t len);

/*
 * NFC-normalize a UTF-8 string.
 * For ASCII-only strings, returns a copy unchanged.
 * Returns a malloc'd string. Caller must free.
 */
char *utf8_nfc_normalize(const char *str);

/*
 * Count grapheme clusters in a UTF-8 string.
 * Simplified: counts codepoints minus combining marks.
 */
size_t utf8_grapheme_count(const char *str);

/*
 * Truncate a UTF-8 string on a grapheme-cluster boundary.
 * Returns a malloc'd string. Caller must free.
 * max_graphemes is the maximum number of grapheme clusters.
 */
char *utf8_truncate_graphemes(const char *str, size_t max_graphemes);

/*
 * Normalize whitespace: replace any run of whitespace (including
 * newlines and tabs) with a single ASCII space. Strip leading/trailing.
 * Returns a malloc'd string. Caller must free.
 */
char *utf8_normalize_whitespace(const char *str);

#endif /* YASS_UTF8_H */
