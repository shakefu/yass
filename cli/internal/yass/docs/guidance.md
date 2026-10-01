# yass — Authoring Guidance

Steerage for AI agents (and humans) writing yass specs.

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

**A program-wide constraint that belongs to no single spec is a design block.** Wire
format, log line shape, exit-code policy, a startup sequence — write it once as a
`design:` document and bind it with a **reference-only `USES`** in the root spec's
`INVARIANT`. That gives the constraint one home and normative force, without inventing a
prose slot the language does not have.

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
  consuming spec owns that decision — make it in an obligation. The violation side of
  that reliance — what the consumer does when a trusted guarantee does not hold — is a
  coverage question: see *Case coverage*, *Trusted inputs*.

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

## Case coverage: enumerate, then account for the remainder

Before considering a draft complete, identify every place it selects behavior,
divides input into cases, names failures, or relies on another component's
guarantees. Then work through each applicable check below, over the whole spec
and its binding references — a single obligation need not carry the entire
decision. `Slot.INPUT` and `Slot.ERROR` make several of these binding; the
checks are the discipline those rules generalize.

1. **Values that select behavior** (usually `INPUT`). For each command, mode,
   enum, or other closed set the spec branches on, account for every recognized
   value, an unrecognized value, and absence: if the enumerated set is
   `{tally, grade, pack}`, say what the program does with `harvest`, and with
   no command at all. Otherwise each implementer invents the out-of-set
   handling — observed in practice as different exit codes and different
   diagnostics for the same input. A reader must never have to deduce the
   residual case from the enumerated ones.

2. **Conditional behavior** (any slot). For each `WHEN` guard, determine what
   constrains behavior when the condition is false. A guard states a
   *sufficient* condition only — "when exactly one item matches, emit a
   fragment" does not itself forbid emitting a fragment otherwise — so an
   existing baseline or another obligation may already cover the false case,
   and that is fine. But if the intent is "only when," say so with obligations:
   a complement-guarded `MUST-NOT`, or an unguarded baseline the guarded
   obligation overrides. Do not encode necessity in the guard's prose — `WHEN`
   cannot carry it. Where guards overlap, make the applicable obligations
   compatible, or state the precedence inside one obligation's prose: a slot's
   obligation order carries no meaning, so list position settles no conflict.

3. **Segmented input** (`INPUT`). When a spec defines how input is broken into
   units — records, lines, fields, tokens — state the boundary behavior
   completely, not just the happy path. For each level of segmentation, name:

   - the exact separator and its character class (e.g. *one ASCII space `0x20`*,
     not the vaguer "whitespace", which invites splitting on tabs and other
     Unicode spaces);
   - empty input (zero units);
   - an empty or blank unit in the interior;
   - a leading or repeated separator;
   - the mechanics of an *optional trailing terminator* — say how many trailing
     separators are absorbed (characteristically exactly one), and therefore
     what a second trailing separator, or a blank final unit, denotes. "MAY
     accept a trailing newline" without the count leaves an implementer to
     guess whether one or all are stripped;
   - the degenerate input that is *only* separators — a lone separator with no
     content, or a run of them — which the cases above otherwise leave
     under-determined (is a lone separator empty input, or one empty unit?).

   Each of these is a case an implementer will hit and otherwise resolve by
   guessing. State the intended outcome as an obligation, or declare it out of
   scope — do not leave it to emerge.

4. **Failures** (`ERROR`). Name each foreseeable failure as a guarded
   obligation carrying its observable outcome (a specific error code, message,
   or exit status); a reader must not have to deduce a domain rule from
   "everything else." Then check whether any failures remain uncovered. If they
   do, state the residual policy in a guard-less obligation: in `ERROR`, an
   obligation with no `WHEN` is the policy for any failure no guarded
   obligation matched (the reading is fixed in `yass.yass.yaml` under
   `Slot.ERROR`). If the guarded cases are instead exhaustive — every input
   rejected by a named guard or accepted as well-formed — state that
   exhaustiveness and omit the residual: a residual over an empty remainder is
   dead and can never fire, and asserting one alongside an exhaustive partition
   is a contradiction careful readers flag (all four panel models once
   independently rejected a guard-less error obligation as unreachable, several
   citing the spec's own well-formed/malformed invariant).

5. **Trusted inputs** (`INPUT`). For each guarantee from a producing spec that
   the consumer relies on without re-validating — *Composition* covers naming
   the producer and the trust boundary — state what the consumer does if that
   guarantee is violated, even if only to declare the behavior unspecified.
   Pointing at the producer's contract does not discharge this: the contract
   says what should arrive, not what the consumer does when it doesn't
   (observed: three of four models silently invented how a consumer counts and
   classifies an out-of-contract input line).

The finishing question: **can a reader locate the contractual disposition of
every case this pass identified?** Defined behavior, an explicitly permitted
choice, and a stated "unspecified" all count; silence does not.

## Open: emergent guidance

As the specs get used, more steerage will surface (placeholder conventions, naming,
when to split a spec, etc.). Collect it here until it earns a permanent home — either
as a language meta-rule or as stable authoring documentation.
