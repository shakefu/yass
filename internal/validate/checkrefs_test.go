package validate

import (
	"os"
	"path/filepath"
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestCheckRefs_ValidSameFileRef(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- MUST: do something\n  USES: SpecB\n---\nspec: SpecB\nINPUT:\n- MUST: do something else\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckRefs_SpecNotFoundSameFile(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- MUST: do something\n  USES: NonExistent\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefSpecNotFoundSameFile {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefSpecNotFoundSameFile, errs[0].Code)
	}
}

func TestCheckRefs_MalformedTarget(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: \"bad target with spaces\"\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefMalformed {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefMalformed, errs[0].Code)
	}
}

func TestCheckRefs_UnknownSlot(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: SpecB::BOGUS\n---\nspec: SpecB\nINPUT:\n- MUST: do something\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefUnknownSlot {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefUnknownSlot, errs[0].Code)
	}
}

func TestCheckRefs_FileNotFound(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: nonexistent@SpecB\n")
	dir := t.TempDir()
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefFileNotFound {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefFileNotFound, errs[0].Code)
	}
}

func TestCheckRefs_FileNotParseable(t *testing.T) {
	dir := t.TempDir()
	// Create a file that exists but is not valid YAML.
	badFile := filepath.Join(dir, "bad.yass.yaml")
	if err := os.WriteFile(badFile, []byte{0xFF, 0xFE}, 0644); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: ./bad@SpecB\n")
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefFileNotParseable {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefFileNotParseable, errs[0].Code)
	}
}

func TestCheckRefs_SpecNotFoundOtherFile(t *testing.T) {
	dir := t.TempDir()
	// Create a valid YAML file without the target spec.
	otherFile := filepath.Join(dir, "other.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: DifferentSpec\n")
	if err := os.WriteFile(otherFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: ./other@NonExistent\n")
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefSpecNotFoundOtherFile {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefSpecNotFoundOtherFile, errs[0].Code)
	}
}

func TestCheckRefs_SlotNotDeclared(t *testing.T) {
	dir := t.TempDir()
	// Create a valid YAML file with a spec that has INPUT but not ERROR.
	otherFile := filepath.Join(dir, "other.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: TargetSpec\nINPUT:\n- MUST: do something\n")
	if err := os.WriteFile(otherFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- CONFORMS: ./other@TargetSpec::ERROR\n")
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefSlotNotDeclared {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefSlotNotDeclared, errs[0].Code)
	}
}

func TestCheckRefs_SlotNotDeclaredSameFile(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- CONFORMS: SpecB::ERROR\n---\nspec: SpecB\nINPUT:\n- MUST: do something\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeRefSlotNotDeclared {
		t.Errorf("expected code %s, got %s", yerrors.CodeRefSlotNotDeclared, errs[0].Code)
	}
}

func TestCheckRefs_SelfReferenceAllowed(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- MUST: do something\n  CONFORMS: SpecA::INPUT\n")
	errs := CheckRefs(docs, "test.yass.yaml", "/tmp", "/tmp")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckRefs_DedupFileNotFoundErrors(t *testing.T) {
	// Two refs to the same non-existent file should emit only one file_not_found error.
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: nonexistent@SpecB\n- USES: nonexistent@SpecC\n")
	dir := t.TempDir()
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	fileNotFoundCount := 0
	for _, e := range errs {
		if e.Code == yerrors.CodeRefFileNotFound {
			fileNotFoundCount++
		}
	}
	if fileNotFoundCount != 1 {
		t.Errorf("expected 1 file_not_found error (deduped), got %d: %v", fileNotFoundCount, errs)
	}
}

func TestCheckRefs_ValidCrossFileRef(t *testing.T) {
	dir := t.TempDir()
	// Create the referenced file.
	otherFile := filepath.Join(dir, "other.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: TargetSpec\nINPUT:\n- MUST: do something\n")
	if err := os.WriteFile(otherFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: ./other@TargetSpec\n")
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckRefs_ValidCrossFileRefWithSlot(t *testing.T) {
	dir := t.TempDir()
	// Create the referenced file.
	otherFile := filepath.Join(dir, "other.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: TargetSpec\nINPUT:\n- MUST: do something\nRETURN:\n- MUST: return something\n")
	if err := os.WriteFile(otherFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- CONFORMS: ./other@TargetSpec::RETURN\n")
	errs := CheckRefs(docs, filepath.Join(dir, "test.yass.yaml"), dir, dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckRefs_ProjectRootRef(t *testing.T) {
	dir := t.TempDir()
	// Create a file at project root level.
	rootFile := filepath.Join(dir, "root.yass.yaml")
	content := []byte("---\ndescription: test\nversion: v1\n---\nspec: RootSpec\nINPUT:\n- MUST: do something\n")
	if err := os.WriteFile(rootFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- USES: root@RootSpec\n")
	errs := CheckRefs(docs, filepath.Join(subDir, "test.yass.yaml"), subDir, dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}
