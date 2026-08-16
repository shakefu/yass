#pragma once

// M5 — structural validation checks (preamble, spec, uniqueness, refs).
//
// Spec basis (spec/cli.validate.yass.yaml):
//   - cli.validate.CheckPreamble   -> check_preamble
//   - cli.validate.CheckSpec       -> check_spec
//   - cli.validate.CheckUniqueness -> check_uniqueness
//   - cli.validate.CheckRefs       -> check_refs
//
// Each check consumes the parsed model (src/model.hpp) built from a
// yaml::ParsedStream and returns a vector of yass::diag::Diagnostic in the exact
// order the spec mandates. Error codes/exit are pinned to the reference yass;
// message prose follows the spec via diag::canonical_message; line/column follow
// cli.ErrorLine. These functions perform NO stdout/stderr I/O and (except for
// check_refs, which opens referenced files for existence + target-spec lookup
// only) NO filesystem access.

#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"
#include "model.hpp"
#include "yaml.hpp"

namespace yass::check {

// --------------------------------------------------------------------------
// cli.validate.CheckPreamble
// --------------------------------------------------------------------------
// Validate the first document as a Preamble. Emits AT MOST ONE diagnostic, in
// the order:
//   (1) yass.preamble.has_spec_key, (2) yass.yaml.empty_stream,
//   (3) yass.preamble.missing, (4) yass.preamble.duplicate,
//   (5) yass.preamble.misplaced, (6) yass.preamble.missing_description,
//   (7) yass.preamble.missing_version, (8) yass.preamble.unknown_version,
//   (9) yass.preamble.bad_related,
// stopping at the first match. `file_label` is the cli.ErrorLine path form.
std::optional<diag::Diagnostic> check_preamble(std::string_view file_label,
                                               const yaml::ParsedStream& stream,
                                               std::string_view source);

// --------------------------------------------------------------------------
// cli.validate.CheckSpec
// --------------------------------------------------------------------------
// Validate one Spec document (a non-first document). Emits one diagnostic per
// failing rule per obligation, in source order. Covers spec.no_name,
// name_not_string/empty/bad_chars/bad_form/reserved, unknown_key,
// slot.value_not_list, obligation.bad_value_shape/missing_normativity_or_ref/
// guard_without_normativity/duplicate_reference/duplicate_normativity,
// normativity.unknown, and reference.unknown_relation.
std::vector<diag::Diagnostic> check_spec(std::string_view file_label,
                                         const model::Spec& spec);

// Convenience: run check_spec over every spec in a model, concatenating the
// diagnostics in document order.
std::vector<diag::Diagnostic> check_specs(std::string_view file_label,
                                          const model::Model& model);

// --------------------------------------------------------------------------
// cli.validate.CheckUniqueness
// --------------------------------------------------------------------------
// Emit yass.spec.duplicate_name once per duplicate-after-the-first occurrence,
// each attributed to the subsequent occurrence's spec-name line. Only specs that
// carry a usable name participate.
std::vector<diag::Diagnostic> check_uniqueness(std::string_view file_label,
                                               const model::Model& model);

// --------------------------------------------------------------------------
// cli.validate.CheckRefs
// --------------------------------------------------------------------------
// For every Reference in every obligation: validate the RefTarget grammar
// (ref.malformed) -> slot suffix (ref.unknown_slot) -> resolution. Same-file
// missing -> ref.spec_not_found_same_file. Cross-file: file missing ->
// ref.file_not_found; unparseable -> ref.file_not_parseable; spec missing ->
// ref.spec_not_found_other_file; slot not declared in the referenced spec's
// source document -> ref.slot_not_declared. Referenced files are opened for
// existence + target-spec lookup ONLY. At most one file_not_found /
// file_not_parseable per (referencing-file, referenced-file) pair. Errors
// attribute to the referencing file's ref line/column. No cycle detection.
//
// `base_path` is the referencing file path (for ./ and ../ relative targets);
// `project_root` is the root for root-relative targets.
std::vector<diag::Diagnostic> check_refs(std::string_view file_label,
                                         const model::Model& model,
                                         std::string_view base_path,
                                         std::string_view project_root);

}  // namespace yass::check
