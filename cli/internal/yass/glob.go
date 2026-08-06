package yass

// globMatch reports whether pattern matches s over its whole length, with `*`
// matching any run of characters including none, `?` matching exactly one
// character, and every other character matching itself. `*` is deliberately
// allowed to cross `/`, which the standard path matchers refuse.
func globMatch(pattern, s string) bool {
	p := []rune(pattern)
	t := []rune(s)
	pi, ti := 0, 0
	star, mark := -1, 0
	for ti < len(t) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == t[ti]):
			pi++
			ti++
		case pi < len(p) && p[pi] == '*':
			star = pi
			mark = ti
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// matchesAny reports whether ref matches any one of the given patterns. An
// empty pattern list matches everything.
func matchesAny(patterns []string, ref string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if globMatch(p, ref) {
			return true
		}
	}
	return false
}
