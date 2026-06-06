"""Shared test fixtures for yass CLI tests."""

import os
import pytest


@pytest.fixture
def yass_dir(tmp_path):
    """Create a directory with a .git marker (simulating a project root)."""
    git_dir = tmp_path / ".git"
    git_dir.mkdir()
    return tmp_path


@pytest.fixture
def sample_spec(tmp_path):
    """Create a minimal valid .yass.yaml file."""
    content = """\
---
description: Test spec file
version: v1
---
spec: TestSpec
INPUT:
- MUST: accept a test input
RETURN:
- MUST: return a test result
"""
    filepath = tmp_path / "test.yass.yaml"
    filepath.write_text(content, encoding="utf-8")
    return filepath


@pytest.fixture
def sample_spec_with_refs(tmp_path):
    """Create .yass.yaml files with cross-references."""
    # Main spec
    main_content = """\
---
description: Main spec with refs
version: v1
---
spec: MainSpec
RETURN:
- CONFORMS: ./other@OtherSpec::RETURN
- MUST: return something
  USES: OtherSpec
"""
    main_file = tmp_path / "main.yass.yaml"
    main_file.write_text(main_content, encoding="utf-8")

    # Referenced spec
    other_content = """\
---
description: Other spec for ref testing
version: v1
---
spec: OtherSpec
RETURN:
- MUST: return a value
"""
    other_file = tmp_path / "other.yass.yaml"
    other_file.write_text(other_content, encoding="utf-8")

    return main_file, other_file


@pytest.fixture
def multi_spec_dir(tmp_path):
    """Create a directory tree with multiple .yass.yaml files."""
    git_dir = tmp_path / ".git"
    git_dir.mkdir()

    # Root-level spec
    root_spec = tmp_path / "root.yass.yaml"
    root_spec.write_text(
        "---\ndescription: Root spec\nversion: v1\n---\nspec: RootSpec\nRETURN:\n- MUST: work\n",
        encoding="utf-8",
    )

    # Subdir spec
    sub_dir = tmp_path / "sub"
    sub_dir.mkdir()
    sub_spec = sub_dir / "child.yass.yaml"
    sub_spec.write_text(
        "---\ndescription: Child spec\nversion: v1\n---\nspec: ChildSpec\nRETURN:\n- MUST: work\n",
        encoding="utf-8",
    )

    # Hidden dir (should be skipped)
    hidden_dir = tmp_path / ".hidden"
    hidden_dir.mkdir()
    hidden_spec = hidden_dir / "hidden.yass.yaml"
    hidden_spec.write_text(
        "---\ndescription: Hidden\nversion: v1\n---\nspec: HiddenSpec\n",
        encoding="utf-8",
    )

    return tmp_path
