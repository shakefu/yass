/*
 * check_refs.h - Reference resolution check per cli.validate.CheckRefs
 */
#ifndef YASS_CHECK_REFS_H
#define YASS_CHECK_REFS_H

#include "yaml_parse.h"

/*
 * Check reference targets in all specs of a file.
 * base_dir: directory containing the file being checked
 * project_root: project root for root-relative refs
 * Returns the number of errors found.
 */
int check_refs(const char *file, yaml_parse_result_t *result,
               const char *base_dir, const char *project_root);

#endif /* YASS_CHECK_REFS_H */
