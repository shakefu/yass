"""CLI dispatch for the yass command-line tool."""

from __future__ import annotations

import os
import signal
import sys
from pathlib import Path
from typing import TextIO


# ---------------------------------------------------------------------------
# Version
# ---------------------------------------------------------------------------

def _read_version() -> str:
    """Read the version string from the VERSION file."""
    version_file = Path(__file__).resolve().parent.parent.parent / "VERSION"
    try:
        return version_file.read_text(encoding="utf-8").strip()
    except FileNotFoundError:
        # Fallback to importlib.metadata
        from importlib.metadata import version
        return version("yass-cli")


# ---------------------------------------------------------------------------
# Usage text
# ---------------------------------------------------------------------------

_USAGE = """\
usage: yass <command> [<args>]

commands:
  validate   Validate .yass.yaml files
  query      Query a spec by name
  list       List specs in scope

global flags:
  --help     Show this help message
  --version  Show the yass version
"""


# ---------------------------------------------------------------------------
# Known tokens for case / abbreviation checks
# ---------------------------------------------------------------------------

_KNOWN_SUBCOMMANDS = frozenset({"validate", "query", "list"})
_KNOWN_FLAGS = frozenset({"--help", "--version"})
_KNOWN_TOKENS = _KNOWN_SUBCOMMANDS | _KNOWN_FLAGS | {"--"}


# ---------------------------------------------------------------------------
# Dispatch internals
# ---------------------------------------------------------------------------

def _emit_cli_error(code: str, message: str, stderr: TextIO) -> None:
    """Write a single error line in ErrorLine format for argv-level errors.

    Uses literal ``yass`` as the <file> since there is no input file.
    """
    from yass.errors import format_error_line
    stderr.write(format_error_line("yass", None, code, message))


def _check_case_mismatch(token: str) -> str | None:
    """Return the canonical form if *token* matches a known token
    case-insensitively but not case-sensitively, else None."""
    lower = token.lower()
    for known in _KNOWN_TOKENS:
        if token != known and lower == known.lower():
            return known
    return None


def _check_abbreviation(token: str) -> str | None:
    """Return the canonical form if *token* is a strict prefix of exactly one
    known token, else None."""
    if not token:
        return None
    # Don't match if it IS a known token
    if token in _KNOWN_TOKENS:
        return None
    candidates = [k for k in _KNOWN_TOKENS if k.startswith(token) and k != token]
    if len(candidates) == 1:
        return candidates[0]
    return None


def _dispatch(argv: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """Parse argv and dispatch to the appropriate subcommand.

    Returns the exit code.
    """
    from yass.errors import (
        ARGV_ABBREVIATION,
        ARGV_CASE_MISMATCH,
        ARGV_EMPTY_ARGUMENT,
        ARGV_NO_SUBCOMMAND,
        ARGV_SHORT_FLAG,
        ARGV_STDIN_DASH,
        ARGV_UNKNOWN_FLAG,
        ARGV_UNKNOWN_SUBCOMMAND,
        INTERNAL_UNCAUGHT,
    )

    # -- Global flags: check anywhere in argv ---------------------------------
    if "--help" in argv:
        stdout.write(_USAGE)
        return 0

    if "--version" in argv:
        ver = _read_version()
        stdout.write(f"yass {ver}\n")
        return 0

    # -- Validate all args before dispatching ---------------------------------
    past_separator = False
    positionals: list[str] = []

    for arg in argv:
        # Empty string argument
        if arg == "":
            _emit_cli_error(
                ARGV_EMPTY_ARGUMENT,
                "empty argument",
                stderr,
            )
            return 2

        # End-of-options marker
        if arg == "--" and not past_separator:
            past_separator = True
            continue

        # Bare dash (stdin)
        if arg == "-":
            _emit_cli_error(
                ARGV_STDIN_DASH,
                "stdin ('-') is not supported",
                stderr,
            )
            return 2

        # Check for flags (starts with -)
        if arg.startswith("-") and not past_separator:
            if not arg.startswith("--"):
                # Short flag
                _emit_cli_error(
                    ARGV_SHORT_FLAG,
                    f"short flags are not supported: {arg}",
                    stderr,
                )
                return 2

            # Double-dash flag
            # Case mismatch check
            canon = _check_case_mismatch(arg)
            if canon is not None:
                _emit_cli_error(
                    ARGV_CASE_MISMATCH,
                    f"did you mean '{canon}'? (flags are case-sensitive)",
                    stderr,
                )
                return 2

            # Abbreviation check
            canon = _check_abbreviation(arg)
            if canon is not None:
                _emit_cli_error(
                    ARGV_ABBREVIATION,
                    f"did you mean '{canon}'? (flag abbreviations are not supported)",
                    stderr,
                )
                return 2

            # Unknown flag
            if arg not in _KNOWN_FLAGS:
                _emit_cli_error(
                    ARGV_UNKNOWN_FLAG,
                    f"unknown flag: {arg}",
                    stderr,
                )
                return 2

            continue

        # Positional argument
        # Case mismatch check for first positional (subcommand)
        if not positionals:
            canon = _check_case_mismatch(arg)
            if canon is not None:
                _emit_cli_error(
                    ARGV_CASE_MISMATCH,
                    f"did you mean '{canon}'? (subcommands are case-sensitive)",
                    stderr,
                )
                return 2

            # Abbreviation check for subcommand
            canon = _check_abbreviation(arg)
            if canon is not None:
                _emit_cli_error(
                    ARGV_ABBREVIATION,
                    f"did you mean '{canon}'? (subcommand abbreviations are not supported)",
                    stderr,
                )
                return 2

        positionals.append(arg)

    # -- Determine subcommand -------------------------------------------------
    if not positionals:
        _emit_cli_error(
            ARGV_NO_SUBCOMMAND,
            "no subcommand provided; see 'yass --help'",
            stderr,
        )
        return 2

    subcmd = positionals[0]
    remaining = positionals[1:]

    if subcmd == "validate":
        from yass.validate import validate_command
        try:
            return validate_command(remaining, stderr=stderr, stdout=stdout)
        except Exception as exc:
            msg = str(exc).replace("\n", " ")
            _emit_cli_error(INTERNAL_UNCAUGHT, f"internal error: {msg}", stderr)
            return 1

    if subcmd == "query":
        from yass.query import query_command
        try:
            return query_command(remaining, stdout=stdout, stderr=stderr)
        except Exception as exc:
            msg = str(exc).replace("\n", " ")
            _emit_cli_error(INTERNAL_UNCAUGHT, f"internal error: {msg}", stderr)
            return 1

    if subcmd == "list":
        from yass.list_cmd import list_command
        try:
            return list_command(remaining, stdout=stdout, stderr=stderr)
        except Exception as exc:
            msg = str(exc).replace("\n", " ")
            _emit_cli_error(INTERNAL_UNCAUGHT, f"internal error: {msg}", stderr)
            return 1

    # Unknown subcommand
    _emit_cli_error(
        ARGV_UNKNOWN_SUBCOMMAND,
        f"unknown subcommand: {subcmd}",
        stderr,
    )
    return 2


# ---------------------------------------------------------------------------
# Signal handlers
# ---------------------------------------------------------------------------

def _handle_sigpipe(signum: int, frame: object) -> None:
    """Handle SIGPIPE: exit 0 cleanly without writing further output."""
    # Close stdout/stderr to avoid further write errors
    try:
        sys.stdout.close()
    except Exception:
        pass
    try:
        sys.stderr.close()
    except Exception:
        pass
    os._exit(0)


def _handle_sigint(signum: int, frame: object) -> None:
    """Handle SIGINT: flush pending error lines and exit 130."""
    try:
        sys.stderr.flush()
    except Exception:
        pass
    os._exit(130)


def _handle_sigterm(signum: int, frame: object) -> None:
    """Handle SIGTERM: flush pending error lines and exit 143."""
    try:
        sys.stderr.flush()
    except Exception:
        pass
    os._exit(143)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main(argv: list[str] | None = None) -> None:
    """Top-level entry point for the yass CLI.

    Reads *argv* (or ``sys.argv[1:]`` when *argv* is ``None``), sets up
    signal handlers, parses global flags and the subcommand, dispatches to
    the correct handler, and calls ``sys.exit()`` with the result.
    """
    if argv is None:
        argv = sys.argv[1:]

    # -- Signal handlers ------------------------------------------------------
    # SIGPIPE: only available on POSIX
    if hasattr(signal, "SIGPIPE"):
        signal.signal(signal.SIGPIPE, _handle_sigpipe)

    signal.signal(signal.SIGINT, _handle_sigint)
    signal.signal(signal.SIGTERM, _handle_sigterm)

    # -- Line-buffer stdout ---------------------------------------------------
    # Ensure stdout is line-buffered for UTF-8 output
    if hasattr(sys.stdout, "reconfigure"):
        try:
            sys.stdout.reconfigure(line_buffering=True)
        except Exception:
            pass

    # -- Dispatch -------------------------------------------------------------
    code = _dispatch(argv, sys.stdout, sys.stderr)
    sys.exit(code)
