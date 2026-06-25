# yass — Authoring Guidance (holding doc)

Steerage for AI agents (and humans) writing yass specs. This is a **holding doc**: the
rules here emerge from trial-and-error using the specs, and their final home is TBD —
some may become language-level meta-rules, others may stay as guidance. Nothing here is
settled; it exists so the guidance is not lost while we figure out where it belongs.

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
  `USES <producer>::RETURN`. That pointer is not decorative: it means *the data this input
  consumes is exactly what that slot produces*, so the producer's `RETURN` guarantees
  characterize the data crossing the boundary. Having named it, **state explicitly which of
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
  (`USES`/`CONFORMS`) from the others. Do not restate the rule in fragments across the
  specs it touches. A reader should learn the whole of a concern from one place rather than
  reconstructing it from scattered, drift-prone mentions.

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
