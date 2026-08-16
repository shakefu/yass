#pragma once

// M3 — YAML well-formedness + parsed-stream access.
//
// Spec basis:
//   - spec/cli.validate.yass.yaml :: cli.validate.CheckYAML
//       INPUT/RETURN/ERROR/INVARIANT — at-most-one error per file, in the
//       preference order not_utf8 > has_bom > empty_file > malformed >
//       duplicate_key > anchor_or_alias.
//   - context/yass-reference.md :: "yass@Document" (a yass file is a UTF-8 YAML
//       1.2 multi-document stream) and the prohibition (encoded as the
//       anchor_or_alias / duplicate_key codes) on anchors, aliases, explicit
//       tags, and duplicate mapping keys.
//   - spec/cli.errors.yass.yaml :: cli.errors RETURN (the yass.yaml.* codes and
//       their exact messages, surfaced via yass::diag).
//
// This module owns the bridge to the third-party ryml parser. Two pieces:
//
//   1) ParsedStream — a façade that OWNS the ryml Parser + Tree (so node ids and
//      Locations stay valid for the wrapper's lifetime) and exposes a thin Node
//      handle. Downstream modules (CheckPreamble, CheckSpec, CheckRefs, query)
//      navigate documents/maps/seqs/scalars through Node WITHOUT depending on
//      ryml types: kind, scalar value, quoted-vs-plain, 1-based line, ordered
//      map-key iteration, and stream-wide anchor/alias/tag detection.
//
//   2) check_yaml() — implements cli.validate.CheckYAML exactly: byte-level
//      checks first (not_utf8, has_bom, empty_file), then a guarded ryml parse
//      (malformed on throw), then a tree walk (duplicate_key, anchor_or_alias).
//      Returns EITHER one Diagnostic OR success carrying the ParsedStream.

#include <memory>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"

namespace yass::yaml {

// --------------------------------------------------------------------------
// Node kind (façade over ryml's type flags).
// --------------------------------------------------------------------------
// A yass document node is one of: a mapping, a sequence, a scalar (has a value
// and is not a container), or null (an explicit `~`/`null` or an empty value
// position). Invalid is the sentinel for a non-existent node handle.
enum class NodeKind {
    Invalid,
    Map,
    Seq,
    Scalar,
    Null,
};

class ParsedStream;  // forward decl; defined below.

// --------------------------------------------------------------------------
// Node — a stable, copyable handle into a ParsedStream.
// --------------------------------------------------------------------------
// A Node borrows the ParsedStream (which owns the ryml Tree + Parser); it does
// NOT own anything and is only valid while its ParsedStream is alive. All
// accessors are read-only. string_view results point into the ParsedStream's
// parse arena, so they too are valid only for the stream's lifetime.
class Node {
public:
    Node() = default;

    // True iff this handle refers to a real node.
    bool valid() const { return stream_ != nullptr && id_ != kNone; }
    explicit operator bool() const { return valid(); }

    NodeKind kind() const;
    bool is_map() const { return kind() == NodeKind::Map; }
    bool is_seq() const { return kind() == NodeKind::Seq; }
    bool is_scalar() const { return kind() == NodeKind::Scalar; }
    bool is_null() const { return kind() == NodeKind::Null; }

    // Scalar value as a view into the parse arena. Empty when the node is not a
    // value-bearing node. For a null node this is the empty / "~" / "null"
    // source text as ryml captured it.
    std::string_view scalar() const;

    // True iff this node's VALUE scalar was written with explicit quotes
    // (single or double). A plain (unquoted) scalar reports false. Only
    // meaningful for value-bearing nodes.
    bool is_value_quoted() const;
    // True iff this node's VALUE scalar was written as a plain (unquoted)
    // scalar. Complement of quoted/literal/folded for value-bearing nodes.
    bool is_value_plain() const;

    // True iff this node carries a key (i.e. it is a member of a mapping).
    bool has_key() const;
    // The key text as a view into the parse arena; empty if !has_key().
    std::string_view key() const;
    // True iff this node's KEY scalar was written with explicit quotes.
    bool is_key_quoted() const;

    // 1-based line number of this node in the source, via the owning stream's
    // Parser locations. ryml reports 0-based; this adds 1. Returns nullopt only
    // when no location is available.
    std::optional<int> line() const;

    // Ordered children. For a Map, each child is a key/value member in source
    // order; iterate keys via child.key(). For a Seq, each child is an element.
    // For a scalar/null/invalid node, returns empty.
    std::vector<Node> children() const;

private:
    friend class ParsedStream;
    static constexpr std::size_t kNone = static_cast<std::size_t>(-1);
    Node(const ParsedStream* stream, std::size_t id) : stream_(stream), id_(id) {}

    const ParsedStream* stream_ = nullptr;
    std::size_t id_ = kNone;
};

// --------------------------------------------------------------------------
// ParsedStream — owns the ryml Parser + Tree; the parsed-stream façade.
// --------------------------------------------------------------------------
// Construct via ParsedStream::parse() (or implicitly via check_yaml()). Holds
// the parse result alive so Node handles, scalar views, and locations remain
// valid for as long as the ParsedStream lives. Move-only (it owns large parser
// state); copying is disabled.
class ParsedStream {
public:
    ~ParsedStream();
    ParsedStream(ParsedStream&&) noexcept;
    ParsedStream& operator=(ParsedStream&&) noexcept;
    ParsedStream(const ParsedStream&) = delete;
    ParsedStream& operator=(const ParsedStream&) = delete;

    // Parse `bytes` as a YAML stream named `file_label` (used only for parser
    // location attribution). Installs throwing ryml callbacks and lets any
    // parse error propagate as yass::yaml::ParseError; callers that want the
    // malformed-as-error behavior should use check_yaml(). The caller's bytes
    // are COPIED into the stream, so `bytes` need not outlive the result.
    static ParsedStream parse(std::string_view file_label, std::string_view bytes);

    // The top-level documents of the stream, in source order. A single-document
    // file (no leading `---` / `...` framing) yields exactly one document whose
    // node is the document's root container or scalar. A file whose content is
    // only comments / blank lines yields zero documents.
    std::vector<Node> documents() const;

    // Convenience: number of top-level documents.
    std::size_t document_count() const { return documents().size(); }

    // Stream-wide structural probes (walk the whole tree once):
    //   has_anchor_alias_or_tag — true iff ANY node anywhere carries a YAML
    //       anchor, an alias (`*ref`), or an explicit (non-default) tag, on
    //       either its key or its value. Underlies yass.yaml.anchor_or_alias.
    //   first_anchor_alias_or_tag_line — the 1-based line of the FIRST such
    //       node in document order, or nullopt if none.
    bool has_anchor_alias_or_tag() const;
    std::optional<int> first_anchor_alias_or_tag_line() const;

    // Find the first duplicate mapping key anywhere in the stream, in document
    // order, scanning each mapping's keys for a repeat. Returns the offending
    // (second-or-later) key text and its 1-based line, or nullopt if every
    // mapping has unique keys. ryml does NOT dedup keys, so both siblings are
    // present and walkable.
    struct DuplicateKey {
        std::string key;
        std::optional<int> line;
    };
    std::optional<DuplicateKey> first_duplicate_key() const;

private:
    friend class Node;
    ParsedStream();

    struct Impl;
    std::unique_ptr<Impl> impl_;

    // Node-facing accessors, defined in the .cpp against the owned ryml Tree.
    NodeKind node_kind(std::size_t id) const;
    std::string_view node_scalar(std::size_t id) const;
    bool node_value_quoted(std::size_t id) const;
    bool node_value_plain(std::size_t id) const;
    bool node_has_key(std::size_t id) const;
    std::string_view node_key(std::size_t id) const;
    bool node_key_quoted(std::size_t id) const;
    std::optional<int> node_line(std::size_t id) const;
    std::vector<Node> node_children(std::size_t id) const;
    Node make_node(std::size_t id) const;
};

// Thrown by ParsedStream::parse() when ryml reports the stream is not
// well-formed YAML 1.2. check_yaml() catches this and maps it to
// yass.yaml.malformed.
//
// `yaml_line` carries the 0-based line of the offending position in the YAML
// source buffer, as ryml's parse-error callback reported it (ErrorDataParse
// ymlloc), or nullopt when ryml could not localize the error. check_yaml clamps
// and 1-bases this to attribute the malformed diagnostic to a concrete line.
class ParseError : public std::runtime_error {
public:
    explicit ParseError(const std::string& what,
                        std::optional<std::size_t> yaml_line = std::nullopt)
        : std::runtime_error(what), yaml_line_(yaml_line) {}

    // 0-based YAML-buffer line of the error, when ryml localized it.
    std::optional<std::size_t> yaml_line() const { return yaml_line_; }

private:
    std::optional<std::size_t> yaml_line_;
};

// --------------------------------------------------------------------------
// CheckYAML result (cli.validate.CheckYAML RETURN).
// --------------------------------------------------------------------------
// EITHER a single Diagnostic (the one error this file earns under the
// preference order) OR success carrying the ParsedStream for downstream checks.
// Exactly one of the two is populated.
struct CheckYamlResult {
    bool ok = false;
    // Populated iff !ok: the single error, per the preference order.
    std::optional<yass::diag::Diagnostic> error;
    // Populated iff ok: the parsed stream, ready for CheckPreamble et al.
    std::optional<ParsedStream> stream;
};

// Run cli.validate.CheckYAML on `bytes`, attributing any error to `file_label`
// (already in cli.ErrorLine path form). Emits AT MOST ONE error, preferring in
// order: yass.yaml.not_utf8, yass.yaml.has_bom, yass.yaml.empty_file,
// yass.yaml.malformed, yass.yaml.duplicate_key, yass.yaml.anchor_or_alias. On a
// well-formed, clean file returns ok with the ParsedStream.
//
// Line attribution: duplicate_key -> the offending key's line;
// anchor_or_alias -> the first offending node's line; not_utf8/has_bom/
// empty_file/malformed -> no line (malformed carries no reliable location).
CheckYamlResult check_yaml(std::string_view file_label, std::string_view bytes);

}  // namespace yass::yaml
