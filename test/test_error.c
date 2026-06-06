/*
 * test_error.c - Tests for error formatting helpers
 */
#include "tinytest.h"
#include "error.h"

#include <stdlib.h>
#include <string.h>

/* --- error_format_path tests --- */

TEST(error_format_path_null_returns_yass)
{
    char *result = error_format_path(NULL);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "yass");
    free(result);
}

TEST(error_format_path_relative)
{
    /* A relative path should come back relative to cwd (unchanged when
     * it is already relative and under cwd). */
    char *result = error_format_path("src/error.c");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "src/error.c");
    free(result);
}

TEST(error_format_path_absolute_outside_cwd)
{
    /* An absolute path outside cwd should stay absolute. */
    char *result = error_format_path("/tmp/nowhere/file.yass.yaml");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "/tmp/nowhere/file.yass.yaml");
    free(result);
}

/* --- error_sanitize_message tests --- */

TEST(error_sanitize_message_no_newlines)
{
    char *result = error_sanitize_message("hello world");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "hello world");
    free(result);
}

TEST(error_sanitize_message_newline)
{
    char *result = error_sanitize_message("line1\nline2");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "line1 line2");
    free(result);
}

TEST(error_sanitize_message_carriage_return)
{
    char *result = error_sanitize_message("line1\rline2");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "line1 line2");
    free(result);
}

TEST(error_sanitize_message_crlf)
{
    char *result = error_sanitize_message("line1\r\nline2");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "line1  line2");
    free(result);
}

TEST(error_sanitize_message_null)
{
    char *result = error_sanitize_message(NULL);
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "");
    free(result);
}

TEST(error_sanitize_message_empty)
{
    char *result = error_sanitize_message("");
    ASSERT_NOT_NULL(result);
    ASSERT_STR_EQ(result, "");
    free(result);
}

/* --- error_counter tests --- */

TEST(error_counter_init_zero)
{
    error_counter_t ec;
    error_counter_init(&ec);
    ASSERT_EQ(ec.count, 0);
}

TEST(error_counter_increment)
{
    error_counter_t ec;
    error_counter_init(&ec);
    error_count(&ec);
    ASSERT_EQ(ec.count, 1);
    error_count(&ec);
    error_count(&ec);
    ASSERT_EQ(ec.count, 3);
}

/* --- Suite runner --- */

void run_suite_error(void)
{
    RUN_TEST(error_format_path_null_returns_yass);
    RUN_TEST(error_format_path_relative);
    RUN_TEST(error_format_path_absolute_outside_cwd);
    RUN_TEST(error_sanitize_message_no_newlines);
    RUN_TEST(error_sanitize_message_newline);
    RUN_TEST(error_sanitize_message_carriage_return);
    RUN_TEST(error_sanitize_message_crlf);
    RUN_TEST(error_sanitize_message_null);
    RUN_TEST(error_sanitize_message_empty);
    RUN_TEST(error_counter_init_zero);
    RUN_TEST(error_counter_increment);
}
