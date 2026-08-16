// M8 — the `list` subcommand (implementation).
//
// See list.hpp for the spec basis and conformance policy. This file resolves the
// optional positional path into the discovered, sorted file set (cli.list INPUT),
// reads + parses each file (no obligation/ref validation — cli.list INVARIANT),
// emits one `file<TAB>name<TAB>description` row per Spec document in document
// order (cli.list RETURN), optionally truncating the description to the terminal
// width, and computes the process exit code (cli.ExitCode). All I/O goes through
// the supplied streams; std::exit is never called.

#include "list.hpp"

#include <algorithm>
#include <filesystem>
#include <optional>
#include <ostream>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"
#include "fs.hpp"
#include "textio.hpp"
#include "yaml.hpp"

namespace yass::list {

namespace sfs = std::filesystem;
namespace pfs = yass::fs;
using diag::Diagnostic;
using diag::ErrorCode;

namespace {

// Lexically absolutize+normalize `p` against `cwd` (no realpath / no symlink
// resolution), mirroring diag::relativize_path's lexical model so the read path
// stays consistent with the discovery output form.
std::string lexical_abs(std::string_view p, const sfs::path& cwd) {
    sfs::path path{std::string(p)};
    if (path.is_relative()) path = cwd / path;
    return path.lexically_normal().generic_string();
}

// cli.list description normalization: collapse every run of ASCII whitespace
// (space, tab, newline, CR, FF, VT) to a single ASCII space, strip leading and
// trailing whitespace, then NFC-normalize. A missing / empty / non-string
// description yields an empty field (the caller passes "" for those).
std::string normalize_description(std::string_view raw) {
    auto is_ws = [](char c) {
        return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
    };
    std::string collapsed;
    collapsed.reserve(raw.size());
    bool in_ws = false;
    for (char c : raw) {
        if (is_ws(c)) {
            in_ws = true;
            continue;
        }
        // Insert exactly one separating space between runs, never leading.
        if (in_ws && !collapsed.empty()) collapsed.push_back(' ');
        in_ws = false;
        collapsed.push_back(c);
    }
    return textio::nfc(collapsed);
}

// Replace any literal tab in the file-path field with a single ASCII space
// (cli.list RETURN). The tab is the field separator, so it must never leak into
// a field value.
std::string detab(std::string_view s) {
    std::string out(s);
    std::replace(out.begin(), out.end(), '\t', ' ');
    return out;
}

// The first document's `description` scalar, normalized, or "" when the file has
// no preamble map, no `description` key, or a non-scalar / empty description
// (cli.list RETURN: missing/empty/non-string -> empty field). The file's preamble
// description is shared by every Spec row from that file.
std::string file_description(const yaml::ParsedStream& stream) {
    std::vector<yaml::Node> docs = stream.documents();
    if (docs.empty() || !docs.front().is_map()) return {};
    for (const yaml::Node& kv : docs.front().children()) {
        if (kv.key() == "description") {
            // A non-string description (mapping / sequence) -> empty field. A null
            // description (`~` / empty) also normalizes to empty.
            if (kv.is_scalar()) return normalize_description(kv.scalar());
            return {};
        }
    }
    return {};
}

// The spec name a document contributes to a row, or nullopt when the document is
// not a Spec (no top-level `spec` key). A `spec` key whose value is a scalar (or
// the null scalar `~`) yields that raw scalar text; a `spec` whose value is a
// mapping / sequence yields the empty string but STILL a row (cli.list never
// skips a Spec document). Obligation content is never inspected (cli.list
// INVARIANT).
std::optional<std::string> doc_spec_name(const yaml::Node& doc) {
    if (!doc.is_map()) return std::nullopt;
    for (const yaml::Node& kv : doc.children()) {
        if (kv.key() == "spec") {
            // Value-bearing scalar (including the null scalar `~`/empty) -> its raw
            // text; a map/seq value -> empty name, but a row is still emitted.
            if (kv.is_scalar() || kv.is_null()) return std::string(kv.scalar());
            return std::string{};
        }
    }
    return std::nullopt;
}

// One emitted row: the detabbed display path, the spec name, and the file's
// normalized description (shared across the file's specs).
struct Row {
    std::string path;  // detabbed cli.ErrorLine display form
    std::string name;  // spec name (raw scalar text, or empty)
    std::string desc;  // normalized preamble description (or empty)
};

// Build the truncated third field for a terminal of `width` columns, per the
// cli.list WHEN-stdout-is-a-terminal obligations. The display line is measured as
//   file-path graphemes + 2 (the two tab separators) + name length + desc length,
// each tab counting as one column. Grapheme clusters are APPROXIMATED by Unicode
// code points (textio::decode_utf8): each code point counts as width 1, and
// truncation lands on a code-point boundary.
//
// Returns the bytes to place in the third field:
//   - the full description when the whole line fits (line length <= width);
//   - "" when the description is empty (no marker ever, regardless of width);
//   - "" when file-path + name + separators + the 3-char marker meets/exceeds the
//     width (no room for any content -> empty third field, no marker);
//   - otherwise the first N code points of the description (N >= 1) followed by
//     the ASCII `...` marker, chosen so the line fits the width.
std::string truncate_description(const std::string& path, const std::string& name,
                                 const std::string& desc, int width) {
    // The two tab separators count as one column each.
    const std::size_t prefix = textio::count_codepoints(path) + 2 +
                               textio::count_codepoints(name);
    const std::size_t desc_len = textio::count_codepoints(desc);
    const std::size_t full = prefix + desc_len;

    // The whole untruncated line already fits (meets-or-equals is a fit).
    if (full <= static_cast<std::size_t>(width)) return desc;

    // The description is empty -> never append a marker (and there is nothing to
    // shorten); leave the field empty.
    if (desc.empty()) return {};

    // No room for even one content code point plus the marker -> empty third
    // field, no marker. (prefix + 3 marker columns meets/exceeds the width.)
    static constexpr std::size_t kMarkerCols = 3;  // the ASCII "..." marker
    if (prefix + kMarkerCols >= static_cast<std::size_t>(width)) return {};

    // Room for `content_budget` content code points before the 3-char marker.
    const std::size_t content_budget =
        static_cast<std::size_t>(width) - prefix - kMarkerCols;

    // Take the first `content_budget` code points of the description on a
    // code-point boundary, then append the marker.
    std::vector<char32_t> cps = textio::decode_utf8(desc);
    std::size_t take = std::min(content_budget, cps.size());
    // Re-encode the kept prefix from the original bytes via a code-point walk so
    // the slice stays byte-correct (decode_utf8 is total; we re-walk the bytes).
    std::string kept;
    {
        std::size_t cp_count = 0;
        std::size_t i = 0;
        const std::string_view sv = desc;
        while (i < sv.size() && cp_count < take) {
            unsigned char b = static_cast<unsigned char>(sv[i]);
            std::size_t len = 1;
            if (b < 0x80) len = 1;
            else if ((b >> 5) == 0x6) len = 2;
            else if ((b >> 4) == 0xE) len = 3;
            else if ((b >> 3) == 0x1E) len = 4;
            else len = 1;  // defensive: malformed lead, advance one byte
            if (i + len > sv.size()) len = sv.size() - i;
            kept.append(sv.substr(i, len));
            i += len;
            ++cp_count;
        }
    }
    kept.append("...");
    return kept;
}

// Emit one error line + '\n' to `err`.
void emit_error(std::ostream& err, const Diagnostic& d) {
    err << diag::format_error_line(d) << '\n';
}

}  // namespace

int run_list(const std::vector<std::string>& args, std::ostream& out, std::ostream& err,
             int tty_width) {
    std::error_code ec;
    sfs::path cwd = sfs::current_path(ec);
    if (ec) cwd = sfs::path(".");
    cwd = cwd.lexically_normal();
    const std::string cwd_str = cwd.generic_string();

    // (INPUT) Reject any positional argument containing a literal ':' BEFORE any
    // discovery work — the reference rejects colon ahead of path classification.
    // `-` is treated literally here (the dispatcher intercepts the stdin marker).
    for (const std::string& a : args) {
        if (a.find(':') != std::string::npos) {
            Diagnostic d;
            d.file.clear();  // -> the bare "yass" token in the ErrorLine.
            d.line = std::nullopt;
            d.code = ErrorCode::path_colon_in_path;
            d.message = diag::canonical_message(ErrorCode::path_colon_in_path, a);
            emit_error(err, d);
            return diag::ExitCode::USAGE;
        }
    }

    // (INPUT) Route EVERY input through cli.DiscoverSpecFiles for existence /
    // extension / fs-error checks. No positional -> empty arg (discover from the
    // project root). A discovery error (not_found / bad_extension / unreadable /
    // invalid_type / findroot) is fatal: one ErrorLine, exit 2.
    const std::string arg = args.empty() ? std::string{} : args.front();
    pfs::DiscoverResult dr = pfs::discover_spec_files(arg, cwd_str);
    if (!dr.ok()) {
        Diagnostic d = *dr.error;
        // (cli.FindProjectRoot ERROR / cli.ErrorLine) The default-search
        // findroot.no_marker diagnostic carries an empty <file> from the fs
        // module; attribute it to the absolute cwd, matching the reference
        // (`<abs-cwd>: [yass.findroot.no_marker] ...`) instead of the bare
        // "yass" token. Other discovery errors already carry their own <file>.
        if (d.code == ErrorCode::findroot_no_marker && d.file.empty()) {
            d.file = cwd_str;
        }
        emit_error(err, d);
        return diag::ExitCode::USAGE;
    }

    // (ERROR / cli.DiscoverSpecFiles) Emit any non-fatal per-directory warnings
    // (yass.discover.dir_unreadable) to stderr; recursion continued past them so
    // the discovered files are still listed. These do NOT change the exit code.
    for (const Diagnostic& w : dr.warnings) {
        emit_error(err, w);
    }

    // (RETURN) No .yass.yaml files in scope -> exit 0, no output. discover already
    // returns its `files` in NFC code-point order, so iterating them honors the
    // file sort with no re-sorting needed.
    bool any_parse_failure = false;
    for (const std::string& display : dr.files) {
        const std::string abs = lexical_abs(display, cwd);

        textio::ReadResult rr = textio::read_file_bytes(abs);
        if (!rr.ok()) {
            // A discovered file we cannot read is surfaced as a malformed-YAML
            // ErrorLine (one per file) and we continue (cli.list ERROR).
            Diagnostic d;
            d.file = display;
            d.line = std::nullopt;
            d.code = ErrorCode::yaml_malformed;
            d.message = diag::canonical_message(ErrorCode::yaml_malformed);
            emit_error(err, d);
            any_parse_failure = true;
            continue;
        }

        yaml::CheckYamlResult yr = yaml::check_yaml(display, rr.bytes);
        if (!yr.ok) {
            // (ERROR) A discovered file failing YAML parse: one ErrorLine with the
            // single CheckYAML error (code/line/exit match the reference; prose per
            // spec). Continue listing the remaining files; the overall exit is 1.
            emit_error(err, *yr.error);
            any_parse_failure = true;
            continue;
        }

        const yaml::ParsedStream& stream = *yr.stream;
        const std::string desc = file_description(stream);
        const std::string path_field = detab(display);

        // (RETURN) One row per Spec document, in document order. Every document
        // carrying a top-level `spec` key contributes a row (the preamble document
        // typically does not); a Spec with a non-scalar `spec` value still emits a
        // row with an empty name field.
        for (const yaml::Node& doc : stream.documents()) {
            std::optional<std::string> name = doc_spec_name(doc);
            if (!name) continue;

            std::string third = desc;
            if (tty_width > 0) {
                third = truncate_description(path_field, *name, desc, tty_width);
            }
            // (RETURN) Always emit BOTH tab separators, even for an empty name or
            // empty description; terminate every row with exactly one LF.
            out << path_field << '\t' << *name << '\t' << third << '\n';
        }
    }

    // (RETURN) exit 1 when any discovered file failed to parse; otherwise exit 0
    // (even when zero rows were emitted).
    return any_parse_failure ? diag::ExitCode::PROCESSING : diag::ExitCode::SUCCESS;
}

}  // namespace yass::list
