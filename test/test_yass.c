/*
 * test_yass.c - Tests for yass.c common utility functions
 */
#include "tinytest.h"
#include "../src/yass.h"

/* --- yass_is_keyword (case-sensitive) --- */

TEST(keyword_exact_match) {
    ASSERT_TRUE(yass_is_keyword("INPUT", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("RETURN", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("ERROR", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("SIDE-EFFECT", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("INVARIANT", SLOT_KEYWORDS));
}

TEST(keyword_exact_match_normativity) {
    ASSERT_TRUE(yass_is_keyword("MUST", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("MUST-NOT", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("SHOULD", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("SHOULD-NOT", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("MAY", NORMATIVITY_KEYWORDS));
}

TEST(keyword_exact_match_reference) {
    ASSERT_TRUE(yass_is_keyword("CONFORMS", REFERENCE_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("USES", REFERENCE_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("SEE", REFERENCE_KEYWORDS));
}

TEST(keyword_case_mismatch_fails) {
    ASSERT_FALSE(yass_is_keyword("input", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("Input", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("must", NORMATIVITY_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("conforms", REFERENCE_KEYWORDS));
}

TEST(keyword_no_match) {
    ASSERT_FALSE(yass_is_keyword("FOOBAR", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("INPU", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("INPUTS", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("MUSTS", NORMATIVITY_KEYWORDS));
}

TEST(keyword_empty_string) {
    ASSERT_FALSE(yass_is_keyword("", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("", NORMATIVITY_KEYWORDS));
}

TEST(keyword_null_name) {
    ASSERT_FALSE(yass_is_keyword(NULL, SLOT_KEYWORDS));
}

TEST(keyword_null_keywords) {
    ASSERT_FALSE(yass_is_keyword("INPUT", NULL));
}

TEST(keyword_both_null) {
    ASSERT_FALSE(yass_is_keyword(NULL, NULL));
}

/* --- yass_is_keyword_ci (case-insensitive) --- */

TEST(keyword_ci_exact_match) {
    ASSERT_TRUE(yass_is_keyword_ci("INPUT", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("MUST", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("CONFORMS", REFERENCE_KEYWORDS));
}

TEST(keyword_ci_lowercase) {
    ASSERT_TRUE(yass_is_keyword_ci("input", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("return", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("error", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("side-effect", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("invariant", SLOT_KEYWORDS));
}

TEST(keyword_ci_mixed_case) {
    ASSERT_TRUE(yass_is_keyword_ci("Input", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("rEtUrN", SLOT_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("Must", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("must-not", NORMATIVITY_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword_ci("Conforms", REFERENCE_KEYWORDS));
}

TEST(keyword_ci_no_match) {
    ASSERT_FALSE(yass_is_keyword_ci("FOOBAR", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword_ci("foobar", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword_ci("INPU", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword_ci("INPUTS", SLOT_KEYWORDS));
}

TEST(keyword_ci_empty_string) {
    ASSERT_FALSE(yass_is_keyword_ci("", SLOT_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword_ci("", NORMATIVITY_KEYWORDS));
}

TEST(keyword_ci_null_name) {
    ASSERT_FALSE(yass_is_keyword_ci(NULL, SLOT_KEYWORDS));
}

TEST(keyword_ci_null_keywords) {
    ASSERT_FALSE(yass_is_keyword_ci("INPUT", NULL));
}

TEST(keyword_ci_both_null) {
    ASSERT_FALSE(yass_is_keyword_ci(NULL, NULL));
}

/* --- reserved keywords --- */

TEST(keyword_reserved_all) {
    ASSERT_TRUE(yass_is_keyword("INPUT", RESERVED_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("MUST", RESERVED_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("WHEN", RESERVED_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("CONFORMS", RESERVED_KEYWORDS));
    ASSERT_TRUE(yass_is_keyword("SEE", RESERVED_KEYWORDS));
    ASSERT_FALSE(yass_is_keyword("BOGUS", RESERVED_KEYWORDS));
}

void run_suite_yass(void) {
    /* case-sensitive */
    RUN_TEST(keyword_exact_match);
    RUN_TEST(keyword_exact_match_normativity);
    RUN_TEST(keyword_exact_match_reference);
    RUN_TEST(keyword_case_mismatch_fails);
    RUN_TEST(keyword_no_match);
    RUN_TEST(keyword_empty_string);
    RUN_TEST(keyword_null_name);
    RUN_TEST(keyword_null_keywords);
    RUN_TEST(keyword_both_null);
    /* case-insensitive */
    RUN_TEST(keyword_ci_exact_match);
    RUN_TEST(keyword_ci_lowercase);
    RUN_TEST(keyword_ci_mixed_case);
    RUN_TEST(keyword_ci_no_match);
    RUN_TEST(keyword_ci_empty_string);
    RUN_TEST(keyword_ci_null_name);
    RUN_TEST(keyword_ci_null_keywords);
    RUN_TEST(keyword_ci_both_null);
    /* reserved */
    RUN_TEST(keyword_reserved_all);
}
