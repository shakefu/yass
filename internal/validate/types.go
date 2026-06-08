package validate

import "regexp"

// ValidationError holds structured validation error information.
type ValidationError struct {
	File    string
	Line    int
	Code    string
	Message string
}

// Valid slot keys.
var SlotKeys = map[string]bool{
	"INPUT":       true,
	"RETURN":      true,
	"ERROR":       true,
	"SIDE-EFFECT": true,
	"INVARIANT":   true,
}

// NormativityKeywords are the recognized normativity keywords.
var NormativityKeywords = map[string]bool{
	"MUST":       true,
	"MUST-NOT":   true,
	"SHOULD":     true,
	"SHOULD-NOT": true,
	"MAY":        true,
}

// ReferenceRelations are the recognized reference relation keys.
var ReferenceRelations = map[string]bool{
	"CONFORMS": true,
	"USES":     true,
	"SEE":      true,
}

// SpecNameRegex validates the composition form of a spec name.
var SpecNameRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// SpecNameAllowedChars validates that a spec name contains only allowed characters.
var SpecNameAllowedChars = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ReservedNames is the set of reserved names (case-insensitive, stored uppercase).
// It includes all slot keys and normativity keywords.
var ReservedNames map[string]bool

func init() {
	ReservedNames = make(map[string]bool)
	for k := range SlotKeys {
		ReservedNames[k] = true
	}
	for k := range NormativityKeywords {
		ReservedNames[k] = true
	}
}
