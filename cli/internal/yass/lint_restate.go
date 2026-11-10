package yass

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// The restatement advisories of 09-lint@Lint::RETURN find an obligation that
// states a fact a catalog design already records. They match fixed patterns
// and weigh nothing they match.
//
//   - A catalogue is every `catalog` design of the project file set. Its
//     identifiers are its record keys, matched case-sensitively as whole
//     tokens; a key of digits alone is never an identifier.
//   - A field is vocabulary-backed when every value its records hold is a key
//     of exactly one other catalogue, its vocabulary. Each key of the
//     vocabulary is then a value of the field, and each field value of the
//     keyed record is a further wording of it.
//   - A wording of one word is stated only within ContextWindow words of a
//     context word: the field's name or, for a vocabulary-backed field, a word
//     of the vocabulary's name. A wording of several words is stated as a
//     whole phrase anywhere. A vocabulary wording of at most PhraseWords words
//     is also stated when every one of its content words occurs.
//   - yass.lint.restated_fact: an obligation names an identifier and states a
//     value its catalogue's field can take, or, for a vocabulary-backed field,
//     borrows another case's outcome with `as when`, `as for`, `as if`, or
//     `as though`.
//   - yass.lint.restated_value: an obligation names no identifier of a
//     catalogue, carries no USES edge to it or to the vocabulary, and states a
//     value some record of it holds in a vocabulary-backed field.

// PhraseWords is the most words a vocabulary wording may hold and still be
// stated by its content words alone: a longer value is prose whose words recur
// everywhere.
const PhraseWords = 8

// ContextWindow is how many words from a context word a one-word wording may
// stand: `exit with the usage status` holds `usage` three words from `exit`.
const ContextWindow = 3

// phraseStop holds the words of four letters or more that carry no content.
var phraseStop = map[string]bool{
	"about": true, "also": true, "been": true, "does": true, "each": true, "from": true,
	"have": true, "into": true, "only": true, "other": true, "some": true, "than": true,
	"that": true, "their": true, "then": true, "there": true, "this": true, "what": true,
	"when": true, "which": true, "with": true, "were": true,
}

var (
	// wordRe matches a word whose stem a content-word wording compares.
	wordRe = regexp.MustCompile(`[a-z0-9]+`)
	// comparison matches an outcome borrowed from another case, such as `fail
	// as when the file does not exist`.
	comparison = regexp.MustCompile(`\bas (when|for|if|though)\b`)
)

// wording is one way a field value may be written.
type wording struct {
	phrase  string   // lower-cased, whitespace-collapsed, when matched as written
	content []string // stems that must all occur, when matched by content words
	source  string   // the value as written, for a message
}

// same reports whether two wordings match the same text the same way.
func (w wording) same(o wording) bool {
	return w.phrase == o.phrase && strings.Join(w.content, " ") == strings.Join(o.content, " ") &&
		(w.content == nil) == (o.content == nil)
}

// shown is the wording as a message names it.
func (w wording) shown() string {
	if w.content != nil {
		return w.source
	}
	return w.phrase
}

// fieldDomain holds the values one field of a catalogue can take, each with
// the wordings that state it.
type fieldDomain struct {
	name    string
	vocab   *catalogue // the catalogue whose keys the values are, or nil
	context map[string]bool
	values  []string             // every value, in a stable order
	words   map[string][]wording // value -> its wordings
	used    map[string]bool      // values some record holds
}

// catalogue is one catalog design read for the restatement advisories.
type catalogue struct {
	doc    *Doc
	keys   []string
	recs   map[string]map[string]string // key -> field -> value
	fields []*fieldDomain
}

// catalogues reads every catalog design of the parsed files. A record that is
// not a mapping, a repeated identifier, and a field value that is not a scalar
// are skipped, since validation reports each.
func catalogues(files []*File) []*catalogue {
	var out []*catalogue
	for _, f := range files {
		if !f.Parsed {
			continue
		}
		for _, d := range f.Docs {
			if d.Kind != KindDesign || !d.Addressable() || d.Type != "catalog" || !d.IsCatalog {
				continue
			}
			c := &catalogue{doc: d, recs: map[string]map[string]string{}}
			var names []string
			for _, r := range d.Catalog {
				if r.Kn.Kind != yaml.ScalarNode || r.Vn.Kind != yaml.MappingNode || c.recs[r.Key] != nil {
					continue
				}
				fields := map[string]string{}
				for _, fl := range r.Fields {
					if fl.Vn.Kind != yaml.ScalarNode {
						continue
					}
					if !contains(names, fl.Name) {
						names = append(names, fl.Name)
					}
					fields[fl.Name] = fl.Vn.Value
				}
				c.keys = append(c.keys, r.Key)
				c.recs[r.Key] = fields
			}
			for _, n := range names {
				c.fields = append(c.fields, &fieldDomain{name: n})
			}
			out = append(out, c)
		}
	}
	for _, c := range out {
		for _, fd := range c.fields {
			fd.build(c, out)
		}
	}
	return out
}

// build fills a field's domain: the values its records hold and, when the
// field is vocabulary-backed, every key of the vocabulary with the field
// values of its record as further wordings.
func (fd *fieldDomain) build(c *catalogue, all []*catalogue) {
	fd.words = map[string][]wording{}
	fd.used = map[string]bool{}
	fd.context = map[string]bool{}
	for _, w := range nameWords(fd.name) {
		fd.context[w] = true
	}
	for _, k := range c.keys {
		if v, ok := c.recs[k][fd.name]; ok && strings.TrimSpace(v) != "" {
			fd.used[v] = true
		}
	}
	add := func(v string, w wording) {
		if w.phrase == "" && w.content == nil {
			return
		}
		for _, e := range fd.words[v] {
			if e.same(w) {
				return
			}
		}
		if _, ok := fd.words[v]; !ok {
			fd.values = append(fd.values, v)
		}
		fd.words[v] = append(fd.words[v], w)
	}
	for _, k := range c.keys {
		if v, ok := c.recs[k][fd.name]; ok && fd.used[v] {
			add(v, literal(v))
		}
	}
	fd.vocab = vocabularyOf(c, fd.used, all)
	if fd.vocab == nil {
		return
	}
	for _, w := range nameWords(fd.vocab.doc.Name) {
		fd.context[w] = true
	}
	for _, k := range fd.vocab.keys {
		add(k, literal(k))
		for _, vf := range fd.vocab.fields {
			u, ok := fd.vocab.recs[k][vf.name]
			if !ok {
				continue
			}
			add(k, literal(u))
			if cw := contentWords(u); cw != nil {
				add(k, wording{content: cw, source: strings.Join(strings.Fields(u), " ")})
			}
		}
	}
}

// vocabularyOf returns the one catalogue other than c whose keys include
// every value in used, or nil when none does or more than one does.
func vocabularyOf(c *catalogue, used map[string]bool, all []*catalogue) *catalogue {
	if len(used) == 0 {
		return nil
	}
	var vocab *catalogue
	for _, o := range all {
		if o == c {
			continue
		}
		every := true
		for v := range used {
			if o.recs[v] == nil {
				every = false
				break
			}
		}
		if every {
			if vocab != nil {
				return nil
			}
			vocab = o
		}
	}
	return vocab
}

// nameWords splits a field or design name into the stems of its words, at
// case changes and at every character that is not a letter or a digit:
// `ExitPolicy` yields `exit` and `policy`.
func nameWords(name string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, stem(strings.ToLower(string(cur))))
			cur = nil
		}
	}
	rs := []rune(name)
	for i, r := range rs {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && i > 0 && unicode.IsLower(rs[i-1]):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

// literal is the as-written wording of a value.
func literal(v string) wording {
	return wording{phrase: strings.ToLower(strings.Join(strings.Fields(v), " "))}
}

// contentWords returns the stems of a short value's content words, or nil when
// the value is too long, or holds fewer than two content words, to be stated
// by them.
func contentWords(v string) []string {
	words := wordRe.FindAllString(strings.ToLower(v), -1)
	if len(words) > PhraseWords {
		return nil
	}
	var out []string
	for _, w := range words {
		if len(w) < 4 || phraseStop[w] || isDigits(w) {
			continue
		}
		if s := stem(w); !contains(out, s) {
			out = append(out, s)
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// stem strips one inflection and then a final `e`, so `name`, `names`,
// `named`, and `naming` meet, leaving at least StemLetters letters.
func stem(w string) string {
	for _, suf := range []string{"ing", "es", "ed", "s"} {
		if strings.HasSuffix(w, suf) && len(w)-len(suf) >= StemLetters {
			w = w[:len(w)-len(suf)]
			break
		}
	}
	if strings.HasSuffix(w, "e") && len(w)-1 >= StemLetters {
		w = w[:len(w)-1]
	}
	return w
}

// StemLetters is the fewest letters stem leaves.
const StemLetters = 3

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// tokenChar reports whether a byte continues an identifier or a word.
func tokenChar(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// joinChar reports whether a byte joins two tokens into one, as the dot of a
// code or the hyphen of `1-based` does, when a token character follows it.
func joinChar(b byte) bool { return b == '.' || b == '-' || b == '/' || b == '@' }

// wholeAt reports whether s[i:i+n] stands as a whole token of s.
func wholeAt(s string, i, n int) bool {
	if i > 0 && (tokenChar(s[i-1]) || joinChar(s[i-1]) && i > 1 && tokenChar(s[i-2])) {
		return false
	}
	j := i + n
	if j < len(s) && (tokenChar(s[j]) || joinChar(s[j]) && j+1 < len(s) && tokenChar(s[j+1])) {
		return false
	}
	return true
}

// findWhole returns the start of every whole-token occurrence of p in s.
func findWhole(s, p string) []int {
	var out []int
	if p == "" {
		return out
	}
	for i := 0; ; {
		k := strings.Index(s[i:], p)
		if k < 0 {
			return out
		}
		if wholeAt(s, i+k, len(p)) {
			out = append(out, i+k)
		}
		i += k + 1
	}
}

// prose is obligation text prepared for value matching.
type prose struct {
	low   string          // lower-cased
	words []string        // whitespace-separated words, edge punctuation trimmed
	stems map[string]bool // the stem of every word
}

func prepare(text string) prose {
	p := prose{low: strings.ToLower(text), stems: map[string]bool{}}
	for _, w := range strings.Fields(p.low) {
		p.words = append(p.words, strings.Trim(w, `.,;:()[]{}"'!?`))
	}
	for _, w := range wordRe.FindAllString(p.low, -1) {
		p.stems[stem(w)] = true
	}
	return p
}

// states reports whether the prose states a wording of a field with the given
// context words.
func (p prose) states(w wording, context map[string]bool) bool {
	if w.content != nil {
		for _, s := range w.content {
			if !p.stems[s] {
				return false
			}
		}
		return true
	}
	if strings.Contains(w.phrase, " ") {
		return len(findWhole(p.low, w.phrase)) > 0
	}
	for i, x := range p.words {
		if x != w.phrase {
			continue
		}
		for j := i - ContextWindow; j <= i+ContextWindow; j++ {
			if j < 0 || j == i || j >= len(p.words) {
				continue
			}
			for _, part := range wordRe.FindAllString(p.words[j], -1) {
				if context[stem(part)] {
					return true
				}
			}
		}
	}
	return false
}

// obligationText is the text an obligation states: its guard and its prose,
// whitespace collapsed, with backticks dropped so a quoted identifier or value
// still counts.
func obligationText(o *Obligation) string {
	var parts []string
	if o.HasGuard && o.GuardString {
		parts = append(parts, o.Guard)
	}
	if o.HasProse {
		parts = append(parts, o.Prose)
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(strings.Join(parts, " "), "`", " ")), " ")
}

// restateAdvisories returns the restatement advisories of one parsed file.
// uses gives each obligation the designs it binds with USES.
func restateAdvisories(f *File, cats []*catalogue, uses map[*Obligation][]*Doc) []Diag {
	var out []Diag
	if len(cats) == 0 {
		return out
	}
	for _, d := range f.Docs {
		if d.Kind != KindSpec || !d.Addressable() {
			continue
		}
		for _, s := range d.Slots {
			for _, o := range s.Obligations {
				text := obligationText(o)
				if text == "" {
					continue
				}
				if msg := restatedFact(text, cats); msg != "" {
					out = append(out, Diag{Severity: "warn", Code: "yass.lint.restated_fact",
						Location: s.Target(), Line: o.Line, Message: msg})
				}
				if msg := restatedValue(text, cats, uses[o]); msg != "" {
					out = append(out, Diag{Severity: "warn", Code: "yass.lint.restated_value",
						Location: s.Target(), Line: o.Line, Message: msg})
				}
			}
		}
	}
	return out
}

// named returns the identifiers of c that text names, longest first. A key of
// digits alone names nothing: prose writes a number as a quantity far more
// often than as the name of a record, and such a key is still a wording of a
// vocabulary-backed value.
func (c *catalogue) named(text string) []string {
	var out []string
	for _, k := range c.keys {
		if !isDigits(k) && len(findWhole(text, k)) > 0 {
			out = append(out, k)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// blank replaces every whole-token occurrence of each identifier with
// underscores, so a value is never matched inside the identifier that names
// its record and the identifier still counts as one word of the context window.
func blank(text string, ids []string) string {
	b := []byte(text)
	for _, id := range ids {
		for _, i := range findWhole(text, id) {
			for j := i; j < i+len(id); j++ {
				b[j] = '_'
			}
		}
	}
	return string(b)
}

// restatedFact reports an identifier the text names together with a value
// its catalogue's field can take, preferring a value that conflicts with the
// record. A stated value conflicts only when no named identifier whose record
// holds the field holds that value, so an obligation naming two identifiers
// may state the value of each.
func restatedFact(text string, cats []*catalogue) string {
	var agree string
	for _, c := range cats {
		ids := c.named(text)
		if len(ids) == 0 {
			continue
		}
		p := prepare(blank(text, ids))
		vocab := false
		for _, fd := range c.fields {
			if fd.vocab != nil {
				vocab = true
			}
			var holders []string // named identifiers whose record holds the field
			for _, id := range ids {
				if _, ok := c.recs[id][fd.name]; ok {
					holders = append(holders, id)
				}
			}
			if len(holders) == 0 {
				continue
			}
			for _, v := range fd.values {
				var hit *wording
				for i, w := range fd.words[v] {
					if p.states(w, fd.context) {
						hit = &fd.words[v][i]
						break
					}
				}
				if hit == nil {
					continue
				}
				owner := ""
				for _, id := range holders {
					if c.recs[id][fd.name] == v {
						owner = id
						break
					}
				}
				if owner == "" {
					id := holders[0]
					return "obligation states " + hit.shown() + " for field " + fd.name + " of " + id +
						", which catalog " + c.doc.Name + " records as " + c.recs[id][fd.name]
				}
				if agree == "" {
					agree = "obligation restates field " + fd.name + " of " + owner +
						", which catalog " + c.doc.Name + " records"
				}
			}
		}
		if vocab && agree == "" && comparison.MatchString(p.low) {
			agree = "obligation borrows another case's outcome for " + ids[0] +
				", which catalog " + c.doc.Name + " records"
		}
	}
	return agree
}

// restatedValue reports a value some record holds in a vocabulary-backed
// field, stated by an obligation that names no identifier of that record's
// catalogue and carries no USES edge to it or to the vocabulary.
func restatedValue(text string, cats []*catalogue, bound []*Doc) string {
	var p *prose
	for _, c := range cats {
		if len(c.named(text)) > 0 || binds(bound, c.doc) {
			continue
		}
		for _, fd := range c.fields {
			if fd.vocab == nil || binds(bound, fd.vocab.doc) {
				continue
			}
			if p == nil {
				pp := prepare(text)
				p = &pp
			}
			for _, v := range fd.values {
				if !fd.used[v] {
					continue
				}
				for _, w := range fd.words[v] {
					if p.states(w, fd.context) {
						return "obligation states " + w.shown() + ", a value of field " + fd.name +
							" in catalog " + c.doc.Name + ", without naming an identifier"
					}
				}
			}
		}
	}
	return ""
}

func binds(docs []*Doc, d *Doc) bool {
	for _, x := range docs {
		if x == d {
			return true
		}
	}
	return false
}
