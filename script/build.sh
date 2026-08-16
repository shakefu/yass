#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

cmake -S . -B build -G Ninja \
    -DCMAKE_BUILD_TYPE=Debug \
    -DYASS_COVERAGE=ON

cmake --build build
