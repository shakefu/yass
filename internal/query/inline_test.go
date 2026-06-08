package query

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/shakefu/yass/internal/parser"
)

// Helper to parse a YAML string and return the documents.
func parseYAML(t *testing.T, content string) []*yaml.Node {
	t.Helper()
	result, err := parser.ParseBytes([]byte(content), "test.yass.yaml")
	if err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}
	return result.Documents
}

// Helper to find a spec document by name in parsed docs.
func findSpec(t *testing.T, docs []*yaml.Node, name string) *yaml.Node {
	t.Helper()
	for _, doc := range docs {
		if extractSpecNameFromDoc(doc) == name {
			return doc
		}
	}
	t.Fatalf("spec %s not found", name)
	return nil
}

func TestInlineConforms_RefOnlyReplacement(t *testing.T) {
	dir := t.TempDir()

	// Create the referenced file
	refContent := `---
description: referenced spec
version: v1
---
spec: cli.ExitCode
RETURN:
- MUST: exit 0 on success
- MUST: exit 1 on error
`
	refFile := filepath.Join(dir, "cli.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	// Create the referencing file
	srcContent := `---
description: test spec
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
- CONFORMS: ./cli@cli.ExitCode::RETURN
`
	srcFile := filepath.Join(dir, "test.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// The CONFORMS obligation should be replaced with the 2 obligations from ExitCode::RETURN
	mapping := getMappingNode(result)
	if mapping == nil {
		t.Fatal("expected mapping node")
	}

	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}
	if returnNode == nil {
		t.Fatal("RETURN slot not found")
	}

	// Should have 3 obligations: original "do something" + 2 inlined
	if len(returnNode.Content) != 3 {
		t.Fatalf("expected 3 obligations, got %d", len(returnNode.Content))
	}

	// Check provenance comments on inlined obligations
	if returnNode.Content[1].HeadComment == "" {
		t.Error("expected provenance comment on first inlined obligation")
	}
	if !strings.Contains(returnNode.Content[1].HeadComment, "CONFORMS: ./cli@cli.ExitCode::RETURN") {
		t.Errorf("unexpected provenance comment: %s", returnNode.Content[1].HeadComment)
	}
}

func TestInlineConforms_NormativeWithConforms(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: first rule
- MUST: second rule
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- MUST: do my thing
  CONFORMS: ./target@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}
	if returnNode == nil {
		t.Fatal("RETURN slot not found")
	}

	// Should have 3: original (stripped CONFORMS) + 2 inlined
	if len(returnNode.Content) != 3 {
		t.Fatalf("expected 3 obligations, got %d", len(returnNode.Content))
	}

	// First obligation should be the carrier without CONFORMS
	firstObl := returnNode.Content[0]
	for i := 0; i < len(firstObl.Content)-1; i += 2 {
		if firstObl.Content[i].Value == "CONFORMS" {
			t.Error("carrier obligation should have CONFORMS stripped")
		}
	}

	// Check it still has MUST
	hasMust := false
	for i := 0; i < len(firstObl.Content)-1; i += 2 {
		if firstObl.Content[i].Value == "MUST" {
			hasMust = true
		}
	}
	if !hasMust {
		t.Error("carrier obligation should still have MUST")
	}
}

func TestInlineConforms_WhenGuardPreserved(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: do thing
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- WHEN: condition holds
  MUST: do my thing
  CONFORMS: ./target@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	// The inlined obligation should have the outer WHEN guard applied
	if len(returnNode.Content) < 2 {
		t.Fatalf("expected at least 2 obligations, got %d", len(returnNode.Content))
	}

	inlined := returnNode.Content[1]
	whenVal := ""
	for i := 0; i < len(inlined.Content)-1; i += 2 {
		if inlined.Content[i].Value == "WHEN" {
			whenVal = inlined.Content[i+1].Value
		}
	}
	if whenVal != "condition holds" {
		t.Errorf("expected WHEN='condition holds' on inlined, got %q", whenVal)
	}
}

func TestInlineConforms_WhenGuardCombined(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- WHEN: inner condition
  MUST: do thing
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- WHEN: outer condition
  MUST: do my thing
  CONFORMS: ./target@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	// The inlined obligation should have combined WHEN: "outer condition and inner condition"
	inlined := returnNode.Content[1]
	whenVal := ""
	for i := 0; i < len(inlined.Content)-1; i += 2 {
		if inlined.Content[i].Value == "WHEN" {
			whenVal = inlined.Content[i+1].Value
		}
	}
	if whenVal != "outer condition and inner condition" {
		t.Errorf("expected WHEN='outer condition and inner condition', got %q", whenVal)
	}
}

func TestInlineConforms_ProvenanceComment(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: exit 0
`
	refFile := filepath.Join(dir, "cli.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./cli@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) < 1 {
		t.Fatal("expected at least 1 obligation")
	}

	// Check provenance is exactly as in source, byte-for-byte
	if returnNode.Content[0].HeadComment != "CONFORMS: ./cli@Target::RETURN" {
		t.Errorf("provenance comment should be 'CONFORMS: ./cli@Target::RETURN', got %q",
			returnNode.Content[0].HeadComment)
	}
}

func TestInlineConforms_NoSlotError(t *testing.T) {
	dir := t.TempDir()

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: SomeSpec
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	_, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) == 0 {
		t.Fatal("expected error for CONFORMS without ::SLOT")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, "yass.query.conforms_no_slot") {
		t.Errorf("expected conforms_no_slot error, got: %s", errStr)
	}
}

func TestInlineConforms_UsesNotInlined(t *testing.T) {
	dir := t.TempDir()

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
  USES: SomeRef
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// USES should remain untouched
	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation (USES not inlined), got %d", len(returnNode.Content))
	}
}

func TestInlineConforms_SEENotInlined(t *testing.T) {
	dir := t.TempDir()

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
  SEE: SomeRef
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation (SEE not inlined), got %d", len(returnNode.Content))
	}
}

func TestInlineConforms_KeepsNonConformsRefs(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: rule one
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- MUST: my rule
  USES: SomeRef
  CONFORMS: ./target@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	// First obligation (carrier) should have USES but not CONFORMS
	carrier := returnNode.Content[0]
	hasUses := false
	hasConforms := false
	for i := 0; i < len(carrier.Content)-1; i += 2 {
		switch carrier.Content[i].Value {
		case "USES":
			hasUses = true
		case "CONFORMS":
			hasConforms = true
		}
	}
	if !hasUses {
		t.Error("carrier should keep USES ref")
	}
	if hasConforms {
		t.Error("carrier should have CONFORMS stripped")
	}
}

func TestInlineConforms_CrossFileResolution(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: cross-file rule
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: target@Target::RETURN
`
	srcFile := filepath.Join(subDir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	// projectRoot is dir, basePath is derived from srcFile's directory
	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation from cross-file ref, got %d", len(returnNode.Content))
	}

	// Check the inlined obligation has the right content
	inlined := returnNode.Content[0]
	for i := 0; i < len(inlined.Content)-1; i += 2 {
		if inlined.Content[i].Value == "MUST" {
			if inlined.Content[i+1].Value != "cross-file rule" {
				t.Errorf("expected 'cross-file rule', got %q", inlined.Content[i+1].Value)
			}
		}
	}
}

func TestInlineConforms_PreservesNormativity(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
RETURN:
- MUST: required thing
- SHOULD: recommended thing
- MAY: optional thing
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./target@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) != 3 {
		t.Fatalf("expected 3 obligations, got %d", len(returnNode.Content))
	}

	// Check normativity keywords are preserved
	expected := []string{"MUST", "SHOULD", "MAY"}
	for idx, obl := range returnNode.Content {
		found := false
		for i := 0; i < len(obl.Content)-1; i += 2 {
			if obl.Content[i].Value == expected[idx] {
				found = true
			}
		}
		if !found {
			t.Errorf("obligation %d should have %s keyword", idx, expected[idx])
		}
	}
}

func TestInlineConforms_UnresolvableRef(t *testing.T) {
	dir := t.TempDir()

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./nonexistent@Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	_, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) == 0 {
		t.Fatal("expected error for unresolvable CONFORMS ref")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, "yass.query.conforms_unresolved") {
		t.Errorf("expected conforms_unresolved error, got: %s", errStr)
	}
}

func TestInlineConforms_NoRecursion(t *testing.T) {
	dir := t.TempDir()

	// Target has its own CONFORMS ref - should NOT be resolved recursively
	refContent := `---
description: ref
version: v1
---
spec: Level1
RETURN:
- MUST: level1 rule
  CONFORMS: ./deeper@Level2::RETURN
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./target@Level1::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	// Should have 1 inlined obligation which still carries its own CONFORMS
	if len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation (no recursion), got %d", len(returnNode.Content))
	}

	// The inlined obligation should still have its CONFORMS ref intact
	inlined := returnNode.Content[0]
	hasConforms := false
	for i := 0; i < len(inlined.Content)-1; i += 2 {
		if inlined.Content[i].Value == "CONFORMS" {
			hasConforms = true
		}
	}
	if !hasConforms {
		t.Error("inlined obligation should retain its own CONFORMS (no recursive inlining)")
	}
}

func TestInlineConforms_SameFileRef(t *testing.T) {
	dir := t.TempDir()

	srcContent := `---
description: src
version: v1
---
spec: Target
RETURN:
- MUST: target rule
---
spec: MySpec
RETURN:
- CONFORMS: Target::RETURN
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)
	var returnNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
			break
		}
	}

	if len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation from same-file ref, got %d", len(returnNode.Content))
	}

	// Check the inlined obligation
	inlined := returnNode.Content[0]
	for i := 0; i < len(inlined.Content)-1; i += 2 {
		if inlined.Content[i].Value == "MUST" {
			if inlined.Content[i+1].Value != "target rule" {
				t.Errorf("expected 'target rule', got %q", inlined.Content[i+1].Value)
			}
		}
	}
}

func TestInlineConforms_MultipleSlots(t *testing.T) {
	dir := t.TempDir()

	refContent := `---
description: ref
version: v1
---
spec: Target
ERROR:
- MUST: report errors
RETURN:
- MUST: return success
`
	refFile := filepath.Join(dir, "target.yass.yaml")
	if err := os.WriteFile(refFile, []byte(refContent), 0644); err != nil {
		t.Fatalf("writing ref file: %v", err)
	}

	srcContent := `---
description: src
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./target@Target::RETURN
ERROR:
- CONFORMS: ./target@Target::ERROR
`
	srcFile := filepath.Join(dir, "src.yass.yaml")
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing src file: %v", err)
	}

	docs := parseYAML(t, srcContent)
	specDoc := findSpec(t, docs, "MySpec")

	result, errs := InlineConforms(specDoc, srcFile, dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mapping := getMappingNode(result)

	// Check RETURN slot
	var returnNode, errorNode *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "RETURN" {
			returnNode = mapping.Content[i+1]
		}
		if mapping.Content[i].Value == "ERROR" {
			errorNode = mapping.Content[i+1]
		}
	}

	if returnNode == nil || len(returnNode.Content) != 1 {
		t.Fatalf("expected 1 obligation in RETURN, got %d", len(returnNode.Content))
	}
	if errorNode == nil || len(errorNode.Content) != 1 {
		t.Fatalf("expected 1 obligation in ERROR, got %d", len(errorNode.Content))
	}
}

func TestCloneNode_Deep(t *testing.T) {
	original := &yaml.Node{
		Kind:  yaml.MappingNode,
		Value: "test",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "key"},
			{Kind: yaml.ScalarNode, Value: "value"},
		},
	}

	clone := cloneNode(original)

	// Mutating clone should not affect original
	clone.Value = "changed"
	clone.Content[1].Value = "changed_value"

	if original.Value != "test" {
		t.Error("original Value was mutated")
	}
	if original.Content[1].Value != "value" {
		t.Error("original Content was mutated")
	}
}
