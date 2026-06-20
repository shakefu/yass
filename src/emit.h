/*
 * emit.h - YAML emitter for query output per cli.query.OutputProfile
 */
#ifndef YASS_EMIT_H
#define YASS_EMIT_H

#include "yaml_parse.h"
#include <stdio.h>

/*
 * Emit a spec document as a YAML fragment per OutputProfile.
 * - Starts with ---
 * - 2-space indentation
 * - Block style
 * - Plain scalars unquoted unless they need quoting
 * - No trailing ...
 * - Ends with exactly one trailing LF
 */
int emit_spec_fragment(FILE *out, yaml_doc_t *doc);

/*
 * Check if a scalar value needs double-quoting per OutputProfile rules.
 */
int emit_needs_quoting(const char *value);

#endif /* YASS_EMIT_H */
