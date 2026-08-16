/*
 * utf8.c - UTF-8 utilities implementation
 */
#include "utf8.h"

#include <stdlib.h>
#include <string.h>

/*
 * Decode a single codepoint from a UTF-8 byte sequence.
 * Returns the number of bytes consumed, or 0 on error.
 * Stores the decoded codepoint in *cp.
 */
static int utf8_decode(const unsigned char *s, size_t len, uint32_t *cp) {
    if (len == 0) return 0;

    unsigned char b0 = s[0];

    /* 1-byte (ASCII) */
    if (b0 <= 0x7F) {
        *cp = b0;
        return 1;
    }

    /* Reject continuation bytes and invalid start bytes */
    if (b0 < 0xC2) return 0;   /* 0x80..0xBF = continuation, 0xC0..0xC1 = overlong */

    int expect;      /* total byte count */
    uint32_t lower;  /* minimum codepoint for this length (overlong check) */

    if (b0 <= 0xDF) {
        expect = 2;
        *cp = b0 & 0x1F;
        lower = 0x80;
    } else if (b0 <= 0xEF) {
        expect = 3;
        *cp = b0 & 0x0F;
        lower = 0x800;
    } else if (b0 <= 0xF4) {
        expect = 4;
        *cp = b0 & 0x07;
        lower = 0x10000;
    } else {
        return 0;  /* 0xF5..0xFF are invalid */
    }

    if ((size_t)expect > len) return 0;  /* truncated */

    for (int i = 1; i < expect; i++) {
        unsigned char c = s[i];
        if ((c & 0xC0) != 0x80) return 0;  /* not a continuation byte */
        *cp = (*cp << 6) | (c & 0x3F);
    }

    /* Overlong encoding check */
    if (*cp < lower) return 0;

    /* Surrogate halves are invalid in UTF-8 */
    if (*cp >= 0xD800 && *cp <= 0xDFFF) return 0;

    /* Values above U+10FFFF are invalid */
    if (*cp > 0x10FFFF) return 0;

    return expect;
}

bool utf8_is_valid(const unsigned char *data, size_t len) {
    size_t i = 0;
    while (i < len) {
        uint32_t cp;
        int n = utf8_decode(data + i, len - i, &cp);
        if (n == 0) return false;
        i += (size_t)n;
    }
    return true;
}

bool utf8_has_bom(const unsigned char *data, size_t len) {
    if (len < 3) return false;
    return data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF;
}

char *utf8_nfc_normalize(const char *str) {
    if (!str) return NULL;
    return strdup(str);
}

/*
 * Check if a codepoint is a combining mark in the common
 * combining diacriticals range U+0300..U+036F.
 */
static bool is_combining_diacritical(uint32_t cp) {
    return cp >= 0x0300 && cp <= 0x036F;
}

size_t utf8_grapheme_count(const char *str) {
    if (!str) return 0;

    const unsigned char *s = (const unsigned char *)str;
    size_t len = strlen(str);
    size_t count = 0;
    size_t i = 0;

    while (i < len) {
        uint32_t cp;
        int n = utf8_decode(s + i, len - i, &cp);
        if (n == 0) {
            /* Invalid byte; skip it and count it as a cluster */
            i++;
            count++;
            continue;
        }
        if (!is_combining_diacritical(cp)) {
            count++;
        }
        i += (size_t)n;
    }
    return count;
}

char *utf8_truncate_graphemes(const char *str, size_t max_graphemes) {
    if (!str) return NULL;

    const unsigned char *s = (const unsigned char *)str;
    size_t len = strlen(str);
    size_t count = 0;
    size_t i = 0;
    size_t last_boundary = 0;  /* byte offset of the last grapheme boundary */

    while (i < len) {
        uint32_t cp;
        int n = utf8_decode(s + i, len - i, &cp);
        if (n == 0) {
            /* Invalid byte; treat as single-byte cluster */
            count++;
            if (count > max_graphemes) break;
            i++;
            last_boundary = i;
            continue;
        }
        if (!is_combining_diacritical(cp)) {
            count++;
            if (count > max_graphemes) break;
        }
        i += (size_t)n;
        /* Update boundary after consuming this codepoint.
         * If the next codepoint is a combining mark, it will still
         * be included (the boundary advances past it below). */
        last_boundary = i;
    }

    /* If we didn't exceed, last_boundary == len (copy everything) */
    char *result = malloc(last_boundary + 1);
    if (!result) return NULL;
    memcpy(result, str, last_boundary);
    result[last_boundary] = '\0';
    return result;
}

char *utf8_normalize_whitespace(const char *str) {
    if (!str) return NULL;

    size_t len = strlen(str);
    char *buf = malloc(len + 1);
    if (!buf) return NULL;

    size_t out = 0;
    bool in_ws = true;  /* start true to strip leading whitespace */

    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)str[i];
        if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
            if (!in_ws) {
                buf[out++] = ' ';
                in_ws = true;
            }
        } else {
            buf[out++] = c;
            in_ws = false;
        }
    }

    /* Strip trailing whitespace (which would be a single space if present) */
    if (out > 0 && buf[out - 1] == ' ') {
        out--;
    }

    buf[out] = '\0';
    return buf;
}
