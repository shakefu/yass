/*
 * validate.c - Validate subcommand per cli.validate spec
 */
#include "validate.h"
#include "discover.h"
#include "glob.h"
#include "path.h"
#include "error.h"
#include "yaml_parse.h"
#include "check_yaml.h"
#include "check_preamble.h"
#include "check_spec.h"
#include "check_uniqueness.h"
#include "check_refs.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

/* ---- helpers ---- */

/*
 * Emit the summary line to stdout after flushing stderr.
 * Format: "checked <N> files, found <M> errors\n"
 */
static void emit_summary(int n, int m)
{
    fflush(stderr);
    fprintf(stdout, "checked %d files, found %d errors\n", n, m);
    fflush(stdout);
}

/* ---- dynamic path list (local to validate) ---- */

typedef struct {
    char **items;
    size_t count;
    size_t capacity;
} val_path_list_t;

static void val_path_list_init(val_path_list_t *pl)
{
    pl->items = NULL;
    pl->count = 0;
    pl->capacity = 0;
}

static int val_path_list_push(val_path_list_t *pl, char *path)
{
    if (pl->count >= pl->capacity) {
        size_t newcap = pl->capacity == 0 ? 16 : pl->capacity * 2;
        char **newitems = realloc(pl->items, newcap * sizeof(char *));
        if (!newitems)
            return -1;
        pl->items = newitems;
        pl->capacity = newcap;
    }
    pl->items[pl->count++] = path;
    return 0;
}

static void val_path_list_free(val_path_list_t *pl)
{
    for (size_t i = 0; i < pl->count; i++)
        free(pl->items[i]);
    free(pl->items);
    pl->items = NULL;
    pl->count = 0;
    pl->capacity = 0;
}

/*
 * Check if abs_path is already in a dedup list.
 */
static int dedup_contains(val_path_list_t *dedup, const char *abs)
{
    for (size_t i = 0; i < dedup->count; i++) {
        if (strcmp(dedup->items[i], abs) == 0)
            return 1;
    }
    return 0;
}

static int path_cmp_nfc(const void *a, const void *b)
{
    const char *pa = *(const char **)a;
    const char *pb = *(const char **)b;
    return path_compare_nfc(pa, pb);
}

/*
 * Merge a discover_result_t into the combined + dedup lists.
 * Takes ownership of nothing; copies strings as needed.
 */
static void merge_discover(discover_result_t *dr,
                           val_path_list_t *combined,
                           val_path_list_t *dedup)
{
    for (size_t j = 0; j < dr->count; j++) {
        char *abspath = path_absolute(dr->paths[j]);
        if (abspath && !dedup_contains(dedup, abspath)) {
            val_path_list_push(dedup, abspath);
            val_path_list_push(combined, strdup(dr->paths[j]));
        } else {
            free(abspath);
        }
    }
}

/* ---- cmd_validate ---- */

int cmd_validate(int argc, char **argv)
{
    /*
     * Step 1: Check ALL positional args for colon-in-path before doing
     * anything else. Per spec, reject the invocation with
     * yass.path.colon_in_path and exit 2.
     */
    for (int i = 0; i < argc; i++) {
        if (path_has_colon(argv[i])) {
            error_emit(argv[i], 0, EC_PATH_COLON_IN_PATH,
                       "path contains a colon character");
            emit_summary(0, 0);
            return 2;
        }
    }

    /*
     * Step 2: Find project root once using find_project_root(NULL).
     * Per spec, compute exactly once at startup from cwd.
     */
    char *project_root = find_project_root(NULL);
    if (!project_root) {
        error_emit(NULL, 0, EC_FINDROOT_NO_MARKER,
                   "no project root marker found");
        emit_summary(0, 0);
        return 2;
    }

    /*
     * Step 3: Collect files.
     *
     * When args are provided, we handle glob expansion here in validate
     * rather than delegating entirely to discover_from_args. This is
     * because the spec requires:
     *   - Glob-expanded paths that are not .yass.yaml: skip silently
     *   - Non-glob file args that are not .yass.yaml: error with bad_extension, exit 2
     *
     * discover_spec_files treats ALL non-.yass.yaml files as bad_extension
     * errors, so we must filter glob-expanded results ourselves.
     */
    val_path_list_t combined;
    val_path_list_init(&combined);
    val_path_list_t dedup;
    val_path_list_init(&dedup);

    int collect_rc = 0;

    if (argc == 0) {
        /* No args: discover from project root */
        discover_result_t dr;
        collect_rc = discover_spec_files(NULL, &dr);
        if (collect_rc == 0) {
            merge_discover(&dr, &combined, &dedup);
        }
        discover_result_free(&dr);
    } else {
        /* Process each positional arg */
        for (int i = 0; i < argc; i++) {
            const char *arg = argv[i];

            if (path_has_glob_chars(arg)) {
                /*
                 * Glob expansion: expand the pattern, then for each result:
                 *   - If it's a directory: recurse via discover_spec_files
                 *   - If it's a file with .yass.yaml: discover it
                 *   - If it's a file without .yass.yaml: skip silently
                 */
                glob_result_t gr;
                int rc = glob_expand(arg, &gr);
                if (rc != 0) {
                    /* glob_expand already emitted the error */
                    if (collect_rc == 0)
                        collect_rc = rc;
                    continue;
                }

                for (size_t g = 0; g < gr.count; g++) {
                    const char *expanded = gr.paths[g];

                    /* Stat to determine file vs directory */
                    struct stat st;
                    if (stat(expanded, &st) != 0) {
                        /* Path doesn't exist after glob expansion -- skip */
                        continue;
                    }

                    if (S_ISDIR(st.st_mode)) {
                        /* Directory: recurse normally */
                        discover_result_t dr;
                        rc = discover_spec_files(expanded, &dr);
                        if (rc == 0) {
                            merge_discover(&dr, &combined, &dedup);
                        }
                        /* For glob dirs, a discover error is non-fatal */
                        discover_result_free(&dr);
                    } else if (S_ISREG(st.st_mode)) {
                        /* File: skip silently if not .yass.yaml */
                        if (!path_has_yass_suffix(expanded))
                            continue;

                        discover_result_t dr;
                        rc = discover_spec_files(expanded, &dr);
                        if (rc == 0) {
                            merge_discover(&dr, &combined, &dedup);
                        } else {
                            if (collect_rc == 0)
                                collect_rc = rc;
                        }
                        discover_result_free(&dr);
                    }
                    /* Other types (symlinks, etc.): skip silently from glob */
                }
                glob_result_free(&gr);
            } else {
                /*
                 * Non-glob arg: pass directly to discover_spec_files.
                 * It will handle file-vs-directory distinction and will
                 * error with bad_extension for non-.yass.yaml files.
                 */
                discover_result_t dr;
                int rc = discover_spec_files(arg, &dr);
                if (rc != 0) {
                    if (collect_rc == 0)
                        collect_rc = rc;
                    discover_result_free(&dr);
                    continue;
                }
                merge_discover(&dr, &combined, &dedup);
                discover_result_free(&dr);
            }
        }
    }

    val_path_list_free(&dedup);

    /* If there was a fatal collection error, exit */
    if (collect_rc != 0) {
        emit_summary(0, 0);
        val_path_list_free(&combined);
        free(project_root);
        return collect_rc;
    }

    /*
     * Step 4: If no files found -> error yass.discover.no_files, exit 2.
     */
    if (combined.count == 0) {
        error_emit(NULL, 0, EC_DISCOVER_NO_FILES,
                   "no .yass.yaml files found");
        emit_summary(0, 0);
        val_path_list_free(&combined);
        free(project_root);
        return 2;
    }

    /*
     * Sort the collected paths by NFC order for deterministic processing.
     */
    if (combined.count > 1) {
        qsort(combined.items, combined.count, sizeof(char *), path_cmp_nfc);
    }

    /*
     * Step 5: Process each file sequentially.
     * Run checks in order: CheckYAML, CheckPreamble, CheckSpec,
     * CheckUniqueness, CheckRefs.
     */
    int N = 0;  /* files checked */
    int M = 0;  /* total errors */

    for (size_t i = 0; i < combined.count; i++) {
        const char *file = combined.items[i];
        N++;

        /* CheckYAML */
        yaml_parse_result_t parse_result;
        int yaml_rc = check_yaml(file, &parse_result);
        if (yaml_rc != 0) {
            /* YAML parse failed: count as exactly one error, skip remaining */
            M += 1;
            continue;
        }

        /* CheckPreamble */
        int preamble_errs = check_preamble(file, &parse_result);
        M += preamble_errs;

        /* CheckSpec */
        int spec_errs = check_spec(file, &parse_result);
        M += spec_errs;

        /* CheckUniqueness */
        int uniq_errs = check_uniqueness(file, &parse_result);
        M += uniq_errs;

        /* CheckRefs - needs dirname of file and project_root */
        char *base_dir = path_dirname(file);
        int ref_errs = check_refs(file, &parse_result,
                                  base_dir ? base_dir : ".",
                                  project_root);
        M += ref_errs;
        free(base_dir);

        /* Free the parse result */
        yaml_parse_result_free(&parse_result);
    }

    /*
     * Step 6-7: Print summary to stdout after flushing stderr.
     */
    emit_summary(N, M);

    /*
     * Step 8: Return exit code.
     */
    int exit_code = M > 0 ? 1 : 0;

    val_path_list_free(&combined);
    free(project_root);

    return exit_code;
}
