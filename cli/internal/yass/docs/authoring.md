# yass — Authoring

*Write path, 1 of 5: **authoring** → documents → obligations → references → coverage. Run each as `yass docs NAME` and read it whole.*

Steerage for AI agents (and humans) writing yass specs: what a spec set must be, how to lay it out, and how specs compose.

## Guiding principle

A specification must be fully intelligible to a reader — human or model — who has never
seen this project, with no prior prompting and no other documents open. Tooling can make
working with specs faster, but a spec must never *require* tooling or a companion document
to be understood or implemented.

## No open questions: a spec holds only decisions

**A spec file MUST NOT carry an open question — not in an obligation, a design, the
preamble, or a comment.** A spec is a contract: its implementer builds what it says, and
an undecided point becomes a guess nobody approved. That covers a question, a `TBD`,
`TODO`, or `FIXME` placeholder, an "open questions" list, and a choice handed to the
implementer ("pick a sensible default", "to be decided").

- Resolve each ambiguity with the user before writing the obligation it touches.
- When no one can answer, stop and report the question to the user. Never write it
  into the spec to settle later.
- `yass lint` reports the markers it can see as `yass.lint.open_question`. A clean lint
  does not prove a set is decided; the reader still has to look.

## Structure, not prose: keywords belong in keys

An agent reads a spec's structure first. A condition, requirement, or reference written
into prose is structure the reader may miss.

- A condition on when an obligation applies is a `WHEN` guard:
  `- WHEN: the queue is full` then `MUST: reject the item`, never
  `- MUST: reject the item, when the queue is full`. A clause that narrows *what* the
  obligation requires ("accept the flag even when it begins with `-`") stays in prose.
- One normativity keyword per obligation: two requirements are two obligations.
- A reference is a relation key (`USES: Policy`), never prose naming a target.
- A keyword mentioned rather than used goes in backticks (`` `WHEN` ``).
- `yass lint` reports the forms it can see as `yass.lint.prose_keyword`.
- An obligation is one requirement, never a procedure, code, or another spec restated:
  *Obligation shape* in `yass docs obligations` holds the absolute rules.

## Granularity: keep specs small and located

The core failure mode: an LLM, unguided, will slop a single giant spec file for a whole
codebase. To prevent that:

- **One spec file per unit of specification.** A `.yass.yaml` is paired with the code it
  describes at the project's chosen unit — a command, module, package, or similar cohesive
  component — not necessarily one file for one source file. A spec file may hold several
  `spec:` documents when they share that unit.
- **One spec definition per public symbol or endpoint.** Each public function, method,
  class, or endpoint gets its own `spec:` document.

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
function — and *Slots* in `yass docs obligations` already says how to read them.

**Dispatch is a guarded `SEE`.** One guarded obligation per recognized subcommand, naming
the spec that owns it: the guard is the subcommand value, the prose says it dispatches, and
`SEE` names where that behavior is defined. This is the decision procedure in
`yass docs references` (*Choosing a relation*) applied to a dispatcher — the handler's
rules do not bind the dispatcher, so `CONFORMS` would be wrong, and dispatch is exactly the "genuine dependency
stated in the obligation's own prose" case that `SEE` is for.

**A program-wide constraint that belongs to no single spec is a design block.** Wire
format, log line shape, exit-code policy, a startup sequence — write it once as a
`design:` document and bind it with a **reference-only `USES`** in the root spec's
`INVARIANT`. That gives the constraint one home and normative force, without inventing a
prose slot the language does not have.

## Deliberate non-goal: no free-prose channel

yass intentionally provides **no formal way to attach non-spec prose or commentary** to
a spec. Authors get structured obligations only. Free text does appear in a few declared
fields — the preamble `description`, the preamble `related` list of external resource
locations, and a design block's `type` hint — but each is structural metadata, not a
commentary channel.

Rationale: when a language allows free prose (as Allium does), the prose goes out of
control — specs bloat with narrative that isn't behavior. Withholding a prose/comment
channel forces authors (especially LLMs) to express intent as obligations, or not at
all.

Note: the only comments yass emits are tooling-generated provenance comments
(`# CONFORMS: ...`) on resolved fragments. Those are not an author-facing channel, and
should not become one.

One fenced carve-out: a **design block** (a `design:` document — see *Design document*
in `yass docs documents`) carries normative content, but it is not a prose channel on
specs. The prose is fenced inside a declared block with a name, a required `type`, and
normative force through the reference graph (`USES`); it cannot leak into a `spec:`
document, and an unreferenced design block is dead weight that lint flags — declared
normative content nothing binds to. The forcing function stays intact for spec
documents: express intent as obligations, or not at all. The preamble `description`
and `related` fields remain the only free-text fields outside a design block.

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
  coverage question: see *Trusted inputs* in `yass docs coverage`.

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

---

Next in the write path: `yass docs documents`.
