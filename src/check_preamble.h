/*
 * check_preamble.h - Preamble check per cli.validate.CheckPreamble
 */
#ifndef YASS_CHECK_PREAMBLE_H
#define YASS_CHECK_PREAMBLE_H

#include "yaml_parse.h"

/*
 * Check preamble structure.
 * Emits at most one error per file.
 * Returns 0 on success, 1 on error.
 */
int check_preamble(const char *file, yaml_parse_result_t *result);

#endif /* YASS_CHECK_PREAMBLE_H */
