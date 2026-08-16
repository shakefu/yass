/*
 * yass.c - Common utility functions for the yass CLI
 */
#include <ctype.h>
#include <string.h>
#include "yass.h"

bool yass_is_keyword_ci(const char *name, const char *keywords[]) {
    if (name == NULL || keywords == NULL) return false;
    for (size_t i = 0; keywords[i] != NULL; i++) {
        const char *a = name;
        const char *b = keywords[i];
        while (*a && *b) {
            if (tolower((unsigned char)*a) != tolower((unsigned char)*b))
                break;
            a++;
            b++;
        }
        if (*a == '\0' && *b == '\0') return true;
    }
    return false;
}

bool yass_is_keyword(const char *name, const char *keywords[]) {
    if (name == NULL || keywords == NULL) return false;
    for (size_t i = 0; keywords[i] != NULL; i++) {
        if (strcmp(name, keywords[i]) == 0) return true;
    }
    return false;
}
