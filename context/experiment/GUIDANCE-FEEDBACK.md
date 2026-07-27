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
Don't make any edits - based on this session, what would you add to the GUIDANCE.md to make implementation from yass specs more effective?
```

## C Agent

Based on the implementation experience, here's what I'd recommend adding:

Spec-to-implementation lessons

Error priority ordering is a hidden contract. Several specs (CheckYAML, CheckPreamble) define "emit at most one error per file in this
priority order." These priority chains are the hardest part to implement correctly — they're easy to miss on first read because they live
in INVARIANT slots rather than ERROR slots, and they create implicit control flow (early returns) that shapes the entire function
structure. Implementers should extract priority chains first and use them as the function skeleton.

The YAML null footgun is real and cross-cutting. YAML 1.2 represents null as bare keys (MUST:), tildes (MUST: ~), and the word null — but
YAML parsers typically surface all of these as scalar events with string content, not as a distinct null type. Any spec that says "value
at a scalar position must not be null" requires the implementation to detect null at the parser layer, not at the checker layer. This
isn't something the spec can express cleanly because it's a YAML-tooling concern, not a domain concern — but it bit every implementation.

Type name collisions with dependencies need upfront planning. Our yaml_node_t collided with libyaml's yaml_node_t. In C, this means the
header design must be settled before any implementation starts. The spec doesn't mention implementation types, but a guidance note like
"prefix all typedefs with the project namespace" would have saved rework across every file.

"Reference-only obligation" is a distinct parse state, not just absence. The spec defines ref-only obligations (CONFORMS/USES/SEE with no
normativity and no WHEN) as carrying no obligation of their own. Implementers need to handle this as a first-class branch in obligation
validation, CONFORMS inlining (replace vs. append), and error checking (guard_without_normativity must not fire on ref-only). The
three-way split — normative, guarded-normative, reference-only — should be identified during spec reading, not discovered during
debugging.

Specs that cross-reference create implicit integration ordering. cli.validate references CheckYAML → CheckPreamble → CheckSpec →
CheckUniqueness → CheckRefs as a pipeline where each step gates the next. cli.query references NameLookup → ExtractFragment →
InlineConforms → OutputProfile. These pipelines aren't explicit in the spec structure — they emerge from USES refs and prose. Drawing the
pipeline DAG before coding would have prevented the "which module do I implement first" question that stalled parallel work.

The error code table is the integration test oracle. cli.errors defines every error code, its message template, and its exit code. This is
the most valuable spec for test generation — each row is a test case. Implementers should generate a test stub per error code before
writing any implementation.

Path formatting rules appear in three places and must agree. cli.ErrorLine, cli.DiscoverSpecFiles, and cli.list all specify how paths
should be formatted (relative to cwd, no leading ./, basename when directly in cwd, absolute when not under cwd). These are the same rules
but stated in three specs. A shared implementation is essential — we extracted path_relative_to_cwd and error_format_path — but the spec
structure doesn't make this obvious. When the same behavioral rules appear in multiple specs, that's a signal to implement once and
CONFORMS.

## Go Agent

Good question. Here's what I'd add based on what actually went wrong and what worked during this implementation:

---
Spec-to-implementation lessons (Go CLI, June 2026)

1. ErrorLine is a protocol, not a format string

The cli.ErrorLine spec constrains every byte written to stderr. This is easy to miss — we shipped usage text to stderr alongside
ErrorLines and didn't catch it until audit. Treat ErrorLine as a wire protocol: every fmt.Fprint(stderr, ...) call must produce a
conforming line, or it's a spec violation.

2. Cross-cutting constraints are the highest-risk obligations

The obligations that span all subcommands — colon-in-path rejection, exit code enumeration, path formatting rules — broke in three places
independently because each subcommand implemented them separately. Extract these into shared functions during scaffold, not after the
fact.

3. Priority-ordered "at most one error" specs need a decision table, not prose

CheckPreamble and CheckYAML both specify "emit at most one error in priority order X > Y > Z." These are the hardest obligations to get
right because the priority list is long and the prose is ambiguous about edge cases (e.g., is "missing" before or after "duplicate"?).
Convert these to an explicit ordered list in implementation comments before writing code.

4. CONFORMS inlining is a separate output pass, not a parse-time feature

The query OutputProfile has strict formatting rules (2-space indent, specific quoting, key reordering, provenance comments at column 0).
Using yaml.v3's built-in encoder won't match the profile — you need a custom emitter. Recognize this upfront and plan for it rather than
trying to coerce the library's output.

5. Ref resolution has two independent coordinate systems

RefTarget paths resolve from two different roots depending on prefix: ./ and ../ resolve relative to the referencing file's directory,
bare paths resolve from the project root. These are easy to conflate. The implementation should have a single resolveRefPath(target,
fileDir, projectRoot) function that handles both cases explicitly.

6. The YAML 1.2 vs 1.1 boolean gap must be handled at parse time

gopkg.in/yaml.v3 treats yes/no/on/off as booleans (YAML 1.1). The yass spec requires YAML 1.2 core schema where these are strings. Fix
this in the parser layer once (walk nodes, convert !!bool → !!str for these values) rather than in every consumer.

7. Scaffold all public API signatures before implementing

Parallel implementation of functions in the same Go package fails when agents don't know each other's types. Define all function
signatures, structs, and constants in a types/scaffold file first. The implementation can replace stubs, but the API surface must exist
from the start.

8. Spec files can have spec violations

Our own spec/cli.shared.yass.yaml has a ../cli@cli.ErrorLine::RETURN ref that resolves to a non-existent file (it should be ./cli@...).
The CLI correctly reports this. Don't assume spec files are error-free — the validator should be tested against the repo's own specs as an
integration test, and the results should be understood, not suppressed.

## Rust Agent

Based on this session, here's what I'd recommend adding:

Lessons for spec-driven implementation

1. Error code tables are the skeleton — build them first.
The cli.errors spec is the single source of truth for every diagnostic the CLI can emit. Implementing it as a constants module before
anything else gave every other module a shared vocabulary and made exit-code mapping trivial. Any yass-specified system with an error
table should start there.

2. Specs define check ordering — respect it as architecture.
cli.validate specifies CheckYAML → CheckPreamble → CheckSpec → CheckUniqueness → CheckRefs, with "skip remaining checks if YAML fails."
That ordering isn't advisory — it's the control flow. Treat spec-mandated sequencing as the function call graph, not just documentation.

3. "At most one error per file" and "one error per failing rule" are different contracts.
CheckYAML emits at most one error (priority-ordered). CheckSpec emits one per violation. CheckPreamble emits at most one (stop at first
match). These aren't interchangeable patterns. The spec must be explicit about error cardinality per check phase, and the implementer must
read it literally.

4. Path formatting rules are surprisingly load-bearing.
The ErrorLine spec's path rules (relative when under cwd, basename when directly in cwd, absolute otherwise, forward slashes always, no
symlink resolution) affect every module that reports errors. Centralizing format_path early prevents inconsistency. Specs should co-locate
all path-emission rules in one place rather than scattering them.

5. CONFORMS inlining is the hardest single feature to get right.
It requires: ref-target grammar parsing, cross-file resolution, slot extraction, WHEN guard combination (and literal, no parentheses),
provenance comment placement, and the distinction between reference-only vs normative carrier obligations. Specs should separate the
inlining algorithm from the output profile — they're independently testable contracts.

6. "MUST-NOT" obligations are test cases, not just constraints.
Every MUST-NOT in the spec is a negative test: "no ANSI escapes," "no locale-aware collation," "no symlink following during traversal."
These are easy to miss during implementation but trivial to test for. A systematic pass through MUST-NOT obligations generates high-value
edge-case tests.

7. The spec should pin message templates byte-for-byte.
The cli.errors spec defines message templates like "unknown subcommand: <arg>". These are the contract for machine consumers. Without
byte-exact templates, implementers will paraphrase ("unknown command 'foo'" vs "unknown subcommand: foo"), breaking any tooling that
parses stderr.

8. Distinguish argv-level validation from subcommand-level validation.
The Dispatch spec and the validate/list/query specs have overlapping concerns (e.g., bare - handling). The Dispatch spec sets defaults;
subcommand specs can override. This layering should be made explicit in guidance: "subcommand specs override Dispatch-level behavior for
their own positionals."

9. Reference resolution needs a project-root anchor.
FindProjectRoot is used by both validate (for CheckRefs) and query (for CONFORMS inlining). The spec correctly mandates computing it once
at startup. Any spec that involves cross-file references should explicitly state the root-resolution strategy and when it's computed.

10. TTY-awareness specs need explicit measurement rules.
The list truncation spec defines how to measure line length (grapheme clusters, tabs as one column) and the truncation marker (... as 3
ASCII chars). Without these, implementations will diverge on Unicode handling. Specs involving terminal output should always pin the
measurement unit and marker format.

## Python Agent

Based on this session, here's what I'd add:

Lessons for spec-driven implementation

1. Error code tables are the skeleton — build them first

The cli.errors spec is the single most valuable artifact for implementation. Every error code defines an exit code, a message template,
and implicitly a code path. Building the error module first gave every subsequent module a shared vocabulary and eliminated naming drift.
Start every implementation from the error table.

2. Specs need to declare data flow, not just behavior

The biggest friction was cross-function contracts. inline_conforms needs the source filepath to resolve same-file refs, but the spec
describes it as "accept a parsed Spec with its obligations" — no mention of the filepath dependency. The implementation discovered this at
integration time. Specs that say "resolve a same-file ref" should also say "given the file the spec was loaded from." INPUT slots should
name the data a function actually needs, not just the logical input.

3. CheckYAML priority ordering is a spec for the control flow, not just the checks

The CheckYAML spec says "emit at most one error per file, preferring not_utf8, then has_bom, then empty_file, then malformed, then
duplicate_key, then anchor_or_alias." This looks like a simple priority list but it's actually a control flow spec: you must run checks in
that order and short-circuit. The explicit tag case (!custom) exposed this — PyYAML's SafeLoader throws a ConstructorError (which looks
like "malformed") but the YAML is structurally valid; only the tag is forbidden. The implementation needed to scan tokens before loading
to correctly classify the error. Priority-ordered error lists are implicitly control flow specs. Call that out.

4. CONFORMS refs create implicit integration contracts

A spec like cli.Dispatch that says CONFORMS: cli.ExitCode::RETURN creates a hard dependency: the query subcommand must be able to resolve
same-file, cross-file, and project-root-relative refs at runtime. The spec for InlineConforms describes the behavior of inlining but
doesn't fully spec the resolution paths (same-file re-reading, directory resolution for ./ vs root-relative). CONFORMS refs should be
accompanied by resolution examples showing each path variant.

5. Output format specs need concrete examples

The OutputProfile spec describes quoting rules, indentation, key ordering, and provenance comment placement. Implementing a custom YAML
emitter from prose alone required significant trial-and-error. A single concrete before/after example — "given this input spec with a
CONFORMS ref, the emitted fragment looks like this" — would have eliminated ambiguity. Every output format spec should carry at least one
worked example as a SHOULD obligation.

6. Path formatting is a cross-cutting concern that should be spec'd once

Three specs independently describe the same path formatting rules (relative to cwd, no leading ./, basename when directly inside cwd,
absolute when not under cwd, forward slashes). ErrorLine, DiscoverSpecFiles, and list each restate them. In implementation, this became a
single format_path() function. Factor cross-cutting formatting rules into a shared spec and CONFORMS to it.

7. The test taxonomy works — but needs the spec to be implementation-aware

The test taxonomy correctly maps normativity keywords to test obligations. But the highest-value tests came from ERROR slots, which
enumerate exact failure modes. The taxonomy could add: for every error code in the error table, there MUST be at least one test that
triggers it and asserts the exact code string. This is the cheapest exhaustiveness guarantee.

8. "MUST-NOT" obligations are the most frequently undertested

Specs like "MUST-NOT follow symbolic links during traversal" and "MUST-NOT match files whose basename begins with ." define invisible
behavior — things that don't happen. These are easy to skip when writing tests manually but are the obligations most likely to regress.
Agents should generate negative tests for every MUST-NOT before positive tests for MUST.

## Zig Agent

Based on this session, here's what I'd add:

---
Distinguish "same-key" from "same-concept" constraints

The duplicate normativity bug was a direct consequence of conflating two distinct failure modes:

- Same key twice (MUST + MUST) — caught by the YAML layer as yass.yaml.duplicate_key
- Multiple keys from the same category (MUST + SHOULD) — a structural obligation rule, caught at the spec layer as
yass.obligation.duplicate_normativity

An LLM implementing from the spec reads "duplicate Normativity keyword" and writes seen.contains(key) — which is the literal
interpretation but the wrong one. YAML already forbids literal key duplication, so the obligation-level check is only useful for the
cross-keyword case.

Candidate guidance: When a spec says "rejected when it carries X more than once" and X is a category of keys (not a single key), the check
must detect multiple distinct members of that category, not just the same member repeated. The same-member case is already handled by
YAML's duplicate-key rule and is unreachable at the spec layer.

Error codes are a contract, not just labels

The error code table (cli.errors) is the single source of truth for every error the CLI can emit. During implementation, agents treated it
as a reference list but didn't use it as a checklist. The duplicate normativity bug would have been caught earlier if each error code had
been paired with a concrete triggering input during implementation.

Candidate guidance: For every error code in the table, the implementor SHOULD construct a minimal triggering input before writing the
detection logic. If the constructed input is unreachable (e.g., YAML rejects it first), that's a signal that the detection must operate on
a different dimension than the obvious one.

Spec obligations that shadow lower-layer rejections need explicit test inputs

Several spec obligations describe conditions that a lower layer (YAML parsing) will reject before the spec-layer check runs. These are the
highest-risk obligations for incorrect implementation because the implementor can write a check that appears to work (the error fires on
a test input) but actually never executes — the lower layer caught it first.

Examples from this session:
- yass.obligation.duplicate_normativity — YAML catches same-key, spec must catch cross-key
- yass.yaml.duplicate_key vs yass.spec.duplicate_name — different layers, similar language

Candidate guidance: When two obligations at different layers describe superficially similar conditions (duplicate keys vs. duplicate
names, malformed YAML vs. malformed spec names), the test for the higher layer MUST use an input that the lower layer accepts. Otherwise
the test passes vacuously.

Arena allocators and resource ownership in CLI tools

The memory leak bug happened because parse results were allocated by library functions and returned to callers that never freed them. In a
CLI that processes files and exits, an arena allocator is the correct default — allocate everything into one arena, free it all on exit.

This isn't yass-specific, but it surfaced because the spec says nothing about resource management (correctly — it's an implementation
concern). An implementor following the spec alone will focus on correctness and miss cleanup.

Candidate guidance (implementation-facing, not spec-facing): When implementing a CLI from a yass spec, prefer arena allocation for the
main execution path. The spec defines what the tool does, not how it manages memory. Resource cleanup is an implementation obligation the
spec intentionally does not cover.

Cross-file ref resolution needs explicit path algebra

The ref target spec (RefTarget) defines precise path resolution rules (./ = relative to referencing file, no dot = project root, append
.yass.yaml). During implementation, the first version of resolveRefFilePath ignored the project root parameter and didn't append the
.yass.yaml suffix — producing paths that never resolved.

Candidate guidance: When a spec defines path resolution rules, the implementor SHOULD write a dedicated test for each resolution form
(./path, ../path, bare-path, path@Spec, path@Spec::SLOT) with concrete filesystem fixtures before implementing the resolver. Path algebra
bugs are invisible until integration testing.

## Bun Agent

Based on this session, here's what I'd add:

Obligation key enumeration is a completeness trap

The spec defines small closed sets (5 normativity keywords, 5 slots, 3 relations, 1 guard). Implementors correctly enumerate the valid
members but fail to implement the cross-product checks. T63 is the canonical example: the code checked "have I seen this exact keyword
before?" instead of "have I seen any keyword from this class before?" — a bug invisible to per-keyword unit tests but caught instantly by
a two-keyword fixture.

Guidance: When a spec says "exactly one X from set S", test with two different members of S, not just the same member twice. YAML
duplicate-key rejection masks same-key duplicates, so the only reachable bug is the cross-member case.

Case-sensitivity obligations propagate across token classes

The spec defines case-mismatch detection for "a subcommand or flag." Implementors handle the first noun (subcommands) and forget the
second (flags) because the two are parsed in different code paths. T89: --Help was routed to the flag handler which only knew
--help/--version exactly, never checking whether the input was a case-variant of a known flag.

Guidance: When a spec obligation names multiple token classes in a single rule (e.g. "subcommand or flag"), the implementation must cover
every class. Treat "or" in obligation prose as an exhaustiveness checklist, not a suggestion.

Error priority ordering is spec-load-bearing

Several specs define explicit priority orderings (CheckYAML's 6-code priority, CheckPreamble's 9-code priority). These aren't style
preferences — they're the spec's way of making error output deterministic. An implementor who reports errors correctly but in wrong
priority order will fail blackbox tests that assert on the first error emitted.

Guidance: When a spec defines "emit at most one error per file in the order (1)…(N), stopping at the first match," the implementation must
use early-return control flow or an explicit priority chain, not a collect-and-filter approach that risks reordering.

Spec prose hides boolean complements

The WHEN guard states a sufficient condition, but the absence of that guard is also load-bearing. Example: WHEN: exactly one spec matches
→ emit YAML fragment implicitly means WHEN: more than one matches → do NOT emit YAML fragment. Implementors test the positive branch and
miss the negative.

Guidance: For every WHEN guard, derive the complement condition and verify the obligation does not fire. This is already in
TEST-TAXONOMY.md ("generate the complementary branch") but deserves emphasis as an implementation pitfall, not just a test obligation.

Path-format obligations are platform-fragile

cli.ErrorLine specifies relative-path emission rules (strip ./, basename when directly in cwd, absolute when outside cwd). These rules
interact with path.resolve, path.relative, symlinks, and OS-specific separators in ways that are easy to get individually right but wrong
in combination. The implementation needed formatFilePath as a standalone testable function to get this right.

Guidance: When a spec defines path-formatting rules, extract them into a pure function that takes (absolutePath, cwd) → displayPath and
test it in isolation before wiring it into error output. Don't inline path formatting into the emitter.

Cross-file reference resolution needs a project-root anchor

Ref targets like ./cli.shared@Foo and cli@Foo resolve relative to different roots (referencing file's directory vs. project root). The
project root must be computed once and threaded through, not re-derived per file. This is explicit in the spec ("computed exactly once per
invocation") but easy to violate when the resolve call is buried inside a per-file validation loop.

Guidance: When a spec says "computed exactly once," enforce it structurally — compute the value at the call site and pass it as a
parameter, never let the callee recompute it. This prevents both correctness bugs and performance issues.

The "at most one per (X, Y) pair" dedup pattern

CheckRefs requires at most one file_not_found or file_not_parseable error per (referencing-file, referenced-file) pair. This dedup is easy
to miss because it's a constraint on error emission, not on validation logic. The validator finds three bad refs to the same missing file
and must suppress two of the three errors.

Guidance: When a spec constrains error multiplicity ("at most one per …"), implement the dedup as an explicit set at the emission layer,
not as early-exit logic in the checker. The checker should find all violations; the emitter should suppress duplicates.

## Haskell Agent

I have the GUIDANCE.md content from earlier in the session. Here's what I'd add based on the hard lessons from this implementation:

---
Lessons for implementing from yass specs

1. Error messages are contracts, not suggestions

Every cli.errors entry specifies an exact message string. Treat these as byte-for-byte API contracts. The pattern message \`` in the
errors spec means that template IS the message — not a paraphrase, not a rewording. An agent left to generate "natural" error messages
will produce plausible-but-wrong text every time.

What went wrong: First-pass implementation produced messages like "preamble is missing 'description' key" when the spec required "Preamble
missing description". Every single error message across CheckPreamble, CheckSpec, CheckUniqueness, CheckRefs, and Discover had to be
rewritten. This was the single largest source of rework.

Guidance: Extract the error message table from cli.errors into a lookup structure (map from error code to template) before writing any
validation logic. Reference that table from every error emission site. Never invent message prose.

2. Priority ordering is load-bearing

Several specs define "at most one error per file, in this order, stopping at first match." This isn't a soft preference — it's a MUST. The
ordering determines which error code fires for inputs that violate multiple rules simultaneously.

What went wrong: CheckPreamble's priority chain (has_spec_key > empty_stream > missing > duplicate > misplaced > missing_description >
...) was implemented with the wrong branching structure. A single non-first doc without a spec key was classified as misplaced instead of
duplicate, because the code tested count > 1 for duplicate and count == 1 for misplaced. The spec's priority says duplicate (4) beats
misplaced (5), so any non-first preamble-like doc is duplicate regardless of count.

Similarly, the Parser checked BOM before UTF-8 validity, but the spec says not_utf8 has higher priority than has_bom. A file starting with
a valid UTF-8 BOM followed by invalid bytes must report not_utf8, not has_bom.

Guidance: When implementing priority chains, write the chain as a linear sequence of guards matching the spec's numbered order. Test each
priority pair with an input that violates both rules — verify the higher-priority error wins.

3. The parser boundary determines which checks are reachable

The YAML parser's strictness determines which downstream error codes are reachable. If the parser rejects duplicate keys at the YAML level
(yass.yaml.duplicate_key), then the semantic check for duplicate reference relations in an obligation
(yass.obligation.duplicate_reference) becomes unreachable — you can't have two USES keys in a YAML mapping if the parser already rejected
duplicate keys.

What went wrong: The obligation.duplicate_reference error code was implemented and tested in unit tests using hand-constructed RawDocument
values, but could never fire through the CLI because the parser catches duplicate keys first. This isn't a bug — it's a consequence of
the spec's own layering — but it means integration tests for this code are impossible to write.

Guidance: Map out which error codes are shadowed by earlier checks in the pipeline. Document which codes are unreachable-by-design. Don't
waste time trying to create integration tests for unreachable paths.

4. Spec-internal references may be intentionally broken

The spec files themselves may contain cross-file references that don't resolve. For example, spec/cli.shared.yass.yaml contains CONFORMS:
../cli@cli.ErrorLine::RETURN, which resolves to a cli.yass.yaml at the project root that doesn't exist (the file is at
spec/cli.yass.yaml). The validator correctly reports ref.file_not_found for this — it's a spec authoring issue, not a tool bug.

Guidance: Don't "fix" the tool to make the spec files validate clean. The tool's job is to report what the spec says to report.

5. Signal handlers need exitImmediately, not exitWith

In Haskell, signal handlers run in a restricted context. System.Exit.exitWith throws an ExitCode exception, which only works if there's a
handler on the main thread to catch it. From a signal handler, the exception is silently swallowed and the process continues.
System.Posix.Process.exitImmediately actually calls _exit() and terminates.

What went wrong: The initial SIGPIPE/SIGINT/SIGTERM handlers used a helper that called hFlush and return () — they flushed buffers but
never actually exited. The process kept running after receiving the signal.

Guidance: Always use exitImmediately from System.Posix.Process in signal handlers. Never use exitWith or throw exceptions from signal
handler context.

6. canonicalizePath resolves symlinks; makeAbsolute doesn't

The spec says MUST-NOT: resolve symbolic links before deciding the path form. Haskell's canonicalizePath is realpath(3) — it resolves all
symlinks. makeAbsolute prepends cwd and normalizes ./.. without touching symlinks.

What went wrong: findProjectRoot used canonicalizePath, causing downstream path comparison (for relativization) to fail when any directory
in the path was a symlink. The lexical cwd and the canonicalized cwd would differ, so paths under cwd appeared to be "not under cwd" and
were emitted as absolute.

Guidance: Use makeAbsolute everywhere the spec says paths should be lexical. Reserve canonicalizePath for cases where the spec explicitly
requires resolving symlinks (none do in yass).

7. -- end-of-options needs two things

The spec requires -- to mark the end of options: everything after -- is a positional argument. This requires:
1. Flag checking must stop at --: The flag-validation pass (checkSubcmdFlags) must only scan args before --.
2. -- itself must be stripped: The subcommand handler must not see -- as a positional argument, otherwise it tries to process it as a file
path.

What went wrong: The first fix added takeWhile (/= "--") to the flag check but still passed all args (including --) to the subcommand. The
second fix added stripEndOfOptions to remove -- from the arg list before passing to the handler.

8. The OutputProfile quoting rules are precise

The spec lists exactly when a scalar must be double-quoted: contains :  (colon-space), has a leading character in ?-*&!|>%@, has leading
or trailing whitespace, or matches a YAML 1.2 core-schema type token (true, false, null, yes, no, on, off, numeric literals). "Leading"
means the first character of the value, not any character.

What went wrong: Early implementations either over-quoted (quoting any string containing special characters anywhere) or under-quoted
(missing numeric literals). The key ordering obligation (Normativity → WHEN → References) was also missed — the emitter preserved source
order instead of enforcing canonical order.

9. CONFORMS inlining has four interacting rules

The InlineConforms logic has four rules that interact:
1. Reference-only (no normativity on carrier): replace carrier with inlined obligations
2. Normative + CONFORMS: keep carrier (sans CONFORMS), append inlined after
3. Guard combination: if carrier has WHEN, inject/combine it on each inlined obligation
4. Provenance: emit # CONFORMS: <target> at column zero above inlined items

Rule 3 has a subtle case: when the carrier has a WHEN guard but an inlined obligation does NOT have its own WHEN, the outer guard must be
ADDED as a new WHEN entry — not silently dropped. The guard combination rule (outer " and " inner) only applies when both exist.

What went wrong: The first implementation only combined existing WHEN entries — it never injected a new WHEN when one was missing.
Obligations without their own guard lost the carrier's guard entirely.

10. Test infrastructure: always use finally for cwd restoration

Many tests create temp directories and setCurrentDirectory into them. If any assertion fails before restoring cwd, every subsequent test
that calls getCurrentDirectory fails with "does not exist" — creating a cascade of 80+ failures from one root cause.

What went wrong: Exactly this. One error message mismatch caused an assertion failure in a test that had setCurrentDirectory tmpDir
without finally. Every subsequent test failed because cwd pointed to a deleted temp directory. The fix was wrapping all cwd-changing test
helpers in Control.Exception.finally.

Guidance: Never use bare setCurrentDirectory in tests. Always wrap in bracket or finally to restore the original cwd. Better yet, use
withCurrentDirectory if available, or pass absolute paths to the functions under test rather than changing cwd.
