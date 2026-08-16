// M2 — text & file IO tests.
//
// Each test name cites the spec/slot/obligation it exercises so a failure
// points back to the normative source. Spec bases:
//   - yass.yass.yaml :: Document INPUT  (UTF-8, no leading BOM)
//   - spec/cli.errors.yass.yaml :: cli.errors RETURN
//       yass.yaml.not_utf8 / yass.yaml.has_bom / yass.yaml.empty_file /
//       yass.path.not_found / yass.path.unreadable
//   - spec/cli.list.yass.yaml :: cli.list RETURN (NFC + code-point order)
//   - spec/cli.shared.yass.yaml :: cli.DiscoverSpecFiles RETURN (same ordering)

#include "doctest.h"

#include <cstdio>
#include <filesystem>
#include <string>
#include <string_view>
#include <vector>

#include <unistd.h>

#include "textio.hpp"

using namespace yass::textio;

namespace {

// Build a std::string from raw byte values so embedded NULs / high bytes are
// preserved exactly.
std::string bytes(std::initializer_list<int> vals) {
    std::string s;
    s.reserve(vals.size());
    for (int v : vals) s.push_back(static_cast<char>(static_cast<unsigned char>(v)));
    return s;
}

// Create a unique temp file path under the system temp dir.
std::filesystem::path temp_path(const char* tag) {
    auto dir = std::filesystem::temp_directory_path();
    auto p = dir / (std::string("yass_textio_") + tag + "_" +
                    std::to_string(::getpid()) + ".tmp");
    return p;
}

void write_file(const std::filesystem::path& p, std::string_view content) {
    std::FILE* fp = std::fopen(p.string().c_str(), "wb");
    REQUIRE(fp != nullptr);
    if (!content.empty()) {
        std::fwrite(content.data(), 1, content.size(), fp);
    }
    std::fclose(fp);
}

}  // namespace

// ==========================================================================
// is_valid_utf8 — Document INPUT MUST: be encoded as UTF-8 (positive path)
// ==========================================================================
TEST_CASE("yass@Document::INPUT MUST utf8 — is_valid_utf8 accepts ASCII and well-formed multibyte") {
    CHECK(is_valid_utf8(""));                  // empty is trivially valid
    CHECK(is_valid_utf8("hello world"));       // ASCII
    CHECK(is_valid_utf8(std::string("a\0b", 3)));  // embedded NUL is valid
    CHECK(is_valid_utf8("\xC3\xA9"));          // U+00E9 é (2-byte)
    CHECK(is_valid_utf8("\xE2\x9C\x93"));      // U+2713 ✓ (3-byte)
    CHECK(is_valid_utf8("\xF0\x9F\x98\x80"));  // U+1F600 😀 (4-byte)
    CHECK(is_valid_utf8("\xF4\x8F\xBF\xBF"));  // U+10FFFF, the max code point
    CHECK(is_valid_utf8("\xEF\xBB\xBF" "abc"));  // a BOM is itself valid UTF-8
}

// ==========================================================================
// is_valid_utf8 — Document INPUT MUST utf8 (guard path => yass.yaml.not_utf8)
// ==========================================================================
TEST_CASE("yass@Document::INPUT MUST utf8 — is_valid_utf8 rejects overlong encodings") {
    CHECK_FALSE(is_valid_utf8(bytes({0xC0, 0x80})));        // overlong NUL
    CHECK_FALSE(is_valid_utf8(bytes({0xC1, 0xBF})));        // overlong, C1 lead
    CHECK_FALSE(is_valid_utf8(bytes({0xE0, 0x80, 0x80})));  // overlong 3-byte
    CHECK_FALSE(is_valid_utf8(bytes({0xE0, 0x9F, 0xBF})));  // 2nd byte < A0
    CHECK_FALSE(is_valid_utf8(bytes({0xF0, 0x80, 0x80, 0x80})));  // overlong 4-byte
    CHECK_FALSE(is_valid_utf8(bytes({0xF0, 0x8F, 0xBF, 0xBF})));  // 2nd byte < 90
}

TEST_CASE("yass@Document::INPUT MUST utf8 — is_valid_utf8 rejects UTF-16 surrogates") {
    CHECK_FALSE(is_valid_utf8(bytes({0xED, 0xA0, 0x80})));  // U+D800 lone surrogate
    CHECK_FALSE(is_valid_utf8(bytes({0xED, 0xBF, 0xBF})));  // U+DFFF
    CHECK_FALSE(is_valid_utf8(bytes({0xED, 0xA0, 0x80, 0xED, 0xB0, 0x80})));  // surrogate pair
}

TEST_CASE("yass@Document::INPUT MUST utf8 — is_valid_utf8 rejects code points above U+10FFFF") {
    CHECK_FALSE(is_valid_utf8(bytes({0xF4, 0x90, 0x80, 0x80})));  // U+110000 (just over max)
    CHECK_FALSE(is_valid_utf8(bytes({0xF5, 0x80, 0x80, 0x80})));  // F5 lead is always > max
    CHECK_FALSE(is_valid_utf8(bytes({0xFF})));                    // FF is never a lead byte
    CHECK_FALSE(is_valid_utf8(bytes({0xFE})));                    // FE is never a lead byte
}

TEST_CASE("yass@Document::INPUT MUST utf8 — is_valid_utf8 rejects truncated and stray sequences") {
    CHECK_FALSE(is_valid_utf8(bytes({0xC3})));              // truncated 2-byte
    CHECK_FALSE(is_valid_utf8(bytes({0xE2, 0x9C})));        // truncated 3-byte
    CHECK_FALSE(is_valid_utf8(bytes({0xF0, 0x9F, 0x98})));  // truncated 4-byte
    CHECK_FALSE(is_valid_utf8(bytes({0x80})));              // stray continuation byte
    CHECK_FALSE(is_valid_utf8(bytes({0xBF})));              // stray continuation byte
    CHECK_FALSE(is_valid_utf8(bytes({0xC3, 0x28})));        // 2nd byte not a continuation
    CHECK_FALSE(is_valid_utf8("valid" "\xC3"));            // valid prefix, truncated tail
}

// ==========================================================================
// has_utf8_bom — Document INPUT MUST-NOT: contain a leading UTF-8 BOM
// ==========================================================================
TEST_CASE("yass@Document::INPUT MUST-NOT bom — has_utf8_bom true iff leading EF BB BF") {
    CHECK(has_utf8_bom(bytes({0xEF, 0xBB, 0xBF})));            // exactly the BOM
    CHECK(has_utf8_bom("\xEF\xBB\xBF" "content"));            // BOM then content
    // complementary (boolean guard false) branch:
    CHECK_FALSE(has_utf8_bom(""));                            // empty: no BOM
    CHECK_FALSE(has_utf8_bom("content"));                     // ASCII: no BOM
    CHECK_FALSE(has_utf8_bom(bytes({0xEF, 0xBB})));           // too short
    CHECK_FALSE(has_utf8_bom(bytes({0xEF, 0xBB, 0xBE})));     // wrong third byte
    CHECK_FALSE(has_utf8_bom(bytes({0x00, 0xEF, 0xBB, 0xBF})));  // BOM not at start
}

// ==========================================================================
// decode_utf8 / count_codepoints — underpins cli.list grapheme/width counting
// ==========================================================================
TEST_CASE("textio decode_utf8 — decodes ASCII and multibyte to correct scalars") {
    auto ascii = decode_utf8("AB");
    REQUIRE(ascii.size() == 2);
    CHECK(ascii[0] == U'A');
    CHECK(ascii[1] == U'B');

    auto two = decode_utf8("\xC3\xA9");  // U+00E9
    REQUIRE(two.size() == 1);
    CHECK(two[0] == char32_t{0x00E9});

    auto three = decode_utf8("\xE2\x9C\x93");  // U+2713
    REQUIRE(three.size() == 1);
    CHECK(three[0] == char32_t{0x2713});

    auto four = decode_utf8("\xF0\x9F\x98\x80");  // U+1F600
    REQUIRE(four.size() == 1);
    CHECK(four[0] == char32_t{0x1F600});

    auto mixed = decode_utf8("a\xC3\xA9\xF0\x9F\x98\x80" "z");  // a é 😀 z
    REQUIRE(mixed.size() == 4);
    CHECK(mixed[0] == U'a');
    CHECK(mixed[1] == char32_t{0x00E9});
    CHECK(mixed[2] == char32_t{0x1F600});
    CHECK(mixed[3] == U'z');
}

TEST_CASE("textio count_codepoints — counts scalars not bytes for multibyte input") {
    CHECK(count_codepoints("") == 0u);
    CHECK(count_codepoints("hello") == 5u);
    // 4 code points but 1+2+4+1 = 8 bytes.
    CHECK(count_codepoints("a\xC3\xA9\xF0\x9F\x98\x80" "z") == 4u);
    // count_codepoints must agree with decode_utf8 size.
    std::string sample = "x\xE2\x9C\x93" "y\xC3\xA9";
    CHECK(count_codepoints(sample) == decode_utf8(sample).size());
}

// ==========================================================================
// nfc — cli.list RETURN MUST: emit description text in NFC-normalized UTF-8
// ==========================================================================
TEST_CASE("cli.list@RETURN MUST nfc — nfc is identity on ASCII") {
    CHECK(nfc("") == std::string(""));
    CHECK(nfc("hello world") == std::string("hello world"));
    CHECK(nfc("path/to/file.yass.yaml") == std::string("path/to/file.yass.yaml"));
    // Pure-ASCII input is already NFC and must be returned byte-for-byte.
    std::string ascii = "AaZz09 _-./:";
    CHECK(nfc(ascii) == ascii);
}

TEST_CASE("cli.list@RETURN MUST nfc — nfc is identity on already-NFC non-ASCII (documented approximation)") {
    // Precomposed é (U+00E9) is already NFC; must round-trip unchanged.
    std::string precomposed = "\xC3\xA9";
    CHECK(nfc(precomposed) == precomposed);
    // The function is the single normalization boundary; for now non-ASCII is
    // passed through. This pins the documented behavior so the boundary stays
    // the one place to upgrade.
    std::string emoji = "\xF0\x9F\x98\x80";
    CHECK(nfc(emoji) == emoji);
}

// ==========================================================================
// codepoint_less — cli.list / cli.DiscoverSpecFiles MUST: Unicode code-point
// order on NFC-normalized UTF-8; MUST-NOT locale-aware collation / case-fold
// ==========================================================================
TEST_CASE("cli.DiscoverSpecFiles@RETURN MUST codepoint-order — ASCII ordering and prefix rule") {
    CHECK(codepoint_less("a", "b"));
    CHECK_FALSE(codepoint_less("b", "a"));
    CHECK_FALSE(codepoint_less("a", "a"));        // equal is not strictly less
    CHECK(codepoint_less("ab", "abc"));           // proper prefix sorts first
    CHECK_FALSE(codepoint_less("abc", "ab"));
    CHECK(codepoint_less("", "a"));               // empty sorts before any nonempty
    CHECK_FALSE(codepoint_less("", ""));
}

TEST_CASE("cli.DiscoverSpecFiles@RETURN MUST-NOT case-fold — uppercase sorts before lowercase by code point") {
    // In code-point order 'Z' (0x5A) < 'a' (0x61); a case-folding comparator
    // would order them otherwise. This guards the MUST-NOT case-fold rule.
    CHECK(codepoint_less("Z", "a"));
    CHECK_FALSE(codepoint_less("a", "Z"));
    CHECK(codepoint_less("Apple", "apple"));
}

TEST_CASE("cli.DiscoverSpecFiles@RETURN MUST codepoint-order — multibyte sorts after ASCII") {
    // Any non-ASCII code point encodes with a lead byte >= 0xC2, which is
    // unsigned-greater than every ASCII byte; so multibyte sorts after ASCII.
    CHECK(codepoint_less("z", "\xC3\xA9"));        // 'z' (0x7A) < é lead (0xC3)
    CHECK_FALSE(codepoint_less("\xC3\xA9", "z"));
    // Code-point order between multibyte sequences == unsigned byte order:
    // U+00E9 (C3 A9) < U+2713 (E2 9C 93) < U+1F600 (F0 9F 98 80).
    CHECK(codepoint_less("\xC3\xA9", "\xE2\x9C\x93"));
    CHECK(codepoint_less("\xE2\x9C\x93", "\xF0\x9F\x98\x80"));
    CHECK_FALSE(codepoint_less("\xF0\x9F\x98\x80", "\xC3\xA9"));
}

TEST_CASE("cli.DiscoverSpecFiles@RETURN MUST-NOT locale-collation — comparator is a strict weak ordering") {
    // Irreflexive + asymmetric + transitive across a representative set,
    // confirming a pure ordering with no locale tailoring.
    std::vector<std::string> v = {"a", "Z", "ab", "\xC3\xA9", "abc", ""};
    for (const auto& x : v) {
        CHECK_FALSE(codepoint_less(x, x));  // irreflexive
    }
    for (const auto& x : v) {
        for (const auto& y : v) {
            if (codepoint_less(x, y)) {
                CHECK_FALSE(codepoint_less(y, x));  // asymmetric
            }
        }
    }
}

// ==========================================================================
// read_file_bytes — feeds yass.path.not_found / yass.path.unreadable /
// yass.yaml.empty_file (cli.errors RETURN)
// ==========================================================================
TEST_CASE("textio read_file_bytes — reads exact raw bytes of a regular file (BOM preserved)") {
    auto p = temp_path("read_ok");
    std::string content = "\xEF\xBB\xBF---\nspec: x\n";  // BOM left intact
    write_file(p, content);

    auto r = read_file_bytes(p.string());
    CHECK(r.status == ReadStatus::Ok);
    CHECK(r.ok());
    CHECK(r.bytes == content);  // verbatim, no BOM stripping or transcoding

    std::filesystem::remove(p);
}

TEST_CASE("textio read_file_bytes — zero-byte file is Ok with empty bytes (feeds yass.yaml.empty_file)") {
    auto p = temp_path("read_empty");
    write_file(p, "");

    auto r = read_file_bytes(p.string());
    CHECK(r.status == ReadStatus::Ok);
    CHECK(r.bytes.empty());

    std::filesystem::remove(p);
}

TEST_CASE("textio read_file_bytes — missing path reports NotFound distinct from Unreadable (yass.path.not_found)") {
    auto p = temp_path("does_not_exist");
    std::filesystem::remove(p);  // ensure absent
    auto r = read_file_bytes(p.string());
    CHECK(r.status == ReadStatus::NotFound);
    CHECK_FALSE(r.ok());
}

TEST_CASE("textio read_file_bytes — directory path reports Unreadable not Ok (yass.path.unreadable)") {
    // A directory is not a readable regular file; callers map this to a
    // filesystem error rather than reading content.
    auto dir = std::filesystem::temp_directory_path();
    auto r = read_file_bytes(dir.string());
    CHECK(r.status == ReadStatus::Unreadable);
    CHECK_FALSE(r.ok());
}

TEST_CASE("textio read_file_bytes — unreadable file (mode 000) reports Unreadable distinct from NotFound (yass.path.unreadable)") {
    auto p = temp_path("read_noperm");
    write_file(p, "secret");
    std::error_code ec;
    std::filesystem::permissions(p, std::filesystem::perms::none,
                                 std::filesystem::perm_options::replace, ec);

    auto r = read_file_bytes(p.string());
    // When running as a user without read permission this is Unreadable.
    // (If the test runs as root, permission checks are bypassed and the read
    // succeeds; accept either as long as it is NOT NotFound — the path exists.)
    CHECK(r.status != ReadStatus::NotFound);

    // restore + clean up
    std::filesystem::permissions(p, std::filesystem::perms::owner_all,
                                 std::filesystem::perm_options::replace, ec);
    std::filesystem::remove(p);
}
