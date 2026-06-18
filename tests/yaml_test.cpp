// M3 — YAML well-formedness + parsed-stream access tests.
//
// Each test name cites the spec/slot/obligation it exercises so a failure
// points back to the normative source. Spec bases:
//   - spec/cli.validate.yass.yaml :: cli.validate.CheckYAML
//       ERROR: yass.yaml.not_utf8 / has_bom / empty_file / malformed /
//              duplicate_key / anchor_or_alias
//       RETURN: WHEN well-formed -> return success
//       INVARIANT: at most one error per file, in the stated preference order
//   - context/yass-reference.md :: yass@Document (UTF-8 YAML 1.2 multi-doc
//       stream; anchors/aliases/tags/duplicate keys prohibited)
//   - spec/cli.errors.yass.yaml :: cli.errors RETURN (yass.yaml.* codes/messages)

#include "doctest.h"

#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"
#include "yaml.hpp"

using namespace yass::yaml;
using yass::diag::ErrorCode;

namespace {

// Build a std::string from raw byte values so embedded NULs / high bytes are
// preserved exactly (mirrors the helper in textio_test.cpp).
std::string bytes(std::initializer_list<int> vals) {
    std::string s;
    s.reserve(vals.size());
    for (int v : vals) s.push_back(static_cast<char>(static_cast<unsigned char>(v)));
    return s;
}

// Run check_yaml and assert it produced exactly the given error code (and
// optionally a line). Returns the diagnostic for further inspection.
yass::diag::Diagnostic expect_error(std::string_view label, std::string_view src,
                                    ErrorCode code) {
    CheckYamlResult r = check_yaml(label, src);
    REQUIRE_FALSE(r.ok);
    REQUIRE(r.error.has_value());
    REQUIRE_FALSE(r.stream.has_value());
    CHECK(r.error->code == code);
    CHECK(r.error->file == std::string(label));
    return *r.error;
}

}  // namespace

// ===========================================================================
// ERROR branch: one crafted input per error obligation.
// ===========================================================================

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.not_utf8 when not UTF-8.
TEST_CASE("CheckYAML ERROR yass.yaml.not_utf8: invalid UTF-8 bytes") {
    // 0xFF is never a valid UTF-8 lead byte.
    std::string src = bytes({'a', ':', ' ', 0xFF, '\n'});
    auto d = expect_error("bad.yass.yaml", src, ErrorCode::yaml_not_utf8);
    // Byte-level error: no line attribution.
    CHECK_FALSE(d.line.has_value());
    CHECK(d.message == "file is not valid UTF-8");
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.has_bom on a leading BOM.
TEST_CASE("CheckYAML ERROR yass.yaml.has_bom: file begins with UTF-8 BOM") {
    std::string src = bytes({0xEF, 0xBB, 0xBF}) + "a: 1\n";
    auto d = expect_error("bom.yass.yaml", src, ErrorCode::yaml_has_bom);
    CHECK_FALSE(d.line.has_value());
    CHECK(d.message == "file begins with a UTF-8 BOM");
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.empty_file on zero bytes.
TEST_CASE("CheckYAML ERROR yass.yaml.empty_file: zero-byte file") {
    auto d = expect_error("empty.yass.yaml", "", ErrorCode::yaml_empty_file);
    CHECK_FALSE(d.line.has_value());
    CHECK(d.message == "empty file");
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.malformed on ill-formed
// YAML 1.2 (ryml parse throws). The offending line is now captured from ryml's
// parse-error Location and clamped to the file's line count (cli.ErrorLine line
// attribution); for this single-line input the offending line is 1, matching the
// reference yass which also attributes `a: [unclosed` to line 1.
TEST_CASE("CheckYAML ERROR yass.yaml.malformed: not well-formed YAML 1.2") {
    auto d = expect_error("malformed.yass.yaml", "a: [unclosed", ErrorCode::yaml_malformed);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 1);
    CHECK(d.message == "YAML well-formedness error");
}

// cli.validate.CheckYAML ERROR: a multi-line malformed file attributes to the
// offending line, not EOF. `a: [` opens a flow sequence on line 1; the parser
// detects the violation at line 2 (`b: 1`). The reference yass agrees (line 2).
TEST_CASE("CheckYAML ERROR yass.yaml.malformed: line attribution mid-file") {
    auto d = expect_error("ml.yass.yaml", "a: [\nb: 1\nc: 2\nd: 3\n",
                          ErrorCode::yaml_malformed);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 2);
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.duplicate_key on a mapping
// with duplicate keys; message "duplicate mapping key: <key>"; at the offending
// (second) occurrence's line.
TEST_CASE("CheckYAML ERROR yass.yaml.duplicate_key: repeated mapping key") {
    auto d = expect_error("dup.yass.yaml", "a: 1\na: 2\n", ErrorCode::yaml_duplicate_key);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 2);  // the second `a:` is on line 2 (1-based)
    CHECK(d.message == "duplicate mapping key: a");
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.anchor_or_alias when an
// anchor + alias pair is present, at the first offending node's line.
TEST_CASE("CheckYAML ERROR yass.yaml.anchor_or_alias: anchor and alias") {
    auto d = expect_error("anchor.yass.yaml", "a: &x 1\nb: *x\n",
                          ErrorCode::yaml_anchor_or_alias);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 1);  // the anchor `&x` (first offender) is on line 1
    CHECK(d.message == "YAML anchors, aliases, and explicit tags are not allowed");
}

// cli.validate.CheckYAML ERROR: MUST error yass.yaml.anchor_or_alias when an
// explicit / non-default tag is present (tag arm of the same obligation).
TEST_CASE("CheckYAML ERROR yass.yaml.anchor_or_alias: explicit tag") {
    auto d = expect_error("tag.yass.yaml", "a: !!str 1\n",
                          ErrorCode::yaml_anchor_or_alias);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 1);
    CHECK(d.message == "YAML anchors, aliases, and explicit tags are not allowed");
}

// Tag arm again with a custom (application) tag, confirming "non-default tag"
// covers more than the !!core tags.
TEST_CASE("CheckYAML ERROR yass.yaml.anchor_or_alias: custom application tag") {
    expect_error("ctag.yass.yaml", "a: !mytag 1\n", ErrorCode::yaml_anchor_or_alias);
}

// ===========================================================================
// INVARIANT: at most one error per file, in the stated preference order.
// ===========================================================================

// not_utf8 outranks has_bom: a BOM made of valid bytes followed by an invalid
// byte still reports not_utf8 (UTF-8 validity is checked first).
TEST_CASE("CheckYAML INVARIANT preference: not_utf8 before has_bom") {
    std::string src = bytes({0xEF, 0xBB, 0xBF, 0xFF});  // BOM then invalid byte
    expect_error("p1.yass.yaml", src, ErrorCode::yaml_not_utf8);
}

// has_bom outranks empty_file is vacuous (BOM => non-empty); has_bom outranks
// malformed: a BOM-prefixed malformed file reports has_bom, not malformed.
TEST_CASE("CheckYAML INVARIANT preference: has_bom before malformed") {
    std::string src = bytes({0xEF, 0xBB, 0xBF}) + "a: [unclosed";
    expect_error("p2.yass.yaml", src, ErrorCode::yaml_has_bom);
}

// malformed outranks duplicate_key/anchor: an unparseable file never reaches
// the tree-walk checks.
TEST_CASE("CheckYAML INVARIANT preference: malformed before duplicate_key") {
    // Well-formed-looking dup keys but inside an unclosed flow map => malformed
    // wins because the parse throws before any tree walk.
    expect_error("p3.yass.yaml", "{a: 1, a: 2", ErrorCode::yaml_malformed);
}

// duplicate_key outranks anchor_or_alias: a file with BOTH a duplicate key and
// an anchor reports the duplicate key (higher in the preference order).
TEST_CASE("CheckYAML INVARIANT preference: duplicate_key before anchor_or_alias") {
    auto d = expect_error("p4.yass.yaml", "a: &x 1\na: *x\n",
                          ErrorCode::yaml_duplicate_key);
    REQUIRE(d.line.has_value());
    CHECK(*d.line == 2);
    CHECK(d.message == "duplicate mapping key: a");
}

// ===========================================================================
// RETURN: WHEN well-formed -> return success (positive paths).
// ===========================================================================

// A plain, single-document, clean file parses to success with one document.
TEST_CASE("CheckYAML RETURN success: clean single-document file") {
    CheckYamlResult r = check_yaml("ok.yass.yaml", "a: 1\nb: 2\n");
    REQUIRE(r.ok);
    REQUIRE(r.stream.has_value());
    CHECK_FALSE(r.error.has_value());
    CHECK(r.stream->document_count() == 1);
}

// A content-free file (only comments / blank lines) is well-formed YAML with
// ZERO documents: CheckYAML returns success (empty-stream is CheckPreamble's
// concern, not a CheckYAML well-formedness error).
TEST_CASE("CheckYAML RETURN success: comment-only file is well-formed, zero docs") {
    CheckYamlResult r = check_yaml("comments.yass.yaml", "# only a comment\n");
    REQUIRE(r.ok);
    REQUIRE(r.stream.has_value());
    CHECK(r.stream->document_count() == 0);
}

// ===========================================================================
// ParsedStream façade: multi-doc ordering, kinds, lines, quoted vs plain.
// (Document INPUT/RETURN: a yass file is an ordered multi-document stream;
//  downstream modules navigate it through the façade.)
// ===========================================================================

TEST_CASE("ParsedStream: valid multi-doc file exposes documents in order with "
          "correct kinds, lines, and quoted-vs-plain detection") {
    // Two documents. Doc 1 (a preamble-shaped map) starts after the first ---;
    // doc 2 (a spec-shaped map) after the second ---. Lines are 1-based.
    //   line 1: ---
    //   line 2: description: "quoted desc"
    //   line 3: version: v1
    //   line 4: ---
    //   line 5: spec: example
    //   line 6: kind: plain
    std::string src =
        "---\n"
        "description: \"quoted desc\"\n"
        "version: v1\n"
        "---\n"
        "spec: example\n"
        "kind: plain\n";
    CheckYamlResult r = check_yaml("multi.yass.yaml", src);
    REQUIRE(r.ok);
    REQUIRE(r.stream.has_value());

    auto docs = r.stream->documents();
    REQUIRE(docs.size() == 2);

    // Doc 1 is a mapping with two members, in order.
    CHECK(docs[0].kind() == NodeKind::Map);
    CHECK(docs[0].is_map());
    auto d0 = docs[0].children();
    REQUIRE(d0.size() == 2);
    CHECK(d0[0].key() == "description");
    CHECK(d0[1].key() == "version");

    // The description value is a quoted scalar; the version value is plain.
    CHECK(d0[0].kind() == NodeKind::Scalar);
    CHECK(d0[0].scalar() == "quoted desc");
    CHECK(d0[0].is_value_quoted());
    CHECK_FALSE(d0[0].is_value_plain());
    CHECK(d0[1].scalar() == "v1");
    CHECK(d0[1].is_value_plain());
    CHECK_FALSE(d0[1].is_value_quoted());

    // Line attribution (1-based) for the members of doc 1.
    REQUIRE(d0[0].line().has_value());
    CHECK(*d0[0].line() == 2);
    REQUIRE(d0[1].line().has_value());
    CHECK(*d0[1].line() == 3);

    // Doc 2 is a mapping; its members carry the right keys, values and lines.
    CHECK(docs[1].kind() == NodeKind::Map);
    auto d1 = docs[1].children();
    REQUIRE(d1.size() == 2);
    CHECK(d1[0].key() == "spec");
    CHECK(d1[0].scalar() == "example");
    CHECK(d1[0].is_value_plain());
    REQUIRE(d1[0].line().has_value());
    CHECK(*d1[0].line() == 5);
    CHECK(d1[1].key() == "kind");
    REQUIRE(d1[1].line().has_value());
    CHECK(*d1[1].line() == 6);
}

// Node kinds: a sequence value and a null value are reported distinctly from
// scalars and maps (Document RETURN: kind = map/seq/scalar/null).
TEST_CASE("ParsedStream: node kinds distinguish map, seq, scalar, and null") {
    std::string src =
        "amap:\n"
        "  k: v\n"
        "aseq:\n"
        "  - one\n"
        "  - two\n"
        "ascalar: text\n"
        "anull: ~\n"
        "implicit:\n";
    CheckYamlResult r = check_yaml("kinds.yass.yaml", src);
    REQUIRE(r.ok);
    REQUIRE(r.stream.has_value());
    auto docs = r.stream->documents();
    REQUIRE(docs.size() == 1);
    auto top = docs[0].children();
    REQUIRE(top.size() == 5);
    CHECK(top[0].key() == "amap");
    CHECK(top[0].kind() == NodeKind::Map);
    CHECK(top[1].key() == "aseq");
    CHECK(top[1].kind() == NodeKind::Seq);
    auto elems = top[1].children();
    REQUIRE(elems.size() == 2);
    CHECK(elems[0].kind() == NodeKind::Scalar);
    CHECK(elems[0].scalar() == "one");
    CHECK(top[2].key() == "ascalar");
    CHECK(top[2].kind() == NodeKind::Scalar);
    CHECK(top[3].key() == "anull");
    CHECK(top[3].kind() == NodeKind::Null);
    CHECK(top[4].key() == "implicit");
    CHECK(top[4].kind() == NodeKind::Null);  // empty value position is null
}

// ===========================================================================
// ParsedStream stream-wide probes (used by check_yaml; tested directly so a
// regression in the primitive is caught in isolation).
// ===========================================================================

TEST_CASE("ParsedStream: has_anchor_alias_or_tag reports absence on a clean file") {
    auto ps = ParsedStream::parse("clean.yass.yaml", "a: 1\nb: two\n");
    CHECK_FALSE(ps.has_anchor_alias_or_tag());
    CHECK_FALSE(ps.first_anchor_alias_or_tag_line().has_value());
    CHECK_FALSE(ps.first_duplicate_key().has_value());
}

TEST_CASE("ParsedStream: first_duplicate_key finds nested duplicate and its line") {
    // Duplicate is nested inside a mapping value; line is the second key's line.
    //   line 1: outer:
    //   line 2:   x: 1
    //   line 3:   x: 2
    std::string src = "outer:\n  x: 1\n  x: 2\n";
    auto ps = ParsedStream::parse("nested.yass.yaml", src);
    auto dup = ps.first_duplicate_key();
    REQUIRE(dup.has_value());
    CHECK(dup->key == "x");
    REQUIRE(dup->line.has_value());
    CHECK(*dup->line == 3);
}

TEST_CASE("ParsedStream: anchor on a later line reports that line, not line 1") {
    // anchor `&x` is on line 2; the alias on line 3 is also an offender but the
    // anchor is encountered first in document order.
    std::string src = "a: 1\nb: &x 2\nc: *x\n";
    auto ps = ParsedStream::parse("late.yass.yaml", src);
    auto line = ps.first_anchor_alias_or_tag_line();
    REQUIRE(line.has_value());
    CHECK(*line == 2);
}
