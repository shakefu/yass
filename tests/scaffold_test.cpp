#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

#include <string>
#include <vector>

#include "support/proc.hpp"

TEST_CASE("subprocess helper works") {
    auto result = yass::test::run({"/bin/echo", "hello"});
    CHECK(result.exit_code == 0);
    CHECK(result.out == "hello\n");
}

TEST_CASE("yass binary runs") {
    // The real main() (M9) dispatches argv. Invoked with no subcommand and no
    // global flag, it reports yass.argv.no_subcommand and exits 2 (cli.Dispatch
    // ERROR / cli.ExitCode), writing nothing to stdout. This supersedes the
    // placeholder main that exited 0. Full argv coverage lives in dispatch_test.
    auto result = yass::test::run({YASS_BIN});
    CHECK(result.exit_code == 2);
    CHECK(result.out.empty());
    CHECK(result.err == "yass: [yass.argv.no_subcommand] no subcommand given\n");
}
