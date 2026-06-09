// M5 — structural validation checks (implementation).
//
// See check.hpp for the spec basis. Each check returns diagnostics in spec
// order; message prose comes from diag::canonical_message (spec PROSE) while
// error code, exit, line, and column are pinned to the reference yass.

#include "check.hpp"

#include <algorithm>
#include <filesystem>
#include <set>
#include <unordered_map>
#include <utility>

#include "textio.hpp"

namespace yass::check {

namespace fs = std::filesystem;
using diag::Diagnostic;
using diag::ErrorCode;

namespace {

Diagnostic make(std::string_view file, std::optional<int> line, ErrorCode code,
                std::string_view arg = {}) {
    Diagnostic d;
    d.file.assign(file);
    d.line = line;
    d.code = code;
    d.message = diag::canonical_message(code, arg);
    return d;
}

std::optional<int> some(int line) {
    if (line <= 0) return std::nullopt;
    return line;
}

// True iff `name` matches the spec-name composition regex
//   ^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$
// which forbids leading/trailing `.` and consecutive `.`. Assumes `name` is
// already known to contain only allowed characters [A-Za-z0-9._-].
bool matches_composition(std::string_view name) {
    if (name.empty()) return false;
    bool segment_empty = true;  // current dot-delimited segment is empty so far
    for (char c : name) {
        if (c == '.') {
            if (segment_empty) return false;  // leading dot or consecutive dots
            segment_empty = true;
        } else {
            segment_empty = false;
        }
    }
    return !segment_empty;  // trailing dot leaves an empty final segment
}

// True iff every character of `name` is in the allowed set [A-Za-z0-9._-].
bool only_allowed_chars(std::string_view name) {
    for (char c : name) {
        bool ok = (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
                  (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-';
        if (!ok) return false;
    }
    return true;
}

}  // namespace

// --------------------------------------------------------------------------
// CheckPreamble
// --------------------------------------------------------------------------
std::optional<Diagnostic> check_preamble(std::string_view file_label,
                                         const yaml::ParsedStream& stream,
                                         std::string_view source) {
    std::vector<yaml::Node> docs = stream.documents();
    std::vector<int> markers = model::document_marker_lines(source);

    // (2) empty_stream — zero documents. Checked before "missing" so a truly
    // empty stream reports empty_stream rather than missing. (1) has_spec_key is
    // checked first below because it only applies when there IS a first document.
    if (docs.empty()) {
        return make(file_label, std::nullopt, ErrorCode::yaml_empty_stream);
    }

    // Helper: 1-based marker line for document i.
    auto marker_line = [&](std::size_t i) -> int {
        // Reuse the model's mapping logic by walking content lines.
        int prev_content = 0;
        for (std::size_t k = 0; k < docs.size(); ++k) {
            int content = docs[k].line().value_or(0);
            int best = 0;
            for (int m : markers) {
                if (m <= content && m > prev_content) best = m;
            }
            int ml = best != 0 ? best : content;
            if (k == i) return ml;
            prev_content = content;
        }
        return 0;
    };

    auto is_preamble_shape = [](const yaml::Node& d) {
        // A Preamble is a YAML mapping without a `spec` key.
        if (!d.is_map()) return false;
        for (const yaml::Node& kv : d.children()) {
            if (kv.key() == "spec") return false;
        }
        return true;
    };
    auto has_spec_key = [](const yaml::Node& d) {
        if (!d.is_map()) return false;
        for (const yaml::Node& kv : d.children()) {
            if (kv.key() == "spec") return true;
        }
        return false;
    };

    const yaml::Node& first = docs[0];

    // (1) has_spec_key — the first document carries a `spec` key.
    if (has_spec_key(first)) {
        return make(file_label, some(marker_line(0)), ErrorCode::preamble_has_spec_key);
    }

    // (3) missing — the first document is not a Preamble shape (a scalar / seq).
    if (!is_preamble_shape(first)) {
        return make(file_label, some(marker_line(0)), ErrorCode::preamble_missing);
    }

    // (4) duplicate / (5) misplaced — a Preamble-shape document at a non-first
    // position. The reference reports duplicate when a later document is also a
    // Preamble shape (a map with no spec key). Misplaced is reserved for a later
    // document that is explicitly a Preamble but only one exists; in practice a
    // non-first map-without-spec is a duplicate. We attribute to that document's
    // marker line, preferring duplicate per the reference.
    for (std::size_t i = 1; i < docs.size(); ++i) {
        if (is_preamble_shape(docs[i])) {
            return make(file_label, some(marker_line(i)), ErrorCode::preamble_duplicate);
        }
    }

    // The first document is the Preamble; validate its content.
    std::optional<yaml::Node> desc, version, related;
    int preamble_map_line = first.line().value_or(0);
    for (const yaml::Node& kv : first.children()) {
        if (kv.key() == "description") desc = kv;
        else if (kv.key() == "version") version = kv;
        else if (kv.key() == "related") related = kv;
    }

    // (6) missing_description.
    if (!desc.has_value()) {
        return make(file_label, some(preamble_map_line),
                    ErrorCode::preamble_missing_description);
    }
    // (7) missing_version.
    if (!version.has_value()) {
        return make(file_label, some(preamble_map_line),
                    ErrorCode::preamble_missing_version);
    }
    // (8) unknown_version — must be the exact string `v1`.
    {
        bool is_v1 = version->is_scalar() && version->scalar() == "v1";
        if (!is_v1) {
            std::string val = version->is_scalar() ? std::string(version->scalar())
                                                   : std::string();
            return make(file_label, some(version->line().value_or(0)),
                        ErrorCode::preamble_unknown_version, val);
        }
    }
    // (9) bad_related — present and not a sequence of strings.
    if (related.has_value()) {
        const yaml::Node& r = *related;
        if (!r.is_seq()) {
            return make(file_label, some(r.line().value_or(0)),
                        ErrorCode::preamble_bad_related);
        }
        for (const yaml::Node& el : r.children()) {
            // Every element must resolve to a YAML string. A quoted scalar is a
            // string; a plain scalar is a string iff it is not a core-schema
            // int/float/bool/null; a mapping/sequence/null element is not.
            bool is_string = el.is_scalar() &&
                             (el.is_value_quoted() ||
                              model::plain_scalar_is_string(el.scalar()));
            if (!is_string) {
                return make(file_label, some(el.line().value_or(0)),
                            ErrorCode::preamble_bad_related);
            }
        }
    }

    return std::nullopt;
}

// --------------------------------------------------------------------------
// CheckSpec
// --------------------------------------------------------------------------
std::vector<Diagnostic> check_spec(std::string_view file_label, const model::Spec& spec) {
    std::vector<Diagnostic> out;

    // spec.no_name — a non-first document with no top-level `spec` key.
    if (!spec.has_spec_key) {
        out.push_back(make(file_label, some(spec.doc_line), ErrorCode::spec_no_name));
        return out;  // nothing else meaningful to validate without a name.
    }

    // Spec-name validation, in order: not_string -> empty -> bad_chars ->
    // bad_form -> reserved. All attributed to the name node's line.
    int nline = spec.name_line;
    if (!spec.name_is_string) {
        out.push_back(make(file_label, some(nline), ErrorCode::spec_name_not_string));
    } else if (spec.name.empty()) {
        out.push_back(make(file_label, some(nline), ErrorCode::spec_name_empty));
    } else if (!only_allowed_chars(spec.name)) {
        out.push_back(make(file_label, some(nline), ErrorCode::spec_name_bad_chars, spec.name));
    } else if (!matches_composition(spec.name)) {
        out.push_back(make(file_label, some(nline), ErrorCode::spec_name_bad_form, spec.name));
    } else if (model::is_reserved_name(spec.name)) {
        out.push_back(make(file_label, some(nline), ErrorCode::spec_name_reserved, spec.name));
    }

    // unknown_key — top-level keys other than `spec` that are not Slot keys.
    for (const auto& uk : spec.unknown_keys) {
        out.push_back(make(file_label, some(uk.line), ErrorCode::spec_unknown_key, uk.key));
    }

    // Per-slot, per-obligation checks.
    for (const model::SlotGroup& grp : spec.slots) {
        if (!grp.value_is_list) {
            out.push_back(make(file_label, some(grp.line), ErrorCode::slot_value_not_list,
                               grp.key));
            continue;  // no obligations to walk when the value is not a list.
        }
        for (const model::Obligation& ob : grp.obligations) {
            if (ob.element_not_map) {
                // The obligation element is a scalar/seq/null, not a mapping.
                out.push_back(make(file_label, some(ob.line),
                                   ErrorCode::obligation_bad_value_shape));
                continue;
            }
            // Per-key issues, in obligation-key source order. The reference
            // splits these around the obligation-level errors: NORMATIVITY-keyword
            // issues (bad_value_shape on a recognized key, normativity.unknown) are
            // emitted BEFORE the obligation-level errors, while RELATION-key issues
            // (reference.unknown_relation) are emitted AFTER them. The file is then
            // stable-sorted by (line, column) by the validate orchestrator, which
            // preserves this insertion order for co-located errors.
            //
            // First pass: the non-relation key issues (kept before obligation-level
            // errors, as the reference emits them while scanning keys).
            for (const auto& ki : ob.key_issues) {
                if (ki.kind == model::Obligation::KeyIssue::BadValueShape) {
                    out.push_back(make(file_label, some(ki.line),
                                       ErrorCode::obligation_bad_value_shape));
                } else if (ki.kind == model::Obligation::KeyIssue::UnknownKey) {
                    out.push_back(make(file_label, some(ki.line),
                                       ErrorCode::normativity_unknown, ki.key));
                }
                // UnknownRelation is deferred to after the obligation-level errors.
            }
            // Obligation-level errors, attributed to the obligation node's line
            // (= its first key's line/column). Order: duplicate_normativity,
            // guard_without_normativity, missing_normativity_or_ref (matches the
            // reference's intra-obligation order). RELATION-key issues
            // (reference.unknown_relation, duplicate_reference) follow LAST.
            //
            // duplicate_normativity: the reference emits ONE error per Normativity
            // keyword AFTER the first, attributed to each duplicate keyword's own
            // line (so N keywords => N-1 errors). M (the error count) and the line
            // numbers both depend on this.
            for (std::size_t i = 1; i < ob.normativity_lines.size(); ++i) {
                out.push_back(make(file_label, some(ob.normativity_lines[i]),
                                   ErrorCode::obligation_duplicate_normativity));
            }
            if (ob.has_guard && ob.normativity_count == 0) {
                out.push_back(make(file_label, some(ob.line),
                                   ErrorCode::obligation_guard_without_normativity));
            }
            if (ob.normativity_count == 0 && ob.refs.empty()) {
                out.push_back(make(file_label, some(ob.line),
                                   ErrorCode::obligation_missing_normativity_or_ref));
            }
            // Second pass: RELATION-key issues — a key resembling a Reference
            // relation but outside the recognized set
            // (yass.reference.unknown_relation). The reference emits these AFTER
            // the obligation-level errors, so for co-located (same line/column)
            // errors they sort after missing_normativity_or_ref.
            for (const auto& ki : ob.key_issues) {
                if (ki.kind == model::Obligation::KeyIssue::UnknownRelation) {
                    out.push_back(make(file_label, some(ki.line),
                                       ErrorCode::reference_unknown_relation, ki.key));
                }
            }
            // duplicate_reference — same relation key more than once (normally
            // shadowed by CheckYAML duplicate_key in the full pipeline). A
            // relation-key error, so it follows the obligation-level errors too.
            for (const auto& rel : ob.duplicate_relation_keys) {
                out.push_back(make(file_label, some(ob.line),
                                   ErrorCode::obligation_duplicate_reference, rel));
            }
        }
    }
    return out;
}

std::vector<Diagnostic> check_specs(std::string_view file_label, const model::Model& model) {
    std::vector<Diagnostic> out;
    for (const model::Spec& spec : model.specs) {
        auto diags = check_spec(file_label, spec);
        out.insert(out.end(), diags.begin(), diags.end());
    }
    return out;
}

// --------------------------------------------------------------------------
// CheckUniqueness
// --------------------------------------------------------------------------
std::vector<Diagnostic> check_uniqueness(std::string_view file_label,
                                         const model::Model& model) {
    std::vector<Diagnostic> out;
    std::set<std::string> seen;
    for (const model::Spec& spec : model.specs) {
        if (!spec.has_spec_key || !spec.name_is_string || spec.name.empty()) continue;
        if (seen.count(spec.name)) {
            out.push_back(make(file_label, some(spec.name_line),
                               ErrorCode::spec_duplicate_name, spec.name));
        } else {
            seen.insert(spec.name);
        }
    }
    return out;
}

// --------------------------------------------------------------------------
// CheckRefs
// --------------------------------------------------------------------------
namespace {

// The set of spec names + (for slot checks) declared slots in one file. Built
// for the referencing file (from its model) or by opening a referenced file.
struct FileSpecs {
    bool parseable = true;  // false => file could not be parsed as YAML
    // spec name -> set of declared slot keys (as written, e.g. SIDE-EFFECT).
    std::unordered_map<std::string, std::set<std::string>> specs;
};

FileSpecs file_specs_from_model(const model::Model& model) {
    FileSpecs fs_specs;
    for (const model::Spec& spec : model.specs) {
        if (!spec.has_spec_key || !spec.name_is_string || spec.name.empty()) continue;
        std::set<std::string> slots;
        for (const model::SlotGroup& grp : spec.slots) {
            slots.insert(grp.key);
        }
        fs_specs.specs.emplace(spec.name, std::move(slots));
    }
    return fs_specs;
}

// Open + parse a referenced file for existence and target-spec lookup ONLY. No
// other checks are run on it. Returns parseable=false when the file cannot be
// parsed as well-formed YAML.
FileSpecs open_referenced_file(const std::string& path) {
    FileSpecs fs_specs;
    textio::ReadResult rr = textio::read_file_bytes(path);
    if (!rr.ok()) {
        fs_specs.parseable = false;  // caller distinguishes not-found beforehand
        return fs_specs;
    }
    // Parse with the same well-formedness gate validate uses for a file.
    yaml::CheckYamlResult r = yaml::check_yaml(path, rr.bytes);
    if (!r.ok || !r.stream.has_value()) {
        fs_specs.parseable = false;
        return fs_specs;
    }
    model::Model m = model::extract(*r.stream, rr.bytes);
    for (const model::Spec& spec : m.specs) {
        if (!spec.has_spec_key || !spec.name_is_string || spec.name.empty()) continue;
        std::set<std::string> slots;
        for (const model::SlotGroup& grp : spec.slots) {
            slots.insert(grp.key);
        }
        fs_specs.specs.emplace(spec.name, std::move(slots));
    }
    return fs_specs;
}

}  // namespace

std::vector<Diagnostic> check_refs(std::string_view file_label,
                                   const model::Model& model,
                                   std::string_view base_path,
                                   std::string_view project_root) {
    std::vector<Diagnostic> out;

    // Self-file specs (for same-file resolution + same-file slot checks).
    FileSpecs self = file_specs_from_model(model);

    // Cache of opened referenced files by resolved absolute path. Also records
    // which (referenced-file) paths have already emitted a file_not_found /
    // file_not_parseable so we emit at most one per (referencing,referenced) pair.
    std::unordered_map<std::string, FileSpecs> cache;
    std::set<std::string> existence_error_emitted;

    for (const model::Spec& spec : model.specs) {
        for (const model::SlotGroup& grp : spec.slots) {
            for (const model::Obligation& ob : grp.obligations) {
                for (const model::Reference& ref : ob.refs) {
                    int line = ref.line;

                    // 1) Grammar.
                    auto parsed = model::parse_ref_target(ref.target);
                    if (!parsed.has_value()) {
                        out.push_back(make(file_label, some(line), ErrorCode::ref_malformed,
                                           ref.target));
                        continue;
                    }
                    // 2) Slot suffix must be a recognized slot keyword.
                    if (parsed->has_slot && !model::is_slot_keyword(parsed->slot)) {
                        out.push_back(make(file_label, some(line), ErrorCode::ref_unknown_slot,
                                           parsed->slot));
                        continue;
                    }
                    // 3) Resolution.
                    model::ResolvedRef resolved =
                        model::resolve_ref(*parsed, base_path, project_root);

                    if (resolved.same_file) {
                        auto it = self.specs.find(resolved.spec_name);
                        if (it == self.specs.end()) {
                            out.push_back(make(file_label, some(line),
                                               ErrorCode::ref_spec_not_found_same_file,
                                               ref.target));
                            continue;
                        }
                        if (parsed->has_slot && it->second.count(parsed->slot) == 0) {
                            out.push_back(make(file_label, some(line),
                                               ErrorCode::ref_slot_not_declared, ref.target));
                        }
                        continue;
                    }

                    // Cross-file. Existence first (at most one per pair).
                    const std::string& fp = resolved.file_path;
                    bool exists = false;
                    {
                        std::error_code ec;
                        exists = fs::is_regular_file(fp, ec);
                    }
                    if (!exists) {
                        if (existence_error_emitted.insert(fp).second) {
                            out.push_back(make(file_label, some(line),
                                               ErrorCode::ref_file_not_found, ref.target));
                        }
                        continue;
                    }
                    // Parse (cached).
                    auto cit = cache.find(fp);
                    if (cit == cache.end()) {
                        cit = cache.emplace(fp, open_referenced_file(fp)).first;
                    }
                    const FileSpecs& other = cit->second;
                    if (!other.parseable) {
                        if (existence_error_emitted.insert(fp).second) {
                            out.push_back(make(file_label, some(line),
                                               ErrorCode::ref_file_not_parseable, ref.target));
                        }
                        continue;
                    }
                    auto sit = other.specs.find(resolved.spec_name);
                    if (sit == other.specs.end()) {
                        out.push_back(make(file_label, some(line),
                                           ErrorCode::ref_spec_not_found_other_file, ref.target));
                        continue;
                    }
                    if (parsed->has_slot && sit->second.count(parsed->slot) == 0) {
                        out.push_back(make(file_label, some(line),
                                           ErrorCode::ref_slot_not_declared, ref.target));
                    }
                }
            }
        }
    }
    return out;
}

}  // namespace yass::check
