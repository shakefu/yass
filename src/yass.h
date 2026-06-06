/*
 * yass.h - Common definitions for the yass CLI
 */
#ifndef YASS_H
#define YASS_H

#include <stddef.h>
#include <stdbool.h>
#include <stdio.h>

/* Exit codes per cli.ExitCode */
#define EXIT_SUCCESS_CODE  0
#define EXIT_PROCESSING    1
#define EXIT_USAGE         2
#define EXIT_SIGINT        130
#define EXIT_SIGTERM       143

/* Slot keywords */
static const char *SLOT_KEYWORDS[] = {
    "INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT", NULL
};

/* Normativity keywords */
static const char *NORMATIVITY_KEYWORDS[] = {
    "MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY", NULL
};

/* Reference relation keywords */
static const char *REFERENCE_KEYWORDS[] = {
    "CONFORMS", "USES", "SEE", NULL
};

/* Guard keyword */
#define GUARD_KEYWORD "WHEN"

/* All reserved keywords (slots + normativity) for spec name collision check */
static const char *RESERVED_KEYWORDS[] = {
    "INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT",
    "MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY",
    "WHEN", "CONFORMS", "USES", "SEE",
    NULL
};

/* Check if a string matches any in a NULL-terminated array (case-insensitive) */
bool yass_is_keyword_ci(const char *name, const char *keywords[]);

/* Check if a string matches any in a NULL-terminated array (case-sensitive) */
bool yass_is_keyword(const char *name, const char *keywords[]);

#endif /* YASS_H */
