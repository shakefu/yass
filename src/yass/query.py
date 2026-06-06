"""Implementation of the ``yass query`` subcommand."""

from __future__ import annotations

import os
import re
import sys
import unicodedata
from dataclasses import dataclass
from typing import Optional, TextIO

from yass.errors import (
    PATH_COLON_IN_PATH,
    QUERY_CONFORMS_NO_SLOT,
    QUERY_CONFORMS_UNRESOLVED,
    QUERY_NAME_BLANK,
    QUERY_NAME_MISSING,
    QUERY_NO_MATCH,
    QUERY_SCOPE_EMPTY,
    QUERY_SCOPE_NOT_FOUND,
    EXIT_CODE_MAP,
    emit_error,
    format_error_line,
    format_path,
)
from yass.parser import ParsedDocument, parse_yaml_file
from yass.shared import (
    DiscoverSpecFiles,
    FindProjectRoot,
    SharedError,
)


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

SLOT_KEYWORDS = frozenset({"INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"})
NORMATIVITY_KEYWORDS = frozenset({"MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"})
GUARD_KEYWORDS = frozenset({"WHEN"})
REFERENCE_KEYWORDS = frozenset({"CONFORMS", "USES", "SEE"})

# YAML core-schema type tokens that must be double-quoted.
_CORE_SCHEMA_TOKENS = frozenset({
    "true", "True", "TRUE",
    "false", "False", "FALSE",
    "null", "Null", "NULL",
    "yes", "Yes", "YES",
    "no", "No", "NO",
    "on", "On", "ON",
    "off", "Off", "OFF",
    "~",
})

# Pattern for numeric literals (int, float, octal, hex, inf, nan, etc.)
_NUMERIC_RE = re.compile(
    r"^[+-]?"
    r"("
    r"0[xX][0-9a-fA-F_]+"       # hex
    r"|0[oO][0-7_]+"             # octal
    r"|0[bB][01_]+"              # binary
    r"|[0-9][0-9_]*"             # plain int
    r"(\.[0-9_]*)?"              # optional decimal part
    r"([eE][+-]?[0-9_]+)?"       # optional exponent
    r"|\.inf|\.Inf|\.INF"        # infinity
    r"|\.nan|\.NaN|\.NAN"        # NaN
    r")$"
)

# Characters that require double-quoting when they appear at the start.
_LEADING_SPECIAL = set("?-*&!|>%@")

# Ref target grammar from validate.py
REF_TARGET_GRAMMAR_RE = re.compile(
    r"^([A-Za-z0-9._/\-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$"
)


# ---------------------------------------------------------------------------
# ErrorInfo (local)
# ---------------------------------------------------------------------------

@dataclass
class ErrorInfo:
    """A single diagnostic from query processing."""
    line: Optional[int]
    code: str
    message: str


# ---------------------------------------------------------------------------
# name_lookup
# ---------------------------------------------------------------------------

def name_lookup(name: str, files: list[str]) -> list[tuple[str, str, dict]]:
    """Look up a spec name across the given files.

    Returns a list of (filepath, spec_name, spec_doc_data) for all matches.

    Matching rules:
    - Exact match (case-sensitive byte comparison)
    - Dot-aligned trailing suffix: the query equals the spec name with zero or
      more leading dot-separated components removed.
    - No partial substring matches.
    - No case-folding or trimming.

    Raises ValueError with code yass.query.name_blank when name is empty.
    When name contains whitespace, returns empty list (no-match).
    """
    if name == "":
        raise ValueError(QUERY_NAME_BLANK)

    # Whitespace in name -> no match (not blank error)
    if any(c.isspace() for c in name):
        return []

    results: list[tuple[str, str, dict]] = []

    for filepath in files:
        abs_filepath = os.path.normpath(os.path.join(os.getcwd(), filepath)) \
            if not os.path.isabs(filepath) else filepath

        documents, errors = parse_yaml_file(abs_filepath)
        if errors:
            continue

        for doc in documents:
            if not isinstance(doc.data, dict) or "spec" not in doc.data:
                continue
            spec_name = doc.data["spec"]
            if not isinstance(spec_name, str):
                continue

            if _matches_name(name, spec_name):
                results.append((filepath, spec_name, doc.data))

    return results


def _matches_name(query: str, spec_name: str) -> bool:
    """Check if query matches spec_name via exact or dot-aligned trailing suffix."""
    # Exact match
    if query == spec_name:
        return True

    # Dot-aligned trailing suffix: spec_name must end with "." + query
    if spec_name.endswith("." + query):
        return True

    return False


# ---------------------------------------------------------------------------
# extract_fragment
# ---------------------------------------------------------------------------

def extract_fragment(filepath: str, spec_name: str) -> str:
    """Extract a single spec document from a file as a YAML fragment string.

    Returns the spec as a YAML document starting with '---'.
    No trailing '...' marker. Preserves key ordering from source.
    """
    abs_filepath = os.path.normpath(os.path.join(os.getcwd(), filepath)) \
        if not os.path.isabs(filepath) else filepath

    documents, errors = parse_yaml_file(abs_filepath)
    if errors:
        raise RuntimeError(f"Cannot parse {filepath}: {errors[0].message}")

    for doc in documents:
        if not isinstance(doc.data, dict) or "spec" not in doc.data:
            continue
        if doc.data["spec"] == spec_name:
            return emit_yaml_fragment(doc.data, {})

    raise RuntimeError(f"Spec {spec_name!r} not found in {filepath}")


# ---------------------------------------------------------------------------
# inline_conforms
# ---------------------------------------------------------------------------

def inline_conforms(
    spec_data: dict,
    filepath: str,
    project_root: str,
) -> tuple[dict, list[ErrorInfo]]:
    """Inline CONFORMS references in a spec's obligations.

    Returns (modified_spec_data, errors).

    Rules:
    - Reference-only obligation with CONFORMS: replace with inlined obligations.
    - Normative obligation with CONFORMS: keep original (sans CONFORMS), append inlined.
    - Carrier WHEN guard is preserved on each inlined obligation.
    - If inlined obligation also has WHEN, combine with ' and '.
    - Preserve original Normativity of inlined obligations.
    - Keep non-CONFORMS refs on carrier.
    - Strip CONFORMS from carrier after inlining.
    - Provenance comment above each inlined obligation.
    - Resolve exactly one level (no recursion).
    - Do NOT inline USES or SEE.
    - Error conforms_no_slot when ref lacks ::SLOT suffix.
    """
    errors: list[ErrorInfo] = []
    result = dict(spec_data)  # shallow copy top-level
    provenance_map: dict[int, str] = {}

    # Track position for provenance mapping (we build a new structure)
    spec_dir = os.path.dirname(os.path.normpath(
        os.path.join(os.getcwd(), filepath) if not os.path.isabs(filepath) else filepath
    ))

    for slot_key in list(result.keys()):
        if slot_key not in SLOT_KEYWORDS:
            continue
        slot_value = result[slot_key]
        if not isinstance(slot_value, list):
            continue

        new_obligations: list[dict] = []
        for obligation in slot_value:
            if not isinstance(obligation, dict):
                new_obligations.append(obligation)
                continue

            if "CONFORMS" not in obligation:
                new_obligations.append(obligation)
                continue

            conforms_ref = obligation["CONFORMS"]
            if not isinstance(conforms_ref, str):
                new_obligations.append(obligation)
                continue

            # Check for ::SLOT suffix
            if "::" not in conforms_ref:
                errors.append(ErrorInfo(
                    line=None,
                    code=QUERY_CONFORMS_NO_SLOT,
                    message=f"CONFORMS ref lacks ::SLOT suffix: {conforms_ref!r}",
                ))
                new_obligations.append(obligation)
                continue

            # Parse the ref target
            slot_idx = conforms_ref.rindex("::")
            ref_prefix = conforms_ref[:slot_idx]
            target_slot = conforms_ref[slot_idx + 2:]

            if target_slot not in SLOT_KEYWORDS:
                errors.append(ErrorInfo(
                    line=None,
                    code=QUERY_CONFORMS_UNRESOLVED,
                    message=f"CONFORMS ref has unknown slot: {conforms_ref!r}",
                ))
                new_obligations.append(obligation)
                continue

            # Resolve the target file and spec
            resolved = _resolve_conforms_ref(
                ref_prefix, target_slot, spec_dir, project_root, errors, conforms_ref,
                filepath,
            )
            if resolved is None:
                # Error already appended
                new_obligations.append(obligation)
                continue

            target_obligations = resolved

            # Determine if this is reference-only or normative carrier
            carrier_when = obligation.get("WHEN")
            has_normativity = any(k in NORMATIVITY_KEYWORDS for k in obligation)

            if has_normativity:
                # Normative carrier: keep original (sans CONFORMS), append inlined
                carrier_copy = {}
                for k, v in obligation.items():
                    if k != "CONFORMS":
                        carrier_copy[k] = v
                new_obligations.append(carrier_copy)

                # Append inlined obligations
                for target_obl in target_obligations:
                    inlined = _build_inlined_obligation(target_obl, carrier_when, conforms_ref)
                    new_obligations.append(inlined)
            else:
                # Reference-only: replace with inlined obligations
                # But keep non-CONFORMS refs if any
                non_conforms_refs = {}
                for k in REFERENCE_KEYWORDS:
                    if k != "CONFORMS" and k in obligation:
                        non_conforms_refs[k] = obligation[k]

                if not target_obligations:
                    # No obligations to inline, just strip CONFORMS
                    if non_conforms_refs or carrier_when:
                        stripped = {}
                        if carrier_when:
                            stripped["WHEN"] = carrier_when
                        stripped.update(non_conforms_refs)
                        if stripped:
                            new_obligations.append(stripped)
                    # else: drop the obligation entirely
                else:
                    for target_obl in target_obligations:
                        inlined = _build_inlined_obligation(target_obl, carrier_when, conforms_ref)
                        # Merge non-CONFORMS refs onto the first inlined obligation
                        # Actually per spec: "keep all non-CONFORMS Reference relations on the carrier"
                        # This means the carrier (not the inlined) keeps them. But for reference-only
                        # the carrier is being replaced... The non-CONFORMS refs go on each inlined?
                        # Actually re-reading: we keep non-CONFORMS refs on the carrier obligation.
                        # For reference-only, the carrier is gone, so we need to think about this.
                        # The spec says "keep all non-CONFORMS Reference relations on the carrier obligation"
                        # and "strip CONFORMS from carrier after inlining". For reference-only carriers
                        # that get replaced, the non-CONFORMS refs should still appear somewhere.
                        # Let's add them to each inlined obligation.
                        for nck, ncv in non_conforms_refs.items():
                            if nck not in inlined:
                                inlined[nck] = ncv
                        new_obligations.append(inlined)

        result[slot_key] = new_obligations

    return result, errors


def _build_inlined_obligation(
    target_obl: dict,
    carrier_when: Optional[str],
    conforms_ref: str,
) -> dict:
    """Build a single inlined obligation from a target obligation.

    - Preserves original Normativity
    - Combines WHEN guards with ' and '
    - Adds provenance marker
    """
    inlined: dict = {"__provenance__": f"# CONFORMS: {conforms_ref}"}

    # Key ordering: Normativity, WHEN, References
    # First, extract normativity
    for k in ("MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"):
        if k in target_obl:
            inlined[k] = target_obl[k]

    # Handle WHEN guard
    target_when = target_obl.get("WHEN")
    if carrier_when and target_when:
        inlined["WHEN"] = f"{carrier_when} and {target_when}"
    elif carrier_when:
        inlined["WHEN"] = carrier_when
    elif target_when:
        inlined["WHEN"] = target_when

    # Copy reference relations (but not CONFORMS - no recursive inlining)
    for k in ("CONFORMS", "USES", "SEE"):
        if k in target_obl:
            inlined[k] = target_obl[k]

    return inlined


def _resolve_conforms_ref(
    ref_prefix: str,
    target_slot: str,
    spec_dir: str,
    project_root: str,
    errors: list[ErrorInfo],
    full_ref: str,
    source_filepath: str = "",
) -> Optional[list[dict]]:
    """Resolve a CONFORMS ref target and return the obligations from the target slot.

    Returns None on failure (error appended to errors list).
    """
    # Parse file@spec from ref_prefix
    if "@" in ref_prefix:
        at_idx = ref_prefix.index("@")
        file_token = ref_prefix[:at_idx]
        spec_name = ref_prefix[at_idx + 1:]
    else:
        file_token = None
        spec_name = ref_prefix

    # Resolve the file path
    if file_token is not None:
        if file_token.startswith("./") or file_token.startswith("../"):
            target_file = os.path.normpath(os.path.join(spec_dir, file_token + ".yass.yaml"))
        else:
            target_file = os.path.normpath(os.path.join(project_root, file_token + ".yass.yaml"))

        if not os.path.isfile(target_file):
            errors.append(ErrorInfo(
                line=None,
                code=QUERY_CONFORMS_UNRESOLVED,
                message=f"CONFORMS ref file not found: {full_ref!r}",
            ))
            return None

        documents, parse_errors = parse_yaml_file(target_file)
        if parse_errors:
            errors.append(ErrorInfo(
                line=None,
                code=QUERY_CONFORMS_UNRESOLVED,
                message=f"CONFORMS ref file not parseable: {full_ref!r}",
            ))
            return None
    else:
        # Same-file reference: re-read the source file to find the spec
        abs_source = os.path.normpath(
            os.path.join(os.getcwd(), source_filepath)
            if not os.path.isabs(source_filepath)
            else source_filepath
        )
        documents, parse_errors = parse_yaml_file(abs_source)
        if parse_errors:
            errors.append(ErrorInfo(
                line=None,
                code=QUERY_CONFORMS_UNRESOLVED,
                message=f"CONFORMS ref could not be resolved: {full_ref!r}",
            ))
            return None

    # Find the spec in the parsed documents
    for doc in documents:
        if not isinstance(doc.data, dict) or "spec" not in doc.data:
            continue
        if doc.data["spec"] == spec_name:
            slot_value = doc.data.get(target_slot)
            if slot_value is None:
                # Slot not declared - return empty list (no obligations to inline)
                return []
            if not isinstance(slot_value, list):
                return []
            # Return only dict obligations
            return [obl for obl in slot_value if isinstance(obl, dict)]

    errors.append(ErrorInfo(
        line=None,
        code=QUERY_CONFORMS_UNRESOLVED,
        message=f"CONFORMS ref spec not found: {full_ref!r}",
    ))
    return None


# ---------------------------------------------------------------------------
# emit_yaml_fragment
# ---------------------------------------------------------------------------

def emit_yaml_fragment(spec_data: dict, provenance_map: dict) -> str:
    """Emit a spec data dict as a YAML fragment string.

    Follows the OutputProfile:
    - UTF-8 with LF line endings
    - Ends with exactly one trailing LF
    - 2-space indentation
    - List items with '- ' dash-space at parent indent
    - Plain scalars unquoted by default
    - Double-quote specific patterns
    - Preserve obligation key order: Normativity, WHEN, References
    - Provenance comments at column zero above inlined obligations
    - No host file header
    - No trailing '...' marker
    """
    lines: list[str] = []
    lines.append("---")
    _emit_mapping(spec_data, 0, lines)
    return "\n".join(lines) + "\n"


def _emit_mapping(data: dict, indent: int, lines: list[str]) -> None:
    """Emit a YAML mapping at the given indentation level."""
    prefix = " " * indent
    for key, value in data.items():
        if key == "__provenance__":
            continue  # handled by list emitter
        if isinstance(value, dict):
            lines.append(f"{prefix}{key}:")
            _emit_mapping(value, indent + 2, lines)
        elif isinstance(value, list):
            lines.append(f"{prefix}{key}:")
            _emit_list(value, indent, lines, is_slot=(key in SLOT_KEYWORDS))
        else:
            scalar = _format_scalar(value)
            lines.append(f"{prefix}{key}: {scalar}")


def _emit_list(data: list, indent: int, lines: list[str], is_slot: bool = False) -> None:
    """Emit a YAML list at the given indentation level.

    List items with '- ' at parent indent level.
    """
    prefix = " " * indent
    for item in data:
        if isinstance(item, dict):
            # Check for provenance comment
            provenance = item.get("__provenance__")
            if provenance:
                lines.append(provenance)

            # Emit the mapping as a list item
            # Sort keys in obligation order: Normativity, WHEN, References
            ordered_keys = _obligation_key_order(item) if is_slot else [k for k in item.keys() if k != "__provenance__"]

            if not ordered_keys:
                lines.append(f"{prefix}- {{}}")
                continue

            first_key = ordered_keys[0]
            first_val = item[first_key]

            if isinstance(first_val, dict):
                lines.append(f"{prefix}- {first_key}:")
                _emit_mapping(first_val, indent + 4, lines)
            elif isinstance(first_val, list):
                lines.append(f"{prefix}- {first_key}:")
                _emit_list(first_val, indent + 2, lines)
            else:
                scalar = _format_scalar(first_val)
                lines.append(f"{prefix}- {first_key}: {scalar}")

            # Remaining keys at indent + 2
            for key in ordered_keys[1:]:
                val = item[key]
                sub_prefix = " " * (indent + 2)
                if isinstance(val, dict):
                    lines.append(f"{sub_prefix}{key}:")
                    _emit_mapping(val, indent + 4, lines)
                elif isinstance(val, list):
                    lines.append(f"{sub_prefix}{key}:")
                    _emit_list(val, indent + 2, lines)
                else:
                    scalar = _format_scalar(val)
                    lines.append(f"{sub_prefix}{key}: {scalar}")
        elif isinstance(item, list):
            # Nested list
            lines.append(f"{prefix}-")
            _emit_list(item, indent + 2, lines)
        else:
            scalar = _format_scalar(item)
            lines.append(f"{prefix}- {scalar}")


def _obligation_key_order(obligation: dict) -> list[str]:
    """Return keys of an obligation dict in canonical order.

    Order: Normativity keywords, WHEN, Reference keywords.
    Within each group, preserve insertion order.
    """
    norm_keys = []
    guard_keys = []
    ref_keys = []
    other_keys = []

    for k in obligation.keys():
        if k == "__provenance__":
            continue
        if k in NORMATIVITY_KEYWORDS:
            norm_keys.append(k)
        elif k in GUARD_KEYWORDS:
            guard_keys.append(k)
        elif k in REFERENCE_KEYWORDS:
            ref_keys.append(k)
        else:
            other_keys.append(k)

    return norm_keys + guard_keys + ref_keys + other_keys


def _format_scalar(value: object) -> str:
    """Format a scalar value as a YAML string.

    Rules:
    - None -> 'null' (but null needs quoting since it's a core-schema token)
      Actually 'null' is a valid YAML scalar. But the spec says to double-quote
      core-schema type tokens. So null should be emitted as 'null' when it's a
      Python None value (which represents YAML null), but when a string value
      equals "null" it should be quoted.
    - Booleans -> true/false
    - Numbers -> their string representation
    - Strings -> unquoted unless they need quoting
    """
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    if not isinstance(value, str):
        return str(value)

    # String value: check if it needs double-quoting
    if _needs_quoting(value):
        return _double_quote(value)
    return value


def _needs_quoting(s: str) -> bool:
    """Check if a string scalar needs double-quoting."""
    if not s:
        # Empty string needs quoting
        return True

    # Contains ": " (colon-space)
    if ": " in s:
        return True

    # Starts with special character
    if s[0] in _LEADING_SPECIAL:
        return True

    # Leading or trailing whitespace
    if s != s.strip():
        return True

    # Matches YAML core-schema type tokens
    if s in _CORE_SCHEMA_TOKENS:
        return True

    # Matches numeric literals
    if _NUMERIC_RE.match(s):
        return True

    # Contains characters that would break YAML flow
    # Newlines, tabs, etc.
    if any(c in s for c in ("\n", "\r", "\t")):
        return True

    # Starts with quotes or special YAML chars
    if s[0] in ('"', "'", "{", "}", "[", "]", ",", "#"):
        return True

    # Contains " #" (comment indicator)
    if " #" in s:
        return True

    # Ends with ":"
    if s.endswith(":"):
        return True

    return False


def _double_quote(s: str) -> str:
    """Wrap a string in double-quotes with YAML escaping."""
    escaped = s.replace("\\", "\\\\")
    escaped = escaped.replace('"', '\\"')
    escaped = escaped.replace("\n", "\\n")
    escaped = escaped.replace("\r", "\\r")
    escaped = escaped.replace("\t", "\\t")
    return f'"{escaped}"'


# ---------------------------------------------------------------------------
# Helpers for list-format disambiguation
# ---------------------------------------------------------------------------

def _nfc(s: str) -> str:
    """Return NFC-normalized form of *s*."""
    return unicodedata.normalize("NFC", s)


def _collapse_whitespace(s: str) -> str:
    """Replace every run of whitespace with a single space and strip."""
    return re.sub(r"\s+", " ", s).strip()


# ---------------------------------------------------------------------------
# query_command
# ---------------------------------------------------------------------------

def query_command(
    args: list[str],
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Execute ``yass query <name> [scope]``.

    Returns the exit code.
    """
    cwd = os.getcwd()

    # -- Parse arguments ------------------------------------------------------
    if not args:
        emit_error(
            "<query>", None, QUERY_NAME_MISSING,
            "no spec name provided",
            stream=stderr,
        )
        return 2

    name = args[0]

    # Check for blank name
    if name == "":
        emit_error(
            "<query>", None, QUERY_NAME_BLANK,
            "spec name is blank",
            stream=stderr,
        )
        return 2

    scope = args[1] if len(args) > 1 else None

    # -- Colon check on scope -------------------------------------------------
    if scope is not None and ":" in scope:
        stderr.write(format_error_line(
            scope, None, PATH_COLON_IN_PATH,
            f"path contains a colon: {scope}",
        ))
        return 2

    # -- Validate scope before name lookup ------------------------------------
    if scope is not None:
        abs_scope = os.path.normpath(os.path.join(cwd, scope)) \
            if not os.path.isabs(scope) else os.path.normpath(scope)

        if not os.path.exists(abs_scope):
            emit_error(
                format_path(scope, cwd=cwd), None, QUERY_SCOPE_NOT_FOUND,
                f"scope path not found: {scope}",
                stream=stderr,
            )
            return 2

    # -- Discover files -------------------------------------------------------
    try:
        project_root = FindProjectRoot(cwd)
    except SharedError as exc:
        emit_error(
            format_path(cwd, cwd=cwd) or ".", None, exc.code, exc.message,
            stream=stderr,
        )
        return 2

    try:
        if scope is not None:
            files = DiscoverSpecFiles(path=scope, project_root=project_root)
        else:
            files = DiscoverSpecFiles(path=None, project_root=project_root)
    except SharedError as exc:
        # If scope exists but has no files, that's scope_empty
        if scope is not None:
            abs_scope = os.path.normpath(os.path.join(cwd, scope)) \
                if not os.path.isabs(scope) else os.path.normpath(scope)
            if os.path.exists(abs_scope):
                emit_error(
                    format_path(scope, cwd=cwd), None, QUERY_SCOPE_EMPTY,
                    f"scope contains no .yass.yaml files: {scope}",
                    stream=stderr,
                )
                return 2
        emit_error(
            format_path(scope or cwd, cwd=cwd), None, exc.code, exc.message,
            stream=stderr,
        )
        return exc.exit_code

    if not files and scope is not None:
        emit_error(
            format_path(scope, cwd=cwd), None, QUERY_SCOPE_EMPTY,
            f"scope contains no .yass.yaml files: {scope}",
            stream=stderr,
        )
        return 2

    # -- Name lookup ----------------------------------------------------------
    try:
        matches = name_lookup(name, files)
    except ValueError as exc:
        # name_blank
        code = str(exc)
        emit_error(
            "<query>", None, code,
            "spec name is blank",
            stream=stderr,
        )
        return EXIT_CODE_MAP.get(code, 2)

    # -- Zero matches ---------------------------------------------------------
    if not matches:
        emit_error(
            "<query>", None, QUERY_NO_MATCH,
            f"no spec matches the name: {name!r}",
            stream=stderr,
        )
        return 1

    # -- Multiple matches -> disambiguation -----------------------------------
    if len(matches) > 1:
        _emit_disambiguation(matches, stdout, cwd)
        return 0

    # -- Single match -> extract fragment with CONFORMS inlining ---------------
    filepath, spec_name, spec_data = matches[0]

    # Inline CONFORMS
    abs_filepath = os.path.normpath(os.path.join(cwd, filepath)) \
        if not os.path.isabs(filepath) else filepath

    inlined_data, inline_errors = inline_conforms(spec_data, abs_filepath, project_root)

    if inline_errors:
        for err in inline_errors:
            emit_error(
                format_path(abs_filepath, cwd=cwd),
                err.line, err.code, err.message,
                stream=stderr,
            )
        return 1

    # Emit YAML fragment
    fragment = emit_yaml_fragment(inlined_data, {})
    stdout.write(fragment)
    return 0


def _emit_disambiguation(
    matches: list[tuple[str, str, dict]],
    stdout: TextIO,
    cwd: str,
) -> None:
    """Emit disambiguation rows in cli.list format (no truncation, no TTY awareness).

    Rows sorted by file path (NFC), specs in document order within each file.
    """
    # Group by filepath to preserve document order within files
    # But we need to sort files by path
    # matches are already in file-order from name_lookup, which iterates files in order

    # Build rows with (filepath, spec_name, description)
    rows: list[tuple[str, str, str]] = []

    # Group matches by file
    files_seen: dict[str, list[tuple[str, dict]]] = {}
    file_order: list[str] = []
    for filepath, spec_name, spec_data in matches:
        if filepath not in files_seen:
            files_seen[filepath] = []
            file_order.append(filepath)
        files_seen[filepath].append((spec_name, spec_data))

    # Sort files by NFC-normalized path
    sorted_files = sorted(file_order, key=lambda p: _nfc(p))

    for filepath in sorted_files:
        # Get description from the file's preamble
        abs_filepath = os.path.normpath(os.path.join(cwd, filepath)) \
            if not os.path.isabs(filepath) else filepath
        preamble_desc = _get_preamble_description(abs_filepath)

        filepath_display = filepath.replace("\t", " ")
        for spec_name, spec_data in files_seen[filepath]:
            row = f"{filepath_display}\t{spec_name}\t{preamble_desc}\n"
            stdout.write(row)


def _get_preamble_description(abs_filepath: str) -> str:
    """Extract the preamble description from a file."""
    documents, errors = parse_yaml_file(abs_filepath)
    if errors or not documents:
        return ""

    first = documents[0]
    if isinstance(first.data, dict) and "spec" not in first.data:
        raw_desc = first.data.get("description", "")
        if isinstance(raw_desc, str) and raw_desc:
            return _nfc(_collapse_whitespace(raw_desc))
    return ""
