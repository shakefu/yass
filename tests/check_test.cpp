// M5 — model + structural checks + ref resolution tests.
//
// Each test cites the spec/slot/obligation it exercises. Spec bases:
//   - spec/cli.validate.yass.yaml :: cli.validate.CheckPreamble / CheckSpec /
//       CheckUniqueness / CheckRefs (ERROR obligations -> one test per code).
//   - spec/yass.yass.yaml :: Preamble / Spec / Slot / Obligation / Normativity /
//       Guard / Reference / RefTarget (the constructs the model mirrors).
//   - spec/cli.errors.yass.yaml :: cli.errors RETURN (codes + spec message prose).
//
// Conformance policy: stderr follows the SPEC (code + line via cli.ErrorLine,
// message via diag::canonical_message); the reference yass is the oracle for
// exit + stdout and for tie-breaking interpretation of tricky codes. The oracle
// cross-checks at the bottom run the reference binary on crafted fixtures and
// assert the emitted [code] matches our interpretation.

#include "doctest.h"

#include <algorithm>
#include <cstdlib>
#include <filesystem>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "check.hpp"
#include "diag.hpp"
#include "model.hpp"
#include "textio.hpp"
#include "yaml.hpp"

#include <unistd.h>

#include "support/diff.hpp"
#include "support/proc.hpp"
#include "support/tmptree.hpp"

using namespace yass;
using yass::diag::Diagnostic;
using yass::diag::ErrorCode;

namespace {

// Parse `src` and return the ParsedStream (must be well-formed).
yaml::ParsedStream parse_ok(std::string_view label, std::string_view src) {
    yaml::CheckYamlResult r = yaml::check_yaml(label, src);
    REQUIRE(r.ok);
    REQUIRE(r.stream.has_value());
    return std::move(*r.stream);
}

// Run all structural checks on `src` (as a file labelled `label`, with ref
// resolution rooted at `root` and the referencing path `base`), collect the
// diagnostics, and stable-sort by line so the order mirrors what the validate
// orchestrator emits (file, line, column ascending; equal-line ties preserved).
std::vector<Diagnostic> run_checks(std::string_view label, std::string_view src,
                                   std::string_view base = "f.yass.yaml",
                                   std::string_view root = ".") {
    yaml::ParsedStream ps = parse_ok(label, src);
    model::Model m = model::extract(ps, src);
    std::vector<Diagnostic> all;
    if (auto pre = check::check_preamble(label, ps, src)) all.push_back(*pre);
    for (auto& d : check::check_specs(label, m)) all.push_back(d);
    for (auto& d : check::check_uniqueness(label, m)) all.push_back(d);
    for (auto& d : check::check_refs(label, m, base, root)) all.push_back(d);
    std::stable_sort(all.begin(), all.end(), [](const Diagnostic& a, const Diagnostic& b) {
        return a.line.value_or(0) < b.line.value_or(0);
    });
    return all;
}

// CheckPreamble convenience.
std::optional<Diagnostic> preamble_of(std::string_view src,
                                      std::string_view label = "f.yass.yaml") {
    yaml::ParsedStream ps = parse_ok(label, src);
    return check::check_preamble(label, ps, src);
}

// CheckSpec on the first spec document (document index 1) of `src`.
std::vector<Diagnostic> spec_of(std::string_view src,
                                std::string_view label = "f.yass.yaml") {
    yaml::ParsedStream ps = parse_ok(label, src);
    model::Model m = model::extract(ps, src);
    return check::check_specs(label, m);
}

// True iff `diags` contains a diagnostic with this code (and optionally line).
bool has_code(const std::vector<Diagnostic>& diags, ErrorCode code) {
    return std::any_of(diags.begin(), diags.end(),
                       [&](const Diagnostic& d) { return d.code == code; });
}
const Diagnostic* find_code(const std::vector<Diagnostic>& diags, ErrorCode code) {
    for (const Diagnostic& d : diags) {
        if (d.code == code) return &d;
    }
    return nullptr;
}
// Number of diagnostics carrying `code`.
int count_code(const std::vector<Diagnostic>& diags, ErrorCode code) {
    return static_cast<int>(std::count_if(
        diags.begin(), diags.end(), [&](const Diagnostic& d) { return d.code == code; }));
}
// Lines (sorted ascending) of every diagnostic carrying `code`.
std::vector<int> lines_of_code(const std::vector<Diagnostic>& diags, ErrorCode code) {
    std::vector<int> ls;
    for (const Diagnostic& d : diags) {
        if (d.code == code && d.line.has_value()) ls.push_back(*d.line);
    }
    std::sort(ls.begin(), ls.end());
    return ls;
}

// A well-formed Preamble prefix for spec-document fixtures.
constexpr std::string_view kPre = "---\ndescription: x\nversion: v1\n---\n";

}  // namespace

// ===========================================================================
// RefTarget grammar + resolution (spec/yass.yass.yaml :: RefTarget).
// ===========================================================================

TEST_CASE("RefTarget grammar: well-formed bare / path / slot targets parse") {
    using model::parse_ref_target;
    CHECK(parse_ref_target("Foo").has_value());
    CHECK(parse_ref_target("pkg.Symbol").has_value());
    CHECK(parse_ref_target("Foo::INPUT").has_value());
    CHECK(parse_ref_target("cli.shared@cli.ExpandGlob").has_value());
    CHECK(parse_ref_target("./cli.shared@Foo::RETURN").has_value());
    CHECK(parse_ref_target("../a/b@Foo").has_value());
    // Slot token need not be a valid slot keyword to pass the GRAMMAR.
    CHECK(parse_ref_target("Foo::BOGUS").has_value());

    auto p = parse_ref_target("./cli.shared@cli.ExpandGlob::RETURN");
    REQUIRE(p.has_value());
    CHECK(p->has_path);
    CHECK(p->path == "./cli.shared");
    CHECK(p->spec_name == "cli.ExpandGlob");
    CHECK(p->has_slot);
    CHECK(p->slot == "RETURN");
}

TEST_CASE("RefTarget grammar: malformed targets are rejected (yass.ref.malformed)") {
    using model::parse_ref_target;
    CHECK_FALSE(parse_ref_target("bad target!").has_value());  // space + '!'
    CHECK_FALSE(parse_ref_target("Foo::input").has_value());   // lowercase slot
    CHECK_FALSE(parse_ref_target("Foo::").has_value());        // empty slot
    CHECK_FALSE(parse_ref_target("@Foo").has_value());         // empty path
    CHECK_FALSE(parse_ref_target("path@").has_value());        // empty spec name
    CHECK_FALSE(parse_ref_target("").has_value());             // empty
    CHECK_FALSE(parse_ref_target("a\\b@Foo").has_value());     // backslash in path
}

TEST_CASE("RefTarget resolve_ref: bare name resolves to the referencing file") {
    using model::resolve_ref;
    auto p = model::parse_ref_target("Foo");
    REQUIRE(p.has_value());
    auto r = resolve_ref(*p, "/proj/sub/a.yass.yaml", "/proj");
    CHECK(r.same_file);
    CHECK(r.spec_name == "Foo");
    CHECK_FALSE(r.has_slot);
    CHECK(r.file_path == std::filesystem::path("/proj/sub/a.yass.yaml").lexically_normal().string());
}

TEST_CASE("RefTarget resolve_ref: ./ and ../ resolve relative to the referencing dir") {
    using model::resolve_ref;
    // ./cli.shared@Foo -> <dir-of-referencing-file>/cli.shared.yass.yaml
    auto p = model::parse_ref_target("./cli.shared@Foo");
    REQUIRE(p.has_value());
    auto r = resolve_ref(*p, "/proj/sub/a.yass.yaml", "/proj");
    CHECK_FALSE(r.same_file);
    CHECK(r.file_path == "/proj/sub/cli.shared.yass.yaml");
    CHECK(r.spec_name == "Foo");

    auto p2 = model::parse_ref_target("../x/b@Bar");
    REQUIRE(p2.has_value());
    auto r2 = resolve_ref(*p2, "/proj/sub/a.yass.yaml", "/proj");
    CHECK(r2.file_path == "/proj/x/b.yass.yaml");
}

TEST_CASE("RefTarget resolve_ref: dotless path resolves from the project root") {
    using model::resolve_ref;
    // cli@Foo -> <project-root>/cli.yass.yaml
    auto p = model::parse_ref_target("cli@Foo");
    REQUIRE(p.has_value());
    auto r = resolve_ref(*p, "/proj/sub/a.yass.yaml", "/proj");
    CHECK(r.file_path == "/proj/cli.yass.yaml");
    CHECK(r.spec_name == "Foo");

    // The literal suffix is appended to the whole path token (incl. dots).
    auto p2 = model::parse_ref_target("cli.shared@cli.ExpandGlob");
    REQUIRE(p2.has_value());
    auto r2 = resolve_ref(*p2, "/proj/a.yass.yaml", "/proj");
    CHECK(r2.file_path == "/proj/cli.shared.yass.yaml");
}

TEST_CASE("model document_marker_lines: doc-start markers located by source scan") {
    // Leading --- then a second doc separator.
    auto ml = model::document_marker_lines("---\ndescription: x\n---\nspec: foo\n");
    REQUIRE(ml.size() == 2);
    CHECK(ml[0] == 1);
    CHECK(ml[1] == 3);
    // No leading marker: only the inter-doc separator is a marker.
    auto ml2 = model::document_marker_lines("description: x\n---\nspec: foo\n");
    REQUIRE(ml2.size() == 1);
    CHECK(ml2[0] == 2);
    // `--- inline content` counts as a marker; `---foo` does not.
    auto ml3 = model::document_marker_lines("--- a\n---foo\n---\n");
    REQUIRE(ml3.size() == 2);
    CHECK(ml3[0] == 1);
    CHECK(ml3[1] == 3);
}

// ===========================================================================
// CheckPreamble — one test per ERROR obligation, in the spec's order.
// ===========================================================================

// yass.preamble.has_spec_key: the first document carries a `spec` key.
TEST_CASE("CheckPreamble ERROR yass.preamble.has_spec_key") {
    auto d = preamble_of("---\nspec: foo\nINPUT:\n- MUST: a\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_has_spec_key);
    CHECK(d->line == 1);  // attributed to the doc-start marker line
    CHECK(d->message == "first document must be a Preamble, not a Spec");
}

// yass.yaml.empty_stream: the parsed stream has zero documents.
TEST_CASE("CheckPreamble ERROR yass.yaml.empty_stream") {
    auto d = preamble_of("# only a comment\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::yaml_empty_stream);
    CHECK_FALSE(d->line.has_value());
    CHECK(d->message == "YAML stream contains no documents");
}

// yass.preamble.missing: the first document is not a Preamble shape (a scalar
// or a sequence rather than a mapping without a spec key).
TEST_CASE("CheckPreamble ERROR yass.preamble.missing (first doc is a scalar)") {
    auto d = preamble_of("---\njust a scalar\n---\nspec: foo\nINPUT:\n- MUST: a\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_missing);
    CHECK(d->line == 1);
    CHECK(d->message == "missing Preamble");
}

TEST_CASE("CheckPreamble ERROR yass.preamble.missing (first doc is a sequence)") {
    auto d = preamble_of("---\n- a\n- b\n---\nspec: foo\nINPUT:\n- MUST: a\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_missing);
}

// yass.preamble.duplicate: a later document is also a Preamble shape (a mapping
// without a spec key), attributed to its doc-start marker line.
TEST_CASE("CheckPreamble ERROR yass.preamble.duplicate") {
    auto d = preamble_of("---\ndescription: x\nversion: v1\n---\ndescription: y\nversion: v1\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_duplicate);
    CHECK(d->line == 4);  // the second preamble's doc-start marker
    CHECK(d->message == "more than one Preamble in file");
}

// yass.preamble.missing_description: the Preamble omits description; attributed
// to the preamble mapping node's line.
TEST_CASE("CheckPreamble ERROR yass.preamble.missing_description") {
    auto d = preamble_of("---\nversion: v1\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_missing_description);
    CHECK(d->line == 2);
    CHECK(d->message == "Preamble missing description");
}

// yass.preamble.missing_version.
TEST_CASE("CheckPreamble ERROR yass.preamble.missing_version") {
    auto d = preamble_of("---\ndescription: x\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_missing_version);
    CHECK(d->line == 2);
    CHECK(d->message == "Preamble missing version");
}

// yass.preamble.unknown_version: version is not the exact string v1.
TEST_CASE("CheckPreamble ERROR yass.preamble.unknown_version (v2)") {
    auto d = preamble_of("---\ndescription: x\nversion: v2\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_unknown_version);
    CHECK(d->line == 3);
    CHECK(d->message == "unsupported Preamble version: v2");
}

TEST_CASE("CheckPreamble ERROR yass.preamble.unknown_version (V1 is case-sensitive)") {
    auto d = preamble_of("---\ndescription: x\nversion: V1\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_unknown_version);
    CHECK(d->message == "unsupported Preamble version: V1");
}

// yass.preamble.bad_related: related present and not a sequence of strings.
TEST_CASE("CheckPreamble ERROR yass.preamble.bad_related (scalar)") {
    auto d = preamble_of("---\ndescription: x\nversion: v1\nrelated: notseq\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_bad_related);
    CHECK(d->line == 4);
    CHECK(d->message == "Preamble related must be a sequence of strings");
}

TEST_CASE("CheckPreamble ERROR yass.preamble.bad_related (sequence with a number)") {
    auto d = preamble_of("---\ndescription: x\nversion: v1\nrelated:\n- 123\n- ok\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_bad_related);
    CHECK(d->line == 5);  // the offending element
}

// RETURN: a well-formed Preamble (incl. empty related list) returns success.
TEST_CASE("CheckPreamble RETURN: a valid Preamble returns success") {
    CHECK_FALSE(preamble_of("---\ndescription: x\nversion: v1\n").has_value());
    CHECK_FALSE(preamble_of("---\ndescription: x\nversion: v1\nrelated: []\n").has_value());
    CHECK_FALSE(preamble_of("---\ndescription: x\nversion: v1\nrelated:\n- https://a\n").has_value());
}

// INVARIANT: at most one error, stopping at the first match in spec order. A
// Preamble that both omits description AND has an unknown version reports only
// the earlier (description) error.
TEST_CASE("CheckPreamble INVARIANT: at most one error in spec order") {
    auto d = preamble_of("---\nversion: v2\n");
    REQUIRE(d.has_value());
    CHECK(d->code == ErrorCode::preamble_missing_description);  // not unknown_version
}

// ===========================================================================
// CheckSpec — one test per ERROR obligation.
// ===========================================================================

// yass.spec.no_name: a non-first document with no top-level `spec` key,
// attributed to the document-start line.
TEST_CASE("CheckSpec ERROR yass.spec.no_name") {
    auto d = spec_of("---\ndescription: x\nversion: v1\n---\nINPUT:\n- MUST: a\n");
    REQUIRE(has_code(d, ErrorCode::spec_no_name));
    const Diagnostic* nn = find_code(d, ErrorCode::spec_no_name);
    CHECK(nn->line == 4);  // the doc-start marker of the second document
    CHECK(nn->message == "spec document missing spec key");
}

// yass.spec.name_not_string: the name value is not a string (number/bool/null/
// seq/map under the YAML 1.2 core schema).
TEST_CASE("CheckSpec ERROR yass.spec.name_not_string (number)") {
    auto d = spec_of(std::string(kPre) + "spec: 123\n");
    REQUIRE(has_code(d, ErrorCode::spec_name_not_string));
    CHECK(find_code(d, ErrorCode::spec_name_not_string)->line == 5);
    CHECK(find_code(d, ErrorCode::spec_name_not_string)->message == "spec name must be a string");
}

TEST_CASE("CheckSpec ERROR yass.spec.name_not_string (bool / null / seq / map)") {
    CHECK(has_code(spec_of(std::string(kPre) + "spec: true\n"), ErrorCode::spec_name_not_string));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: null\n"), ErrorCode::spec_name_not_string));
    CHECK(has_code(spec_of(std::string(kPre) + "spec:\n"), ErrorCode::spec_name_not_string));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: []\n"), ErrorCode::spec_name_not_string));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: {}\n"), ErrorCode::spec_name_not_string));
    // yass@Document keeps yes/no/on/off as plain strings: NOT name_not_string.
    CHECK_FALSE(has_code(spec_of(std::string(kPre) + "spec: yes\n"), ErrorCode::spec_name_not_string));
    CHECK_FALSE(has_code(spec_of(std::string(kPre) + "spec: on\n"), ErrorCode::spec_name_not_string));
}

// yass.spec.name_empty: a quoted empty string.
TEST_CASE("CheckSpec ERROR yass.spec.name_empty") {
    auto d = spec_of(std::string(kPre) + "spec: \"\"\n");
    REQUIRE(has_code(d, ErrorCode::spec_name_empty));
    CHECK(find_code(d, ErrorCode::spec_name_empty)->message == "spec name is empty");
}

// yass.spec.name_bad_chars: a character outside [A-Za-z0-9._-].
TEST_CASE("CheckSpec ERROR yass.spec.name_bad_chars (space / @)") {
    auto d = spec_of(std::string(kPre) + "spec: \"foo bar\"\n");
    REQUIRE(has_code(d, ErrorCode::spec_name_bad_chars));
    CHECK(find_code(d, ErrorCode::spec_name_bad_chars)->message ==
          "spec name contains disallowed characters: foo bar");
    CHECK(has_code(spec_of(std::string(kPre) + "spec: \"foo@bar\"\n"),
                   ErrorCode::spec_name_bad_chars));
}

// yass.spec.name_bad_form: chars are allowed but the composition regex fails
// (leading/trailing dot, consecutive dots).
TEST_CASE("CheckSpec ERROR yass.spec.name_bad_form (dots)") {
    CHECK(has_code(spec_of(std::string(kPre) + "spec: \".foo\"\n"), ErrorCode::spec_name_bad_form));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: \"foo.\"\n"), ErrorCode::spec_name_bad_form));
    auto d = spec_of(std::string(kPre) + "spec: \"foo..bar\"\n");
    REQUIRE(has_code(d, ErrorCode::spec_name_bad_form));
    CHECK(find_code(d, ErrorCode::spec_name_bad_form)->message == "spec name is malformed: foo..bar");
    // bad_chars takes precedence over bad_form: a space is reported as bad_chars.
    CHECK_FALSE(has_code(spec_of(std::string(kPre) + "spec: \"foo bar\"\n"),
                         ErrorCode::spec_name_bad_form));
}

// yass.spec.name_reserved: case-insensitive match of a Slot or Normativity kw.
TEST_CASE("CheckSpec ERROR yass.spec.name_reserved") {
    CHECK(has_code(spec_of(std::string(kPre) + "spec: input\n"), ErrorCode::spec_name_reserved));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: MUST\n"), ErrorCode::spec_name_reserved));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: \"side-effect\"\n"),
                   ErrorCode::spec_name_reserved));
    auto d = spec_of(std::string(kPre) + "spec: \"must-not\"\n");
    REQUIRE(has_code(d, ErrorCode::spec_name_reserved));
    CHECK(find_code(d, ErrorCode::spec_name_reserved)->message ==
          "spec name collides with a reserved keyword: must-not");
}

// yass.spec.unknown_key: a top-level key other than `spec` that is not a Slot.
TEST_CASE("CheckSpec ERROR yass.spec.unknown_key") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nFOObar:\n- MUST: a\n");
    REQUIRE(has_code(d, ErrorCode::spec_unknown_key));
    const Diagnostic* uk = find_code(d, ErrorCode::spec_unknown_key);
    CHECK(uk->line == 6);
    CHECK(uk->message == "unknown spec key: FOObar");
}

// yass.slot.value_not_list: a valid Slot key whose value is not a list.
TEST_CASE("CheckSpec ERROR yass.slot.value_not_list (scalar / map / null)") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT: notalist\n");
    REQUIRE(has_code(d, ErrorCode::slot_value_not_list));
    const Diagnostic* sv = find_code(d, ErrorCode::slot_value_not_list);
    CHECK(sv->line == 6);
    CHECK(sv->message == "slot value must be a list: INPUT");
    CHECK(has_code(spec_of(std::string(kPre) + "spec: foo\nINPUT:\n  k: v\n"),
                   ErrorCode::slot_value_not_list));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: foo\nINPUT:\n"),
                   ErrorCode::slot_value_not_list));
}

// yass.obligation.bad_value_shape: a recognized key's value is map/seq/null.
// The error attributes to the VALUE's content line (the reference yass uses the
// value node, not the key): for `- MUST:` (line 7) / `    nested: y` (line 8),
// the value's first content line is 8.
TEST_CASE("CheckSpec ERROR yass.obligation.bad_value_shape (MUST value is a map)") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n    nested: y\n");
    REQUIRE(has_code(d, ErrorCode::obligation_bad_value_shape));
    CHECK(find_code(d, ErrorCode::obligation_bad_value_shape)->line == 8);
    CHECK(find_code(d, ErrorCode::obligation_bad_value_shape)->message ==
          "obligation value must be a quoted scalar");
}

// A null value attributes to the key's own line (no value content line); a
// sequence value attributes to its first element's line.
TEST_CASE("CheckSpec ERROR yass.obligation.bad_value_shape (null at key line, seq at element line)") {
    auto dnull = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n");
    REQUIRE(has_code(dnull, ErrorCode::obligation_bad_value_shape));
    CHECK(find_code(dnull, ErrorCode::obligation_bad_value_shape)->line == 7);
    auto dseq = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n  - a\n  - b\n");
    REQUIRE(has_code(dseq, ErrorCode::obligation_bad_value_shape));
    CHECK(find_code(dseq, ErrorCode::obligation_bad_value_shape)->line == 8);
}

TEST_CASE("CheckSpec ERROR yass.obligation.bad_value_shape (seq / null value)") {
    CHECK(has_code(spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n  - a\n  - b\n"),
                   ErrorCode::obligation_bad_value_shape));
    CHECK(has_code(spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n"),
                   ErrorCode::obligation_bad_value_shape));
}

TEST_CASE("CheckSpec ERROR yass.obligation.bad_value_shape (obligation element is a scalar)") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- just a string\n");
    REQUIRE(has_code(d, ErrorCode::obligation_bad_value_shape));
    CHECK(find_code(d, ErrorCode::obligation_bad_value_shape)->line == 7);
}

// yass.obligation.missing_normativity_or_ref.
TEST_CASE("CheckSpec ERROR yass.obligation.missing_normativity_or_ref") {
    // An obligation whose only key is WHEN carries neither Normativity nor ref.
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- WHEN: cond\n");
    REQUIRE(has_code(d, ErrorCode::obligation_missing_normativity_or_ref));
    CHECK(find_code(d, ErrorCode::obligation_missing_normativity_or_ref)->line == 7);
    CHECK(find_code(d, ErrorCode::obligation_missing_normativity_or_ref)->message ==
          "obligation must carry a Normativity keyword or a Reference");
}

// yass.obligation.guard_without_normativity.
TEST_CASE("CheckSpec ERROR yass.obligation.guard_without_normativity") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- WHEN: cond\n");
    REQUIRE(has_code(d, ErrorCode::obligation_guard_without_normativity));
    CHECK(find_code(d, ErrorCode::obligation_guard_without_normativity)->message ==
          "WHEN guard requires a Normativity keyword");
}

// A reference-only obligation (>=1 ref, no Normativity, no WHEN) is well-formed:
// neither missing_normativity_or_ref nor guard_without_normativity fires.
TEST_CASE("CheckSpec RETURN: a reference-only obligation is valid") {
    auto d = spec_of(std::string(kPre) +
                     "spec: foo\nINPUT:\n- USES: bar\n---\nspec: bar\nINPUT:\n- MUST: a\n");
    CHECK_FALSE(has_code(d, ErrorCode::obligation_missing_normativity_or_ref));
    CHECK_FALSE(has_code(d, ErrorCode::obligation_guard_without_normativity));
}

// yass.obligation.duplicate_normativity (cli.validate CheckSpec MUST). The
// reference emits ONE error per Normativity keyword AFTER the first, attributed
// to each duplicate keyword's OWN line (NOT the obligation node's line). Both
// the error COUNT (which feeds the M stdout summary) and the LINE numbers depend
// on this per-keyword emission.
TEST_CASE("CheckSpec ERROR yass.obligation.duplicate_normativity") {
    // Two keywords (MUST@7, SHOULD@8): exactly ONE dup error, at the SECOND
    // keyword's line (8), not the obligation line (7).
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  SHOULD: b\n");
    REQUIRE(has_code(d, ErrorCode::obligation_duplicate_normativity));
    CHECK(count_code(d, ErrorCode::obligation_duplicate_normativity) == 1);
    CHECK(find_code(d, ErrorCode::obligation_duplicate_normativity)->line == 8);
    CHECK(find_code(d, ErrorCode::obligation_duplicate_normativity)->message ==
          "duplicate Normativity keyword in obligation");
}

// Four Normativity-shaped keywords (MUST@7, SHOULD@8, MAY@9, plus a non-keyword
// WEEK@10): the three recognized keywords yield N-1 = 2 dup errors at lines 8
// and 9 (the second and third keywords); WEEK is unrecognized. Confirmed
// byte-for-byte against the reference (M and line numbers both match).
TEST_CASE("CheckSpec ERROR yass.obligation.duplicate_normativity: one per keyword after the first") {
    auto d = spec_of(std::string(kPre) +
                     "spec: foo\nINPUT:\n- MUST: a\n  SHOULD: b\n  MAY: c\n  WEEK: d\n");
    // Exactly two dup errors, at the 2nd (line 8) and 3rd (line 9) keyword lines.
    CHECK(count_code(d, ErrorCode::obligation_duplicate_normativity) == 2);
    CHECK(lines_of_code(d, ErrorCode::obligation_duplicate_normativity) ==
          std::vector<int>({8, 9}));
    // WEEK is not a recognized Normativity keyword and does not resemble a
    // Reference relation -> normativity.unknown at its line (10).
    REQUIRE(has_code(d, ErrorCode::normativity_unknown));
    CHECK(find_code(d, ErrorCode::normativity_unknown)->line == 10);
}

// yass.normativity.unknown: a key that is neither Normativity, WHEN, nor a
// Reference relation (the reference treats it as a candidate Normativity kw).
TEST_CASE("CheckSpec ERROR yass.normativity.unknown") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUSTNT: a\n");
    REQUIRE(has_code(d, ErrorCode::normativity_unknown));
    const Diagnostic* nu = find_code(d, ErrorCode::normativity_unknown);
    CHECK(nu->line == 7);
    CHECK(nu->message == "unknown Normativity keyword: MUSTNT");
}

TEST_CASE("CheckSpec ERROR yass.normativity.unknown (extra key alongside MUST)") {
    // MUST + an unrecognized FOO key: FOO -> normativity.unknown at FOO's line.
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  FOO: b\n");
    REQUIRE(has_code(d, ErrorCode::normativity_unknown));
    CHECK(find_code(d, ErrorCode::normativity_unknown)->line == 8);
    CHECK(find_code(d, ErrorCode::normativity_unknown)->message ==
          "unknown Normativity keyword: FOO");
}

// yass.obligation.duplicate_reference: the same relation key more than once.
// In the full pipeline CheckYAML's duplicate_key gate shadows this (a file with
// duplicate mapping keys is rejected before CheckSpec runs), so we exercise
// CheckSpec in ISOLATION by parsing directly (bypassing the duplicate-key gate)
// and building the model from the resulting stream, which retains both keys.
TEST_CASE("CheckSpec ERROR yass.obligation.duplicate_reference (isolated)") {
    std::string src = std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: x\n  USES: y\n";
    yaml::ParsedStream ps = yaml::ParsedStream::parse("f.yass.yaml", src);
    model::Model m = model::extract(ps, src);
    auto d = check::check_specs("f.yass.yaml", m);
    REQUIRE(has_code(d, ErrorCode::obligation_duplicate_reference));
    CHECK(find_code(d, ErrorCode::obligation_duplicate_reference)->message ==
          "duplicate Reference relation in obligation: USES");
}

// yass.normativity.unknown vs yass.reference.unknown_relation (cli.validate
// CheckSpec). An unrecognized key that does NOT resemble a Reference relation
// (REQUIRES shares no prefix relationship with CONFORMS/USES/SEE) reports
// normativity.unknown, NOT reference.unknown_relation. Confirmed against the
// reference (REQUIRES -> yass.normativity.unknown).
TEST_CASE("CheckSpec: a non-relation-looking unknown key reports normativity.unknown") {
    auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  REQUIRES: x\n");
    CHECK(has_code(d, ErrorCode::normativity_unknown));
    CHECK_FALSE(has_code(d, ErrorCode::reference_unknown_relation));
}

// yass.reference.unknown_relation (cli.validate CheckSpec MUST: "error with code
// yass.reference.unknown_relation when a Reference relation key is outside the
// recognized set"). An unrecognized key that RESEMBLES a Reference relation
// (case-insensitive prefix relationship with CONFORMS/USES/SEE, length >= 2)
// reports reference.unknown_relation at the key's line, NOT normativity.unknown.
// Confirmed against the reference (e.g. CONFORM, USE, SEEN, conforms all yield
// yass.reference.unknown_relation 'unknown Reference relation: <key>').
TEST_CASE("CheckSpec ERROR yass.reference.unknown_relation (relation-resembling key)") {
    SUBCASE("CONFORM (prefix of CONFORMS)") {
        auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  CONFORM: x\n");
        REQUIRE(has_code(d, ErrorCode::reference_unknown_relation));
        const Diagnostic* r = find_code(d, ErrorCode::reference_unknown_relation);
        CHECK(r->line == 8);
        CHECK(r->message == "unknown Reference relation: CONFORM");
        CHECK_FALSE(has_code(d, ErrorCode::normativity_unknown));
    }
    SUBCASE("USE (prefix of USES)") {
        auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USE: x\n");
        REQUIRE(has_code(d, ErrorCode::reference_unknown_relation));
        CHECK(find_code(d, ErrorCode::reference_unknown_relation)->message ==
              "unknown Reference relation: USE");
    }
    SUBCASE("SEEN (SEE is a prefix of SEEN)") {
        auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  SEEN: x\n");
        REQUIRE(has_code(d, ErrorCode::reference_unknown_relation));
        CHECK(find_code(d, ErrorCode::reference_unknown_relation)->message ==
              "unknown Reference relation: SEEN");
    }
    SUBCASE("conforms (case-insensitive match) resembles a relation") {
        auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  conforms: x\n");
        CHECK(has_code(d, ErrorCode::reference_unknown_relation));
    }
    SUBCASE("single-char C does NOT resemble (length < 2)") {
        auto d = spec_of(std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  C: x\n");
        CHECK(has_code(d, ErrorCode::normativity_unknown));
        CHECK_FALSE(has_code(d, ErrorCode::reference_unknown_relation));
    }
}

// CheckSpec ordering (after stable sort by line): a single obligation that is
// ONLY a relation-resembling key (`- CONFORM: x`) emits BOTH the obligation-level
// missing_normativity_or_ref AND the relation-key reference.unknown_relation on
// the same line/column. Per cli.validate ("emit error lines in (file, line,
// column) ascending order") the within-file stable sort preserves insertion
// order for co-located errors, so the emitted order must place RELATION-key
// errors AFTER the obligation-level error. Confirmed against the reference
// (/tmp/yass): [missing_normativity_or_ref] THEN [reference.unknown_relation].
TEST_CASE("CheckSpec ordering: missing_normativity_or_ref before reference.unknown_relation") {
    auto diags = run_checks("f.yass.yaml", std::string(kPre) +
                            "spec: foo\nINPUT:\n- CONFORM: x\n");
    // The FULL ordered list of emitted codes for the `- CONFORM: x` obligation.
    REQUIRE(diags.size() == 2);
    CHECK(diags[0].code == ErrorCode::obligation_missing_normativity_or_ref);
    CHECK(diags[0].line == 7);
    CHECK(diags[1].code == ErrorCode::reference_unknown_relation);
    CHECK(diags[1].line == 7);
    CHECK(diags[1].message == "unknown Reference relation: CONFORM");
}

// CheckSpec ordering (after stable sort by line): an obligation with an unknown
// first key emits normativity.unknown(first) then missing_normativity_or_ref on
// the same line, then a later unknown key. Mirrors the reference.
TEST_CASE("CheckSpec ordering: normativity.unknown before obligation-level on same line") {
    auto diags = run_checks("f.yass.yaml", std::string(kPre) +
                            "spec: foo\nINPUT:\n- FOO: x\n  BAR: y\n");
    // Expect, in order: normativity.unknown(FOO,7), missing_normativity_or_ref(7),
    // normativity.unknown(BAR,8).
    REQUIRE(diags.size() == 3);
    CHECK(diags[0].code == ErrorCode::normativity_unknown);
    CHECK(diags[0].line == 7);
    CHECK(diags[1].code == ErrorCode::obligation_missing_normativity_or_ref);
    CHECK(diags[1].line == 7);
    CHECK(diags[2].code == ErrorCode::normativity_unknown);
    CHECK(diags[2].line == 8);
}

// RETURN: a fully valid spec document produces no diagnostics.
TEST_CASE("CheckSpec RETURN: a valid spec returns success") {
    auto d = spec_of(std::string(kPre) +
                     "spec: pkg.Symbol\nINPUT:\n- MUST: accept a thing\nRETURN:\n- MAY: do a thing\n");
    CHECK(d.empty());
}

// ===========================================================================
// CheckUniqueness (spec/cli.validate.yass.yaml :: cli.validate.CheckUniqueness).
// ===========================================================================

// yass.spec.duplicate_name: once per duplicate-after-the-first, at the
// subsequent occurrence's name line.
TEST_CASE("CheckUniqueness ERROR yass.spec.duplicate_name") {
    std::string src = std::string(kPre) +
        "spec: foo\nINPUT:\n- MUST: a\n---\nspec: foo\nINPUT:\n- MUST: b\n";
    yaml::ParsedStream ps = parse_ok("f.yass.yaml", src);
    model::Model m = model::extract(ps, src);
    auto d = check::check_uniqueness("f.yass.yaml", m);
    REQUIRE(d.size() == 1);
    CHECK(d[0].code == ErrorCode::spec_duplicate_name);
    CHECK(d[0].line == 9);  // the second `spec: foo`
    CHECK(d[0].message == "duplicate spec name in file: foo");
}

TEST_CASE("CheckUniqueness ERROR: each duplicate-after-first reported once") {
    std::string src = std::string(kPre) +
        "spec: foo\nINPUT:\n- MUST: a\n---\nspec: foo\nINPUT:\n- MUST: b\n"
        "---\nspec: foo\nINPUT:\n- MUST: c\n";
    yaml::ParsedStream ps = parse_ok("f.yass.yaml", src);
    model::Model m = model::extract(ps, src);
    auto d = check::check_uniqueness("f.yass.yaml", m);
    CHECK(d.size() == 2);  // two duplicates after the first
}

TEST_CASE("CheckUniqueness RETURN: unique names return success") {
    std::string src = std::string(kPre) +
        "spec: foo\nINPUT:\n- MUST: a\n---\nspec: bar\nINPUT:\n- MUST: b\n";
    yaml::ParsedStream ps = parse_ok("f.yass.yaml", src);
    model::Model m = model::extract(ps, src);
    CHECK(check::check_uniqueness("f.yass.yaml", m).empty());
}

// ===========================================================================
// CheckRefs (spec/cli.validate.yass.yaml :: cli.validate.CheckRefs).
// ===========================================================================

// yass.ref.malformed: grammar failure, at the ref line, with the raw target.
TEST_CASE("CheckRefs ERROR yass.ref.malformed") {
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"bad target!\"\n");
    REQUIRE(has_code(d, ErrorCode::ref_malformed));
    const Diagnostic* m = find_code(d, ErrorCode::ref_malformed);
    CHECK(m->line == 8);
    CHECK(m->message == "malformed ref target: bad target!");
}

// yass.ref.unknown_slot: a slot suffix outside the recognized set; reported
// before resolution.
TEST_CASE("CheckRefs ERROR yass.ref.unknown_slot") {
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) +
                        "spec: foo\nINPUT:\n- MUST: a\n  USES: \"bar::BOGUS\"\n"
                        "---\nspec: bar\nINPUT:\n- MUST: c\n");
    REQUIRE(has_code(d, ErrorCode::ref_unknown_slot));
    CHECK(find_code(d, ErrorCode::ref_unknown_slot)->message == "unknown slot in ref target: BOGUS");
}

TEST_CASE("CheckRefs ERROR yass.ref.unknown_slot takes precedence over resolution") {
    // A missing spec with an invalid slot suffix reports unknown_slot, not
    // spec_not_found.
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"nope::BOGUS\"\n");
    CHECK(has_code(d, ErrorCode::ref_unknown_slot));
    CHECK_FALSE(has_code(d, ErrorCode::ref_spec_not_found_same_file));
}

// yass.ref.spec_not_found_same_file.
TEST_CASE("CheckRefs ERROR yass.ref.spec_not_found_same_file") {
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: nonexistent\n");
    REQUIRE(has_code(d, ErrorCode::ref_spec_not_found_same_file));
    CHECK(find_code(d, ErrorCode::ref_spec_not_found_same_file)->message ==
          "spec not found in file: nonexistent");
}

// yass.ref.slot_not_declared (same file): the spec exists but lacks the slot.
TEST_CASE("CheckRefs ERROR yass.ref.slot_not_declared (same file)") {
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) +
                        "spec: foo\nINPUT:\n- MUST: a\n  USES: \"bar::RETURN\"\n"
                        "---\nspec: bar\nINPUT:\n- MUST: c\n");
    REQUIRE(has_code(d, ErrorCode::ref_slot_not_declared));
    CHECK(find_code(d, ErrorCode::ref_slot_not_declared)->message ==
          "referenced spec does not declare slot: bar::RETURN");
}

// A same-file ref that resolves (and a self CONFORMS) is allowed.
TEST_CASE("CheckRefs RETURN: a resolving same-file ref / self CONFORMS is allowed") {
    auto d = run_checks("f.yass.yaml",
                        std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"foo::INPUT\"\n");
    CHECK(d.empty());
    // Self CONFORMS is permitted (no cycle detection in v1).
    auto d2 = run_checks("f.yass.yaml",
                         std::string(kPre) + "spec: foo\nINPUT:\n- CONFORMS: \"foo::INPUT\"\n");
    CHECK(d2.empty());
}

// Cross-file ref cases use a temp tree with a .git marker so the project root is
// well defined and the referenced files live on disk.
TEST_CASE("CheckRefs ERROR yass.ref.file_not_found (cross-file)") {
    test::TmpTree tree;
    tree.mkdir(".git");
    std::string base = (tree.root() / "a.yass.yaml").string();
    tree.write("a.yass.yaml",
               std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"nope@Bar\"\n");
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    REQUIRE(has_code(d, ErrorCode::ref_file_not_found));
    CHECK(find_code(d, ErrorCode::ref_file_not_found)->message ==
          "referenced file not found: nope@Bar");
}

TEST_CASE("CheckRefs ERROR yass.ref.file_not_parseable (cross-file)") {
    test::TmpTree tree;
    tree.mkdir(".git");
    std::string base = (tree.root() / "a.yass.yaml").string();
    tree.write("a.yass.yaml",
               std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"./broken@Whatever\"\n");
    tree.write("broken.yass.yaml", "not: [valid\n");
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    REQUIRE(has_code(d, ErrorCode::ref_file_not_parseable));
    CHECK(find_code(d, ErrorCode::ref_file_not_parseable)->message ==
          "referenced file not parseable: ./broken@Whatever");
}

TEST_CASE("CheckRefs ERROR yass.ref.spec_not_found_other_file (cross-file)") {
    test::TmpTree tree;
    tree.mkdir(".git");
    std::string base = (tree.root() / "a.yass.yaml").string();
    tree.write("a.yass.yaml",
               std::string(kPre) + "spec: foo\nINPUT:\n- MUST: a\n  USES: \"./other@NoSuchSpec\"\n");
    tree.write("other.yass.yaml", std::string(kPre) + "spec: other\nINPUT:\n- MUST: a\n");
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    REQUIRE(has_code(d, ErrorCode::ref_spec_not_found_other_file));
    CHECK(find_code(d, ErrorCode::ref_spec_not_found_other_file)->message ==
          "spec not found in referenced file: ./other@NoSuchSpec");
}

// Relative (./) targets resolve to the referencing file's directory; root
// targets resolve from the project root. Both resolve successfully here.
TEST_CASE("CheckRefs RETURN: relative and root-relative targets resolve") {
    test::TmpTree tree;
    tree.mkdir(".git");
    tree.mkdir("sub");
    std::string base = (tree.root() / "sub" / "ref.yass.yaml").string();
    tree.write("sub/other.yass.yaml", std::string(kPre) + "spec: other\nINPUT:\n- MUST: a\n");
    tree.write("toplevel.yass.yaml", std::string(kPre) + "spec: rootspec\nINPUT:\n- MUST: a\n");
    tree.write("sub/ref.yass.yaml",
               std::string(kPre) +
               "spec: foo\nINPUT:\n- MUST: a\n  USES: \"./other@other\"\n"
               "- MUST: b\n  USES: \"toplevel@rootspec\"\n");
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    CHECK(d.empty());
}

// At most one file_not_found per (referencing-file, referenced-file) pair: two
// refs to the same missing file produce a single error.
TEST_CASE("CheckRefs INVARIANT: one file_not_found per (referencing, referenced) pair") {
    test::TmpTree tree;
    tree.mkdir(".git");
    std::string base = (tree.root() / "a.yass.yaml").string();
    tree.write("a.yass.yaml",
               std::string(kPre) +
               "spec: foo\nINPUT:\n- MUST: a\n  USES: \"missingfile@A\"\n"
               "- MUST: b\n  USES: \"missingfile@B\"\n");
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    int count = 0;
    for (const auto& diag : d) {
        if (diag.code == ErrorCode::ref_file_not_found) ++count;
    }
    CHECK(count == 1);
}

// ===========================================================================
// Reference-oracle cross-checks. These run the real `yass` binary on crafted
// fixtures inside a temp tree (with a .git marker) and assert the emitted [code]
// matches our interpretation, pinning the tricky classifications. When the
// reference binary cannot be located the checks are skipped (the unit tests
// above already pin behavior; the oracle is a confirmation, not a dependency).
// ===========================================================================

namespace {

// The reference binary is resolved at CMake configure time and validated by the
// shared yass::test::find_ref_bin() (support/diff.hpp). Alias it here so the
// oracle helpers below keep their short call sites.
using yass::test::find_ref_bin;

// Extract the first [code] token from the reference's stderr, or empty.
std::string first_code_token(const std::string& stderr_text) {
    std::size_t lb = stderr_text.find('[');
    if (lb == std::string::npos) return {};
    std::size_t rb = stderr_text.find(']', lb);
    if (rb == std::string::npos) return {};
    return stderr_text.substr(lb + 1, rb - lb - 1);
}

// Write `content` into a temp tree (with a .git marker), run the reference
// `yass validate <file>`, and return the first emitted [code] token. Returns
// nullopt when the reference binary is unavailable.
std::optional<std::string> oracle_first_code(const std::string& filename,
                                             const std::string& content) {
    std::string ref = find_ref_bin();
    if (ref.empty()) return std::nullopt;
    test::TmpTree tree;
    tree.mkdir(".git");
    tree.write(filename, content);
    std::string file = (tree.root() / filename).string();
    test::ProcResult r = test::run({ref, "validate", file}, tree.root().string());
    return first_code_token(r.err);
}

}  // namespace

TEST_CASE("oracle: unknown_key vs normativity.unknown vs bad_value_shape") {
    // A top-level non-Slot key -> spec.unknown_key.
    if (auto c = oracle_first_code("u.yass.yaml",
                                   std::string(kPre) + "spec: foo\nBADSLOT:\n- MUST: a\n")) {
        CHECK(*c == "yass.spec.unknown_key");
    }
    // An unrecognized obligation key -> normativity.unknown.
    if (auto c = oracle_first_code("n.yass.yaml",
                                   std::string(kPre) + "spec: foo\nINPUT:\n- MUSTNT: a\n")) {
        // The first emitted code by the reference for this case is
        // normativity.unknown (line 7), ahead of missing_normativity_or_ref.
        CHECK((*c == "yass.normativity.unknown" ||
               *c == "yass.obligation.missing_normativity_or_ref"));
    }
    // An obligation value that is a mapping -> obligation.bad_value_shape.
    if (auto c = oracle_first_code("b.yass.yaml",
                                   std::string(kPre) + "spec: foo\nINPUT:\n- MUST:\n    k: v\n")) {
        CHECK(*c == "yass.obligation.bad_value_shape");
    }
}

TEST_CASE("oracle: name_bad_chars vs name_bad_form vs name_reserved") {
    if (auto c = oracle_first_code("c.yass.yaml", std::string(kPre) + "spec: \"foo bar\"\n")) {
        CHECK(*c == "yass.spec.name_bad_chars");
    }
    if (auto c = oracle_first_code("f.yass.yaml", std::string(kPre) + "spec: \"foo..bar\"\n")) {
        CHECK(*c == "yass.spec.name_bad_form");
    }
    if (auto c = oracle_first_code("r.yass.yaml", std::string(kPre) + "spec: \"side-effect\"\n")) {
        CHECK(*c == "yass.spec.name_reserved");
    }
    if (auto c = oracle_first_code("nn.yass.yaml", std::string(kPre) + "spec: 123\n")) {
        CHECK(*c == "yass.spec.name_not_string");
    }
}

TEST_CASE("oracle: ref resolution codes") {
    if (auto c = oracle_first_code("m.yass.yaml",
                                   std::string(kPre) +
                                   "spec: foo\nINPUT:\n- MUST: a\n  USES: \"bad target!\"\n")) {
        CHECK(*c == "yass.ref.malformed");
    }
    if (auto c = oracle_first_code("s.yass.yaml",
                                   std::string(kPre) +
                                   "spec: foo\nINPUT:\n- MUST: a\n  USES: nonexistent\n")) {
        CHECK(*c == "yass.ref.spec_not_found_same_file");
    }
    if (auto c = oracle_first_code("fnf.yass.yaml",
                                   std::string(kPre) +
                                   "spec: foo\nINPUT:\n- MUST: a\n  USES: \"nope@Bar\"\n")) {
        CHECK(*c == "yass.ref.file_not_found");
    }
}

// Positive oracle: the reference reports 0 errors on a clean cross-file fixture,
// and so do our checks. (Pins that our resolution agrees with the reference for
// a passing case, not only failing ones.)
TEST_CASE("oracle: a clean cross-file fixture validates with zero errors") {
    std::string ref = find_ref_bin();
    test::TmpTree tree;
    tree.mkdir(".git");
    tree.write("other.yass.yaml", std::string(kPre) + "spec: other\nINPUT:\n- MUST: a\n");
    std::string a_content = std::string(kPre) +
        "spec: foo\nINPUT:\n- MUST: a\n  USES: \"./other@other::INPUT\"\n";
    tree.write("a.yass.yaml", a_content);
    std::string base = (tree.root() / "a.yass.yaml").string();

    // Our checks: zero diagnostics.
    auto rr = textio::read_file_bytes(base);
    REQUIRE(rr.ok());
    auto ps = parse_ok(base, rr.bytes);
    auto m = model::extract(ps, rr.bytes);
    auto d = check::check_refs(base, m, base, tree.root().string());
    CHECK(d.empty());

    // The reference, when available, agrees (0 errors on stdout summary).
    if (!ref.empty()) {
        test::ProcResult r = test::run({ref, "validate", base}, tree.root().string());
        CHECK(r.exit_code == 0);
        CHECK(r.out.find("found 0 errors") != std::string::npos);
    }
}
