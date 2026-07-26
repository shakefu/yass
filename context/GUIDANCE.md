# yass — Authoring Guidance (holding doc)

Steerage for AI agents (and humans) writing yass specs. This is a **holding doc**: the
rules here emerge from trial-and-error using the specs, and their final home is TBD —
some may become language-level meta-rules, others may stay as guidance. Nothing here is
settled; it exists so the guidance is not lost while we figure out where it belongs.

## Guiding principle

A specification must be fully intelligible to a reader — human or model — who has never
seen this project, with no prior prompting and no other documents open. Tooling can make
working with specs faster, but a spec must never *require* tooling or a companion document
to be understood or implemented.

## Granularity: keep specs small and located

The core failure mode: an LLM, unguided, will slop a single giant spec file for a whole
codebase. To prevent that:

- **One spec file per code file.** A `.yass.yaml` is paired 1:1 with the code file it
  describes.
- **One spec definition per public symbol or endpoint.** Each public function, method,
  class, or endpoint gets its own `spec:` document.

Candidate phrasing as a *future* meta-rule (deliberately NOT in the self-defining
`yass.yass.yaml` — the generalized language definition is not the place for code-authoring
steerage):

    WHEN: the spec defines code behavior
    MUST: maintain one spec file per code file, and one spec definition per public symbol

## The root file: say what the project is, first

`root.yass.yaml` is required, and it is the **first file a reader opens** — the entry
point into the spec set, the same way a `main` is the entry point into a program. Write it
so a reader (or an implementing agent) learns **what the project is** before reading
anything else: what it is called, how it is used, what it does with its input, and what it
leaves behind. A root file that reads as a list of pointers has failed; the reader should
be able to stop after it and still describe the project.

It carries **exactly one spec** for the project, and that spec is an ordinary one: there is
no special document type, and its five slots carry exactly the meaning they carry in any
other spec. The only thing that changes is the subject — the project rather than a
function — and the reference's *Slots* section already says how to read them.

**Dispatch is a guarded `SEE`.** One guarded obligation per recognized subcommand, naming
the spec that owns it: the guard is the subcommand value, the prose says it dispatches, and
`SEE` names where that behavior is defined. This is the decision procedure in the reference
(*Choosing a relation*) applied to a dispatcher — the handler's rules do not bind the
dispatcher, so `CONFORMS` would be wrong, and dispatch is exactly the "genuine dependency
stated in the obligation's own prose" case that `SEE` is for.

**Membership is reachability, so the root file is a completeness check.** Every spec and
design under the project root must be reachable from the root spec through a chain of
references (`CONFORMS`, `USES`, and `SEE` alike). Use it as a lint you can run by eye: a
spec that no chain of references reaches is either **dead** — nothing in the project
invokes it — or the root file is **missing an obligation**, most often an unlisted
subcommand or an entry point nobody declared. Decide which; never leave it unreached.

**A program-wide constraint that belongs to no single spec is a design block.** Wire
format, log line shape, exit-code policy, a startup sequence — write it once as a
`design:` document and bind it with a **reference-only `USES`** in the root spec's
`INVARIANT` (`- USES: ExitCodes`). That gives the constraint one home, normative force,
and a reachability edge, without inventing a prose slot the language does not have.

A worked root spec for `ledger`, a command-line project with three subcommands and an
exit-code policy factored out as a design block:

```yaml
---
description: >
  ledger — the project as a whole. Invocation contract, subcommand dispatch, and
  program-wide rules. Each subcommand's behavior lives in its own spec file.
version: v1
---
spec: Ledger
INPUT:
- MUST: "take the subcommand as `argv[1]`, one of `add`, `list`, or `total`"
- MUST: "treat every remaining argument as an operand of that subcommand"
- WHEN: "`argv[1]` is absent or outside that set"
  MUST: "be rejected per `ERROR`, dispatching no subcommand"
- WHEN: "the subcommand is `add`"
  MUST: read the entries to record from stdin, one per line
RETURN:
- WHEN: "the subcommand is `add`"
  MUST: dispatch to the entry-recording spec and write its output unchanged to stdout
  SEE: cmd/add@Add
- WHEN: "the subcommand is `list`"
  MUST: dispatch to the listing spec and write its output unchanged to stdout
  SEE: cmd/list@List
- WHEN: "the subcommand is `total`"
  MUST: dispatch to the summing spec and write its output unchanged to stdout
  SEE: cmd/total@Total
- MUST-NOT: write anything to stdout that the dispatched spec does not state
ERROR:
- WHEN: "`argv[1]` is absent"
  MUST: "write the one-line usage synopsis to stderr and exit `2`"
- WHEN: "`argv[1]` is outside the recognized set"
  MUST: "write `unknown subcommand` with the offending value to stderr and exit `2`"
- MUST: "write one diagnostic line to stderr and exit `1` for any other failure"
SIDE-EFFECT:
- MUST: "read and write the ledger file named by `LEDGER_FILE`, and no other path"
- MUST-NOT: open a network connection
INVARIANT:
- USES: ExitCodes
---
design: ExitCodes
type: constraint
content: |
  0 — the subcommand completed and its output is complete.
  1 — a failure attributable to the ledger file or its contents.
  2 — a failure attributable to the invocation (arguments, usage).
  No other status is emitted by any subcommand.
```

Note the reference forms: `cmd/add@Add` has no leading dot, so it resolves **from the
project root** — which is the directory holding this file. That is the characteristic
shape of a root-file reference, and the reason the root file is the one place where
root-relative paths read naturally.

## Deliberate non-goal: no free-prose channel

yass intentionally provides **no formal way to attach non-spec prose or commentary** to
a spec. Authors get structured obligations only; the sole free-text field is the
preamble `description`.

Rationale: when a language allows free prose (as Allium does), the prose goes out of
control — specs bloat with narrative that isn't behavior. Withholding a prose/comment
channel forces authors (especially LLMs) to express intent as obligations, or not at
all.

Note: the only comments yass emits are tooling-generated provenance comments
(`# CONFORMS: ...`) on resolved fragments. Those are not an author-facing channel, and
should not become one.

One fenced carve-out: a **design block** (a `design:` document — see *Design document*
in the reference) carries normative freeform content, but it is not a prose channel on
specs. The prose is fenced inside a declared block with a name, a required `type`, and
normative force through the reference graph (`USES`); it cannot leak into a `spec:`
document, and an unreferenced design block is dead weight that lint flags — declared
normative content nothing binds to. The forcing function stays intact for spec
documents: express intent as obligations, or not at all. The preamble `description`
remains the only free-text field outside a design block.

## Ordering: implementation sequence

Spec document fragments (`spec:` documents) within a file SHOULD be ordered in the
sequence they should be implemented. This gives an implementing agent (or human) a
natural top-to-bottom work order and makes dependency ordering explicit without extra
metadata.

Two levels of ordering are at play:

- **Inter-file ordering** — across spec files. Enforceable today with numeric-prefix
  naming conventions (e.g. `00-init.yass.yaml`, `01-core.yass.yaml`,
  `02-api.yass.yaml`).
- **Intra-file ordering** — within a single spec file. The YAML document stream is
  ordered; spec documents earlier in the file should be implemented before later ones.

Candidate phrasing (not yet a meta-rule):

    WHEN: the spec defines code behavior
    SHOULD: order spec documents in implementation sequence, both across files
            (via naming convention) and within files (via document position)

Note: this is emergent guidance from early usage — not yet formalized. A second pass on
spec revisions may promote it to a meta-rule or refine the conventions. Captured here so
it isn't lost.

## Error obligations: guarded for the foreseeable, guard-less for the residual

In the `ERROR` slot, a guarded obligation (one with a `WHEN`) states a specific,
foreseeable failure; a guard-less obligation is the **residual** — the policy for any
failure not matched by a guarded obligation in the same slot. (This reading is fixed in
`yass.yass.yaml` under `Slot.ERROR`.)

Two rules follow:

- **Always state a residual.** If a spec rejects anything, it MUST also say what happens
  to failures it did not enumerate. Without a guard-less catch-all, each implementer
  invents their own handling and the results diverge.
- **Never fold a foreseeable case into the residual.** Any failure you can name in
  advance deserves its own guarded obligation, and if it has a distinct observable
  outcome (a specific error code, message, or exit status) that outcome MUST be stated on
  that obligation — not left for the reader to infer from the catch-all. A reader (or
  model) must not have to deduce a domain rule from "everything else."
- **Do not state a residual the guards have already exhausted.** A residual only has
  meaning when the guarded obligations leave some input unmatched. When the enumerated
  guards partition the input completely — every unit is either rejected by a named guard
  or accepted as well-formed, with no remainder — there is no residual set, and a
  guard-less catch-all is *dead*: it can never fire. A dead residual asserted alongside an
  exhaustive partition is a contradiction every careful reader flags. (Observed directly:
  all four panel models independently rejected a guard-less `E90` error obligation as
  unreachable, several noting it would violate the spec's own well-formed/malformed
  invariant.) Before adding a residual, confirm the guarded cases leave a genuine
  remainder; when they are exhaustive, state that exhaustiveness rather than a residual
  that cannot be reached. `Slot.ERROR` makes this binding: a guard-less residual MUST-NOT
  be carried when the guarded obligations already account for every input.

## Input segmentation: specify every boundary

When a spec defines how input is broken into units — records, lines, fields, tokens — it
MUST state the boundary behavior completely, not just the happy path. For each level of
segmentation, name:

- the exact separator and its character class (e.g. *one ASCII space `0x20`*, not the
  vaguer "whitespace", which invites splitting on tabs and other Unicode spaces);
- empty input (zero units);
- an empty or blank unit in the interior;
- a leading or repeated separator;
- the mechanics of an *optional trailing terminator* — say how many trailing separators are
  absorbed (characteristically exactly one), and therefore what a second trailing separator,
  or a blank final unit, denotes. "MAY accept a trailing newline" without the count leaves
  an implementer to guess whether one or all are stripped;
- the degenerate input that is *only* separators — a lone separator with no content, or a
  run of them — which the cases above otherwise leave under-determined (is a lone separator
  empty input, or one empty unit?).

Each of these is a case an implementer will hit and otherwise resolve by guessing. State
the intended outcome as an obligation, or declare it out of scope — do not leave it to
emerge.

## Closed-set dispatch: state the out-of-set case

The residual rule for the `ERROR` slot (above) is one instance of a more general
discipline: **whenever a spec branches on a closed set of values, it MUST state what
happens for a value outside that set, or a value that is missing.** This applies to the
`INPUT` slot too — most often when an `INPUT` dispatches on a subcommand, a mode, or an
enum and routes each recognized value to a different behavior.

If the enumerated set is `{tally, grade, pack}`, the spec must also say what the program
does when invoked with `harvest`, or with no subcommand at all. Without that obligation
each implementer invents the unknown-input handling and the results diverge — observed in
practice as different exit codes and different diagnostics for the same out-of-set input.
A reader must never have to deduce the residual case from the enumerated ones.

## Composition: dataflow and cross-cutting concerns across specs

Specs describe components one at a time, but real programs are wired together. Two gaps
recur when a reader has only the specs and no architecture note, and both are closed by
obligations, not prose:

- **Name the dataflow and the trust boundary.** When one spec's `INPUT` consumes the data
  another spec's `RETURN` produces (a pipeline stage, a handler reading a producer's
  output), point at the producer with a slot-targeted reference —
  `CONFORMS <producer>::RETURN`. That pointer is not decorative: it means *the data this
  input consumes must match exactly what that slot produces*, so the producer's `RETURN`
  guarantees characterize the data crossing the boundary — and inlining puts them in
  front of the consumer's `INPUT`. Having named it, **state explicitly which of
  those upstream guarantees the consuming spec relies on (and therefore does NOT
  re-validate) and which it re-checks.** A consumer that silently re-validates, or silently
  trusts, forces every implementer to guess the boundary; they will guess differently. The
  consuming spec owns that decision — make it in an obligation. **And for each guarantee it
  relies on without re-validating, state what it does if that guarantee is violated** — even
  if only to declare the behavior unspecified. Stating the trust without the violation
  residual is the same omission as a rejection with no catch-all: observed in practice as
  three of four models silently inventing how a consumer counts and classifies an
  out-of-contract input line. The residual principle — every delegated check needs its
  failure case pinned — applies to a trust boundary exactly as it does to the `ERROR` slot
  and to closed-set dispatch.

- **Give every cross-cutting concern a single home.** When a rule spans many specs — a wire
  format, the shape of an error line, how input is segmented, how a subcommand is
  dispatched — write it once in one spec that owns it completely, and reference that spec
  from the others. Do not restate the rule in fragments across the specs it touches. A
  reader should learn the whole of a concern from one place rather than reconstructing it
  from scattered, drift-prone mentions.

- **"That spec owns rules I must obey" is whole-spec `CONFORMS`, never `SEE`.** This is the
  commonest whole-spec reference there is — a shared wire-protocol spec owning line
  segmentation, the error-line format, and exit policy for three stage specs that must all
  obey it. Point at it with whole-spec `CONFORMS`, which means *must match the referenced
  spec*. `SEE` is the wrong key: it names a spec for the reader and imposes nothing, so an
  author who reaches for it here silently drops a real constraint and every implementer is
  free to ignore the owning spec. Reserve `SEE` for pointers that bind nothing — a dispatch
  target, an ordering note, related reading.

## Open: how a skill uses the test taxonomy

Moved here from TEST-TAXONOMY.md — depends on tooling (CLI commands, obligation-JSON
projection) that is not yet spec'd. Revisit once the CLI is scoped.

1. Retrieve the spec (with `CONFORMS` inlined and provenance attached) via whatever
   tooling exists.
2. For each slot, for each obligation, look up its row in the test taxonomy (by slot +
   normativity + presence of `WHEN`/relation) to get the fixed test obligation(s).
3. Emit the obligation list (this is the deterministic artifact) before writing any
   code, so it can be reviewed against the spec.
4. Locate the implementing symbol in the paired code file (the meta-rule: every
   code file is paired with a `.yass.yaml`); reuse existing test infrastructure.
5. Write tests in the target framework, one mapped to each obligation, labeled with
   the originating `spec`/`slot`/obligation so failures point back to the spec.
6. Report coverage per the model above.

## Open: emergent guidance

As the specs get used, more steerage will surface (placeholder conventions, naming,
when to split a spec, etc.). Collect it here until it earns a permanent home — either
as a language meta-rule or as stable authoring documentation.
