/*
 * check_yaml.h - YAML well-formedness check per cli.validate.CheckYAML
 */
#ifndef YASS_CHECK_YAML_H
#define YASS_CHECK_YAML_H

#include "yaml_parse.h"

/*
 * Check YAML well-formedness of a file.
 * On success, populates result with parsed documents.
 * On error, emits one ErrorLine and returns non-zero.
 *
 * Priority order per spec:
 * 1. yass.yaml.not_utf8
 * 2. yass.yaml.has_bom
 * 3. yass.yaml.empty_file
 * 4. yass.yaml.malformed
 * 5. yass.yaml.duplicate_key
 * 6. yass.yaml.anchor_or_alias
 */
int check_yaml(const char *file, yaml_parse_result_t *result);

#endif /* YASS_CHECK_YAML_H */
