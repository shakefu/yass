"""Tests for yass.errors — error codes, formatting helpers, and emitter."""

from __future__ import annotations

import io
import os

import pytest

from yass import errors
from yass.errors import (
    emit_error,
    format_error_line,
    format_path,
)


# ── Exhaustive error-code value checks ─────────────────────────────────────

# Each tuple: (constant_name_on_module, expected_string_value, expected_exit)
ALL_CODES = [
    # exit
    ("EXIT_SUCCESS", "yass.exit.success", 0),
    ("EXIT_PROCESSING", "yass.exit.processing", 1),
    ("EXIT_USAGE", "yass.exit.usage", 2),
    ("EXIT_SIGINT", "yass.exit.sigint", 130),
    ("EXIT_SIGTERM", "yass.exit.sigterm", 143),
    # argv
    ("ARGV_UNKNOWN_SUBCOMMAND", "yass.argv.unknown_subcommand", 2),
    ("ARGV_NO_SUBCOMMAND", "yass.argv.no_subcommand", 2),
    ("ARGV_UNKNOWN_FLAG", "yass.argv.unknown_flag", 2),
    ("ARGV_EMPTY_ARGUMENT", "yass.argv.empty_argument", 2),
    ("ARGV_SHORT_FLAG", "yass.argv.short_flag", 2),
    ("ARGV_CASE_MISMATCH", "yass.argv.case_mismatch", 2),
    ("ARGV_ABBREVIATION", "yass.argv.abbreviation", 2),
    ("ARGV_MISSING_POSITIONAL", "yass.argv.missing_positional", 2),
    ("ARGV_STDIN_DASH", "yass.argv.stdin_dash", 2),
    # path
    ("PATH_NOT_FOUND", "yass.path.not_found", 2),
    ("PATH_BAD_EXTENSION", "yass.path.bad_extension", 2),
    ("PATH_UNREADABLE", "yass.path.unreadable", 2),
    ("PATH_INVALID_TYPE", "yass.path.invalid_type", 2),
    ("PATH_COLON_IN_PATH", "yass.path.colon_in_path", 2),
    # glob
    ("GLOB_NO_MATCH", "yass.glob.no_match", 2),
    # discover
    ("DISCOVER_NO_FILES", "yass.discover.no_files", 2),
    ("DISCOVER_DIR_UNREADABLE", "yass.discover.dir_unreadable", 1),
    # findroot
    ("FINDROOT_NO_MARKER", "yass.findroot.no_marker", 2),
    # yaml
    ("YAML_NOT_UTF8", "yass.yaml.not_utf8", 1),
    ("YAML_HAS_BOM", "yass.yaml.has_bom", 1),
    ("YAML_MALFORMED", "yass.yaml.malformed", 1),
    ("YAML_EMPTY_FILE", "yass.yaml.empty_file", 1),
    ("YAML_DUPLICATE_KEY", "yass.yaml.duplicate_key", 1),
    ("YAML_ANCHOR_OR_ALIAS", "yass.yaml.anchor_or_alias", 1),
    ("YAML_EMPTY_STREAM", "yass.yaml.empty_stream", 1),
    # preamble
    ("PREAMBLE_HAS_SPEC_KEY", "yass.preamble.has_spec_key", 1),
    ("PREAMBLE_MISSING", "yass.preamble.missing", 1),
    ("PREAMBLE_MISPLACED", "yass.preamble.misplaced", 1),
    ("PREAMBLE_DUPLICATE", "yass.preamble.duplicate", 1),
    ("PREAMBLE_MISSING_DESCRIPTION", "yass.preamble.missing_description", 1),
    ("PREAMBLE_MISSING_VERSION", "yass.preamble.missing_version", 1),
    ("PREAMBLE_UNKNOWN_VERSION", "yass.preamble.unknown_version", 1),
    ("PREAMBLE_BAD_RELATED", "yass.preamble.bad_related", 1),
    # spec
    ("SPEC_NO_NAME", "yass.spec.no_name", 1),
    ("SPEC_NAME_NOT_STRING", "yass.spec.name_not_string", 1),
    ("SPEC_NAME_EMPTY", "yass.spec.name_empty", 1),
    ("SPEC_NAME_BAD_CHARS", "yass.spec.name_bad_chars", 1),
    ("SPEC_NAME_BAD_FORM", "yass.spec.name_bad_form", 1),
    ("SPEC_NAME_RESERVED", "yass.spec.name_reserved", 1),
    ("SPEC_UNKNOWN_KEY", "yass.spec.unknown_key", 1),
    ("SPEC_DUPLICATE_NAME", "yass.spec.duplicate_name", 1),
    # slot
    ("SLOT_VALUE_NOT_LIST", "yass.slot.value_not_list", 1),
    # obligation
    ("OBLIGATION_BAD_VALUE_SHAPE", "yass.obligation.bad_value_shape", 1),
    ("OBLIGATION_MISSING_NORMATIVITY_OR_REF", "yass.obligation.missing_normativity_or_ref", 1),
    ("OBLIGATION_GUARD_WITHOUT_NORMATIVITY", "yass.obligation.guard_without_normativity", 1),
    ("OBLIGATION_DUPLICATE_REFERENCE", "yass.obligation.duplicate_reference", 1),
    ("OBLIGATION_DUPLICATE_NORMATIVITY", "yass.obligation.duplicate_normativity", 1),
    # normativity
    ("NORMATIVITY_UNKNOWN", "yass.normativity.unknown", 1),
    # reference
    ("REFERENCE_UNKNOWN_RELATION", "yass.reference.unknown_relation", 1),
    # ref
    ("REF_MALFORMED", "yass.ref.malformed", 1),
    ("REF_UNKNOWN_SLOT", "yass.ref.unknown_slot", 1),
    ("REF_SLOT_NOT_DECLARED", "yass.ref.slot_not_declared", 1),
    ("REF_SPEC_NOT_FOUND_SAME_FILE", "yass.ref.spec_not_found_same_file", 1),
    ("REF_FILE_NOT_FOUND", "yass.ref.file_not_found", 1),
    ("REF_FILE_NOT_PARSEABLE", "yass.ref.file_not_parseable", 1),
    ("REF_SPEC_NOT_FOUND_OTHER_FILE", "yass.ref.spec_not_found_other_file", 1),
    # query
    ("QUERY_NAME_MISSING", "yass.query.name_missing", 2),
    ("QUERY_NAME_BLANK", "yass.query.name_blank", 2),
    ("QUERY_NO_MATCH", "yass.query.no_match", 1),
    ("QUERY_CONFORMS_UNRESOLVED", "yass.query.conforms_unresolved", 1),
    ("QUERY_CONFORMS_NO_SLOT", "yass.query.conforms_no_slot", 1),
    ("QUERY_SCOPE_NOT_FOUND", "yass.query.scope_not_found", 2),
    ("QUERY_SCOPE_EMPTY", "yass.query.scope_empty", 2),
    # internal
    ("INTERNAL_UNCAUGHT", "yass.internal.uncaught", 1),
]


class TestErrorCodeConstants:
    """Every error-code constant must exist and carry the right string value."""

    @pytest.mark.parametrize("attr,expected_value,_exit", ALL_CODES, ids=[c[0] for c in ALL_CODES])
    def test_constant_value(self, attr: str, expected_value: str, _exit: int) -> None:
        assert hasattr(errors, attr), f"errors module missing constant {attr}"
        assert getattr(errors, attr) == expected_value

    @pytest.mark.parametrize("attr,expected_value,expected_exit", ALL_CODES, ids=[c[0] for c in ALL_CODES])
    def test_exit_code_mapping(self, attr: str, expected_value: str, expected_exit: int) -> None:
        assert expected_value in errors.EXIT_CODE_MAP, (
            f"EXIT_CODE_MAP missing entry for {expected_value}"
        )
        assert errors.EXIT_CODE_MAP[expected_value] == expected_exit


# ── ErrorLine formatting ───────────────────────────────────────────────────

class TestFormatErrorLine:
    """format_error_line produces the right diagnostic string."""

    def test_with_line_number(self) -> None:
        result = format_error_line("foo.yass.yaml", 42, "yass.yaml.malformed", "bad token")
        assert result == "foo.yass.yaml:42: [yass.yaml.malformed] bad token\n"

    def test_without_line_number(self) -> None:
        result = format_error_line("bar.yass.yaml", None, "yass.yaml.empty_file", "empty file")
        assert result == "bar.yass.yaml: [yass.yaml.empty_file] empty file\n"

    def test_message_newline_collapsed(self) -> None:
        result = format_error_line("x.yaml", 1, "yass.yaml.malformed", "line one\nline two")
        assert result == "x.yaml:1: [yass.yaml.malformed] line one line two\n"

    def test_message_multiple_newlines_collapsed(self) -> None:
        result = format_error_line("x.yaml", None, "yass.yaml.malformed", "a\nb\nc")
        assert result == "x.yaml: [yass.yaml.malformed] a b c\n"

    def test_line_zero(self) -> None:
        """Line 0 is unusual but should still render as a number."""
        result = format_error_line("f.yaml", 0, "yass.yaml.malformed", "msg")
        assert result == "f.yaml:0: [yass.yaml.malformed] msg\n"


# ── Path formatting ────────────────────────────────────────────────────────

class TestFormatPath:
    """format_path returns correct relative/absolute results."""

    def test_relative_under_cwd(self) -> None:
        result = format_path("/home/user/project/src/foo.yaml", cwd="/home/user/project")
        assert result == "src/foo.yaml"

    def test_basename_when_directly_inside_cwd(self) -> None:
        result = format_path("/home/user/project/foo.yaml", cwd="/home/user/project")
        assert result == "foo.yaml"

    def test_absolute_when_not_under_cwd(self) -> None:
        result = format_path("/other/place/foo.yaml", cwd="/home/user/project")
        assert result == "/other/place/foo.yaml"

    def test_forward_slashes(self) -> None:
        # Even on platforms where os.sep could be backslash, the output
        # must use forward slashes.  On macOS/Linux this is a no-op but
        # the test still validates the contract.
        result = format_path("/a/b/c/d.yaml", cwd="/a/b")
        assert "\\" not in result
        assert result == "c/d.yaml"

    def test_no_leading_dot_slash(self) -> None:
        result = format_path("/cwd/file.yaml", cwd="/cwd")
        assert not result.startswith("./")
        assert result == "file.yaml"

    def test_relative_input_resolved_against_cwd(self) -> None:
        result = format_path("sub/file.yaml", cwd="/home/user/project")
        assert result == "sub/file.yaml"

    def test_cwd_defaults_to_os_getcwd(self) -> None:
        cwd = os.getcwd()
        abs_file = os.path.join(cwd, "some_file.yaml")
        result = format_path(abs_file)
        assert result == "some_file.yaml"

    def test_cwd_with_trailing_slash(self) -> None:
        result = format_path("/a/b/c.yaml", cwd="/a/b/")
        assert result == "c.yaml"

    def test_cwd_equals_file_parent_deeply_nested(self) -> None:
        result = format_path("/a/b/c/d/e.yaml", cwd="/a/b")
        assert result == "c/d/e.yaml"


# ── emit_error ─────────────────────────────────────────────────────────────

class TestEmitError:
    """emit_error writes formatted output to the given stream."""

    def test_writes_to_stream(self) -> None:
        buf = io.StringIO()
        emit_error("f.yaml", 10, "yass.yaml.malformed", "oops", stream=buf)
        assert buf.getvalue() == "f.yaml:10: [yass.yaml.malformed] oops\n"

    def test_defaults_to_stderr(self) -> None:
        # Just confirm the signature accepts no stream argument (default).
        # We don't actually capture stderr in this test.
        import inspect
        sig = inspect.signature(emit_error)
        assert sig.parameters["stream"].default is not inspect.Parameter.empty
