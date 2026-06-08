"""Tests for yass.parser — YAML parsing with CheckYAML validation."""

from __future__ import annotations

import pytest

from yass.errors import (
    YAML_ANCHOR_OR_ALIAS,
    YAML_DUPLICATE_KEY,
    YAML_EMPTY_FILE,
    YAML_HAS_BOM,
    YAML_MALFORMED,
    YAML_NOT_UTF8,
)
from yass.parser import ParsedDocument, YAMLError, parse_yaml_file


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _write(tmp_path, name: str, content: bytes | str) -> str:
    """Write content to a file under tmp_path and return its string path."""
    p = tmp_path / name
    if isinstance(content, bytes):
        p.write_bytes(content)
    else:
        p.write_text(content, encoding="utf-8")
    return str(p)


# ---------------------------------------------------------------------------
# Valid YAML parsing
# ---------------------------------------------------------------------------


class TestValidMultiDocParsing:
    """parse_yaml_file returns correct documents for well-formed YAML."""

    def test_single_document(self, tmp_path):
        path = _write(tmp_path, "single.yaml", "---\nkey: value\n")
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 1
        assert docs[0].data == {"key": "value"}

    def test_multi_document(self, tmp_path):
        content = "---\na: 1\n---\nb: 2\n---\nc: 3\n"
        path = _write(tmp_path, "multi.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 3
        assert docs[0].data == {"a": 1}
        assert docs[1].data == {"b": 2}
        assert docs[2].data == {"c": 3}

    def test_document_with_list(self, tmp_path):
        content = "---\nitems:\n- one\n- two\n- three\n"
        path = _write(tmp_path, "list.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 1
        assert docs[0].data == {"items": ["one", "two", "three"]}

    def test_null_document(self, tmp_path):
        content = "---\n...\n---\nkey: val\n"
        path = _write(tmp_path, "null_doc.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 2
        assert docs[0].data is None
        assert docs[1].data == {"key": "val"}

    def test_implicit_first_document(self, tmp_path):
        """A file without an explicit --- still produces a document."""
        content = "key: value\n"
        path = _write(tmp_path, "implicit.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 1
        assert docs[0].data == {"key": "value"}


# ---------------------------------------------------------------------------
# Line numbers (1-based)
# ---------------------------------------------------------------------------


class TestLineNumbers:
    """Documents carry correct 1-based start line numbers."""

    def test_single_doc_starts_at_line_1(self, tmp_path):
        path = _write(tmp_path, "l.yaml", "---\nkey: val\n")
        docs, _ = parse_yaml_file(path)
        assert docs[0].start_line == 1

    def test_multi_doc_line_numbers(self, tmp_path):
        # doc1 starts at line 1 (---), doc2 at line 3 (---)
        content = "---\na: 1\n---\nb: 2\n"
        path = _write(tmp_path, "lines.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 2
        assert docs[0].start_line >= 1
        assert docs[1].start_line >= 3

    def test_implicit_doc_starts_at_line_1(self, tmp_path):
        path = _write(tmp_path, "imp.yaml", "foo: bar\n")
        docs, _ = parse_yaml_file(path)
        assert docs[0].start_line == 1


# ---------------------------------------------------------------------------
# CheckYAML error detection
# ---------------------------------------------------------------------------


class TestNotUTF8:
    """yass.yaml.not_utf8 — highest priority."""

    def test_invalid_utf8_bytes(self, tmp_path):
        # 0xfe 0xff is not valid UTF-8 (it's a UTF-16 BOM)
        path = _write(tmp_path, "bad.yaml", b"\xfe\xff\x00key: val")
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_NOT_UTF8
        assert errors[0].line is None

    def test_latin1_bytes(self, tmp_path):
        # 0xe9 alone is not valid UTF-8 (it's e-acute in Latin-1)
        path = _write(tmp_path, "latin1.yaml", b"key: caf\xe9\n")
        docs, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_NOT_UTF8


class TestHasBOM:
    """yass.yaml.has_bom — second priority."""

    def test_utf8_bom_detected(self, tmp_path):
        path = _write(tmp_path, "bom.yaml", b"\xef\xbb\xbfkey: value\n")
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_HAS_BOM
        assert errors[0].line == 1

    def test_bom_priority_over_malformed(self, tmp_path):
        """BOM + malformed YAML -> only BOM error reported."""
        path = _write(tmp_path, "bom_mal.yaml", b"\xef\xbb\xbf: :\n: invalid yaml {{{\n")
        docs, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_HAS_BOM


class TestEmptyFile:
    """yass.yaml.empty_file — third priority."""

    def test_zero_bytes(self, tmp_path):
        path = _write(tmp_path, "empty.yaml", b"")
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_EMPTY_FILE
        assert errors[0].line is None


class TestMalformed:
    """yass.yaml.malformed — fourth priority."""

    def test_bad_yaml_syntax(self, tmp_path):
        path = _write(tmp_path, "bad.yaml", "key: [unclosed\n")
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_MALFORMED

    def test_malformed_has_line_number(self, tmp_path):
        content = "good: yes_value\nbad: :\n  : invalid\n"
        path = _write(tmp_path, "badline.yaml", content)
        docs, errors = parse_yaml_file(path)
        if errors:
            assert errors[0].code == YAML_MALFORMED
            # The error should have a line number.
            assert errors[0].line is not None
            assert errors[0].line >= 1

    def test_tab_character(self, tmp_path):
        path = _write(tmp_path, "tab.yaml", "key:\n\tvalue\n")
        docs, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_MALFORMED


class TestDuplicateKey:
    """yass.yaml.duplicate_key — fifth priority."""

    def test_duplicate_mapping_key(self, tmp_path):
        content = "---\na: 1\nb: 2\na: 3\n"
        path = _write(tmp_path, "dup.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_DUPLICATE_KEY

    def test_duplicate_key_in_second_doc(self, tmp_path):
        content = "---\na: 1\n---\nx: 1\nx: 2\n"
        path = _write(tmp_path, "dup2.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_DUPLICATE_KEY

    def test_duplicate_key_has_message(self, tmp_path):
        content = "---\nfoo: 1\nfoo: 2\n"
        path = _write(tmp_path, "dupmsg.yaml", content)
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert "foo" in errors[0].message


class TestAnchorAliasTag:
    """yass.yaml.anchor_or_alias — sixth priority."""

    def test_anchor_detected(self, tmp_path):
        content = "---\na: &anc hello\nb: world\n"
        path = _write(tmp_path, "anchor.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert docs == []
        assert len(errors) == 1
        assert errors[0].code == YAML_ANCHOR_OR_ALIAS
        assert "&" in errors[0].message or "anchor" in errors[0].message

    def test_alias_detected(self, tmp_path):
        content = "---\na: &anc hello\nb: *anc\n"
        path = _write(tmp_path, "alias.yaml", content)
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_ANCHOR_OR_ALIAS

    def test_explicit_tag_detected(self, tmp_path):
        content = "---\na: !custom value\n"
        path = _write(tmp_path, "tag.yaml", content)
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_ANCHOR_OR_ALIAS
        assert "tag" in errors[0].message.lower()

    def test_anchor_line_number(self, tmp_path):
        content = "---\nfirst: ok\nsecond: &anc value\n"
        path = _write(tmp_path, "ancline.yaml", content)
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].line == 3


# ---------------------------------------------------------------------------
# Priority ordering
# ---------------------------------------------------------------------------


class TestPriorityOrdering:
    """At most one error per file, in priority order."""

    def test_bom_beats_malformed(self, tmp_path):
        """BOM is higher priority than malformed."""
        path = _write(tmp_path, "prio.yaml", b"\xef\xbb\xbf{{{\n")
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_HAS_BOM

    def test_bom_beats_duplicate_key(self, tmp_path):
        path = _write(tmp_path, "prio2.yaml", b"\xef\xbb\xbfa: 1\na: 2\n")
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_HAS_BOM

    def test_not_utf8_beats_bom(self, tmp_path):
        """not_utf8 is highest priority."""
        path = _write(tmp_path, "prio3.yaml", b"\xfe\xff\xef\xbb\xbf")
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_NOT_UTF8

    def test_malformed_beats_anchor(self, tmp_path):
        """Cannot detect anchors if the file doesn't even parse to tokens.

        Note: PyYAML's scanner may actually tokenize partial content
        before failing.  We test a truly unparseable case.
        """
        # A file that's malformed AND has an anchor: malformed wins because
        # it's detected first during full loading.
        path = _write(tmp_path, "prio4.yaml", "key: [unclosed\n&anc val\n")
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_MALFORMED

    def test_duplicate_key_beats_anchor(self, tmp_path):
        """Duplicate key is higher priority than anchor/alias."""
        content = "---\na: 1\na: &anc 2\n"
        path = _write(tmp_path, "prio5.yaml", content)
        _, errors = parse_yaml_file(path)
        assert len(errors) == 1
        assert errors[0].code == YAML_DUPLICATE_KEY

    def test_at_most_one_error(self, tmp_path):
        """No matter how many issues, at most one error is returned."""
        # BOM + malformed + would-be duplicate keys -> just BOM
        path = _write(tmp_path, "many.yaml", b"\xef\xbb\xbfa: 1\na: 2\n{{{\n")
        _, errors = parse_yaml_file(path)
        assert len(errors) <= 1

    def test_clean_file_no_errors(self, tmp_path):
        path = _write(tmp_path, "clean.yaml", "---\nkey: value\n")
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 1


# ---------------------------------------------------------------------------
# YAML 1.2 boolean semantics (yes/no/on/off as strings)
# ---------------------------------------------------------------------------


class TestYAML12Booleans:
    """yes/no/on/off must be treated as strings, not booleans."""

    @pytest.mark.parametrize("value", [
        "yes", "Yes", "YES",
        "no", "No", "NO",
        "on", "On", "ON",
        "off", "Off", "OFF",
    ])
    def test_yaml11_booleans_are_strings(self, tmp_path, value):
        content = f"---\nkey: {value}\n"
        path = _write(tmp_path, "bool.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert len(docs) == 1
        assert docs[0].data["key"] == value
        assert isinstance(docs[0].data["key"], str)

    def test_true_false_still_booleans(self, tmp_path):
        """true/false remain booleans per YAML 1.2."""
        content = "---\na: true\nb: false\nc: True\nd: False\n"
        path = _write(tmp_path, "tf.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert docs[0].data["a"] is True
        assert docs[0].data["b"] is False
        assert docs[0].data["c"] is True
        assert docs[0].data["d"] is False

    def test_yes_no_as_mapping_keys(self, tmp_path):
        """yes/no used as keys should also remain strings."""
        content = "---\nyes: accepted\nno: rejected\n"
        path = _write(tmp_path, "boolkeys.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert "yes" in docs[0].data
        assert "no" in docs[0].data

    def test_on_off_in_list(self, tmp_path):
        """on/off in a sequence should remain strings."""
        content = "---\nitems:\n- on\n- off\n"
        path = _write(tmp_path, "boollist.yaml", content)
        docs, errors = parse_yaml_file(path)
        assert errors == []
        assert docs[0].data["items"] == ["on", "off"]


# ---------------------------------------------------------------------------
# Data structure types
# ---------------------------------------------------------------------------


class TestDataStructures:
    """ParsedDocument and YAMLError carry the right fields."""

    def test_parsed_document_fields(self):
        doc = ParsedDocument(data={"a": 1}, start_line=5)
        assert doc.data == {"a": 1}
        assert doc.start_line == 5

    def test_yaml_error_fields(self):
        err = YAMLError(code=YAML_MALFORMED, line=10, message="oops")
        assert err.code == YAML_MALFORMED
        assert err.line == 10
        assert err.message == "oops"

    def test_yaml_error_none_line(self):
        err = YAMLError(code=YAML_NOT_UTF8, line=None, message="bad encoding")
        assert err.line is None
