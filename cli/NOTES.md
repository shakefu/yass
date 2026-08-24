# Build notes — the yass CLI

What the specs left me guessing about, what I chose, and what I could not
satisfy. Written to the instruction in `build-prompt.md`: an obligation skipped
and named is more useful than one silently approximated.

## Language and libraries

**Go 1.24.** One non-stdlib dependency: `gopkg.in/yaml.v3` v3.0.1, used only to
get a node tree. Everything else is hand-rolled or standard library:

- the emitter for `query` — direct string formatting, no serializer;
- the `--filter` glob matcher — `path.Match` refuses to let `*` cross `/`,
  which `05-list@List::INPUT` requires it to do;
- case folding — `unicode.SimpleFold`, which is the *simple* folding
  `06-find@Find::INPUT` asks for;
- traversal — `filepath.WalkDir`, which does not follow symlinks, plus
  `DirEntry.Type()` for the file-versus-directory symlink distinction.

Layout: `cli/main.go` is a six-line entry point; everything else is in
`cli/internal/yass/`, one file per spec file, plus `emit.go`, `output.go`,
`model.go`, `target.go`, `fileset.go`, `glob.go`, `fold.go`, `errors.go`.

### One correction to the build prompt

`build-prompt.md` warns that `gopkg.in/yaml.v3` tags `yes`/`no`/`on`/`off` as
`!!bool`. It does not — v3 implements the YAML 1.2 core schema and resolves only
`true`/`false` as booleans; I verified this against v3.0.1 before relying on it.
The retag pass is still in `load.go` (`retagNorway`), so
`02-load@Load::INPUT`'s obligation is a property of the model rather than of a
library version that could change under it.

## Reading scope

I read exactly what the build prompt fences: every `.yass.yaml` in `cli/`,
`build-prompt.md` itself, `../yass.yass.yaml`, `../context/yass-reference.md`,
and `../context/GUIDANCE.md`. Named as required, one file outside that list:

- **`../root.yass.yaml`** (10 lines of spec). `cli/root.yass.yaml@YassCli::INVARIANT`
  and `08-validate@Validate::INVARIANT` both carry `SEE: ../root@Yass` for the
  rule that the language definition — not the derived JSON Schema — is
  normative, so I opened the referenced document. It is a spec file the CLI is
  pointed at, not prior art.

I did not open `experiment/`, `context/experiment/`, `yass.v1.schema.json`, the
git history, the changelog, the CI configuration, any other branch or worktree,
or the web.

## Ambiguities found, and how I resolved them

### 1. Document kind is decided by keys, not by position

`02-load@Load::RETURN` says each document is tagged "by whether it is the first
document **and** by whether it carries a spec key or a design key". Read
literally, the first document is always a preamble — but then
`05-list@List::ERROR`'s guard "an addressed file parsed but carries no preamble
as its first document" could never fire, and `08-validate`'s
`yass.preamble.missing` ("a file's first document carries a `spec` key or a
`design` key") would be unreachable too.

**Chosen:** kind is decided by keys alone — a `spec` key makes a spec, a
`design` key makes a design, neither makes a preamble — which is also
`../yass@Preamble::INPUT`'s structural identification. Position decides only
whether the preamble sits legally, which is validation's question.

### 2. `--help` writes the table, not the paragraphs around it

`00-invocation@Synopsis` calls its content "the exact text `--help` writes.
Nothing is added to it and nothing is left out." But the block opens with a
two-line paragraph *about itself* ("This block is both the option table…") and
closes with a paragraph defining TARGET and PATH — while `root@Brevity` says
help is "a synopsis line, one line per subcommand, and one line per global
option, plus the exit-code table. No examples, no paragraphs, no epilogue."

**Chosen:** Brevity wins. `--help` writes from the `usage:` line through the
exit-code line, verbatim, and drops both prose paragraphs.

### 3. Every diagnostic uses the five-field record, on either stream

`root@DiagnosticFormat` introduces itself as "the record form shared by
`yass validate` and `yass lint`", which would leave the format of, say,
`yass.args.no_subcommand` unstated. But its own area list includes `args` and
`io` — areas only ERROR-slot diagnostics ever use — and `root@YassCli::INVARIANT`
binds the design program-wide with `USES`.

**Chosen:** every diagnostic this program writes, on stdout or stderr, is the
same five fields: SEVERITY, CODE, LOCATION, LINE, MESSAGE. LOCATION holds the
most precise ref target known, or the path being named when the failure is
about a path rather than a document, or `-` when neither exists (an argument
error).

### 4. Provenance comment placement for an appended design

`04-query@Resolution::RETURN` requires the `# USES: ` comment to *precede the
appended design document*; `04-query@Query::RETURN` requires each fragment to be
*opened by a `---` line*. Putting the comment before the `---` satisfies the
first and breaks the second.

**Chosen:** `---`, then `# USES: <target>`, then the `design:` key. The comment
still immediately precedes the document's content and every fragment still
opens with `---`.

### 5. Which codes reference resolution selects

`04-query@Resolution::ERROR` enumerates exactly four codes —
`yass.ref.syntax`, `yass.ref.unresolved`, `yass.ref.conforms_design`,
`yass.ref.uses_spec` — plus a residual that reports `yass.ref.unresolved`.
`08-validate` defines three finer codes for conditions Resolution's list does
not name: `yass.ref.slot_unknown`, `yass.ref.slot_on_design`,
`yass.ref.slot_undeclared`.

**Chosen:** Resolution's enumeration is the contract, so during `query` those
three conditions fall to the residual and are reported as `yass.ref.unresolved`.
`validate` still names them precisely, which is where `08-validate@Validate::INVARIANT`
says rule-naming belongs. (Note this is only for references *inside* a spec
file. A `::SLOT` problem in a command-line TARGET is target resolution's, and
`02-load@TargetResolution::ERROR` does name all three codes there.)

### 6. `yass.obligation.not_scalar` versus `yass.ref.not_a_string`

Both could claim a relation value that is a mapping, a sequence, or null.
Emitting both would be double-reporting one malformed value.

**Chosen:** the grouping in `08-validate@Validate::RETURN` decides it. The
`not_scalar` obligation sits inside the Obligation/Guard group, before
`CONFORMS: ../yass@Reference`, so it is applied to normativity and `WHEN`
values; `not_a_string` sits inside the Reference group and is applied to
relation values. One diagnostic per malformed value.

### 7. What `--raw` means by "exactly as its source file states it"

The clause is followed by three qualifiers — "resolving no reference, inlining
nothing, and inserting no comment" — all of which describe *resolution*
behaviour, not byte fidelity to the source text. Byte-slicing the source would
also fight `04-query@Query::INPUT`'s unguarded obligation to emit a `::SLOT`
target "under the addressed document's `spec` key", which no contiguous slice of
the source produces.

**Chosen:** `--raw` re-renders through the same emitter with resolution switched
off. Consequence, stated plainly: a scalar authored as a folded block (`>`)
comes back as a literal block (`|`). The *string* is byte-for-byte what the
source holds; the *presentation* of it is not always the source's.

### 8. `--` before the subcommand token

`00-invocation@Invocation::INPUT` says the first argument that is neither a
global option nor a global option value is the subcommand token, and separately
that `--` is end-of-options. Taken literally the first rule makes `--` itself
the subcommand token and `yass -- root` a usage error.

**Chosen:** `--` ends the global option phase; the next argument is the
subcommand token whatever it begins with, and every argument after that is an
operand. `yass -- root` runs `root`; `yass find -- --raw` searches for the
literal string `--raw`.

### 9. An empty path token or an empty name token

`@Name` and `a@` both pass the ref-target charset rules of
`../yass@RefTarget::ERROR` but address nothing. Both are rejected as
`yass.ref.syntax`.

### 10. A reference-only obligation carrying a mixture of relations

`04-query@Resolution::RETURN` says a reference-only obligation whose relation
transcludes "resolve[s] it exactly as a carrier obligation resolves, then emit[s]
no obligation of its own". An obligation carrying, say, a slot-targeted
`CONFORMS` *and* a `SEE` is therefore dropped in full, taking the `SEE` with it.
Implemented as written; flagging it because the `SEE` disappears silently from
the fragment.

### 11. `--filter` and an unparsed file's record

`05-list@List::INPUT` says emit only records whose REF matches the glob;
`::ERROR` says an unparsed file's record is emitted with `-` for DETAIL and
DESCRIPTION.

**Chosen:** the filter still governs whether the record is written, but the
findings status is selected regardless — the file was addressed and it did fail,
and the exit status is about the spec set rather than about the glob.

### 12. "Whitespace", for field collapsing

`root@RecordFormat` says "each run of whitespace within a field value is
collapsed to one ASCII space" without pinning the character class, which
`GUIDANCE.md` (*Input segmentation*) warns against. Implemented as
`unicode.IsSpace`, so U+00A0 and friends collapse too. The hard obligations —
never a TAB, a CR, or an LF in a field — hold under any reading.

### 13. An empty slot in an emitted fragment

A spec may declare a slot with no obligations (lint reports it as
`yass.lint.slot_empty`, so the language permits it). Emitted as `SLOT: []`, so
the fragment reads back as a slot holding an empty obligation list rather than
as a null.

### 14. The program version

`00-invocation@Invocation::RETURN` requires a version line holding "the program
name, one ASCII space, the program version, one ASCII space, and the yass
language version it implements", but no obligation fixes the program version.
Set to `1.0.0`; the language version is the specified `v1`. Output:
`yass 1.0.0 v1`.

### 15. `Slot.SIDE-EFFECT`-style names in `SEE` targets

`09-lint@Lint::INVARIANT` carries `SEE: ../yass@Slot.ERROR` — a whole-document
target naming the spec `Slot.ERROR`, not a slot target. The ref-target grammar
resolves it as a document name, which is what the file holds, so it resolves.
Noting it because `@Slot.ERROR` reads at a glance like a mis-typed `::ERROR`.

## Obligations I could not fully satisfy

### A round-tripped REF for a file above the project root

`root@RecordFormat` says a REF is written project-root-relative "so it can be
passed back to `yass query` unchanged". For a file above the project root — which
`02-load@TargetResolution::RETURN` explicitly allows, with `MAY: yield a file
outside the project root` — the root-relative rendering begins `../`, and
`02-load@TargetResolution::INPUT` resolves a leading-dot path token against the
**starting directory**, not the project root. So the round trip holds only when
the starting directory *is* the project root.

I implemented the rendering the spec requires and left the gap rather than
inventing a different rendering, which would break the root-relative obligation
outright. In practice: `yass -C <root> list ../other.yass.yaml` emits
`../other@Doc`, and `yass -C <root> query ../other@Doc` resolves it correctly;
running from a subdirectory does not.

### SIGPIPE has to be intercepted for `exit 0` on a closed pipe to be reachable

`root@ExitPolicy` says "A signal is not caught, so no status outside this table
is ever selected by this program", and `root@OutputPolicy` says a write to
standard output that fails because the reading end of the pipe is closed must
stop writing immediately and exit 0.

On Go those two cannot both be honoured naively: the runtime's default
disposition for SIGPIPE on fd 1 or 2 kills the process, which selects a status
outside the table — exactly what ExitPolicy forbids. `main.go` therefore
registers a notification channel for SIGPIPE (and nothing else), which makes the
write return `EPIPE` so the OutputPolicy rule can run. No signal changes control
flow, no other signal is touched, and no status outside the table is ever
selected. I read this as serving the obligation's intent, but it is literally a
caught signal, so it is named here.

### Narrower behaviours worth knowing about

- **Unreadable directory on the ascent.** `01-locate@RootDiscovery::ERROR`
  requires `yass.io.unreadable` for a directory on the ascent that cannot be
  read. Detected from the permission error raised while stat'ing that
  directory's `root.yass.yaml`. A directory that is unreadable but still
  searchable is not distinguished from a readable one — the ascent only ever
  needs to stat one name in it.
- **Second and later preamble-shaped documents.** A preamble-shaped document
  after the first is reported as `yass.document.unknown_kind` and is not
  additionally re-checked against the preamble rules (`incomplete`, `version`,
  `unknown_key`, `related_shape`), which are applied to the first document only.
  Re-checking it would stack two diagnostics on one document for what is one
  problem.
- **CRLF.** `../yass@Document::INPUT` MAY-allows CRLF input; YAML 1.2 treats it
  as equivalent to LF and go-yaml normalises it during parsing. Output is always
  LF, per `root@OutputPolicy`.
- **An argument that is not valid UTF-8.** `00-invocation@Invocation::INPUT`
  requires the vector be rejected rather than repaired, but names no code for
  it. Reported under that slot's guard-less residual,
  `yass.args.unknown_option`, with the usage status.

## Second pass — the orientation entry point

Added after the first build, to a single requirement: an agent that has the
binary and nothing else — no checkout, no `CLAUDE.md`, no `context/` — should be
able to run `yass`, learn what the project is and what yass is, and find the
language documents when, and only when, it needs them.

Two subcommands, `overview` and `docs`, and one change to invocation: an
argument vector naming no subcommand is now well formed and yields `overview`.
`yass.args.no_subcommand` is gone and `yass.docs.unknown` is new, so the closed
set is still exactly 55 codes.

### 1. The corpus is embedded, not read

`root@YassCli::SIDE-EFFECT` forbids reading any file that is not a
`root.yass.yaml` on the ascent, a path named on the command line, or a
`.yass.yaml` under the project root. Serving `context/GUIDANCE.md` off disk
would break that — and would fail anyway for the reader this is for, who has the
binary and not the checkout.

**Chosen:** `//go:embed`. `script/sync-docs` copies the three authoritative
language files into `cli/internal/yass/docs/` and `TestCorpusInSyncWithCheckout`
fails when a copy has drifted from its source. The corpus is beside the package
that embeds it rather than at `cli/docs/`, because an embed pattern cannot
reach outside its own package directory.

**One trap worth naming:** an embedded copy must not keep the `.yass.yaml`
suffix. Every file with that suffix under a project root is collected as part of
that project, so `cli/internal/yass/docs/language.yass.yaml` would be indexed by
`list`, checked by `validate`, and flagged by `lint` as a document unreachable
from the CLI's own root spec. The self-definition is carried as `language.yaml`.

### 2. Why `Brevity` was amended rather than worked around

`root@Brevity` forbids a banner and forbids a pointer to documentation, which is
most of what an orientation block is. Rather than smuggle the block past the
rule, the rule now names the two exempt commands and says why: the policy exists
to protect the reader's context, and a reader who cannot tell what a project is
opens files until it can — which costs more of that context than the block does.
The exemption is bounded to output that was asked for and whose wording is fixed
by a design block.

### 3. `overview` never selects the no-root status

`root@ExitPolicy` gives 4 to "no `root.yass.yaml` at or above the starting path",
and says statuses 2 through 5 leave standard output empty. But `overview`
answers perfectly well with no project root — it reports the absence as one line
of its block — so exiting 4 would contradict the empty-output rule.

**Chosen:** ExitPolicy now says 4 is selected only by a command that needs a root
in order to answer at all, which `overview` and `docs` do not. Reported as a
fact, exit 0.

### 4. An unknown document name is 3, and does not enumerate

`yass docs nope` exits 3 (unresolved), not 2: the request is well formed and
names something absent, which is what 3 is for.

The diagnostic does *not* list the recognized names. `Brevity`'s one enumeration
carve-out is justified by the reader being unable to recover otherwise, and here
it can: `yass docs` with no operand *is* the index. So the strict rule applies.

### 5. What the block carries, and what it deliberately does not

The notation summary in section 3 is scoped to what a reader needs in order to
*read* a spec file — the five slots, the obligation shape, the three relations,
the target grammar. What an author needs in order to *write* one stays in the
corpus, behind an explicit `yass docs` call.

That split is the whole point, and it is stated normatively in `10-docs@Corpus`
rather than left to taste: an agent implementing existing specs pays nothing for
guidance it does not need, and an agent changing the specs asks for it by name.
The index carries a WHEN TO READ field so the decision can be made without
fetching anything.

### 6. Description wrapping

The root preamble description is the single highest-value line in the block, so
it gets more room than `list` gives it: collapsed to one line, truncated at 240
Unicode scalar values rather than 100, then broken at spaces and indented to
align under itself. A single word longer than the line budget still overflows —
the block is written *within* 88 columns rather than guaranteed under it, and
`11-overview@OrientationBlock` says so rather than claiming a bound the wrapper
does not enforce.

## What was checked

`script/test` runs `gofmt -l`, `go vet`, and `go test ./...`. The test suite
(`cli/internal/yass/yass_test.go`, 79 test functions) builds
throwaway spec trees and drives `Main` end to end: every argument-vector
failure, nested project roots, symlink policy, the file-set ordering, Norway
words, one-level resolution, guard conjunction, insertion non-deduplication,
design append deduplication, truncation at exactly 100 Unicode scalar values,
simple-versus-full case folding, `*` crossing `/`, record field counts and LF
termination, and byte-identical output across repeated runs.

The program defines exactly 55 diagnostic codes, matching the closed set the
specs enumerate. 54 of them are exercised by a test. The 55th is `yass.internal`,
the environment-status residual every command carries for a failure no guarded
obligation matched; it is unreachable by construction, since each guarantee it
guards is established earlier in the same invocation, and I could not build a
fixture that reaches it without deliberately corrupting internal state.

Beyond the fixtures, both real projects in this repository are clean:

    cd cli && ./yass validate && ./yass lint          # exit 0, no output
    cd ..  && cli/yass validate && cli/yass lint      # exit 0, no output

and `cli/yass query` output for the whole CLI spec set parses back as a YAML
stream.

The second pass adds 15 test functions covering both new commands: the bare and
named forms of `overview` agreeing byte for byte, the project counts, the
absent-root line, an unparsed file counted but not diagnosed, a root with no
description, the block's column budget, operand and option rejection, the corpus
index shape, each document served byte-for-byte, exact-match-only name
resolution, service with no project root, and the drift guard against the
checkout. Every fixed line of `11-overview@OrientationBlock` was also checked
against the emitted block: all 29 sample lines in the design block appear in the
output verbatim.
