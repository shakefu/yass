package validate

import (
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestCheckUniqueness_Unique(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\n---\nspec: SpecB\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_Duplicate(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\n---\nspec: SpecA\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSpecDuplicateName {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecDuplicateName, errs[0].Code)
	}
}

func TestCheckUniqueness_MultipleDuplicates(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\n---\nspec: SpecA\n---\nspec: SpecA\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		if e.Code != yerrors.CodeSpecDuplicateName {
			t.Errorf("expected code %s, got %s", yerrors.CodeSpecDuplicateName, e.Code)
		}
	}
}

func TestCheckUniqueness_SkipsPreamble(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: SpecA\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_NoDocs(t *testing.T) {
	errs := CheckUniqueness(nil, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_SingleSpec(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: OnlyOne\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_ManyDistinct(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: A\n---\nspec: B\n---\nspec: C\n---\nspec: D\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_DuplicatePointsAtLine(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: Alpha\n---\nspec: Alpha\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	// The error should point at the second occurrence, which is on a later line.
	if errs[0].Line <= 0 {
		t.Errorf("expected positive line for duplicate, got %d", errs[0].Line)
	}
}

func TestCheckUniqueness_EmptySpecNames_Ignored(t *testing.T) {
	// Documents without a spec key or with empty spec name should be skipped.
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: \"\"\n---\nspec: \"\"\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	// Empty names are ignored by extractSpecName.
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty spec names, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_ThreeDuplicates(t *testing.T) {
	// Three occurrences of same name: 2 errors (for 2nd and 3rd).
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: X\n---\nspec: X\n---\nspec: X\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_MixedDuplicates(t *testing.T) {
	// A and B each appear twice.
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: A\n---\nspec: B\n---\nspec: A\n---\nspec: B\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckUniqueness_CaseSensitive(t *testing.T) {
	// "Foo" and "foo" should NOT be considered duplicates (case-sensitive).
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: Foo\n---\nspec: foo\n")
	errs := CheckUniqueness(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors (case-sensitive), got %d: %v", len(errs), errs)
	}
}
