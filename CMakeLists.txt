cmake_minimum_required(VERSION 3.24)

project(yass CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

option(YASS_COVERAGE "Enable source-based coverage instrumentation" ON)

# ---------------------------------------------------------------------------
# Tool version (cli.Dispatch SIDE-EFFECT: `yass --version` prints
# `yass <version>\n` where <version> is the VERSION file contents with no `v`
# prefix and no build metadata). Read at configure time and baked in as the
# YASS_VERSION compile definition. CONFIGURE_DEPENDS via the file(READ) is not
# available; re-run CMake (the build script does) when VERSION changes.
# ---------------------------------------------------------------------------
file(READ ${CMAKE_SOURCE_DIR}/VERSION YASS_VERSION_RAW)
string(STRIP "${YASS_VERSION_RAW}" YASS_VERSION)
string(REGEX REPLACE "^v" "" YASS_VERSION "${YASS_VERSION}")

# INTERFACE target carrying coverage flags. Linked only into instrumented
# targets (yasslib, yass, yass_tests). Third-party code (ryml) is excluded so
# it does not skew coverage numbers.
add_library(yass_coverage INTERFACE)
if(YASS_COVERAGE)
    target_compile_options(yass_coverage INTERFACE
        -fprofile-instr-generate -fcoverage-mapping -O0 -g)
    target_link_options(yass_coverage INTERFACE
        -fprofile-instr-generate -fcoverage-mapping)
endif()

# ---------------------------------------------------------------------------
# ryml: third-party rapidyaml single-header implementation TU.
# Compiled WITHOUT coverage instrumentation and with warnings silenced.
# ---------------------------------------------------------------------------
add_library(ryml STATIC src/ryml_impl.cpp)
target_include_directories(ryml PUBLIC ${CMAKE_SOURCE_DIR}/third_party/rapidyaml)
target_compile_options(ryml PRIVATE -w)

# ---------------------------------------------------------------------------
# yasslib: project library (everything in src/ except main.cpp & ryml_impl.cpp)
# ---------------------------------------------------------------------------
file(GLOB YASSLIB_SOURCES CONFIGURE_DEPENDS ${CMAKE_SOURCE_DIR}/src/*.cpp)
list(REMOVE_ITEM YASSLIB_SOURCES
    ${CMAKE_SOURCE_DIR}/src/main.cpp
    ${CMAKE_SOURCE_DIR}/src/ryml_impl.cpp)

add_library(yasslib STATIC ${YASSLIB_SOURCES})
target_include_directories(yasslib PUBLIC
    ${CMAKE_SOURCE_DIR}/src
    ${CMAKE_SOURCE_DIR}/third_party/rapidyaml
    ${CMAKE_SOURCE_DIR}/third_party/doctest)
target_link_libraries(yasslib PUBLIC ryml)
target_compile_options(yasslib PRIVATE -Wall -Wextra)
target_link_libraries(yasslib PRIVATE yass_coverage)
# YASS_VERSION is consumed by src/dispatch.cpp (compiled into yasslib) to print
# `yass <version>`. PUBLIC so any target linking yasslib (yass, yass_tests) also
# sees the macro if a test references it directly.
target_compile_definitions(yasslib PUBLIC YASS_VERSION="${YASS_VERSION}")

# ---------------------------------------------------------------------------
# yass: CLI executable
# ---------------------------------------------------------------------------
add_executable(yass src/main.cpp)
target_link_libraries(yass PRIVATE yasslib yass_coverage)

# ---------------------------------------------------------------------------
# Reference (canonical) binary for differential tests.
#
# Resolved ONCE at configure time to an ABSOLUTE path and baked into the test
# runner as the YASS_REF_BIN compile definition. This pins the oracle so the
# differential suite is reproducible and does not depend on execv/PATH quirks
# (the test harness no longer searches PATH or probes hardcoded fallbacks at
# runtime). Resolution priority:
#   1. The YASS_REF cache/env variable, if set (override).
#   2. /tmp/yass, if it exists AND runs (`/tmp/yass --version` exits 0).
#   3. find_program(yass) on PATH.
# If none resolves, YASS_REF_BIN is the empty string and the differential cases
# SKIP gracefully (doctest MESSAGE) rather than failing or using a stale path.
# ---------------------------------------------------------------------------
set(YASS_REF_BIN_RESOLVED "")
if(DEFINED ENV{YASS_REF} AND NOT "$ENV{YASS_REF}" STREQUAL "")
    # Explicit override. Normalize to an absolute path.
    get_filename_component(YASS_REF_BIN_RESOLVED "$ENV{YASS_REF}" ABSOLUTE)
    message(STATUS "yass reference binary (YASS_REF override): ${YASS_REF_BIN_RESOLVED}")
elseif(DEFINED YASS_REF AND NOT "${YASS_REF}" STREQUAL "")
    get_filename_component(YASS_REF_BIN_RESOLVED "${YASS_REF}" ABSOLUTE)
    message(STATUS "yass reference binary (YASS_REF override): ${YASS_REF_BIN_RESOLVED}")
else()
    # Probe /tmp/yass: must exist AND actually run.
    if(EXISTS "/tmp/yass")
        execute_process(
            COMMAND /tmp/yass --version
            RESULT_VARIABLE _yass_ref_rc
            OUTPUT_QUIET ERROR_QUIET)
        if(_yass_ref_rc EQUAL 0)
            set(YASS_REF_BIN_RESOLVED "/tmp/yass")
            message(STATUS "yass reference binary (/tmp/yass): ${YASS_REF_BIN_RESOLVED}")
        endif()
    endif()
    # Fall back to PATH.
    if(YASS_REF_BIN_RESOLVED STREQUAL "")
        find_program(YASS_REF_ON_PATH yass)
        if(YASS_REF_ON_PATH)
            set(YASS_REF_BIN_RESOLVED "${YASS_REF_ON_PATH}")
            message(STATUS "yass reference binary (PATH): ${YASS_REF_BIN_RESOLVED}")
        endif()
    endif()
endif()
if(YASS_REF_BIN_RESOLVED STREQUAL "")
    message(WARNING "No yass reference binary resolved; differential tests will SKIP.")
endif()

# ---------------------------------------------------------------------------
# yass_tests: doctest-based test runner
# ---------------------------------------------------------------------------
file(GLOB YASS_TEST_SOURCES CONFIGURE_DEPENDS ${CMAKE_SOURCE_DIR}/tests/*.cpp)

add_executable(yass_tests ${YASS_TEST_SOURCES})
target_include_directories(yass_tests PRIVATE
    ${CMAKE_SOURCE_DIR}/tests
    ${CMAKE_SOURCE_DIR}/src
    ${CMAKE_SOURCE_DIR}/third_party/doctest)
target_link_libraries(yass_tests PRIVATE yasslib yass_coverage)
target_compile_definitions(yass_tests PRIVATE
    YASS_BIN="$<TARGET_FILE:yass>"
    YASS_REF_BIN="${YASS_REF_BIN_RESOLVED}")

# Ensure the binary under test is built before the tests run.
add_dependencies(yass_tests yass)

enable_testing()
add_test(NAME yass_tests COMMAND yass_tests)
