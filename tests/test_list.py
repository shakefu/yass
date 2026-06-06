"""Tests for yass.list_cmd — the ``yass list`` subcommand."""

from __future__ import annotations

import io
import os
import textwrap

import pytest

from yass.list_cmd import list_command


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _run(args, *, tmp_path=None, monkeypatch=None, env=None, isatty=False):
    """Run list_command and return (exit_code, stdout_str, stderr_str).

    If *isatty* is True, the stdout wrapper reports isatty() == True.
    """
    stdout = io.StringIO()
    stderr = io.StringIO()

    if isatty:
        stdout.isatty = lambda: True  # type: ignore[attr-defined]
    else:
        stdout.isatty = lambda: False  # type: ignore[attr-defined]

    if env:
        for k, v in env.items():
            monkeypatch.setenv(k, v)

    rc = list_command(args, stdout=stdout, stderr=stderr)
    return rc, stdout.getvalue(), stderr.getvalue()


def _write_spec(path, content):
    """Write *content* to *path*, creating parent dirs as needed."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


# ---------------------------------------------------------------------------
# Single file with specs
# ---------------------------------------------------------------------------

class TestSingleFileWithSpecs:
    """A single .yass.yaml file containing preamble + spec documents."""

    def test_single_spec(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        content = textwrap.dedent("""\
            ---
            description: A test specification
            version: v1
            ---
            spec: MySpec
            INPUT:
            - MUST: accept input
        """)
        (tmp_path / "api.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "api.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        assert err == ""
        lines = out.strip().split("\n")
        assert len(lines) == 1
        parts = lines[0].split("\t")
        assert len(parts) == 3
        assert parts[0].endswith("api.yass.yaml")
        assert parts[1] == "MySpec"
        assert parts[2] == "A test specification"

    def test_multiple_specs_in_one_file(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        content = textwrap.dedent("""\
            ---
            description: Multi-spec file
            version: v1
            ---
            spec: SpecAlpha
            RETURN:
            - MUST: work
            ---
            spec: SpecBeta
            INPUT:
            - MUST: accept
        """)
        (tmp_path / "multi.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "multi.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        lines = out.strip().split("\n")
        assert len(lines) == 2
        # Document order preserved
        assert lines[0].split("\t")[1] == "SpecAlpha"
        assert lines[1].split("\t")[1] == "SpecBeta"
        # Both share the preamble description
        assert lines[0].split("\t")[2] == "Multi-spec file"
        assert lines[1].split("\t")[2] == "Multi-spec file"


# ---------------------------------------------------------------------------
# File with no specs
# ---------------------------------------------------------------------------

class TestFileWithNoSpecs:
    """A file that parses but has no Spec documents produces no rows."""

    def test_preamble_only(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        content = textwrap.dedent("""\
            ---
            description: Just a preamble
            version: v1
        """)
        (tmp_path / "empty.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "empty.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        assert out == ""
        assert err == ""

    def test_file_with_non_dict_docs(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = "---\n- just a list\n"
        (tmp_path / "nospec.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "nospec.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        assert out == ""


# ---------------------------------------------------------------------------
# Multiple files sorted by path
# ---------------------------------------------------------------------------

class TestMultipleFilesSorted:
    """When listing a directory, files are sorted by NFC path."""

    def test_sort_order(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()

        for name in ["z.yass.yaml", "a.yass.yaml", "m.yass.yaml"]:
            content = textwrap.dedent(f"""\
                ---
                description: Spec in {name}
                version: v1
                ---
                spec: Spec_{name[0].upper()}
                RETURN:
                - MUST: work
            """)
            (tmp_path / name).write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path)], monkeypatch=monkeypatch)
        assert rc == 0
        lines = out.strip().split("\n")
        assert len(lines) == 3
        paths = [l.split("\t")[0] for l in lines]
        assert paths == sorted(paths)
        # Verify the order: a, m, z
        assert lines[0].split("\t")[1] == "Spec_A"
        assert lines[1].split("\t")[1] == "Spec_M"
        assert lines[2].split("\t")[1] == "Spec_Z"


# ---------------------------------------------------------------------------
# Description normalization
# ---------------------------------------------------------------------------

class TestDescriptionNormalization:
    """Whitespace collapsing and NFC normalization of descriptions."""

    def test_whitespace_collapsing(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = "---\ndescription: \"hello   world\\n  foo\\tbar\"\nversion: v1\n---\nspec: WS\nRETURN:\n- MUST: work\n"
        (tmp_path / "ws.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "ws.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        desc = out.strip().split("\t")[2]
        assert desc == "hello world foo bar"

    def test_nfc_normalization(self, tmp_path, monkeypatch):
        """NFD characters in description are normalized to NFC."""
        import unicodedata
        monkeypatch.chdir(tmp_path)
        # e + combining acute accent (NFD form of e-acute)
        nfd_desc = "café"
        assert unicodedata.is_normalized("NFD", nfd_desc)
        content = f"---\ndescription: \"{nfd_desc}\"\nversion: v1\n---\nspec: NFC\nRETURN:\n- MUST: work\n"
        (tmp_path / "nfc.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "nfc.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        desc = out.strip().split("\t")[2]
        assert unicodedata.is_normalized("NFC", desc)
        assert desc == unicodedata.normalize("NFC", nfd_desc)

    def test_multiline_description(self, tmp_path, monkeypatch):
        """Multi-line YAML description gets whitespace-collapsed."""
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: >
              This is a long
              description that spans
              multiple lines.
            version: v1
            ---
            spec: MultiLine
            RETURN:
            - MUST: work
        """)
        (tmp_path / "ml.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "ml.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        desc = out.strip().split("\t")[2]
        # No runs of whitespace remain
        assert "  " not in desc
        assert "\n" not in desc


# ---------------------------------------------------------------------------
# Empty/missing description
# ---------------------------------------------------------------------------

class TestEmptyMissingDescription:
    """When preamble has no description, the third field is empty."""

    def test_no_description_key(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            version: v1
            ---
            spec: NoDesc
            RETURN:
            - MUST: work
        """)
        (tmp_path / "nodesc.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "nodesc.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert parts[2] == ""

    def test_empty_string_description(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: ""
            version: v1
            ---
            spec: EmptyDesc
            RETURN:
            - MUST: work
        """)
        (tmp_path / "emptydesc.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "emptydesc.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert parts[2] == ""

    def test_non_string_description(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: 42
            version: v1
            ---
            spec: NumDesc
            RETURN:
            - MUST: work
        """)
        (tmp_path / "numdesc.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "numdesc.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert parts[2] == ""

    def test_two_tabs_always_present(self, tmp_path, monkeypatch):
        """Each row always has exactly 2 tab characters."""
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            version: v1
            ---
            spec: Tabs
            RETURN:
            - MUST: work
        """)
        (tmp_path / "tabs.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "tabs.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        line = out.rstrip("\n")
        assert line.count("\t") == 2


# ---------------------------------------------------------------------------
# Tab in filepath
# ---------------------------------------------------------------------------

class TestTabInFilepath:
    """Literal tabs in the filepath field are replaced with single spaces."""

    def test_tab_replaced(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        # Create a directory with a tab in its name
        tab_dir = tmp_path / "dir\there"
        try:
            tab_dir.mkdir()
        except OSError:
            pytest.skip("OS does not support tabs in directory names")
        content = textwrap.dedent("""\
            ---
            description: Tab test
            version: v1
            ---
            spec: TabSpec
            RETURN:
            - MUST: work
        """)
        spec_file = tab_dir / "spec.yass.yaml"
        spec_file.write_text(content, encoding="utf-8")

        rc, out, err = _run([str(spec_file)], monkeypatch=monkeypatch)
        assert rc == 0
        # The filepath field should not contain literal tabs
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert "\t" not in parts[0]
        assert " " in parts[0]  # tab was replaced with space


# ---------------------------------------------------------------------------
# TTY truncation
# ---------------------------------------------------------------------------

class TestTTYTruncation:
    """Truncation behavior when stdout is a terminal."""

    def test_truncation_with_columns(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("COLUMNS", "40")
        content = textwrap.dedent("""\
            ---
            description: This is a very long description that should be truncated when displayed on a narrow terminal
            version: v1
            ---
            spec: Trunc
            RETURN:
            - MUST: work
        """)
        (tmp_path / "t.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(
            [str(tmp_path / "t.yass.yaml")],
            monkeypatch=monkeypatch,
            isatty=True,
        )
        assert rc == 0
        line = out.strip()
        parts = line.split("\t")
        assert len(parts) == 3
        # Description should be truncated with "..."
        assert parts[2].endswith("...")

    def test_no_truncation_when_not_tty(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("COLUMNS", "40")
        content = textwrap.dedent("""\
            ---
            description: This is a very long description that should NOT be truncated when not a TTY
            version: v1
            ---
            spec: NoTrunc
            RETURN:
            - MUST: work
        """)
        (tmp_path / "nt.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(
            [str(tmp_path / "nt.yass.yaml")],
            monkeypatch=monkeypatch,
            isatty=False,
        )
        assert rc == 0
        line = out.strip()
        parts = line.split("\t")
        assert parts[2] == "This is a very long description that should NOT be truncated when not a TTY"

    def test_empty_desc_no_marker(self, tmp_path, monkeypatch):
        """Empty description gets no truncation marker even in TTY mode."""
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("COLUMNS", "40")
        content = textwrap.dedent("""\
            ---
            version: v1
            ---
            spec: NoDescTTY
            RETURN:
            - MUST: work
        """)
        (tmp_path / "nd.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(
            [str(tmp_path / "nd.yass.yaml")],
            monkeypatch=monkeypatch,
            isatty=True,
        )
        assert rc == 0
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert parts[2] == ""
        assert "..." not in out

    def test_prefix_too_wide_for_marker(self, tmp_path, monkeypatch):
        """When file+name+separators+marker >= width: empty desc, no marker."""
        monkeypatch.chdir(tmp_path)
        # Make a very narrow terminal so that filepath + spec name + tabs + "..." >= width
        monkeypatch.setenv("COLUMNS", "15")
        content = textwrap.dedent("""\
            ---
            description: Something
            version: v1
            ---
            spec: LongSpecName
            RETURN:
            - MUST: work
        """)
        (tmp_path / "wide.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(
            [str(tmp_path / "wide.yass.yaml")],
            monkeypatch=monkeypatch,
            isatty=True,
        )
        assert rc == 0
        line = out.rstrip("\n")
        parts = line.split("\t")
        assert len(parts) == 3
        assert parts[2] == ""

    def test_description_fits_no_truncation(self, tmp_path, monkeypatch):
        """When description fits within width, no marker appended."""
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("COLUMNS", "200")
        content = textwrap.dedent("""\
            ---
            description: Short
            version: v1
            ---
            spec: S
            RETURN:
            - MUST: work
        """)
        (tmp_path / "s.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(
            [str(tmp_path / "s.yass.yaml")],
            monkeypatch=monkeypatch,
            isatty=True,
        )
        assert rc == 0
        parts = out.strip().split("\t")
        assert parts[2] == "Short"
        assert "..." not in parts[2]


# ---------------------------------------------------------------------------
# Parse failure
# ---------------------------------------------------------------------------

class TestParseFailure:
    """Files that fail YAML parsing produce errors but listing continues."""

    def test_parse_error_exit_1(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()

        # Good file
        good = textwrap.dedent("""\
            ---
            description: Good
            version: v1
            ---
            spec: GoodSpec
            RETURN:
            - MUST: work
        """)
        (tmp_path / "a_good.yass.yaml").write_text(good, encoding="utf-8")

        # Bad file (malformed YAML)
        bad = "---\n  bad: [yaml: {broken\n"
        (tmp_path / "b_bad.yass.yaml").write_text(bad, encoding="utf-8")

        rc, out, err = _run([str(tmp_path)], monkeypatch=monkeypatch)
        assert rc == 1
        # The good file should still be listed
        assert "GoodSpec" in out
        # Error should be on stderr
        assert "b_bad.yass.yaml" in err

    def test_parse_error_to_stderr(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        bad = "---\n  bad: [yaml: {broken\n"
        (tmp_path / "bad.yass.yaml").write_text(bad, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "bad.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 1
        assert out == ""
        assert "bad.yass.yaml" in err


# ---------------------------------------------------------------------------
# Path not found
# ---------------------------------------------------------------------------

class TestPathNotFound:
    """Non-existent path exits with code 2."""

    def test_exit_2(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        rc, out, err = _run([str(tmp_path / "nonexistent.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 2
        assert out == ""
        assert err != ""


# ---------------------------------------------------------------------------
# Bad extension
# ---------------------------------------------------------------------------

class TestBadExtension:
    """File with wrong extension exits with code 2."""

    def test_exit_2(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "readme.yaml"
        f.write_text("---\nspec: X\n", encoding="utf-8")
        rc, out, err = _run([str(f)], monkeypatch=monkeypatch)
        assert rc == 2
        assert out == ""
        assert err != ""


# ---------------------------------------------------------------------------
# Colon in path
# ---------------------------------------------------------------------------

class TestColonInPath:
    """Paths containing ':' exit with code 2."""

    def test_exit_2(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        rc, out, err = _run(["some:path.yass.yaml"], monkeypatch=monkeypatch)
        assert rc == 2
        assert out == ""
        assert PATH_COLON_MSG in err or "colon" in err.lower()


PATH_COLON_MSG = "yass.path.colon_in_path"


# ---------------------------------------------------------------------------
# No files found
# ---------------------------------------------------------------------------

class TestNoFilesFound:
    """When no .yass.yaml files are in scope, exit 0 with no output."""

    def test_exit_0_empty_dir(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        rc, out, err = _run([str(tmp_path)], monkeypatch=monkeypatch)
        assert rc == 0
        assert out == ""

    def test_exit_0_no_path(self, tmp_path, monkeypatch):
        """No path arg, project root has no spec files."""
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        rc, out, err = _run([], monkeypatch=monkeypatch)
        assert rc == 0
        assert out == ""


# ---------------------------------------------------------------------------
# Row formatting details
# ---------------------------------------------------------------------------

class TestRowFormatting:
    """Each row ends with exactly one LF and has exactly 2 tabs."""

    def test_lf_terminated(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: LF
            RETURN:
            - MUST: work
        """)
        (tmp_path / "lf.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path / "lf.yass.yaml")], monkeypatch=monkeypatch)
        assert rc == 0
        assert out.endswith("\n")
        # No double newlines
        assert "\n\n" not in out

    def test_exact_two_tabs_per_row(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        for name in ["a.yass.yaml", "b.yass.yaml"]:
            content = f"---\ndescription: Desc\nversion: v1\n---\nspec: Spec_{name[0]}\nRETURN:\n- MUST: work\n"
            (tmp_path / name).write_text(content, encoding="utf-8")

        rc, out, err = _run([str(tmp_path)], monkeypatch=monkeypatch)
        assert rc == 0
        for line in out.strip().split("\n"):
            assert line.count("\t") == 2


# ---------------------------------------------------------------------------
# Dash argument treated as literal path
# ---------------------------------------------------------------------------

class TestDashAsLiteralPath:
    """The '-' argument is treated as a literal path, not stdin."""

    def test_dash_path_not_found(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        rc, out, err = _run(["-"], monkeypatch=monkeypatch)
        assert rc == 2
        assert out == ""


# ---------------------------------------------------------------------------
# No path uses project root discovery
# ---------------------------------------------------------------------------

class TestNoPathProjectRoot:
    """When no path is given, discover from project root."""

    def test_discovers_from_root(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / ".git").mkdir()
        sub = tmp_path / "sub"
        sub.mkdir()
        content = textwrap.dedent("""\
            ---
            description: Sub spec
            version: v1
            ---
            spec: SubSpec
            RETURN:
            - MUST: work
        """)
        (sub / "sub.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run([], monkeypatch=monkeypatch)
        assert rc == 0
        assert "SubSpec" in out
