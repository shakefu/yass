/*
 * list.c - List subcommand per cli.list spec
 *
 * Lists specs discovered in .yass.yaml files, showing spec names
 * with file paths and truncated preamble descriptions.
 */
#include "list.h"
#include "yaml_parse.h"
#include "discover.h"
#include "error.h"
#include "path.h"
#include "utf8.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/ioctl.h>

/*
 * Replace literal tab characters in a string with single spaces.
 * Returns a malloc'd string. Caller must free.
 */
static char *replace_tabs(const char *s)
{
    if (!s)
        return strdup("");
    size_t len = strlen(s);
    char *out = malloc(len + 1);
    if (!out)
        return NULL;
    for (size_t i = 0; i < len; i++)
        out[i] = (s[i] == '\t') ? ' ' : s[i];
    out[len] = '\0';
    return out;
}

/*
 * Get terminal width.
 * 1. COLUMNS env var (if set and > 0)
 * 2. ioctl TIOCGWINSZ
 * 3. Default 80
 */
static int get_terminal_width(void)
{
    const char *cols_env = getenv("COLUMNS");
    if (cols_env && cols_env[0]) {
        char *end = NULL;
        long val = strtol(cols_env, &end, 10);
        if (end && *end == '\0' && val > 0)
            return (int)val;
    }

    struct winsize ws;
    if (ioctl(fileno(stdout), TIOCGWINSZ, &ws) == 0 && ws.ws_col > 0)
        return ws.ws_col;

    return 80;
}

int cmd_list(int argc, char **argv)
{
    /* Step 1: Check all positional args for colon */
    for (int i = 0; i < argc; i++) {
        if (path_has_colon(argv[i])) {
            error_emit(NULL, 0, EC_PATH_COLON_IN_PATH,
                       "path contains a colon character");
            return 2;
        }
    }

    /* Step 2: Discover .yass.yaml files */
    discover_result_t dr;
    memset(&dr, 0, sizeof(dr));
    int rc;

    if (argc == 0) {
        rc = discover_spec_files(NULL, &dr);
    } else {
        rc = discover_from_args(argv, argc, &dr);
    }

    /* If discover returned a fatal error (exit 2), propagate it */
    if (rc != 0) {
        discover_result_free(&dr);
        return rc;
    }

    /* Step 3: No files found -> exit 0, no output, no error */
    if (dr.count == 0) {
        discover_result_free(&dr);
        return 0;
    }

    /* TTY detection */
    int is_tty = isatty(fileno(stdout));
    int term_width = 0;
    if (is_tty)
        term_width = get_terminal_width();

    int had_error = 0;

    /* Step 4: Process each discovered file */
    for (size_t fi = 0; fi < dr.count; fi++) {
        const char *file_path = dr.paths[fi];

        /* Parse as YAML */
        yass_yaml_parse_result_t pr;
        memset(&pr, 0, sizeof(pr));
        const char *err_code = NULL;
        char *err_msg = NULL;
        int err_line = 0;

        int parse_rc = yaml_parse_file(file_path, &pr, &err_code, &err_msg,
                                       &err_line);
        if (parse_rc != 0) {
            /*
             * Empty file / empty stream: treat as zero spec documents,
             * not as a parse error. Per spec: "file with zero Spec
             * documents -> no rows, no error."
             */
            if (err_code &&
                (strcmp(err_code, EC_YAML_EMPTY_FILE) == 0 ||
                 strcmp(err_code, EC_YAML_EMPTY_STREAM) == 0)) {
                free(err_msg);
                continue;
            }

            /* Parse failed: emit ErrorLine, set had_error, continue */
            error_emit(file_path, err_line,
                       err_code ? err_code : EC_YAML_MALFORMED,
                       err_msg ? err_msg : "failed to parse YAML");
            free(err_msg);
            had_error = 1;
            continue;
        }

        /* Get preamble description from first doc (index 0) */
        char *description = NULL;
        if (pr.doc_count > 0) {
            const char *raw_desc = yaml_doc_get_string(&pr.docs[0],
                                                       "description");
            if (raw_desc && raw_desc[0]) {
                /* Normalize whitespace */
                char *norm = utf8_normalize_whitespace(raw_desc);
                if (norm) {
                    /* NFC-normalize */
                    char *nfc = utf8_nfc_normalize(norm);
                    free(norm);
                    description = nfc;
                }
            }
        }

        /* Format file path: replace tabs with spaces */
        char *display_path = error_format_path(file_path);
        char *clean_path = replace_tabs(display_path);
        free(display_path);

        /* For each spec document (index >= 1, has "spec" key) */
        for (int di = 1; di < pr.doc_count; di++) {
            const char *spec_name = yaml_doc_get_string(&pr.docs[di], "spec");
            if (!spec_name)
                continue;

            const char *desc = description ? description : "";

            if (is_tty && term_width > 0) {
                /* Measure widths in grapheme clusters */
                size_t path_len = utf8_grapheme_count(clean_path);
                size_t name_len = utf8_grapheme_count(spec_name);
                /* Two tab separators count as 1 column each */
                size_t fixed = path_len + 1 + name_len + 1;
                size_t marker_len = 3; /* "..." */

                if (desc[0] == '\0') {
                    /* Empty description: no marker, emit empty field */
                    fprintf(stdout, "%s\t%s\t\n", clean_path, spec_name);
                } else {
                    size_t desc_len = utf8_grapheme_count(desc);
                    size_t total = fixed + desc_len;

                    if (fixed + marker_len >= (size_t)term_width) {
                        /* No room for description or marker */
                        fprintf(stdout, "%s\t%s\t\n", clean_path, spec_name);
                    } else if (total <= (size_t)term_width) {
                        /* Fits without truncation */
                        fprintf(stdout, "%s\t%s\t%s\n", clean_path, spec_name,
                                desc);
                    } else {
                        /* Must truncate: reserve space for marker */
                        size_t avail = (size_t)term_width - fixed - marker_len;
                        char *trunc = utf8_truncate_graphemes(desc, avail);
                        if (trunc) {
                            fprintf(stdout, "%s\t%s\t%s...\n", clean_path,
                                    spec_name, trunc);
                            free(trunc);
                        } else {
                            fprintf(stdout, "%s\t%s\t\n", clean_path,
                                    spec_name);
                        }
                    }
                }
            } else {
                /* Not a TTY: no truncation */
                fprintf(stdout, "%s\t%s\t%s\n", clean_path, spec_name, desc);
            }
        }

        free(clean_path);
        free(description);
        yaml_parse_result_free(&pr);
    }

    discover_result_free(&dr);
    return had_error ? 1 : 0;
}
