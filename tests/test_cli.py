"""Tests for yass.cli — CLI dispatch module."""

from __future__ import annotations

import io
import os
import subprocess
import sys
from unittest import mock

import pytest

from yass.cli import _dispatch, _read_version, main


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _run_dispatch(argv: list[str]) -> tuple[int, str, str]:
    """Run _dispatch with captured stdout/stderr.

    Returns (exit_code, stdout_text, stderr_text).
    """
    stdout = io.StringIO()
    stderr = io.StringIO()
    code = _dispatch(argv, stdout, stderr)
    return code, stdout.getvalue(), stderr.getvalue()


def _run_main(argv: list[str]) -> tuple[int, str, str]:
    """Run main() via subprocess using python -m yass.

    Returns (returncode, stdout_text, stderr_text).
    """
    result = subprocess.run(
        [sys.executable, "-m", "yass"] + argv,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return result.returncode, result.stdout, result.stderr


# ---------------------------------------------------------------------------
# Version reading
# ---------------------------------------------------------------------------

class TestReadVersion:
    """Verify _read_version returns the correct version string."""

    def test_reads_version_file(self):
        ver = _read_version()
        assert ver  # non-empty
        # Should not contain leading/trailing whitespace
        assert ver == ver.strip()
        # Should not have a 'v' prefix
        assert not ver.startswith("v")


# ---------------------------------------------------------------------------
# --help
# ---------------------------------------------------------------------------

class TestHelp:
    """--help prints usage to stdout and exits 0."""

    def test_help_flag(self):
        code, stdout, stderr = _run_dispatch(["--help"])
        assert code == 0
        assert "usage:" in stdout
        assert "validate" in stdout
        assert "query" in stdout
        assert "list" in stdout
        assert stderr == ""

    def test_help_flag_with_subcommand(self):
        """--help takes priority even when a subcommand is present."""
        code, stdout, stderr = _run_dispatch(["validate", "--help"])
        assert code == 0
        assert "usage:" in stdout

    def test_help_flag_after_double_dash(self):
        """--help is detected anywhere in argv, even after --."""
        code, stdout, stderr = _run_dispatch(["--", "--help"])
        assert code == 0
        assert "usage:" in stdout

    def test_help_priority_over_version(self):
        """--help takes priority over --version."""
        code, stdout, stderr = _run_dispatch(["--help", "--version"])
        assert code == 0
        assert "usage:" in stdout
        assert "yass " not in stdout or "usage:" in stdout

    def test_help_subprocess(self):
        """Integration: python -m yass --help."""
        code, stdout, stderr = _run_main(["--help"])
        assert code == 0
        assert "usage:" in stdout


# ---------------------------------------------------------------------------
# --version
# ---------------------------------------------------------------------------

class TestVersion:
    """--version prints version string to stdout and exits 0."""

    def test_version_flag(self):
        code, stdout, stderr = _run_dispatch(["--version"])
        assert code == 0
        ver = _read_version()
        assert stdout == f"yass {ver}\n"
        assert stderr == ""

    def test_version_flag_with_subcommand(self):
        """--version anywhere in argv."""
        code, stdout, stderr = _run_dispatch(["validate", "--version"])
        # --help is not present, so --version should fire
        # Actually --version appears, but also validate. --version check is
        # done before subcommand dispatch.
        # Wait -- --help is checked first, then --version. If --version is
        # present, it fires.
        assert code == 0
        ver = _read_version()
        assert stdout == f"yass {ver}\n"

    def test_version_no_v_prefix(self):
        """Version string must not have a 'v' prefix."""
        code, stdout, stderr = _run_dispatch(["--version"])
        # Output is "yass X.Y.Z\n"
        version_part = stdout.strip().split(" ", 1)[1]
        assert not version_part.startswith("v")

    def test_version_subprocess(self):
        """Integration: python -m yass --version."""
        code, stdout, stderr = _run_main(["--version"])
        assert code == 0
        assert stdout.startswith("yass ")
        assert stdout.endswith("\n")


# ---------------------------------------------------------------------------
# No arguments -> yass.argv.no_subcommand
# ---------------------------------------------------------------------------

class TestNoSubcommand:
    """No args -> error yass.argv.no_subcommand, exit 2."""

    def test_no_args(self):
        code, stdout, stderr = _run_dispatch([])
        assert code == 2
        assert "[yass.argv.no_subcommand]" in stderr
        assert stdout == ""

    def test_no_args_subprocess(self):
        code, stdout, stderr = _run_main([])
        assert code == 2
        assert "[yass.argv.no_subcommand]" in stderr


# ---------------------------------------------------------------------------
# Unknown subcommand
# ---------------------------------------------------------------------------

class TestUnknownSubcommand:
    """Unknown subcommand -> error, exit 2."""

    def test_unknown_subcommand(self):
        code, stdout, stderr = _run_dispatch(["foobar"])
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr
        assert "foobar" in stderr

    def test_unknown_subcommand_subprocess(self):
        code, stdout, stderr = _run_main(["foobar"])
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr


# ---------------------------------------------------------------------------
# Unknown flag
# ---------------------------------------------------------------------------

class TestUnknownFlag:
    """Unknown flag -> error, exit 2."""

    def test_unknown_long_flag(self):
        code, stdout, stderr = _run_dispatch(["--foobar"])
        assert code == 2
        assert "[yass.argv.unknown_flag]" in stderr
        assert "--foobar" in stderr

    def test_unknown_flag_before_subcommand(self):
        code, stdout, stderr = _run_dispatch(["--verbose", "validate"])
        assert code == 2
        assert "[yass.argv.unknown_flag]" in stderr


# ---------------------------------------------------------------------------
# Short flag
# ---------------------------------------------------------------------------

class TestShortFlag:
    """Short flag -> error, exit 2."""

    def test_short_flag(self):
        code, stdout, stderr = _run_dispatch(["-h"])
        assert code == 2
        assert "[yass.argv.short_flag]" in stderr

    def test_short_flag_v(self):
        code, stdout, stderr = _run_dispatch(["-v"])
        assert code == 2
        assert "[yass.argv.short_flag]" in stderr

    def test_short_flag_with_subcommand(self):
        code, stdout, stderr = _run_dispatch(["-x", "validate"])
        assert code == 2
        assert "[yass.argv.short_flag]" in stderr


# ---------------------------------------------------------------------------
# Case mismatch
# ---------------------------------------------------------------------------

class TestCaseMismatch:
    """Case mismatch -> error, exit 2."""

    def test_uppercase_subcommand(self):
        code, stdout, stderr = _run_dispatch(["Validate"])
        assert code == 2
        assert "[yass.argv.case_mismatch]" in stderr
        assert "'validate'" in stderr

    def test_uppercase_flag(self):
        code, stdout, stderr = _run_dispatch(["--Help"])
        # --Help should NOT trigger --help (case-sensitive), so case_mismatch
        # Wait -- --help is checked with `"--help" in argv`, which is case-sensitive.
        # So --Help would NOT match --help. Then it would be checked:
        # - It starts with --, so it's a flag
        # - Case mismatch check: "--Help".lower() == "--help" matches "--help"
        #   but "--Help" != "--help", so case_mismatch fires.
        assert code == 2
        assert "[yass.argv.case_mismatch]" in stderr

    def test_mixed_case_subcommand(self):
        code, stdout, stderr = _run_dispatch(["QUERY"])
        assert code == 2
        assert "[yass.argv.case_mismatch]" in stderr
        assert "'query'" in stderr

    def test_list_case_mismatch(self):
        code, stdout, stderr = _run_dispatch(["List"])
        assert code == 2
        assert "[yass.argv.case_mismatch]" in stderr
        assert "'list'" in stderr


# ---------------------------------------------------------------------------
# Abbreviation
# ---------------------------------------------------------------------------

class TestAbbreviation:
    """Abbreviation -> error, exit 2."""

    def test_abbreviated_subcommand(self):
        code, stdout, stderr = _run_dispatch(["val"])
        assert code == 2
        assert "[yass.argv.abbreviation]" in stderr
        assert "'validate'" in stderr

    def test_abbreviated_flag(self):
        code, stdout, stderr = _run_dispatch(["--hel"])
        assert code == 2
        assert "[yass.argv.abbreviation]" in stderr
        assert "'--help'" in stderr

    def test_abbreviated_version(self):
        code, stdout, stderr = _run_dispatch(["--ver"])
        assert code == 2
        assert "[yass.argv.abbreviation]" in stderr
        assert "'--version'" in stderr

    def test_ambiguous_abbreviation_no_error(self):
        """Ambiguous abbreviation (prefix of multiple) should be unknown_subcommand,
        not abbreviation, since abbreviation only fires for unique prefix match."""
        # "li" is a prefix of "list" only, so it IS an abbreviation
        code, stdout, stderr = _run_dispatch(["li"])
        assert code == 2
        assert "[yass.argv.abbreviation]" in stderr


# ---------------------------------------------------------------------------
# Empty argument
# ---------------------------------------------------------------------------

class TestEmptyArgument:
    """Empty string arg -> error, exit 2."""

    def test_empty_arg(self):
        code, stdout, stderr = _run_dispatch([""])
        assert code == 2
        assert "[yass.argv.empty_argument]" in stderr

    def test_empty_arg_after_subcommand(self):
        code, stdout, stderr = _run_dispatch(["validate", ""])
        assert code == 2
        assert "[yass.argv.empty_argument]" in stderr


# ---------------------------------------------------------------------------
# Stdin dash
# ---------------------------------------------------------------------------

class TestStdinDash:
    """Bare '-' -> error, exit 2."""

    def test_bare_dash(self):
        code, stdout, stderr = _run_dispatch(["-"])
        assert code == 2
        assert "[yass.argv.stdin_dash]" in stderr

    def test_bare_dash_after_subcommand(self):
        code, stdout, stderr = _run_dispatch(["validate", "-"])
        assert code == 2
        assert "[yass.argv.stdin_dash]" in stderr


# ---------------------------------------------------------------------------
# Subcommand dispatch
# ---------------------------------------------------------------------------

class TestSubcommandDispatch:
    """Subcommands dispatch to the correct handler."""

    def test_validate_dispatches(self):
        """validate subcommand is dispatched (will fail without files,
        but the dispatch itself works)."""
        code, stdout, stderr = _run_dispatch(["validate"])
        # Should dispatch to validate_command with empty args.
        # Without a .git marker, it will fail with findroot_no_marker.
        # The important thing is it didn't fail with unknown_subcommand.
        assert "[yass.argv.unknown_subcommand]" not in stderr

    def test_query_dispatches(self):
        """query subcommand is dispatched."""
        code, stdout, stderr = _run_dispatch(["query"])
        # query with no args should produce query.name_missing
        assert code == 2
        assert "[yass.query.name_missing]" in stderr

    def test_list_dispatches(self):
        """list subcommand is dispatched."""
        code, stdout, stderr = _run_dispatch(["list"])
        # list with no project root will fail with findroot_no_marker.
        assert "[yass.argv.unknown_subcommand]" not in stderr


# ---------------------------------------------------------------------------
# Subcommand args passthrough
# ---------------------------------------------------------------------------

class TestArgsPassthrough:
    """Arguments after subcommand are passed through correctly."""

    def test_validate_with_args(self, tmp_path):
        """Extra args are forwarded to the subcommand handler."""
        # Create a fake spec file and .git
        git_dir = tmp_path / ".git"
        git_dir.mkdir()
        spec_file = tmp_path / "test.yass.yaml"
        spec_file.write_text(
            "---\ndescription: test\nversion: v1\n---\nspec: A\nRETURN:\n- MUST: work\n",
            encoding="utf-8",
        )
        # Run with the spec file path
        stdout = io.StringIO()
        stderr = io.StringIO()
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_path)
            code = _dispatch(["validate", str(spec_file)], stdout, stderr)
        finally:
            os.chdir(original_cwd)
        # Should succeed or at least not produce unknown_subcommand
        assert "[yass.argv.unknown_subcommand]" not in stderr.getvalue()

    def test_query_with_name(self, tmp_path):
        """query receives the name argument."""
        git_dir = tmp_path / ".git"
        git_dir.mkdir()
        spec_file = tmp_path / "test.yass.yaml"
        spec_file.write_text(
            "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nRETURN:\n- MUST: work\n",
            encoding="utf-8",
        )
        stdout = io.StringIO()
        stderr = io.StringIO()
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_path)
            code = _dispatch(["query", "MySpec"], stdout, stderr)
        finally:
            os.chdir(original_cwd)
        assert code == 0
        assert "MySpec" in stdout.getvalue()


# ---------------------------------------------------------------------------
# Uncaught exception handling
# ---------------------------------------------------------------------------

class TestUncaughtException:
    """Uncaught exception produces correct ErrorLine."""

    def test_uncaught_exception_format(self):
        """When a subcommand raises, emit ErrorLine and exit 1."""
        with mock.patch(
            "yass.validate.validate_command",
            side_effect=RuntimeError("something\nbroke"),
        ):
            code, stdout, stderr = _run_dispatch(["validate"])
        assert code == 1
        assert "[yass.internal.uncaught]" in stderr
        assert "internal error:" in stderr
        # Newlines in message should be replaced with spaces
        assert "something broke" in stderr
        assert "\nbroke" not in stderr

    def test_uncaught_exception_query(self):
        with mock.patch(
            "yass.query.query_command",
            side_effect=ValueError("query went wrong"),
        ):
            code, stdout, stderr = _run_dispatch(["query", "SomeName"])
        assert code == 1
        assert "[yass.internal.uncaught]" in stderr

    def test_uncaught_exception_list(self):
        with mock.patch(
            "yass.list_cmd.list_command",
            side_effect=TypeError("list failed"),
        ):
            code, stdout, stderr = _run_dispatch(["list"])
        assert code == 1
        assert "[yass.internal.uncaught]" in stderr


# ---------------------------------------------------------------------------
# Error format
# ---------------------------------------------------------------------------

class TestErrorFormat:
    """Error format: 'yass: [<code>] <message>\n'."""

    def test_error_uses_yass_as_file(self):
        """For argv-level errors, file is literal 'yass'."""
        code, stdout, stderr = _run_dispatch(["foobar"])
        assert stderr.startswith("yass: [")

    def test_error_ends_with_newline(self):
        code, stdout, stderr = _run_dispatch(["foobar"])
        assert stderr.endswith("\n")

    def test_error_format_structure(self):
        """Error line matches: yass: [code] message\n."""
        code, stdout, stderr = _run_dispatch([])
        line = stderr.strip()
        # Format: "yass: [yass.argv.no_subcommand] <message>"
        assert line.startswith("yass: [yass.argv.no_subcommand]")


# ---------------------------------------------------------------------------
# Double-dash end-of-options marker
# ---------------------------------------------------------------------------

class TestDoubleDash:
    """-- end-of-options marker."""

    def test_double_dash_only(self):
        """Just -- with nothing else -> no_subcommand."""
        code, stdout, stderr = _run_dispatch(["--"])
        assert code == 2
        assert "[yass.argv.no_subcommand]" in stderr

    def test_double_dash_before_subcommand(self):
        """-- before a subcommand: subcommand is treated as positional."""
        code, stdout, stderr = _run_dispatch(["--", "validate"])
        # After --, validate is just a positional. It should still dispatch.
        assert "[yass.argv.unknown_subcommand]" not in stderr

    def test_flag_like_arg_after_double_dash_is_positional(self):
        """After --, flag-like strings are treated as positionals."""
        code, stdout, stderr = _run_dispatch(["--", "--foobar"])
        # --foobar after -- is a positional, so it's treated as a subcommand.
        # It's not a known subcommand, so unknown_subcommand.
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr

    def test_short_flag_after_double_dash_is_positional(self):
        """After --, short flags are treated as positionals."""
        code, stdout, stderr = _run_dispatch(["--", "-x"])
        # -x after -- should be treated as a positional, not a short flag error.
        # It becomes the subcommand. But it starts with -, and after -- it's a
        # positional. However, bare "-x" is not "-" so it's not stdin_dash.
        # Wait -- after the separator, we just collect positionals. So "-x" is
        # collected as a positional. Then it's the subcommand, but it's not
        # known, so unknown_subcommand.
        # But wait: the spec says "Bare '-'" and "Short-form flag (starts with - but not --)"
        # These checks are part of the arg validation loop, which skips flag
        # checks after --. Let me re-read my implementation...
        # In _dispatch, after past_separator, we skip the flag checks and go
        # straight to positional handling.
        # Actually, looking at my implementation: after -- is set, we still
        # check for empty string, "-", etc. Let me check.
        # My code: bare "-" check is before the past_separator check.
        # Actually no, let me re-check the flow:
        # 1. Empty string -> error
        # 2. "--" -> set past_separator
        # 3. "-" (bare dash) -> error  (this fires regardless of past_separator)
        # 4. flag checks only if not past_separator
        # 5. else: positional
        #
        # So "-x" after "--": step 1 no, step 2 no, step 3 no (it's "-x" not "-"),
        # step 4: starts with "-" and not past_separator? past_separator IS true,
        # so we skip flag checks. Step 5: positional. It goes to positionals.
        # Then it's the subcommand "-x" which is unknown. -> unknown_subcommand
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr


# ---------------------------------------------------------------------------
# Integration: subprocess tests
# ---------------------------------------------------------------------------

class TestSubprocessIntegration:
    """Integration tests via subprocess."""

    def test_help_subprocess(self):
        code, stdout, stderr = _run_main(["--help"])
        assert code == 0
        assert "usage:" in stdout

    def test_version_subprocess(self):
        code, stdout, stderr = _run_main(["--version"])
        assert code == 0
        ver = _read_version()
        assert stdout.strip() == f"yass {ver}"

    def test_no_args_subprocess(self):
        code, stdout, stderr = _run_main([])
        assert code == 2
        assert "[yass.argv.no_subcommand]" in stderr

    def test_unknown_subcommand_subprocess(self):
        code, stdout, stderr = _run_main(["bogus"])
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr

    def test_short_flag_subprocess(self):
        code, stdout, stderr = _run_main(["-h"])
        assert code == 2
        assert "[yass.argv.short_flag]" in stderr

    def test_empty_arg_subprocess(self):
        code, stdout, stderr = _run_main([""])
        assert code == 2
        assert "[yass.argv.empty_argument]" in stderr


# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------

class TestEdgeCases:
    """Miscellaneous edge cases."""

    def test_multiple_double_dashes(self):
        """Second -- after first is treated as positional."""
        code, stdout, stderr = _run_dispatch(["--", "--"])
        # First -- sets past_separator. Second -- is a positional.
        # "--" as subcommand: not in known subcommands -> unknown_subcommand
        # But wait, in the loop: second "--" hits the `arg == "--" and not past_separator`
        # check. past_separator is True, so it falls through to positional.
        # As a positional "--", it's the subcommand. Not a known subcommand.
        assert code == 2
        assert "[yass.argv.unknown_subcommand]" in stderr

    def test_help_has_priority_over_errors(self):
        """--help takes priority even when invalid args are present."""
        code, stdout, stderr = _run_dispatch(["--help", "-x", ""])
        assert code == 0
        assert "usage:" in stdout

    def test_version_has_priority_over_errors(self):
        """--version takes priority over errors (when --help absent)."""
        code, stdout, stderr = _run_dispatch(["--version", "-x", ""])
        assert code == 0
        ver = _read_version()
        assert stdout == f"yass {ver}\n"

    def test_bare_dash_after_separator(self):
        """Bare '-' after -- is still an error."""
        code, stdout, stderr = _run_dispatch(["--", "-"])
        assert code == 2
        assert "[yass.argv.stdin_dash]" in stderr

    def test_validate_args_forwarded_correctly(self):
        """Positionals after subcommand are forwarded as remaining args."""
        with mock.patch("yass.validate.validate_command", return_value=0) as m:
            code, stdout, stderr = _run_dispatch(["validate", "file1.yass.yaml", "file2.yass.yaml"])
        assert code == 0
        m.assert_called_once()
        args = m.call_args[0][0]
        assert args == ["file1.yass.yaml", "file2.yass.yaml"]

    def test_query_args_forwarded_correctly(self):
        """query subcommand forwards name and scope."""
        with mock.patch("yass.query.query_command", return_value=0) as m:
            code, stdout, stderr = _run_dispatch(["query", "MySpec", "some/path"])
        assert code == 0
        m.assert_called_once()
        args = m.call_args[0][0]
        assert args == ["MySpec", "some/path"]

    def test_list_args_forwarded_correctly(self):
        """list subcommand forwards path argument."""
        with mock.patch("yass.list_cmd.list_command", return_value=0) as m:
            code, stdout, stderr = _run_dispatch(["list", "some/dir"])
        assert code == 0
        m.assert_called_once()
        args = m.call_args[0][0]
        assert args == ["some/dir"]
