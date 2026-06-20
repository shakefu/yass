/*
 * tinytest.h - Minimal C test framework
 *
 * Usage:
 *   TEST(name) { ASSERT(...); ASSERT_EQ(...); ASSERT_STR_EQ(...); }
 *   REGISTER_SUITE(name, test1, test2, ...);
 *   In main: RUN_SUITE(name);
 */
#ifndef TINYTEST_H
#define TINYTEST_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Global counters - defined in test_main.c */
extern int tt_total;
extern int tt_passed;
extern int tt_failed;
extern int tt_current_failed;

#define TEST(name) static void test_##name(void)

#define ASSERT(expr) do { \
    if (!(expr)) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT(%s)\n", __FILE__, __LINE__, #expr); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_EQ(a, b) do { \
    long long _a = (long long)(a), _b = (long long)(b); \
    if (_a != _b) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_EQ(%s, %s) => %lld != %lld\n", \
                __FILE__, __LINE__, #a, #b, _a, _b); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_NEQ(a, b) do { \
    long long _a = (long long)(a), _b = (long long)(b); \
    if (_a == _b) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_NEQ(%s, %s) => both %lld\n", \
                __FILE__, __LINE__, #a, #b, _a); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_STR_EQ(a, b) do { \
    const char *_a = (a), *_b = (b); \
    if (_a == NULL || _b == NULL || strcmp(_a, _b) != 0) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_STR_EQ(%s, %s) => \"%s\" != \"%s\"\n", \
                __FILE__, __LINE__, #a, #b, _a ? _a : "(null)", _b ? _b : "(null)"); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_STR_CONTAINS(haystack, needle) do { \
    const char *_h = (haystack), *_n = (needle); \
    if (_h == NULL || _n == NULL || strstr(_h, _n) == NULL) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_STR_CONTAINS(%s, %s) => \"%s\" does not contain \"%s\"\n", \
                __FILE__, __LINE__, #haystack, #needle, _h ? _h : "(null)", _n ? _n : "(null)"); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_NULL(expr) do { \
    if ((expr) != NULL) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_NULL(%s)\n", __FILE__, __LINE__, #expr); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_NOT_NULL(expr) do { \
    if ((expr) == NULL) { \
        fprintf(stderr, "  FAIL: %s:%d: ASSERT_NOT_NULL(%s)\n", __FILE__, __LINE__, #expr); \
        tt_current_failed = 1; \
        return; \
    } \
} while(0)

#define ASSERT_TRUE(expr) ASSERT(expr)
#define ASSERT_FALSE(expr) ASSERT(!(expr))

typedef void (*tt_test_fn)(void);

typedef struct {
    const char *name;
    tt_test_fn fn;
} tt_test_case;

#define RUN_TEST(name) do { \
    tt_total++; \
    tt_current_failed = 0; \
    test_##name(); \
    if (tt_current_failed) { \
        fprintf(stderr, "  FAIL: %s\n", #name); \
        tt_failed++; \
    } else { \
        tt_passed++; \
    } \
} while(0)

#define RUN_SUITE(name) do { \
    fprintf(stderr, "Suite: %s\n", #name); \
    run_suite_##name(); \
} while(0)

#define TT_SUMMARY() do { \
    fprintf(stderr, "\n%d tests, %d passed, %d failed\n", tt_total, tt_passed, tt_failed); \
    return tt_failed > 0 ? 1 : 0; \
} while(0)

#endif /* TINYTEST_H */
