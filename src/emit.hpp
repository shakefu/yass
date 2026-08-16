#pragma once

// M7 — the OutputProfile YAML emitter (spec/cli.query.yass.yaml ::
// cli.query.OutputProfile + cli.query.InlineConforms emitter invariants).
//
// This module owns the byte-exact serialization of a query fragment: a single
// YAML document (leading `---`, NO trailing `...`), 2-space block indentation,
// `- ` dash-space list items aligned at the parent indent, plain scalars
// unquoted by default and double-quoted exactly when the OutputProfile rule
// requires, obligation key order reordered to Normativity-keyword / WHEN guard /
// Reference relations, and provenance comments at column zero immediately above
// each inlined obligation.
//
// The emitter is value-driven: a scalar's quoting is recomputed from its decoded
// value content (NOT from whether the source quoted it), per cli.query
// .OutputProfile RETURN. The model the emitter consumes is the lightweight
// Fragment built by src/query.cpp (ExtractFragment + InlineConforms); this file
// is the pure formatter and carries no filesystem or resolution logic.

#include <optional>
#include <string>
#include <string_view>
#include <vector>

namespace yass::emit {

// One key/value pair other than the recognized Normativity / WHEN / Reference
// keys, preserved in source order so a (validated, well-formed) obligation never
// silently drops content. Emitted after the references in the obligation body.
struct ExtraKey {
    std::string key;
    std::string value;
};

// A single Reference relation on an obligation (CONFORMS / USES / SEE), captured
// with the relation key as written and the raw RefTarget value. CONFORMS refs
// are consumed by InlineConforms before emission; only USES / SEE (and any
// CONFORMS that the carrier kept — none, by spec) reach the emitter.
struct Ref {
    std::string key;    // "USES" / "SEE" (CONFORMS is stripped before emit)
    std::string value;  // raw RefTarget bytes
};

// One obligation in the fragment, in emit-ready form. The emitter writes the
// keys in OutputProfile order: Normativity keyword (if any), then WHEN guard (if
// any), then Reference relations in source order, then any extra keys. When
// `provenance` is set the emitter writes `# CONFORMS: <provenance>` at column
// zero on the line immediately above this obligation's `- ` (InlineConforms).
struct Obligation {
    bool has_norm = false;
    std::string norm_key;    // "MUST" / "MUST-NOT" / "SHOULD" / "SHOULD-NOT" / "MAY"
    std::string norm_value;  // obligation prose

    bool has_when = false;
    std::string when_value;  // guard prose

    std::vector<Ref> refs;          // non-CONFORMS relations, source order
    std::vector<ExtraKey> extras;   // unrecognized keys, source order

    // Provenance ref-target (byte-for-byte as written in source) when this
    // obligation was produced by inlining a CONFORMS ref; nullopt otherwise.
    std::optional<std::string> provenance;
};

// One slot group: the slot key exactly as written (e.g. "SIDE-EFFECT") and its
// obligations in emit order.
struct SlotGroup {
    std::string key;
    std::vector<Obligation> obligations;
};

// The whole extracted fragment: the spec name and its slot groups in source
// order. The preamble and sibling specs are excluded by construction.
struct Fragment {
    std::string spec_name;
    std::vector<SlotGroup> slots;
};

// Serialize `fragment` to a UTF-8 / LF byte string per cli.query.OutputProfile:
//   - a single leading `---\n`, then `spec: <name>\n`;
//   - each slot key at column zero followed by `:`;
//   - each obligation as a `- ` item with 2-space-indented continuation keys;
//   - provenance comments at column zero above inlined obligations;
//   - exactly one trailing LF; no trailing `...`; no host header.
std::string emit_fragment(const Fragment& fragment);

// --------------------------------------------------------------------------
// Scalar quoting (cli.query.OutputProfile RETURN) — exposed for unit testing.
// --------------------------------------------------------------------------
// True iff the plain (decoded) scalar `value` MUST be emitted double-quoted:
//   - it contains `: ` (colon followed by a space);
//   - its first character is one of `?-*&!|>%@`;
//   - it has leading or trailing ASCII whitespace;
//   - it is empty;
//   - it matches a YAML 1.2 core-schema type token (true/false/null/yes/no/on/
//     off case-insensitive, the `~` null, or any int/float/hex/oct/exp numeric
//     literal).
bool needs_double_quote(std::string_view value);

// Emit `value` as a YAML scalar: plain when needs_double_quote is false, else
// wrapped in double quotes. (Matches the reference, which wraps the decoded
// value verbatim; no value in the conformance corpus carries an embedded `"` or
// backslash escape.)
std::string emit_scalar(std::string_view value);

}  // namespace yass::emit
