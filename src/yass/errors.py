"""Machine-stable error codes, formatting helpers, and error emitter."""

from __future__ import annotations

import os
import sys
from typing import Optional, TextIO


# ---------------------------------------------------------------------------
# Error codes — every constant is a dot-delimited string used as [code]
# in diagnostic output.  The associated exit code and default human message
# are documented next to each constant; the constants themselves carry only
# the machine-stable code string.
# ---------------------------------------------------------------------------

# -- exit --------------------------------------------------------------------
EXIT_SUCCESS = "yass.exit.success"                    # exit 0
EXIT_PROCESSING = "yass.exit.processing"              # exit 1
EXIT_USAGE = "yass.exit.usage"                        # exit 2
EXIT_SIGINT = "yass.exit.sigint"                      # exit 130
EXIT_SIGTERM = "yass.exit.sigterm"                    # exit 143

# -- argv --------------------------------------------------------------------
ARGV_UNKNOWN_SUBCOMMAND = "yass.argv.unknown_subcommand"
ARGV_NO_SUBCOMMAND = "yass.argv.no_subcommand"
ARGV_UNKNOWN_FLAG = "yass.argv.unknown_flag"
ARGV_EMPTY_ARGUMENT = "yass.argv.empty_argument"
ARGV_SHORT_FLAG = "yass.argv.short_flag"
ARGV_CASE_MISMATCH = "yass.argv.case_mismatch"
ARGV_ABBREVIATION = "yass.argv.abbreviation"
ARGV_MISSING_POSITIONAL = "yass.argv.missing_positional"
ARGV_STDIN_DASH = "yass.argv.stdin_dash"

# -- path --------------------------------------------------------------------
PATH_NOT_FOUND = "yass.path.not_found"
PATH_BAD_EXTENSION = "yass.path.bad_extension"
PATH_UNREADABLE = "yass.path.unreadable"
PATH_INVALID_TYPE = "yass.path.invalid_type"
PATH_COLON_IN_PATH = "yass.path.colon_in_path"

# -- glob --------------------------------------------------------------------
GLOB_NO_MATCH = "yass.glob.no_match"

# -- discover ----------------------------------------------------------------
DISCOVER_NO_FILES = "yass.discover.no_files"
DISCOVER_DIR_UNREADABLE = "yass.discover.dir_unreadable"

# -- findroot ----------------------------------------------------------------
FINDROOT_NO_MARKER = "yass.findroot.no_marker"

# -- yaml --------------------------------------------------------------------
YAML_NOT_UTF8 = "yass.yaml.not_utf8"
YAML_HAS_BOM = "yass.yaml.has_bom"
YAML_MALFORMED = "yass.yaml.malformed"
YAML_EMPTY_FILE = "yass.yaml.empty_file"
YAML_DUPLICATE_KEY = "yass.yaml.duplicate_key"
YAML_ANCHOR_OR_ALIAS = "yass.yaml.anchor_or_alias"
YAML_EMPTY_STREAM = "yass.yaml.empty_stream"

# -- preamble ----------------------------------------------------------------
PREAMBLE_HAS_SPEC_KEY = "yass.preamble.has_spec_key"
PREAMBLE_MISSING = "yass.preamble.missing"
PREAMBLE_MISPLACED = "yass.preamble.misplaced"
PREAMBLE_DUPLICATE = "yass.preamble.duplicate"
PREAMBLE_MISSING_DESCRIPTION = "yass.preamble.missing_description"
PREAMBLE_MISSING_VERSION = "yass.preamble.missing_version"
PREAMBLE_UNKNOWN_VERSION = "yass.preamble.unknown_version"
PREAMBLE_BAD_RELATED = "yass.preamble.bad_related"

# -- spec --------------------------------------------------------------------
SPEC_NO_NAME = "yass.spec.no_name"
SPEC_NAME_NOT_STRING = "yass.spec.name_not_string"
SPEC_NAME_EMPTY = "yass.spec.name_empty"
SPEC_NAME_BAD_CHARS = "yass.spec.name_bad_chars"
SPEC_NAME_BAD_FORM = "yass.spec.name_bad_form"
SPEC_NAME_RESERVED = "yass.spec.name_reserved"
SPEC_UNKNOWN_KEY = "yass.spec.unknown_key"
SPEC_DUPLICATE_NAME = "yass.spec.duplicate_name"

# -- slot --------------------------------------------------------------------
SLOT_VALUE_NOT_LIST = "yass.slot.value_not_list"

# -- obligation --------------------------------------------------------------
OBLIGATION_BAD_VALUE_SHAPE = "yass.obligation.bad_value_shape"
OBLIGATION_MISSING_NORMATIVITY_OR_REF = "yass.obligation.missing_normativity_or_ref"
OBLIGATION_GUARD_WITHOUT_NORMATIVITY = "yass.obligation.guard_without_normativity"
OBLIGATION_DUPLICATE_REFERENCE = "yass.obligation.duplicate_reference"
OBLIGATION_DUPLICATE_NORMATIVITY = "yass.obligation.duplicate_normativity"

# -- normativity -------------------------------------------------------------
NORMATIVITY_UNKNOWN = "yass.normativity.unknown"

# -- reference ---------------------------------------------------------------
REFERENCE_UNKNOWN_RELATION = "yass.reference.unknown_relation"

# -- ref ---------------------------------------------------------------------
REF_MALFORMED = "yass.ref.malformed"
REF_UNKNOWN_SLOT = "yass.ref.unknown_slot"
REF_SLOT_NOT_DECLARED = "yass.ref.slot_not_declared"
REF_SPEC_NOT_FOUND_SAME_FILE = "yass.ref.spec_not_found_same_file"
REF_FILE_NOT_FOUND = "yass.ref.file_not_found"
REF_FILE_NOT_PARSEABLE = "yass.ref.file_not_parseable"
REF_SPEC_NOT_FOUND_OTHER_FILE = "yass.ref.spec_not_found_other_file"

# -- query -------------------------------------------------------------------
QUERY_NAME_MISSING = "yass.query.name_missing"
QUERY_NAME_BLANK = "yass.query.name_blank"
QUERY_NO_MATCH = "yass.query.no_match"
QUERY_CONFORMS_UNRESOLVED = "yass.query.conforms_unresolved"
QUERY_CONFORMS_NO_SLOT = "yass.query.conforms_no_slot"
QUERY_SCOPE_NOT_FOUND = "yass.query.scope_not_found"
QUERY_SCOPE_EMPTY = "yass.query.scope_empty"

# -- internal ----------------------------------------------------------------
INTERNAL_UNCAUGHT = "yass.internal.uncaught"


# ---------------------------------------------------------------------------
# Canonical mapping: error code -> exit code
# ---------------------------------------------------------------------------
EXIT_CODE_MAP: dict[str, int] = {
    EXIT_SUCCESS: 0,
    EXIT_PROCESSING: 1,
    EXIT_USAGE: 2,
    EXIT_SIGINT: 130,
    EXIT_SIGTERM: 143,
}

# All argv / path / glob / discover / findroot / query-scope codes -> exit 2
for _code in (
    ARGV_UNKNOWN_SUBCOMMAND, ARGV_NO_SUBCOMMAND, ARGV_UNKNOWN_FLAG,
    ARGV_EMPTY_ARGUMENT, ARGV_SHORT_FLAG, ARGV_CASE_MISMATCH,
    ARGV_ABBREVIATION, ARGV_MISSING_POSITIONAL, ARGV_STDIN_DASH,
    PATH_NOT_FOUND, PATH_BAD_EXTENSION, PATH_UNREADABLE,
    PATH_INVALID_TYPE, PATH_COLON_IN_PATH,
    GLOB_NO_MATCH,
    DISCOVER_NO_FILES,
    FINDROOT_NO_MARKER,
    QUERY_NAME_MISSING, QUERY_NAME_BLANK,
    QUERY_SCOPE_NOT_FOUND, QUERY_SCOPE_EMPTY,
):
    EXIT_CODE_MAP[_code] = 2

# All yaml / preamble / spec / slot / obligation / normativity / reference /
# ref / query / internal codes -> exit 1
for _code in (
    YAML_NOT_UTF8, YAML_HAS_BOM, YAML_MALFORMED, YAML_EMPTY_FILE,
    YAML_DUPLICATE_KEY, YAML_ANCHOR_OR_ALIAS, YAML_EMPTY_STREAM,
    PREAMBLE_HAS_SPEC_KEY, PREAMBLE_MISSING, PREAMBLE_MISPLACED,
    PREAMBLE_DUPLICATE, PREAMBLE_MISSING_DESCRIPTION,
    PREAMBLE_MISSING_VERSION, PREAMBLE_UNKNOWN_VERSION, PREAMBLE_BAD_RELATED,
    SPEC_NO_NAME, SPEC_NAME_NOT_STRING, SPEC_NAME_EMPTY,
    SPEC_NAME_BAD_CHARS, SPEC_NAME_BAD_FORM, SPEC_NAME_RESERVED,
    SPEC_UNKNOWN_KEY, SPEC_DUPLICATE_NAME,
    SLOT_VALUE_NOT_LIST,
    OBLIGATION_BAD_VALUE_SHAPE, OBLIGATION_MISSING_NORMATIVITY_OR_REF,
    OBLIGATION_GUARD_WITHOUT_NORMATIVITY, OBLIGATION_DUPLICATE_REFERENCE,
    OBLIGATION_DUPLICATE_NORMATIVITY,
    NORMATIVITY_UNKNOWN,
    REFERENCE_UNKNOWN_RELATION,
    REF_MALFORMED, REF_UNKNOWN_SLOT, REF_SLOT_NOT_DECLARED,
    REF_SPEC_NOT_FOUND_SAME_FILE, REF_FILE_NOT_FOUND,
    REF_FILE_NOT_PARSEABLE, REF_SPEC_NOT_FOUND_OTHER_FILE,
    QUERY_NO_MATCH, QUERY_CONFORMS_UNRESOLVED, QUERY_CONFORMS_NO_SLOT,
    DISCOVER_DIR_UNREADABLE,
    INTERNAL_UNCAUGHT,
):
    EXIT_CODE_MAP[_code] = 1


# ---------------------------------------------------------------------------
# Path formatting
# ---------------------------------------------------------------------------

def format_path(filepath: str, cwd: Optional[str] = None) -> str:
    """Return *filepath* relative to *cwd* when it lives under *cwd*.

    * Emits the path relative to *cwd* when the lexical absolute path of
      *filepath* starts with the lexical absolute *cwd* followed by ``/``.
    * Emits a bare basename when the file is directly inside *cwd*.
    * Emits the absolute path when *filepath* is not under *cwd*.
    * Always uses forward slashes.
    * Does **not** resolve symbolic links (purely lexical).
    """
    if cwd is None:
        cwd = os.getcwd()

    # Make both absolute (lexical only — no realpath).
    abs_file = os.path.normpath(os.path.join(cwd, filepath)) \
        if not os.path.isabs(filepath) else os.path.normpath(filepath)
    abs_cwd = os.path.normpath(cwd)

    # Ensure forward slashes everywhere.
    abs_file = abs_file.replace(os.sep, "/")
    abs_cwd = abs_cwd.replace(os.sep, "/")

    # Check prefix match (cwd + /).
    prefix = abs_cwd if abs_cwd.endswith("/") else abs_cwd + "/"
    if abs_file.startswith(prefix):
        rel = abs_file[len(prefix):]
        return rel.replace(os.sep, "/")

    # Not under cwd -> absolute with forward slashes.
    return abs_file


# ---------------------------------------------------------------------------
# ErrorLine formatting
# ---------------------------------------------------------------------------

def format_error_line(
    file: str,
    line: Optional[int],
    code: str,
    message: str,
) -> str:
    """Build a single diagnostic line.

    Format (with line number)::

        <file>:<line>: [<code>] <message>\\n

    Format (without line number)::

        <file>: [<code>] <message>\\n

    Any newlines inside *message* are collapsed to a single space.
    """
    message = message.replace("\n", " ")
    if line is not None:
        return f"{file}:{line}: [{code}] {message}\n"
    return f"{file}: [{code}] {message}\n"


def emit_error(
    file: str,
    line: Optional[int],
    code: str,
    message: str,
    stream: TextIO = sys.stderr,
) -> None:
    """Write a formatted error line to *stream* (default ``sys.stderr``)."""
    stream.write(format_error_line(file, line, code, message))
