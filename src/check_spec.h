/*
 * check_spec.h - Spec document check per cli.validate.CheckSpec
 */
#ifndef YASS_CHECK_SPEC_H
#define YASS_CHECK_SPEC_H

#include "yaml_parse.h"

/*
 * Check spec document structure for all non-first documents.
 * Returns the number of errors found.
 */
int check_spec(const char *file, yaml_parse_result_t *result);

#endif /* YASS_CHECK_SPEC_H */
