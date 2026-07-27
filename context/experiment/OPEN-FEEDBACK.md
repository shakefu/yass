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
Any other final feedback on yass development?
```

## C Agent

What worked

The error code table was the highest-value spec artifact. cli.errors defined every observable failure mode — code, message template, exit
code — in one place. Each row was a test case. If I could only keep one spec file, it'd be that one. The structured obligation format
(normativity + guard + ref) genuinely constrained the specs in useful ways; WHEN: a glob pattern matches zero files / MUST: exit 2 told me
exactly what to implement and exactly what to assert.

CONFORMS inlining justified its complexity. cli.ExitCode::RETURN was defined once and consumed by four specs. The provenance comments made
it traceable when debugging. The one-level-deep limit was the right call — it kept resolution predictable without needing cycle
detection.

The five-slot model mapped cleanly to CLI subcommands. INPUT = what it accepts, RETURN = what it produces, ERROR = failure modes,
SIDE-EFFECT = stdout/stderr/signals, INVARIANT = cross-cutting constraints. For function-shaped things, this decomposition forced
completeness — I couldn't write a spec that forgot to specify error behavior, because the ERROR slot was staring at me.

What hurt

The inter-spec dependency graph was invisible and had to be inferred. I had 7 spec files with USES/CONFORMS refs forming a DAG. The
implementation order mattered — check_refs.c needed yaml_parse.c, path.c, and discover.c. I had to mentally build the DAG by reading every
ref in every file. A yass deps command (or even a topological sort printed by yass list --refs) would have saved real time. This is the
difference between "specs exist" and "specs are navigable."

Priority chains were the hardest thing to implement correctly despite being the most mechanically precise. CheckYAML's "emit at most one
error in this order: not_utf8, has_bom, empty_file, malformed, duplicate_key, anchor_or_alias" is an ordered decision list — pure control
flow — encoded as an INVARIANT prose sentence. I implemented the ERROR cases first (naturally), then discovered the INVARIANT that made
most of them unreachable per invocation. The information architecture fights the implementation flow: the thing that shapes the function
skeleton lives at the bottom of the spec, not the top.

"Scalar position" was the null obligation bug. The spec says obligation values "at a scalar position" must not be null. But YAML parsers
don't distinguish between MUST: (null) and MUST: "value" (string) at the event level — both arrive as scalar events. The spec assumes a
clean type system the parser doesn't provide. This class of bug — where the spec's model of the data doesn't match the parser's model —
will recur for every implementation language. The spec can't fix this (it's a YAML tooling concern), but a "known footguns" annotation on
obligations that touch parser boundaries would help.

Negative-space obligations are invisible until they bite. MUST-NOT resolve symbolic links. MUST-NOT emit ANSI codes. MUST-NOT spawn
subprocesses. MUST-NOT page output. These don't produce visible behavior — you only notice their absence when a test specifically checks.
I counted 47 MUST-NOT obligations across the specs. Under time pressure, these are the first things an implementer (human or agent) skips,
and they're the hardest to test because you're asserting the absence of something. The test taxonomy handles this ("assert the effect is
absent"), but there's no tooling to flag untested MUST-NOTs.

On the agent development story

Spec → test derivation is the highest-leverage path, and it's not automated. The test taxonomy document tells an agent what to do with
each obligation — MUST under ERROR becomes "arrange the trigger, assert the failure." I derived 359 tests from the specs, but I did it
manually. A yass test-gen command that emits test stubs per obligation (with the test taxonomy applied) would be the single most impactful
tool addition. The spec structure is rich enough to support this: slot determines test shape, normativity determines assertion strength,
WHEN determines arrange step, error code determines expected output.

Spec → implementation still requires architectural judgment the spec can't provide. I could derive what to build from the specs, but how
to structure it — the module decomposition, the type hierarchy, the memory management strategy — required decisions the spec deliberately
doesn't make (language-agnostic). That's correct, but it means the agent still needs a separate design step between "read the spec" and
"write the code." For this project, the headers (*.h files) were that design step. For a future agent workflow, an explicit "design from
spec" phase that produces the module graph before implementation starts would reduce rework.

The query subcommand is the agent's primary interface, and its output gets large fast. Querying cli.Dispatch produced ~120 lines after
CONFORMS inlining — the ExitCode and ErrorLine contracts each expanded to 15+ obligations. For an agent consuming this in a prompt, that's
useful context. For a human reading it, it's noise. The tension between "show me this spec" and "show me everything this spec commits to"
will grow. Depth control (--no-inline), summary views, or a --format json for programmatic consumption would help agent workflows. The
spec says "MUST-NOT emit JSON in v1" — I'd revisit that in v2 specifically for agent consumption.

Self-validation should be test case zero. The most valuable single test was yass validate spec/ — running the tool against its own specs.
It caught a real bug (cross-file ref resolution for ../cli@cli.ErrorLine::RETURN) that unit tests missed. For any future implementation,
"the tool validates the specs it was built from" should be the first acceptance test written, before any unit tests.

If I were starting a second implementation

1. Start from cli.errors, generate one test stub per error code
2. Run yass validate spec/ as the integration gate from commit zero
3. Use a higher-level YAML library — the event-based parser was 666 lines of commodity code that the spec doesn't care about
4. Implement the pipeline DAG (CheckYAML → CheckPreamble → CheckSpec → CheckUniqueness → CheckRefs) as explicit middleware, not implicit
control flow
5. Extract priority chains into data (ordered arrays of condition/error pairs), not code (nested if-else with early returns)
6. Write the module headers first as the design artifact, then implement

## Go Agent

A few things beyond syntax and guidance, from having taken the whole thing from spec to shipping code:

The error code table is the best thing in the spec suite

cli.errors.yass.yaml was disproportionately useful compared to every other spec file. It gave us a single canonical source for every error
code, its message template, and its exit code. Implementation was: read the table, define constants, done. Every other spec file required
interpretation; this one was a lookup table. If I were advising someone writing yass specs for a new project, I'd say "write the error
table first."

The logical extension: a first-class error-table construct in the language itself, rather than encoding it as a long list of MUST
obligations on a single spec.

The spec describes components well but interactions poorly

Each spec in isolation is clear. FindProjectRoot is well-defined. DiscoverSpecFiles is well-defined. CheckRefs is well-defined. But the
interactions between them — FindProjectRoot is called once at startup, its result is passed to CheckRefs, CheckRefs uses it differently
for ./ vs bare paths, and the path it reports errors against comes from DiscoverSpecFiles not CheckRefs — required reading 4 specs
simultaneously and mentally simulating the data flow. Three of our five critical bugs were at component boundaries, not within components.

The spec syntax has no way to express dataflow between specs. USES says "depends on" but doesn't say how. A sequence diagram or dataflow
spec would have prevented more bugs than any additional obligation on an individual spec.

The 3:1 expansion ratio is the right density

~1,000 lines of spec produced ~3,100 lines of Go implementation. That's a healthy ratio — the spec is dense enough to be worth writing but
not so detailed that it's a shadow implementation. The test code was ~5,900 lines, which means the spec-to-test ratio is roughly 1:6.
That's where the real leverage is: each obligation generates multiple test cases. The test taxonomy work in GUIDANCE.md is the
highest-leverage next investment.

CONFORMS is too complex for a v1 language feature

CONFORMS does three things: (1) declares a conformance relationship, (2) drives one-level obligation inlining with guard combination, and
(3) requires provenance comments in output. During implementation, InlineConforms was the most bug-prone component — guard combination
("<outer> and <inner>"), carrier obligation handling (reference-only vs normative+CONFORMS), and the no-slot error case all had subtle
edge cases. USES and SEE are simple — they're annotations. CONFORMS is an operation disguised as an annotation. It might deserve its own
spec construct rather than being a reference relation.

The self-hosting spec is a great integration test but a poor bootstrap

yass.yass.yaml validating cleanly was our first integration test, and it caught real bugs. But the ../cli@cli.ErrorLine::RETURN ref in
cli.shared.yass.yaml that resolves to a non-existent file is still there. The spec files that define the tool have a ref error that the
tool correctly reports. This is fine in practice — it's a known issue in the spec files, not the tool — but it means yass validate spec/
will never exit 0 on this repo without either fixing the ref or suppressing the error. For a self-hosting language, a clean
self-validation should be a release gate.

Deterministic behavior specs are really design docs

Obligations like "MUST: process files sequentially in DiscoverSpecFiles sort order" and "MUST: run checks per file in the order CheckYAML,
CheckPreamble, CheckSpec, CheckUniqueness, CheckRefs" aren't describing what the system does — they're prescribing how it does it. This
is necessary for deterministic output (error ordering matters for diffing), but it blurs the line between specification and implementation
design. The spec syntax might benefit from distinguishing between behavioral obligations ("MUST exit 1 when errors are found") and
implementation constraints ("MUST process files in sort order") since they serve different purposes and should be verified differently.

The biggest missing piece is spec-to-test generation

Right now the path is: human reads spec → human writes test → human writes code. The spec is structured enough that the middle step could
be automated. Each obligation with a WHEN guard is a test case. Each error code is a negative test case. The slot structure
(INPUT/RETURN/ERROR) maps naturally to test structure (setup/assert/error-assert). The test taxonomy in GUIDANCE.md sketches this but
doesn't formalize it. A yass test-gen command that emits a test skeleton from a spec file would be the single highest-leverage tool
addition.

## Rust Agent

A few categories of feedback, from concrete to strategic.

Spec bugs I worked around

The error code regex is wrong. The cli.errors spec says codes use [a-z0-9.] but every single code uses underscores:
yass.argv.unknown_subcommand, yass.ref.spec_not_found_same_file, etc. The regex should be [a-z0-9._]. I caught this because my test that
validated all constants against the stated character class failed on every code.

yes/no/on/off rule is redundant with YAML 1.2. The Document spec says "MUST treat yes, no, on, off as plain strings rather than booleans."
The Document spec also says "MUST be parsed under the YAML 1.2 core schema." YAML 1.2 core schema already treats these as strings —
that's the entire point of 1.2 vs 1.1. Stating it separately creates ambiguity about whether the implementer needs to do something extra
beyond choosing a 1.2-compliant parser. I'd either drop the obligation or add a note that it restates a YAML 1.2 behavior for emphasis.

The Dispatch spec's bare-dash rule contradicts subcommand specs. Dispatch says WHEN: a positional argument is the bare token - → MUST:
exit 2 with yass.argv.stdin_dash. But cli.validate and cli.list say MUST-NOT recognize the bare - token as a stdin marker; MUST treat - as
a literal path argument. There's no language-level mechanism to express which wins. I implemented it as: Dispatch catches - in the
subcommand position, subcommands handle - in their own positionals. But that interpretation required judgment — the spec didn't guide me
there.

Spec quality observations

The specs are genuinely good enough to implement from cold. I built a working CLI from the specs alone with no reference implementation.
The places I had to exercise judgment (YAML library choice, internal module boundaries, error propagation strategy) were things that
should be unspecified. That's the right level of prescription.

cli.errors is the best spec in the set. It's a complete, machine-readable registry. Every code has an exact message template, an exit
code, and a clear triggering condition. This should be the model for how other structured registries are specified in yass.

cli.query is the weakest spec. It's doing too many things — name lookup, scope validation, CONFORMS inlining, YAML emission, OutputProfile
formatting, multi-match disambiguation. Each of those is a spec-sized concern. The sub-specs (NameLookup, ExtractFragment,
InlineConforms, OutputProfile) help, but the top-level cli.query has obligations that span all of them, creating a cross-cutting
specification that's hard to implement incrementally. Compare with cli.validate, which cleanly decomposes into CheckYAML → CheckPreamble →
CheckSpec → CheckUniqueness → CheckRefs with clear boundaries.

The OutputProfile quoting rules need test vectors. The spec says double-quote when a scalar contains : , leading ?-*&!|>%@,
leading/trailing whitespace, or matches a core-schema type token. But: does 0x1F count as numeric? Does .inf? Does 1_000? Does 1e10? YAML
1.2's definition of "numeric literal" is surprisingly broad. Without test vectors, two conforming implementations will disagree on edge
cases. I'd add an appendix of (input, expected-output) pairs for the quoting rules.

Development process feedback

The one-spec-per-code-file guidance doesn't match the CLI specs. cli.yass.yaml contains Dispatch, ExitCode, and ErrorLine — those ended up
in argv.rs, errors.rs, and error_line.rs respectively. Meanwhile cli.validate.yass.yaml maps cleanly to validate.rs. The guidance should
acknowledge that some spec files are "registry" files (containing related but independent definitions) vs "module" files (one behavioral
unit).

Integration test scenarios are the missing artifact. Each obligation maps cleanly to a unit test. But end-to-end scenarios — "given these
three files on disk, validate should produce this exact stderr and stdout and exit code" — aren't expressible in yass. I generated them by
composing obligations, but a companion artifact (a scenarios/ directory with input fixtures and expected output) would make
cross-implementation conformance testing trivial. This is the fastest way to verify that the Rust, Go, Python, C, and Zig implementations
are actually interchangeable.

CONFORMS inlining cost is disproportionate to its value in v1. Implementing query required building: a ref-target grammar parser, a
cross-file resolver, a slot extractor, a WHEN guard combiner, a provenance comment emitter, and a YAML serializer with custom quoting
rules. That's roughly 40% of the total implementation complexity for a feature that helps humans read specs but isn't needed by any
machine consumer in v1. Consider making inlining opt-in (--inline) with raw extraction as the default.

Strategic observations

Ship one implementation as the conformance oracle. Right now each language implementation is validated against the spec text, which
requires human interpretation. Once one CLI passes a comprehensive scenario suite, that suite becomes the truth — other implementations
just need to produce byte-identical output for the same inputs. The Rust implementation is a reasonable candidate since it's strict about
types and has the spec's error codes baked in as compile-time constants.

The test taxonomy in GUIDANCE.md is the most important unfinished idea. The mapping from (slot × normativity × guard-presence ×
reference-presence) → test obligation type is mechanical and deterministic. If formalized, it turns every yass spec into a test generation
input. That's the killer feature for adoption — "write a spec, get a test suite for free." Prioritize this over language syntax
extensions.

The v1 MUST-NOTs are well-chosen scope fences. No JSON output, no stdin, no subcommand-specific help, no file mutation, no network, no
caching. These prevent scope creep while leaving clean extension points for v2. The only one I'd reconsider is "no subcommand-specific
--help" — users will expect yass validate --help to work, and emitting the top-level help instead is confusing.

## Python Agent

A few things beyond the spec syntax and guidance observations I already gave.

The error table is the real spec

Of the ~900 lines of CLI spec, the error code table (cli.errors) was the most implementation-productive artifact — more than any
individual subcommand spec. It gave me: the complete set of code paths, the exit code for each, and the exact message template. Every
other spec was essentially "here's when you emit these codes."

If I were advising someone building a new yass-specified system, I'd say: write the error table first, then write the specs that reference
it. The current spec set was clearly written the other direction — behavior first, error table extracted — and the table ended up being
the most rigorous part.

This has a design implication: the error table pattern should be promoted from "something the CLI specs happen to do" to a recommended
practice in the guidance. Any spec that describes a system with failure modes benefits from a centralized code table that other specs
reference by code string.

The specs describe trees, not graphs

Each spec describes one function in isolation. The CLI is a pipeline: dispatch → discover → parse → check → emit. That pipeline structure
is never stated in one place. It's reconstructed from USES refs, from obligation prose ("validate YAML well-formedness before running
structural checks"), and from the validate spec's ordering invariant.

During implementation, my first task was reverse-engineering the call graph from scattered references. An LLM doing this without a human
in the loop would likely get the ordering wrong — the spec for CheckPreamble doesn't say "only run after CheckYAML succeeds," it says
"accept a parsed YAML multi-document stream" with a USES ref to CheckYAML. The dependency is implicit.

This isn't a syntax problem — it's a granularity problem. The "one spec per public symbol" rule is correct for the leaf functions, but the
system needs a composition spec that shows how the leaves connect. cli.validate partially does this, but its ordering obligations are
buried in a wall of other concerns.

The spec set has one real bug

spec/cli.shared.yass.yaml line 68 has CONFORMS: ../cli@cli.ErrorLine::RETURN. Since cli.shared.yass.yaml lives in spec/, the ../cli
resolves to <project-root>/cli.yass.yaml, which doesn't exist — the actual file is spec/cli.yass.yaml. The validator correctly reports
this. It's a spec authoring error, not a tooling error. The same pattern appears to affect other ../cli@ refs in that file.

Coverage reality vs. coverage numbers

83% line coverage sounds solid, but the actual spec-adherence coverage is uneven. The areas with deep coverage are the ones with explicit
ERROR obligations — CheckYAML, CheckPreamble, CheckSpec, argv parsing. These are easy to test because the spec enumerates every failure
mode.

The areas with shallow coverage are the behavioral invariants: "MUST line-buffer stdout," "MUST handle SIGPIPE by exiting 0 cleanly,"
"MUST emit LF as the only line terminator on all platforms." These are INVARIANT obligations that require platform-specific or
environment-specific test setups. The test taxonomy says to generate property tests for these, but in practice they're the obligations
most likely to be hand-waved.

The gap suggests that INVARIANT obligations need a different testing strategy than ERROR obligations. ERROR obligations are naturally
one-test-per-code. INVARIANT obligations are naturally property tests or integration tests that cut across multiple operations. The test
taxonomy treats them uniformly, but the implementation experience says they're fundamentally different beasts.

The self-defining spec is clever but mostly inert

yass.yass.yaml defines the language constructs — Document, Preamble, Spec, Slot, Obligation, etc. During implementation, I referenced it
to understand the language rules. But the CLI specs (spec/*.yass.yaml) restated everything they needed from the language spec via USES
refs and prose. I could have implemented the CLI from the spec/ directory alone, never reading yass.yass.yaml.

This suggests the self-defining spec serves a different audience than implementers. It's a language reference, not an implementation spec.
That's fine, but the relationship should be explicit. The CLI specs are the implementation specs; yass.yass.yaml is the grammar spec. An
implementer starts from the CLI specs and refers to the grammar spec for clarification — they don't implement from the grammar spec
directly.

What I'd build next

If this were my project and I had the spec-writing time, the two highest-value next steps are:

1. yass lint — the v1 validator only checks MUST/MUST-NOT structural rules. A linter checking SHOULD violations, empty ERROR slots,
stub-length obligations, and the colon-space footgun would catch the spec-quality problems that valid-but-hollow specs create. This is the
#2 item in IDEAS.md and it's correct — it's the cheapest quality win.
2. A script/test that runs yass validate on the spec files themselves — the CI already has a script/test hook. Making the specs
self-validating in CI would have caught the ../cli@ ref bug before it shipped. This is zero new tooling — just wire up the CLI that now
exists.

## Zig Agent

A few things that go beyond spec syntax and implementation guidance.

The error code table is the best thing in the spec and should be promoted.

cli.errors was the single most useful artifact during implementation. It's a complete, machine-stable enumeration of every failure mode
with its exit code and message template. I used it more than any other spec. But it's structured as a peer of cli.validate and cli.query —
just another spec file. It's actually the contract that every other spec references. If I were onboarding someone to yass, I'd say "read
cli.errors first, then read the spec that emits each code." That reading order isn't obvious from the file structure.

The spec is good at describing one function. It's weaker at describing function composition.

Each spec in isolation is precise and implementable. The difficulty is in the seams — where CheckYAML hands off to CheckPreamble, where
DiscoverSpecFiles feeds into validate, where InlineConforms depends on CheckRefs-style resolution but with different semantics. The
ordering spec in cli.validate handles the check pipeline well, but the data flow between components (what does CheckYAML return that
CheckPreamble consumes?) is implicit. I inferred the interfaces from the obligations. A USES ref from CheckPreamble to CheckYAML tells me
there's a dependency but not the shape of the data at the boundary.

This isn't a yass language problem — it's a "specs describe behavior, not interfaces" observation. But it's the gap that caused the most
implementation rework.

Self-hosting works and is persuasive.

yass validate yass.yass.yaml returning 0 errors is a strong signal. The spec files in spec/ also validate clean (except the one ../cli
cross-file ref, which is a resolution path issue). The fact that the tool can consume its own language definition closes a credibility
loop that most spec systems never reach.

The CONFORMS inlining model is the right abstraction but needs more usage to prove out.

The one-level-deep inlining with provenance comments is the most novel design choice in yass. It solves the DRY problem (reuse obligations
across specs) without the complexity of recursive resolution or inheritance. But in this implementation, I only exercised it on the
specs' own CONFORMS refs (like cli.ExitCode::RETURN). The edge cases — guard combination, normative vs. reference-only carriers, stripping
— are all implemented and tested, but they haven't been stress-tested by real-world specs with deep conformance chains. The v1: one level
only constraint is wise, but the question of whether one level is enough for real domain specs is unanswered.

The "no free prose" constraint worked here. It might not work everywhere.

The yass spec files are clean because they describe CLI behavior — a domain where inputs, outputs, and error conditions are concrete. The
obligations read like requirements because the subject matter is mechanical. I'm less confident the constraint holds for specs describing
business logic, user-facing behavior, or anything where the why matters as much as the what. The preamble description is the only place to
put context, and it's a single field shared across all specs in the file. During implementation, I never missed free prose — but I also
had the context/ directory with GUIDANCE.md, FUTURE.md, and the reference doc as escape valves.

The biggest risk to yass adoption is the YAML footgun, not the spec model.

The :  (colon-space) quoting requirement bit me zero times during implementation because I was generating YAML, not writing it by hand.
But every human author will hit it. The spec acknowledges this as a footgun. A yass fmt or yass lint command that auto-quotes obligation
prose containing :  would eliminate the most common authoring error before it reaches validation. That's higher leverage than any spec
language change.

What I'd build next, in priority order:

1. yass fmt — auto-format and auto-quote spec files
2. Test skeleton generation from the error code table — one test per code with a triggering input
3. A --json output mode for validate — makes CI integration trivial and the spec explicitly defers it to post-v1
4. Cross-file CONFORMS resolution caching — the current implementation re-parses referenced files on every ref

## Bun Agent

A few things beyond the language-syntax and guidance feedback I already gave.

The error code table is the most valuable spec artifact

Every bug caught by blackbox testing (T63, T89) was caught because the error codes are a machine-stable contract. The tester didn't need
to understand the implementation — they asserted on [yass.argv.case_mismatch] in stderr. The 85-code table in cli.errors eliminated nearly
all implementation decisions. If I were speccing a new CLI, I'd write the error code table before any behavioral spec.

The corollary: the error code table is also the worst fit for obligation syntax. Each "obligation" is really a row in a data table (code,
exit, message template). I spent more implementation time parsing prose strings from cli.errors than on any behavioral spec. This is the
strongest argument for a structured error construct in the language — not because it's theoretically cleaner, but because it was the
single largest source of tedious, error-prone work.

The ERROR slot is the highest-value slot for TDD

The implementation cycle that worked was: read ERROR slot first → derive test cases (each obligation is a concrete failure mode with a
specific code) → write tests → implement → verify against RETURN and INVARIANT. ERROR obligations are the most testable because they
specify a triggering condition and an observable outcome. INPUT and RETURN obligations often require reading the whole spec to understand
what "success" means. ERROR obligations are self-contained.

The test taxonomy already says this ("highest-value slot") but the implementation experience makes it stronger: ERROR obligations have
nearly a 1:1 mapping to test cases. Every other slot requires judgment about how many tests to write per obligation.

Specs with clean boundaries parallelize; specs with sequencing don't

I implemented 6 foundation modules (errors, error-line, find-root, discover, glob, yaml-parser) in parallel because each spec defined its
INPUT/RETURN types precisely enough to be independent. This is the payoff of the DbC structure.

The validate pipeline (CheckYAML → CheckPreamble → CheckSpec → CheckUniqueness → CheckRefs) could not be parallelized because the ordering
and skip-on-failure dependencies are prose in the parent spec. The individual check specs don't know they're phases of a pipeline — they
look like standalone functions. This architecture had to be inferred by reading the parent spec carefully, and it's the kind of thing an
implementor gets wrong on first pass.

This suggests a design principle for spec authors: if specs are meant to be composed sequentially, the parent spec's obligations aren't
enough — the child specs should carry some structural signal that they're pipeline phases rather than independent functions. Currently the
only signal is USES, which means the same thing for both "I call this" and "I run after this."

The OutputProfile spec doesn't fit the function model

cli.query.OutputProfile describes a YAML serializer's formatting rules. It uses RETURN for "emit UTF-8 with LF line endings" and INVARIANT
for "don't emit a trailing ...". These aren't really postconditions or universal constraints — they're formatting rules for a serializer.
The spec tried to fit a configuration/ruleset into the function-shaped slot model and it read awkwardly.

This is a case where the slots are being read structurally ("INPUT = the form a thing takes, RETURN = what a well-formed thing denotes")
as yass-reference.md suggests for non-function specs. It works, but the implementor has to mentally translate "RETURN: MUST emit 2-space
indentation" into "this is a serializer setting" rather than "this is a function postcondition." The language handles this correctly —
it's a spec-authoring observation.

Example pairs would complement obligation prose for emitters

The OutputProfile quoting rules ("double-quote when scalar contains : , leading ?-*&!|>%@, leading/trailing whitespace, or matches a YAML
1.2 core-schema type token such as true, false, null, yes, no, on, off") required careful implementation because the rules interact. A
scalar might match multiple conditions, and the implementor needs to know the expected output for each combination.

This is where the spec language's "no free-prose channel" principle has a cost. Example input→output pairs — true → "true", hello world →
hello world, value: with colon → "value: with colon" — would be enormously useful for emitter specs but there's no structured way to
express them. They'd have to be obligation prose like MUST: emit "true" as a double-quoted scalar, which is verbose and fragile.

This isn't a language change suggestion — it's an observation that rule-based specs and example-based specs serve different implementation
needs, and yass is exclusively rule-based. For most specs that's correct. For serializer/formatter specs, it creates extra implementation
friction.

The "at most one error per file" pattern is an implementation hotspot

CheckYAML and CheckPreamble both define "emit at most one error per file in priority order, stopping at first match." This is the right
user experience (users don't want 5 errors for one broken file) but it forces the implementor to build a priority chain with early
returns, rather than the simpler "collect all errors, filter, report" approach. Both T63-class bugs — where one error masks another — live
in this pattern.

If more specs adopt this pattern, it's worth having a named concept for it (something like "priority-gated emission") so implementors
recognize the pattern and its pitfalls consistently.

The spec validates the spec

Running yass validate against its own specs is the strongest confidence signal. The fact that yass validate spec/ (once the cross-file ref
issue with ../cli is fixed for the project layout) would validate the CLI specs using the code those specs define is a powerful bootstrap
property. This is worth preserving as a CI gate — the CLI should validate its own specs as part of its test suite.

Scale is unspecified and might matter

The spec says "process files sequentially" and "open referenced files for existence and target-spec lookup only." These are correctness
constraints, not performance constraints. But for a CLI that might validate hundreds of files in a monorepo, the sequential processing and
per-ref file I/O have performance implications. The spec is correct to not spec performance, but a non-normative note about expected
scale (is this a 10-file tool or a 10,000-file tool?) would help implementors make informed architectural choices. I built everything
synchronous and in-memory, which is fine for dozens of files but would need rethinking for thousands.

## Haskell Agent

A few observations that don't fit under guidance or syntax — things about the system as a whole.

The error message table was the dominant source of rework

This isn't a syntax issue or a guidance issue — it's a workflow issue. The cli.errors spec contains ~45 error codes, each with an exact
message template embedded in prose. The implementation cycle was:

1. Agent writes validation logic with "reasonable" error messages
2. Audit discovers every message is wrong
3. Tedious find-and-replace across ~15 source files and ~20 test files
4. Tests that asserted the old messages all break
5. Another pass to fix the tests

This happened three times across the session. The fix isn't "write better guidance" — it's that the error table should be extractable as
machine-readable data before any code is written. Today it requires reading prose strings, mentally parsing out the backtick-delimited
template, and copying it by hand. A tool that extracts a JSON/YAML lookup table from cli.errors would have eliminated the single largest
category of rework.

CONFORMS is doing too much

CONFORMS is simultaneously:
- A contract assertion ("this spec satisfies that spec's obligations")
- A code generation directive ("inline those obligations into this output")
- A documentation mechanism ("here's where these requirements come from")

The inlining semantics (reference-only vs normative carrier, guard combination with and, provenance comments, one-level-deep limit,
stripping CONFORMS but keeping USES/SEE) are complex enough that they became their own mini-spec (cli.query.InlineConforms) with 15+ MUST
obligations. The guard combination rule alone produced a subtle bug where obligations without their own WHEN guard silently lost the
carrier's guard.

The one-level-deep limit is the most surprising design choice. CONFORMS: Foo::RETURN inlines Foo's RETURN obligations, but if any of those
obligations themselves have CONFORMS refs, they're left unresolved. This means the output of query contains a mix of inlined and
non-inlined CONFORMS refs — structurally inconsistent. The spec explicitly allows this and even allows CONFORMS cycles because of the
depth limit. It works, but it means CONFORMS doesn't mean "fully satisfies" — it means "satisfies one level of."

If I were splitting CONFORMS, I'd separate the contract assertion ("`this spec satisfies that spec") from the output transformation
("inline when rendering"). The contract could be validated statically without inlining. The inlining could be a rendering concern with
clearer, simpler rules.

The self-spec validates clean but the CLI specs don't

Running yass validate yass.yass.yaml produces 0 errors. Running yass validate spec/ produces 1 error — a cross-file ref
../cli@cli.ErrorLine::RETURN in cli.shared.yass.yaml that resolves to a nonexistent file. The ref uses ../ to go up from the spec/
directory, expecting cli.yass.yaml at the project root. But the file is at spec/cli.yass.yaml.

This means the tool you're building can't validate its own spec files clean. The specs are written for human reading (where ../cli is
understood as "the cli spec"), not for machine resolution (where ../cli means "go up one directory and find cli.yass.yaml"). Either the
ref should be ./cli (same directory), or the spec files should be reorganized so the ref resolves.

This is a small thing but it matters for credibility — a validator that can't validate its own specs invites skepticism.

The test taxonomy is the most valuable document and should be promoted

context/TEST-TAXONOMY.md provides a deterministic mapping from spec constructs (slot × normativity × WHEN guard × relation) to test
obligations. This is exactly the right abstraction for systematic test generation. The taxonomy made it possible to verify that every MUST
and MUST-NOT obligation had corresponding test coverage.

But it's buried in context/ as a "holding doc" with caveats about being unsettled. If yass is meant to be used by AI agents to generate
implementations, the test taxonomy is arguably more important than the language spec itself — it's the bridge from "what the spec says" to
"what the tests should verify." I'd promote it to a first-class document.

The v1 constraints are excellent scope control

The pattern MUST-NOT: <feature> in v1 appears throughout the specs — no subcommand-specific --help, no structured JSON output, no stdin
reading, no draft mode, no skip patterns, no cycle detection, no NDJSON. Each one is a feature someone will eventually want, and each one
is explicitly excluded.

This is the best scope control technique in the entire project. It's better than a roadmap because it's embedded in the spec itself — you
can't implement the feature without actively violating a MUST-NOT. It forces the conversation about when to lift the restriction rather
than letting features creep in.

The spec-per-file mapping worked cleanly

The guidance says "one spec file per code file, one spec per public symbol." The CLI specs follow this — cli.yass.yaml for dispatch,
cli.validate.yass.yaml for validate, etc. This mapped naturally to Haskell modules: Yass.CLI, Yass.Validate, Yass.List, Yass.Query. The
1:1 correspondence made it easy to know where to look when a spec obligation wasn't met.

The only friction was with deeply nested specs. cli.validate.CheckYAML is a spec within cli.validate.yass.yaml, which maps to
Yass.Validate.CheckYAML — a submodule. The nesting is natural but the file organization (one .yass.yaml file containing multiple specs)
means you have to read the whole file to find one spec. Per-symbol spec files would be more granular but might create too many small
files.

What I'd want next as an implementer

1. A yass extract-errors command that outputs the error code table as JSON — code, message template, exit code. This would eliminate the
message-alignment rework entirely.
2. A yass check-coverage command that takes a test file and a spec file and reports which obligations have corresponding tests and which
don't. The test taxonomy defines the mapping; the tool should automate it.
3. A yass diff command that compares two versions of a spec and reports which obligations changed, were added, or were removed. This
matters for the "codes MUST NOT change once published" invariant.
