package yass

// lintExplanation carries the documentation requested by 13-explain@Explain.
// Examples are complete root.yass.yaml files, including deliberately invalid
// input for the parse advisory. Their detector behavior is checked in tests.
type lintExplanation struct {
	ID, Detects, Why, Remedy, Bad, Good string
}

const explainPreamble = Modeline + "\n---\ndescription: example\nversion: v1\n---\n"
const explainRoot = "spec: Root\nRETURN:\n- MUST: return a result\n"
const explainPolicy = "---\ndesign: Policy\ntype: constraint\ncontent: preserve order\n"
const explainCatalogue = `---
design: Statuses
type: catalog
content:
  failed: {meaning: the request was rejected}
---
design: Codes
type: catalog
content:
  invalid_input: {status: failed}
`

// The registry is in byte-wise ascending ID order, per 13-explain@Explain.
// Its IDs are checked against both the lint contract and emitting Go code.
var lintExplanations = []lintExplanation{
	{
		ID:      "yass.lint.design_code",
		Detects: "Source-code patterns in the string content of a schema or algorithm design, outside backtick spans.",
		Why:     "A design states a shape or computation for the implementer to write; source code invites transcription.",
		Remedy:  "Describe shapes for schemas and computations as pseudocode for algorithms. Bare assignments and calls are exempt only without braces or terminating semicolons; for each loops and guards closed by then or a colon are also exempt.",
		Bad:     explainPreamble + "spec: Root\nRETURN:\n- USES: Step\n---\ndesign: Step\ntype: algorithm\ncontent: |\n  func run() {}\n",
		Good:    explainPreamble + "spec: Root\nRETURN:\n- USES: Step\n---\ndesign: Step\ntype: algorithm\ncontent: |\n  return the result\n",
	},
	{
		ID:      "yass.lint.design_table",
		Detects: "A pipe-containing line followed by a Markdown table delimiter row in design string content or a catalogue's string field, outside backtick spans.",
		Why:     "A Markdown table is a second syntax the tools do not check; its rows belong in a catalogue.",
		Remedy:  "Represent the rows as records in a catalog design.",
		Bad:     explainPreamble + "spec: Root\nRETURN:\n- USES: Codes\n---\ndesign: Codes\ntype: constraint\ncontent: |\n  | code | status |\n  | ---- | ------ |\n  | bad  | failed |\n",
		Good:    explainPreamble + "spec: Root\nRETURN:\n- USES: Codes\n---\ndesign: Codes\ntype: catalog\ncontent:\n  bad: {status: failed}\n",
	},
	{
		ID:      "yass.lint.design_unreferenced",
		Detects: "An addressable design targeted by no USES reference anywhere in the project file set.",
		Why:     "A design that nothing binds declares normative content with no force.",
		Remedy:  "Bind the design with USES from the obligation it constrains, or remove content that supplies no contract.",
		Bad:     explainPreamble + explainRoot + explainPolicy,
		Good:    explainPreamble + "spec: Root\nRETURN:\n- MUST: return a result\n  USES: Policy\n" + explainPolicy,
	},
	{
		ID:      "yass.lint.modeline",
		Detects: "A file whose first line is not the schema modeline.",
		Why:     "Without the modeline an editor does not validate the file against the yass schema.",
		Remedy:  "Put # yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json on the first line.",
		Bad:     "---\ndescription: example\nversion: v1\n---\n" + explainRoot,
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.no_origin",
		Detects: "A root file that fails to parse, supplies no single spec, or supplies a spec whose name is missing, empty, or not a string.",
		Why:     "Reachability has no single addressable origin. Other advisories still run, but unreachable advisories are suppressed.",
		Remedy:  "Make root.yass.yaml parse and give it exactly one spec with a nonempty string name.",
		Bad:     explainPreamble + "design: Policy\ntype: constraint\ncontent: preserve order\n",
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.obligation_shape",
		Detects: "Obligation prose over 400 characters; prose or guards matching code, step sequences, multiple clauses, or rationale asides; or prose mapping errors outside ERROR. Clause checks exempt e.g., i.e., etc., vs., and cf. Backtick spans are skipped for pattern checks.",
		Why:     "One obligation states one requirement. Procedures and shapes belong in designs, and failure outcomes belong in ERROR.",
		Remedy:  "Split independent requirements, bind procedures or shapes with USES, and place failure outcomes in ERROR. Review the matched pattern rather than treating this advisory as a semantic judgment.",
		Bad:     explainPreamble + "spec: Root\nRETURN:\n- MUST: return a result; preserve its order\n",
		Good:    explainPreamble + "spec: Root\nRETURN:\n- MUST: return a result\n- MUST: preserve its order\n",
	},
	{
		ID:      "yass.lint.open_question",
		Detects: "Whole-word TBD, TODO, FIXME, or XXX in capitals, open question in any casing, or ?? on any file line; or ? ending a value or followed by whitespace in obligation prose, a guard, or a preamble description. Backtick spans are skipped.",
		Why:     "A spec is a contract; an undecided point makes its implementer guess.",
		Remedy:  "Resolve the question and state the chosen behavior. Backtick spans may quote markers the spec discusses rather than uses.",
		Bad:     explainPreamble + "spec: Root\nRETURN:\n- MUST: TODO\n",
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.prose_keyword",
		Detects: "Obligation prose starting with when, if, unless, or whenever in any casing, or holding one of those words in lower case after a comma or semicolon and whitespace; MUST/SHOULD/MAY/WHEN as capitalized whole words in prose or guards; or @ followed by a letter or underscore there. Backtick spans are skipped.",
		Why:     "A guard or obligation hidden in prose loses structure; a reference hidden there creates no traversable edge.",
		Remedy:  "Put conditions in WHEN, separate requirements into obligation entries, and use CONFORMS, USES, or SEE for references. Backtick literal syntax discussed by the spec.",
		Bad:     explainPreamble + "spec: Root\nRETURN:\n- MUST: if input is empty return no items\n",
		Good:    explainPreamble + "spec: Root\nRETURN:\n- WHEN: input is empty\n  MUST: return no items\n",
	},
	{
		ID:      "yass.lint.restated_fact",
		Detects: "Prose or a guard names a non-numeric catalogue identifier as a case-sensitive whole token and states, outside named identifiers, a value or vocabulary wording of a field its record holds, matching or conflicting. For vocabulary-backed fields, as when/as for/as if/as though also matches. Backticks are stripped; single-word values need field or vocabulary context within three words; multiword wordings match phrases or specified content-word patterns.",
		Why:     "A fact repeated outside its catalogue creates a second source that can drift.",
		Remedy:  "Name the identifier and bind the catalogue with USES instead of repeating its value or borrowing another case's outcome.",
		Bad:     explainPreamble + "spec: Root\nERROR:\n- MUST: report invalid_input with status failed\n  USES: Codes\n- USES: Statuses\n" + explainCatalogue,
		Good:    explainPreamble + "spec: Root\nERROR:\n- MUST: report invalid_input\n  USES: Codes\n- USES: Statuses\n" + explainCatalogue,
	},
	{
		ID:      "yass.lint.restated_value",
		Detects: "Prose or a guard names no non-numeric identifier of a catalogue as a case-sensitive whole token, binds neither that catalogue nor its field vocabulary with USES, and states a value some record holds in a vocabulary-backed field. Unused vocabulary values are exempt. Backticks are stripped; single-word values need field or vocabulary context within three words; multiword wordings match phrases or specified content-word patterns. Experimental.",
		Why:     "The obligation states a record's fact without naming the record; fixed patterns can produce false advisories.",
		Remedy:  "Name the applicable record, bind its catalogue with USES, and remove the stated value so the catalogue remains its sole source. Binding the governing catalogue or vocabulary also suppresses this advisory, but repeating a value beside its record can trigger restated_fact. Review the match; these patterns may change between releases.",
		Bad:     explainPreamble + "spec: Root\nERROR:\n- MUST: return status failed\n" + explainCatalogue,
		Good:    explainPreamble + "spec: Root\nERROR:\n- MUST: report invalid_input\n  USES: Codes\n- USES: Statuses\n" + explainCatalogue,
	},
	{
		ID:      "yass.lint.slot_empty",
		Detects: "An addressable spec declaring a slot with an empty obligation list.",
		Why:     "The declared slot carries no obligation.",
		Remedy:  "State the slot's obligations, or omit the slot when it carries none.",
		Bad:     explainPreamble + "spec: Root\nRETURN: []\n",
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.spec_empty",
		Detects: "An addressable spec declaring no slot.",
		Why:     "A spec with no slot states no obligation and binds nothing.",
		Remedy:  "Give the spec its contract obligations, or remove a document that supplies no contract.",
		Bad:     explainPreamble + "spec: Root\n",
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.unparsed",
		Detects: "A project spec file that failed to parse because of malformed YAML, invalid UTF-8, or a leading byte-order mark.",
		Why:     "A partial graph can yield false reachability advisories. Lint reports the parse failure and continues its other checks.",
		Remedy:  "Correct malformed YAML or encode the file as UTF-8 without a byte-order mark; use validate to check well-formedness.",
		Bad:     explainPreamble + "spec: Root\nRETURN: [\n",
		Good:    explainPreamble + explainRoot,
	},
	{
		ID:      "yass.lint.unreachable",
		Detects: "An addressable spec or design under the project root reached by no chain of CONFORMS, USES, and SEE edges from the root spec. The root spec itself is exempt.",
		Why:     "A project contract is connected through references from its root; an unreachable document is outside that chain.",
		Remedy:  "Connect the document through the appropriate reference relation, or remove a document outside the project's contract. Reachability does not cross nested project roots.",
		Bad:     explainPreamble + explainRoot + explainPolicy,
		Good:    explainPreamble + "spec: Root\nRETURN:\n- MUST: return a result\n  USES: Policy\n" + explainPolicy,
	},
}
