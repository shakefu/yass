// M5 — parsed structural model (implementation).
//
// See model.hpp for the spec basis. This TU builds the ryml-free Spec/Obligation
// view from a yaml::ParsedStream, recovers document-start marker lines from the
// source bytes, and implements the RefTarget grammar + resolution.

#include "model.hpp"

#include <algorithm>
#include <array>
#include <cctype>
#include <filesystem>
#include <utility>

namespace yass::model {

namespace {

namespace fs = std::filesystem;

// ASCII lower-case a single byte.
char ascii_lower(char c) {
    if (c >= 'A' && c <= 'Z') return static_cast<char>(c - 'A' + 'a');
    return c;
}

// Case-insensitive ASCII equality.
bool iequals(std::string_view a, std::string_view b) {
    if (a.size() != b.size()) return false;
    for (std::size_t i = 0; i < a.size(); ++i) {
        if (ascii_lower(a[i]) != ascii_lower(b[i])) return false;
    }
    return true;
}

int line_or_zero(const yaml::Node& n) {
    auto l = n.line();
    return l.has_value() ? *l : 0;
}

// True iff `s` matches the YAML 1.2 core-schema null token.
bool is_core_null(std::string_view s) {
    return s.empty() || s == "~" || s == "null" || s == "Null" || s == "NULL";
}

// True iff `s` matches the YAML 1.2 core-schema bool token. (Note: yes/no/on/off
// are NOT booleans under the core schema and are plain strings — yass@Document.)
bool is_core_bool(std::string_view s) {
    return s == "true" || s == "True" || s == "TRUE" ||
           s == "false" || s == "False" || s == "FALSE";
}

bool all_digits(std::string_view s) {
    if (s.empty()) return false;
    for (char c : s) {
        if (c < '0' || c > '9') return false;
    }
    return true;
}

// True iff `s` matches the YAML 1.2 core-schema int production:
//   [-+]?[0-9]+ | 0o[0-7]+ | 0x[0-9a-fA-F]+
bool is_core_int(std::string_view s) {
    if (s.empty()) return false;
    std::string_view body = s;
    if (body[0] == '+' || body[0] == '-') body.remove_prefix(1);
    if (body.empty()) return false;
    if (body.size() > 2 && body[0] == '0' && (body[1] == 'x' || body[1] == 'X')) {
        for (std::size_t i = 2; i < body.size(); ++i) {
            char c = body[i];
            bool hex = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') ||
                       (c >= 'A' && c <= 'F');
            if (!hex) return false;
        }
        return body.size() > 2;
    }
    if (body.size() > 2 && body[0] == '0' && (body[1] == 'o' || body[1] == 'O')) {
        for (std::size_t i = 2; i < body.size(); ++i) {
            if (body[i] < '0' || body[i] > '7') return false;
        }
        return body.size() > 2;
    }
    return all_digits(body);
}

// True iff `s` matches the YAML 1.2 core-schema float production (decimal,
// exponent, .inf / .nan).
bool is_core_float(std::string_view s) {
    if (s.empty()) return false;
    std::string_view body = s;
    if (body[0] == '+' || body[0] == '-') body.remove_prefix(1);
    if (body == ".inf" || body == ".Inf" || body == ".INF") return true;
    if (s == ".nan" || s == ".NaN" || s == ".NAN") return true;
    // [0-9]*(\.[0-9]*)?([eE][-+]?[0-9]+)?  with at least one digit and either a
    // dot or an exponent to distinguish from an int.
    std::size_t i = 0;
    bool any_digit = false;
    while (i < body.size() && body[i] >= '0' && body[i] <= '9') { ++i; any_digit = true; }
    bool has_dot = false;
    if (i < body.size() && body[i] == '.') {
        has_dot = true;
        ++i;
        while (i < body.size() && body[i] >= '0' && body[i] <= '9') { ++i; any_digit = true; }
    }
    bool has_exp = false;
    if (i < body.size() && (body[i] == 'e' || body[i] == 'E')) {
        has_exp = true;
        ++i;
        if (i < body.size() && (body[i] == '+' || body[i] == '-')) ++i;
        std::size_t exp_digits = 0;
        while (i < body.size() && body[i] >= '0' && body[i] <= '9') { ++i; ++exp_digits; }
        if (exp_digits == 0) return false;
    }
    if (i != body.size()) return false;
    if (!any_digit) return false;
    return has_dot || has_exp;
}

}  // namespace

// A plain scalar resolves to a YAML string iff it is not a core-schema
// null/bool/int/float. Quoted scalars are always strings.
bool plain_scalar_is_string(std::string_view s) {
    if (is_core_null(s)) return false;
    if (is_core_bool(s)) return false;
    if (is_core_int(s)) return false;
    if (is_core_float(s)) return false;
    return true;
}

// --------------------------------------------------------------------------
// Vocabulary.
// --------------------------------------------------------------------------
std::optional<Slot> slot_from_key(std::string_view key) {
    if (key == "INPUT") return Slot::INPUT;
    if (key == "RETURN") return Slot::RETURN;
    if (key == "ERROR") return Slot::ERROR;
    if (key == "SIDE-EFFECT") return Slot::SIDE_EFFECT;
    if (key == "INVARIANT") return Slot::INVARIANT;
    return std::nullopt;
}

std::string_view slot_to_key(Slot slot) {
    switch (slot) {
        case Slot::INPUT: return "INPUT";
        case Slot::RETURN: return "RETURN";
        case Slot::ERROR: return "ERROR";
        case Slot::SIDE_EFFECT: return "SIDE-EFFECT";
        case Slot::INVARIANT: return "INVARIANT";
    }
    return "";
}

bool is_slot_keyword(std::string_view s) {
    return slot_from_key(s).has_value();
}

std::optional<Normativity> normativity_from_key(std::string_view key) {
    if (key == "MUST") return Normativity::MUST;
    if (key == "MUST-NOT") return Normativity::MUST_NOT;
    if (key == "SHOULD") return Normativity::SHOULD;
    if (key == "SHOULD-NOT") return Normativity::SHOULD_NOT;
    if (key == "MAY") return Normativity::MAY;
    return std::nullopt;
}

bool is_reserved_name(std::string_view name) {
    static constexpr std::array<std::string_view, 10> kKeywords = {
        "INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT",
        "MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"};
    for (std::string_view kw : kKeywords) {
        if (iequals(name, kw)) return true;
    }
    return false;
}

namespace {

// Recognized Reference relation keys.
std::optional<Reference::Relation> relation_from_key(std::string_view key) {
    if (key == "CONFORMS") return Reference::CONFORMS;
    if (key == "USES") return Reference::USES;
    if (key == "SEE") return Reference::SEE;
    return std::nullopt;
}

// True iff `a` is a (case-insensitive) prefix of `b`.
bool iprefix(std::string_view a, std::string_view b) {
    if (a.size() > b.size()) return false;
    for (std::size_t i = 0; i < a.size(); ++i) {
        if (ascii_lower(a[i]) != ascii_lower(b[i])) return false;
    }
    return true;
}

// True iff an UNRECOGNIZED obligation key RESEMBLES a Reference relation, in
// which case the reference yass reports yass.reference.unknown_relation rather
// than yass.normativity.unknown. Empirically (differential probe of the
// reference binary), a key resembles a relation iff, case-insensitively, it is
// at least 2 characters long AND it shares a prefix relationship with one of the
// recognized relation tokens (CONFORMS / USES / SEE) — i.e. the key is a prefix
// of a token OR a token is a prefix of the key. E.g. CONFORM, CONFORMS2, USE,
// SEEN, conforms all resemble; CONFORMING, USE_X, XYZ, single-char C/U/S do not.
bool resembles_relation_key(std::string_view key) {
    if (key.size() < 2) return false;
    static constexpr std::string_view kTokens[] = {"CONFORMS", "USES", "SEE"};
    for (std::string_view tok : kTokens) {
        if (iprefix(key, tok) || iprefix(tok, key)) return true;
    }
    return false;
}

}  // namespace

// --------------------------------------------------------------------------
// Document-start marker lines.
// --------------------------------------------------------------------------
std::vector<int> document_marker_lines(std::string_view source) {
    std::vector<int> out;
    int line = 1;
    std::size_t i = 0;
    const std::size_t n = source.size();
    while (i <= n) {
        // Determine [bol, eol) for the current line.
        std::size_t bol = i;
        std::size_t j = bol;
        while (j < n && source[j] != '\n') ++j;
        std::string_view text = source.substr(bol, j - bol);
        // Strip a trailing CR for CRLF files.
        if (!text.empty() && text.back() == '\r') text.remove_suffix(1);
        // A YAML document-start marker is a line that is exactly `---` or starts
        // with `---` followed by whitespace.
        if (text.size() >= 3 && text[0] == '-' && text[1] == '-' && text[2] == '-') {
            if (text.size() == 3 ||
                text[3] == ' ' || text[3] == '\t') {
                out.push_back(line);
            }
        }
        if (j >= n) break;
        i = j + 1;
        ++line;
    }
    return out;
}

// --------------------------------------------------------------------------
// Obligation / Spec extraction.
// --------------------------------------------------------------------------
namespace {

Obligation extract_obligation(const yaml::Node& elem) {
    Obligation ob;
    ob.line = line_or_zero(elem);
    if (!elem.is_map()) {
        // The obligation element is a scalar / sequence / null, not a mapping:
        // obligation.bad_value_shape with no recognized keys.
        ob.element_not_map = true;
        return ob;
    }
    ob.is_map = true;

    // The line a bad-value-shape error attributes to: for a map/seq value, the
    // value's first content line (the first child); for a null value, the key's
    // own line. Matches the reference yass.
    auto bad_shape_line = [](const yaml::Node& kv, int key_line) -> int {
        if (kv.is_map() || kv.is_seq()) {
            std::vector<yaml::Node> ch = kv.children();
            if (!ch.empty()) {
                auto l = ch.front().line();
                if (l.has_value()) return *l;
            }
        }
        return key_line;
    };

    std::vector<std::string> seen_relations;
    for (const yaml::Node& kv : elem.children()) {
        std::string_view key = kv.key();
        int kl = line_or_zero(kv);
        bool bad_shape = kv.is_map() || kv.is_seq() || kv.is_null();
        int bsl = bad_shape ? bad_shape_line(kv, kl) : kl;

        if (auto norm = normativity_from_key(key)) {
            ++ob.normativity_count;
            ob.normativity_lines.push_back(kl);
            if (!ob.norm.has_value()) ob.norm = *norm;
            // A normativity value that is a mapping/sequence/null is bad shape.
            if (bad_shape) {
                ob.key_issues.push_back(
                    {Obligation::KeyIssue::BadValueShape, std::string(key), bsl, 0});
            }
            continue;
        }
        if (key == "WHEN") {
            ob.has_guard = true;
            if (kv.is_scalar()) {
                ob.guard_value.assign(kv.scalar());
            } else if (bad_shape) {
                ob.key_issues.push_back(
                    {Obligation::KeyIssue::BadValueShape, std::string(key), bsl, 0});
            }
            continue;
        }
        if (auto rel = relation_from_key(key)) {
            // Track duplicate relation keys (normally caught earlier by
            // CheckYAML duplicate_key, but correct in isolation).
            std::string ks(key);
            if (std::find(seen_relations.begin(), seen_relations.end(), ks) !=
                seen_relations.end()) {
                ob.duplicate_relation_keys.push_back(ks);
            } else {
                seen_relations.push_back(ks);
            }
            Reference ref;
            ref.relation = *rel;
            ref.relation_key = ks;
            ref.line = kl;
            if (kv.is_scalar()) {
                ref.target.assign(kv.scalar());
            } else if (bad_shape) {
                ob.key_issues.push_back(
                    {Obligation::KeyIssue::BadValueShape, ks, bsl, 0});
            }
            ob.refs.push_back(std::move(ref));
            continue;
        }
        // Any other key is unrecognized. The reference distinguishes two cases:
        //   - a key that RESEMBLES a Reference relation (CONFORMS/USES/SEE prefix
        //     relationship) -> yass.reference.unknown_relation;
        //   - otherwise a candidate Normativity keyword -> yass.normativity.unknown.
        // Both attribute to the key's own line.
        auto kind = resembles_relation_key(key) ? Obligation::KeyIssue::UnknownRelation
                                                 : Obligation::KeyIssue::UnknownKey;
        ob.key_issues.push_back({kind, std::string(key), kl, 0});
    }

    // reference-only iff >=1 Reference AND no Normativity AND no WHEN guard.
    ob.reference_only =
        !ob.refs.empty() && ob.normativity_count == 0 && !ob.has_guard;
    return ob;
}

Spec build_spec(const yaml::Node& doc, int marker_line) {
    Spec spec;
    spec.doc_line = marker_line;
    spec.doc_col = 1;

    if (!doc.is_map()) {
        // A non-map document has no spec key (no_name); nothing else to extract.
        return spec;
    }

    for (const yaml::Node& kv : doc.children()) {
        std::string_view key = kv.key();
        int kl = line_or_zero(kv);

        if (key == "spec") {
            spec.has_spec_key = true;
            spec.name_line = kl;
            // The spec value is a string iff it is a quoted scalar, or a plain
            // scalar that does not resolve to a core-schema int/float/bool/null
            // (YAML 1.2 core schema; yass@Document keeps yes/no/on/off as plain
            // strings). A mapping/sequence/null value is not a string.
            if (kv.is_scalar()) {
                spec.name_is_quoted = kv.is_value_quoted();
                spec.name.assign(kv.scalar());
                spec.name_is_string =
                    spec.name_is_quoted || plain_scalar_is_string(kv.scalar());
            } else {
                // null / seq / map: not a string.
                spec.name_is_string = false;
            }
            continue;
        }

        if (auto slot = slot_from_key(key)) {
            SlotGroup grp;
            grp.slot = *slot;
            grp.key.assign(key);
            grp.line = kl;
            if (kv.is_seq()) {
                grp.value_is_list = true;
                for (const yaml::Node& elem : kv.children()) {
                    grp.obligations.push_back(extract_obligation(elem));
                }
            } else {
                grp.value_is_list = false;  // scalar/map/null => value_not_list
            }
            spec.slots.push_back(std::move(grp));
            continue;
        }

        // A top-level key other than `spec` that is not a Slot key.
        spec.unknown_keys.push_back({std::string(key), kl, 0});
    }
    return spec;
}

Document::Kind doc_kind(const yaml::Node& n) {
    switch (n.kind()) {
        case yaml::NodeKind::Map: return Document::MAP;
        case yaml::NodeKind::Seq: return Document::SEQ;
        case yaml::NodeKind::Scalar: return Document::SCALAR;
        case yaml::NodeKind::Null: return Document::NULL_;
        default: return Document::INVALID;
    }
}

// Map document content lines onto their introducing `---` marker lines.
//
// For document i with content on `content_line`, the marker is the greatest
// marker line that is <= content_line and strictly greater than the previous
// document's content line. The first document may be unmarked (then its marker
// line is its own content line).
int marker_for(const std::vector<int>& markers, int content_line, int prev_content_line) {
    int best = 0;
    for (int m : markers) {
        if (m <= content_line && m > prev_content_line) {
            best = m;  // markers are ascending; keep the last (closest) one
        }
    }
    return best != 0 ? best : content_line;
}

}  // namespace

Model extract(const yaml::ParsedStream& stream, std::string_view source) {
    Model model;
    std::vector<int> markers = document_marker_lines(source);
    std::vector<yaml::Node> docs = stream.documents();

    int prev_content = 0;
    for (std::size_t i = 0; i < docs.size(); ++i) {
        const yaml::Node& d = docs[i];
        Document info;
        info.kind = doc_kind(d);
        info.content_line = line_or_zero(d);
        info.content_col = 1;
        info.marker_line = marker_for(markers, info.content_line, prev_content);
        prev_content = info.content_line;

        // has_spec_key: a top-level `spec` key.
        if (d.is_map()) {
            for (const yaml::Node& kv : d.children()) {
                if (kv.key() == "spec") {
                    info.has_spec_key = true;
                    break;
                }
            }
        }
        model.documents.push_back(info);

        // Spec candidates: every document beyond the first.
        if (i >= 1) {
            model.specs.push_back(build_spec(d, info.marker_line));
        }
    }
    return model;
}

// --------------------------------------------------------------------------
// RefTarget grammar.
// --------------------------------------------------------------------------
namespace {

bool is_path_char(char c) {
    return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
           (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '/' || c == '-';
}
bool is_name_char(char c) {
    return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
           (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-';
}
bool is_slot_char(char c) {
    return (c >= 'A' && c <= 'Z') || c == '-';
}

}  // namespace

std::optional<RefTarget> parse_ref_target(std::string_view target) {
    // Grammar: ^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$
    RefTarget rt;
    std::size_t i = 0;
    const std::size_t n = target.size();

    // Optional path@ : find the LAST '@' (the spec/slot tokens never contain '@',
    // and the path char set excludes '@', so at most one '@' can appear in a
    // well-formed target; locate it to split path from the rest).
    std::size_t at = target.find('@');
    if (at != std::string_view::npos) {
        if (at == 0) return std::nullopt;  // empty path token
        std::string_view path = target.substr(0, at);
        for (char c : path) {
            if (!is_path_char(c)) return std::nullopt;
        }
        rt.has_path = true;
        rt.path.assign(path);
        i = at + 1;
    }

    // Spec-name token: one or more name chars, up to `::` or end.
    std::size_t name_begin = i;
    while (i < n) {
        // Stop at the slot separator `::`.
        if (target[i] == ':' && i + 1 < n && target[i + 1] == ':') break;
        if (!is_name_char(target[i])) return std::nullopt;
        ++i;
    }
    if (i == name_begin) return std::nullopt;  // empty spec name
    rt.spec_name.assign(target.substr(name_begin, i - name_begin));

    // Optional ::SLOT.
    if (i < n) {
        if (!(target[i] == ':' && i + 1 < n && target[i + 1] == ':')) {
            return std::nullopt;
        }
        i += 2;
        std::size_t slot_begin = i;
        while (i < n) {
            if (!is_slot_char(target[i])) return std::nullopt;
            ++i;
        }
        if (i == slot_begin) return std::nullopt;  // empty slot token
        rt.has_slot = true;
        rt.slot.assign(target.substr(slot_begin, i - slot_begin));
    }
    return rt;
}

ResolvedRef resolve_ref(const RefTarget& target,
                        std::string_view referencing_file_path,
                        std::string_view project_root) {
    ResolvedRef out;
    out.spec_name = target.spec_name;
    out.has_slot = target.has_slot;
    out.slot = target.slot;

    if (!target.has_path) {
        out.same_file = true;
        out.file_path.assign(referencing_file_path);
        // Normalize for stable comparison.
        out.file_path = fs::path(out.file_path).lexically_normal().string();
        return out;
    }

    // Append the literal `.yass.yaml` suffix to the path token.
    std::string rel = target.path + ".yass.yaml";

    fs::path base;
    bool relative_to_referencing =
        target.path.rfind("./", 0) == 0 || target.path.rfind("../", 0) == 0;
    if (relative_to_referencing) {
        base = fs::path(referencing_file_path).parent_path();
    } else {
        base = fs::path(project_root);
    }
    fs::path full = (base / rel).lexically_normal();
    out.file_path = full.string();
    out.same_file = false;
    return out;
}

}  // namespace yass::model
