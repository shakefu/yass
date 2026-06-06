package parser

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestParseBytes_ValidYAML(t *testing.T) {
	data := []byte("key: value\n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(result.Documents))
	}
	if result.Documents[0].Kind != yaml.DocumentNode {
		t.Errorf("expected DocumentNode, got %v", result.Documents[0].Kind)
	}
}

func TestParseBytes_MultiDocument(t *testing.T) {
	data := []byte("---\nkey1: value1\n---\nkey2: value2\n---\nkey3: value3\n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Documents) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(result.Documents))
	}
}

func TestParseBytes_NotUTF8(t *testing.T) {
	// 0xFF 0xFE is not valid UTF-8.
	data := []byte{0xFF, 0xFE, 0x00}
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for non-UTF-8 input")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlNotUTF8 {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlNotUTF8, pe.Code)
	}
}

func TestParseBytes_UTF8BOM(t *testing.T) {
	// Valid UTF-8 with BOM prefix.
	data := []byte{0xEF, 0xBB, 0xBF}
	data = append(data, []byte("key: value\n")...)
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for UTF-8 BOM")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlHasBOM {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlHasBOM, pe.Code)
	}
}

func TestParseBytes_EmptyFile(t *testing.T) {
	data := []byte{}
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlEmptyFile {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlEmptyFile, pe.Code)
	}
}

func TestParseBytes_MalformedYAML(t *testing.T) {
	data := []byte("key: [unclosed\n")
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlMalformed {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlMalformed, pe.Code)
	}
}

func TestParseBytes_PriorityOrder_NonUTF8BeforeBOM(t *testing.T) {
	// Non-UTF-8 that also starts with BOM-like bytes but is still invalid UTF-8
	// The BOM bytes themselves (EF BB BF) ARE valid UTF-8, so we need data
	// that has the BOM but is not valid UTF-8 overall.
	data := []byte{0xEF, 0xBB, 0xBF, 0xFF, 0xFE}
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	// Should be not_utf8 because UTF-8 check comes before BOM check.
	if pe.Code != yerrors.CodeYamlNotUTF8 {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlNotUTF8, pe.Code)
	}
}

func TestParseBytes_YesNoOnOff_TreatedAsStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"yes lowercase", "key: yes\n", "yes"},
		{"no lowercase", "key: no\n", "no"},
		{"on lowercase", "key: on\n", "on"},
		{"off lowercase", "key: off\n", "off"},
		{"Yes titlecase", "key: Yes\n", "Yes"},
		{"NO uppercase", "key: \"NO\"\n", "NO"},
		{"On titlecase", "key: On\n", "On"},
		{"OFF uppercase", "key: OFF\n", "OFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBytes([]byte(tt.input), "test.yaml")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Documents) != 1 {
				t.Fatalf("expected 1 document, got %d", len(result.Documents))
			}
			// Navigate: DocumentNode -> MappingNode -> value scalar
			doc := result.Documents[0]
			if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
				t.Fatalf("expected DocumentNode with content")
			}
			mapping := doc.Content[0]
			if mapping.Kind != yaml.MappingNode || len(mapping.Content) < 2 {
				t.Fatalf("expected MappingNode with at least 2 children")
			}
			valNode := mapping.Content[1]
			if valNode.Tag == "!!bool" {
				t.Errorf("expected tag to be !!str, got !!bool for value %q", valNode.Value)
			}
		})
	}
}

func TestParseBytes_TrueFalse_RemainBooleans(t *testing.T) {
	// true and false should remain as booleans - they are booleans in YAML 1.2.
	tests := []struct {
		name  string
		input string
	}{
		{"true", "key: true\n"},
		{"false", "key: false\n"},
		{"True", "key: True\n"},
		{"FALSE", "key: FALSE\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBytes([]byte(tt.input), "test.yaml")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			doc := result.Documents[0]
			mapping := doc.Content[0]
			valNode := mapping.Content[1]
			if valNode.Tag != "!!bool" {
				t.Errorf("expected tag !!bool for %s, got %s", tt.name, valNode.Tag)
			}
		})
	}
}

func TestParseBytes_RawBytesPreserved(t *testing.T) {
	data := []byte("key: value\n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.RawBytes) != string(data) {
		t.Errorf("RawBytes mismatch: got %q, want %q", result.RawBytes, data)
	}
}

func TestParseBytes_SingleDocumentNoDashes(t *testing.T) {
	data := []byte("hello: world\nfoo: bar\n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(result.Documents))
	}
}

func TestParseBytes_WhitespaceOnlyIsNotEmpty(t *testing.T) {
	// Whitespace-only file is not empty (len > 0) and is valid YAML
	// that parses to zero documents.
	data := []byte("   \n  \n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents for whitespace-only file, got %d", len(result.Documents))
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yass.yaml")
	err := os.WriteFile(path, []byte("---\nkey: value\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(result.Documents))
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/file.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yass.yaml")
	err := os.WriteFile(path, []byte{}, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = ParseFile(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlEmptyFile {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlEmptyFile, pe.Code)
	}
}

func TestParseError_ErrorMethod(t *testing.T) {
	pe := &ParseError{
		Code:    yerrors.CodeYamlMalformed,
		Message: "YAML well-formedness error",
		Line:    10,
	}
	got := pe.Error()
	want := "[yass.yaml.malformed] YAML well-formedness error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestParseBytes_NestedYesNoOnOff(t *testing.T) {
	// Test that yes/no/on/off in nested structures are also fixed.
	data := []byte("outer:\n  inner: yes\n  other: off\n")
	result, err := ParseBytes(data, "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := result.Documents[0]
	// Walk to find scalar nodes with yes/off.
	var found int
	walkNodes(doc, func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && (n.Value == "yes" || n.Value == "off") {
			found++
			if n.Tag == "!!bool" {
				t.Errorf("nested value %q should not have !!bool tag", n.Value)
			}
		}
	})
	if found != 2 {
		t.Errorf("expected to find 2 yes/off values, found %d", found)
	}
}

func walkNodes(node *yaml.Node, fn func(*yaml.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for _, child := range node.Content {
		walkNodes(child, fn)
	}
}

func TestParseBytes_MalformedTabIndentation(t *testing.T) {
	// Tabs are not allowed as indentation in YAML.
	data := []byte("key:\n\tvalue: bad\n")
	_, err := ParseBytes(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for tab indentation")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Code != yerrors.CodeYamlMalformed {
		t.Errorf("expected code %s, got %s", yerrors.CodeYamlMalformed, pe.Code)
	}
}
