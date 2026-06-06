package validate

import (
	"testing"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// parseOneDoc parses a single YAML document from a string.
func parseOneDoc(t *testing.T, content string) *yaml.Node {
	t.Helper()
	docs := mustParseDocs(t, content)
	if len(docs) == 0 {
		t.Fatal("expected at least one document")
	}
	return docs[0]
}

func TestCheckSpec_NoName(t *testing.T) {
	doc := parseOneDoc(t, "---\nINPUT:\n- MUST: do something\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSpecNoName {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNoName, errs[0].Code)
	}
}

func TestCheckSpec_NameNotString(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: 123\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSpecNameNotString {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameNotString, errs[0].Code)
	}
}

func TestCheckSpec_NameEmpty(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: \"\"\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSpecNameEmpty {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameEmpty, errs[0].Code)
	}
}

func TestCheckSpec_NameBadChars(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: \"my spec!\"\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) < 1 {
		t.Fatalf("expected at least 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeSpecNameBadChars {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameBadChars, errs[0].Code)
	}
}

func TestCheckSpec_NameBadForm_StartDot(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: \".MySpec\"\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) < 1 {
		t.Fatalf("expected at least 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeSpecNameBadForm {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameBadForm, errs[0].Code)
	}
}

func TestCheckSpec_NameBadForm_EndDot(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: \"MySpec.\"\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) < 1 {
		t.Fatalf("expected at least 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeSpecNameBadForm {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameBadForm, errs[0].Code)
	}
}

func TestCheckSpec_NameBadForm_ConsecutiveDots(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: \"My..Spec\"\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) < 1 {
		t.Fatalf("expected at least 1 error, got %d", len(errs))
	}
	if errs[0].Code != yerrors.CodeSpecNameBadForm {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecNameBadForm, errs[0].Code)
	}
}

func TestCheckSpec_NameReserved(t *testing.T) {
	reserved := []string{"INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT",
		"MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY",
		"input", "must", "may"}
	for _, name := range reserved {
		doc := parseOneDoc(t, "---\nspec: "+name+"\n")
		errs := CheckSpec(doc, "test.yass.yaml")
		found := false
		for _, e := range errs {
			if e.Code == yerrors.CodeSpecNameReserved {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected reserved error for name %q, got: %v", name, errs)
		}
	}
}

func TestCheckSpec_UnknownKey(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nBOGUS: something\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSpecUnknownKey {
		t.Errorf("expected code %s, got %s", yerrors.CodeSpecUnknownKey, errs[0].Code)
	}
}

func TestCheckSpec_SlotValueNotList(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT: not a list\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != yerrors.CodeSlotValueNotList {
		t.Errorf("expected code %s, got %s", yerrors.CodeSlotValueNotList, errs[0].Code)
	}
}

func TestCheckSpec_ObligationBadValueShape(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST:\n    nested: mapping\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationBadValueShape {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected bad_value_shape error, got: %v", errs)
	}
}

func TestCheckSpec_ObligationMissingNormativityOrRef(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- WHEN: something\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	foundMissing := false
	foundGuard := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationMissingNormativityOrRef {
			foundMissing = true
		}
		if e.Code == yerrors.CodeObligationGuardWithoutNormativity {
			foundGuard = true
		}
	}
	if !foundMissing {
		t.Errorf("expected missing_normativity_or_ref error, got: %v", errs)
	}
	if !foundGuard {
		t.Errorf("expected guard_without_normativity error, got: %v", errs)
	}
}

func TestCheckSpec_ObligationGuardWithoutNormativity(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- WHEN: something\n  USES: SomeSpec\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationGuardWithoutNormativity {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected guard_without_normativity error, got: %v", errs)
	}
}

func TestCheckSpec_ObligationDuplicateNormativity(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST: do this\n  MAY: do that\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationDuplicateNormativity {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate_normativity error, got: %v", errs)
	}
}

func TestCheckSpec_ObligationDuplicateReference_YAML(t *testing.T) {
	// YAML parsers typically merge duplicate keys, so this test verifies
	// that the YAML parser behavior doesn't produce duplicate ref errors.
	// The actual duplicate reference check is tested programmatically below.
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST: do something\n  USES: SpecA\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid spec, got %d: %v", len(errs), errs)
	}
}

func TestCheckSpec_ObligationDuplicateReference_Programmatic(t *testing.T) {
	// Build a node tree with duplicate reference keys programmatically.
	oblMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Line: 5,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "MUST", Line: 5},
			{Kind: yaml.ScalarNode, Value: "do something", Tag: "!!str", Line: 5},
			{Kind: yaml.ScalarNode, Value: "USES", Line: 6},
			{Kind: yaml.ScalarNode, Value: "SpecA", Tag: "!!str", Line: 6},
			{Kind: yaml.ScalarNode, Value: "USES", Line: 7},
			{Kind: yaml.ScalarNode, Value: "SpecB", Tag: "!!str", Line: 7},
		},
	}
	slotSeq := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Content: []*yaml.Node{oblMapping},
	}
	specMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Line: 1,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "spec", Line: 1},
			{Kind: yaml.ScalarNode, Value: "MySpec", Tag: "!!str", Line: 1},
			{Kind: yaml.ScalarNode, Value: "INPUT", Line: 4},
			slotSeq,
		},
	}
	doc := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{specMapping},
	}

	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationDuplicateReference {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate_reference error, got: %v", errs)
	}
}

func TestCheckSpec_NormativityUnknown(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- SHOUD: typo\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeNormativityUnknown {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown normativity error, got: %v", errs)
	}
}

func TestCheckSpec_Valid(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST: do something\n  USES: OtherSpec\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckSpec_ValidRefOnly(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- CONFORMS: OtherSpec\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckSpec_ObligationNullValue(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST:\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	found := false
	for _, e := range errs {
		if e.Code == yerrors.CodeObligationBadValueShape {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected bad_value_shape error for null value, got: %v", errs)
	}
}

func TestCheckSpec_MultipleSlots(t *testing.T) {
	doc := parseOneDoc(t, "---\nspec: MySpec\nINPUT:\n- MUST: accept input\nRETURN:\n- MUST: return output\n")
	errs := CheckSpec(doc, "test.yass.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}
