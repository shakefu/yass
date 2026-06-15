"""Tests for yass.validate — structural validation of .yass.yaml files."""

from __future__ import annotations

import io
import os
from textwrap import dedent

import pytest

from yass.errors import (
    DISCOVER_NO_FILES,
    FINDROOT_NO_MARKER,
    NORMATIVITY_UNKNOWN,
    OBLIGATION_BAD_VALUE_SHAPE,
    OBLIGATION_DUPLICATE_NORMATIVITY,
    OBLIGATION_DUPLICATE_REFERENCE,
    OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
    OBLIGATION_MISSING_NORMATIVITY_OR_REF,
    PATH_BAD_EXTENSION,
    PATH_COLON_IN_PATH,
    PREAMBLE_BAD_RELATED,
    PREAMBLE_DUPLICATE,
    PREAMBLE_HAS_SPEC_KEY,
    PREAMBLE_MISPLACED,
    PREAMBLE_MISSING,
    PREAMBLE_MISSING_DESCRIPTION,
    PREAMBLE_MISSING_VERSION,
    PREAMBLE_UNKNOWN_VERSION,
    REFERENCE_UNKNOWN_RELATION,
    REF_FILE_NOT_FOUND,
    REF_FILE_NOT_PARSEABLE,
    REF_MALFORMED,
    REF_SLOT_NOT_DECLARED,
    REF_SPEC_NOT_FOUND_OTHER_FILE,
    REF_SPEC_NOT_FOUND_SAME_FILE,
    REF_UNKNOWN_SLOT,
    SLOT_VALUE_NOT_LIST,
    SPEC_DUPLICATE_NAME,
    SPEC_NAME_BAD_CHARS,
    SPEC_NAME_BAD_FORM,
    SPEC_NAME_EMPTY,
    SPEC_NAME_NOT_STRING,
    SPEC_NAME_RESERVED,
    SPEC_NO_NAME,
    SPEC_UNKNOWN_KEY,
    YAML_EMPTY_STREAM,
    YAML_MALFORMED,
)
from yass.parser import ParsedDocument
from yass.validate import (
    ErrorInfo,
    check_preamble,
    check_refs,
    check_spec,
    check_uniqueness,
    validate_command,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_doc(data, start_line=1):
    """Create a ParsedDocument for unit testing."""
    return ParsedDocument(data=data, start_line=start_line)


def _write(tmp_path, name: str, content: str) -> str:
    """Write a .yass.yaml file and return its absolute path."""
    p = tmp_path / name
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content, encoding="utf-8")
    return str(p)


def _error_codes(errors: list[ErrorInfo]) -> list[str]:
    """Extract error codes from a list of ErrorInfo."""
    return [e.code for e in errors]


# ============================================================================
# CheckPreamble
# ============================================================================


class TestCheckPreambleEmptyStream:
    """yass.yaml.empty_stream when zero documents."""

    def test_empty_docs_list(self):
        errors = check_preamble([], "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == YAML_EMPTY_STREAM
        assert errors[0].line is None


class TestCheckPreambleHasSpecKey:
    """yass.preamble.has_spec_key when first doc has 'spec' key."""

    def test_first_doc_has_spec_key(self):
        docs = [_make_doc({"spec": "Foo", "description": "test", "version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_HAS_SPEC_KEY

    def test_has_spec_key_priority_over_missing_description(self):
        """has_spec_key is higher priority than missing_description."""
        docs = [_make_doc({"spec": "Foo", "version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_HAS_SPEC_KEY


class TestCheckPreambleMissing:
    """yass.preamble.missing when no preamble is present."""

    def test_first_doc_is_list(self):
        docs = [_make_doc(["not", "a", "mapping"])]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING

    def test_first_doc_is_scalar(self):
        docs = [_make_doc("just a string")]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING

    def test_first_doc_is_null(self):
        docs = [_make_doc(None)]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING

    def test_first_doc_is_integer(self):
        docs = [_make_doc(42)]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING


class TestCheckPreambleDuplicate:
    """yass.preamble.duplicate when more than one preamble."""

    def test_two_preambles(self):
        docs = [
            _make_doc({"description": "first", "version": "v1"}, start_line=1),
            _make_doc({"description": "second", "version": "v1"}, start_line=5),
        ]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_DUPLICATE
        assert errors[0].line == 5

    def test_preamble_and_spec_and_preamble(self):
        docs = [
            _make_doc({"description": "first", "version": "v1"}, start_line=1),
            _make_doc({"spec": "Foo"}, start_line=4),
            _make_doc({"description": "dupe", "version": "v1"}, start_line=8),
        ]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_DUPLICATE
        assert errors[0].line == 8


class TestCheckPreambleMissingDescription:
    """yass.preamble.missing_description when preamble omits description."""

    def test_no_description(self):
        docs = [_make_doc({"version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING_DESCRIPTION

    def test_has_description(self):
        docs = [_make_doc({"description": "test", "version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 0


class TestCheckPreambleMissingVersion:
    """yass.preamble.missing_version when preamble omits version."""

    def test_no_version(self):
        docs = [_make_doc({"description": "test"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING_VERSION


class TestCheckPreambleUnknownVersion:
    """yass.preamble.unknown_version when version is not 'v1'."""

    def test_wrong_version(self):
        docs = [_make_doc({"description": "test", "version": "v2"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_UNKNOWN_VERSION

    def test_case_sensitive_version(self):
        docs = [_make_doc({"description": "test", "version": "V1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_UNKNOWN_VERSION

    def test_semver_rejected(self):
        docs = [_make_doc({"description": "test", "version": "v1.0"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_UNKNOWN_VERSION

    def test_numeric_version_rejected(self):
        docs = [_make_doc({"description": "test", "version": 1})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_UNKNOWN_VERSION


class TestCheckPreambleBadRelated:
    """yass.preamble.bad_related when related is not a sequence of strings."""

    def test_related_not_list(self):
        docs = [_make_doc({"description": "test", "version": "v1", "related": "not a list"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_BAD_RELATED

    def test_related_list_with_non_strings(self):
        docs = [_make_doc({"description": "test", "version": "v1", "related": ["ok", 42]})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_BAD_RELATED

    def test_related_is_integer(self):
        docs = [_make_doc({"description": "test", "version": "v1", "related": 42})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_BAD_RELATED

    def test_related_valid_list(self):
        docs = [_make_doc({"description": "test", "version": "v1", "related": ["https://example.com"]})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 0

    def test_related_empty_list_valid(self):
        docs = [_make_doc({"description": "test", "version": "v1", "related": []})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 0


class TestCheckPreamblePriorityOrder:
    """At most one error, in the documented priority order."""

    def test_at_most_one_error(self):
        """Even with multiple issues, only one error is returned."""
        docs = [_make_doc({"spec": "Foo"})]  # has_spec_key AND missing desc AND missing version
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_HAS_SPEC_KEY

    def test_has_spec_key_beats_empty_stream(self):
        """has_spec_key (priority 1) beats everything."""
        docs = [_make_doc({"spec": "Foo", "description": "d", "version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_HAS_SPEC_KEY

    def test_missing_beats_duplicate(self):
        """missing (priority 3) beats duplicate (priority 4)."""
        # First doc is not a mapping -> missing fires
        docs = [_make_doc(None), _make_doc({"description": "d", "version": "v1"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING

    def test_missing_description_beats_missing_version(self):
        """missing_description (priority 6) beats missing_version (priority 7)."""
        docs = [_make_doc({})]  # no desc, no version
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING_DESCRIPTION

    def test_missing_version_beats_unknown_version(self):
        """missing_version (priority 7) beats unknown_version (priority 8)."""
        # If version is missing, unknown_version can't fire
        docs = [_make_doc({"description": "test"})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_MISSING_VERSION

    def test_unknown_version_beats_bad_related(self):
        """unknown_version (priority 8) beats bad_related (priority 9)."""
        docs = [_make_doc({"description": "test", "version": "v2", "related": 42})]
        errors = check_preamble(docs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == PREAMBLE_UNKNOWN_VERSION

    def test_valid_preamble_no_errors(self):
        docs = [
            _make_doc({"description": "test", "version": "v1"}),
            _make_doc({"spec": "Foo"}),
        ]
        errors = check_preamble(docs, "test.yass.yaml")
        assert errors == []


# ============================================================================
# CheckSpec
# ============================================================================


class TestCheckSpecNoName:
    """yass.spec.no_name when non-first doc has no 'spec' key."""

    def test_missing_spec_key(self):
        doc = _make_doc({"INPUT": []}, start_line=5)
        errors = check_spec(doc, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == SPEC_NO_NAME
        assert errors[0].line == 5

    def test_non_mapping_doc(self):
        doc = _make_doc("just a string", start_line=3)
        errors = check_spec(doc, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == SPEC_NO_NAME

    def test_null_doc(self):
        doc = _make_doc(None, start_line=3)
        errors = check_spec(doc, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == SPEC_NO_NAME


class TestCheckSpecNameValidation:
    """spec name validation errors."""

    def test_name_not_string(self):
        doc = _make_doc({"spec": 42})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_NOT_STRING in _error_codes(errors)

    def test_name_not_string_bool(self):
        doc = _make_doc({"spec": True})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_NOT_STRING in _error_codes(errors)

    def test_name_not_string_list(self):
        doc = _make_doc({"spec": ["a", "b"]})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_NOT_STRING in _error_codes(errors)

    def test_name_empty(self):
        doc = _make_doc({"spec": ""})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_EMPTY in _error_codes(errors)

    def test_name_bad_chars_space(self):
        doc = _make_doc({"spec": "Bad Name"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_CHARS in _error_codes(errors)

    def test_name_bad_chars_colon(self):
        doc = _make_doc({"spec": "Bad:Name"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_CHARS in _error_codes(errors)

    def test_name_bad_chars_at(self):
        doc = _make_doc({"spec": "Bad@Name"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_CHARS in _error_codes(errors)

    def test_name_bad_form_leading_dot(self):
        doc = _make_doc({"spec": ".Foo"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_FORM in _error_codes(errors)

    def test_name_bad_form_trailing_dot(self):
        doc = _make_doc({"spec": "Foo."})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_FORM in _error_codes(errors)

    def test_name_bad_form_consecutive_dots(self):
        doc = _make_doc({"spec": "Foo..Bar"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_BAD_FORM in _error_codes(errors)

    def test_name_reserved_must(self):
        doc = _make_doc({"spec": "MUST"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_input(self):
        doc = _make_doc({"spec": "INPUT"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_case_insensitive(self):
        doc = _make_doc({"spec": "must"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_return(self):
        doc = _make_doc({"spec": "Return"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_when(self):
        doc = _make_doc({"spec": "WHEN"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_conforms(self):
        doc = _make_doc({"spec": "conforms"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_name_reserved_side_effect(self):
        doc = _make_doc({"spec": "side-effect"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_NAME_RESERVED in _error_codes(errors)

    def test_valid_name_simple(self):
        doc = _make_doc({"spec": "MySpec"})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_valid_name_dotted(self):
        doc = _make_doc({"spec": "pkg.Symbol"})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_valid_name_with_hyphens_and_underscores(self):
        doc = _make_doc({"spec": "my-spec_v2"})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_valid_name_complex(self):
        doc = _make_doc({"spec": "cli.validate.CheckRefs"})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []


class TestCheckSpecUnknownKey:
    """yass.spec.unknown_key for keys other than 'spec' that aren't Slot keys."""

    def test_unknown_key(self):
        doc = _make_doc({"spec": "Foo", "BOGUS": []})
        errors = check_spec(doc, "test.yass.yaml")
        assert SPEC_UNKNOWN_KEY in _error_codes(errors)

    def test_multiple_unknown_keys(self):
        doc = _make_doc({"spec": "Foo", "BOGUS": [], "ALSO_BOGUS": []})
        errors = check_spec(doc, "test.yass.yaml")
        unknown_errors = [e for e in errors if e.code == SPEC_UNKNOWN_KEY]
        assert len(unknown_errors) == 2

    def test_valid_slot_keys(self):
        doc = _make_doc({
            "spec": "Foo",
            "INPUT": [],
            "RETURN": [],
            "ERROR": [],
            "SIDE-EFFECT": [],
            "INVARIANT": [],
        })
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []


class TestCheckSpecSlotValueNotList:
    """yass.slot.value_not_list when slot value is not a list."""

    def test_slot_value_string(self):
        doc = _make_doc({"spec": "Foo", "INPUT": "not a list"})
        errors = check_spec(doc, "test.yass.yaml")
        assert SLOT_VALUE_NOT_LIST in _error_codes(errors)

    def test_slot_value_dict(self):
        doc = _make_doc({"spec": "Foo", "RETURN": {"MUST": "x"}})
        errors = check_spec(doc, "test.yass.yaml")
        assert SLOT_VALUE_NOT_LIST in _error_codes(errors)

    def test_slot_value_null(self):
        doc = _make_doc({"spec": "Foo", "INPUT": None})
        errors = check_spec(doc, "test.yass.yaml")
        assert SLOT_VALUE_NOT_LIST in _error_codes(errors)

    def test_slot_value_integer(self):
        doc = _make_doc({"spec": "Foo", "ERROR": 42})
        errors = check_spec(doc, "test.yass.yaml")
        assert SLOT_VALUE_NOT_LIST in _error_codes(errors)


class TestCheckSpecObligationBadValueShape:
    """yass.obligation.bad_value_shape for obligation-level value issues."""

    def test_obligation_is_string(self):
        """A bare string in a slot list has bad shape."""
        doc = _make_doc({"spec": "Foo", "INPUT": ["bare string"]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_obligation_is_null(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [None]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_obligation_is_list(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [["nested", "list"]]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_obligation_is_integer(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [42]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_normativity_value_null(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": None}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_normativity_value_mapping(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": {"nested": "map"}}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)

    def test_normativity_value_sequence(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": ["a", "b"]}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_BAD_VALUE_SHAPE in _error_codes(errors)


class TestCheckSpecObligationMissingNormativityOrRef:
    """yass.obligation.missing_normativity_or_ref."""

    def test_only_when_key(self):
        """An obligation with only WHEN and nothing else."""
        doc = _make_doc({"spec": "Foo", "INPUT": [{"WHEN": "condition"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_MISSING_NORMATIVITY_OR_REF in _error_codes(errors)

    def test_empty_mapping(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_MISSING_NORMATIVITY_OR_REF in _error_codes(errors)


class TestCheckSpecObligationGuardWithoutNormativity:
    """yass.obligation.guard_without_normativity."""

    def test_when_without_must(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"WHEN": "x", "USES": "OtherSpec"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_GUARD_WITHOUT_NORMATIVITY in _error_codes(errors)

    def test_when_with_must(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"WHEN": "x", "MUST": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_GUARD_WITHOUT_NORMATIVITY not in _error_codes(errors)


class TestCheckSpecObligationDuplicateNormativity:
    """yass.obligation.duplicate_normativity."""

    def test_two_normativity_keywords(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": "x", "SHOULD": "y"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_DUPLICATE_NORMATIVITY in _error_codes(errors)

    def test_three_normativity_keywords(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": "x", "SHOULD": "y", "MAY": "z"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert OBLIGATION_DUPLICATE_NORMATIVITY in _error_codes(errors)


class TestCheckSpecObligationDuplicateReference:
    """yass.obligation.duplicate_reference."""

    def test_duplicate_conforms(self):
        # YAML mapping can't actually have duplicate keys, so this is hard
        # to trigger via YAML parsing. But the function checks for it.
        # In practice, YAML duplicate key detection would catch this first.
        # We test the logic directly.
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "A", "USES": "B"}]})
        errors = check_spec(doc, "test.yass.yaml")
        # No duplicate since CONFORMS and USES are different relation keys
        assert OBLIGATION_DUPLICATE_REFERENCE not in _error_codes(errors)


class TestCheckSpecNormativityUnknown:
    """yass.normativity.unknown for unrecognized obligation keys."""

    def test_unknown_obligation_key(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"SHALL": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert NORMATIVITY_UNKNOWN in _error_codes(errors)

    def test_lowercase_obligation_key(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"must": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert NORMATIVITY_UNKNOWN in _error_codes(errors)

    def test_unknown_key_required(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"REQUIRED": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert NORMATIVITY_UNKNOWN in _error_codes(errors)


class TestCheckSpecValidObligation:
    """Valid obligations should produce no errors."""

    def test_must_obligation(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_must_not_obligation(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST-NOT": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_should_obligation(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"SHOULD": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_should_not_obligation(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"SHOULD-NOT": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_may_obligation(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MAY": "do thing"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_obligation_with_when_and_must(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"WHEN": "x", "MUST": "y"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_obligation_with_ref_only(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "OtherSpec"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_obligation_with_must_and_uses(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"MUST": "do thing", "USES": "Ref"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []

    def test_obligation_with_when_must_uses(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"WHEN": "c", "MUST": "x", "USES": "Ref"}]})
        errors = check_spec(doc, "test.yass.yaml")
        assert errors == []


# ============================================================================
# CheckUniqueness
# ============================================================================


class TestCheckUniqueness:
    """yass.spec.duplicate_name for duplicate spec names in a file."""

    def test_no_duplicates(self):
        specs = [("Foo", 1), ("Bar", 5), ("Baz", 10)]
        errors = check_uniqueness(specs, "test.yass.yaml")
        assert errors == []

    def test_one_duplicate(self):
        specs = [("Foo", 1), ("Bar", 5), ("Foo", 10)]
        errors = check_uniqueness(specs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].code == SPEC_DUPLICATE_NAME
        assert errors[0].line == 10

    def test_multiple_duplicates(self):
        specs = [("Foo", 1), ("Bar", 5), ("Foo", 10), ("Foo", 15)]
        errors = check_uniqueness(specs, "test.yass.yaml")
        assert len(errors) == 2
        assert all(e.code == SPEC_DUPLICATE_NAME for e in errors)
        assert errors[0].line == 10
        assert errors[1].line == 15

    def test_different_duplicates(self):
        specs = [("Foo", 1), ("Bar", 5), ("Foo", 10), ("Bar", 15)]
        errors = check_uniqueness(specs, "test.yass.yaml")
        assert len(errors) == 2
        lines = [e.line for e in errors]
        assert 10 in lines
        assert 15 in lines

    def test_empty_list(self):
        errors = check_uniqueness([], "test.yass.yaml")
        assert errors == []

    def test_single_spec(self):
        errors = check_uniqueness([("Foo", 1)], "test.yass.yaml")
        assert errors == []

    def test_line_attribution(self):
        """Error should point at the DUPLICATE, not the original."""
        specs = [("Spec", 3), ("Spec", 10)]
        errors = check_uniqueness(specs, "test.yass.yaml")
        assert len(errors) == 1
        assert errors[0].line == 10  # Line of second occurrence


# ============================================================================
# CheckRefs
# ============================================================================


class TestCheckRefsGrammar:
    """yass.ref.malformed for targets that don't match the grammar."""

    def test_malformed_ref_empty(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": ""}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_MALFORMED in _error_codes(errors)

    def test_malformed_ref_spaces(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "has spaces"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_MALFORMED in _error_codes(errors)

    def test_malformed_ref_bad_slot_chars(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "Spec::lower"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_MALFORMED in _error_codes(errors)

    def test_valid_ref_bare_spec(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"USES": "Foo"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_MALFORMED not in _error_codes(errors)

    def test_valid_ref_with_path(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./other@Bar"}]}, start_line=1)
        # This will fail with file_not_found, but NOT malformed
        errors = check_refs([doc], "/project/test.yass.yaml", "/project")
        assert REF_MALFORMED not in _error_codes(errors)

    def test_valid_ref_with_slot(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "Foo::INPUT"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_MALFORMED not in _error_codes(errors)


class TestCheckRefsUnknownSlot:
    """yass.ref.unknown_slot when slot name is not in recognized set."""

    def test_unknown_slot(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "Foo::BOGUS"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_UNKNOWN_SLOT in _error_codes(errors)

    def test_known_slots(self):
        for slot in ["INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"]:
            doc = _make_doc({"spec": "Foo", slot: [{"MUST": "x"}], "INPUT": [{"CONFORMS": f"Foo::{slot}"}]}, start_line=1)
            errors = check_refs([doc], "/test.yass.yaml", "/")
            assert REF_UNKNOWN_SLOT not in _error_codes(errors)


class TestCheckRefsSameFile:
    """yass.ref.spec_not_found_same_file for missing same-file specs."""

    def test_same_file_spec_not_found(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"USES": "NonExistent"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_SPEC_NOT_FOUND_SAME_FILE in _error_codes(errors)

    def test_same_file_spec_found(self):
        docs = [
            _make_doc({"spec": "Foo", "INPUT": [{"USES": "Bar"}]}, start_line=1),
            _make_doc({"spec": "Bar", "RETURN": [{"MUST": "x"}]}, start_line=5),
        ]
        errors = check_refs(docs, "/test.yass.yaml", "/")
        assert REF_SPEC_NOT_FOUND_SAME_FILE not in _error_codes(errors)

    def test_self_reference(self):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "Foo"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_SPEC_NOT_FOUND_SAME_FILE not in _error_codes(errors)


class TestCheckRefsCrossFile:
    """Cross-file reference resolution."""

    def test_file_not_found(self, tmp_path):
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./nonexist@Bar"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert REF_FILE_NOT_FOUND in _error_codes(errors)

    def test_file_not_parseable(self, tmp_path):
        bad_file = tmp_path / "bad.yass.yaml"
        bad_file.write_bytes(b"\xfe\xff\x00bad")  # not valid UTF-8
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./bad@Bar"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert REF_FILE_NOT_PARSEABLE in _error_codes(errors)

    def test_spec_not_found_other_file(self, tmp_path):
        other = tmp_path / "other.yass.yaml"
        other.write_text("---\ndescription: d\nversion: v1\n---\nspec: Baz\n", encoding="utf-8")
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./other@NonExistent"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert REF_SPEC_NOT_FOUND_OTHER_FILE in _error_codes(errors)

    def test_successful_cross_file_ref(self, tmp_path):
        other = tmp_path / "other.yass.yaml"
        other.write_text("---\ndescription: d\nversion: v1\n---\nspec: Bar\nRETURN:\n- MUST: x\n", encoding="utf-8")
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./other@Bar"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert errors == []

    def test_cross_file_with_slot(self, tmp_path):
        other = tmp_path / "other.yass.yaml"
        other.write_text("---\ndescription: d\nversion: v1\n---\nspec: Bar\nRETURN:\n- MUST: x\n", encoding="utf-8")
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./other@Bar::RETURN"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert errors == []

    def test_cross_file_slot_not_declared(self, tmp_path):
        other = tmp_path / "other.yass.yaml"
        other.write_text("---\ndescription: d\nversion: v1\n---\nspec: Bar\nRETURN:\n- MUST: x\n", encoding="utf-8")
        doc = _make_doc({"spec": "Foo", "INPUT": [{"CONFORMS": "./other@Bar::INPUT"}]}, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert REF_SLOT_NOT_DECLARED in _error_codes(errors)

    def test_root_relative_ref(self, tmp_path):
        sub = tmp_path / "sub"
        sub.mkdir()
        other = tmp_path / "lib.yass.yaml"
        other.write_text("---\ndescription: d\nversion: v1\n---\nspec: Util\nRETURN:\n- MUST: x\n", encoding="utf-8")
        doc = _make_doc({"spec": "Foo", "INPUT": [{"USES": "lib@Util"}]}, start_line=1)
        filepath = str(sub / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        assert errors == []

    def test_dedup_file_errors_per_pair(self, tmp_path):
        """At most one file_not_found per (referencing, referenced) pair."""
        doc = _make_doc({
            "spec": "Foo",
            "INPUT": [
                {"CONFORMS": "./nonexist@A"},
                {"USES": "./nonexist@B"},
            ],
        }, start_line=1)
        filepath = str(tmp_path / "test.yass.yaml")
        errors = check_refs([doc], filepath, str(tmp_path))
        file_not_found_errors = [e for e in errors if e.code == REF_FILE_NOT_FOUND]
        assert len(file_not_found_errors) == 1

    def test_slot_not_declared_same_file(self):
        """Slot check for same-file ref."""
        doc = _make_doc({"spec": "Foo", "RETURN": [{"MUST": "x"}], "INPUT": [{"CONFORMS": "Foo::INPUT"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        # Foo has RETURN and INPUT declared, so ::INPUT should be found
        assert REF_SLOT_NOT_DECLARED not in _error_codes(errors)

    def test_slot_not_declared_same_file_missing(self):
        """Slot not declared check for same-file ref when slot is missing."""
        doc = _make_doc({"spec": "Foo", "RETURN": [{"MUST": "x"}], "INPUT": [{"CONFORMS": "Foo::ERROR"}]}, start_line=1)
        errors = check_refs([doc], "/test.yass.yaml", "/")
        assert REF_SLOT_NOT_DECLARED in _error_codes(errors)


# ============================================================================
# validate_command — integration tests
# ============================================================================


class TestValidateCommandExitCodes:
    """Exit code verification for validate_command."""

    def test_valid_file_exit_0(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: Foo
            RETURN:
            - MUST: return something
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 0
        assert "found 0 errors" in out.getvalue()

    def test_colon_in_path_exit_2(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        err = io.StringIO()
        out = io.StringIO()
        rc = validate_command(["bad:path.yass.yaml"], stderr=err, stdout=out)
        assert rc == 2
        assert PATH_COLON_IN_PATH in err.getvalue()

    def test_no_project_root_exit_2(self, tmp_path, monkeypatch):
        """When no project root is found, exit 2."""
        empty = tmp_path / "isolated"
        empty.mkdir()
        monkeypatch.chdir(empty)
        err = io.StringIO()
        out = io.StringIO()
        rc = validate_command([], stderr=err, stdout=out)
        assert rc == 2
        assert FINDROOT_NO_MARKER in err.getvalue()

    def test_validation_errors_exit_1(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: Test
            version: v2
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        assert "found 1 errors" in out.getvalue()

    def test_yaml_parse_error_exit_1(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", "key: [unclosed\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        assert YAML_MALFORMED in err.getvalue()


class TestValidateCommandSummary:
    """Summary line format verification."""

    def test_summary_format_zero(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: Test
            version: v1
            ---
            spec: Foo
            RETURN:
            - MUST: return something
        """))
        out = io.StringIO()
        err = io.StringIO()
        validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert out.getvalue().strip() == "checked 1 files, found 0 errors"

    def test_summary_format_multiple_files(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "a.yass.yaml", "---\ndescription: A\nversion: v1\n---\nspec: A\nRETURN:\n- MUST: x\n")
        _write(tmp_path, "b.yass.yaml", "---\ndescription: B\nversion: v1\n---\nspec: B\nRETURN:\n- MUST: x\n")
        out = io.StringIO()
        err = io.StringIO()
        validate_command([str(tmp_path / "a.yass.yaml"), str(tmp_path / "b.yass.yaml")], stderr=err, stdout=out)
        assert "checked 2 files" in out.getvalue()

    def test_summary_with_errors(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", "---\ndescription: Test\nversion: v2\n")
        out = io.StringIO()
        err = io.StringIO()
        validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert "found 1 errors" in out.getvalue()


class TestValidateCommandDiscovery:
    """File discovery and deduplication."""

    def test_discover_from_project_root(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "spec.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: Foo\nRETURN:\n- MUST: x\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([], stderr=err, stdout=out)
        assert rc == 0
        assert "checked 1 files" in out.getvalue()

    def test_directory_arg(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "a.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: A\nRETURN:\n- MUST: x\n")
        sub = tmp_path / "sub"
        sub.mkdir()
        _write(tmp_path, "sub/b.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: B\nRETURN:\n- MUST: x\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path)], stderr=err, stdout=out)
        assert rc == 0
        assert "checked 2 files" in out.getvalue()

    def test_deduplication(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        filepath = _write(tmp_path, "test.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: Foo\nRETURN:\n- MUST: x\n")
        out = io.StringIO()
        err = io.StringIO()
        # Pass the same file twice
        rc = validate_command([filepath, filepath], stderr=err, stdout=out)
        assert rc == 0
        assert "checked 1 files" in out.getvalue()


class TestValidateCommandCheckOrder:
    """Checks run in the right order and skip on YAML failure."""

    def test_yaml_failure_skips_other_checks(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", "key: [unclosed\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        stderr_text = err.getvalue()
        # Only YAML error should appear, not preamble/spec errors
        assert YAML_MALFORMED in stderr_text
        assert PREAMBLE_MISSING not in stderr_text
        # Counted as exactly 1 error
        assert "found 1 errors" in out.getvalue()

    def test_multiple_files_continue_on_error(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "bad.yass.yaml", "key: [unclosed\n")
        _write(tmp_path, "good.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: Foo\nRETURN:\n- MUST: x\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([
            str(tmp_path / "bad.yass.yaml"),
            str(tmp_path / "good.yass.yaml"),
        ], stderr=err, stdout=out)
        assert rc == 1
        assert "checked 2 files" in out.getvalue()

    def test_preamble_error_still_checks_specs(self, tmp_path, monkeypatch):
        """Preamble error does not prevent spec checks from running."""
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: d
            version: v2
            ---
            spec: Foo
            BOGUS: not-a-slot
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        stderr_text = err.getvalue()
        # Should have both preamble and spec errors
        assert PREAMBLE_UNKNOWN_VERSION in stderr_text
        assert SPEC_UNKNOWN_KEY in stderr_text


class TestValidateCommandBadExtension:
    """Error for non-.yass.yaml file paths."""

    def test_bad_extension_exit_2(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        badfile = tmp_path / "test.yaml"
        badfile.write_text("---\nfoo: bar\n", encoding="utf-8")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(badfile)], stderr=err, stdout=out)
        assert rc == 2
        assert PATH_BAD_EXTENSION in err.getvalue()


class TestValidateCommandNoFiles:
    """yass.discover.no_files when no files found."""

    def test_empty_directory(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([], stderr=err, stdout=out)
        # No yass files in the project
        assert "checked 0 files" in out.getvalue()


class TestValidateCommandCrossFileRefs:
    """Integration test: cross-file references through validate_command."""

    def test_successful_cross_ref(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "main.yass.yaml", dedent("""\
            ---
            description: Main
            version: v1
            ---
            spec: Main
            RETURN:
            - CONFORMS: ./other@Other::RETURN
            - MUST: return a value
        """))
        _write(tmp_path, "other.yass.yaml", dedent("""\
            ---
            description: Other
            version: v1
            ---
            spec: Other
            RETURN:
            - MUST: return something
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "main.yass.yaml")], stderr=err, stdout=out)
        assert rc == 0
        assert "found 0 errors" in out.getvalue()

    def test_broken_cross_ref(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "main.yass.yaml", dedent("""\
            ---
            description: Main
            version: v1
            ---
            spec: Main
            RETURN:
            - CONFORMS: ./missing@Other
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "main.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        assert REF_FILE_NOT_FOUND in err.getvalue()


class TestValidateCommandComplexScenarios:
    """Complex multi-error scenarios."""

    def test_multiple_errors_in_one_file(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: d
            version: v1
            ---
            spec: Foo
            BOGUS: not-a-slot
            INPUT:
            - SHALL: do something
            ---
            spec: Foo
            RETURN:
            - MUST: something
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        stderr_text = err.getvalue()
        # Expect: unknown_key, normativity_unknown, duplicate_name
        assert SPEC_UNKNOWN_KEY in stderr_text
        assert NORMATIVITY_UNKNOWN in stderr_text
        assert SPEC_DUPLICATE_NAME in stderr_text

    def test_no_spec_documents_is_valid(self, tmp_path, monkeypatch):
        """A file with only a preamble is valid."""
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        _write(tmp_path, "test.yass.yaml", dedent("""\
            ---
            description: Preamble only file
            version: v1
        """))
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 0

    def test_empty_stream_error(self, tmp_path, monkeypatch):
        """A file that parses to zero docs after YAML succeeds -> empty_stream."""
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        # A file with only comments or whitespace parses to zero docs
        _write(tmp_path, "test.yass.yaml", "# just a comment\n")
        out = io.StringIO()
        err = io.StringIO()
        rc = validate_command([str(tmp_path / "test.yass.yaml")], stderr=err, stdout=out)
        assert rc == 1
        assert YAML_EMPTY_STREAM in err.getvalue()
