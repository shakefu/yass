"""Shared utilities: project-root discovery, spec-file discovery, glob expansion."""

from __future__ import annotations

import glob as _glob_mod
import os
import unicodedata
from typing import Optional

from yass.errors import (
    DISCOVER_DIR_UNREADABLE,
    FINDROOT_NO_MARKER,
    GLOB_NO_MATCH,
    PATH_BAD_EXTENSION,
    PATH_INVALID_TYPE,
    PATH_NOT_FOUND,
    PATH_UNREADABLE,
    EXIT_CODE_MAP,
    format_path,
)


# ---------------------------------------------------------------------------
# Custom exception used by all three public functions
# ---------------------------------------------------------------------------

class SharedError(Exception):
    """Carries an error *code*, human *message*, and suggested *exit_code*."""

    def __init__(self, code: str, message: str, exit_code: int | None = None):
        self.code = code
        self.message = message
        self.exit_code = exit_code if exit_code is not None else EXIT_CODE_MAP.get(code, 1)
        super().__init__(message)


# ---------------------------------------------------------------------------
# NFC sort key
# ---------------------------------------------------------------------------

def _nfc_sort_key(path: str) -> str:
    """Return the NFC-normalized form of *path* for sort comparison."""
    return unicodedata.normalize("NFC", path)


# ---------------------------------------------------------------------------
# FindProjectRoot
# ---------------------------------------------------------------------------

def FindProjectRoot(start_dir: Optional[str] = None) -> str:
    """Walk upward from *start_dir* and return the project root.

    First pass: look for a ``.git`` entry (file or directory).
    Second pass (only if no ``.git`` found): look for any ``*.yass.yaml`` file.
    Raises :class:`SharedError` with :data:`FINDROOT_NO_MARKER` if nothing found.
    """
    if start_dir is None:
        start_dir = os.getcwd()

    # Normalise to absolute without resolving symlinks.
    current = os.path.normpath(os.path.abspath(start_dir))

    # --- First pass: .git ---------------------------------------------------
    check = current
    while True:
        if os.path.exists(os.path.join(check, ".git")):
            return check
        parent = os.path.dirname(check)
        if parent == check:
            break
        check = parent

    # --- Second pass: any *.yass.yaml file -----------------------------------
    check = current
    while True:
        try:
            entries = os.listdir(check)
        except OSError:
            entries = []
        for entry in entries:
            if entry.endswith(".yass.yaml") and len(entry) > len(".yass.yaml"):
                return check
        parent = os.path.dirname(check)
        if parent == check:
            break
        check = parent

    raise SharedError(
        FINDROOT_NO_MARKER,
        f"no project root marker found above {start_dir}",
    )


# ---------------------------------------------------------------------------
# DiscoverSpecFiles
# ---------------------------------------------------------------------------

def DiscoverSpecFiles(
    path: Optional[str] = None,
    project_root: Optional[str] = None,
) -> list[str]:
    """Discover ``.yass.yaml`` spec files.

    * If *path* is a file, return ``[path]`` (after extension check).
    * If *path* is a directory, recursively find all matching files.
    * If *path* is ``None``, search from *project_root*.

    Raises :class:`SharedError` on bad input.
    """
    cwd = os.getcwd()

    if path is None:
        if project_root is None:
            project_root = FindProjectRoot()
        path = project_root

    # Resolve the effective path (lexical only — no realpath).
    abs_path = os.path.normpath(os.path.join(cwd, path)) \
        if not os.path.isabs(path) else os.path.normpath(path)

    # --- existence ----------------------------------------------------------
    # Use lstat so that a broken symlink still "exists" for our check.
    try:
        os.lstat(abs_path)
    except OSError:
        raise SharedError(PATH_NOT_FOUND, f"path not found: {path}")

    # --- file ---------------------------------------------------------------
    if os.path.isfile(abs_path) or (os.path.islink(abs_path) and _is_file_symlink(abs_path)):
        _check_extension(abs_path, path)
        _check_readable(abs_path, path)
        return [format_path(abs_path, cwd=cwd)]

    # --- directory ----------------------------------------------------------
    if os.path.isdir(abs_path):
        _check_readable_dir(abs_path, path, fatal=True)
        results = _walk_for_specs(abs_path, cwd)
        return sorted(results, key=_nfc_sort_key)

    # --- neither file nor directory -----------------------------------------
    raise SharedError(PATH_INVALID_TYPE, f"path is neither file nor directory: {path}")


def _is_file_symlink(p: str) -> bool:
    """Return True if *p* is a symlink whose target is a file (or itself)."""
    try:
        target = os.path.realpath(p)
        return os.path.isfile(target)
    except OSError:
        return False


def _check_extension(abs_path: str, display_path: str) -> None:
    """Raise if *abs_path* does not end with ``.yass.yaml``."""
    basename = os.path.basename(abs_path)
    if not basename.endswith(".yass.yaml") or len(basename) <= len(".yass.yaml"):
        raise SharedError(PATH_BAD_EXTENSION, f"file does not have .yass.yaml extension: {display_path}")


def _check_readable(abs_path: str, display_path: str) -> None:
    """Raise if *abs_path* is not readable."""
    if not os.access(abs_path, os.R_OK):
        raise SharedError(PATH_UNREADABLE, f"file is not readable: {display_path}")


def _check_readable_dir(abs_path: str, display_path: str, fatal: bool = False) -> bool:
    """Check if a directory is readable/listable.

    If *fatal* is True, raises :class:`SharedError` with PATH_UNREADABLE.
    Otherwise returns False (non-fatal) — caller should skip.
    """
    if not os.access(abs_path, os.R_OK | os.X_OK):
        if fatal:
            raise SharedError(PATH_UNREADABLE, f"directory is not readable: {display_path}")
        return False
    return True


def _walk_for_specs(root: str, cwd: str) -> list[str]:
    """Recursively collect ``.yass.yaml`` files under *root*.

    - Skips hidden directories (name starts with ``"."``)
    - Skips hidden files
    - Does not follow symlinks during traversal
    - Non-listable subdirectories produce a non-fatal warning (continue)
    """
    results: list[str] = []

    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        # --- prune hidden directories and symlink directories ---------------
        dirnames[:] = [
            d for d in dirnames
            if not d.startswith(".")
            and not os.path.islink(os.path.join(dirpath, d))
        ]
        # Sort dirnames for deterministic traversal (though final sort is NFC).
        dirnames.sort()

        # --- check dir readability (non-fatal) ------------------------------
        # root itself was already checked; subdirs may fail.
        if dirpath != root:
            if not os.access(dirpath, os.R_OK | os.X_OK):
                # Non-fatal: skip this subtree.
                dirnames.clear()
                continue

        for fname in filenames:
            # Skip hidden files.
            if fname.startswith("."):
                continue
            # Skip symlink files encountered during traversal.
            full = os.path.join(dirpath, fname)
            if os.path.islink(full):
                continue
            # Match .yass.yaml suffix with non-empty prefix.
            if fname.endswith(".yass.yaml") and len(fname) > len(".yass.yaml"):
                results.append(format_path(full, cwd=cwd))

    return results


# ---------------------------------------------------------------------------
# ExpandGlob
# ---------------------------------------------------------------------------

def ExpandGlob(pattern: str) -> list[str]:
    """Expand *pattern* using double-star glob semantics.

    If *pattern* contains no glob meta-characters (``*``, ``?``, ``[``),
    return ``[pattern]`` unchanged.

    Raises :class:`SharedError` with :data:`GLOB_NO_MATCH` when expansion
    yields zero results.
    """
    if not any(ch in pattern for ch in ("*", "?", "[")):
        return [pattern]

    # Use recursive=True for ** support.
    raw = _glob_mod.glob(pattern, recursive=True)

    # Filter hidden files/dirs — any path component starting with "." is excluded.
    filtered: list[str] = []
    for p in raw:
        parts = p.replace(os.sep, "/").split("/")
        if any(part.startswith(".") for part in parts if part):
            continue
        # Do not follow symlinks.
        if os.path.islink(p):
            continue
        filtered.append(p.replace(os.sep, "/"))

    if not filtered:
        raise SharedError(GLOB_NO_MATCH, f"glob pattern matched zero files: {pattern}")

    return sorted(filtered, key=_nfc_sort_key)
