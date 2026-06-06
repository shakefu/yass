package validate

import (
	"os"
	"path/filepath"
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestCheckYAML_NotUTF8(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bad.yass.yaml")
	// Write invalid UTF-8 bytes.
	if err := os.WriteFile(fp, []byte{0xFF, 0xFE, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlNotUTF8 {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlNotUTF8, errs[0].Code)
	}
}

func TestCheckYAML_HasBOM(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bom.yass.yaml")
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("key: value\n")...)
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlHasBOM {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlHasBOM, errs[0].Code)
	}
}

func TestCheckYAML_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "empty.yass.yaml")
	if err := os.WriteFile(fp, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlEmptyFile {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlEmptyFile, errs[0].Code)
	}
}

func TestCheckYAML_Malformed(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bad.yass.yaml")
	content := []byte(":\n  - [invalid\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlMalformed {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlMalformed, errs[0].Code)
	}
}

func TestCheckYAML_DuplicateKey(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "dup.yass.yaml")
	content := []byte("key: value1\nkey: value2\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlDuplicateKey {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlDuplicateKey, errs[0].Code)
	}
}

func TestCheckYAML_DuplicateKeyNested(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "dup_nested.yass.yaml")
	content := []byte("outer:\n  inner: 1\n  inner: 2\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlDuplicateKey {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlDuplicateKey, errs[0].Code)
	}
}

func TestCheckYAML_Anchor(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "anchor.yass.yaml")
	content := []byte("key: &myanchor value\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlAnchorOrAlias {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlAnchorOrAlias, errs[0].Code)
	}
}

func TestCheckYAML_Alias(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "alias.yass.yaml")
	content := []byte("a: &anc value\nb: *anc\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlAnchorOrAlias {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlAnchorOrAlias, errs[0].Code)
	}
}

func TestCheckYAML_ValidFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "valid.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: MySpec\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestCheckYAML_ExplicitTag(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tag.yass.yaml")
	content := []byte("key: !custom value\n")
	if err := os.WriteFile(fp, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs, errs := CheckYAML(fp)
	if docs != nil {
		t.Error("expected nil docs")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlAnchorOrAlias {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlAnchorOrAlias, errs[0].Code)
	}
}
