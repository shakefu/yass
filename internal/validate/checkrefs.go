package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/parser"
)

// refTargetGrammar matches the full ref target grammar:
// optional_path@spec_name::optional_SLOT
var refTargetGrammar = regexp.MustCompile(`^([A-Za-z0-9._/\-]+@)?[A-Za-z0-9._\-]+(::[A-Z\-]+)?$`)

// cachedFile holds a parsed file result or an error flag.
type cachedFile struct {
	docs []*yaml.Node
	err  bool // true if file could not be read/parsed
}

// CheckRefs validates all reference targets in all specs within a file.
// It checks CONFORMS, USES, and SEE values for validity.
func CheckRefs(docs []*yaml.Node, filePath string, basePath string, projectRoot string) []ValidationError {
	var errs []ValidationError

	// Collect spec names in this file for same-file ref checks.
	localSpecs := make(map[string]*yaml.Node)
	for _, doc := range docs {
		name, _ := extractSpecName(doc)
		if name != "" {
			if _, exists := localSpecs[name]; !exists {
				localSpecs[name] = doc
			}
		}
	}

	// Track file errors to deduplicate file_not_found/file_not_parseable per
	// (referencing-file, referenced-file) pair.
	fileErrors := make(map[string]bool)

	// Cache parsed referenced files.
	parsedFiles := make(map[string]*cachedFile)

	for _, doc := range docs {
		mapping := getMappingNode(doc)
		if mapping == nil {
			continue
		}
		if !docHasSpecKey(doc) {
			continue
		}

		// Walk all slots.
		for i := 0; i < len(mapping.Content)-1; i += 2 {
			keyNode := mapping.Content[i]
			valNode := mapping.Content[i+1]

			if keyNode.Kind != yaml.ScalarNode {
				continue
			}
			key := keyNode.Value
			if key == "spec" || !SlotKeys[key] {
				continue
			}
			if valNode.Kind != yaml.SequenceNode {
				continue
			}

			// Walk obligations.
			for _, oblNode := range valNode.Content {
				if oblNode.Kind != yaml.MappingNode {
					continue
				}

				for j := 0; j < len(oblNode.Content)-1; j += 2 {
					oblKeyNode := oblNode.Content[j]
					oblValNode := oblNode.Content[j+1]

					if oblKeyNode.Kind != yaml.ScalarNode {
						continue
					}

					if !ReferenceRelations[oblKeyNode.Value] {
						continue
					}

					// This is a reference. Validate its target.
					if oblValNode.Kind != yaml.ScalarNode || oblValNode.Value == "" {
						continue
					}

					target := oblValNode.Value
					refErrs := checkRefTarget(
						target,
						oblValNode.Line,
						filePath,
						basePath,
						projectRoot,
						localSpecs,
						fileErrors,
						parsedFiles,
					)
					errs = append(errs, refErrs...)
				}
			}
		}
	}

	return errs
}

// checkRefTarget validates a single reference target string.
func checkRefTarget(
	target string,
	line int,
	filePath string,
	basePath string,
	projectRoot string,
	localSpecs map[string]*yaml.Node,
	fileErrors map[string]bool,
	parsedFiles map[string]*cachedFile,
) []ValidationError {
	var errs []ValidationError

	// Check grammar.
	if !refTargetGrammar.MatchString(target) {
		return []ValidationError{
			{
				File:    filePath,
				Line:    line,
				Code:    yerrors.CodeRefMalformed,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefMalformed], target),
			},
		}
	}

	// Parse the ref target into path, specName, slot.
	path, specName, slot := parseRefTarget(target)

	// Check slot validity.
	if slot != "" && !SlotKeys[slot] {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    line,
			Code:    yerrors.CodeRefUnknownSlot,
			Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefUnknownSlot], slot),
		})
		return errs
	}

	if path == "" {
		// Same-file ref.
		specDoc, exists := localSpecs[specName]
		if !exists {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    line,
				Code:    yerrors.CodeRefSpecNotFoundSameFile,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefSpecNotFoundSameFile], target),
			})
			return errs
		}

		// Check slot declaration.
		if slot != "" {
			if !specDeclaresSlot(specDoc, slot) {
				errs = append(errs, ValidationError{
					File:    filePath,
					Line:    line,
					Code:    yerrors.CodeRefSlotNotDeclared,
					Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefSlotNotDeclared], target),
				})
			}
		}
	} else {
		// Cross-file ref.
		var resolvedPath string
		if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
			resolvedPath = filepath.Join(basePath, path+".yass.yaml")
		} else {
			resolvedPath = filepath.Join(projectRoot, path+".yass.yaml")
		}
		resolvedPath = filepath.Clean(resolvedPath)

		// Check cache.
		cached, hasCached := parsedFiles[resolvedPath]
		if !hasCached {
			// Check if file exists first.
			if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				cached = &cachedFile{nil, true}
				parsedFiles[resolvedPath] = cached

				if !fileErrors[resolvedPath] {
					fileErrors[resolvedPath] = true
					errs = append(errs, ValidationError{
						File:    filePath,
						Line:    line,
						Code:    yerrors.CodeRefFileNotFound,
						Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefFileNotFound], target),
					})
				}
				return errs
			}

			result, err := parser.ParseFile(resolvedPath)
			if err != nil {
				cached = &cachedFile{nil, true}
				parsedFiles[resolvedPath] = cached

				if !fileErrors[resolvedPath] {
					fileErrors[resolvedPath] = true
					errs = append(errs, ValidationError{
						File:    filePath,
						Line:    line,
						Code:    yerrors.CodeRefFileNotParseable,
						Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefFileNotParseable], target),
					})
				}
				return errs
			}
			cached = &cachedFile{result.Documents, false}
			parsedFiles[resolvedPath] = cached
		}

		if cached.err {
			// Already reported error for this file — deduplicate.
			return errs
		}

		// Look for spec in the referenced file.
		var foundSpec *yaml.Node
		for _, doc := range cached.docs {
			name, _ := extractSpecName(doc)
			if name == specName {
				foundSpec = doc
				break
			}
		}

		if foundSpec == nil {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    line,
				Code:    yerrors.CodeRefSpecNotFoundOtherFile,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefSpecNotFoundOtherFile], target),
			})
			return errs
		}

		// Check slot declaration.
		if slot != "" {
			if !specDeclaresSlot(foundSpec, slot) {
				errs = append(errs, ValidationError{
					File:    filePath,
					Line:    line,
					Code:    yerrors.CodeRefSlotNotDeclared,
					Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeRefSlotNotDeclared], target),
				})
			}
		}
	}

	return errs
}

// parseRefTarget parses a ref target into its components.
// Grammar: (path@)?specName(::SLOT)?
func parseRefTarget(target string) (path, specName, slot string) {
	// Extract slot.
	if idx := strings.Index(target, "::"); idx >= 0 {
		slot = target[idx+2:]
		target = target[:idx]
	}

	// Extract path.
	if idx := strings.LastIndex(target, "@"); idx >= 0 {
		path = target[:idx]
		specName = target[idx+1:]
	} else {
		specName = target
	}

	return
}

// specDeclaresSlot checks if a spec document declares the given slot key.
func specDeclaresSlot(doc *yaml.Node, slot string) bool {
	mapping := getMappingNode(doc)
	if mapping == nil {
		return false
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == slot {
			return true
		}
	}

	return false
}
