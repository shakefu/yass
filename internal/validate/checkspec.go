package validate

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// CheckSpec validates a single spec document (one that has a "spec" key).
// The doc parameter is a DocumentNode; Content[0] is the mapping.
// It emits one error per failing rule per obligation.
func CheckSpec(doc *yaml.Node, filePath string) []ValidationError {
	var errs []ValidationError

	mapping := getMappingNode(doc)
	if mapping == nil {
		// Not a mapping — report no_name since we can't find a spec key.
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    doc.Line,
			Code:    yerrors.CodeSpecNoName,
			Message: yerrors.Messages[yerrors.CodeSpecNoName],
		})
		return errs
	}

	// Check for "spec" key presence and value.
	specKeyFound := false
	var specValNode *yaml.Node
	var specKeyLine int

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "spec" {
			specKeyFound = true
			specValNode = mapping.Content[i+1]
			specKeyLine = keyNode.Line
			break
		}
	}

	if !specKeyFound {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    doc.Line,
			Code:    yerrors.CodeSpecNoName,
			Message: yerrors.Messages[yerrors.CodeSpecNoName],
		})
		return errs
	}

	// Validate spec name.
	nameErrors := validateSpecName(specValNode, specKeyLine, filePath)
	errs = append(errs, nameErrors...)

	// Check top-level keys. All keys other than "spec" must be valid slot keys.
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		key := keyNode.Value
		if key == "spec" {
			continue
		}
		if !SlotKeys[key] {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    keyNode.Line,
				Code:    yerrors.CodeSpecUnknownKey,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSpecUnknownKey], key),
			})
			continue
		}

		// Valid slot key — check that value is a list.
		valNode := mapping.Content[i+1]
		if valNode.Kind != yaml.SequenceNode {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    keyNode.Line,
				Code:    yerrors.CodeSlotValueNotList,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSlotValueNotList], key),
			})
			continue
		}

		// Validate each obligation in the slot.
		for _, oblNode := range valNode.Content {
			oblErrors := validateObligation(oblNode, filePath)
			errs = append(errs, oblErrors...)
		}
	}

	return errs
}

// validateSpecName validates the spec name value.
func validateSpecName(valNode *yaml.Node, keyLine int, filePath string) []ValidationError {
	var errs []ValidationError

	if valNode == nil {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    keyLine,
			Code:    yerrors.CodeSpecNameEmpty,
			Message: yerrors.Messages[yerrors.CodeSpecNameEmpty],
		})
		return errs
	}

	// Check if the value is a scalar string.
	if valNode.Kind != yaml.ScalarNode {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameNotString,
			Message: yerrors.Messages[yerrors.CodeSpecNameNotString],
		})
		return errs
	}

	// Check tag — must be a string type.
	if valNode.Tag != "" && valNode.Tag != "!!str" {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameNotString,
			Message: yerrors.Messages[yerrors.CodeSpecNameNotString],
		})
		return errs
	}

	name := valNode.Value

	// Check empty name.
	if name == "" {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameEmpty,
			Message: yerrors.Messages[yerrors.CodeSpecNameEmpty],
		})
		return errs
	}

	// Check allowed characters.
	if !SpecNameAllowedChars.MatchString(name) {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameBadChars,
			Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSpecNameBadChars], name),
		})
		return errs
	}

	// Check composition form.
	if !SpecNameRegex.MatchString(name) {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameBadForm,
			Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSpecNameBadForm], name),
		})
		return errs
	}

	// Check reserved names (case-insensitive).
	upper := strings.ToUpper(name)
	if ReservedNames[upper] {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeSpecNameReserved,
			Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSpecNameReserved], name),
		})
	}

	return errs
}

// validateObligation validates a single obligation node within a slot.
func validateObligation(oblNode *yaml.Node, filePath string) []ValidationError {
	var errs []ValidationError

	// An obligation must be a mapping.
	if oblNode.Kind != yaml.MappingNode {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    oblNode.Line,
			Code:    yerrors.CodeObligationBadValueShape,
			Message: yerrors.Messages[yerrors.CodeObligationBadValueShape],
		})
		return errs
	}

	hasNormativity := false
	hasReference := false
	hasGuard := false
	normativityCount := 0
	refSeen := make(map[string]bool)

	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		valNode := oblNode.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode {
			continue
		}

		key := keyNode.Value

		if key == "WHEN" {
			hasGuard = true
			// Check WHEN value shape — must be a scalar string, not mapping/sequence/null.
			if err := checkScalarValue(valNode, filePath); err != nil {
				errs = append(errs, *err)
			}
			continue
		}

		if NormativityKeywords[key] {
			hasNormativity = true
			normativityCount++
			if normativityCount > 1 {
				errs = append(errs, ValidationError{
					File:    filePath,
					Line:    keyNode.Line,
					Code:    yerrors.CodeObligationDuplicateNormativity,
					Message: yerrors.Messages[yerrors.CodeObligationDuplicateNormativity],
				})
			}
			// Check normativity value shape.
			if err := checkScalarValue(valNode, filePath); err != nil {
				errs = append(errs, *err)
			}
			continue
		}

		if ReferenceRelations[key] {
			hasReference = true
			if refSeen[key] {
				errs = append(errs, ValidationError{
					File:    filePath,
					Line:    keyNode.Line,
					Code:    yerrors.CodeObligationDuplicateReference,
					Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeObligationDuplicateReference], key),
				})
			}
			refSeen[key] = true
			// Check reference value shape.
			if err := checkScalarValue(valNode, filePath); err != nil {
				errs = append(errs, *err)
			}
			continue
		}

		// Unknown key at obligation level — classify as unknown reference
		// relation or unknown normativity using heuristics.
		if looksLikeReferenceRelation(key) {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    keyNode.Line,
				Code:    yerrors.CodeReferenceUnknownRelation,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeReferenceUnknownRelation], key),
			})
		} else {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    keyNode.Line,
				Code:    yerrors.CodeNormativityUnknown,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeNormativityUnknown], key),
			})
		}
	}

	// Check missing_normativity_or_ref.
	if !hasNormativity && !hasReference {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    oblNode.Line,
			Code:    yerrors.CodeObligationMissingNormativityOrRef,
			Message: yerrors.Messages[yerrors.CodeObligationMissingNormativityOrRef],
		})
	}

	// Check guard_without_normativity.
	if hasGuard && !hasNormativity {
		errs = append(errs, ValidationError{
			File:    filePath,
			Line:    oblNode.Line,
			Code:    yerrors.CodeObligationGuardWithoutNormativity,
			Message: yerrors.Messages[yerrors.CodeObligationGuardWithoutNormativity],
		})
	}

	return errs
}

// checkScalarValue checks that a value node at a scalar position is not
// a mapping, sequence, or null. Returns a ValidationError pointer or nil.
func checkScalarValue(valNode *yaml.Node, filePath string) *ValidationError {
	if valNode == nil {
		return nil
	}

	switch valNode.Kind {
	case yaml.MappingNode, yaml.SequenceNode:
		return &ValidationError{
			File:    filePath,
			Line:    valNode.Line,
			Code:    yerrors.CodeObligationBadValueShape,
			Message: yerrors.Messages[yerrors.CodeObligationBadValueShape],
		}
	case yaml.ScalarNode:
		if valNode.Tag == "!!null" || (valNode.Value == "" && valNode.Tag == "") {
			// Null value.
			return &ValidationError{
				File:    filePath,
				Line:    valNode.Line,
				Code:    yerrors.CodeObligationBadValueShape,
				Message: yerrors.Messages[yerrors.CodeObligationBadValueShape],
			}
		}
	}

	return nil
}

// looksLikeReferenceRelation uses heuristics to determine if an unknown key
// at the obligation level is likely a misspelled reference relation rather than
// a misspelled normativity keyword. Reference relations are title-case words
// like CONFORMS, USES, SEE. Normativity keywords contain a hyphen (MUST-NOT,
// SHOULD-NOT) or are the canonical set (MUST, SHOULD, MAY).
//
// Heuristic: if the key is a close match (by prefix or case variation) to a
// known reference relation, classify it as an unknown reference relation.
func looksLikeReferenceRelation(key string) bool {
	upper := strings.ToUpper(key)

	// Check if it's a case variation or prefix of a known reference relation.
	for rel := range ReferenceRelations {
		// Case-insensitive exact match (e.g., "Conforms", "conforms", "CONFORM").
		if upper == rel {
			return true
		}
		// Prefix match (e.g., "CONFORM" is a prefix of "CONFORMS").
		if len(upper) >= 2 && strings.HasPrefix(rel, upper) {
			return true
		}
		// The key is a prefix of the uppercase key (e.g., "USE" prefix of "USES").
		if len(upper) >= 2 && strings.HasPrefix(upper, rel) {
			return true
		}
	}

	// Not close to any known reference relation.
	return false
}
