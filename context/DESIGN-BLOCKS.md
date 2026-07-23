# Design Blocks — a second document type

Status: **drafted into the language on `goal-experiment`** (PR #16, merged
2026-07-22) as an evaluation draft — a new document type (`design:`) carrying
named, typed, freeform normative content — algorithms, technology constraints,
multi-spec lifecycle procedures — and a repurposing of the `USES` relation to
target design blocks exclusively. Experiment rounds 06–08 found no one-shot
correctness, clarity, or compliance differential (see `experiment/FINDINGS.md`,
`design-blocks-proposal`); the merge is a preference-based draft adoption for
manual/authoring evaluation, not a validated necessity. yass is pre-release
(v0.0.x); this remains in-development shaping of what v1 *says*, not a
migration. No CLI implements the resolution semantics yet.

## Motivation

Specs occasionally need a normative home for content that is not an obligation on
observable behavior:

1. **Algorithms / pseudocode.** A spec that must pin a particular computation
   (a hash construction, a precedence-resolution procedure) has nowhere to state
   it except prose packed into obligations. SPEC-FEEDBACK.md's Rust agent asked
   for exactly this — "a PROCEDURE or ALGORITHM slot type that explicitly allows
   ordered steps" — because procedural specs "fight the syntax: ordering goes in
   INVARIANT, accumulation goes in SIDE-EFFECT, and the control flow is
   invisible."
2. **Technology / stack constraints from outside forces.** "The service is
   already on Postgres", "compliance requires at-rest encryption via KMS", "the
   team wants Go". These are real, binding constraints on an implementation, but
   they are not INVARIANTs of observable behavior — a stack choice is not a
   constraint that "always holds, independent of any call". Forcing them into a
   slot distorts the slot's meaning.
3. **Coarse lifecycle / operational procedures spanning multiple specs.** "Warm
   the cache → connect the database → mark the service live" orders work across
   several specs' subjects and has **no owning symbol** — no single spec whose
   slots it belongs in. The one-spec-per-symbol granularity rule (GUIDANCE,
   yass-reference *Spec document*) gives it no home at all.

This maps to the open `slot-model-for-non-functional-specs` finding in
`experiment/FINDINGS.md` (the PROCEDURE/ALGORITHM slot request) — but resolves it
as a **new document type**, not a new slot. A slot is a facet of one spec's
contract; the content above is either not a contract facet (tech constraints) or
not one spec's (lifecycle procedures).

### Why the refuted sequencing rounds do not cover this

Rounds 02 and 05 refuted `cross-spec-sequencing` (REQUIRES/AFTER),
`priority-chains-prose`, and `dispatch-subcommand-override`: per-call and
fine-grained sequencing — a dataflow header gate, a per-identifier stateful
precondition, an 18-entry precedence chain, a scoped override — is expressible
today as normative prose obligations inside the spec that owns the behavior, and
4/4 models read it correctly cold. All of those cases had an owning spec whose
slots could carry the prose. The **multi-spec lifecycle procedure** — ordering
that belongs to no symbol — is the case those rounds never tested. This proposal
does not reopen the refuted findings; it addresses the residue they left
untouched.

### Why tech constraints do not reverse the language-agnostic non-goal

The recorded decision (yass-reference *Notes*: "Verification is out of scope —
tooling routes/retrieves, never verifies obligation content (keeps it
language-agnostic). A `COMPATIBLE` relation was deliberately excluded.") was
about **tooling verification** — yass tooling never checks whether an
implementation is in Go or talks to Postgres. That stands. Authors can already
state tech constraints as prose today; what is missing is a home that does not
distort slot semantics. A design block is author-stated, tooling-opaque content —
the tooling still routes and retrieves, never verifies.

### A side benefit: the USES ambiguity gets resolved

`USES` today is the most-used relation (MAN-ALIGNMENT.md counts 57 uses) and the
only one with no external vocabulary anchor — and it is overloaded. It reads as
"calls", "depends on the output of", "runs after", and (in the self-definition)
"structurally contains". Round-05 closed the overload as "ergonomic/lint, not a
one-shot-failure defect", but the ambiguity is recorded. The repurposing below
gives **each relation exactly one target kind and one resolution rule**, making
the CONFORMS/USES distinction machine-checkable instead of a judgment call.

## The design: a `design:` document type

A new document type, a peer of `spec:` in the YAML stream:

```yaml
---
design: StartupSequence     # new document type, peer of `spec:`
type: ordered-steps         # required; brief freeform interpretation hint
content: |
  1. Warm the cache
  2. Connect the database
  3. Mark the service live
```

- **`design:`** names the block. Names share the **file-wide uniqueness
  namespace** with `spec:` names, so a RefTarget stays unambiguous — a bare name
  or `path@Name` resolves to exactly one document, spec or design.
- **`type`** is **required**: a brief freeform interpretation hint, one or two
  words (`pseudocode`, `ordered-steps`, `constraint`, `stack`, …), **opaque to
  tooling**. Deliberately NOT a closed set — a closed vocabulary would trigger
  the closed-set-dispatch residual discipline (GUIDANCE, *Closed-set dispatch*;
  `Slot.INPUT`) for no benefit: tooling never branches on the value, so there is
  no out-of-set case to pin.
- **`content`** is a **required block scalar**. The body is **opaque**: no refs
  are recognized inside it, and a design document carries **no slots**.
- **Normative force:** an implementation bound to a design block via reference
  MUST follow it. The block is not commentary; it binds through the reference
  graph.

## Repurposed `USES`

`USES` is redefined to target design blocks **only**. `CONFORMS` targets specs
and slots **only**. `SEE` targets either and stays a pure pointer.

| Relation   | Target          | Resolution                                  |
|------------|-----------------|---------------------------------------------|
| `CONFORMS` | spec or slot    | must match; slot-targeted inlines           |
| `USES`     | design block    | inlined with provenance                     |
| `SEE`      | spec or design  | pure pointer, never inlined                 |

- A relation/target-kind mismatch — `USES` → spec, `CONFORMS` → design — is a
  **validation error**. The CONFORMS-vs-USES distinction, today a discriminator
  the author applies by judgment ("must match" vs "draws on"), becomes
  machine-checkable.
- **USES resolution:** the yass CLI inlines it like `CONFORMS`, but the
  mechanics differ — a design block has no obligations to splice into a slot.
  Resolution **appends the typed content block to the emitted fragment**, once,
  with a provenance comment (`# USES: StartupSequence`), deduplicated when the
  same block is referenced from multiple obligations.
- **`::SLOT` on a design target is a validation error** — design blocks have no
  slots, so there is nothing finer to address.
- **Ref-only obligations:** a bare `- USES: Name` list item is valid — no guard
  or normativity keyword required, consistent with the existing ref-only rule.
  Existing guard rules are unchanged: if `WHEN` is present it must be
  accompanied by a normativity keyword; when a guarded obligation carries a
  `USES`, the guard scopes **when the design binds**.
- **Ref-only resolution changes:** today only a slot-targeted `CONFORMS`
  transcludes from a ref-only obligation (yass-reference *Obligation*). Under
  this proposal a ref-only `USES` also transcludes — it appends its design
  block's content, per the mechanics above.

## Migration of existing spec→spec `USES`

Every current `USES` targets a spec or slot, so all of them move:

1. **The dataflow/trust-boundary reading** — `Slot.INPUT`'s
   `USES <producer>::RETURN`, the resolved `dataflow-invisible` finding —
   migrates to slot-targeted **`CONFORMS: <producer>::RETURN`**. Semantically
   this is "the consumer's input must match what the producer returns", which is
   what the dataflow reading always meant. CONFORMS inlining then makes the
   producer's guarantees **visible in the consumer's INPUT** — the original
   round-02 complaint was precisely that the `USES` pointer "carried only
   structural meaning" and the data contract across the boundary was invisible.
   The trust-boundary obligations (state what is relied on vs re-checked, and
   the violation residual — `trust-boundary-violation-residual`) are unchanged;
   they are prose obligations in the consumer and do not depend on which
   relation carries the pointer.
2. **Structural navigation links in the self-definition** — `Document USES
   Preamble`, `Slot USES Obligation`, `Spec USES Slot`, and the rest of the
   "this construct contains that construct" links — migrate to **`SEE`** or are
   dropped. They were never dataflow and never "draws on behavior"; they are
   related-context pointers, which is exactly what `SEE` means.

## Relationship to the no-free-prose non-goal

This is the direct collision point: a design block is a **normative freeform
body**, and GUIDANCE's *Deliberate non-goal* withholds exactly that channel
because "when a language allows free prose, the prose goes out of control —
specs bloat with narrative that isn't behavior."

The argument that this survives the non-goal:

- The prose is **fenced inside a declared block** with a name, a required
  `type`, and normative force through the reference graph — it is not
  commentary attached to obligations, and it cannot leak into a spec document.
- An **unreferenced design block is dead weight**, flagged by lint — analogous
  to a dead residual (`residual-reachability`): declared normative content
  nothing binds to is a contradiction a careful reader flags.
- The non-goal's forcing function — express intent as obligations, or not at
  all — **stays intact for spec documents**. Nothing here adds a prose channel
  to a `spec:` document; the preamble `description` remains the file's only
  free-text field outside a design block.

## Costs / blast radius

This is not a small edit. Rewrite sites:

- `yass.yass.yaml` — the `Reference` and `RefTarget` specs (target-kind rules,
  resolution, the `::SLOT`-on-design error), `Slot.INPUT` (the dataflow
  obligation moves from `USES` to `CONFORMS` phrasing), the self-definition's
  own ~20 structural `USES` links (→ `SEE` or dropped), plus a **new
  Design/DesignBlock spec** defining the document type's well-formedness.
- `yass.v1.schema.json` — the `design:` document shape (`design`/`type`/
  `content`), and whatever relation-key constraints the schema can express.
- `context/GUIDANCE.md` — the *Composition* section (dataflow pointer becomes
  `CONFORMS`), and the no-free-prose section gains the fenced-block carve-out.
- `context/yass-reference.md` — the *References* section (relations table,
  dataflow reading, ref-only rules) and a new *Design document* section.
- The `dataflow-invisible` finding was **validated with `USES`** (rounds 02–03,
  4/4); the CONFORMS-based dataflow reading is semantically equivalent but
  **experimentally untested** — it needs a probe before this lands (below).

## Validation plan

Per the round discipline in `experiment/FINDINGS.md`, an experiment probe
before commit:

1. **Re-validate the dataflow reading as slot-targeted `CONFORMS`.** The
   round-02/03 result (producer pointer + stated trust boundary + violation
   residual, 4/4) must reproduce with `CONFORMS: <producer>::RETURN` carrying
   the pointer and its inlined guarantees in the consumer's INPUT.
2. **Probe the lifecycle case both ways.** A multi-spec lifecycle-ordering case
   (the startup-sequence shape): does it one-shot **with** a design block, and
   does it fail **without** one? The second arm is the one that matters — it
   establishes whether the construct answers a correctness need or only an
   ergonomic one, which is the distinction that closed five prior findings as
   `wontfix`.

## Open questions

1. **Placement of appended design fragments** in emitted output — after the
   queried spec's slots, grouped at the end, or adjacent to the referencing
   obligation?
2. Does **`SEE` → design** need distinct provenance from `SEE` → spec, or is
   the pointer form identical?
3. **Lint rule naming** for unreferenced design blocks (candidate home:
   `lint-anti-slop` in TOOLING.md).
4. Should **`type` be surfaced in `yass list`** output alongside the design
   block's name?
