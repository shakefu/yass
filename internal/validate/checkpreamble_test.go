package validate

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// helper to parse YAML bytes into documents for preamble tests.
func mustParseDocs(t *testing.T, content string) []*yaml.Node {
	t.Helper()
	var docs []*yaml.Node
	dec := yaml.NewDecoder(strings.NewReader(content))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err != nil {
			break
		}
		docs = append(docs, &doc)
	}
	return docs
}

func TestCheckPreamble_EmptyStream(t *testing.T) {
	errs := CheckPreamble(nil, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlEmptyStream {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlEmptyStream, errs[0].Code)
	}
}

func TestCheckPreamble_EmptyDocsSlice(t *testing.T) {
	errs := CheckPreamble([]*yaml.Node{}, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeYamlEmptyStream {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlEmptyStream, errs[0].Code)
	}
}

func TestCheckPreamble_HasSpecKey(t *testing.T) {
	docs := mustParseDocs(t, "spec: MySpec\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleHasSpecKey {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleHasSpecKey, errs[0].Code)
	}
}

func TestCheckPreamble_DuplicatePreamble(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\ndescription: test2\nversion: v1\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleDuplicate {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleDuplicate, errs[0].Code)
	}
}

func TestCheckPreamble_MisplacedPreamble(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: MySpec\n---\ndescription: misplaced\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	// This is both duplicate and misplaced. Per priority, duplicate (4) comes first.
	if errs[0].Code != yerrors.CodePreambleDuplicate {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleDuplicate, errs[0].Code)
	}
}

func TestCheckPreamble_MissingDescription(t *testing.T) {
	docs := mustParseDocs(t, "---\nversion: v1\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleMissingDescription {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleMissingDescription, errs[0].Code)
	}
}

func TestCheckPreamble_MissingVersion(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleMissingVersion {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleMissingVersion, errs[0].Code)
	}
}

func TestCheckPreamble_UnknownVersion(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v2\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleUnknownVersion {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleUnknownVersion, errs[0].Code)
	}
}

func TestCheckPreamble_BadRelated(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\nrelated: not-a-list\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleBadRelated {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleBadRelated, errs[0].Code)
	}
}

func TestCheckPreamble_BadRelatedNonStringItems(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\nrelated:\n- 123\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodePreambleBadRelated {
		t.Errorf("expected code %s, got %s", yerrors.CodePreambleBadRelated, errs[0].Code)
	}
}

func TestCheckPreamble_Valid(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckPreamble_ValidWithRelated(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\nrelated:\n- https://example.com\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckPreamble_ValidWithSpec(t *testing.T) {
	docs := mustParseDocs(t, "---\ndescription: test\nversion: v1\n---\nspec: MySpec\n")
	errs := CheckPreamble(docs, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}
