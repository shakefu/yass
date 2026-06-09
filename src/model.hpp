#pragma once

// M5 — parsed structural model (preamble + specs + obligations + references).
//
// Spec basis:
//   - spec/yass.yass.yaml :: Document, Preamble, Spec, Slot, Obligation,
//       Normativity, Guard, Reference, RefTarget, Keyword — the language's
//       self-definition of every construct this model mirrors.
//   - spec/cli.validate.yass.yaml :: cli.validate.CheckPreamble / CheckSpec /
//       CheckUniqueness / CheckRefs — the consumers of this model.
//
// This header turns a yaml::ParsedStream into a structured, ryml-free view that
// the validate checks (M5) and the query subcommand (later) navigate without
// re-walking the YAML tree. It also implements the RefTarget grammar + the
// path/spec/slot resolution rules from spec/yass.yass.yaml :: RefTarget.
//
// Line attribution mirrors the reference yass and cli.ErrorLine:
//   - Document-position facts (which document is a Preamble / Spec, a missing
//     spec key) attribute to the document-start `---` marker line, captured here
//     from the source bytes since ryml's doc-node Location points at the first
//     content line, not the marker.
//   - Content facts (a spec name, a slot key, an obligation, a reference) attribute
//     to the offending node's own 1-based line, taken from yaml::Node::line().

#include <cstddef>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "yaml.hpp"

namespace yass::model {

// --------------------------------------------------------------------------
// Vocabulary (spec/yass.yass.yaml :: Slot, Normativity, Reference).
// --------------------------------------------------------------------------
enum class Slot { INPUT, RETURN, ERROR, SIDE_EFFECT, INVARIANT };
enum class Normativity { MUST, MUST_NOT, SHOULD, SHOULD_NOT, MAY };

// The five Slot keys exactly as they appear as YAML keys (SIDE-EFFECT hyphen).
std::optional<Slot> slot_from_key(std::string_view key);
std::string_view slot_to_key(Slot slot);
// True iff `s` is a valid slot suffix token (one of the five SLOT keywords as
// used after `::` in a RefTarget). Same set as slot_from_key.
bool is_slot_keyword(std::string_view s);

// The five Normativity keys exactly as they appear as YAML keys (hyphenated
// negatives). Returns nullopt for any other token.
std::optional<Normativity> normativity_from_key(std::string_view key);

// True iff `name` case-insensitively equals a Slot or Normativity keyword
// (spec.name_reserved). Comparison is ASCII case-insensitive.
bool is_reserved_name(std::string_view name);

// True iff a PLAIN (unquoted) scalar text resolves to a YAML 1.2 core-schema
// string — i.e. it is NOT a core int, float, bool, or null. Per yass@Document,
// `yes`, `no`, `on`, and `off` remain plain strings. Quoted scalars are always
// strings and need not be passed here. Used for spec.name_not_string and the
// Preamble `related`-is-a-sequence-of-strings check.
bool plain_scalar_is_string(std::string_view text);

// --------------------------------------------------------------------------
// Reference (spec/yass.yass.yaml :: Reference).
// --------------------------------------------------------------------------
struct Reference {
    enum Relation { CONFORMS, USES, SEE };
    Relation relation = USES;
    // The relation key as written (CONFORMS / USES / SEE).
    std::string relation_key;
    // The raw RefTarget string value.
    std::string target;
    // 1-based line/column of the relation key node (cli.ErrorLine attribution).
    int line = 0;
    int col = 0;
};

// --------------------------------------------------------------------------
// Obligation (spec/yass.yass.yaml :: Obligation).
// --------------------------------------------------------------------------
// One element of a Slot's list. Mirrors the obligation mapping: its (single)
// Normativity keyword if any, its guard (WHEN) prose if any, its references in
// source order, and the structural flags the checks need. Unknown keys (neither
// a recognized Normativity keyword nor WHEN nor a Reference relation) are kept
// for normativity.unknown reporting.
struct Obligation {
    // 1-based line/column of the obligation element node (the `- ...` item).
    int line = 0;
    int col = 0;

    // The obligation element's YAML kind; only Map is well-formed. When the
    // element is not a map (scalar/seq/null) the obligation is mis-shaped and
    // value_bad_shape is set.
    bool is_map = false;
    // Set when the obligation element itself is a scalar/sequence/null (i.e. not
    // a mapping) — obligation.bad_value_shape with no recognized keys.
    bool element_not_map = false;

    // The recognized Normativity keyword(s) found, in source order. >1 => the
    // obligation carries duplicate normativity. The first is `norm` below.
    std::optional<Normativity> norm;
    int normativity_count = 0;
    // 1-based source line of each recognized Normativity keyword, in source
    // order (size == normativity_count). The reference emits one
    // duplicate_normativity per keyword AFTER the first, attributed to THAT
    // keyword's line (not the obligation node's line), so CheckSpec needs the
    // per-keyword lines to match both the error count and the line numbers.
    std::vector<int> normativity_lines;

    // True iff a WHEN guard key is present.
    bool has_guard = false;
    std::string guard_value;

    // References (CONFORMS/USES/SEE) in source order.
    std::vector<Reference> refs;

    // Per-key issues, in obligation-key source order. The reference emits these
    // as it scans the obligation's keys (before the obligation-level errors),
    // then the whole file is stable-sorted by (line, column). Two kinds:
    //   BadValueShape  — a recognized key (Normativity / WHEN / Reference) whose
    //                    YAML value is a mapping/sequence/null => bad_value_shape.
    //   UnknownKey     — a key that is not Normativity / WHEN / a Reference
    //                    relation and does NOT resemble a Reference relation =>
    //                    normativity.unknown.
    //   UnknownRelation — an unrecognized key that RESEMBLES a Reference relation
    //                    (case-insensitive prefix relationship with CONFORMS /
    //                    USES / SEE, length >= 2) => reference.unknown_relation.
    //                    The reference yass distinguishes these two cases.
    struct KeyIssue {
        enum Kind { BadValueShape, UnknownKey, UnknownRelation } kind = BadValueShape;
        std::string key;
        int line = 0;
        int col = 0;
    };
    std::vector<KeyIssue> key_issues;

    // Reference relation keys that appear more than once (duplicate_reference).
    // Normally unreachable through the full pipeline because CheckYAML rejects
    // duplicate mapping keys first; retained so CheckSpec is correct in isolation.
    std::vector<std::string> duplicate_relation_keys;

    // True iff the obligation is reference-only: it carries at least one Reference
    // relation key AND no Normativity keyword AND no WHEN guard
    // (spec/yass.yass.yaml :: Obligation INVARIANT).
    bool reference_only = false;
};

// --------------------------------------------------------------------------
// SlotGroup — a slot key and its obligations within one spec.
// --------------------------------------------------------------------------
struct SlotGroup {
    Slot slot = Slot::INPUT;
    std::string key;  // the slot key as written
    int line = 0;     // 1-based line of the slot key node
    // True iff the slot value was a YAML sequence (well-formed). When false the
    // value was a scalar/map/null — slot.value_not_list.
    bool value_is_list = false;
    std::vector<Obligation> obligations;
};

// --------------------------------------------------------------------------
// Spec (spec/yass.yass.yaml :: Spec).
// --------------------------------------------------------------------------
struct Spec {
    // The spec name as written, when the `spec` value is a string.
    std::string name;
    // 1-based line/column of the document-start `---` marker (doc-position
    // attribution: no_name, duplicate_name point here / at the name node).
    int doc_line = 0;
    int doc_col = 1;
    // 1-based line/column of the `spec` value node, when present (name-* errors
    // attribute here).
    int name_line = 0;
    int name_col = 0;

    bool has_spec_key = false;     // a top-level `spec` key is present
    bool name_is_string = false;   // the `spec` value is a (scalar) string
    bool name_is_quoted = false;   // the `spec` value was quoted

    // Top-level keys other than `spec` that are not a valid Slot key
    // (spec.unknown_key), each at its own key line.
    struct UnknownKey {
        std::string key;
        int line = 0;
        int col = 0;
    };
    std::vector<UnknownKey> unknown_keys;

    // The recognized slot groups, in source order.
    std::vector<SlotGroup> slots;
};

// --------------------------------------------------------------------------
// Document classification.
// --------------------------------------------------------------------------
// A parsed document's structural shape, used by CheckPreamble/CheckSpec to tell
// Preambles from Specs and to locate the document-start marker line.
struct Document {
    enum Kind { MAP, SEQ, SCALAR, NULL_, INVALID };
    Kind kind = INVALID;
    // 1-based line of the document-start `---` marker (or, for an unmarked first
    // document, the first content line). Doc-position attribution uses this.
    int marker_line = 0;
    // 1-based line/column of the document's content node (the map/seq/scalar),
    // as ryml reports it.
    int content_line = 0;
    int content_col = 1;
    bool has_spec_key = false;  // a top-level `spec` key (=> a Spec candidate)
};

// --------------------------------------------------------------------------
// Model — the whole file's structured view.
// --------------------------------------------------------------------------
struct Model {
    // Every top-level document in source order, classified.
    std::vector<Document> documents;
    // The parsed specs, one per non-first document (or per document that carries
    // a `spec` key). Built for every document beyond the first; the consumer
    // (CheckSpec/CheckUniqueness/CheckRefs) decides which to validate.
    std::vector<Spec> specs;
};

// Extract the structured model from a parsed stream.
//
// `source` is the original file bytes, needed to recover document-start `---`
// marker lines (ryml does not expose them). The model walks documents[1..] as
// spec candidates; documents[0] is the Preamble position and is exposed only via
// `documents` (CheckPreamble inspects it directly through the ParsedStream).
Model extract(const yaml::ParsedStream& stream, std::string_view source);

// --------------------------------------------------------------------------
// Document-start marker lines (exposed for testing).
// --------------------------------------------------------------------------
// Scan `source` for YAML document-start markers. Returns the 1-based line of
// every line that is exactly `---` or begins with `--- ` (a doc-start marker),
// in ascending order. The first document of a stream may be unmarked.
std::vector<int> document_marker_lines(std::string_view source);

// --------------------------------------------------------------------------
// RefTarget grammar + resolution (spec/yass.yass.yaml :: RefTarget).
// --------------------------------------------------------------------------
// A parsed RefTarget: an optional `path@`, a spec name, and an optional `::SLOT`.
struct RefTarget {
    bool has_path = false;
    std::string path;        // path token before `@` (without trailing `@`)
    std::string spec_name;   // the spec-name token
    bool has_slot = false;
    std::string slot;        // the SLOT token after `::` (uppercase letters/`-`)
};

// Parse `target` against the grammar
//   ^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$
// Returns the parsed parts on success, or nullopt on grammar failure
// (yass.ref.malformed). The slot token is NOT validated against the slot set
// here (that is yass.ref.unknown_slot, a separate step).
std::optional<RefTarget> parse_ref_target(std::string_view target);

// A resolved RefTarget: the absolute target file path, the spec name, and the
// optional slot. `same_file` is true iff the target addressed the referencing
// file itself (a bare name with no `path@`).
struct ResolvedRef {
    std::string file_path;   // lexically-normalized absolute path to target file
    std::string spec_name;
    bool has_slot = false;
    std::string slot;
    bool same_file = false;
};

// Resolve a parsed RefTarget per spec/yass.yass.yaml :: RefTarget:
//   - bare name (no path)            -> the referencing file itself.
//   - path beginning with ./ or ../  -> relative to the referencing file's dir.
//   - any other path                 -> relative to the project root.
// The literal suffix `.yass.yaml` is appended to the path token to form the file
// path. Paths are lexically normalized (no realpath). `referencing_file_path`
// and `project_root` are absolute (or resolvable) paths.
ResolvedRef resolve_ref(const RefTarget& target,
                        std::string_view referencing_file_path,
                        std::string_view project_root);

}  // namespace yass::model
