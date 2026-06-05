# yass — Feature & Tooling Ideas (Scored Backlog)

A scored ideation backlog of **tooling / feature** ideas for yass and the `yass` CLI.
This is distinct from FUTURE.md, which holds deferred **language-design** decisions.
Nothing here is committed. The list was produced by an ideation pass followed by an
adversarial dedup-and-scoring pass, and screened against everything already recorded in
DECISIONS.md, FUTURE.md, GUIDANCE.md, and TEST-TAXONOMY.md. Ideas that merely restate a
decided, deferred, rejected, or built item were excluded.

Effort tags are **S** / **M** / **L** (small / medium / large), reproduced from the
report where it supplies one.

## Recommended first wave

The seven highest-ranked ideas, best-first. The first three — **elicit**, **lint**, and
**freshness signal** — are the low-effort wins: each attacks a failure mode the docs
name but no current tool addresses, and each adds no schema or syntax surface — elicit
and lint reuse the already-built v1 parse layer, while freshness stays on file metadata
(mtime/VCS), parsing nothing.

| # | Idea | Effort | Theme |
|---|------|--------|-------|
| 1 | elicit (interactive slot-by-slot intake) | S | Authoring |
| 2 | lint command (anti-slop structural rules) | S | Quality |
| 3 | freshness signal (paired-mtime staleness flag) | S | Lifecycle |
| 4 | colon-space / Norway auto-quote formatter | M | Authoring |
| 5 | Spec-diff test impact (regeneration worklist) | M | Generation |
| 6 | Spec-aware merge driver | M–L | Ecosystem |
| 7 | yass rename (ref-graph-aware identifier propagation) | M | Lifecycle |

### 1. elicit (interactive slot-by-slot intake) — S, Authoring

The cleanest win: a pure skill / prompt layer over the already-built `validate`/`query`
CLI, with no CLI or schema change, reusing the `allium:elicit` pattern. It attacks the
documented core failure mode head-on — GUIDANCE.md says an unguided LLM slops one giant
file — by running a slot-by-slot intake that enforces DbC slot order, RFC-2119 keying,
`WHEN` guards, and `CONFORMS`/`USES`, and catches the colon-space and Norway footguns at
write time. The one caveat: every answer MUST become a keyed obligation so no narrative
prose leaks in.

### 2. lint command (anti-slop structural rules) — S, Quality

Closes a concrete, unaddressed gap. GUIDANCE.md names slop as the core failure but only
covers the "one giant file" axis, leaving "schema-valid but hollow spec" uncovered — and
the coverage model is genuinely fooled by it, since a spec with zero `MUST`/`MUST-NOT`
reports as vacuously 100% covered. Trivially buildable on the v1 parse layer, read-only,
language-agnostic. Ship empty-`ERROR` and stub-length checks as advisory warnings, never
hard failures, and fold in the deferred YAML-footgun lint.

### 3. freshness signal (paired-mtime staleness flag) — S, Lifecycle

Targets the uncovered spec↔code axis — all documented drift detection is spec↔spec
content-hashing — with a cheaper mechanism, naming the most common rot at scale: the code
moved on, the spec did not. It sits inside the sanctioned "tooling drift-tracks" mandate,
leans on the decided 1:1 pairing, and needs no schema change and no obligation parsing.
It honors no-verification and language-agnosticism by staying on file metadata.

### 4. colon-space / Norway auto-quote formatter — M, Authoring

A deterministic safety net for the footguns the docs admit `validate` cannot catch
("parses wrong silently"): an unquoted `: ` becomes a valid-but-wrong nested map, and
Norway coercions flip values, corrupting obligation text while schema and validator pass.
A fixer beats both documented alternatives (lint nags, require-quoting rejects) by
removing a burden LLM authors reliably forget. Surface-only, language-agnostic, verifies
no content. Effort is more than a thin wrapper: it needs a careful targeted source-span
rewrite that preserves the modeline, `---` separators, doc order, and provenance
comments.

### 5. Spec-diff test impact (regeneration worklist) — M, Generation

A novel temporal framing distinct from the documented cross-reference drift work: diff
revision A against B and emit a per-obligation regeneration worklist (orphan / re-derive
/ relax / harden) classified by test impact. It targets a real agent-TDD pain — knowing
exactly which tests to touch after a spec edit — by reusing the fixed
normativity→test-strength mapping rather than inventing anything. Pure structural
routing, no content verification. It stacks on the deferred per-slot digests/index and
assumes a per-obligation identity the language withholds, so classification is
best-effort.

### 6. Spec-aware merge driver — M–L, Ecosystem

The format is unusually suited to it: each spec is an independent `---` document, and both
slot order and spec position are explicitly meaningless, so a structural three-way merge at
obligation granularity is sound in a way it never is for ordinary source. It targets a
pain the project created (1:1 pairing plus concurrent agents churning one file) and stays
in-philosophy — no content verification, no new vocabulary, no prose. The cost is real: it
needs a canonical emitter (which must pick an order the language leaves unspecified and
round-trip provenance and quoting), a three-way merge algorithm, and `.gitattributes`
plumbing; it reuses `ParseSpecFile`.

### 7. yass rename (ref-graph-aware identifier propagation) — M, Lifecycle

Fills the under-populated Lifecycle theme as the first write-side tool — every documented
command is read-only. Renaming a spec today silently breaks dependents; folding
validate-then-fix into one safe, stateless operation pays off as specs evolve. It stays
out of obligation content, touching only targets and names, and reads the filesystem
fresh on each invocation. It reuses the resolver but forces the whole-graph walk that
lazy-by-default and the deferred index avoided; bare-name ambiguity is left to manual
handling.

## Ideas by theme

All 28 surviving ideas, grouped by theme. Effort tags appear where the report supplies
one.

### Authoring

- **elicit (interactive slot-by-slot intake)** — S — Conversational intake that turns
  each answer into a correctly-keyed obligation, enforcing DbC order and catching
  footguns at write time; a pure skill over the existing CLI.
- **colon-space / Norway auto-quote formatter** — M — A `fmt --safe-quote` fixer that
  deterministically quotes the bare scalars that silently mis-parse; a third option
  beyond the documented lint-vs-require-quoting fork.
- **validate --explain --fix (error recovery)** — Plain-language messages tied to each
  violated schema rule (the strong half) plus narrowly-scoped autofix for the one or two
  truly canonical repairs (strip-extension, bare-`WHEN`).
- **obligation snippet palette (slot × normativity)** — Editor snippets/templates that
  pre-place block scalars to defuse the colon-space footgun — the preventive complement to
  the schema's detective validation; mostly serves human authors.
- **ref completion + transclusion preview** — Spec-aware editor completion of real
  on-disk spec names and slots plus a reverse "N specs `CONFORMS` this slot" list; useful
  DX but a heavy LSP lift that leans on the deferred index.

### Retrieval

- **Slot-scoped projection** — Narrow `query` output to just the addressed `::SLOT`(s); a
  cheap third query output shape, but specs are already five-slot units so per-query
  savings are marginal.
- **Task-anchored retrieval from a diff** — A "relevant" command mapping changed files to
  specs; the forward half is near-free via 1:1 pairing (and largely redundant), the
  inbound-edge half is gated on the deferred index.

### Generation

- **Spec-diff test impact (regeneration worklist)** — M — Temporal A-vs-B revision diff
  emitting a per-obligation regeneration worklist classified by test impact; reuses the
  fixed normativity→test-strength mapping.
- **Coverage ledger** — Persist the computed coverage as a regenerated, diffable,
  CI-gated artifact; novel as a gate, but its "required" denominator is fuzzy because
  boolean-`WHEN` classification is an agent judgment.
- **Implementation skeleton from slots** — Per-language skill that deterministically
  emits which stubs (one branch per `ERROR`, one guard per `INVARIANT`) while leaving
  contents to the agent; valid only if it mirrors the test-taxonomy mapping/content split
  and drops per-obligation anchoring.
- **Spec mutation drills (mutmut for prose)** — Flip normativity / invert `WHEN` / drop a
  slot, re-run the suite, expect a failure — empirically proves tests assert something,
  closing a loop the static coverage model leaves open; compute-heavy.
- **Coverage heat in provenance** — Render coverage spatially inline beside each
  obligation via the provenance-comment channel; a skill-side rendering of already-decided
  data, which must not live on the stateless `query` CLI.

### Lifecycle

- **freshness signal (paired-mtime staleness flag)** — S — Flag specs whose paired code
  file was touched more recently via mtime/VCS; cheap, language-agnostic, fills the
  uncovered spec↔code drift axis.
- **yass rename (ref-graph-aware identifier propagation)** — M — The first write-side
  tool: rename a spec and propagate the change across all dependent refs in one safe,
  stateless operation.
- **yass migrate (v1→vN language-version transformer)** — Envelope-only version
  transformer giving the inert `version:` key operational meaning; philosophically clean
  but unbuildable until a v2 actually exists to migrate to.
- **yass diff (obligation-level semantic diff)** — Turn a spec PR into a reviewable
  behavioral changelog (`MUST`→`SHOULD` weakened, retargeted `CONFORMS`); deterministic on
  keywords and refs, but cross-revision alignment leans on brittle prose similarity.

### Graph

- **ref-graph export (typed edge list)** — Emit `(source, relation, target)` triples;
  trivially feasible (the resolver already does the pass) and high-value, but an eager
  whole-graph walk that overlaps the deferred index.
- **dependents / rdeps (reverse-ref blast-radius lookup)** — Answer "what `CONFORMS` to
  this before I edit it"; valuable and a true inverse of `query`, but an eager full-corpus
  scan that fights lazy-by-default and needs real ref resolution, not literal grep.
- **impact-path trace (why is B reachable from A)** — Typed pathfinding between two
  anchors; novel output, but requires the deferred transitive-walk / cycle-detection
  machinery, and there is barely any multi-hop graph in v1 to trace.
- **coverage rollup across the corpus** — Whole-corpus coverage with shared contracts
  attributed once to their `CONFORMS` hub; the hub-dedup insight is genuinely new, but the
  "covered" half drags in language-specific test results.

### Quality

- **lint command (anti-slop structural rules)** — S — Catch schema-valid-but-hollow specs
  (all-`MAY`, empty-`ERROR`, stub-length, placeholder tokens) that fool the coverage model
  into reporting 100%; cheap, advisory-only.
- **Pairing gate + gap report (orphan & missing-spec audit)** — Mechanically enforce the
  honor-system 1:1 pairing rule as a CI gate; the file-level orphan check is clean, the
  symbol-coverage half needs deferred LSP / code-awareness.
- **Obligation-coverage CI gate** — A build-failing gate (vs. agent self-grade) for
  required obligations with zero linked tests; keepable only if scoped to validating a
  caller-supplied coverage map, keeping framework parsing outside the binary.
- **contradiction sweep (cross-obligation conflict flag)** — Flag `MUST`-vs-`MUST-NOT`
  conflicts from slop-by-accretion; only survivable as an explicitly-heuristic critique
  skill fenced off from `validate`, since judging conflict interprets prose.
- **redundancy lint (DRY-by-transclusion suggester)** — Detect undeclared near-duplicate
  prose and suggest a `CONFORMS` ref; the first tool to inspect prose for
  meaning-adjacency, an index-era feature, not a v1 build.

### Ecosystem

- **Spec-aware merge driver** — M–L — Structural three-way merge at obligation
  granularity, sound because slot and spec order are declared meaningless; gated behind
  the canonical emitter it implies.
- **OpenAPI ↔ yass bridge** — Adoption on-ramp mapping HTTP contracts to DbC slots; clean
  alignment but narrow (HTTP-only), lossy, and a substantial out-of-core artifact that
  leans on the rejected typed-schema territory.

### LLM-native

- **ambiguity marker (UNSURE as a first-class authored signal)** — A structured
  author-confidence channel validated like `WHEN`; cheap and novel, but it reopens the
  deliberately-closed no-free-prose relief valve and may weaken the forcing function.

## Open questions / philosophy tensions

Four ideas that score well but fight a stated principle or non-goal. Each is framed as a
decision to make, not a recommendation.

### ref-graph export (typed edge list)

*Strains: lazy-by-default retrieval; the eager-full-walk non-goal; the deferred generated
index.* High value and trivially feasible — the validator already resolves every ref, so
emitting triples is the same pass with a different sink, and rdeps / fan-in / orphan /
impact-path all derive from it. But it self-describes as "one on-demand eager walk of
edges," directly fighting the lazy-by-default principle and the eager-full-walk non-goal,
and it overlaps the deferred generated index that is meant to be the home for cross-file
graph data. The conversation: is this a standalone primitive, or just an output mode of
the index once it lands?

### Pairing gate + gap report (orphan & missing-spec audit)

*Strains: language-agnosticism (the line-coupling that got `COMPATIBLE` rejected).* The
1:1 pairing rule is the central anti-slop mechanism, yet ships as honor-system convention
with no enforcing tool — making it a cheap CI gate is squarely in spirit. But the
high-value "public symbols missing from spec" check requires per-language parsing / LSP,
exactly the language coupling that got `COMPATIBLE` rejected. The bet: ship the
language-agnostic file-level orphan gate now, defer the symbol-coverage half behind LSP /
index work.

### dependents / rdeps (reverse-ref blast-radius lookup)

*Strains: lazy-by-default retrieval (the eager full-graph walk the philosophy forbids).*
"What `CONFORMS` to this before I edit it" is genuinely valuable and implicitly demanded
by the deferred drift work, but every invocation must read the whole corpus's outbound
refs to find inbound edges — the precise eager full-graph walk the philosophy forbids —
and the grep-plus-parse framing undersells it (path-relative, slot-addressed refs need
real resolution, not literal grep). Worth deciding whether to defer it to land with the
index, where reverse edges naturally live.

### ambiguity marker (UNSURE as a first-class authored signal)

*Strains: the no-free-prose / no-commentary-channel non-goal; the small-closed-vocabulary
principle.* It captures calibrated LLM uncertainty as structure rather than as the banned
prose, with precedent in keeping orthogonal axes (necessary/sufficient) separate from
normativity — but it reopens the no-free-prose relief valve the design deliberately
closed, adds the first non-RFC-2119 reserved keyword, and an agent that can mark `UNSURE`
may flag rather than resolve. The conversation worth having: does this help calibration,
or does it erode the forcing function that makes yass work?

## Out of scope here

Ideas already decided, deferred, rejected, or built were deliberately excluded from this
backlog; they live in **DECISIONS.md** and **FUTURE.md** (and the meta-rule / authoring
material in GUIDANCE.md and TEST-TAXONOMY.md). Look there before proposing anything that
sounds like a CLI command already in scope, a drift-detection or index mechanism, or a
language-vocabulary change.

One dependency observation: most **Graph**-theme ideas (ref-graph export, rdeps,
impact-path trace, coverage rollup) collapse into the deferred **generated project-level
index** — they are eager whole-graph reads whose natural home is the index. The index
question (FUTURE.md) should be settled first, since whether these ship as standalone
primitives or as output modes of the index follows directly from it.
