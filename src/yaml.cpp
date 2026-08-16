// M3 — YAML well-formedness + parsed-stream access (implementation).
//
// See yaml.hpp for the spec basis. This TU owns the only direct dependency on
// the third-party ryml parser; the public header exposes a ryml-free façade.

#include "yaml.hpp"

#include <string>
#include <vector>

#include "rapidyaml.hpp"

#include "textio.hpp"

namespace yass::yaml {

namespace {

// ryml requires error callbacks that DO NOT return (they must interrupt). We
// throw a C++ exception that ParsedStream::parse lets propagate and check_yaml
// catches. Both the parse-error and basic-error hooks funnel here.
//
// The parse-error hook captures the YAML-buffer location (ErrorDataParse.ymlloc)
// so check_yaml can attribute yass.yaml.malformed to the offending line, per the
// cli.validate.CheckYAML / cli.ErrorLine line-attribution requirement. ryml's
// Location.line is 0-based; it is carried raw here and converted/clamped where
// the diagnostic is built. A line of npos means ryml could not localize.
[[noreturn]] void on_parse_error(c4::csubstr msg, ryml::ErrorDataParse const& ed, void* /*ud*/) {
    std::optional<std::size_t> line;
    if (ed.ymlloc.line != c4::yml::npos) {
        line = ed.ymlloc.line;
    }
    throw ParseError(std::string(msg.str, msg.len), line);
}
[[noreturn]] void on_basic_error(c4::csubstr msg, ryml::ErrorDataBasic const& /*ed*/, void* /*ud*/) {
    throw ParseError(std::string(msg.str, msg.len));
}

c4::csubstr to_csub(std::string_view s) {
    return c4::csubstr(s.data(), s.size());
}
std::string_view from_csub(c4::csubstr s) {
    return std::string_view(s.str, s.len);
}

}  // namespace

// --------------------------------------------------------------------------
// ParsedStream::Impl — owns the parser, the tree, and the source buffers.
// --------------------------------------------------------------------------
// The byte buffer and the file label are stored so the csubstr views handed to
// the parser stay valid for the stream's lifetime, even though parse_in_arena
// copies scalar text into the tree's own arena.
struct ParsedStream::Impl {
    std::string label;
    std::string bytes;
    ryml::Callbacks callbacks{};
    // EventHandlerTree + Parser must outlive any t.location() call, so they are
    // owned here alongside the Tree. unique_ptr keeps Impl movable/stable.
    std::unique_ptr<ryml::EventHandlerTree> evt;
    std::unique_ptr<ryml::Parser> parser;
    ryml::Tree tree;
};

ParsedStream::ParsedStream() : impl_(std::make_unique<Impl>()) {}
ParsedStream::~ParsedStream() = default;
ParsedStream::ParsedStream(ParsedStream&&) noexcept = default;
ParsedStream& ParsedStream::operator=(ParsedStream&&) noexcept = default;

ParsedStream ParsedStream::parse(std::string_view file_label, std::string_view bytes) {
    ParsedStream ps;
    Impl& impl = *ps.impl_;
    impl.label.assign(file_label);
    impl.bytes.assign(bytes);

    // Install throwing callbacks (global + per-parser). EventHandlerTree and
    // Parser are constructed with the same callbacks so errors during parse and
    // during tree construction both throw.
    impl.callbacks = ryml::Callbacks();
    impl.callbacks.m_error_parse = &on_parse_error;
    impl.callbacks.m_error_basic = &on_basic_error;
    ryml::set_callbacks(impl.callbacks);

    impl.evt = std::make_unique<ryml::EventHandlerTree>(impl.callbacks);
    impl.parser = std::make_unique<ryml::Parser>(impl.evt.get(),
                                                 ryml::ParserOptions().locations(true));
    impl.tree = ryml::parse_in_arena(impl.parser.get(),
                                     to_csub(impl.label),
                                     to_csub(impl.bytes));
    return ps;
}

// --------------------------------------------------------------------------
// ParsedStream — Node-facing accessors against the owned ryml Tree.
// --------------------------------------------------------------------------
Node ParsedStream::make_node(std::size_t id) const {
    return Node(this, id);
}

NodeKind ParsedStream::node_kind(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size()) return NodeKind::Invalid;
    if (t.is_map(nid)) return NodeKind::Map;
    if (t.is_seq(nid)) return NodeKind::Seq;
    if (t.has_val(nid)) {
        // A null scalar (`~`, `null`, or an empty value position) reports as
        // Null; any other value-bearing node is a Scalar.
        if (t.val_is_null(nid)) return NodeKind::Null;
        return NodeKind::Scalar;
    }
    return NodeKind::Invalid;
}

std::string_view ParsedStream::node_scalar(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.has_val(nid)) return {};
    return from_csub(t.val(nid));
}

bool ParsedStream::node_value_quoted(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.has_val(nid)) return false;
    return t.is_val_quoted(nid);
}

bool ParsedStream::node_value_plain(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.has_val(nid)) return false;
    return t.is_val_plain(nid);
}

bool ParsedStream::node_has_key(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size()) return false;
    return t.has_key(nid);
}

std::string_view ParsedStream::node_key(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.has_key(nid)) return {};
    return from_csub(t.key(nid));
}

bool ParsedStream::node_key_quoted(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.has_key(nid)) return false;
    return t.is_key_quoted(nid);
}

std::optional<int> ParsedStream::node_line(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size()) return std::nullopt;
    // ryml reports 0-based lines; cli.ErrorLine wants 1-based.
    ryml::Location loc = t.location(*impl_->parser, nid);
    return static_cast<int>(loc.line) + 1;
}

std::vector<Node> ParsedStream::node_children(std::size_t id) const {
    const ryml::Tree& t = impl_->tree;
    std::vector<Node> out;
    auto nid = static_cast<ryml::id_type>(id);
    if (id == Node::kNone || nid >= t.size() || !t.is_container(nid)) return out;
    for (auto c = t.first_child(nid); c != ryml::NONE; c = t.next_sibling(c)) {
        out.push_back(make_node(static_cast<std::size_t>(c)));
    }
    return out;
}

std::vector<Node> ParsedStream::documents() const {
    const ryml::Tree& t = impl_->tree;
    std::vector<Node> out;
    if (t.empty()) return out;
    auto rid = t.root_id();
    if (t.is_stream(rid)) {
        // Multi-document stream: each child is a DOC.
        for (auto d = t.first_child(rid); d != ryml::NONE; d = t.next_sibling(d)) {
            out.push_back(make_node(static_cast<std::size_t>(d)));
        }
        return out;
    }
    // Single document with no `---` framing. The root is the document's content
    // (a map, seq, or scalar). A content-free file (only comments / blanks)
    // leaves the root with no type -> zero documents.
    if (t.is_map(rid) || t.is_seq(rid) || t.has_val(rid)) {
        out.push_back(make_node(static_cast<std::size_t>(rid)));
    }
    return out;
}

// --------------------------------------------------------------------------
// Stream-wide structural probes.
// --------------------------------------------------------------------------
namespace {

// True iff node `nid` itself carries an anchor, alias, or explicit tag on its
// key or value. Default (resolved) tags are NOT flagged: ryml only sets the tag
// flags for tags written in the source, which is exactly the "non-default tag"
// the spec prohibits.
bool node_marks_anchor_alias_or_tag(const ryml::Tree& t, ryml::id_type nid) {
    return t.has_val_anchor(nid) || t.has_key_anchor(nid) ||
           t.is_val_ref(nid) || t.is_key_ref(nid) ||
           t.has_val_tag(nid) || t.has_key_tag(nid);
}

// Depth-first, document-order walk. Returns the id of the first node that marks
// an anchor/alias/tag, or ryml::NONE.
ryml::id_type find_first_marked(const ryml::Tree& t, ryml::id_type nid) {
    if (nid == ryml::NONE) return ryml::NONE;
    if (node_marks_anchor_alias_or_tag(t, nid)) return nid;
    for (auto c = t.first_child(nid); c != ryml::NONE; c = t.next_sibling(c)) {
        auto found = find_first_marked(t, c);
        if (found != ryml::NONE) return found;
    }
    return ryml::NONE;
}

}  // namespace

bool ParsedStream::has_anchor_alias_or_tag() const {
    return first_anchor_alias_or_tag_line().has_value();
}

std::optional<int> ParsedStream::first_anchor_alias_or_tag_line() const {
    const ryml::Tree& t = impl_->tree;
    if (t.empty()) return std::nullopt;
    auto found = find_first_marked(t, t.root_id());
    if (found == ryml::NONE) return std::nullopt;
    return node_line(static_cast<std::size_t>(found));
}

std::optional<ParsedStream::DuplicateKey> ParsedStream::first_duplicate_key() const {
    const ryml::Tree& t = impl_->tree;
    if (t.empty()) return std::nullopt;

    // Walk every container in document order. For each mapping, scan its members
    // left-to-right; the first key equal to an earlier key in the SAME mapping
    // is the duplicate, attributed to its own (later) line. The recursive
    // pre-order walk keeps the earliest duplicate by line across all mappings.
    std::optional<DuplicateKey> result;
    auto visit = [&](auto&& self, ryml::id_type nid) -> void {
        if (nid == ryml::NONE) return;
        if (t.is_map(nid)) {
            std::vector<c4::csubstr> seen;
            for (auto c = t.first_child(nid); c != ryml::NONE; c = t.next_sibling(c)) {
                if (t.has_key(c)) {
                    c4::csubstr k = t.key(c);
                    bool dup = false;
                    for (const auto& s : seen) {
                        if (s == k) { dup = true; break; }
                    }
                    if (dup) {
                        DuplicateKey hit;
                        hit.key.assign(k.str, k.len);
                        hit.line = node_line(static_cast<std::size_t>(c));
                        // Earliest by line wins; ties keep the first found.
                        if (!result.has_value() ||
                            (hit.line.has_value() &&
                             (!result->line.has_value() || *hit.line < *result->line))) {
                            result = hit;
                        }
                    } else {
                        seen.push_back(k);
                    }
                }
                self(self, c);
            }
            return;
        }
        for (auto c = t.first_child(nid); c != ryml::NONE; c = t.next_sibling(c)) {
            self(self, c);
        }
    };
    visit(visit, t.root_id());
    return result;
}

// --------------------------------------------------------------------------
// Node — forwards to its owning ParsedStream.
// --------------------------------------------------------------------------
NodeKind Node::kind() const {
    if (!valid()) return NodeKind::Invalid;
    return stream_->node_kind(id_);
}
std::string_view Node::scalar() const {
    if (!valid()) return {};
    return stream_->node_scalar(id_);
}
bool Node::is_value_quoted() const {
    if (!valid()) return false;
    return stream_->node_value_quoted(id_);
}
bool Node::is_value_plain() const {
    if (!valid()) return false;
    return stream_->node_value_plain(id_);
}
bool Node::has_key() const {
    if (!valid()) return false;
    return stream_->node_has_key(id_);
}
std::string_view Node::key() const {
    if (!valid()) return {};
    return stream_->node_key(id_);
}
bool Node::is_key_quoted() const {
    if (!valid()) return false;
    return stream_->node_key_quoted(id_);
}
std::optional<int> Node::line() const {
    if (!valid()) return std::nullopt;
    return stream_->node_line(id_);
}
std::vector<Node> Node::children() const {
    if (!valid()) return {};
    return stream_->node_children(id_);
}

// --------------------------------------------------------------------------
// check_yaml — cli.validate.CheckYAML.
// --------------------------------------------------------------------------
namespace {

yass::diag::Diagnostic make_diag(std::string_view file_label,
                                 yass::diag::ErrorCode code,
                                 std::optional<int> line,
                                 std::string_view arg = {}) {
    yass::diag::Diagnostic d;
    d.file.assign(file_label);
    d.line = line;
    d.code = code;
    d.message = yass::diag::canonical_message(code, arg);
    return d;
}

// Count the lines in `bytes` for malformed-line clamping. A non-empty file with
// no content after its final newline counts that newline as terminating the last
// line (so "a\n" is one line, "a\nb" is two). An empty file is zero lines.
int count_lines(std::string_view bytes) {
    if (bytes.empty()) return 0;
    int lines = 1;
    for (std::size_t i = 0; i + 1 < bytes.size(); ++i) {
        if (bytes[i] == '\n') ++lines;
    }
    // A trailing newline does not open a new (empty) line.
    return lines;
}

// Map ryml's 0-based parse-error line onto the 1-based line cli.ErrorLine emits,
// clamped to the file's actual line count. ryml frequently reports the position
// just past the offending token (one line beyond), which the diagnostic line
// must not exceed the file for; clamping keeps attribution inside the source.
std::optional<int> malformed_line(std::optional<std::size_t> ryml_line,
                                  std::string_view bytes) {
    if (!ryml_line.has_value()) return std::nullopt;
    int total = count_lines(bytes);
    if (total <= 0) return std::nullopt;
    long long ln = static_cast<long long>(*ryml_line);
    if (ln < 1) ln = 1;
    if (ln > total) ln = total;
    return static_cast<int>(ln);
}

}  // namespace

CheckYamlResult check_yaml(std::string_view file_label, std::string_view bytes) {
    using yass::diag::ErrorCode;
    CheckYamlResult result;

    // Preference order, byte-level checks first.
    // 1) not_utf8 — strict UTF-8 well-formedness (BOM is valid UTF-8 here).
    if (!yass::textio::is_valid_utf8(bytes)) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_not_utf8, std::nullopt);
        return result;
    }
    // 2) has_bom — a leading EF BB BF.
    if (yass::textio::has_utf8_bom(bytes)) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_has_bom, std::nullopt);
        return result;
    }
    // 3) empty_file — zero bytes.
    if (bytes.empty()) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_empty_file, std::nullopt);
        return result;
    }

    // 4) malformed — ryml parse throws. No reliable line is attributed.
    std::optional<ParsedStream> stream;
    try {
        stream.emplace(ParsedStream::parse(file_label, bytes));
    } catch (const ParseError& e) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_malformed,
                                 malformed_line(e.yaml_line(), bytes));
        return result;
    } catch (const std::exception&) {
        // Any other ryml-originated failure during parse is treated as a
        // well-formedness error (cli.validate: torn/parse failure -> malformed).
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_malformed, std::nullopt);
        return result;
    }

    // 5) duplicate_key — a mapping with a repeated key, at the offending line.
    if (auto dup = stream->first_duplicate_key()) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_duplicate_key, dup->line, dup->key);
        return result;
    }

    // 6) anchor_or_alias — any anchor, alias, or explicit tag, at its line.
    if (auto line = stream->first_anchor_alias_or_tag_line()) {
        result.ok = false;
        result.error = make_diag(file_label, ErrorCode::yaml_anchor_or_alias, line);
        return result;
    }

    // Well-formed and clean: success carries the parsed stream.
    result.ok = true;
    result.stream = std::move(stream);
    return result;
}

}  // namespace yass::yaml
