/*
 * argv.c - Argument parsing per cli.Dispatch spec
 */
#include "argv.h"
#include "error.h"
#include <string.h>
#include <stdlib.h>
#include <ctype.h>

/* Valid subcommand names */
static const char *SUBCOMMANDS[] = {"validate", "query", "list", NULL};

/* Valid flag names (with leading --) */
static const char *FLAGS[] = {"--help", "--version", NULL};

/*
 * Check if str equals target when both are lowercased.
 * Returns true if lowercased str matches lowercased target exactly.
 */
static int str_eq_icase(const char *str, const char *target) {
    size_t i;
    for (i = 0; str[i] && target[i]; i++) {
        if (tolower((unsigned char)str[i]) != tolower((unsigned char)target[i]))
            return 0;
    }
    return str[i] == '\0' && target[i] == '\0';
}

/*
 * Check if str is a proper prefix of target (str is shorter but matches
 * the beginning of target when compared case-insensitively).
 */
static int is_prefix_icase(const char *str, const char *target) {
    size_t i;
    if (str[0] == '\0') return 0;
    for (i = 0; str[i] && target[i]; i++) {
        if (tolower((unsigned char)str[i]) != tolower((unsigned char)target[i]))
            return 0;
    }
    /* str must be exhausted and target must have remaining chars */
    return str[i] == '\0' && target[i] != '\0';
}

/*
 * Check if a token (without leading --) is a case-insensitive prefix of any
 * subcommand.
 */
static int is_subcommand_abbreviation(const char *token) {
    for (int i = 0; SUBCOMMANDS[i]; i++) {
        if (is_prefix_icase(token, SUBCOMMANDS[i]))
            return 1;
    }
    return 0;
}

/*
 * Check if a flag token (with leading --) is a case-insensitive prefix of any
 * valid flag.
 */
static int is_flag_abbreviation(const char *token) {
    for (int i = 0; FLAGS[i]; i++) {
        if (is_prefix_icase(token, FLAGS[i]))
            return 1;
    }
    return 0;
}

/*
 * Check if a token (without leading --) case-insensitively matches a
 * subcommand but is not an exact byte-for-byte match.
 */
static int is_subcommand_case_mismatch(const char *token) {
    for (int i = 0; SUBCOMMANDS[i]; i++) {
        if (str_eq_icase(token, SUBCOMMANDS[i]) && strcmp(token, SUBCOMMANDS[i]) != 0)
            return 1;
    }
    return 0;
}

/*
 * Check if a flag token (with leading --) case-insensitively matches a valid
 * flag but is not an exact byte-for-byte match.
 */
static int is_flag_case_mismatch(const char *token) {
    for (int i = 0; FLAGS[i]; i++) {
        if (str_eq_icase(token, FLAGS[i]) && strcmp(token, FLAGS[i]) != 0)
            return 1;
    }
    return 0;
}

static int emit_error(const char *code, const char *message) {
    error_emit("yass", 0, code, message);
    return 2;
}

int argv_parse(int argc, char **argv, yass_args_t *args) {
    /* Initialize output */
    memset(args, 0, sizeof(*args));

    /* Skip argv[0] (program name) */
    int start = 1;

    /* Phase 1: Scan ALL args for --help and --version first.
     * --help takes priority over --version. */
    int saw_help = 0;
    int saw_version = 0;
    for (int i = start; i < argc; i++) {
        if (strcmp(argv[i], "--help") == 0)
            saw_help = 1;
        else if (strcmp(argv[i], "--version") == 0)
            saw_version = 1;
    }

    if (saw_help) {
        args->command = CMD_HELP;
        args->help = true;
        return 0;
    }
    if (saw_version) {
        args->command = CMD_VERSION;
        args->version = true;
        return 0;
    }

    /* Phase 2: Parse arguments in order, checking for errors */
    int end_of_options = 0;
    int subcommand_found = 0;

    /* Allocate space for positionals (at most argc-1 entries) */
    if (argc > 1) {
        args->positionals = calloc((size_t)(argc - 1), sizeof(char *));
        if (!args->positionals) {
            return emit_error(EC_INTERNAL_UNCAUGHT, "memory allocation failed");
        }
    }

    for (int i = start; i < argc; i++) {
        char *arg = argv[i];

        /* After --, everything is a positional */
        if (end_of_options) {
            args->positionals[args->positional_count++] = arg;
            continue;
        }

        /* Check for -- end-of-options marker */
        if (strcmp(arg, "--") == 0) {
            end_of_options = 1;
            continue;
        }

        /* Error 1: Empty argument */
        if (arg[0] == '\0') {
            free(args->positionals);
            args->positionals = NULL;
            args->positional_count = 0;
            return emit_error(EC_ARGV_EMPTY_ARGUMENT,
                              "empty argument is not allowed");
        }

        /* Error 2: Bare "-" token */
        if (strcmp(arg, "-") == 0) {
            free(args->positionals);
            args->positionals = NULL;
            args->positional_count = 0;
            return emit_error(EC_ARGV_STDIN_DASH,
                              "stdin dash '-' is not supported");
        }

        /* Error 3: Short flags (single dash + chars, like -h, -v) */
        if (arg[0] == '-' && arg[1] != '-' && arg[1] != '\0') {
            free(args->positionals);
            args->positionals = NULL;
            args->positional_count = 0;
            return emit_error(EC_ARGV_SHORT_FLAG,
                              "short flags are not supported; use --help or --version");
        }

        /* Handle flags (starts with --) */
        if (arg[0] == '-' && arg[1] == '-') {
            /* Error 4: Case mismatch for flags */
            if (is_flag_case_mismatch(arg)) {
                free(args->positionals);
                args->positionals = NULL;
                args->positional_count = 0;
                return emit_error(EC_ARGV_CASE_MISMATCH,
                                  "flags must be lowercase");
            }

            /* Error 5: Abbreviation for flags */
            if (is_flag_abbreviation(arg)) {
                free(args->positionals);
                args->positionals = NULL;
                args->positional_count = 0;
                return emit_error(EC_ARGV_ABBREVIATION,
                                  "flag abbreviations are not supported; use the full flag name");
            }

            /* Error 6: Unknown flag */
            free(args->positionals);
            args->positionals = NULL;
            args->positional_count = 0;
            return emit_error(EC_ARGV_UNKNOWN_FLAG,
                              "unknown flag");
        }

        /* Handle positional args / subcommands */
        if (!subcommand_found) {
            /* First positional should be the subcommand */

            /* Error 4: Case mismatch for subcommands */
            if (is_subcommand_case_mismatch(arg)) {
                free(args->positionals);
                args->positionals = NULL;
                args->positional_count = 0;
                return emit_error(EC_ARGV_CASE_MISMATCH,
                                  "subcommands must be lowercase");
            }

            /* Error 5: Abbreviation for subcommands */
            if (is_subcommand_abbreviation(arg)) {
                free(args->positionals);
                args->positionals = NULL;
                args->positional_count = 0;
                return emit_error(EC_ARGV_ABBREVIATION,
                                  "subcommand abbreviations are not supported; use the full name");
            }

            /* Try exact match */
            if (strcmp(arg, "validate") == 0) {
                args->command = CMD_VALIDATE;
                subcommand_found = 1;
            } else if (strcmp(arg, "query") == 0) {
                args->command = CMD_QUERY;
                subcommand_found = 1;
            } else if (strcmp(arg, "list") == 0) {
                args->command = CMD_LIST;
                subcommand_found = 1;
            } else {
                /* Error 7: Unknown subcommand */
                free(args->positionals);
                args->positionals = NULL;
                args->positional_count = 0;
                return emit_error(EC_ARGV_UNKNOWN_SUBCOMMAND,
                                  "unknown subcommand");
            }
        } else {
            /* Subsequent positionals after subcommand */
            args->positionals[args->positional_count++] = arg;
        }
    }

    /* Error 8: No subcommand given */
    if (!subcommand_found) {
        free(args->positionals);
        args->positionals = NULL;
        args->positional_count = 0;
        return emit_error(EC_ARGV_NO_SUBCOMMAND,
                          "no subcommand provided");
    }

    return 0;
}
