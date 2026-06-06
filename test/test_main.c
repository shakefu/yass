/*
 * test_main.c - Test runner for yass CLI
 */
#include "tinytest.h"

/* Define the global counters */
int tt_total = 0;
int tt_passed = 0;
int tt_failed = 0;
int tt_current_failed = 0;

/* Suite declarations - each test file defines a run_suite_<name>() function */
extern void run_suite_yass(void);
extern void run_suite_utf8(void);
extern void run_suite_path(void);
extern void run_suite_error(void);
extern void run_suite_argv(void);
extern void run_suite_yaml_parse(void);
extern void run_suite_discover(void);
extern void run_suite_glob(void);
extern void run_suite_check_yaml(void);
extern void run_suite_check_preamble(void);
extern void run_suite_check_spec(void);
extern void run_suite_check_uniqueness(void);
extern void run_suite_check_refs(void);
extern void run_suite_validate(void);
extern void run_suite_query(void);
extern void run_suite_list(void);
extern void run_suite_emit(void);

int main(void) {
    fprintf(stderr, "=== yass test suite ===\n\n");

    RUN_SUITE(yass);
    RUN_SUITE(utf8);
    RUN_SUITE(path);
    RUN_SUITE(error);
    RUN_SUITE(argv);
    RUN_SUITE(yaml_parse);
    RUN_SUITE(discover);
    RUN_SUITE(glob);
    RUN_SUITE(check_yaml);
    RUN_SUITE(check_preamble);
    RUN_SUITE(check_spec);
    RUN_SUITE(check_uniqueness);
    RUN_SUITE(check_refs);
    RUN_SUITE(validate);
    RUN_SUITE(query);
    RUN_SUITE(list);
    RUN_SUITE(emit);

    TT_SUMMARY();
}
