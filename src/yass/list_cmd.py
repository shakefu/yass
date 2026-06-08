"""Implementation of the ``yass list`` subcommand."""

from __future__ import annotations

import os
import re
import sys
import unicodedata
from typing import TextIO

import wcwidth

from yass.errors import (
    PATH_COLON_IN_PATH,
    YAML_MALFORMED,
    emit_error,
    format_error_line,
    format_path,
)
from yass.parser import parse_yaml_file
from yass.shared import (
    DiscoverSpecFiles,
    FindProjectRoot,
    SharedError,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _nfc(s: str) -> str:
    """Return NFC-normalized form of *s*."""
    return unicodedata.normalize("NFC", s)


def _collapse_whitespace(s: str) -> str:
    """Replace every run of whitespace with a single space and strip."""
    return re.sub(r"\s+", " ", s).strip()


def _display_width(s: str) -> int:
    """Return the display width of *s* using wcswidth.

    Falls back to ``len(s)`` when wcswidth returns -1 (unprintable chars).
    """
    if not s:
        return 0
    w = wcwidth.wcswidth(s)
    return w if w >= 0 else len(s)


def _terminal_width(stdout: TextIO) -> int:
    """Determine terminal width: COLUMNS env, os.get_terminal_size(), or 80."""
    # 1. COLUMNS env var
    cols = os.environ.get("COLUMNS")
    if cols is not None:
        try:
            val = int(cols)
            if val > 0:
                return val
        except ValueError:
            pass

    # 2. os.get_terminal_size()
    try:
        return os.get_terminal_size().columns
    except (OSError, ValueError):
        pass

    # 3. Default
    return 80


def _truncate_description(
    filepath_display: str,
    spec_name: str,
    description: str,
    width: int,
) -> str:
    """Truncate *description* for TTY output within *width* columns.

    Returns the (possibly truncated) description string.
    """
    if not description:
        return ""

    marker = "..."
    fp_w = _display_width(filepath_display)
    name_w = _display_width(spec_name)
    # 2 tab separators
    prefix_w = fp_w + 1 + name_w + 1
    marker_w = _display_width(marker)

    # If file+name+separators+marker >= width: emit empty third field
    if prefix_w + marker_w >= width:
        return ""

    available = width - prefix_w
    desc_w = _display_width(description)

    # Description fits entirely
    if desc_w <= available:
        return description

    # Need to truncate on grapheme-cluster boundary
    # We need to find the longest prefix of description that fits in
    # (available - marker_w) display columns, then append marker.
    budget = available - marker_w
    truncated = ""
    current_w = 0
    for ch in description:
        ch_w = wcwidth.wcwidth(ch)
        if ch_w < 0:
            ch_w = 1
        if current_w + ch_w > budget:
            break
        truncated += ch
        current_w += ch_w

    return truncated + marker


# ---------------------------------------------------------------------------
# Core: extract specs from parsed documents
# ---------------------------------------------------------------------------

def _extract_specs(filepath: str, documents):
    """Yield (spec_name, description) for each Spec document in the file.

    A Spec document is a YAML mapping that has a ``spec`` key.
    The preamble is the first document (if it lacks a ``spec`` key).
    Description comes from the preamble's ``description`` field.
    """
    if not documents:
        return

    preamble_desc = ""
    start = 0

    # Check if first document is a preamble (no "spec" key)
    first = documents[0]
    if isinstance(first.data, dict) and "spec" not in first.data:
        raw_desc = first.data.get("description", "")
        if isinstance(raw_desc, str) and raw_desc:
            preamble_desc = _nfc(_collapse_whitespace(raw_desc))
        start = 1

    for doc in documents[start:]:
        if isinstance(doc.data, dict) and "spec" in doc.data:
            spec_name = doc.data["spec"]
            if isinstance(spec_name, str):
                yield (spec_name, preamble_desc)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def list_command(
    args: list[str],
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Execute ``yass list [path]``.

    Returns the exit code (0 on success, 1 on parse failures, 2 on usage errors).
    """
    cwd = os.getcwd()

    # -- Parse positional argument --------------------------------------------
    path = None
    if args:
        path = args[0]

    # -- Colon check ----------------------------------------------------------
    if path is not None and ":" in path:
        stderr.write(format_error_line(
            path, None, PATH_COLON_IN_PATH,
            f"path contains a colon: {path}",
        ))
        return 2

    # -- Discover files -------------------------------------------------------
    try:
        if path is None:
            project_root = FindProjectRoot(cwd)
            files = DiscoverSpecFiles(path=None, project_root=project_root)
        else:
            # Treat "-" as a literal path
            files = DiscoverSpecFiles(path=path)
    except SharedError as exc:
        stderr.write(format_error_line(
            path or cwd, None, exc.code, exc.message,
        ))
        return exc.exit_code

    if not files:
        return 0

    # -- Sort files by NFC-normalized Unicode code-point order ----------------
    files = sorted(files, key=lambda p: _nfc(p))

    # -- Determine TTY mode ---------------------------------------------------
    is_tty = hasattr(stdout, "isatty") and stdout.isatty()
    term_width = _terminal_width(stdout) if is_tty else 0

    # -- Process each file ----------------------------------------------------
    had_parse_error = False
    rows: list[str] = []

    for filepath in files:
        # Resolve to absolute for parsing
        abs_filepath = os.path.normpath(os.path.join(cwd, filepath)) \
            if not os.path.isabs(filepath) else filepath

        documents, errors = parse_yaml_file(abs_filepath)

        if errors:
            had_parse_error = True
            for err in errors:
                emit_error(
                    format_path(abs_filepath, cwd=cwd),
                    err.line,
                    err.code,
                    err.message,
                    stream=stderr,
                )
            # Continue with other files — do not emit rows for this file
            continue

        # Replace tabs in filepath display with spaces
        filepath_display = filepath.replace("\t", " ")

        for spec_name, description in _extract_specs(filepath, documents):
            if is_tty:
                desc_out = _truncate_description(
                    filepath_display, spec_name, description, term_width,
                )
            else:
                desc_out = description

            row = f"{filepath_display}\t{spec_name}\t{desc_out}\n"
            rows.append(row)

    # -- Emit output ----------------------------------------------------------
    for row in rows:
        stdout.write(row)

    return 1 if had_parse_error else 0
