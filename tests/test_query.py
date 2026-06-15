"""Tests for yass.query -- the ``yass query`` subcommand."""

from __future__ import annotations

import io
import os
import textwrap

import pytest

from yass.query import (
    emit_yaml_fragment,
    extract_fragment,
    inline_conforms,
    name_lookup,
    query_command,
)
from yass.errors import (
    QUERY_CONFORMS_NO_SLOT,
    QUERY_CONFORMS_UNRESOLVED,
    QUERY_NAME_BLANK,
    QUERY_NAME_MISSING,
    QUERY_NO_MATCH,
    QUERY_SCOPE_EMPTY,
    QUERY_SCOPE_NOT_FOUND,
    PATH_COLON_IN_PATH,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _run(args, *, monkeypatch=None):
    """Run query_command and return (exit_code, stdout_str, stderr_str)."""
    stdout = io.StringIO()
    stderr = io.StringIO()
    stdout.isatty = lambda: False  # type: ignore[attr-defined]
    rc = query_command(args, stdout=stdout, stderr=stderr)
    return rc, stdout.getvalue(), stderr.getvalue()


def _write_spec(path, content):
    """Write *content* to *path*, creating parent dirs as needed."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _make_project(tmp_path):
    """Create a .git marker in tmp_path."""
    (tmp_path / ".git").mkdir(exist_ok=True)


# ---------------------------------------------------------------------------
# name_lookup tests
# ---------------------------------------------------------------------------

class TestNameLookup:
    """Tests for name_lookup function."""

    def test_exact_match(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        results = name_lookup("MySpec", [str(f)])
        assert len(results) == 1
        assert results[0][1] == "MySpec"

    def test_trailing_suffix_match(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: pkg.Foo
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        results = name_lookup("Foo", [str(f)])
        assert len(results) == 1
        assert results[0][1] == "pkg.Foo"

    def test_multi_level_suffix_match(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: a.b.Foo
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        # "b.Foo" matches "a.b.Foo"
        results = name_lookup("b.Foo", [str(f)])
        assert len(results) == 1
        assert results[0][1] == "a.b.Foo"

    def test_no_partial_substring(self, tmp_path, monkeypatch):
        """Partial substrings that are not dot-aligned should NOT match."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: ab.Foo
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        # "b.Foo" should NOT match "ab.Foo" (not at dot boundary)
        results = name_lookup("b.Foo", [str(f)])
        assert len(results) == 0

    def test_no_match(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        results = name_lookup("OtherSpec", [str(f)])
        assert len(results) == 0

    def test_whitespace_name_no_match(self, tmp_path, monkeypatch):
        """Names with whitespace should return no matches, not raise."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        results = name_lookup("My Spec", [str(f)])
        assert len(results) == 0

    def test_blank_name_error(self, tmp_path, monkeypatch):
        """Empty string name should raise ValueError with QUERY_NAME_BLANK."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        with pytest.raises(ValueError, match=QUERY_NAME_BLANK):
            name_lookup("", [])

    def test_case_sensitivity(self, tmp_path, monkeypatch):
        """Name lookup is case-sensitive."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        # Lowercase should not match
        results = name_lookup("myspec", [str(f)])
        assert len(results) == 0

        # Different case should not match
        results = name_lookup("MYSPEC", [str(f)])
        assert len(results) == 0

    def test_multiple_files(self, tmp_path, monkeypatch):
        """Matches across multiple files."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        c1 = textwrap.dedent("""\
            ---
            description: File 1
            version: v1
            ---
            spec: pkg.Foo
            RETURN:
            - MUST: work
        """)
        c2 = textwrap.dedent("""\
            ---
            description: File 2
            version: v1
            ---
            spec: other.Foo
            RETURN:
            - MUST: work
        """)
        f1 = tmp_path / "file1.yass.yaml"
        f2 = tmp_path / "file2.yass.yaml"
        f1.write_text(c1, encoding="utf-8")
        f2.write_text(c2, encoding="utf-8")

        results = name_lookup("Foo", [str(f1), str(f2)])
        assert len(results) == 2


# ---------------------------------------------------------------------------
# extract_fragment tests
# ---------------------------------------------------------------------------

class TestExtractFragment:
    """Tests for extract_fragment function."""

    def test_correct_yaml_output(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        result = extract_fragment(str(f), "MySpec")
        assert result.startswith("---\n")
        assert "spec: MySpec" in result
        assert "MUST: work" in result
        # No trailing "..."
        assert not result.rstrip("\n").endswith("...")

    def test_key_ordering(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: OrderSpec
            INPUT:
            - MUST: accept input
            RETURN:
            - MUST: return result
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        result = extract_fragment(str(f), "OrderSpec")
        # "spec" should come before "INPUT", which should come before "RETURN"
        spec_pos = result.index("spec:")
        input_pos = result.index("INPUT:")
        return_pos = result.index("RETURN:")
        assert spec_pos < input_pos < return_pos

    def test_no_preamble(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Should not appear
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        result = extract_fragment(str(f), "MySpec")
        assert "description:" not in result
        assert "version:" not in result
        assert "Should not appear" not in result

    def test_no_other_specs(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: SpecA
            RETURN:
            - MUST: work A
            ---
            spec: SpecB
            RETURN:
            - MUST: work B
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        result = extract_fragment(str(f), "SpecA")
        assert "SpecA" in result
        assert "SpecB" not in result
        assert "work B" not in result

    def test_ends_with_one_lf(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        f = tmp_path / "test.yass.yaml"
        f.write_text(content, encoding="utf-8")

        result = extract_fragment(str(f), "MySpec")
        assert result.endswith("\n")
        assert not result.endswith("\n\n")


# ---------------------------------------------------------------------------
# inline_conforms tests
# ---------------------------------------------------------------------------

class TestInlineConforms:
    """Tests for inline_conforms function."""

    def test_reference_only_inlining(self, tmp_path, monkeypatch):
        """Reference-only CONFORMS obligation is replaced with inlined obligations."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        # Referenced spec
        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
            - SHOULD: be fast
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 2
        assert obligations[0].get("MUST") == "return a value"
        assert obligations[1].get("SHOULD") == "be fast"

    def test_normative_with_conforms(self, tmp_path, monkeypatch):
        """Normative obligation with CONFORMS keeps original and appends inlined."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "do something", "CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 2
        # First is the original (sans CONFORMS)
        assert obligations[0].get("MUST") == "do something"
        assert "CONFORMS" not in obligations[0]
        # Second is inlined
        assert obligations[1].get("MUST") == "return a value"

    def test_when_guard_combination(self, tmp_path, monkeypatch):
        """Carrier WHEN guard combined with inlined obligation WHEN guard."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - WHEN: the value is ready
              MUST: return it
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"WHEN": "called by the user", "CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 1
        assert obligations[0]["WHEN"] == "called by the user and the value is ready"
        assert obligations[0]["MUST"] == "return it"

    def test_carrier_when_without_inner_when(self, tmp_path, monkeypatch):
        """Carrier WHEN guard preserved on inlined obligation that has no WHEN."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"WHEN": "called by the user", "CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 1
        assert obligations[0]["WHEN"] == "called by the user"
        assert obligations[0]["MUST"] == "return a value"

    def test_provenance_comments(self, tmp_path, monkeypatch):
        """Provenance comments are added to inlined obligations."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert obligations[0].get("__provenance__") == "# CONFORMS: ./other@OtherSpec::RETURN"

    def test_no_recursive_inlining(self, tmp_path, monkeypatch):
        """CONFORMS refs within inlined obligations are NOT recursively resolved."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        deep = textwrap.dedent("""\
            ---
            description: Deep
            version: v1
            ---
            spec: DeepSpec
            RETURN:
            - MUST: deep value
        """)
        (tmp_path / "deep.yass.yaml").write_text(deep, encoding="utf-8")

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
              CONFORMS: ./deep@DeepSpec::RETURN
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        # The inlined obligation should still have its own CONFORMS (not recursively resolved)
        assert obligations[0].get("CONFORMS") == "./deep@DeepSpec::RETURN"

    def test_conforms_no_slot_error(self, tmp_path, monkeypatch):
        """CONFORMS ref without ::SLOT suffix produces conforms_no_slot error."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"CONFORMS": "./other@OtherSpec"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert len(errors) == 1
        assert errors[0].code == QUERY_CONFORMS_NO_SLOT

    def test_does_not_inline_uses(self, tmp_path, monkeypatch):
        """USES refs are NOT inlined, only CONFORMS."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work", "USES": "SomeSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 1
        assert obligations[0].get("USES") == "SomeSpec::RETURN"

    def test_does_not_inline_see(self, tmp_path, monkeypatch):
        """SEE refs are NOT inlined."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work", "SEE": "SomeSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert len(obligations) == 1
        assert obligations[0].get("SEE") == "SomeSpec::RETURN"

    def test_preserves_normativity_of_inlined(self, tmp_path, monkeypatch):
        """Inlined obligations preserve their original Normativity keyword."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - SHOULD: be fast
            - MAY: be cached
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {"CONFORMS": "./other@OtherSpec::RETURN"},
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        assert obligations[0].get("SHOULD") == "be fast"
        assert obligations[1].get("MAY") == "be cached"

    def test_strip_conforms_keep_other_refs(self, tmp_path, monkeypatch):
        """Non-CONFORMS refs are kept on the carrier obligation."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        spec_data = {
            "spec": "MySpec",
            "RETURN": [
                {
                    "MUST": "do something",
                    "CONFORMS": "./other@OtherSpec::RETURN",
                    "USES": "SomeSpec",
                },
            ],
        }

        result, errors = inline_conforms(
            spec_data,
            str(tmp_path / "main.yass.yaml"),
            str(tmp_path),
        )

        assert errors == []
        obligations = result["RETURN"]
        # First obligation (carrier) should keep USES but not CONFORMS
        assert obligations[0].get("MUST") == "do something"
        assert obligations[0].get("USES") == "SomeSpec"
        assert "CONFORMS" not in obligations[0]


# ---------------------------------------------------------------------------
# emit_yaml_fragment tests
# ---------------------------------------------------------------------------

class TestEmitYamlFragment:
    """Tests for emit_yaml_fragment function."""

    def test_basic_output(self):
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert result.startswith("---\n")
        assert "spec: MySpec" in result
        assert "MUST: work" in result
        assert result.endswith("\n")
        assert not result.endswith("\n\n")

    def test_two_space_indentation(self):
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        lines = result.split("\n")
        # RETURN: should be at indent 0
        for line in lines:
            if line.startswith("RETURN:"):
                assert not line.startswith(" ")
        # '- MUST:' should be at indent 0 (list item at parent indent)
        for line in lines:
            if "MUST: work" in line:
                assert line.startswith("- MUST:")

    def test_quoting_colon_space(self):
        """Scalars containing ': ' should be double-quoted."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "key: value"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert '"key: value"' in result

    def test_quoting_leading_special(self):
        """Scalars starting with special chars should be double-quoted."""
        for char in "?-*&!|>%@":
            data = {
                "spec": "MySpec",
                "RETURN": [
                    {"MUST": f"{char}test"},
                ],
            }
            result = emit_yaml_fragment(data, {})
            assert f'"{char}test"' in result, f"Failed for leading char: {char}"

    def test_quoting_core_schema_types(self):
        """Core-schema type tokens should be double-quoted."""
        for token in ["true", "false", "null", "yes", "no", "on", "off",
                       "True", "False", "Null", "Yes", "No", "On", "Off",
                       "TRUE", "FALSE", "NULL", "YES", "NO", "ON", "OFF"]:
            data = {
                "spec": "MySpec",
                "RETURN": [
                    {"MUST": token},
                ],
            }
            result = emit_yaml_fragment(data, {})
            assert f'"{token}"' in result, f"Failed to quote: {token}"

    def test_quoting_numeric_literals(self):
        """Numeric literal strings should be double-quoted."""
        for num in ["42", "3.14", "0xFF", "0o77", "-1", "+2", ".inf", ".nan"]:
            data = {
                "spec": "MySpec",
                "RETURN": [
                    {"MUST": num},
                ],
            }
            result = emit_yaml_fragment(data, {})
            assert f'"{num}"' in result, f"Failed to quote numeric: {num}"

    def test_no_quoting_plain_scalars(self):
        """Plain scalars should NOT be quoted."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "accept input"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert "MUST: accept input" in result
        assert '"accept input"' not in result

    def test_quoting_leading_trailing_whitespace(self):
        """Scalars with leading or trailing whitespace should be quoted."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": " leading"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert '" leading"' in result

    def test_key_ordering_in_obligation(self):
        """Obligation keys should be in order: Normativity, WHEN, References."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"USES": "SomeSpec", "WHEN": "called", "MUST": "work"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        lines = result.split("\n")
        must_line = None
        when_line = None
        uses_line = None
        for i, line in enumerate(lines):
            if "MUST:" in line:
                must_line = i
            if "WHEN:" in line:
                when_line = i
            if "USES:" in line:
                uses_line = i
        assert must_line is not None and when_line is not None and uses_line is not None
        assert must_line < when_line < uses_line

    def test_provenance_comments(self):
        """Provenance comments appear at column zero above inlined obligations."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"__provenance__": "# CONFORMS: ./other@OtherSpec::RETURN", "MUST": "return a value"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        lines = result.split("\n")
        # Find the provenance comment
        for i, line in enumerate(lines):
            if line == "# CONFORMS: ./other@OtherSpec::RETURN":
                # Next line should be the obligation
                assert lines[i + 1].startswith("- MUST:")
                break
        else:
            pytest.fail("Provenance comment not found in output")

    def test_no_trailing_document_end(self):
        """Output should NOT end with '...' marker."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert not result.rstrip("\n").endswith("...")

    def test_empty_string_quoted(self):
        """Empty strings should be double-quoted."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": ""},
            ],
        }
        result = emit_yaml_fragment(data, {})
        assert 'MUST: ""' in result

    def test_dash_space_list_items(self):
        """List items use '- ' (dash-space) prefix."""
        data = {
            "spec": "MySpec",
            "RETURN": [
                {"MUST": "work"},
                {"SHOULD": "be fast"},
            ],
        }
        result = emit_yaml_fragment(data, {})
        lines = result.split("\n")
        list_items = [l for l in lines if l.startswith("- ")]
        assert len(list_items) == 2


# ---------------------------------------------------------------------------
# query_command integration tests
# ---------------------------------------------------------------------------

class TestQueryCommand:
    """Integration tests for query_command."""

    def test_single_match_returns_fragment(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        (tmp_path / "test.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(["MySpec"], monkeypatch=monkeypatch)
        assert rc == 0
        assert err == ""
        assert out.startswith("---\n")
        assert "spec: MySpec" in out
        assert "MUST: work" in out

    def test_multiple_matches_returns_disambiguation(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        c1 = textwrap.dedent("""\
            ---
            description: File 1
            version: v1
            ---
            spec: pkg.Foo
            RETURN:
            - MUST: work
        """)
        c2 = textwrap.dedent("""\
            ---
            description: File 2
            version: v1
            ---
            spec: other.Foo
            RETURN:
            - MUST: work
        """)
        (tmp_path / "file1.yass.yaml").write_text(c1, encoding="utf-8")
        (tmp_path / "file2.yass.yaml").write_text(c2, encoding="utf-8")

        rc, out, err = _run(["Foo"], monkeypatch=monkeypatch)
        assert rc == 0
        assert err == ""
        lines = out.strip().split("\n")
        assert len(lines) == 2
        # Each line has tab-separated fields
        for line in lines:
            parts = line.split("\t")
            assert len(parts) == 3

    def test_no_match_returns_error(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        (tmp_path / "test.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(["NonExistent"], monkeypatch=monkeypatch)
        assert rc == 1
        assert out == ""
        assert QUERY_NO_MATCH in err

    def test_name_missing_returns_error(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        rc, out, err = _run([], monkeypatch=monkeypatch)
        assert rc == 2
        assert out == ""
        assert QUERY_NAME_MISSING in err

    def test_scope_file(self, tmp_path, monkeypatch):
        """Scope narrows search to specific file."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        c1 = textwrap.dedent("""\
            ---
            description: File 1
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work 1
        """)
        c2 = textwrap.dedent("""\
            ---
            description: File 2
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work 2
        """)
        (tmp_path / "file1.yass.yaml").write_text(c1, encoding="utf-8")
        (tmp_path / "file2.yass.yaml").write_text(c2, encoding="utf-8")

        # Search only in file1
        rc, out, err = _run(
            ["MySpec", str(tmp_path / "file1.yass.yaml")],
            monkeypatch=monkeypatch,
        )
        assert rc == 0
        assert "work 1" in out

    def test_scope_not_found(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        rc, out, err = _run(
            ["MySpec", str(tmp_path / "nonexistent")],
            monkeypatch=monkeypatch,
        )
        assert rc == 2
        assert QUERY_SCOPE_NOT_FOUND in err

    def test_scope_empty(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        empty_dir = tmp_path / "empty"
        empty_dir.mkdir()

        rc, out, err = _run(
            ["MySpec", str(empty_dir)],
            monkeypatch=monkeypatch,
        )
        assert rc == 2
        assert QUERY_SCOPE_EMPTY in err

    def test_scope_colon_in_path(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        rc, out, err = _run(
            ["MySpec", "some:path"],
            monkeypatch=monkeypatch,
        )
        assert rc == 2
        assert PATH_COLON_IN_PATH in err

    def test_conforms_inlining_in_output(self, tmp_path, monkeypatch):
        """Single match with CONFORMS ref should inline the referenced obligations."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        other = textwrap.dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: OtherSpec
            RETURN:
            - MUST: return a value
        """)
        (tmp_path / "other.yass.yaml").write_text(other, encoding="utf-8")

        main = textwrap.dedent("""\
            ---
            description: Main
            version: v1
            ---
            spec: MainSpec
            RETURN:
            - CONFORMS: ./other@OtherSpec::RETURN
        """)
        (tmp_path / "main.yass.yaml").write_text(main, encoding="utf-8")

        rc, out, err = _run(["MainSpec"], monkeypatch=monkeypatch)
        assert rc == 0
        assert err == ""
        # The output should contain the inlined obligation
        assert "MUST: return a value" in out
        # Should contain provenance comment
        assert "# CONFORMS: ./other@OtherSpec::RETURN" in out

    def test_conforms_unresolved_error(self, tmp_path, monkeypatch):
        """Unresolvable CONFORMS ref should error and not emit fragment."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        main = textwrap.dedent("""\
            ---
            description: Main
            version: v1
            ---
            spec: MainSpec
            RETURN:
            - CONFORMS: ./nonexistent@MissingSpec::RETURN
        """)
        (tmp_path / "main.yass.yaml").write_text(main, encoding="utf-8")

        rc, out, err = _run(["MainSpec"], monkeypatch=monkeypatch)
        assert rc == 1
        assert out == ""
        assert QUERY_CONFORMS_UNRESOLVED in err

    def test_multi_match_no_stderr(self, tmp_path, monkeypatch):
        """Multiple matches should NOT write to stderr."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        c1 = textwrap.dedent("""\
            ---
            description: File 1
            version: v1
            ---
            spec: pkg.Foo
            RETURN:
            - MUST: work
        """)
        c2 = textwrap.dedent("""\
            ---
            description: File 2
            version: v1
            ---
            spec: other.Foo
            RETURN:
            - MUST: work
        """)
        (tmp_path / "file1.yass.yaml").write_text(c1, encoding="utf-8")
        (tmp_path / "file2.yass.yaml").write_text(c2, encoding="utf-8")

        rc, out, err = _run(["Foo"], monkeypatch=monkeypatch)
        assert rc == 0
        assert err == ""

    def test_multi_match_sorted_by_path(self, tmp_path, monkeypatch):
        """Multi-match rows are sorted by file path."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        c1 = textwrap.dedent("""\
            ---
            description: File Z
            version: v1
            ---
            spec: z.Foo
            RETURN:
            - MUST: work
        """)
        c2 = textwrap.dedent("""\
            ---
            description: File A
            version: v1
            ---
            spec: a.Foo
            RETURN:
            - MUST: work
        """)
        (tmp_path / "z_file.yass.yaml").write_text(c1, encoding="utf-8")
        (tmp_path / "a_file.yass.yaml").write_text(c2, encoding="utf-8")

        rc, out, err = _run(["Foo"], monkeypatch=monkeypatch)
        assert rc == 0
        lines = out.strip().split("\n")
        assert len(lines) == 2
        # a_file should come before z_file
        assert "a_file" in lines[0].split("\t")[0]
        assert "z_file" in lines[1].split("\t")[0]

    def test_scope_validated_before_name_lookup(self, tmp_path, monkeypatch):
        """Scope validation happens before name lookup."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        rc, out, err = _run(
            ["AnyName", str(tmp_path / "nonexistent_dir")],
            monkeypatch=monkeypatch,
        )
        assert rc == 2
        assert QUERY_SCOPE_NOT_FOUND in err

    def test_no_host_file_header(self, tmp_path, monkeypatch):
        """Output fragment should NOT include a host file header."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        (tmp_path / "test.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(["MySpec"], monkeypatch=monkeypatch)
        assert rc == 0
        # First line should be --- (document start marker)
        lines = out.split("\n")
        assert lines[0] == "---"
        # No file path in the output
        assert "test.yass.yaml" not in out

    def test_multi_match_no_truncation(self, tmp_path, monkeypatch):
        """Multi-match disambiguation rows are never truncated."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)

        long_desc = "A" * 200  # Very long description
        c1 = textwrap.dedent(f"""\
            ---
            description: {long_desc}
            version: v1
            ---
            spec: pkg.Foo
            RETURN:
            - MUST: work
        """)
        c2 = textwrap.dedent(f"""\
            ---
            description: {long_desc}
            version: v1
            ---
            spec: other.Foo
            RETURN:
            - MUST: work
        """)
        (tmp_path / "file1.yass.yaml").write_text(c1, encoding="utf-8")
        (tmp_path / "file2.yass.yaml").write_text(c2, encoding="utf-8")

        rc, out, err = _run(["Foo"], monkeypatch=monkeypatch)
        assert rc == 0
        lines = out.strip().split("\n")
        for line in lines:
            parts = line.split("\t")
            assert long_desc in parts[2]  # Full description, not truncated

    def test_whitespace_name_no_match(self, tmp_path, monkeypatch):
        """Name with whitespace returns no match, not blank error."""
        monkeypatch.chdir(tmp_path)
        _make_project(tmp_path)
        content = textwrap.dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: MySpec
            RETURN:
            - MUST: work
        """)
        (tmp_path / "test.yass.yaml").write_text(content, encoding="utf-8")

        rc, out, err = _run(["My Spec"], monkeypatch=monkeypatch)
        assert rc == 1
        assert QUERY_NO_MATCH in err
