/*
 * test_utf8.c - Tests for UTF-8 utilities
 */
#include "tinytest.h"
#include "../src/utf8.h"
#include "../src/utf8.c"

#include <stdint.h>
#include <string.h>
#include <stdlib.h>

/* --- utf8_is_valid --- */

TEST(valid_ascii) {
    const unsigned char s[] = "Hello, world!";
    ASSERT_TRUE(utf8_is_valid(s, strlen((const char *)s)));
}

TEST(valid_empty) {
    ASSERT_TRUE(utf8_is_valid((const unsigned char *)"", 0));
}

TEST(valid_2byte) {
    /* U+00E9 = C3 A9 (e with acute) */
    const unsigned char s[] = {0xC3, 0xA9, 0};
    ASSERT_TRUE(utf8_is_valid(s, 2));
}

TEST(valid_3byte) {
    /* U+20AC = E2 82 AC (euro sign) */
    const unsigned char s[] = {0xE2, 0x82, 0xAC, 0};
    ASSERT_TRUE(utf8_is_valid(s, 3));
}

TEST(valid_4byte) {
    /* U+1F600 = F0 9F 98 80 (grinning face) */
    const unsigned char s[] = {0xF0, 0x9F, 0x98, 0x80, 0};
    ASSERT_TRUE(utf8_is_valid(s, 4));
}

TEST(valid_mixed) {
    /* "cafe\xCC\x81" = "cafe" + combining acute accent */
    const unsigned char s[] = {'c', 'a', 'f', 'e', 0xCC, 0x81, 0};
    ASSERT_TRUE(utf8_is_valid(s, 6));
}

TEST(invalid_truncated_2byte) {
    /* Start of 2-byte sequence but missing continuation */
    const unsigned char s[] = {0xC3};
    ASSERT_FALSE(utf8_is_valid(s, 1));
}

TEST(invalid_truncated_3byte) {
    /* Start of 3-byte sequence with only one continuation */
    const unsigned char s[] = {0xE2, 0x82};
    ASSERT_FALSE(utf8_is_valid(s, 2));
}

TEST(invalid_truncated_4byte) {
    /* Start of 4-byte sequence with only two continuations */
    const unsigned char s[] = {0xF0, 0x9F, 0x98};
    ASSERT_FALSE(utf8_is_valid(s, 3));
}

TEST(invalid_continuation_alone) {
    /* Bare continuation byte */
    const unsigned char s[] = {0x80};
    ASSERT_FALSE(utf8_is_valid(s, 1));
}

TEST(invalid_overlong_2byte) {
    /* Overlong encoding of U+0000: C0 80 */
    const unsigned char s[] = {0xC0, 0x80};
    ASSERT_FALSE(utf8_is_valid(s, 2));
}

TEST(invalid_overlong_slash) {
    /* Overlong encoding of '/' (U+002F): C0 AF */
    const unsigned char s[] = {0xC0, 0xAF};
    ASSERT_FALSE(utf8_is_valid(s, 2));
}

TEST(invalid_overlong_3byte) {
    /* Overlong encoding of U+007F: E0 81 BF */
    const unsigned char s[] = {0xE0, 0x81, 0xBF};
    ASSERT_FALSE(utf8_is_valid(s, 3));
}

TEST(invalid_surrogate_high) {
    /* U+D800 = ED A0 80 (high surrogate) */
    const unsigned char s[] = {0xED, 0xA0, 0x80};
    ASSERT_FALSE(utf8_is_valid(s, 3));
}

TEST(invalid_surrogate_low) {
    /* U+DFFF = ED BF BF (low surrogate) */
    const unsigned char s[] = {0xED, 0xBF, 0xBF};
    ASSERT_FALSE(utf8_is_valid(s, 3));
}

TEST(invalid_too_large) {
    /* U+110000 = F4 90 80 80 (beyond U+10FFFF) */
    const unsigned char s[] = {0xF4, 0x90, 0x80, 0x80};
    ASSERT_FALSE(utf8_is_valid(s, 4));
}

TEST(invalid_fe_byte) {
    const unsigned char s[] = {0xFE};
    ASSERT_FALSE(utf8_is_valid(s, 1));
}

TEST(invalid_ff_byte) {
    const unsigned char s[] = {0xFF};
    ASSERT_FALSE(utf8_is_valid(s, 1));
}

TEST(invalid_bad_continuation) {
    /* 2-byte start followed by non-continuation */
    const unsigned char s[] = {0xC3, 0x00};
    ASSERT_FALSE(utf8_is_valid(s, 2));
}

/* --- utf8_has_bom --- */

TEST(bom_present) {
    const unsigned char s[] = {0xEF, 0xBB, 0xBF, 'h', 'i'};
    ASSERT_TRUE(utf8_has_bom(s, 5));
}

TEST(bom_absent) {
    const unsigned char s[] = "hello";
    ASSERT_FALSE(utf8_has_bom(s, 5));
}

TEST(bom_too_short) {
    const unsigned char s[] = {0xEF, 0xBB};
    ASSERT_FALSE(utf8_has_bom(s, 2));
}

TEST(bom_empty) {
    ASSERT_FALSE(utf8_has_bom((const unsigned char *)"", 0));
}

TEST(bom_exact) {
    /* Exactly the BOM and nothing else */
    const unsigned char s[] = {0xEF, 0xBB, 0xBF};
    ASSERT_TRUE(utf8_has_bom(s, 3));
}

/* --- utf8_nfc_normalize --- */

TEST(nfc_ascii) {
    char *r = utf8_nfc_normalize("hello");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello");
    free(r);
}

TEST(nfc_null) {
    ASSERT_NULL(utf8_nfc_normalize(NULL));
}

TEST(nfc_empty) {
    char *r = utf8_nfc_normalize("");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "");
    free(r);
}

/* --- utf8_normalize_whitespace --- */

TEST(ws_simple_spaces) {
    char *r = utf8_normalize_whitespace("hello   world");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_tabs) {
    char *r = utf8_normalize_whitespace("hello\t\tworld");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_newlines) {
    char *r = utf8_normalize_whitespace("hello\n\nworld");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_carriage_return) {
    char *r = utf8_normalize_whitespace("hello\r\nworld");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_mixed) {
    char *r = utf8_normalize_whitespace("  hello \t\n\r world  ");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_leading) {
    char *r = utf8_normalize_whitespace("   hello");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello");
    free(r);
}

TEST(ws_trailing) {
    char *r = utf8_normalize_whitespace("hello   ");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello");
    free(r);
}

TEST(ws_all_whitespace) {
    char *r = utf8_normalize_whitespace("   \t\n\r   ");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "");
    free(r);
}

TEST(ws_empty) {
    char *r = utf8_normalize_whitespace("");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "");
    free(r);
}

TEST(ws_null) {
    ASSERT_NULL(utf8_normalize_whitespace(NULL));
}

TEST(ws_no_change) {
    char *r = utf8_normalize_whitespace("hello world");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello world");
    free(r);
}

TEST(ws_single_word) {
    char *r = utf8_normalize_whitespace("hello");
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello");
    free(r);
}

/* --- utf8_grapheme_count --- */

TEST(gc_ascii) {
    ASSERT_EQ(utf8_grapheme_count("hello"), 5);
}

TEST(gc_empty) {
    ASSERT_EQ(utf8_grapheme_count(""), 0);
}

TEST(gc_null) {
    ASSERT_EQ(utf8_grapheme_count(NULL), 0);
}

TEST(gc_2byte_chars) {
    /* "cafe\xCC\x81" = cafe + combining acute = 4 graphemes not 5 */
    ASSERT_EQ(utf8_grapheme_count("caf\xC3\xA9"), 4);  /* precomposed e-acute */
}

TEST(gc_combining_mark) {
    /* 'e' + combining acute (U+0301): should be 1 grapheme, not 2 */
    ASSERT_EQ(utf8_grapheme_count("e\xCC\x81"), 1);
}

TEST(gc_multiple_combining) {
    /* 'a' + combining acute + combining tilde: still 1 grapheme */
    ASSERT_EQ(utf8_grapheme_count("a\xCC\x81\xCC\x83"), 1);
}

TEST(gc_emoji) {
    /* U+1F600 grinning face = 1 grapheme */
    ASSERT_EQ(utf8_grapheme_count("\xF0\x9F\x98\x80"), 1);
}

TEST(gc_mixed) {
    /* "hi" + emoji + "!" = 4 graphemes */
    ASSERT_EQ(utf8_grapheme_count("hi\xF0\x9F\x98\x80!"), 4);
}

TEST(gc_euro_sign) {
    /* 3 euro signs = 3 graphemes */
    ASSERT_EQ(utf8_grapheme_count("\xE2\x82\xAC\xE2\x82\xAC\xE2\x82\xAC"), 3);
}

/* --- utf8_truncate_graphemes --- */

TEST(trunc_ascii) {
    char *r = utf8_truncate_graphemes("hello world", 5);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hello");
    free(r);
}

TEST(trunc_no_trunc) {
    char *r = utf8_truncate_graphemes("hi", 10);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "hi");
    free(r);
}

TEST(trunc_exact) {
    char *r = utf8_truncate_graphemes("abc", 3);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "abc");
    free(r);
}

TEST(trunc_zero) {
    char *r = utf8_truncate_graphemes("hello", 0);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "");
    free(r);
}

TEST(trunc_multibyte) {
    /* Three euro signs (each 3 bytes), truncate to 2 */
    char *r = utf8_truncate_graphemes("\xE2\x82\xAC\xE2\x82\xAC\xE2\x82\xAC", 2);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "\xE2\x82\xAC\xE2\x82\xAC");
    free(r);
}

TEST(trunc_combining) {
    /* "e" + combining acute (2 codepoints, 1 grapheme) + "x" (1 grapheme)
     * Truncate to 1 grapheme should keep "e" + combining mark */
    char *r = utf8_truncate_graphemes("e\xCC\x81x", 1);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "e\xCC\x81");
    free(r);
}

TEST(trunc_null) {
    ASSERT_NULL(utf8_truncate_graphemes(NULL, 5));
}

TEST(trunc_empty) {
    char *r = utf8_truncate_graphemes("", 5);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "");
    free(r);
}

TEST(trunc_emoji) {
    /* Two emojis, truncate to 1 */
    char *r = utf8_truncate_graphemes("\xF0\x9F\x98\x80\xF0\x9F\x98\x82", 1);
    ASSERT_NOT_NULL(r);
    ASSERT_STR_EQ(r, "\xF0\x9F\x98\x80");
    free(r);
}

/* --- Suite runner --- */

void run_suite_utf8(void) {
    /* utf8_is_valid */
    RUN_TEST(valid_ascii);
    RUN_TEST(valid_empty);
    RUN_TEST(valid_2byte);
    RUN_TEST(valid_3byte);
    RUN_TEST(valid_4byte);
    RUN_TEST(valid_mixed);
    RUN_TEST(invalid_truncated_2byte);
    RUN_TEST(invalid_truncated_3byte);
    RUN_TEST(invalid_truncated_4byte);
    RUN_TEST(invalid_continuation_alone);
    RUN_TEST(invalid_overlong_2byte);
    RUN_TEST(invalid_overlong_slash);
    RUN_TEST(invalid_overlong_3byte);
    RUN_TEST(invalid_surrogate_high);
    RUN_TEST(invalid_surrogate_low);
    RUN_TEST(invalid_too_large);
    RUN_TEST(invalid_fe_byte);
    RUN_TEST(invalid_ff_byte);
    RUN_TEST(invalid_bad_continuation);

    /* utf8_has_bom */
    RUN_TEST(bom_present);
    RUN_TEST(bom_absent);
    RUN_TEST(bom_too_short);
    RUN_TEST(bom_empty);
    RUN_TEST(bom_exact);

    /* utf8_nfc_normalize */
    RUN_TEST(nfc_ascii);
    RUN_TEST(nfc_null);
    RUN_TEST(nfc_empty);

    /* utf8_normalize_whitespace */
    RUN_TEST(ws_simple_spaces);
    RUN_TEST(ws_tabs);
    RUN_TEST(ws_newlines);
    RUN_TEST(ws_carriage_return);
    RUN_TEST(ws_mixed);
    RUN_TEST(ws_leading);
    RUN_TEST(ws_trailing);
    RUN_TEST(ws_all_whitespace);
    RUN_TEST(ws_empty);
    RUN_TEST(ws_null);
    RUN_TEST(ws_no_change);
    RUN_TEST(ws_single_word);

    /* utf8_grapheme_count */
    RUN_TEST(gc_ascii);
    RUN_TEST(gc_empty);
    RUN_TEST(gc_null);
    RUN_TEST(gc_2byte_chars);
    RUN_TEST(gc_combining_mark);
    RUN_TEST(gc_multiple_combining);
    RUN_TEST(gc_emoji);
    RUN_TEST(gc_mixed);
    RUN_TEST(gc_euro_sign);

    /* utf8_truncate_graphemes */
    RUN_TEST(trunc_ascii);
    RUN_TEST(trunc_no_trunc);
    RUN_TEST(trunc_exact);
    RUN_TEST(trunc_zero);
    RUN_TEST(trunc_multibyte);
    RUN_TEST(trunc_combining);
    RUN_TEST(trunc_null);
    RUN_TEST(trunc_empty);
    RUN_TEST(trunc_emoji);
}
