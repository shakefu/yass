/*
 * test_argv.c - Tests for argv parsing per cli.Dispatch spec
 */
#include "tinytest.h"
#include "../src/argv.h"
#include "../src/error.h"
#include <string.h>

/* --- No arguments -> no_subcommand error --- */

TEST(argv_no_args) {
    yass_args_t args;
    char *av[] = {"yass"};
    int rc = argv_parse(1, av, &args);
    ASSERT_EQ(rc, 2);
    ASSERT_EQ(args.command, CMD_NONE);
}

/* --- --help -> CMD_HELP --- */

TEST(argv_help_flag) {
    yass_args_t args;
    char *av[] = {"yass", "--help"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_HELP);
    ASSERT_TRUE(args.help);
}

/* --- --version -> CMD_VERSION --- */

TEST(argv_version_flag) {
    yass_args_t args;
    char *av[] = {"yass", "--version"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_VERSION);
    ASSERT_TRUE(args.version);
}

/* --- --help with subcommand -> CMD_HELP (help takes priority) --- */

TEST(argv_help_with_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "validate", "--help"};
    int rc = argv_parse(3, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_HELP);
    ASSERT_TRUE(args.help);
}

TEST(argv_help_before_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "--help", "validate"};
    int rc = argv_parse(3, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_HELP);
    ASSERT_TRUE(args.help);
}

/* --- --version with subcommand -> CMD_VERSION --- */

TEST(argv_version_with_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "validate", "--version"};
    int rc = argv_parse(3, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_VERSION);
    ASSERT_TRUE(args.version);
}

/* --- validate -> CMD_VALIDATE --- */

TEST(argv_validate) {
    yass_args_t args;
    char *av[] = {"yass", "validate"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_VALIDATE);
    ASSERT_EQ(args.positional_count, 0);
}

/* --- query -> CMD_QUERY --- */

TEST(argv_query) {
    yass_args_t args;
    char *av[] = {"yass", "query"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_QUERY);
    ASSERT_EQ(args.positional_count, 0);
}

/* --- list -> CMD_LIST --- */

TEST(argv_list) {
    yass_args_t args;
    char *av[] = {"yass", "list"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_LIST);
    ASSERT_EQ(args.positional_count, 0);
}

/* --- validate with positionals -> positionals captured --- */

TEST(argv_validate_with_positionals) {
    yass_args_t args;
    char *av[] = {"yass", "validate", "foo.yaml", "bar.yaml"};
    int rc = argv_parse(4, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_VALIDATE);
    ASSERT_EQ(args.positional_count, 2);
    ASSERT_NOT_NULL(args.positionals);
    ASSERT_STR_EQ(args.positionals[0], "foo.yaml");
    ASSERT_STR_EQ(args.positionals[1], "bar.yaml");
}

/* --- Unknown subcommand -> error --- */

TEST(argv_unknown_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "foobar"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Short flag -h -> error --- */

TEST(argv_short_flag) {
    yass_args_t args;
    char *av[] = {"yass", "-h"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

TEST(argv_short_flag_v) {
    yass_args_t args;
    char *av[] = {"yass", "-v"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Case mismatch Validate -> error --- */

TEST(argv_case_mismatch_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "Validate"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

TEST(argv_case_mismatch_flag) {
    yass_args_t args;
    char *av[] = {"yass", "--Help"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Abbreviation val -> error --- */

TEST(argv_abbreviation_subcommand) {
    yass_args_t args;
    char *av[] = {"yass", "val"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

TEST(argv_abbreviation_flag) {
    yass_args_t args;
    char *av[] = {"yass", "--ver"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Unknown flag --foo -> error --- */

TEST(argv_unknown_flag) {
    yass_args_t args;
    char *av[] = {"yass", "--foo"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Empty argument -> error --- */

TEST(argv_empty_argument) {
    yass_args_t args;
    char *av[] = {"yass", ""};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- Bare dash -> error --- */

TEST(argv_bare_dash) {
    yass_args_t args;
    char *av[] = {"yass", "-"};
    int rc = argv_parse(2, av, &args);
    ASSERT_EQ(rc, 2);
}

/* --- -- end of options marker --- */

TEST(argv_end_of_options) {
    yass_args_t args;
    char *av[] = {"yass", "validate", "--", "--foo", "-h"};
    int rc = argv_parse(5, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_VALIDATE);
    ASSERT_EQ(args.positional_count, 2);
    ASSERT_NOT_NULL(args.positionals);
    ASSERT_STR_EQ(args.positionals[0], "--foo");
    ASSERT_STR_EQ(args.positionals[1], "-h");
}

/* --- --help --version -> CMD_HELP (help priority) --- */

TEST(argv_help_version_priority) {
    yass_args_t args;
    char *av[] = {"yass", "--help", "--version"};
    int rc = argv_parse(3, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_HELP);
    ASSERT_TRUE(args.help);
}

TEST(argv_version_help_priority) {
    yass_args_t args;
    char *av[] = {"yass", "--version", "--help"};
    int rc = argv_parse(3, av, &args);
    ASSERT_EQ(rc, 0);
    ASSERT_EQ(args.command, CMD_HELP);
    ASSERT_TRUE(args.help);
}

/* --- Suite runner --- */

void run_suite_argv(void) {
    RUN_TEST(argv_no_args);
    RUN_TEST(argv_help_flag);
    RUN_TEST(argv_version_flag);
    RUN_TEST(argv_help_with_subcommand);
    RUN_TEST(argv_help_before_subcommand);
    RUN_TEST(argv_version_with_subcommand);
    RUN_TEST(argv_validate);
    RUN_TEST(argv_query);
    RUN_TEST(argv_list);
    RUN_TEST(argv_validate_with_positionals);
    RUN_TEST(argv_unknown_subcommand);
    RUN_TEST(argv_short_flag);
    RUN_TEST(argv_short_flag_v);
    RUN_TEST(argv_case_mismatch_subcommand);
    RUN_TEST(argv_case_mismatch_flag);
    RUN_TEST(argv_abbreviation_subcommand);
    RUN_TEST(argv_abbreviation_flag);
    RUN_TEST(argv_unknown_flag);
    RUN_TEST(argv_empty_argument);
    RUN_TEST(argv_bare_dash);
    RUN_TEST(argv_end_of_options);
    RUN_TEST(argv_help_version_priority);
    RUN_TEST(argv_version_help_priority);
}
