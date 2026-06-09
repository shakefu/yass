#pragma once

// M2 — text & file IO primitives.
//
// Spec basis:
//   - yass.yass.yaml :: Document INPUT
//       * MUST: be encoded as UTF-8            -> is_valid_utf8
//       * MUST-NOT: contain a leading UTF-8 BOM -> has_utf8_bom
//   - spec/cli.errors.yass.yaml :: cli.errors RETURN
//       * yass.yaml.not_utf8   (file is not valid UTF-8)   <- is_valid_utf8 == false
//       * yass.yaml.has_bom    (file begins with a UTF-8 BOM) <- has_utf8_bom == true
//       * yass.yaml.empty_file (file is zero bytes)        <- read_file_bytes size 0
//       * yass.path.not_found / yass.path.unreadable       <- read_file_bytes status
//   - spec/cli.list.yass.yaml :: cli.list RETURN
//       * MUST: emit description text in NFC-normalized UTF-8 -> nfc
//       * MUST: sort files by Unicode code-point order on the NFC-normalized
//               UTF-8 path string                            -> codepoint_less
//       * MUST-NOT: use locale-aware collation / case-fold    -> codepoint_less
//   - spec/cli.shared.yass.yaml :: cli.DiscoverSpecFiles RETURN (same ordering rule)
//
// This module is the single boundary every caller routes UTF-8 validity, BOM
// detection, NFC normalization, code-point decoding, and code-point ordering
// through. Higher modules map results onto the cli.errors codes above; this
// module deliberately stays code-agnostic (it returns booleans / status enums,
// not yass::diag::ErrorCode).

#include <cstdint>
#include <string>
#include <string_view>
#include <vector>

namespace yass::textio {

// --------------------------------------------------------------------------
// File reading.
// --------------------------------------------------------------------------
// Distinguish the cases callers must map onto distinct cli.errors codes:
//   NotFound   -> yass.path.not_found
//   Unreadable -> yass.path.unreadable  (exists but cannot be opened/read,
//                 e.g. permission denied, or a non-regular/non-directory type)
//   Ok         -> bytes are populated (possibly empty -> yass.yaml.empty_file)
enum class ReadStatus {
    Ok,
    NotFound,
    Unreadable,
};

struct ReadResult {
    ReadStatus status = ReadStatus::Unreadable;
    // Raw bytes exactly as on disk, no transcoding, BOM left intact. Valid
    // only when status == Ok. May be empty for a zero-byte file (status Ok).
    std::string bytes;

    bool ok() const { return status == ReadStatus::Ok; }
};

// Read the raw bytes of the regular file at `path`. No transcoding, no BOM
// stripping; the file is opened in binary mode and read verbatim. A missing
// path yields ReadStatus::NotFound; any other open/read failure (including a
// path that exists but is not a readable regular file) yields
// ReadStatus::Unreadable. On success the bytes (possibly empty) are returned
// with ReadStatus::Ok.
ReadResult read_file_bytes(std::string_view path);

// --------------------------------------------------------------------------
// UTF-8 validation (Document INPUT: MUST be encoded as UTF-8).
// --------------------------------------------------------------------------
// Strict, well-formedness-checked UTF-8 per the Unicode "shortest form"
// constraints. Rejects:
//   - overlong encodings (e.g. C0 80 for U+0000, E0 80 .. for low code points),
//   - UTF-16 surrogate halves U+D800..U+DFFF,
//   - code points above U+10FFFF,
//   - truncated / dangling continuation bytes,
//   - stray continuation bytes (0x80..0xBF) in lead position,
//   - the invalid lead bytes 0xC0, 0xC1, 0xF5..0xFF.
// A BOM (EF BB BF) is *valid* UTF-8 here; BOM rejection is a separate concern
// (see has_utf8_bom) so the not_utf8 and has_bom codes stay independent.
bool is_valid_utf8(std::string_view s);

// --------------------------------------------------------------------------
// BOM detection (Document INPUT: MUST-NOT contain a leading UTF-8 BOM).
// --------------------------------------------------------------------------
// True iff `s` begins with the three UTF-8 BOM bytes EF BB BF.
bool has_utf8_bom(std::string_view s);

// --------------------------------------------------------------------------
// Code-point decoding (underpins grapheme/width counting in cli.list).
// --------------------------------------------------------------------------
// Decode valid UTF-8 `s` into its sequence of Unicode scalar code points.
// PRECONDITION: callers should validate with is_valid_utf8 first. To stay
// total even on malformed input, any byte that does not start a well-formed
// sequence is decoded to U+FFFD (replacement character) and scanning resumes
// at the next byte, so the function never throws and never reads out of range.
std::vector<char32_t> decode_utf8(std::string_view s);

// Number of Unicode code points in valid UTF-8 `s` (== decode_utf8(s).size(),
// computed without materializing the vector). On malformed input each
// undecodable byte counts as one replacement code point, mirroring decode_utf8.
std::size_t count_codepoints(std::string_view s);

// --------------------------------------------------------------------------
// NFC normalization (cli.list RETURN: emit description text in NFC).
// --------------------------------------------------------------------------
// Return the NFC (Normalization Form C) of `s`.
//
// IMPORTANT — SCOPE / APPROXIMATION: full Unicode canonical
// decomposition + canonical ordering + canonical composition requires the
// Unicode character database (decomposition mappings, combining classes,
// composition exclusions). Those tables are NOT bundled in this build, so this
// implementation is an APPROXIMATION:
//   - Pure-ASCII input (every byte < 0x80) is returned UNCHANGED. ASCII is
//     already in NFC, so this is exact for the dominant case (paths and most
//     descriptions in this codebase).
//   - Any non-ASCII input is returned UNCHANGED (treated as already-NFC).
// This is the documented identity boundary: callers ALWAYS route description
// and path text through nfc(), so when real NFC tables are added later only
// this one function changes and every call site upgrades for free. Inputs that
// are already in NFC (the common case) are correct today; inputs that are not
// already normalized are passed through unchanged rather than mis-normalized.
std::string nfc(std::string_view s);

// --------------------------------------------------------------------------
// Code-point ordering (cli.list / cli.DiscoverSpecFiles sort key).
// --------------------------------------------------------------------------
// Strict weak ordering implementing Unicode code-point order over `a` and `b`.
//
// EQUIVALENCE NOTE: for well-formed UTF-8, lexicographic comparison of the
// sequences of Unicode scalar code points is byte-for-byte identical to
// lexicographic comparison of the raw bytes as UNSIGNED octets — this is a
// designed property of the UTF-8 encoding (a code point that is numerically
// larger always encodes to a byte sequence that is byte-wise greater). We
// therefore implement code-point order as an unsigned byte comparison, which is
// both correct for valid UTF-8 and free of any locale or case-folding (it
// satisfies cli.shared's MUST-NOT use locale-aware collation / MUST-NOT
// case-fold). Returns true iff `a` sorts strictly before `b`.
bool codepoint_less(std::string_view a, std::string_view b);

}  // namespace yass::textio
