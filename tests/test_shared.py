"""Tests for yass.shared — FindProjectRoot, DiscoverSpecFiles, ExpandGlob."""

from __future__ import annotations

import os
import stat

import pytest

from yass.errors import (
    DISCOVER_DIR_UNREADABLE,
    FINDROOT_NO_MARKER,
    GLOB_NO_MATCH,
    PATH_BAD_EXTENSION,
    PATH_INVALID_TYPE,
    PATH_NOT_FOUND,
    PATH_UNREADABLE,
)
from yass.shared import (
    DiscoverSpecFiles,
    ExpandGlob,
    FindProjectRoot,
    SharedError,
    format_path,
)


# ============================================================================
# FindProjectRoot
# ============================================================================

class TestFindProjectRoot:
    """FindProjectRoot walks upward and finds the correct root marker."""

    def test_git_dir_found(self, tmp_path):
        """A .git directory marks the project root."""
        (tmp_path / ".git").mkdir()
        sub = tmp_path / "a" / "b"
        sub.mkdir(parents=True)
        assert FindProjectRoot(str(sub)) == str(tmp_path)

    def test_git_file_found(self, tmp_path):
        """A .git *file* (submodule worktree) also marks the root."""
        (tmp_path / ".git").write_text("gitdir: ../foo.git/worktrees/bar", encoding="utf-8")
        sub = tmp_path / "deep" / "child"
        sub.mkdir(parents=True)
        assert FindProjectRoot(str(sub)) == str(tmp_path)

    def test_yass_yaml_fallback(self, tmp_path):
        """When no .git exists, a directory with *.yass.yaml is the root."""
        spec = tmp_path / "api.yass.yaml"
        spec.write_text("---\nspec: X\n", encoding="utf-8")
        sub = tmp_path / "sub"
        sub.mkdir()
        assert FindProjectRoot(str(sub)) == str(tmp_path)

    def test_git_takes_precedence_over_yass_yaml(self, tmp_path):
        """When both .git and .yass.yaml exist, .git wins."""
        (tmp_path / ".git").mkdir()
        spec = tmp_path / "api.yass.yaml"
        spec.write_text("---\nspec: X\n", encoding="utf-8")
        assert FindProjectRoot(str(tmp_path)) == str(tmp_path)

    def test_yass_yaml_ignored_when_git_exists_higher(self, tmp_path):
        """A .yass.yaml in a child should NOT be used when .git exists above."""
        (tmp_path / ".git").mkdir()
        child = tmp_path / "child"
        child.mkdir()
        (child / "api.yass.yaml").write_text("---\nspec: X\n", encoding="utf-8")
        # Starting from child, .git is found at tmp_path (parent).
        assert FindProjectRoot(str(child)) == str(tmp_path)

    def test_no_marker_raises(self, tmp_path):
        """No .git and no .yass.yaml -> error."""
        empty = tmp_path / "empty"
        empty.mkdir()
        with pytest.raises(SharedError) as exc_info:
            FindProjectRoot(str(empty))
        assert exc_info.value.code == FINDROOT_NO_MARKER

    def test_defaults_to_cwd(self, tmp_path, monkeypatch):
        """Omitting start_dir uses os.getcwd()."""
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        assert FindProjectRoot() == str(tmp_path)

    def test_deepest_git_ancestor(self, tmp_path):
        """Returns the deepest (closest to start) ancestor with .git."""
        # Outer repo.
        (tmp_path / ".git").mkdir()
        # Inner repo (nested).
        inner = tmp_path / "inner"
        inner.mkdir()
        (inner / ".git").mkdir()
        sub = inner / "deep"
        sub.mkdir()
        assert FindProjectRoot(str(sub)) == str(inner)

    def test_bare_yass_yaml_not_marker(self, tmp_path):
        """A file named exactly '.yass.yaml' (no prefix) is NOT a marker."""
        bare = tmp_path / ".yass.yaml"
        bare.write_text("---\nspec: X\n", encoding="utf-8")
        with pytest.raises(SharedError) as exc_info:
            FindProjectRoot(str(tmp_path))
        assert exc_info.value.code == FINDROOT_NO_MARKER

    def test_start_dir_is_root_with_git(self, tmp_path):
        """start_dir itself has .git -> returns start_dir."""
        (tmp_path / ".git").mkdir()
        assert FindProjectRoot(str(tmp_path)) == str(tmp_path)


# ============================================================================
# DiscoverSpecFiles
# ============================================================================

class TestDiscoverSpecFilesSingleFile:
    """DiscoverSpecFiles given a single file path."""

    def test_valid_file(self, tmp_path):
        f = tmp_path / "api.yass.yaml"
        f.write_text("---\nspec: X\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(f))
        assert len(result) == 1
        # The path should be absolute since tmp_path is not under cwd typically.
        assert result[0].endswith("api.yass.yaml")

    def test_bad_extension_raises(self, tmp_path):
        f = tmp_path / "api.yaml"
        f.write_text("---\n", encoding="utf-8")
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(f))
        assert exc_info.value.code == PATH_BAD_EXTENSION

    def test_nonexistent_raises(self, tmp_path):
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(tmp_path / "nope.yass.yaml"))
        assert exc_info.value.code == PATH_NOT_FOUND

    def test_bare_yass_yaml_rejected(self, tmp_path):
        """A file named exactly '.yass.yaml' has bad extension (empty prefix)."""
        f = tmp_path / ".yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(f))
        assert exc_info.value.code == PATH_BAD_EXTENSION


class TestDiscoverSpecFilesDirectory:
    """DiscoverSpecFiles given a directory path."""

    def test_recursive_discovery(self, tmp_path):
        (tmp_path / ".git").mkdir()
        (tmp_path / "a.yass.yaml").write_text("---\n", encoding="utf-8")
        sub = tmp_path / "sub"
        sub.mkdir()
        (sub / "b.yass.yaml").write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        names = [os.path.basename(r) for r in result]
        assert "a.yass.yaml" in names
        assert "b.yass.yaml" in names

    def test_hidden_dir_skipped(self, tmp_path):
        (tmp_path / ".hidden").mkdir()
        (tmp_path / ".hidden" / "secret.yass.yaml").write_text("---\n", encoding="utf-8")
        (tmp_path / "visible.yass.yaml").write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        names = [os.path.basename(r) for r in result]
        assert "secret.yass.yaml" not in names
        assert "visible.yass.yaml" in names

    def test_hidden_file_skipped(self, tmp_path):
        (tmp_path / ".hidden.yass.yaml").write_text("---\n", encoding="utf-8")
        (tmp_path / "visible.yass.yaml").write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        names = [os.path.basename(r) for r in result]
        assert ".hidden.yass.yaml" not in names
        assert "visible.yass.yaml" in names

    def test_non_yass_yaml_skipped(self, tmp_path):
        (tmp_path / "readme.yaml").write_text("---\n", encoding="utf-8")
        (tmp_path / "spec.yass.yaml").write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        names = [os.path.basename(r) for r in result]
        assert "readme.yaml" not in names
        assert "spec.yass.yaml" in names

    def test_sort_order_unicode_codepoint(self, tmp_path):
        """Results are sorted by NFC-normalized unicode codepoint order."""
        for name in ["z.yass.yaml", "a.yass.yaml", "m.yass.yaml"]:
            (tmp_path / name).write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        basenames = [os.path.basename(r) for r in result]
        assert basenames == sorted(basenames)

    def test_symlink_file_in_traversal_skipped(self, tmp_path):
        """Symlink files encountered during recursive walk are skipped."""
        real = tmp_path / "real.yass.yaml"
        real.write_text("---\n", encoding="utf-8")
        link = tmp_path / "link.yass.yaml"
        link.symlink_to(real)
        result = DiscoverSpecFiles(path=str(tmp_path))
        names = [os.path.basename(r) for r in result]
        assert "real.yass.yaml" in names
        assert "link.yass.yaml" not in names

    def test_symlink_dir_in_traversal_skipped(self, tmp_path):
        """Symlink directories encountered during recursive walk are skipped."""
        real_dir = tmp_path / "realdir"
        real_dir.mkdir()
        (real_dir / "spec.yass.yaml").write_text("---\n", encoding="utf-8")
        link_dir = tmp_path / "linkdir"
        link_dir.symlink_to(real_dir)
        result = DiscoverSpecFiles(path=str(tmp_path))
        # Only files under realdir should appear, not under linkdir.
        paths_str = " ".join(result)
        assert "linkdir" not in paths_str
        assert "realdir" in paths_str or "spec.yass.yaml" in paths_str


class TestDiscoverSpecFilesPathFormatting:
    """Path formatting: relative, basename, absolute, forward slashes."""

    def test_relative_under_cwd(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        sub = tmp_path / "sub"
        sub.mkdir()
        f = sub / "api.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        assert result == ["sub/api.yass.yaml"]

    def test_basename_when_in_cwd(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "api.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(f))
        assert result == ["api.yass.yaml"]

    def test_absolute_when_not_under_cwd(self, tmp_path, monkeypatch):
        """When path is outside cwd, result is absolute."""
        other = tmp_path / "other"
        other.mkdir()
        cwd_dir = tmp_path / "cwd"
        cwd_dir.mkdir()
        monkeypatch.chdir(cwd_dir)
        f = other / "api.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(f))
        assert os.path.isabs(result[0])

    def test_no_leading_dot_slash(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "api.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(f))
        for r in result:
            assert not r.startswith("./")

    def test_forward_slashes(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        sub = tmp_path / "a" / "b"
        sub.mkdir(parents=True)
        f = sub / "spec.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        result = DiscoverSpecFiles(path=str(tmp_path))
        for r in result:
            assert "\\" not in r


class TestDiscoverSpecFilesSymlinks:
    """Symlink handling edge cases."""

    def test_file_arg_symlink(self, tmp_path):
        """File arg that is a symlink: use symlink path, read target."""
        real = tmp_path / "real.yass.yaml"
        real.write_text("---\n", encoding="utf-8")
        link = tmp_path / "link.yass.yaml"
        link.symlink_to(real)
        result = DiscoverSpecFiles(path=str(link))
        assert len(result) == 1
        assert "link.yass.yaml" in result[0]

    def test_dir_arg_symlink(self, tmp_path):
        """Dir arg that is a symlink: traverse the symlink directory."""
        real_dir = tmp_path / "realdir"
        real_dir.mkdir()
        (real_dir / "spec.yass.yaml").write_text("---\n", encoding="utf-8")
        link_dir = tmp_path / "linkdir"
        link_dir.symlink_to(real_dir)
        result = DiscoverSpecFiles(path=str(link_dir))
        assert len(result) >= 1


class TestDiscoverSpecFilesErrors:
    """Error conditions for DiscoverSpecFiles."""

    def test_path_not_found(self, tmp_path):
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(tmp_path / "no_such"))
        assert exc_info.value.code == PATH_NOT_FOUND

    def test_path_bad_extension(self, tmp_path):
        f = tmp_path / "readme.txt"
        f.write_text("hello", encoding="utf-8")
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(f))
        assert exc_info.value.code == PATH_BAD_EXTENSION

    def test_path_invalid_type(self, tmp_path):
        """A FIFO (named pipe) is neither file nor directory."""
        fifo = tmp_path / "pipe"
        os.mkfifo(str(fifo))
        with pytest.raises(SharedError) as exc_info:
            DiscoverSpecFiles(path=str(fifo))
        assert exc_info.value.code == PATH_INVALID_TYPE

    def test_no_path_uses_project_root(self, tmp_path, monkeypatch):
        """When path is None, discovers from project_root."""
        (tmp_path / ".git").mkdir()
        (tmp_path / "spec.yass.yaml").write_text("---\n", encoding="utf-8")
        monkeypatch.chdir(tmp_path)
        result = DiscoverSpecFiles(path=None, project_root=str(tmp_path))
        assert len(result) >= 1


class TestDiscoverSpecFilesUnreadable:
    """Unreadable path handling."""

    def test_unreadable_file_raises(self, tmp_path):
        f = tmp_path / "locked.yass.yaml"
        f.write_text("---\n", encoding="utf-8")
        f.chmod(0o000)
        try:
            with pytest.raises(SharedError) as exc_info:
                DiscoverSpecFiles(path=str(f))
            assert exc_info.value.code == PATH_UNREADABLE
        finally:
            f.chmod(0o644)

    def test_unreadable_subdir_skipped(self, tmp_path):
        """A non-listable subdirectory is skipped (non-fatal)."""
        (tmp_path / "ok.yass.yaml").write_text("---\n", encoding="utf-8")
        bad = tmp_path / "locked"
        bad.mkdir()
        (bad / "hidden.yass.yaml").write_text("---\n", encoding="utf-8")
        bad.chmod(0o000)
        try:
            result = DiscoverSpecFiles(path=str(tmp_path))
            names = [os.path.basename(r) for r in result]
            assert "ok.yass.yaml" in names
            # hidden.yass.yaml should NOT appear (dir is unreadable).
            assert "hidden.yass.yaml" not in names
        finally:
            bad.chmod(0o755)


# ============================================================================
# ExpandGlob
# ============================================================================

class TestExpandGlobLiteral:
    """ExpandGlob with no metacharacters returns the pattern unchanged."""

    def test_plain_path(self):
        assert ExpandGlob("some/file.txt") == ["some/file.txt"]

    def test_no_expansion_no_check(self):
        """A literal path is returned even if the file doesn't exist."""
        assert ExpandGlob("nonexistent/path") == ["nonexistent/path"]


class TestExpandGlobWildcard:
    """ExpandGlob with wildcard patterns."""

    def test_star_expansion(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / "a.yass.yaml").write_text("", encoding="utf-8")
        (tmp_path / "b.yass.yaml").write_text("", encoding="utf-8")
        result = ExpandGlob("*.yass.yaml")
        assert len(result) == 2
        basenames = [os.path.basename(r) for r in result]
        assert "a.yass.yaml" in basenames
        assert "b.yass.yaml" in basenames

    def test_question_mark(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / "a1.txt").write_text("", encoding="utf-8")
        (tmp_path / "a2.txt").write_text("", encoding="utf-8")
        (tmp_path / "ab.txt").write_text("", encoding="utf-8")
        result = ExpandGlob("a?.txt")
        assert len(result) == 3

    def test_bracket_expression(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / "a1.txt").write_text("", encoding="utf-8")
        (tmp_path / "a2.txt").write_text("", encoding="utf-8")
        (tmp_path / "a3.txt").write_text("", encoding="utf-8")
        result = ExpandGlob("a[12].txt")
        assert len(result) == 2

    def test_doublestar(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        sub = tmp_path / "d1" / "d2"
        sub.mkdir(parents=True)
        (sub / "deep.yass.yaml").write_text("", encoding="utf-8")
        (tmp_path / "top.yass.yaml").write_text("", encoding="utf-8")
        result = ExpandGlob("**/*.yass.yaml")
        # Should find at least the nested file.
        basenames = [os.path.basename(r) for r in result]
        assert "deep.yass.yaml" in basenames

    def test_hidden_files_excluded(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        (tmp_path / "visible.txt").write_text("", encoding="utf-8")
        (tmp_path / ".hidden.txt").write_text("", encoding="utf-8")
        result = ExpandGlob("*.txt")
        basenames = [os.path.basename(r) for r in result]
        assert "visible.txt" in basenames
        assert ".hidden.txt" not in basenames

    def test_hidden_dirs_excluded(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        hidden = tmp_path / ".secret"
        hidden.mkdir()
        (hidden / "file.txt").write_text("", encoding="utf-8")
        (tmp_path / "visible.txt").write_text("", encoding="utf-8")
        result = ExpandGlob("**/*.txt")
        all_paths = " ".join(result)
        assert ".secret" not in all_paths

    def test_symlinks_excluded(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        real = tmp_path / "real.txt"
        real.write_text("", encoding="utf-8")
        link = tmp_path / "link.txt"
        link.symlink_to(real)
        result = ExpandGlob("*.txt")
        basenames = [os.path.basename(r) for r in result]
        assert "real.txt" in basenames
        assert "link.txt" not in basenames

    def test_no_match_raises(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        with pytest.raises(SharedError) as exc_info:
            ExpandGlob("*.nonexistent_extension_xyz")
        assert exc_info.value.code == GLOB_NO_MATCH

    def test_sort_order(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        for name in ["z.txt", "a.txt", "m.txt"]:
            (tmp_path / name).write_text("", encoding="utf-8")
        result = ExpandGlob("*.txt")
        assert result == sorted(result)

    def test_forward_slashes(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        sub = tmp_path / "sub"
        sub.mkdir()
        (sub / "file.txt").write_text("", encoding="utf-8")
        result = ExpandGlob("**/*.txt")
        for r in result:
            assert "\\" not in r


class TestExpandGlobNoExpansion:
    """ExpandGlob does NOT perform tilde, env-var, or brace expansion."""

    def test_tilde_passthrough(self):
        assert ExpandGlob("~/file.txt") == ["~/file.txt"]

    def test_env_var_passthrough(self):
        assert ExpandGlob("$HOME/file.txt") == ["$HOME/file.txt"]

    def test_brace_passthrough(self):
        # Braces are not glob metacharacters for our purposes.
        assert ExpandGlob("{a,b}.txt") == ["{a,b}.txt"]
