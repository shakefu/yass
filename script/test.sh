#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CHECK=0
if [[ "${1:-}" == "--check" ]]; then
    CHECK=1
    shift
fi

./script/build.sh

rm -rf build/prof
mkdir -p build/prof
export LLVM_PROFILE_FILE="$PWD/build/prof/%p-%m.profraw"

# Run the test binary, capturing its exit code without letting `set -e` abort
# before we have produced the coverage report.
test_rc=0
./build/yass_tests "$@" || test_rc=$?

# Merge raw profiles.
xcrun llvm-profdata merge -sparse build/prof/*.profraw -o build/coverage.profdata

# Human-readable coverage report.
xcrun llvm-cov report \
    ./build/yass_tests \
    -object ./build/yass \
    -instr-profile=build/coverage.profdata \
    -ignore-filename-regex='(third_party|/tests/|ryml_impl)' \
    src

if [[ "$CHECK" -eq 1 ]]; then
    pct=$(xcrun llvm-cov export \
        ./build/yass_tests \
        -object ./build/yass \
        -instr-profile=build/coverage.profdata \
        -ignore-filename-regex='(third_party|/tests/|ryml_impl)' \
        -summary-only \
        src \
        | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"][0]["totals"]["lines"]["percent"])')

    printf 'Total line coverage: %s%%\n' "$pct"

    gate_fail=$(python3 -c "import sys; sys.exit(0 if float('$pct') < 80.0 else 1)" && echo 1 || echo 0)
    if [[ "$gate_fail" -eq 1 ]]; then
        printf 'Coverage gate FAILED: %s%% < 80.0%%\n' "$pct" >&2
        exit 1
    fi
    if [[ "$test_rc" -ne 0 ]]; then
        exit "$test_rc"
    fi
    printf 'Coverage gate PASSED: %s%% >= 80.0%%\n' "$pct"
    exit 0
fi

exit "$test_rc"
