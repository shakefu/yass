/*
 * check_uniqueness.h - Spec name uniqueness check per cli.validate.CheckUniqueness
 */
#ifndef YASS_CHECK_UNIQUENESS_H
#define YASS_CHECK_UNIQUENESS_H

#include "yaml_parse.h"

/*
 * Check that all spec names are unique within a file.
 * Returns the number of errors found.
 */
int check_uniqueness(const char *file, yaml_parse_result_t *result);

#endif /* YASS_CHECK_UNIQUENESS_H */
