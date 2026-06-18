// M7 — the `query` subcommand (implementation).
//
// See query.hpp for the spec basis and conformance policy. This file is the
// orchestrator:
//   1. parse the positional args (name + optional scope) and reject colon-in-path;
//   2. validate the scope BEFORE name lookup (scope_not_found / scope_empty);
//   3. run NameLookup across the resolved candidate files;
//   4. dispatch on the match count:
//        - 0  -> one yass.query.no_match ErrorLine to stderr, exit 1;
//        - >1 -> cli.list-format disambiguation rows to stdout, exit 0;
//        - 1  -> ExtractFragment + InlineConforms + emit to stdout, exit 0,
//                or a CONFORMS error (conforms_no_slot / conforms_unresolved) to
//                stderr with NO stdout and exit 1.
//
// All I/O goes through the supplied streams; std::exit is never called. The
// process cwd is used for path resolution, matching the reference.

#include "query.hpp"

#include <algorithm>
#include <filesystem>
#include <optional>
#include <ostream>
#include <string>
#include <vector>

#include "diag.hpp"
#include "emit.hpp"
#include "fs.hpp"
#include "model.hpp"
#include "textio.hpp"
#include "yaml.hpp"

namespace yass::query {

namespace sfs = std::filesystem;
namespace pfs = yass::fs;
using diag::Diagnostic;
using diag::ErrorCode;

namespace {

// --------------------------------------------------------------------------
// Small shared helpers.
// --------------------------------------------------------------------------

Diagnostic make_diag(std::string_view file, std::optional<int> line, ErrorCode code,
                     std::string_view arg = {}) {
    Diagnostic d;
    d.file.assign(file);
    d.line = line;
    d.code = code;
    d.message = diag::canonical_message(code, arg);
    return d;
}

void emit_error(std::ostream& err, const Diagnostic& d) {
    err << diag::format_error_line(d) << '\n';
}

// Lexically absolutize+normalize `p` against `cwd` (no realpath), mirroring the
// validate orchestrator so resolution stays consistent with discovery output.
std::string lexical_abs(std::string_view p, const sfs::path& cwd) {
    sfs::path path{std::string(p)};
    if (path.is_relative()) path = cwd / path;
    return path.lexically_normal().generic_string();
}

// CONFORMS/USES/SEE relation key recognizer (mirrors model.cpp's private one).
bool is_relation_key(std::string_view key) {
    return key == "CONFORMS" || key == "USES" || key == "SEE";
}

// cli.list description normalization: collapse every run of ASCII whitespace
// (space, tab, newline, CR, FF, VT) to a single space, strip leading/trailing
// whitespace, then NFC-normalize. A missing / empty / non-string description
// yields an empty field.
std::string normalize_description(std::string_view raw) {
    std::string collapsed;
    collapsed.reserve(raw.size());
    bool in_ws = false;
    auto is_ws = [](char c) {
        return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
    };
    for (char c : raw) {
        if (is_ws(c)) {
            in_ws = true;
            continue;
        }
        if (in_ws && !collapsed.empty()) collapsed.push_back(' ');
        in_ws = false;
        collapsed.push_back(c);
    }
    return textio::nfc(collapsed);
}

// Replace any literal tab in a path field with a single space (cli.list RETURN).
std::string detab(std::string_view s) {
    std::string out(s);
    std::replace(out.begin(), out.end(), '\t', ' ');
    return out;
}

// --------------------------------------------------------------------------
// Name matching (cli.query.NameLookup).
// --------------------------------------------------------------------------
// True iff `query_name` matches `spec_name`: a full byte-equal match, or the
// query equals the spec name with zero or more LEADING dot-separated components
// removed (a dot-aligned trailing suffix). Case-sensitive, no trimming.
bool name_matches(std::string_view query_name, std::string_view spec_name) {
    if (query_name == spec_name) return true;
    // Each candidate trailing suffix begins right after a '.' in spec_name.
    for (std::size_t i = 0; i + 1 < spec_name.size(); ++i) {
        if (spec_name[i] == '.') {
            if (spec_name.substr(i + 1) == query_name) return true;
        }
    }
    return false;
}

bool contains_whitespace(std::string_view s) {
    for (char c : s) {
        if (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v') return true;
    }
    return false;
}

// --------------------------------------------------------------------------
// Parsed-file cache: a candidate spec file's bytes + parsed stream.
// --------------------------------------------------------------------------
// Owns the bytes and the ParsedStream so Node handles stay valid while we walk
// the matched document and any CONFORMS targets in the same file.
struct ParsedFile {
    std::string bytes;
    std::optional<yaml::ParsedStream> stream;
    bool ok = false;
};

ParsedFile parse_file(const std::string& abs) {
    ParsedFile pf;
    textio::ReadResult rr = textio::read_file_bytes(abs);
    if (!rr.ok()) return pf;
    pf.bytes = std::move(rr.bytes);
    yaml::CheckYamlResult yr = yaml::check_yaml(abs, pf.bytes);
    if (!yr.ok) return pf;
    pf.stream.emplace(std::move(*yr.stream));
    pf.ok = true;
    return pf;
}

// Find the preamble description (first document's `description` scalar), or "".
std::string preamble_description(const yaml::ParsedStream& stream) {
    std::vector<yaml::Node> docs = stream.documents();
    if (docs.empty() || !docs.front().is_map()) return {};
    for (const yaml::Node& kv : docs.front().children()) {
        if (kv.key() == "description" && kv.is_scalar()) {
            return normalize_description(kv.scalar());
        }
    }
    return {};
}

// Return the spec-name of a document if it is a Spec document (a top-level
// `spec` scalar), else nullopt. Mirrors the model's notion of a Spec doc.
std::optional<std::string> doc_spec_name(const yaml::Node& doc) {
    if (!doc.is_map()) return std::nullopt;
    for (const yaml::Node& kv : doc.children()) {
        if (kv.key() == "spec") {
            if (kv.is_scalar()) return std::string(kv.scalar());
            return std::nullopt;
        }
    }
    return std::nullopt;
}

// A single NameLookup hit.
struct Match {
    std::string display;   // cli.ErrorLine path form (for rows + errors)
    std::string abs;       // lexically-normalized absolute path
    std::string spec_name; // the matched spec name as written
    std::size_t doc_index; // document index within the file (source order)
    std::string description;  // normalized preamble description (for rows)
};

// --------------------------------------------------------------------------
// Obligation walking (ExtractFragment) over a Spec document node.
// --------------------------------------------------------------------------
// A raw obligation read from the source, retaining the CONFORMS ref (if any) for
// InlineConforms. Non-recognized keys are preserved in source order.
struct RawObligation {
    bool has_norm = false;
    std::string norm_key;
    std::string norm_value;
    bool has_when = false;
    std::string when_value;
    // All references in source order (CONFORMS retained here; the inliner strips
    // CONFORMS from the carrier and consumes the target).
    std::vector<emit::Ref> refs;
    std::vector<emit::ExtraKey> extras;
    // The single CONFORMS ref carried by this obligation (first one), if any.
    std::optional<std::string> conforms_target;
    bool reference_only = false;  // >=1 ref, no normativity, no WHEN guard.
};

RawObligation read_obligation(const yaml::Node& elem) {
    RawObligation ob;
    int relation_count = 0;
    for (const yaml::Node& kv : elem.children()) {
        std::string_view key = kv.key();
        std::string value = kv.is_scalar() ? std::string(kv.scalar()) : std::string();
        if (model::normativity_from_key(key)) {
            if (!ob.has_norm) {
                ob.has_norm = true;
                ob.norm_key.assign(key);
                ob.norm_value = value;
            }
            continue;
        }
        if (key == "WHEN") {
            ob.has_when = true;
            ob.when_value = value;
            continue;
        }
        if (is_relation_key(key)) {
            ++relation_count;
            if (key == "CONFORMS" && !ob.conforms_target) {
                // The FIRST CONFORMS is the resolution target for InlineConforms.
                ob.conforms_target = value;
            }
            // ALL relations (including CONFORMS) are retained in source order.
            // For a CARRIER, build_fragment drops the resolved (first) CONFORMS
            // before emitting; for an INLINED obligation (cli.query.InlineConforms
            // RETURN: "preserve each inlined obligation as written, keep its refs"
            // and INVARIANT: "MUST-NOT recursively resolve CONFORMS within inlined
            // obligations"), the inner CONFORMS is kept here verbatim as a relation.
            ob.refs.push_back(emit::Ref{std::string(key), value});
            continue;
        }
        ob.extras.push_back(emit::ExtraKey{std::string(key), value});
    }
    ob.reference_only = relation_count > 0 && !ob.has_norm && !ob.has_when;
    return ob;
}

// The carrier's emitted relations: ALL of `raw.refs` EXCEPT the first CONFORMS
// (which is the resolution target consumed by InlineConforms and therefore
// stripped from the carrier's own output). A second CONFORMS, if any, is kept.
std::vector<emit::Ref> carrier_refs(const RawObligation& raw) {
    std::vector<emit::Ref> out;
    bool dropped_first_conforms = false;
    for (const emit::Ref& r : raw.refs) {
        if (!dropped_first_conforms && r.key == "CONFORMS") {
            dropped_first_conforms = true;
            continue;
        }
        out.push_back(r);
    }
    return out;
}

// Read all obligations of a given slot key from a Spec document node.
std::vector<RawObligation> read_slot_obligations(const yaml::Node& doc, std::string_view slot_key) {
    std::vector<RawObligation> obs;
    if (!doc.is_map()) return obs;
    for (const yaml::Node& kv : doc.children()) {
        if (kv.key() == slot_key && kv.is_seq()) {
            for (const yaml::Node& elem : kv.children()) {
                if (elem.is_map()) obs.push_back(read_obligation(elem));
            }
        }
    }
    return obs;
}

// Find the Spec document node for `spec_name` within a parsed stream.
std::optional<yaml::Node> find_spec_doc(const yaml::ParsedStream& stream,
                                        std::string_view spec_name) {
    for (const yaml::Node& doc : stream.documents()) {
        auto name = doc_spec_name(doc);
        if (name && *name == spec_name) return doc;
    }
    return std::nullopt;
}

// --------------------------------------------------------------------------
// CONFORMS resolution + inlining (cli.query.InlineConforms).
// --------------------------------------------------------------------------
// The outcome of resolving a CONFORMS ref into a list of emit-ready obligations.
struct InlineResult {
    bool ok = true;
    std::optional<Diagnostic> error;       // conforms_no_slot / conforms_unresolved
    std::vector<emit::Obligation> obligations;
};

// Convert a RawObligation to an emit::Obligation, applying an outer WHEN guard
// (from the carrier) and a provenance ref-target. Used for INLINED obligations.
emit::Obligation to_inlined(const RawObligation& raw, const std::optional<std::string>& outer_when,
                            const std::string& provenance) {
    emit::Obligation o;
    o.has_norm = raw.has_norm;
    o.norm_key = raw.norm_key;
    o.norm_value = raw.norm_value;
    o.refs = raw.refs;        // non-CONFORMS relations of the inlined obligation.
    o.extras = raw.extras;
    o.provenance = provenance;

    // Guard combination (cli.query.InlineConforms): outer + " and " + inner, or
    // whichever is present.
    if (outer_when && raw.has_when) {
        o.has_when = true;
        o.when_value = *outer_when + " and " + raw.when_value;
    } else if (outer_when) {
        o.has_when = true;
        o.when_value = *outer_when;
    } else if (raw.has_when) {
        o.has_when = true;
        o.when_value = raw.when_value;
    }
    return o;
}

// Resolve a carrier's CONFORMS target into inlined obligations. `referencing_abs`
// is the matched spec's file (for ./ and ../ refs); `project_root` anchors
// root-relative refs. `same_file_stream` is the matched file's stream (so a
// same-file target need not be re-read). On failure returns a diagnostic
// (attributed to `display_file`, no line, exit per the code).
InlineResult resolve_conforms(const std::string& target, const std::optional<std::string>& outer_when,
                              const std::string& referencing_abs, const std::string& project_root,
                              const std::string& display_file,
                              const yaml::ParsedStream& same_file_stream) {
    InlineResult res;

    // A CONFORMS target MUST address a slot (`::SLOT`); otherwise conforms_no_slot.
    auto parsed = model::parse_ref_target(target);
    if (!parsed || !parsed->has_slot || parsed->slot.empty()) {
        res.ok = false;
        res.error = make_diag(display_file, std::nullopt, ErrorCode::query_conforms_no_slot, target);
        return res;
    }
    if (!model::is_slot_keyword(parsed->slot)) {
        // An unknown slot token cannot resolve.
        res.ok = false;
        res.error = make_diag(display_file, std::nullopt, ErrorCode::query_conforms_unresolved, target);
        return res;
    }

    model::ResolvedRef resolved = model::resolve_ref(*parsed, referencing_abs, project_root);

    // Locate the target spec document, reading the target file when cross-file.
    auto fail_unresolved = [&]() {
        res.ok = false;
        res.error = make_diag(display_file, std::nullopt, ErrorCode::query_conforms_unresolved, target);
        return res;
    };

    std::optional<yaml::Node> target_doc;
    ParsedFile target_file;  // keeps a cross-file stream alive for the node walk.
    if (resolved.same_file) {
        target_doc = find_spec_doc(same_file_stream, resolved.spec_name);
        if (!target_doc) return fail_unresolved();
        auto obs = read_slot_obligations(*target_doc, parsed->slot);
        if (obs.empty()) return fail_unresolved();  // slot not declared / empty.
        for (const RawObligation& raw : obs) {
            res.obligations.push_back(to_inlined(raw, outer_when, target));
        }
        return res;
    }

    target_file = parse_file(resolved.file_path);
    if (!target_file.ok) return fail_unresolved();
    target_doc = find_spec_doc(*target_file.stream, resolved.spec_name);
    if (!target_doc) return fail_unresolved();
    auto obs = read_slot_obligations(*target_doc, parsed->slot);
    if (obs.empty()) return fail_unresolved();
    for (const RawObligation& raw : obs) {
        res.obligations.push_back(to_inlined(raw, outer_when, target));
    }
    return res;
}

// --------------------------------------------------------------------------
// Build the emit::Fragment for the single matched spec (ExtractFragment +
// InlineConforms). On a CONFORMS failure returns the diagnostic and leaves the
// fragment unusable (no partial fragment is ever emitted).
// --------------------------------------------------------------------------
struct BuildResult {
    bool ok = true;
    std::optional<Diagnostic> error;
    emit::Fragment fragment;
};

BuildResult build_fragment(const Match& match, const ParsedFile& file, const std::string& project_root) {
    BuildResult res;
    auto doc = find_spec_doc(*file.stream, match.spec_name);
    if (!doc) {
        // Unreachable: the match came from this file/stream. Treat as no fragment.
        res.ok = false;
        res.error = make_diag(match.display, std::nullopt, ErrorCode::query_no_match, match.spec_name);
        return res;
    }

    res.fragment.spec_name = match.spec_name;

    // Walk top-level keys in source order; the `spec` key produced the name, the
    // slot keys produce slot groups, everything else is excluded (preamble lives
    // in a separate document; sibling specs are separate documents).
    for (const yaml::Node& kv : doc->children()) {
        std::string_view key = kv.key();
        if (!model::slot_from_key(key)) continue;  // only the five Slot keys.
        if (!kv.is_seq()) {
            // A malformed slot value would be a validate error; emit an empty slot
            // group so the slot key is still present (graceful, never reached by a
            // validated corpus).
            res.fragment.slots.push_back(emit::SlotGroup{std::string(key), {}});
            continue;
        }
        emit::SlotGroup group;
        group.key.assign(key);

        for (const yaml::Node& elem : kv.children()) {
            if (!elem.is_map()) continue;
            RawObligation raw = read_obligation(elem);

            if (raw.conforms_target) {
                std::optional<std::string> outer_when;
                if (raw.has_when) outer_when = raw.when_value;

                InlineResult inl = resolve_conforms(*raw.conforms_target, outer_when, match.abs,
                                                    project_root, match.display, *file.stream);
                if (!inl.ok) {
                    res.ok = false;
                    res.error = inl.error;
                    return res;  // no partial fragment.
                }

                if (raw.reference_only) {
                    // Reference-only carrier: REPLACE with the inlined obligations.
                    for (emit::Obligation& o : inl.obligations) {
                        group.obligations.push_back(std::move(o));
                    }
                } else {
                    // Normative carrier: KEEP it (CONFORMS stripped), then APPEND
                    // the inlined obligations.
                    emit::Obligation carrier;
                    carrier.has_norm = raw.has_norm;
                    carrier.norm_key = raw.norm_key;
                    carrier.norm_value = raw.norm_value;
                    carrier.has_when = raw.has_when;
                    carrier.when_value = raw.when_value;
                    // The carrier keeps its non-resolved relations: all refs in
                    // source order EXCEPT the first CONFORMS (the resolution
                    // target, consumed by InlineConforms above).
                    carrier.refs = carrier_refs(raw);
                    carrier.extras = raw.extras;
                    group.obligations.push_back(std::move(carrier));
                    for (emit::Obligation& o : inl.obligations) {
                        group.obligations.push_back(std::move(o));
                    }
                }
                continue;
            }

            // No CONFORMS: emit the obligation as-is (reordered by the emitter).
            emit::Obligation o;
            o.has_norm = raw.has_norm;
            o.norm_key = raw.norm_key;
            o.norm_value = raw.norm_value;
            o.has_when = raw.has_when;
            o.when_value = raw.when_value;
            o.refs = raw.refs;
            o.extras = raw.extras;
            group.obligations.push_back(std::move(o));
        }
        res.fragment.slots.push_back(std::move(group));
    }
    return res;
}

// --------------------------------------------------------------------------
// Scope resolution (cli.query INPUT / ERROR — validated BEFORE name lookup).
// --------------------------------------------------------------------------
struct ScopeResult {
    std::vector<std::string> files;   // candidate file display paths (sorted).
    std::optional<Diagnostic> error;  // scope_not_found / scope_empty.
    bool ok() const { return !error.has_value(); }
};

// Resolve the candidate files NameLookup will search:
//   - no scope -> discover from the project root (cli.DiscoverSpecFiles default);
//   - a scope path -> validate existence (scope_not_found) then discover within
//     it (a single file or a recursive directory); zero spec files -> scope_empty.
ScopeResult resolve_scope(const std::optional<std::string>& scope, const sfs::path& cwd) {
    ScopeResult res;
    const std::string cwd_str = cwd.generic_string();

    if (!scope) {
        pfs::DiscoverResult dr = pfs::discover_spec_files({}, cwd_str);
        if (!dr.ok()) {
            // A findroot/no-files failure from the default search surfaces as the
            // discovery diagnostic (exit 2). (No scope arg was given.)
            Diagnostic d = *dr.error;
            // (cli.FindProjectRoot ERROR / cli.ErrorLine) The fs module leaves
            // the findroot.no_marker <file> empty; attribute it to the absolute
            // cwd to match the reference (`<abs-cwd>: [yass.findroot.no_marker]
            // ...`) rather than rendering the bare "yass" token.
            if (d.code == ErrorCode::findroot_no_marker && d.file.empty()) {
                d.file = cwd_str;
            }
            res.error = std::move(d);
            return res;
        }
        res.files = dr.files;
        return res;
    }

    // A scope was provided: validate existence first (scope_not_found), then
    // discover (scope_empty when zero .yass.yaml files are found within it).
    const std::string& s = *scope;
    std::string abs = lexical_abs(s, cwd);
    std::error_code ec;
    sfs::file_status st = sfs::status(abs, ec);
    if (ec || !sfs::exists(st)) {
        res.error = make_diag(diag::relativize_path(s, cwd_str), std::nullopt,
                              ErrorCode::query_scope_not_found, s);
        return res;
    }

    pfs::DiscoverResult dr = pfs::discover_spec_files(s, cwd_str);
    // A scope that exists but yields no spec files is scope_empty. discover may
    // also flag bad_extension for a non-spec file scope; the reference treats any
    // "exists but no specs" scope uniformly as scope_empty.
    if (!dr.ok() || dr.files.empty()) {
        res.error = make_diag(diag::relativize_path(s, cwd_str), std::nullopt,
                              ErrorCode::query_scope_empty, s);
        return res;
    }
    res.files = dr.files;
    return res;
}

}  // namespace

int run_query(const std::vector<std::string>& args, std::ostream& out, std::ostream& err) {
    std::error_code ec;
    sfs::path cwd = sfs::current_path(ec);
    if (ec) cwd = sfs::path(".");
    cwd = cwd.lexically_normal();
    const std::string cwd_str = cwd.generic_string();

    // (INPUT) The spec name is required.
    if (args.empty()) {
        emit_error(err, make_diag("", std::nullopt, ErrorCode::query_name_missing));
        return diag::ExitCode::USAGE;
    }
    const std::string& name = args[0];
    std::optional<std::string> scope;
    if (args.size() >= 2) scope = args[1];

    // (INPUT) Reject a scope containing a literal ':' BEFORE any other work.
    if (scope && scope->find(':') != std::string::npos) {
        emit_error(err, make_diag("", std::nullopt, ErrorCode::path_colon_in_path, *scope));
        return diag::ExitCode::USAGE;
    }

    // (NameLookup) An empty name is blank; a whitespace-bearing name is a
    // no-match (NOT blank). The scope is validated BEFORE name lookup, but the
    // blank-name check is an argument-shape error that precedes scope work.
    if (name.empty()) {
        emit_error(err, make_diag("", std::nullopt, ErrorCode::query_name_blank));
        return diag::ExitCode::USAGE;
    }

    // (ERROR) Validate the scope BEFORE performing name lookup. When the scope is
    // invalid the scope error is the only thing emitted.
    ScopeResult sc = resolve_scope(scope, cwd);
    if (!sc.ok()) {
        emit_error(err, *sc.error);
        return diag::ExitCode::USAGE;
    }
    // NOTE: unlike `list` and `validate`, the reference `query` does NOT surface
    // the non-fatal yass.discover.dir_unreadable warnings during scope discovery
    // (its stderr stays empty for an unreadable subdir). To match the reference
    // byte-for-byte, query deliberately discards sc.warnings.

    // A whitespace-bearing name is a guaranteed no-match (treated as such only
    // after the scope is validated).
    const bool whitespace_name = contains_whitespace(name);

    // (NameLookup) Walk the candidate files; collect matches in cli.list order
    // (files already code-point-sorted by discovery; specs in document order).
    pfs::FindRootResult root = pfs::find_project_root(cwd_str);
    const std::string project_root = root.ok() ? root.root : cwd_str;

    std::vector<Match> matches;
    // Cache parsed files so a single-match fragment build can reuse the stream.
    std::vector<ParsedFile> parsed(sc.files.size());

    for (std::size_t fi = 0; fi < sc.files.size(); ++fi) {
        const std::string& display = sc.files[fi];
        std::string abs = lexical_abs(display, cwd);
        ParsedFile pf = parse_file(abs);
        if (!pf.ok) {
            // An unparseable candidate contributes no specs (NameLookup cannot
            // match within it). The reference does not error on it here.
            parsed[fi] = std::move(pf);
            continue;
        }
        std::string desc;
        if (!whitespace_name) {
            // Only compute the description when we may need it for rows.
            std::vector<yaml::Node> docs = pf.stream->documents();
            std::string file_desc = preamble_description(*pf.stream);
            for (std::size_t di = 0; di < docs.size(); ++di) {
                auto sn = doc_spec_name(docs[di]);
                if (!sn) continue;
                if (name_matches(name, *sn)) {
                    Match m;
                    m.display = display;
                    m.abs = abs;
                    m.spec_name = *sn;
                    m.doc_index = di;
                    m.description = file_desc;
                    matches.push_back(std::move(m));
                }
            }
        }
        parsed[fi] = std::move(pf);
    }

    // (RETURN) Dispatch on the match count.
    if (matches.empty()) {
        // Zero matches: one ErrorLine to stderr, no stdout, exit 1.
        emit_error(err, make_diag("", std::nullopt, ErrorCode::query_no_match, name));
        return diag::ExitCode::PROCESSING;
    }

    if (matches.size() > 1) {
        // (RETURN) More than one match: cli.list-format disambiguation rows to
        // stdout (file<TAB>name<TAB>description), no truncation, no stderr, exit 0.
        for (const Match& m : matches) {
            out << detab(m.display) << '\t' << m.spec_name << '\t' << m.description << '\n';
        }
        return diag::ExitCode::SUCCESS;
    }

    // (RETURN) Exactly one match: ExtractFragment + InlineConforms + emit.
    const Match& match = matches.front();
    // Find the cached ParsedFile for the matched file.
    const ParsedFile* file = nullptr;
    for (std::size_t fi = 0; fi < sc.files.size(); ++fi) {
        if (sc.files[fi] == match.display && parsed[fi].ok) {
            file = &parsed[fi];
            break;
        }
    }
    if (!file) {
        emit_error(err, make_diag("", std::nullopt, ErrorCode::query_no_match, name));
        return diag::ExitCode::PROCESSING;
    }

    BuildResult br = build_fragment(match, *file, project_root);
    if (!br.ok) {
        // A CONFORMS failure: emit the ErrorLine, NO stdout, exit 1.
        emit_error(err, *br.error);
        return diag::ExitCode::PROCESSING;
    }

    out << emit::emit_fragment(br.fragment);
    return diag::ExitCode::SUCCESS;
}

}  // namespace yass::query
