# yass — Tooling Backlog (holding doc)

The routing target for **irreducible tooling**: capabilities the experiment surfaced
that are *not* language changes and cannot be one. A finding lands here when fixing the
source of truth (the schema, `yass.yass.yaml`, the guidance/reference docs) would not
address it — because the gap is in a checker, a generator, or a CI gate rather than in
what a spec can express.

Each entry is indexed by id in `FINDINGS.md` under **tooling-request**; the detail lives
here. Nothing here is built yet; this preserves scope so it is not lost.

## Standing constraint: the language stays scalar-prose-only

Every capability below operates on specs whose obligation values are **scalar prose**.
The experiment tested — and **refuted across two rounds** — the hypothesis that
obligations needed richer value shapes (mappings carrying code/class/priority/metadata,
a structured error table, a `CODE` key, ordered/priority constructs) to be implemented
correctly cold. A panel of four models reproduced an 18-row error registry, byte-exact
messages, bare error-code literals read in two places, and prose priority chains with
**zero functional misses at adversarial scale** (see `FINDINGS.md`, Round-03 evidence;
ids `mapping-valued-obligations`, `error-table-structured`, `error-code-refs`,
`error-cardinality-implicit`, `priority-chains-prose`, all `wontfix`).

The residual value of that whole cluster is **ergonomic and machine-checkability**, not
correctness — and that residual is tooling, captured under `lint-anti-slop` below. The
language does not change.

## `lint-anti-slop`

`yass lint` — checks a spec is well-formed *in substance*, beyond schema validity.
Recurrence: repeated. Status: open.

Original scope:

- Flag **schema-valid-but-hollow** specs — obligations that parse but say nothing
  binding (empty prose, restatements of the slot name, pure boilerplate).
- Flag the **colon-space `: ` footgun** (unquoted prose that mis-parses as a nested
  mapping) and the **Norway problem** (`yes`/`no`/`on`/`off` read as booleans).
- An auto-quote `fmt` pass that quotes prose which needs it rather than rejecting it.

Owns the refuted structured-obligation cluster's **ergonomic / machine-checkability
residue** (routed here because the language stays scalar-prose-only — see above):

- **Error/defect registry extraction** — a `yass extract-errors` projection that reads
  the prose `ERROR` (and dispatch) obligations of a spec set and emits a machine-readable
  registry: one row per failure with `{code, class, condition, message-template, exit}`.
  This is the capability the `error-table-structured` / `mapping-valued-obligations`
  proposals were really after; it belongs in a projector over scalar prose, not in the
  obligation value shape.
- **Byte-exact message-template lint** — given that registry, flag the *same* logical
  error whose pinned message text differs across the specs that emit it (reworded,
  re-punctuated, drifted). Pins the wording the contract depends on without adding a
  message-template construct to the language.
- **Error-code reference validation** — validate that every error/defect code cited as a
  bare literal in prose (e.g. `V01`, `E90`) resolves to a defined entry in the extracted
  registry, and flag dangling or duplicate codes. This is the `error-code-refs` concern:
  real, but a cross-spec checker capability, not a language defect.
- **Idiom recognition as named lint patterns** — recognize the prose idioms the cluster
  proposed promoting to syntax, and lint them where they are error-prone, without
  promoting them:
  - **priority / precedence chains** ("emit the first matching …") — flag a chain whose
    ordering is ambiguous or whose cases overlap without a stated tie-break;
  - **cardinality bounds** ("at most one per X", "exactly one Y") — flag a stated bound
    that no obligation enforces, or two obligations that bound the same thing
    inconsistently.

## `test-gen-and-coverage`

spec → test generation plus a coverage check (every obligation maps to at least one
test). Promote `TEST-TAXONOMY.md` into the generator's mapping table. Recurrence:
universal. Status: open. **Gated** on per-obligation identity language work (obligations
are not individually addressable today — `RefTarget` stops at the slot), so a generator
cannot yet label a test with a stable obligation id.

## `self-validate-ci-gate`

Wire `yass validate spec/` into CI as a gate. Recurrence: repeated. Status: open
(**ungated** as of Round 04). The blocker — `self-validation-ref-bug` — is resolved:
commit `868112e` corrected the broken `../cli@…` refs, and a Round-04 repo-wide
re-verification confirms 17 specs schema-valid with all 114 refs resolving (0 dangling).
The self-definition now validates clean against its own schema, so the gate would be
meaningful; only the CI wiring remains, which is tooling, not a language change.
