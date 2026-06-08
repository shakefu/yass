"""Validate subcommand: structural validation of .yass.yaml files."""

from __future__ import annotations

import os
import re
import sys
from dataclasses import dataclass
from typing import Optional, TextIO

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
    emit_error,
    format_error_line,
    format_path,
)
from yass.parser import ParsedDocument, parse_yaml_file
from yass.shared import (
    DiscoverSpecFiles,
    ExpandGlob,
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
ALL_OBLIGATION_KEYS = NORMATIVITY_KEYWORDS | GUARD_KEYWORDS | REFERENCE_KEYWORDS

# Reserved names: Slot keywords, Normativity keywords, Guard keyword, Reference keywords
RESERVED_NAMES = SLOT_KEYWORDS | NORMATIVITY_KEYWORDS | GUARD_KEYWORDS | REFERENCE_KEYWORDS

SPEC_NAME_CHARS_RE = re.compile(r"^[A-Za-z0-9._-]+$")
SPEC_NAME_FORM_RE = re.compile(r"^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$")

REF_TARGET_GRAMMAR_RE = re.compile(
    r"^([A-Za-z0-9._/\-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$"
)


# ---------------------------------------------------------------------------
# ErrorInfo
# ---------------------------------------------------------------------------

@dataclass
class ErrorInfo:
    """A single validation diagnostic."""

    line: Optional[int]
    code: str
    message: str


# ---------------------------------------------------------------------------
# CheckPreamble
# ---------------------------------------------------------------------------

def check_preamble(
    docs: list[ParsedDocument],
    filepath: str,
) -> list[ErrorInfo]:
    """Validate preamble structure. Returns at most one error per file.

    Priority order:
    1. yass.preamble.has_spec_key
    2. yass.yaml.empty_stream
    3. yass.preamble.missing
    4. yass.preamble.duplicate
    5. yass.preamble.misplaced
    6. yass.preamble.missing_description
    7. yass.preamble.missing_version
    8. yass.preamble.unknown_version
    9. yass.preamble.bad_related
    """
    # 2. empty stream
    if len(docs) == 0:
        return [ErrorInfo(line=None, code=YAML_EMPTY_STREAM, message="YAML stream contains zero documents")]

    first = docs[0]

    # 1. has_spec_key — first doc is a mapping AND has "spec" key
    if isinstance(first.data, dict) and "spec" in first.data:
        return [ErrorInfo(
            line=first.start_line,
            code=PREAMBLE_HAS_SPEC_KEY,
            message="first document contains a 'spec' key",
        )]

    # 3. missing — first doc is not a mapping (or is None, list, scalar, etc.)
    if not isinstance(first.data, dict):
        return [ErrorInfo(
            line=first.start_line,
            code=PREAMBLE_MISSING,
            message="no preamble found; first document is not a mapping",
        )]

    # At this point, first doc IS a mapping without "spec" key -> it IS a preamble.
    # Identify all preambles (docs without "spec" key that are mappings)
    preamble_indices: list[int] = []
    for i, doc in enumerate(docs):
        if isinstance(doc.data, dict) and "spec" not in doc.data:
            preamble_indices.append(i)

    # 4. duplicate — more than one preamble
    if len(preamble_indices) > 1:
        # Report the second preamble's line
        second_idx = preamble_indices[1]
        return [ErrorInfo(
            line=docs[second_idx].start_line,
            code=PREAMBLE_DUPLICATE,
            message="more than one preamble found",
        )]

    # 5. misplaced — preamble at non-first position
    # Since we already confirmed first doc is a preamble, and there's only one preamble,
    # misplaced doesn't apply (the single preamble IS at position 0).
    # But if there were multiple preambles, we already caught that above.
    # Misplaced would fire if preamble_indices contains an index > 0.
    # Since we only reach here with len(preamble_indices)==1 and preamble_indices[0]==0,
    # misplaced cannot fire.

    preamble = first.data

    # 6. missing_description
    if "description" not in preamble:
        return [ErrorInfo(
            line=first.start_line,
            code=PREAMBLE_MISSING_DESCRIPTION,
            message="preamble omits 'description'",
        )]

    # 7. missing_version
    if "version" not in preamble:
        return [ErrorInfo(
            line=first.start_line,
            code=PREAMBLE_MISSING_VERSION,
            message="preamble omits 'version'",
        )]

    # 8. unknown_version
    if preamble["version"] != "v1":
        return [ErrorInfo(
            line=first.start_line,
            code=PREAMBLE_UNKNOWN_VERSION,
            message=f"preamble version is not 'v1': {preamble['version']!r}",
        )]

    # 9. bad_related
    if "related" in preamble:
        related = preamble["related"]
        bad = False
        if not isinstance(related, list):
            bad = True
        else:
            for item in related:
                if not isinstance(item, str):
                    bad = True
                    break
        if bad:
            return [ErrorInfo(
                line=first.start_line,
                code=PREAMBLE_BAD_RELATED,
                message="'related' is not a sequence of strings",
            )]

    return []


# ---------------------------------------------------------------------------
# CheckSpec
# ---------------------------------------------------------------------------

def check_spec(
    doc: ParsedDocument,
    filepath: str,
) -> list[ErrorInfo]:
    """Validate a single spec document. Returns errors for all violations found."""
    errors: list[ErrorInfo] = []
    line = doc.start_line

    # If doc.data is not a dict, it's a non-mapping spec doc
    if not isinstance(doc.data, dict):
        errors.append(ErrorInfo(
            line=line,
            code=SPEC_NO_NAME,
            message="non-first document does not contain a 'spec' key",
        ))
        return errors

    data = doc.data

    # Check for "spec" key
    if "spec" not in data:
        errors.append(ErrorInfo(
            line=line,
            code=SPEC_NO_NAME,
            message="non-first document does not contain a 'spec' key",
        ))
        return errors

    spec_name = data["spec"]

    # Validate spec name
    name_valid = True
    if not isinstance(spec_name, str):
        errors.append(ErrorInfo(
            line=line,
            code=SPEC_NAME_NOT_STRING,
            message=f"spec name is not a string: {spec_name!r}",
        ))
        name_valid = False
    elif spec_name == "":
        errors.append(ErrorInfo(
            line=line,
            code=SPEC_NAME_EMPTY,
            message="spec name is empty",
        ))
        name_valid = False
    else:
        # Check chars
        if not SPEC_NAME_CHARS_RE.match(spec_name):
            errors.append(ErrorInfo(
                line=line,
                code=SPEC_NAME_BAD_CHARS,
                message=f"spec name contains invalid characters: {spec_name!r}",
            ))
            name_valid = False
        # Check form
        elif not SPEC_NAME_FORM_RE.match(spec_name) or spec_name.startswith(".") or spec_name.endswith(".") or ".." in spec_name:
            errors.append(ErrorInfo(
                line=line,
                code=SPEC_NAME_BAD_FORM,
                message=f"spec name has bad form: {spec_name!r}",
            ))
            name_valid = False
        # Check reserved
        elif spec_name.upper() in {kw.upper() for kw in RESERVED_NAMES}:
            errors.append(ErrorInfo(
                line=line,
                code=SPEC_NAME_RESERVED,
                message=f"spec name is a reserved keyword: {spec_name!r}",
            ))
            name_valid = False

    # Check keys other than "spec"
    for key in data:
        if key == "spec":
            continue
        if key not in SLOT_KEYWORDS:
            errors.append(ErrorInfo(
                line=line,
                code=SPEC_UNKNOWN_KEY,
                message=f"unknown key: {key!r}",
            ))
            continue

        # Valid slot key — check value is a list
        slot_value = data[key]
        if not isinstance(slot_value, list):
            errors.append(ErrorInfo(
                line=line,
                code=SLOT_VALUE_NOT_LIST,
                message=f"slot '{key}' value is not a list",
            ))
            continue

        # Validate each obligation in the slot
        for obligation in slot_value:
            errors.extend(_check_obligation(obligation, line, filepath))

    return errors


def _check_obligation(
    obligation: object,
    doc_line: int,
    filepath: str,
) -> list[ErrorInfo]:
    """Validate a single obligation within a slot."""
    errors: list[ErrorInfo] = []

    # An obligation must be a mapping
    if not isinstance(obligation, dict):
        # A scalar string is a valid shape for a simple obligation?
        # No — per the spec, an obligation must be a YAML mapping.
        # A bare string in a list is bad_value_shape.
        if obligation is None or isinstance(obligation, (list, dict)):
            errors.append(ErrorInfo(
                line=doc_line,
                code=OBLIGATION_BAD_VALUE_SHAPE,
                message=f"obligation value has bad shape: {type(obligation).__name__}",
            ))
        elif isinstance(obligation, str):
            # A bare string in a list position: this is missing_normativity_or_ref
            # Actually, a bare string is a scalar, not a mapping — it has bad value shape.
            errors.append(ErrorInfo(
                line=doc_line,
                code=OBLIGATION_BAD_VALUE_SHAPE,
                message="obligation is a scalar string, not a mapping",
            ))
        else:
            # Some other scalar (int, float, bool)
            errors.append(ErrorInfo(
                line=doc_line,
                code=OBLIGATION_BAD_VALUE_SHAPE,
                message=f"obligation value has bad shape: {type(obligation).__name__}",
            ))
        return errors

    # It is a mapping. Check keys.
    normativity_count = 0
    guard_count = 0
    reference_keys: list[str] = []
    has_normativity = False
    has_reference = False

    for key, value in obligation.items():
        if key in NORMATIVITY_KEYWORDS:
            normativity_count += 1
            has_normativity = True
            # Check value shape: must not be mapping, sequence, or null
            if value is None or isinstance(value, (dict, list)):
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=OBLIGATION_BAD_VALUE_SHAPE,
                    message=f"obligation value for '{key}' has bad shape",
                ))
        elif key in GUARD_KEYWORDS:
            guard_count += 1
            # Check value shape
            if value is None or isinstance(value, (dict, list)):
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=OBLIGATION_BAD_VALUE_SHAPE,
                    message=f"obligation value for '{key}' has bad shape",
                ))
        elif key in REFERENCE_KEYWORDS:
            has_reference = True
            reference_keys.append(key)
            # Check value shape
            if value is None or isinstance(value, (dict, list)):
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=OBLIGATION_BAD_VALUE_SHAPE,
                    message=f"obligation value for '{key}' has bad shape",
                ))
        else:
            # Unknown key at obligation level.
            # Determine if it looks like a normativity keyword or reference keyword.
            # Per spec: normativity.unknown when keyword is outside recognized normativity set,
            # reference.unknown_relation when key is outside recognized reference set.
            # We check: does it look like it could be either?
            # Simple approach: if it's all-uppercase (possibly with hyphens) it could be
            # either. We'll use heuristics.
            _classify_unknown_obligation_key(key, doc_line, errors)

    # duplicate_normativity
    if normativity_count > 1:
        errors.append(ErrorInfo(
            line=doc_line,
            code=OBLIGATION_DUPLICATE_NORMATIVITY,
            message="obligation has more than one normativity keyword",
        ))

    # duplicate_reference — same relation key more than once
    seen_refs: set[str] = set()
    for rk in reference_keys:
        if rk in seen_refs:
            errors.append(ErrorInfo(
                line=doc_line,
                code=OBLIGATION_DUPLICATE_REFERENCE,
                message=f"duplicate reference relation: {rk!r}",
            ))
        seen_refs.add(rk)

    # missing_normativity_or_ref
    if not has_normativity and not has_reference:
        errors.append(ErrorInfo(
            line=doc_line,
            code=OBLIGATION_MISSING_NORMATIVITY_OR_REF,
            message="obligation has neither normativity keyword nor reference",
        ))

    # guard_without_normativity
    if guard_count > 0 and not has_normativity:
        errors.append(ErrorInfo(
            line=doc_line,
            code=OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
            message="obligation has WHEN guard but no normativity keyword",
        ))

    return errors


def _classify_unknown_obligation_key(
    key: str,
    doc_line: int,
    errors: list[ErrorInfo],
) -> None:
    """Classify an unknown obligation key and emit the appropriate error."""
    # If the key is all uppercase letters and hyphens, it likely was intended as
    # a normativity keyword or reference keyword.
    # We use a simple heuristic: if it matches the pattern of normativity keywords
    # (all caps, may contain hyphens), check if it's closer to normativity or reference.
    # Per spec, normativity.unknown fires "when a Normativity keyword is outside the
    # recognized set" and reference.unknown_relation fires "when a Reference relation
    # key is outside the recognized set".
    #
    # Since we can't perfectly distinguish intent, we apply a simple rule:
    # - Anything that looks like it could be a keyword (uppercase with hyphens)
    #   gets normativity.unknown.
    # - This matches the spec's intent: unknown obligation-level keys that are
    #   uppercase are likely intended as normativity keywords.
    #
    # Actually, re-reading more carefully: the spec says "when a Normativity keyword
    # is outside the recognized set" — meaning the user tried to use a normativity
    # keyword but used the wrong one. Same for reference.
    #
    # Since there's no "unknown obligation key" error code, and the spec only has
    # normativity.unknown and reference.unknown_relation, we'll just flag it as
    # normativity.unknown since that's the broader category.
    errors.append(ErrorInfo(
        line=doc_line,
        code=NORMATIVITY_UNKNOWN,
        message=f"unknown obligation key: {key!r}",
    ))


# ---------------------------------------------------------------------------
# CheckUniqueness
# ---------------------------------------------------------------------------

def check_uniqueness(
    specs: list[tuple[str, int]],
    filepath: str,
) -> list[ErrorInfo]:
    """Check that spec names are unique within a file.

    *specs* is a list of ``(name, line)`` tuples for all spec docs in the file.
    Returns one error per duplicate-after-the-first occurrence.
    """
    errors: list[ErrorInfo] = []
    seen: dict[str, int] = {}  # name -> first-occurrence line

    for name, line in specs:
        if name in seen:
            errors.append(ErrorInfo(
                line=line,
                code=SPEC_DUPLICATE_NAME,
                message=f"duplicate spec name: {name!r}",
            ))
        else:
            seen[name] = line

    return errors


# ---------------------------------------------------------------------------
# CheckRefs
# ---------------------------------------------------------------------------

def check_refs(
    specs: list[ParsedDocument],
    filepath: str,
    project_root: str,
) -> list[ErrorInfo]:
    """Validate all reference targets in the spec documents of a file.

    *specs* is the list of non-preamble ParsedDocuments from the file.
    *filepath* is the absolute path to the referencing file.
    *project_root* is the absolute path to the project root.
    """
    errors: list[ErrorInfo] = []

    # Build a set of spec names in THIS file for same-file lookups
    local_specs: dict[str, ParsedDocument] = {}
    for doc in specs:
        if isinstance(doc.data, dict) and "spec" in doc.data:
            name = doc.data["spec"]
            if isinstance(name, str) and name:
                local_specs[name] = doc

    # Cache for cross-file lookups: filepath -> (specs_dict, parse_ok)
    file_cache: dict[str, tuple[dict[str, ParsedDocument] | None, bool]] = {}
    # Track (referencing_file, referenced_file) pairs for dedup of file_not_found/not_parseable
    file_error_pairs: set[tuple[str, str]] = set()

    for doc in specs:
        if not isinstance(doc.data, dict):
            continue
        doc_line = doc.start_line

        for slot_key in SLOT_KEYWORDS:
            if slot_key not in doc.data:
                continue
            slot_value = doc.data[slot_key]
            if not isinstance(slot_value, list):
                continue
            for obligation in slot_value:
                if not isinstance(obligation, dict):
                    continue
                for ref_key in REFERENCE_KEYWORDS:
                    if ref_key not in obligation:
                        continue
                    target = obligation[ref_key]
                    if not isinstance(target, str):
                        continue
                    _check_single_ref(
                        target=target,
                        doc_line=doc_line,
                        filepath=filepath,
                        project_root=project_root,
                        local_specs=local_specs,
                        file_cache=file_cache,
                        file_error_pairs=file_error_pairs,
                        errors=errors,
                    )

    return errors


def _check_single_ref(
    target: str,
    doc_line: int,
    filepath: str,
    project_root: str,
    local_specs: dict[str, ParsedDocument],
    file_cache: dict[str, tuple[dict[str, ParsedDocument] | None, bool]],
    file_error_pairs: set[tuple[str, str]],
    errors: list[ErrorInfo],
) -> None:
    """Validate a single reference target string."""

    # Match grammar
    m = REF_TARGET_GRAMMAR_RE.match(target)
    if not m:
        errors.append(ErrorInfo(
            line=doc_line,
            code=REF_MALFORMED,
            message=f"malformed ref target: {target!r}",
        ))
        return

    # Parse components
    path_part = m.group(1)  # e.g. "./foo@" or "bar/baz@" or None
    # Extract spec name and slot from the remaining string
    # The grammar: ^(path@)?specname(::SLOT)?$
    # Let's re-parse more carefully
    rest = target
    file_path_token = None
    if path_part is not None:
        file_path_token = path_part[:-1]  # strip trailing @
        rest = target[len(path_part):]

    slot_name = None
    if "::" in rest:
        idx = rest.index("::")
        spec_name = rest[:idx]
        slot_name = rest[idx + 2:]
    else:
        spec_name = rest

    # Check slot validity
    if slot_name is not None and slot_name not in SLOT_KEYWORDS:
        errors.append(ErrorInfo(
            line=doc_line,
            code=REF_UNKNOWN_SLOT,
            message=f"unknown slot in ref target: {slot_name!r}",
        ))
        return

    # Same-file ref (no path part)
    if file_path_token is None:
        if spec_name not in local_specs:
            errors.append(ErrorInfo(
                line=doc_line,
                code=REF_SPEC_NOT_FOUND_SAME_FILE,
                message=f"spec not found in same file: {spec_name!r}",
            ))
            return

        # Check slot declared
        if slot_name is not None:
            target_doc = local_specs[spec_name]
            if isinstance(target_doc.data, dict) and slot_name not in target_doc.data:
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_SLOT_NOT_DECLARED,
                    message=f"slot '{slot_name}' not declared in spec '{spec_name}'",
                ))
        return

    # Cross-file ref
    # Build target file path
    if file_path_token.startswith("./") or file_path_token.startswith("../"):
        # Relative to referencing file's directory
        ref_dir = os.path.dirname(filepath)
        target_file = os.path.normpath(os.path.join(ref_dir, file_path_token + ".yass.yaml"))
    else:
        # From project root
        target_file = os.path.normpath(os.path.join(project_root, file_path_token + ".yass.yaml"))

    pair_key = (filepath, target_file)

    # Load target file (cached)
    if target_file not in file_cache:
        if not os.path.isfile(target_file):
            file_cache[target_file] = (None, False)
            if pair_key not in file_error_pairs:
                file_error_pairs.add(pair_key)
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_FILE_NOT_FOUND,
                    message=f"referenced file not found: {target_file}",
                ))
            return
        # Try to parse
        try:
            docs, yaml_errors = parse_yaml_file(target_file)
        except Exception:
            file_cache[target_file] = (None, False)
            if pair_key not in file_error_pairs:
                file_error_pairs.add(pair_key)
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_FILE_NOT_PARSEABLE,
                    message=f"referenced file not parseable: {target_file}",
                ))
            return

        if yaml_errors:
            file_cache[target_file] = (None, False)
            if pair_key not in file_error_pairs:
                file_error_pairs.add(pair_key)
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_FILE_NOT_PARSEABLE,
                    message=f"referenced file not parseable: {target_file}",
                ))
            return

        # Build spec map from the target file
        target_specs: dict[str, ParsedDocument] = {}
        for d in docs:
            if isinstance(d.data, dict) and "spec" in d.data:
                sn = d.data["spec"]
                if isinstance(sn, str) and sn:
                    target_specs[sn] = d
        file_cache[target_file] = (target_specs, True)

    cached = file_cache[target_file]
    target_specs_map, parse_ok = cached

    if not parse_ok:
        # Already emitted file_not_found or file_not_parseable for this pair
        if pair_key not in file_error_pairs:
            file_error_pairs.add(pair_key)
            if not os.path.isfile(target_file):
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_FILE_NOT_FOUND,
                    message=f"referenced file not found: {target_file}",
                ))
            else:
                errors.append(ErrorInfo(
                    line=doc_line,
                    code=REF_FILE_NOT_PARSEABLE,
                    message=f"referenced file not parseable: {target_file}",
                ))
        return

    assert target_specs_map is not None

    if spec_name not in target_specs_map:
        errors.append(ErrorInfo(
            line=doc_line,
            code=REF_SPEC_NOT_FOUND_OTHER_FILE,
            message=f"spec '{spec_name}' not found in {target_file}",
        ))
        return

    # Check slot declared
    if slot_name is not None:
        target_doc = target_specs_map[spec_name]
        if isinstance(target_doc.data, dict) and slot_name not in target_doc.data:
            errors.append(ErrorInfo(
                line=doc_line,
                code=REF_SLOT_NOT_DECLARED,
                message=f"slot '{slot_name}' not declared in spec '{spec_name}' in {target_file}",
            ))


# ---------------------------------------------------------------------------
# validate_command
# ---------------------------------------------------------------------------

def validate_command(
    args: list[str],
    stderr: TextIO | None = None,
    stdout: TextIO | None = None,
) -> int:
    """Main entry point for the validate subcommand.

    Returns exit code: 0 success, 1 validation errors, 2 argv/path errors.
    """
    if stderr is None:
        stderr = sys.stderr
    if stdout is None:
        stdout = sys.stdout

    cwd = os.getcwd()

    # Check for colon in path arguments
    for arg in args:
        if ":" in arg:
            emit_error(
                format_path(arg, cwd=cwd),
                None,
                PATH_COLON_IN_PATH,
                f"path contains a ':' character: {arg}",
                stream=stderr,
            )
            return 2

    # Compute project root
    try:
        project_root = FindProjectRoot(cwd)
    except SharedError:
        emit_error(
            format_path(cwd, cwd=cwd) or ".",
            None,
            FINDROOT_NO_MARKER,
            "no project root marker found",
            stream=stderr,
        )
        return 2

    # Expand globs and discover files
    all_files: list[str] = []
    if not args:
        # Discover from project root
        try:
            discovered = DiscoverSpecFiles(path=None, project_root=project_root)
            all_files.extend(discovered)
        except SharedError as exc:
            if exc.code == DISCOVER_NO_FILES:
                # No files found — emit summary and return
                stdout.write("checked 0 files, found 0 errors\n")
                return 0
            emit_error(
                format_path(cwd, cwd=cwd) or ".",
                None,
                exc.code,
                exc.message,
                stream=stderr,
            )
            return 2
    else:
        for arg in args:
            # Check for glob metacharacters
            if any(ch in arg for ch in ("*", "?", "[")):
                try:
                    expanded = ExpandGlob(arg)
                except SharedError as exc:
                    emit_error(
                        format_path(arg, cwd=cwd),
                        None,
                        exc.code,
                        exc.message,
                        stream=stderr,
                    )
                    return 2
                # For glob-expanded paths, skip non-.yass.yaml files silently
                for p in expanded:
                    basename = os.path.basename(p)
                    if basename.endswith(".yass.yaml") and len(basename) > len(".yass.yaml"):
                        try:
                            found = DiscoverSpecFiles(path=p, project_root=project_root)
                            all_files.extend(found)
                        except SharedError:
                            # Skip silently for glob-expanded paths
                            pass
            else:
                # Direct path
                try:
                    found = DiscoverSpecFiles(path=arg, project_root=project_root)
                    all_files.extend(found)
                except SharedError as exc:
                    emit_error(
                        format_path(arg, cwd=cwd),
                        None,
                        exc.code,
                        exc.message,
                        stream=stderr,
                    )
                    return 2

    # Deduplicate by lexically-normalized absolute path
    seen_paths: set[str] = set()
    unique_files: list[str] = []
    for f in all_files:
        abs_f = os.path.normpath(os.path.join(cwd, f)) if not os.path.isabs(f) else os.path.normpath(f)
        if abs_f not in seen_paths:
            seen_paths.add(abs_f)
            unique_files.append(f)

    if not unique_files:
        emit_error(
            format_path(cwd, cwd=cwd) or ".",
            None,
            DISCOVER_NO_FILES,
            "no .yass.yaml files found",
            stream=stderr,
        )
        stdout.write("checked 0 files, found 0 errors\n")
        return 2

    # Process files
    total_files = len(unique_files)
    total_errors = 0

    for display_path in unique_files:
        abs_path = os.path.normpath(os.path.join(cwd, display_path)) if not os.path.isabs(display_path) else os.path.normpath(display_path)

        # CheckYAML
        docs, yaml_errors = parse_yaml_file(abs_path)

        if yaml_errors:
            for yerr in yaml_errors:
                emit_error(display_path, yerr.line, yerr.code, yerr.message, stream=stderr)
            total_errors += 1  # count as exactly one error per file
            continue

        # CheckPreamble
        preamble_errors = check_preamble(docs, abs_path)
        for err in preamble_errors:
            emit_error(display_path, err.line, err.code, err.message, stream=stderr)
        total_errors += len(preamble_errors)

        # Determine spec docs (all non-preamble docs = docs after the first)
        spec_docs = docs[1:] if len(docs) > 1 else []

        # CheckSpec
        spec_errors: list[ErrorInfo] = []
        for doc in spec_docs:
            spec_errors.extend(check_spec(doc, abs_path))
        for err in spec_errors:
            emit_error(display_path, err.line, err.code, err.message, stream=stderr)
        total_errors += len(spec_errors)

        # CheckUniqueness
        spec_names: list[tuple[str, int]] = []
        for doc in spec_docs:
            if isinstance(doc.data, dict) and "spec" in doc.data:
                name = doc.data["spec"]
                if isinstance(name, str):
                    spec_names.append((name, doc.start_line))
        uniq_errors = check_uniqueness(spec_names, abs_path)
        for err in uniq_errors:
            emit_error(display_path, err.line, err.code, err.message, stream=stderr)
        total_errors += len(uniq_errors)

        # CheckRefs
        ref_errors = check_refs(spec_docs, abs_path, project_root)
        for err in ref_errors:
            emit_error(display_path, err.line, err.code, err.message, stream=stderr)
        total_errors += len(ref_errors)

    # Flush stderr before writing summary
    stderr.flush()

    # Summary line
    stdout.write(f"checked {total_files} files, found {total_errors} errors\n")

    if total_errors > 0:
        return 1
    return 0
