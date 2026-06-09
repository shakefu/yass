// M7 — the OutputProfile YAML emitter (implementation).
//
// See emit.hpp for the spec basis. This file is the pure byte-exact formatter:
// it takes an emit::Fragment (already inlined / reordered by src/query.cpp) and
// renders the cli.query.OutputProfile profile. The only nontrivial logic here is
// the scalar-quoting predicate, which recomputes quoting from the decoded value
// content per cli.query.OutputProfile RETURN.

#include "emit.hpp"

#include <cctype>
#include <string>

namespace yass::emit {

namespace {

// --------------------------------------------------------------------------
// YAML 1.2 core-schema token recognizers (cli.query.OutputProfile RETURN).
// --------------------------------------------------------------------------

bool is_ascii_space(char c) {
    return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// One of the core-schema null / bool tokens, OR yes/no/on/off, each matched
// case-insensitively per the spec's explicit token list.
bool is_core_word_token(std::string_view s) {
    auto ieq = [&](std::string_view w) {
        if (s.size() != w.size()) return false;
        for (std::size_t i = 0; i < s.size(); ++i) {
            char a = s[i];
            char b = w[i];
            if (a >= 'A' && a <= 'Z') a = static_cast<char>(a - 'A' + 'a');
            if (b >= 'A' && b <= 'Z') b = static_cast<char>(b - 'A' + 'a');
            if (a != b) return false;
        }
        return true;
    };
    return ieq("null") || ieq("true") || ieq("false") || ieq("yes") || ieq("no") ||
           ieq("on") || ieq("off");
}

bool all_digits(std::string_view s) {
    if (s.empty()) return false;
    for (char c : s) {
        if (c < '0' || c > '9') return false;
    }
    return true;
}

// A YAML 1.2 core-schema numeric literal: decimal/hex/octal int, or a float
// (optional sign, optional fraction, optional exponent), or the inf / nan forms.
bool is_core_numeric(std::string_view s) {
    if (s.empty()) return false;

    // .inf / .nan family (with optional leading sign for inf).
    {
        std::string_view t = s;
        if (t.size() >= 2 && (t[0] == '+' || t[0] == '-')) {
            // Only .inf may carry a sign.
        }
        auto ieq = [](std::string_view a, std::string_view b) {
            if (a.size() != b.size()) return false;
            for (std::size_t i = 0; i < a.size(); ++i) {
                char x = a[i], y = b[i];
                if (x >= 'A' && x <= 'Z') x = static_cast<char>(x - 'A' + 'a');
                if (y >= 'A' && y <= 'Z') y = static_cast<char>(y - 'A' + 'a');
                if (x != y) return false;
            }
            return true;
        };
        std::string_view body = s;
        if (!body.empty() && (body[0] == '+' || body[0] == '-')) body.remove_prefix(1);
        if (ieq(body, ".inf")) return true;
        if (ieq(s, ".nan")) return true;
    }

    // Hex / octal integer (no sign in YAML 1.2 core schema).
    if (s.size() > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X')) {
        for (std::size_t i = 2; i < s.size(); ++i) {
            char c = s[i];
            bool hex = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F');
            if (!hex) return false;
        }
        return true;
    }
    if (s.size() > 2 && s[0] == '0' && (s[1] == 'o' || s[1] == 'O')) {
        for (std::size_t i = 2; i < s.size(); ++i) {
            if (s[i] < '0' || s[i] > '7') return false;
        }
        return true;
    }

    // Decimal int / float with optional sign, fraction, and exponent.
    std::string_view t = s;
    if (!t.empty() && (t[0] == '+' || t[0] == '-')) t.remove_prefix(1);
    if (t.empty()) return false;

    // Split off an optional exponent.
    std::string_view mantissa = t;
    std::string_view exponent;
    for (std::size_t i = 0; i < t.size(); ++i) {
        if (t[i] == 'e' || t[i] == 'E') {
            mantissa = t.substr(0, i);
            exponent = t.substr(i + 1);
            break;
        }
    }
    if (!exponent.empty()) {
        std::string_view e = exponent;
        if (e[0] == '+' || e[0] == '-') e.remove_prefix(1);
        if (!all_digits(e)) return false;
    } else {
        // No 'e'/'E' present is fine; but if the loop never split, mantissa==t.
    }
    if (mantissa.empty()) return false;

    // Mantissa: digits, optional single '.', digits. Forms: D, D., .D, D.D.
    std::size_t dot = std::string_view::npos;
    for (std::size_t i = 0; i < mantissa.size(); ++i) {
        if (mantissa[i] == '.') {
            if (dot != std::string_view::npos) return false;  // two dots
            dot = i;
        }
    }
    if (dot == std::string_view::npos) {
        return all_digits(mantissa);  // pure integer (decimal)
    }
    std::string_view before = mantissa.substr(0, dot);
    std::string_view after = mantissa.substr(dot + 1);
    if (before.empty() && after.empty()) return false;  // lone "."
    if (!before.empty() && !all_digits(before)) return false;
    if (!after.empty() && !all_digits(after)) return false;
    return true;
}

}  // namespace

// True iff `c` forces a double-quoted scalar. Per cli.query.OutputProfile RETURN
// the enumerated quoting triggers do NOT include interior control bytes (other
// than line breaks) nor interior double-quotes: only a line break (LF or CR)
// cannot appear verbatim in a single-line plain scalar and therefore forces
// quoting. Matching the reference, interior TAB / FF / VT / ESC / DEL and an
// interior `"` are emitted PLAIN (NOT quoted). A literal backslash also does not
// force quoting (in a plain scalar it is just a backslash); it IS escaped when
// the scalar is double-quoted for another reason.
bool needs_quote_char(char c) {
    return c == '\n' || c == '\r';
}

bool needs_double_quote(std::string_view value) {
    if (value.empty()) return true;  // empty scalar must be quoted to be a string.

    // Any character that cannot live in a plain scalar (a decoded control byte or
    // a double-quote) forces a double-quoted, escaped scalar.
    for (char c : value) {
        if (needs_quote_char(c)) return true;
    }

    // Leading / trailing ASCII whitespace.
    if (is_ascii_space(value.front()) || is_ascii_space(value.back())) return true;

    // First character is a YAML indicator that would change the node's meaning.
    switch (value.front()) {
        case '?':
        case '-':
        case '*':
        case '&':
        case '!':
        case '|':
        case '>':
        case '%':
        case '@':
            return true;
        default:
            break;
    }

    // Contains a colon-space anywhere (would be read as a mapping entry).
    for (std::size_t i = 0; i + 1 < value.size(); ++i) {
        if (value[i] == ':' && value[i + 1] == ' ') return true;
    }

    // Matches a YAML 1.2 core-schema type token (bool/null/yes/no/on/off or a
    // numeric literal) and would otherwise resolve to a non-string.
    if (value == "~") return true;
    if (is_core_word_token(value)) return true;
    if (is_core_numeric(value)) return true;

    return false;
}

std::string emit_scalar(std::string_view value) {
    if (!needs_double_quote(value)) return std::string(value);
    std::string out;
    out.reserve(value.size() + 2);
    out.push_back('"');
    for (char c : value) {
        switch (c) {
            case '\\': out.append("\\\\"); break;
            case '"': out.append("\\\""); break;
            case '\n': out.append("\\n"); break;
            case '\t': out.append("\\t"); break;
            case '\r': out.append("\\r"); break;
            case '\f': out.append("\\f"); break;
            case '\v': out.append("\\v"); break;
            case '\0': out.append("\\0"); break;
            default:
                out.push_back(c);
                break;
        }
    }
    out.push_back('"');
    return out;
}

namespace {

// Append one obligation body. `indent` is the column the continuation keys align
// to (2 spaces under a top-level slot). The `- ` prefix is written by the
// caller; here we render the key/value lines in OutputProfile order.
void emit_obligation(std::string& out, const Obligation& ob, const std::string& indent) {
    // Provenance comment at column zero, immediately above the `- ` line.
    if (ob.provenance) {
        out.append("# CONFORMS: ");
        out.append(*ob.provenance);
        out.push_back('\n');
    }

    bool first = true;  // the first key shares the `- ` line.
    auto write_key = [&](std::string_view key, std::string_view value) {
        if (first) {
            out.append("- ");
            first = false;
        } else {
            out.append(indent);
        }
        out.append(key);
        out.append(": ");
        out.append(emit_scalar(value));
        out.push_back('\n');
    };

    // (1) Normativity keyword, (2) WHEN guard, (3) Reference relations, (4) any
    // remaining keys — the cli.query.OutputProfile reorder.
    if (ob.has_norm) write_key(ob.norm_key, ob.norm_value);
    if (ob.has_when) write_key("WHEN", ob.when_value);
    for (const Ref& r : ob.refs) write_key(r.key, r.value);
    for (const ExtraKey& e : ob.extras) write_key(e.key, e.value);

    // An obligation must emit at least one line; a fully empty obligation is not
    // reachable from a well-formed spec, but guard against silent loss anyway.
    if (first) {
        out.append("- {}\n");
    }
}

}  // namespace

std::string emit_fragment(const Fragment& fragment) {
    std::string out;
    out.append("---\n");
    out.append("spec: ");
    out.append(emit_scalar(fragment.spec_name));
    out.push_back('\n');

    const std::string indent = "  ";  // 2-space block indent for obligation keys.
    for (const SlotGroup& slot : fragment.slots) {
        out.append(slot.key);
        out.append(":\n");
        for (const Obligation& ob : slot.obligations) {
            emit_obligation(out, ob, indent);
        }
    }
    return out;
}

}  // namespace yass::emit
