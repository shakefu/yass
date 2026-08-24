package yass

import (
	"unicode"
	"unicode/utf8"
)

// foldEqual reports whether two runes are equal under simple Unicode case
// folding. Simple folding, not full folding: ß does not fold to ss.
func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

func hasFoldPrefix(s, prefix string) bool {
	for len(prefix) > 0 {
		if len(s) == 0 {
			return false
		}
		pr, psz := utf8.DecodeRuneInString(prefix)
		sr, ssz := utf8.DecodeRuneInString(s)
		if !foldEqual(sr, pr) {
			return false
		}
		s, prefix = s[ssz:], prefix[psz:]
	}
	return true
}

// foldContains reports whether pattern occurs in s as a literal substring,
// compared case-insensitively under simple Unicode case folding.
func foldContains(s, pattern string) bool {
	if pattern == "" {
		return true
	}
	for i := range s {
		if hasFoldPrefix(s[i:], pattern) {
			return true
		}
	}
	return false
}
