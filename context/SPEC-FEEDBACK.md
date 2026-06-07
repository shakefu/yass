# Feedback for the Guidance doc

This is AI-generated feedback, don't take any of it at face value.

Seven agents were asked to implement the yass CLI in different programming languages. C, Go, Rust, Python, Zig, Bun, and Haskell.

They were all given the same initial prompt:

```
claude -- "/goal We are creating a Haskell based CLI from the yass spec/

Read the yass.yass.yaml - and read context/GUIDANCE.md - and read spec/*

Then use an ultracode dynamic workflow to drive implementation of the CLI using TDD.

Your acceptance criteria is >80% test coverage and 100% adherence to the specs.

You have full autonomy to make all decisions to complete this goal."
```

After completing the implementation, they were given the following prompt:

```
Don't make any edits - based on this session, what changes would you make to the yass spec syntax itself?
```

## C Agent

Based on implementing from these specs, here are the syntax/language changes I'd propose:

Priority chains need a first-class construct

CheckYAML and CheckPreamble both express "emit at most one error in this order" as prose in an INVARIANT obligation. This is the most
mechanically precise thing in the entire spec — it's a ranked list with early-exit semantics — but it's encoded as a sentence the
implementer has to parse mentally. Something like:

INVARIANT:
- PRIORITY:
  - yass.yaml.not_utf8
  - yass.yaml.has_bom
  - yass.yaml.empty_file
  - yass.yaml.malformed
  - yass.yaml.duplicate_key
  - yass.yaml.anchor_or_alias

A PRIORITY construct would make the ordering machine-readable, testable, and unambiguous about "stop at first match."

Pipeline ordering between specs needs a structural expression

cli.validate defines a check pipeline: CheckYAML → CheckPreamble → CheckSpec → CheckUniqueness → CheckRefs, where earlier failures gate
later checks. This is expressed in prose obligations ("run checks per file in the order...") and USES refs. But USES doesn't convey
ordering — it means "depends on," not "runs before." A pipeline or sequencing relation would distinguish "A depends on B's definition"
from "A must execute after B succeeds."

WHEN guards can't express "for each" iteration

Several obligations say things like "for each spec document" or "for each obligation in each slot." The WHEN guard expresses a condition,
not iteration. Implementers have to infer the loop structure from prose. A quantifier — FOREACH or similar — would make the iteration
scope explicit and would directly generate test scaffolding (one test per element type).

There's no way to express "same rules as X" without inlining

Path formatting rules appear in ErrorLine, DiscoverSpecFiles, and list. Each restates the same rules. CONFORMS inlines them, which is
correct for contracts, but it means the query output repeats the same 15 obligations three times. A lighter reference — something like
SHARES or IMPLEMENTS that says "these are the same rules" without physical inlining — would reduce noise in resolved output while
preserving the contract relationship.

Error code tables want a structured form, not 80 MUST obligations

cli.errors is 80+ lines of MUST: "yass.foo.bar means X; message Y; exit Z". Every single one follows the same schema: code, meaning,
message template, exit code. This is structured data encoded as prose. A dedicated error-table construct — or at minimum, a convention for
expressing tabular data within a slot — would make this machine-extractable rather than requiring regex parsing of obligation prose.

Ref targets should be expressible as lists

Currently, to reference two targets from one obligation, you split it into two obligations. InlineConforms on cli.Dispatch produces 15
inlined obligations from cli.ExitCode::RETURN, plus 18 from two separate cli.ErrorLine::RETURN references. A list-valued reference
(CONFORMS: [cli.ExitCode::RETURN, cli.ErrorLine::RETURN]) would keep the carrier obligation's intent clearer. This is already noted in
FUTURE.md but after implementing, I'd escalate it — the single-ref constraint creates the most structural noise in real specs.

The "reference-only obligation" rule is subtle enough to deserve a keyword

An obligation with a CONFORMS ref and no normativity keyword is "reference-only" — it replaces rather than appends during inlining. An
obligation with CONFORMS and a normativity keyword keeps itself and appends. This distinction is critical for correct CONFORMS inlining
and is currently implicit in the absence of a keyword. An explicit marker (even just documenting CONFORMS: alone on a line as the
canonical reference-only form) would reduce implementation bugs.

The spec is silent on what happens inside prose

The biggest gap: obligation prose is opaque. MUST: "format each error line as \:: [] `"contains a format string with four named
parameters, but the spec syntax has no way to express that the, , , and ` tokens are formal parameters that other specs reference. The
colon-space quoting footgun is a symptom of this — the prose layer has structure the syntax can't see. I don't have a concrete proposal
here, but it's the boundary where most implementation ambiguity lives.


## Go Agent

Based on what actually caused bugs during implementation:

1. Priority-ordered error specs need a first-class construct

The most error-prone obligations were the "emit at most one error in priority order" rules in CheckPreamble and CheckYAML. They're
expressed as a single INVARIANT obligation with a comma-separated list buried in prose:

INVARIANT:
- MUST: "emit at most one error per file in the order (1) yass.preamble.has_spec_key, (2) yass.yaml.empty_stream, ..."

This is a ranked decision table masquerading as a sentence. The spec syntax has no way to express ordering relationships between
obligations, so authors cram them into prose. A PRECEDENCE or PRIORITY construct — or even just allowing ordered sub-lists within an
obligation — would make these unambiguous and machine-parseable.

2. Cross-cutting constraints need explicit scoping

The ErrorLine spec says "MUST-NOT: emit any input path containing the literal : character." This applies to every subcommand's stderr
output, but it's defined in one spec (cli.ErrorLine) and referenced via CONFORMS in others. We violated it in three places because each
subcommand reimplemented the colon check independently.

The problem is that CONFORMS inlines obligations but doesn't express scope of enforcement. There's no way to say "this constraint applies
to all stderr writes across all specs in this file." The language could benefit from a construct that expresses a cross-cutting invariant
— something like a spec-level CONFORMS that applies transitively to all specs that USES a given spec.

3. "At most one" vs "one per" cardinality is implicit

CheckYAML says "emit at most one error per file." CheckSpec says "emit one error per failing rule per obligation." These are fundamentally
different error-reporting strategies, but they're both expressed as prose within MUST obligations. An explicit cardinality annotation —
ONCE-PER: file vs EACH: obligation — would prevent the ambiguity and let test generators know whether to expect 1 error or N.

4. Negative space obligations are hard to audit

The spec has many "MUST-NOT emit X" obligations (no ANSI codes, no JSON output, no stdin reading, no file modification). These are
important but essentially unprovable through testing — you can't enumerate everything a program doesn't do. The spec would benefit from
distinguishing between testable constraints ("MUST exit 2") and environmental constraints ("MUST-NOT spawn subprocesses") at the syntax
level, since they require fundamentally different verification strategies.

5. The WHEN guard semantics are underspecified for composition

When InlineConforms combines an outer WHEN and inner WHEN, the spec prescribes literal string concatenation with and. But the language
definition for Guard says "MUST: express a sufficient condition" and "MUST-NOT: express a necessary condition." Whether the concatenated
result preserves these semantics is undefined. The spec syntax doesn't have a way to express guard composition rules — it just says what
the string looks like. If CONFORMS inlining is a first-class operation, the guard algebra should be part of the language definition, not
just the CLI spec.

6. RefTarget resolution rules are spec content, not language rules

The entire path resolution system (./ relative to file, bare relative to project root, append .yass.yaml, case-sensitive comparison) is
defined in yass.yass.yaml as obligations on the RefTarget spec. But these are really language semantics — they determine what a reference
means. Embedding resolution rules as obligations on a spec named "RefTarget" works, but it means a tool author has to read and implement a
spec's obligations to understand what a reference target is. These might belong in the preamble or as language-level meta-rules rather
than as obligations on a named spec.

7. The CONFORMS relation creates implicit ordering dependencies

A spec that says CONFORMS: cli.ExitCode::RETURN means "inline those obligations here." But the language definition says nothing about what
happens when inlined obligations conflict with local ones, or whether inlined obligations are additive or overriding. In practice we
treat them as additive (append after the carrier), but this is defined in the CLI spec for InlineConforms, not in the language itself. If
CONFORMS is a language-level construct, its composition semantics should be too.

## Rust Agent

Good question. Based on implementing the CLI from the yass specs, these are the friction points in the syntax itself:

1. No way to express error cardinality.
The validate spec has three different error-reporting contracts: "at most one error per file" (CheckYAML), "one error per failing rule per
obligation" (CheckSpec), and "stop at first match" (CheckPreamble). These are buried in INVARIANT prose. The language needs a way to
express cardinality constraints on ERROR slots — something like ERROR(at-most-one): or an obligation-level annotation. Right now every
implementer has to read natural language to discover the error-suppression strategy.

2. No way to express ordering between obligations.
cli.validate says "run checks per file in the order CheckYAML, CheckPreamble, CheckSpec, CheckUniqueness, CheckRefs." The spec also says
"MUST-NOT carry meaning in the order of its obligations" (Slot spec). These contradict. Validation pipelines have inherent sequencing —
CheckPreamble depends on CheckYAML succeeding. The language needs either a SEQUENCE slot or a way to annotate ordering dependencies
between obligations. The current rule that obligation order is meaningless is wrong for procedural specs.

3. CONFORMS should require a slot suffix at the language level.
The query spec adds yass.query.conforms_no_slot to reject CONFORMS refs without ::SLOT. If a bare-spec CONFORMS is never valid in v1, the
RefTarget grammar for CONFORMS should enforce the slot suffix structurally rather than making every tool validate it at runtime. The
reference relation type should constrain the ref-target grammar.

4. No way to express priority ordering.
CheckYAML's priority (not_utf8 > has_bom > empty_file > malformed > duplicate_key > anchor_or_alias) and CheckPreamble's
stop-at-first-match ordering are expressed as numbered prose lists in INVARIANT. This is a recurring pattern — "try these conditions in
order, emit the first match." The language should have a PRIORITY or PRECEDENCE construct rather than relying on authors to write "emit at
most one error in the order (1)...(2)...(3)..."

5. Override/inheritance semantics between specs are implicit.
The Dispatch spec says bare - should error with stdin_dash. The validate spec says "MUST treat - as a literal path argument." Which wins?
The answer is "subcommand specs override Dispatch-level behavior" but nothing in the language expresses this. CONFORMS gives you additive
inlining, but there's no OVERRIDES relation. When specs at different levels of a dispatch hierarchy contradict, the language provides no
resolution mechanism.

6. Message templates belong in the language, not prose.
The cli.errors spec encodes message templates like "unknown subcommand: <arg>" inside obligation prose strings. These are machine-stable
contracts — tooling that parses stderr depends on exact byte patterns. The language should have a structured way to define parameterized
message templates, distinguishing the fixed prefix from the variable <arg> slot. Right now the template syntax is a convention inside a
string, invisible to tooling.

7. No way to express mutual exclusion between obligations.
"MUST exit 0 on success" and "MUST exit 1 when validation fails" are mutually exclusive outcomes, not independent requirements. The spec
reads as if both could apply simultaneously. A WHEN guard partially addresses this, but there's no way to say "exactly one of these N
obligations will hold per invocation." This matters for test generation — you need to know these are branches, not independent checks.

8. Cross-file ref resolution rules are too far from the ref syntax.
RefTarget defines the grammar (path@SpecName::SLOT), but the resolution semantics (./ = relative to file, no prefix = project root, append
.yass.yaml) are split across RefTarget, FindProjectRoot, and each subcommand that resolves refs. A single spec should own the complete
resolution algorithm. Currently an implementer has to read four specs to understand how ./cli.shared@Foo::RETURN resolves to a file path.

9. The SIDE-EFFECT slot conflates two things.
SIDE-EFFECT is used for both "writes errors to stderr" (an output contract) and "CONFORMS refs are inlined when resolved" (a
transformation rule). These have different testing strategies. Output contracts are observable; transformation rules define an algorithm.
Splitting SIDE-EFFECT into OUTPUT (observable effects) and TRANSFORM (structural mutations) would make specs more precise and more
testable.

10. No way to express "this spec is procedural, not declarative."
Specs like cli.validate describe a pipeline with sequenced stages, early exits, and accumulated state (error count M, file count N). The
slot structure (INPUT/RETURN/ERROR) assumes a functional contract. Procedural specs fight the syntax — ordering goes in INVARIANT,
accumulation goes in SIDE-EFFECT, and the control flow is invisible. The language might need a PROCEDURE or ALGORITHM slot type that
explicitly allows ordered steps.

## Python Agent

Based on implementing a full CLI from these specs, here are the syntax-level changes I'd propose:

1. Add an EXAMPLE slot

The single biggest implementation friction was ambiguous output formats. OutputProfile describes quoting rules in prose ("when a scalar
contains : ...") but a worked example would have been unambiguous. The current five slots are function-shaped — EXAMPLE would be the
sixth, carrying concrete input→output pairs as obligations:

```
EXAMPLE:
- WHEN: "a spec has a CONFORMS ref to another spec's RETURN slot"
  MUST: |
    ---
    spec: Foo
    RETURN:
    # CONFORMS: ./bar@Bar::RETURN
    - MUST: return a value
```

This isn't prose verification — it's a deterministic fixture that tooling can extract for test generation. The test taxonomy already needs
worked examples to be useful; this gives them a home.

Counter-argument: This pushes toward verification, which yass explicitly defers. But examples don't verify content — they're structured
fixtures, same as obligations. The spec already carries prose that describes examples inline ("resolve ./cli.shared@Foo to..."). Giving
them structure just makes them machine-extractable.

2. Allow USES on slots, not just obligations

I repeatedly needed to express "this function's input is the output of that function" at the slot level. Currently you write:

INPUT:
- MUST: accept a parsed YAML multi-document stream
  USES: cli.validate.CheckYAML

The USES is on the obligation, but the dependency is on the slot. The obligation prose restates what CheckYAML already defines. If USES
could appear at the slot level:

INPUT:
  USES: cli.validate.CheckYAML::RETURN
- MUST: accept the first document as a candidate Preamble

This separates "where the data comes from" (structural) from "what constraints apply" (behavioral). It also makes data flow visible to
tooling without parsing obligation prose.

3. Formalize error code references

The cli.errors spec is a flat list of MUST obligations carrying error codes in prose. Every other spec references these codes by string
literal embedded in prose ("error with code yass.yaml.not_utf8"). There's no structured way to:
- Verify a referenced error code exists
- Find all specs that emit a given code
- Detect unused codes

A lightweight addition: allow a CODE key (or similar) on obligations that names the error code as a structured value rather than prose:

ERROR:
- MUST: error when the file is not valid UTF-8
  CODE: yass.yaml.not_utf8

Tooling can then validate code references across specs the same way it validates CONFORMS/USES refs.

4. Add a DEPENDS or REQUIRES relation

CONFORMS means "must match." USES means "behavior draws on." But implementation repeatedly needed "this function requires this other
function to have run first" — a sequencing/precondition dependency that neither relation captures. CheckPreamble depends on CheckYAML
having succeeded, but it doesn't CONFORM to CheckYAML and it doesn't USE CheckYAML's behavior — it requires CheckYAML as a precondition.

Currently this is expressed in prose: "validate YAML well-formedness before running structural checks." A REQUIRES relation would make
ordering dependencies explicit and machine-checkable.

Counter-argument: This is dangerously close to specifying implementation control flow. But the specs already do this — cli.validate
explicitly specs the check order. A relation just makes it referable.

5. Support list-valued WHEN guards

The spec for cli.validate.CheckPreamble has a priority-ordered error list that is really a cascade of WHEN conditions. Currently each
condition is a separate obligation. What I actually needed during implementation was:

- WHEN:
  - "the first document contains a `spec` key"
  - "the parsed YAML stream contains zero documents"
  - "no Preamble is present as the first document"
  MUST: emit the first matching error and stop

The current syntax forces the priority ordering into an INVARIANT obligation as prose ("emit at most one error per file in the order
(1)... (2)... (3)..."). This is the most complex obligation in the entire spec set, and it's invisible to tooling — the ordering is buried
in a parenthetical in a single INVARIANT string.

6. No changes needed

Two things I expected to want but didn't:

- Multi-target refs (already in FUTURE.md) — splitting obligations worked fine in practice. The verbosity was tolerable.
- Typed values — the spec avoids typed schemas deliberately, and that was correct. The implementation derived types from context without
friction. Adding types would bloat the syntax for marginal gain.

---
The theme across all of these: the spec syntax handles behavioral obligations well but has no structured channel for data flow,
sequencing, and concrete fixtures. Those three things were expressed in prose and required the implementer to reverse-engineer them. The
syntax changes above would make them machine-readable without adding verification — staying within yass's "structured envelope around
prose" philosophy.

## Zig Agent

Based on implementing the full CLI from the specs:

---
1. The "duplicate normativity" obligation is ambiguous because it uses the same word ("duplicate") that YAML uses for duplicate keys.

The spec says: "rejected when it carries the same Normativity keyword more than once". The phrase "the same keyword" reads as "the
identical key repeated" — which YAML already forbids. What the spec actually means is "more than one keyword from the Normativity set."

I'd reword to: "rejected when it carries more than one Normativity keyword". Drop "same" and "duplicate" entirely. The word "duplicate" is
a trap here because it means something specific in YAML, and the obligation is about cardinality (exactly one), not uniqueness (no
repeats).

The error code yass.obligation.duplicate_normativity reinforces the confusion. Something like yass.obligation.multiple_normativity would
be unambiguous. But codes are frozen once published, so this is a v2 consideration.

2. The cli.errors spec duplicates prose that belongs to the emitting spec.

Every error code's message template is defined in cli.errors, and then the emitting obligation (in cli.validate, cli.query, etc.) restates
the same condition in different words. During implementation I had to cross-reference both specs constantly to make sure the error code,
message text, and triggering condition were consistent. They occasionally disagreed — the emitting spec says one thing about when to fire,
the error table says a slightly different thing about the message.

I'd make cli.errors the sole authority for the message string and triggering condition, and have the emitting obligations just cite the
error code with a USES ref rather than restating the condition. One source of truth, not two.

3. The RefTarget path resolution rules are split across three specs.

RefTarget defines the grammar, Reference defines the resolution semantics, and cli.validate.CheckRefs defines what to reject. During
implementation I had to read all three simultaneously to get path resolution right. The ./ vs root-relative distinction, the .yass.yaml
suffix appending, and the slot validation are all spread across these specs.

I wouldn't merge them — they describe different things — but I'd add explicit USES refs from CheckRefs to the specific RefTarget
obligations it enforces, rather than just USES: yass@RefTarget at the spec level. The granularity of the ref is too coarse to be useful.
This is a ::SLOT ref waiting to happen.

4. The spec doesn't distinguish "must detect X" from "must detect X at this layer."

CheckYAML emits at most one error per file. CheckSpec emits one error per failing rule per obligation. But when CheckSpec says "error with
code yass.obligation.duplicate_normativity when an obligation carries more than one Normativity keyword" — it doesn't say whether this
means the same key (unreachable, because CheckYAML already caught it) or different keys. The layer ordering is defined in cli.validate,
but the individual check specs don't acknowledge what the prior layer already handled.

A WHEN: CheckYAML has passed for this file guard on CheckSpec obligations would make the layer contract explicit. Right now it's implicit
in the check ordering, which is defined elsewhere.

5. The CONFORMS inlining semantics could use a worked example as an obligation.

The InlineConforms spec has 13 obligations covering carrier guards, provenance comments, guard combination, and stripping. I got the guard
combination wrong on first pass because the prose "concatenating the outer prose, and, and the inner prose" is precise but hard to
visualize without a before/after. A spec can't carry examples (no free-prose channel — by design), but the obligation could be more
explicit:

"MUST combine by producing the string <outer> and <inner> where <outer> is the carrier WHEN value and <inner> is the inlined obligation's
WHEN value, with no parentheses, no case changes, and exactly one space before and after the word and"

That's more verbose but eliminates parsing the current sentence.

6. The MUST-NOT obligations that define what v1 does not do are valuable for implementors but invisible to test generation.

The specs are full of MUST-NOT: emit structured JSON, MUST-NOT: implement subcommand-specific --help in v1, MUST-NOT: read from stdin.
These are the most useful obligations for an implementor deciding what not to build, but the test taxonomy says "generate an assertion
that the prohibited behavior never occurs" — which for "don't implement JSON output" is meaningless. There's no code path to test.

I wouldn't change the syntax, but I'd distinguish between "MUST-NOT that prohibits a failure mode" (testable — ensure the bad thing
doesn't happen under specific inputs) and "MUST-NOT that defers a feature" (not testable — there's no code to verify the absence of).
Maybe a convention like putting feature deferrals in INVARIANT and behavioral prohibitions in the relevant slot.

## Bun Agent

Good question. I'll separate these into changes I'm confident about from this session versus observations that might warrant change but
need more evidence.

Changes I'd make

1. Add a ONE-OF or cardinality keyword for mutual exclusion

T63 happened because "exactly one Normativity keyword" was split between an INPUT obligation ("carry exactly one") and an ERROR obligation
("rejected when more than one"). The implementor built the valid-case and missed the violation-case because they're in different slots.

The language has no way to say "exactly one key from this set" as structure. You're forced into prose, and prose gets split across slots.
A cardinality keyword on the obligation — ONE-OF: Normativity — would make the constraint atomic and tooling-checkable. It doesn't
interpret prose content (stays language-agnostic); it constrains the shape of the obligation mapping, which the language already does with
WHEN, CONFORMS, etc.

2. Make INVARIANT ordering explicit when the spec says it matters

CheckPreamble's INVARIANT says "emit at most one error per file in the order (1) yass.preamble.has_spec_key, (2) yass.yaml.empty_stream, …
(9) yass.preamble.bad_related, stopping at the first match." That's a priority chain encoded as a numbered list inside a single prose
string. CheckYAML has the same pattern.

The problem: the INVARIANT slot is declared unordered ("MUST-NOT carry meaning in the order of its obligations"). So a priority chain —
where order is the entire point — is stuffed into a single obligation's prose value, invisible to tooling.

Options: (a) a PRIORITY slot that is explicitly ordered, (b) allow an obligation-level ORDER: N key, or (c) accept that this is always
prose. I lean toward (a) — it's a small addition, it's the truth (some constraints are ordered), and pretending they aren't causes bugs.

3. Structured error definitions

cli.errors is 85 MUST obligations that each encode three things in prose: an error code, a message template with <param> placeholders, and
an exit code. Implementing this required hand-parsing 85 prose strings to build a code→exit and code→message mapping.

This isn't an obligation — it's a data table. The spec language forced a table into obligation syntax because there's no other container.
A structured ERRORS construct (or even just acknowledging that some specs are enumerations, not behavioral contracts) would make this
implementable without prose parsing. The error code table was the single largest source of tedium in the implementation.

4. Explicit cross-spec sequencing

The validate spec says "run checks per file in the order CheckYAML, CheckPreamble, CheckSpec, CheckUniqueness, CheckRefs" and "when YAML
parsing fails, skip CheckPreamble, CheckSpec, CheckUniqueness, and CheckRefs." These are control-flow dependencies between specs — A must
precede B, and A's failure skips B — expressed as prose.

USES says "behavior depends on" but doesn't distinguish "depends on the output of" from "must execute after" from "is skipped when the
other fails." A AFTER or REQUIRES relation — distinct from USES — would make sequencing tooling-visible. Right now an implementor can read
five specs, understand each one perfectly, and still wire them together wrong.

Observations that need more evidence

The WHEN complement problem

Every WHEN guard implicitly creates a negative branch the spec doesn't express. T89 was partly this — the case-mismatch obligation says
"subcommand or flag" but the implementor only handled subcommands because the flag code path was a different WHEN branch (the flag
handler) where the obligation wasn't restated.

FUTURE.md already defers ONLY WHEN / UNLESS. I'd note that the implementation experience confirms the gap is real — but I'm not sure the
fix is language syntax versus better guidance. The complement is derivable; the question is whether it's derivable reliably enough without
syntax.

"At most one per (X, Y)" dedup constraints

CheckRefs requires "at most one yass.ref.file_not_found per (referencing-file, referenced-file) pair." This is an emission cardinality
constraint with a compound key, buried in INVARIANT prose. It's unusual enough that I wouldn't add syntax for it yet, but if more specs
exhibit this pattern, a DEDUP-BY key on error obligations would prevent the class of bugs where a validator correctly finds N violations
but should only report M < N.

The "or" scope ambiguity

"A subcommand or flag" in one obligation means the rule applies to both independently. "A file or directory path" means the input can be
either. Same word, different logical scopes. The language can't distinguish these, and the test taxonomy can't generate the right cases
without understanding which "or" it is. I don't have a syntax proposal — this might just be a guidance item about prose discipline — but
it caused a real bug.

## Haskell Agent

Good question. This is based on watching what the spec syntax caused during implementation — where the language structure helped, and
where it forced important behavioral semantics into prose that implementations then got wrong.

---
1. Error tables need structure, not prose

The cli.errors spec is a lookup table. Every entry follows the same pattern:

- MUST: "yass.argv.unknown_subcommand means the first positional did not match a known subcommand; message `unknown subcommand: <arg>`;
exit 2"

That single string encodes four distinct fields: the code name, the semantic meaning, the exact message template, and the exit code.
Implementations must parse this prose to extract the machine contract. Every implementation will get at least some messages wrong on the
first pass because the template is buried inside a sentence.

yass has no construct for "this is a structured table row with named fields." The language forces you to encode structured data as prose
obligations, then hope the reader parses the prose correctly.

What I'd add: A TABLE slot or a structured obligation form that lets you express key-value records as data rather than prose. Something
like:

spec: cli.errors
TABLE:
  - CODE: yass.argv.unknown_subcommand
    MESSAGE: "unknown subcommand: <arg>"
    EXIT: 2
    MEANING: the first positional did not match a known subcommand

Or, less invasively, allow obligation values to be mappings rather than requiring everything to be a scalar. The current rule (obligation
value must be a quoted scalar) actively prevents structured content.

2. Priority ordering has no syntactic expression

Several specs contain obligations like:

INVARIANT:
- MUST: "emit at most one error per file in the order (1) yass.preamble.has_spec_key, (2) yass.yaml.empty_stream, (3)
yass.preamble.missing, (4) yass.preamble.duplicate, (5) yass.preamble.misplaced, (6) yass.preamble.missing_description, (7)
yass.preamble.missing_version, (8) yass.preamble.unknown_version, (9) yass.preamble.bad_related, stopping at first match"

This is the most load-bearing obligation in the entire validate pipeline, and it's a single prose string. The ordering determines which
error code fires for every ambiguous input. Getting it wrong (as the implementation did — confusing duplicate vs. misplaced) produces
subtly incorrect behavior that passes casual testing.

What I'd add: An ORDERED modifier or a PRIORITY construct that expresses "these items are evaluated in sequence, first match wins" as
structure rather than prose. The ordering IS the specification — it shouldn't be less structured than a grocery list.

3. CONFORMS inlining semantics are defined in the wrong place

The yass language definition (yass.yass.yaml) says CONFORMS means "the obligation must match the referenced spec or slot" and that
CONFORMS refs "MUST be inlined into the referencing spec when resolved." But the actual inlining rules — reference-only replacement,
normative carrier preservation, guard combination with and, provenance comments, one-level-deep resolution — are defined in the CLI spec
(cli.query.InlineConforms), not in the language spec.

This means the CONFORMS inlining behavior is an application-level concern, not a language-level one. Every tool that implements CONFORMS
inlining must independently discover and implement these rules from the CLI spec. If someone builds a different tool that resolves
CONFORMS refs, they have no language-level guidance on how inlining works.

What I'd change: Move the inlining semantics (at least the core rules: replacement vs. append, guard combination, provenance, depth limit)
into the language definition under the Reference spec. The query OutputProfile and provenance comment format can stay in the CLI spec,
but the structural transformation rules belong in the language.

4. Unreachable error codes are a design smell

The spec defines error codes that are unreachable by design:

- yass.preamble.missing — unreachable because any first document without a spec key is treated as a preamble (possibly invalid), and
has_spec_key covers the only case where no preamble exists
- yass.preamble.misplaced — unreachable because duplicate has higher priority and covers the same inputs
- yass.obligation.duplicate_reference — unreachable because the YAML parser rejects duplicate mapping keys before semantic checks run
- yass.argv.missing_positional — unreachable because query.name_missing covers the only case

These codes exist in the table, implementations must define them, but no input can ever trigger them given the specified check ordering.
The spec has no way to express "this code exists for the semantic space but is shadowed by check X."

What I'd add: A SHADOWED-BY or UNREACHABLE-WHEN annotation on error codes. Or, more practically, remove the unreachable codes. A spec that
defines behavior that can't occur is making a promise about nothing.

5. The scalar-only obligation value constraint is too restrictive

INVARIANT:
- MUST: "be rejected when it carries neither a Normativity keyword nor a Reference"

Every obligation value is a single string. The spec syntax doesn't let you attach metadata to an obligation — its error code, its priority
relative to siblings, its test category, whether it's enforced in v1 or deferred. All of that must go into the prose or into a separate
spec.

The cli.errors spec exists entirely because obligations can't carry structured metadata. If an ERROR obligation could carry its error code
as a named field rather than requiring a separate lookup table, the error table spec wouldn't need to exist at all.

What I'd consider: Allowing obligation values to be either a scalar (for simple prose) or a mapping (for structured obligations). A
mapping-valued obligation could carry its prose plus machine-stable metadata:

ERROR:
- WHEN: the file is not valid UTF-8
  MUST: reject the file
  CODE: yass.yaml.not_utf8
  MESSAGE: "file is not valid UTF-8"
  EXIT: 1

This breaks the current "obligation value at a scalar position is a mapping → bad_value_shape" rule, but that rule exists to protect the
current design, not because mappings are inherently wrong.

6. The RefTarget grammar should be a first-class construct

The ref target syntax (path@SpecName::SLOT) is described across four prose obligations and one regex in a MUST:

- MUST: "first match the ref target against the grammar `^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$`"

Grammars and regexes are structural specifications — they define a formal language. Encoding them as prose inside a MUST obligation means
the implementation must extract the regex from the string, compile it, and hope the escaping survived. The regex itself is the
specification, not the sentence containing it.

What I'd add: A GRAMMAR or PATTERN construct at the language level for expressing formal syntactic rules. The RefTarget spec should be
able to say "this is a grammar" in structure, not in prose.

7. The distinction between sufficient and necessary conditions is unenforceable

The Guard spec says:

INVARIANT:
- MUST-NOT: express a necessary condition

This means WHEN: X means "if X, then the obligation applies" but NOT "the obligation applies only when X." This is a semantic distinction
about the interpretation of prose that no tooling can enforce and no validation can check. It's guidance for spec authors, not a
machine-verifiable rule.

What I'd change: Move this to the authoring guidance document. Language-level INVARIANT obligations should be things that tooling can
verify. "Don't express necessary conditions" is a writing style rule, not a structural constraint.

Summary

The common thread: yass forces structured data into prose, then makes implementations responsible for extracting the structure. Error
tables, priority orderings, grammars, and inlining rules are all examples of structured behavioral specifications that the language
requires you to write as strings. The syntax is disciplined about what slots exist and which keywords are valid, but has no tools for
expressing the machine-relevant data those slots contain.

The highest-value change would be allowing structured obligation values — mappings alongside scalars. That single change would eliminate
the need for the error table spec, make priority orderings expressible as structure, and let obligations carry their own machine-stable
metadata.
