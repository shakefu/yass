"""YAML parser for yass files with CheckYAML validation.

Parses files as YAML 1.2 multi-document streams and detects CheckYAML
errors in priority order, emitting at most one error per file.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Optional

import yaml

from yass.errors import (
    YAML_ANCHOR_OR_ALIAS,
    YAML_DUPLICATE_KEY,
    YAML_EMPTY_FILE,
    YAML_HAS_BOM,
    YAML_MALFORMED,
    YAML_NOT_UTF8,
)

# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------


@dataclass
class ParsedDocument:
    """A single YAML document parsed from a multi-document stream."""

    data: dict | list | str | int | float | bool | None
    start_line: int  # 1-based line number


@dataclass
class YAMLError:
    """A CheckYAML diagnostic for a parsed file."""

    code: str
    line: Optional[int]  # 1-based, or None
    message: str


# ---------------------------------------------------------------------------
# YAML 1.2 string-safe loader
# ---------------------------------------------------------------------------

# PyYAML's SafeLoader resolves yes/no/on/off as booleans (YAML 1.1).
# The yass spec requires YAML 1.2 semantics where only true/false are
# booleans.  We build a custom loader that keeps those values as strings.

# Values that YAML 1.1 treats as booleans but YAML 1.2 does not.
_YAML11_EXTRA_BOOLS = frozenset({
    "yes", "Yes", "YES",
    "no", "No", "NO",
    "on", "On", "ON",
    "off", "Off", "OFF",
})


class _YassLoader(yaml.SafeLoader):
    """SafeLoader subclass with YAML 1.2 boolean semantics and duplicate
    key detection."""
    pass


# Remove the bool resolver for the extra YAML 1.1 booleans.
# We rebuild the implicit_resolvers mapping, stripping the bool tag for
# characters that are first-chars of the extra bool values only (and
# replacing the bool regexp with one that matches only true/false).
_YAML12_BOOL_RE = re.compile(
    r"^(?:true|True|TRUE|false|False|FALSE)$"
)

# Build a fresh resolvers dict so we don't mutate SafeLoader's class state.
_YassLoader.yaml_implicit_resolvers = {}
for _key_char, _resolver_list in yaml.SafeLoader.yaml_implicit_resolvers.items():
    new_list = []
    for _tag, _regexp in _resolver_list:
        if _tag == "tag:yaml.org,2002:bool":
            # Replace with the YAML 1.2 bool pattern.
            new_list.append((_tag, _YAML12_BOOL_RE))
        else:
            new_list.append((_tag, _regexp))
    _YassLoader.yaml_implicit_resolvers[_key_char] = new_list


# ---------------------------------------------------------------------------
# Duplicate-key–detecting construct_mapping
# ---------------------------------------------------------------------------

class _DuplicateKeyError(Exception):
    """Raised when a duplicate key is found during mapping construction."""

    def __init__(self, key: str, line: Optional[int]) -> None:
        self.key = key
        self.line = line
        super().__init__(f"duplicate key: {key!r}")


def _construct_mapping(loader: _YassLoader, node: yaml.MappingNode, deep: bool = False) -> dict:  # noqa: FBT001,FBT002
    """Construct a mapping, raising on duplicate keys."""
    loader.flatten_mapping(node)
    pairs = loader.construct_pairs(node, deep=deep)
    seen: dict[str, bool] = {}
    for key, _value in pairs:
        str_key = str(key)
        if str_key in seen:
            # node.start_mark.line is 0-based; we report 1-based.
            line = node.start_mark.line + 1 if node.start_mark else None
            raise _DuplicateKeyError(str_key, line)
        seen[str_key] = True
    return dict(pairs)


_YassLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    _construct_mapping,
)


# ---------------------------------------------------------------------------
# Token-stream anchor/alias/tag scanner
# ---------------------------------------------------------------------------

def _scan_tokens(text: str) -> tuple[Optional[YAMLError], Optional[YAMLError]]:
    """Scan the YAML token stream for malformed YAML and anchors/aliases/tags.

    Returns ``(malformed_error, anchor_error)`` where each is ``None`` if
    the respective issue was not found.  The scanner runs to completion (or
    failure), so both results are available in one pass.
    """
    anchor_err: Optional[YAMLError] = None
    try:
        for token in yaml.scan(text):
            # Only capture the first anchor/alias/tag occurrence.
            if anchor_err is not None:
                continue
            if isinstance(token, yaml.AnchorToken):
                line = token.start_mark.line + 1 if token.start_mark else None
                anchor_err = YAMLError(
                    code=YAML_ANCHOR_OR_ALIAS,
                    line=line,
                    message=f"anchor &{token.value} is not allowed",
                )
            elif isinstance(token, yaml.AliasToken):
                line = token.start_mark.line + 1 if token.start_mark else None
                anchor_err = YAMLError(
                    code=YAML_ANCHOR_OR_ALIAS,
                    line=line,
                    message=f"alias *{token.value} is not allowed",
                )
            elif isinstance(token, yaml.TagToken):
                line = token.start_mark.line + 1 if token.start_mark else None
                handle, suffix = token.value
                tag_str = f"{handle}{suffix}" if handle else suffix
                anchor_err = YAMLError(
                    code=YAML_ANCHOR_OR_ALIAS,
                    line=line,
                    message=f"explicit tag {tag_str} is not allowed",
                )
    except yaml.YAMLError as exc:
        # Scanner-level failure means malformed YAML.
        line: Optional[int] = None
        if hasattr(exc, "problem_mark") and exc.problem_mark is not None:
            line = exc.problem_mark.line + 1
        malformed_err = YAMLError(
            code=YAML_MALFORMED,
            line=line,
            message=str(exc).replace("\n", " "),
        )
        return malformed_err, anchor_err
    return None, anchor_err


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def parse_yaml_file(filepath: str) -> tuple[list[ParsedDocument], list[YAMLError]]:
    """Parse a YAML file with full CheckYAML validation.

    Returns ``(documents, errors)`` where *documents* is a list of
    :class:`ParsedDocument` instances and *errors* contains at most one
    :class:`YAMLError` (the highest-priority violation found).

    CheckYAML priority order (at most one error emitted):

    1. ``yass.yaml.not_utf8``
    2. ``yass.yaml.has_bom``
    3. ``yass.yaml.empty_file``
    4. ``yass.yaml.malformed``
    5. ``yass.yaml.duplicate_key``
    6. ``yass.yaml.anchor_or_alias``
    """
    # -- Read raw bytes --------------------------------------------------------
    with open(filepath, "rb") as fh:
        raw = fh.read()

    # -- 1. UTF-8 validity -----------------------------------------------------
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        return [], [YAMLError(
            code=YAML_NOT_UTF8,
            line=None,
            message=f"file is not valid UTF-8: {exc}",
        )]

    # -- 2. BOM ----------------------------------------------------------------
    if raw[:3] == b"\xef\xbb\xbf":
        return [], [YAMLError(
            code=YAML_HAS_BOM,
            line=1,
            message="file begins with a UTF-8 BOM",
        )]

    # -- 3. Empty file ---------------------------------------------------------
    if len(raw) == 0:
        return [], [YAMLError(
            code=YAML_EMPTY_FILE,
            line=None,
            message="file is zero bytes",
        )]

    # -- 4. Malformed YAML & 6. Anchor/alias/tag (token scan) ----------------
    # We scan the token stream first.  This detects scanner-level
    # malformation (priority 4) and anchors/aliases/tags (priority 6) in
    # a single pass.  Anchors/aliases/tags are recorded even if the
    # scanner ultimately fails -- but malformed always wins.
    malformed_err, anchor_err = _scan_tokens(text)

    if malformed_err is not None:
        return [], [malformed_err]

    # -- 5. Duplicate key (requires full loading) ------------------------------
    documents: list[ParsedDocument] = []
    try:
        loader = _YassLoader(text)
        try:
            while loader.check_data():
                mark = loader.get_mark()
                start_line = (mark.line + 1) if mark else 1
                data = loader.get_data()
                documents.append(ParsedDocument(data=data, start_line=start_line))
        finally:
            loader.dispose()
    except _DuplicateKeyError as exc:
        return [], [YAMLError(
            code=YAML_DUPLICATE_KEY,
            line=exc.line,
            message=str(exc),
        )]
    except yaml.YAMLError as exc:
        # The scanner passed but the loader failed.  If the token scan
        # already flagged an anchor/alias/tag (e.g. an unknown tag
        # constructor), that error takes precedence over a generic
        # malformed classification.
        if anchor_err is not None:
            return [], [anchor_err]
        line: Optional[int] = None
        if hasattr(exc, "problem_mark") and exc.problem_mark is not None:
            line = exc.problem_mark.line + 1
        return [], [YAMLError(
            code=YAML_MALFORMED,
            line=line,
            message=str(exc).replace("\n", " "),
        )]

    # -- 6. Anchor / alias / tag (from the earlier scan) -----------------------
    if anchor_err is not None:
        return [], [anchor_err]

    return documents, []


def parse_yaml_string(text: str) -> tuple[list[ParsedDocument], list[YAMLError]]:
    """Parse a YAML string (convenience wrapper without file-level checks).

    Skips UTF-8, BOM, and empty-file checks (the caller already has a
    string).  Useful for testing.
    """
    malformed_err, anchor_err = _scan_tokens(text)
    if malformed_err is not None:
        return [], [malformed_err]

    documents: list[ParsedDocument] = []
    try:
        loader = _YassLoader(text)
        try:
            while loader.check_data():
                mark = loader.get_mark()
                start_line = (mark.line + 1) if mark else 1
                data = loader.get_data()
                documents.append(ParsedDocument(data=data, start_line=start_line))
        finally:
            loader.dispose()
    except _DuplicateKeyError as exc:
        return [], [YAMLError(
            code=YAML_DUPLICATE_KEY,
            line=exc.line,
            message=str(exc),
        )]
    except yaml.YAMLError as exc:
        if anchor_err is not None:
            return [], [anchor_err]
        line: Optional[int] = None
        if hasattr(exc, "problem_mark") and exc.problem_mark is not None:
            line = exc.problem_mark.line + 1
        return [], [YAMLError(
            code=YAML_MALFORMED,
            line=line,
            message=str(exc).replace("\n", " "),
        )]

    if anchor_err is not None:
        return [], [anchor_err]

    return documents, []
