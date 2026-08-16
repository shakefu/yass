cmake_minimum_required(VERSION 3.16)
project(yass C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

# Read version from VERSION file
file(READ "${CMAKE_SOURCE_DIR}/VERSION" YASS_VERSION)
string(STRIP "${YASS_VERSION}" YASS_VERSION)
add_compile_definitions(YASS_VERSION="${YASS_VERSION}")

# Compiler warnings
add_compile_options(-Wall -Wextra -Wpedantic -Werror)

# Find libyaml
find_package(PkgConfig REQUIRED)
pkg_check_modules(YAML REQUIRED yaml-0.1)

# Source files (everything except main.c for test linking)
set(YASS_LIB_SOURCES
    src/argv.c
    src/error.c
    src/path.c
    src/utf8.c
    src/discover.c
    src/glob.c
    src/yaml_parse.c
    src/yass.c
    src/validate.c
    src/check_yaml.c
    src/check_preamble.c
    src/check_spec.c
    src/check_uniqueness.c
    src/check_refs.c
    src/query.c
    src/list.c
    src/emit.c
)

# Static library for linking into both main and tests
add_library(yass_lib STATIC ${YASS_LIB_SOURCES})
target_include_directories(yass_lib PUBLIC src ${YAML_INCLUDE_DIRS})
target_link_directories(yass_lib PUBLIC ${YAML_LIBRARY_DIRS})
target_link_libraries(yass_lib PUBLIC ${YAML_LIBRARIES})

# Main executable
add_executable(yass src/main.c)
target_link_libraries(yass PRIVATE yass_lib)

# Test executable
enable_testing()

file(GLOB TEST_SOURCES test/test_*.c)
add_executable(yass_test test/test_main.c ${TEST_SOURCES})
target_link_libraries(yass_test PRIVATE yass_lib)
target_include_directories(yass_test PRIVATE src)

add_test(NAME yass_tests COMMAND yass_test)

# Coverage target
option(COVERAGE "Enable code coverage" OFF)
if(COVERAGE)
    target_compile_options(yass_lib PRIVATE --coverage -fprofile-arcs -ftest-coverage)
    target_link_options(yass_lib PRIVATE --coverage)
    target_compile_options(yass_test PRIVATE --coverage -fprofile-arcs -ftest-coverage)
    target_link_options(yass_test PRIVATE --coverage)
endif()

# Install
install(TARGETS yass DESTINATION bin)
