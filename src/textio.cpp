#include "textio.hpp"

#include <cerrno>
#include <cstdio>
#include <filesystem>
#include <system_error>

namespace yass::textio {

namespace {

// One unsigned byte from a string_view at index i (caller guarantees range).
inline unsigned char byte_at(std::string_view s, std::size_t i) {
    return static_cast<unsigned char>(s[i]);
}

// A continuation byte has the high bits 10xxxxxx.
inline bool is_cont(unsigned char b) { return (b & 0xC0u) == 0x80u; }

}  // namespace

// --------------------------------------------------------------------------
// read_file_bytes
// --------------------------------------------------------------------------
ReadResult read_file_bytes(std::string_view path) {
    ReadResult result;

    std::string path_str(path);

    // Distinguish "does not exist" from "exists but unreadable / wrong type"
    // up front using a stat, so callers can map to not_found vs unreadable.
    std::error_code ec;
    std::filesystem::file_status st = std::filesystem::status(path_str, ec);
    if (ec) {
        // status() failed. ENOENT-family => NotFound, anything else =>
        // Unreadable (e.g. EACCES on a parent directory component).
        if (st.type() == std::filesystem::file_type::not_found ||
            ec == std::errc::no_such_file_or_directory) {
            result.status = ReadStatus::NotFound;
        } else {
            result.status = ReadStatus::Unreadable;
        }
        return result;
    }
    if (st.type() == std::filesystem::file_type::not_found) {
        result.status = ReadStatus::NotFound;
        return result;
    }
    if (st.type() != std::filesystem::file_type::regular) {
        // Directories / sockets / fifos / devices are not readable as a file
        // here; callers treat this as a filesystem (unreadable) error.
        result.status = ReadStatus::Unreadable;
        return result;
    }

    std::FILE* fp = std::fopen(path_str.c_str(), "rb");
    if (fp == nullptr) {
        result.status =
            (errno == ENOENT) ? ReadStatus::NotFound : ReadStatus::Unreadable;
        return result;
    }

    std::string bytes;
    char buf[65536];
    std::size_t n = 0;
    while ((n = std::fread(buf, 1, sizeof(buf), fp)) > 0) {
        bytes.append(buf, n);
    }
    bool read_error = (std::ferror(fp) != 0);
    std::fclose(fp);

    if (read_error) {
        result.status = ReadStatus::Unreadable;
        return result;
    }

    result.status = ReadStatus::Ok;
    result.bytes = std::move(bytes);
    return result;
}

// --------------------------------------------------------------------------
// is_valid_utf8
// --------------------------------------------------------------------------
bool is_valid_utf8(std::string_view s) {
    const std::size_t n = s.size();
    std::size_t i = 0;
    while (i < n) {
        unsigned char b0 = byte_at(s, i);

        if (b0 < 0x80u) {
            // ASCII, single byte 0x00..0x7F.
            ++i;
            continue;
        }

        // Determine sequence length and the valid range of the first
        // continuation byte to forbid overlong forms and surrogates.
        std::size_t len;
        unsigned char lo;  // min legal value of the SECOND byte
        unsigned char hi;  // max legal value of the SECOND byte

        if (b0 >= 0xC2u && b0 <= 0xDFu) {
            // 2-byte: U+0080 .. U+07FF. (C0/C1 are always overlong.)
            len = 2;
            lo = 0x80u;
            hi = 0xBFu;
        } else if (b0 == 0xE0u) {
            // 3-byte, restrict 2nd byte to A0..BF to forbid overlong.
            len = 3;
            lo = 0xA0u;
            hi = 0xBFu;
        } else if (b0 >= 0xE1u && b0 <= 0xECu) {
            len = 3;
            lo = 0x80u;
            hi = 0xBFu;
        } else if (b0 == 0xEDu) {
            // 3-byte, restrict 2nd byte to 80..9F to forbid surrogates
            // (U+D800..U+DFFF would need ED A0..BF).
            len = 3;
            lo = 0x80u;
            hi = 0x9Fu;
        } else if (b0 >= 0xEEu && b0 <= 0xEFu) {
            len = 3;
            lo = 0x80u;
            hi = 0xBFu;
        } else if (b0 == 0xF0u) {
            // 4-byte, restrict 2nd byte to 90..BF to forbid overlong.
            len = 4;
            lo = 0x90u;
            hi = 0xBFu;
        } else if (b0 >= 0xF1u && b0 <= 0xF3u) {
            len = 4;
            lo = 0x80u;
            hi = 0xBFu;
        } else if (b0 == 0xF4u) {
            // 4-byte, restrict 2nd byte to 80..8F to cap at U+10FFFF.
            len = 4;
            lo = 0x80u;
            hi = 0x8Fu;
        } else {
            // 0x80..0xBF (stray continuation), 0xC0/0xC1 (overlong),
            // 0xF5..0xFF (> U+10FFFF / invalid): all illegal lead bytes.
            return false;
        }

        // Need the full sequence present (reject truncated).
        if (i + len > n) {
            return false;
        }

        // Second byte: range-restricted to kill overlong/surrogate/out-of-range.
        unsigned char b1 = byte_at(s, i + 1);
        if (b1 < lo || b1 > hi) {
            return false;
        }

        // Remaining continuation bytes: plain 0x80..0xBF.
        for (std::size_t k = 2; k < len; ++k) {
            if (!is_cont(byte_at(s, i + k))) {
                return false;
            }
        }

        i += len;
    }
    return true;
}

// --------------------------------------------------------------------------
// has_utf8_bom
// --------------------------------------------------------------------------
bool has_utf8_bom(std::string_view s) {
    return s.size() >= 3 && byte_at(s, 0) == 0xEFu && byte_at(s, 1) == 0xBBu &&
           byte_at(s, 2) == 0xBFu;
}

// --------------------------------------------------------------------------
// decode_utf8 / count_codepoints
// --------------------------------------------------------------------------
namespace {

// Decode the sequence starting at index i. On success, write the scalar to
// `out` and return its byte length. On any malformedness, leave `out` as
// U+FFFD and return 1 (advance one byte, total function).
std::size_t decode_one(std::string_view s, std::size_t i, char32_t& out) {
    const std::size_t n = s.size();
    unsigned char b0 = byte_at(s, i);

    if (b0 < 0x80u) {
        out = static_cast<char32_t>(b0);
        return 1;
    }

    std::size_t len;
    unsigned char lo, hi;
    char32_t cp;

    if (b0 >= 0xC2u && b0 <= 0xDFu) {
        len = 2;
        lo = 0x80u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x1Fu);
    } else if (b0 == 0xE0u) {
        len = 3;
        lo = 0xA0u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x0Fu);
    } else if (b0 >= 0xE1u && b0 <= 0xECu) {
        len = 3;
        lo = 0x80u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x0Fu);
    } else if (b0 == 0xEDu) {
        len = 3;
        lo = 0x80u;
        hi = 0x9Fu;
        cp = static_cast<char32_t>(b0 & 0x0Fu);
    } else if (b0 >= 0xEEu && b0 <= 0xEFu) {
        len = 3;
        lo = 0x80u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x0Fu);
    } else if (b0 == 0xF0u) {
        len = 4;
        lo = 0x90u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x07u);
    } else if (b0 >= 0xF1u && b0 <= 0xF3u) {
        len = 4;
        lo = 0x80u;
        hi = 0xBFu;
        cp = static_cast<char32_t>(b0 & 0x07u);
    } else if (b0 == 0xF4u) {
        len = 4;
        lo = 0x80u;
        hi = 0x8Fu;
        cp = static_cast<char32_t>(b0 & 0x07u);
    } else {
        out = 0xFFFDu;
        return 1;
    }

    if (i + len > n) {
        out = 0xFFFDu;
        return 1;
    }

    unsigned char b1 = byte_at(s, i + 1);
    if (b1 < lo || b1 > hi) {
        out = 0xFFFDu;
        return 1;
    }
    cp = (cp << 6) | static_cast<char32_t>(b1 & 0x3Fu);

    for (std::size_t k = 2; k < len; ++k) {
        unsigned char bk = byte_at(s, i + k);
        if (!is_cont(bk)) {
            out = 0xFFFDu;
            return 1;
        }
        cp = (cp << 6) | static_cast<char32_t>(bk & 0x3Fu);
    }

    out = cp;
    return len;
}

}  // namespace

std::vector<char32_t> decode_utf8(std::string_view s) {
    std::vector<char32_t> out;
    out.reserve(s.size());
    std::size_t i = 0;
    while (i < s.size()) {
        char32_t cp = 0;
        i += decode_one(s, i, cp);
        out.push_back(cp);
    }
    return out;
}

std::size_t count_codepoints(std::string_view s) {
    std::size_t count = 0;
    std::size_t i = 0;
    while (i < s.size()) {
        char32_t cp = 0;
        i += decode_one(s, i, cp);
        ++count;
    }
    return count;
}

// --------------------------------------------------------------------------
// nfc
// --------------------------------------------------------------------------
std::string nfc(std::string_view s) {
    // See header: full Unicode NFC needs the UCD tables we don't bundle.
    // ASCII is already NFC -> identity; non-ASCII is passed through unchanged
    // (treated as already-NFC). This is the single normalization boundary.
    return std::string(s);
}

// --------------------------------------------------------------------------
// codepoint_less
// --------------------------------------------------------------------------
bool codepoint_less(std::string_view a, std::string_view b) {
    // For valid UTF-8, code-point order == unsigned byte order (see header).
    // std::string_view::compare uses char_traits<char>::compare which on every
    // supported platform compares as unsigned char, but we make the unsigned
    // semantics explicit and locale-free here to satisfy the MUST-NOT
    // locale-aware / MUST-NOT case-fold obligations beyond doubt.
    const std::size_t n = a.size() < b.size() ? a.size() : b.size();
    for (std::size_t i = 0; i < n; ++i) {
        unsigned char ca = static_cast<unsigned char>(a[i]);
        unsigned char cb = static_cast<unsigned char>(b[i]);
        if (ca != cb) {
            return ca < cb;
        }
    }
    return a.size() < b.size();
}

}  // namespace yass::textio
