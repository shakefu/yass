# Round 7 — Plan (DESIGN-BLOCKS clarity probe: step-wise algorithm)

## What this round tests

Round-06 settled the **correctness** question: the `design:` construct is
cold-legible but answers no one-shot-failure need for lifecycle ordering
(A pass + B pass → ergonomic only). What remains of
`context/DESIGN-BLOCKS.md` is its strongest surviving rationale: a
**step-wise ALGORITHM** has no natural home in the five slots — "ordering
goes in INVARIANT, accumulation goes in SIDE-EFFECT, and the control flow is
invisible" (the original `slot-model-for-non-functional-specs` complaint).
Round-07 measures that **clarity claim directly**: content fully definable in
current v1 as declarative obligations, where the design block should merely
be *clearer*. Both arms pin byte-identical behavior; the signal is the
differential, not pass/fail.

Secondary target: the **guarded-USES→design rule** ("when a guarded
obligation carries a USES, the guard scopes when the design binds") was
flagged untested in round-06. This round exercises it once, with a small
second design block deliberately orthogonal to the algorithm.

## Targets

1. **`design-blocks-proposal` / `slot-model-for-non-functional-specs`** —
   the algorithm-home rationale. Deciding question: given a naturally
   procedural computation that BOTH arms pin completely and unambiguously,
   do cold models reading the current-v1 constraint formulation (arm B)
   hedge, guess, or misdescribe the algorithm more than models reading the
   `design:` pseudocode block (arm A)? And do any marginal-corner misses
   (ties, rounding edges, boundary iterations) appear in arm B but not arm A?
2. **guarded USES→design** (arm A only) — a `WHEN <condition> … MUST … USES:
   <block>` binding whose behavior is observable-if-ignored: a model that
   skips the design block fails the batch, so cold coverage is real, and the
   block is orthogonal to the algorithm so it cannot confound target 1.

## Domain and algorithm

**Communal granary allotment** (`granary`). Fresh vs prior rounds (berth,
apiary, vault, axle, weighbridge, kiln). One binary, subcommands
`allot`/`tally` (producer | consumer pipe). Deterministic, exact integer
arithmetic; no clock/network/randomness.

`granary allot` distributes a pool of storage units over claims by
**largest-remainder proportional allotment** — naturally procedural (6
ordered steps), fully definable declaratively, and dense with classic
under-determination traps. Both arms pin every trap explicitly; only the
carrier differs.

### The procedure (identical semantics in both arms)

1. `total` ← sum of accepted claims' weights, accumulated from zero;
   rejected records contribute nothing.
2. If `total` = 0 (no accepted claims, or all weights 0): every claim is
   allotted 0, the whole pool remains, no later step runs.
3. Per claim: `base` ← `(pool × weight) // total` (integer **floor**, per
   claim, at this step — never half-up, never at the end);
   `remainder` ← `(pool × weight) − (base × total)`, an exact integer;
   remainders are compared as these integers, never as float fractions.
4. `leftover` ← `pool − Σ base`.
5. Distribute **exactly `leftover`** extra units, one per claim, to the first
   `leftover` claims by: **larger remainder first; ties → smaller base
   first; still tied → later input position first.** No claim gets more than
   one extra; when `leftover` = 0 the step runs zero times.
6. Units = base + extra. Unallotted remainder = pool − Σ units (0 whenever
   `total` > 0; the whole pool otherwise).

### The traps (≥3, pinned in BOTH arms, placement differs)

| # | trap class | the pinned rule | wrong-prior it defeats |
|---|-----------|-----------------|------------------------|
| T1 | rounding direction/timing | floor per claim at step 3, never half-up, never at-end | half-up (`7/2 → 4,4` over-allots), round-at-end |
| T2 | tie-break order | remainder desc → **base asc** → **later position first** | input-order / largest-quota / earlier-position conventions (the standard largest-remainder prior) |
| T3 | iteration/termination boundary | exactly `leftover` units, ≤1 per claim, zero iterations when leftover = 0 | loop-until-pool-empty; distributing to zero-remainder claims |
| T4 | accumulator init / exactness | total accumulated from zero over **accepted** claims only; remainders as exact integer numerators | rejected rows participating; float remainders (ties break wrongly) |
| T5 | zero-total residual | all claims GET 0, REMAIN = pool, no distribution | division by zero / invented pro-rata |

**Arm A:** steps 1–6 (with T1–T5 inline in the step text) live in
`design: AllotmentProcedure` (`type: pseudocode`) in `granary.allot.yass.yaml`;
the spec's RETURN binds via `USES: AllotmentProcedure` and carries only the
observable output contract. **Arm B (good-faith, complete, not a strawman):**
the identical rules as declarative obligations, necessarily scattered —
participation/accumulation (T4) in `INPUT`, floor + remainder + distribution
+ zero-total (T1, T2, T3, T5) across five guarded `RETURN` obligations,
exactness (T4) in `INVARIANT`. Every rule text is near-identical between
arms; only placement and carrier differ.

## CLI shape (identical both arms, byte-exact)

- Dispatch: `argv[1]` ∈ closed set `{allot, tally}`; missing / unknown /
  wrong-case / bare `-` → `usage: granary {allot|tally}\n` to stderr, exit 2,
  no stdout.
- **`granary allot`** — header gate: the FIRST record must be `POOL <units>`
  (exactly two fields, well-formed non-negative integer); otherwise (including
  empty input or a blank first record) → `E40 bad pool header` to stderr,
  nothing to stdout, no records processed, exit 2. Every later record is a
  claim `CLAIM <id> <weight>` (id: 1–6 chars, first A–Z, all A–Z0–9; weight:
  digit run, no leading zero unless `0`), rejected per the ordered registry
  E10 empty line / E25 unknown operation / E10 field-count / E15 bad claim id
  / E20 bad number (one error line per record, stop at first failure,
  exhaustive — no residual). Output: one `<id> GET <units>` per accepted
  claim **in input order** (regardless of distribution order), then exactly
  one `REMAIN <r>` line. Exit 0, or 1 if any claim record was rejected.
- **`granary tally`** — the consumer. WHEN stdin is completely empty (zero
  bytes) → the **EmptyFeed** behavior: write the single line `EMPTY FEED`,
  nothing else, exit 0 (arm A: guarded `USES: EmptyFeed` design block — the
  secondary target; arm B: the same rule as guarded prose). Otherwise
  classify each record: second field `GET` → claim line (CLAIMS +1; third
  field added to UNITS when present and well-formed — the violation residual
  adds nothing); else exactly-two-field `REMAIN <n>` with well-formed `<n>`
  → REMAIN sum; else (unknown token, wrong shape, blank line) → LINES only.
  Output exactly `LINES/CLAIMS/UNITS/REMAIN`; always exit 0. Trust boundary:
  rely on allot's line-shape guarantees, never re-validate ids or re-derive
  the allotment; the violation residual is stated.

## Specs and file layout

Five specs across three files per arm (`test-specs/round-07-granary/spec-arm-{a,b}/`):

- `granary.shared.yass.yaml` — `granary` (dispatch + usage residual + global
  line/echo invariants + SHOULD stream-output), `granary.lines` (segmentation,
  the standing GUIDANCE checklist — byte-identical to round-06's text modulo
  names), `granary.errors` (error-line shape, one-per-record).
- `granary.allot.yass.yaml` — `granary.allot` (+ arm A:
  `design: AllotmentProcedure`).
- `granary.tally.yass.yaml` — `granary.tally` (+ arm A: `design: EmptyFeed`).

### Arm deltas (everything else byte-identical, diff-verified)

| site | Arm A | Arm B |
|---|---|---|
| algorithm carrier | `design: AllotmentProcedure` (`type: pseudocode`, steps 1–6); allot RETURN: "compute … by the AllotmentProcedure design, exactly as its numbered steps state — the computation is stated there and nowhere else" + `USES: AllotmentProcedure` (bare, same-file) | no design doc; the six steps as declarative obligations: participation in INPUT, floor/remainder/distribution/zero-total as guarded RETURN obligations, exactness in INVARIANT |
| empty-feed carrier | `design: EmptyFeed` (`type: ordered-steps`); tally: `WHEN stdin empty … MUST follow the EmptyFeed procedure … USES: EmptyFeed` (**the guarded-USES→design probe**) | tally: `WHEN stdin empty … MUST write the single line \`EMPTY FEED\` …` (guarded prose) |
| dataflow pointer (tally INPUT) | `CONFORMS: ./granary.allot@granary.allot::RETURN` (as validated round-06) | `USES: ./granary.allot@granary.allot::RETURN` (current rule) |
| spec→spec USES | none anywhere (structural dispatch links are SEE; validated by ref-check) | dataflow USES only; all refs spec/slot |

Both arms share slot-targeted `CONFORMS` for segmentation
(`granary.lines::INPUT`) and error shape (`granary.errors::INVARIANT`) —
unchanged under the proposal, minimizing confounds.

## Measurement design (pre-hoc — fixed before any panel run)

Correctness is **expected** 8/8 clean; the round's signal is the differential.

### PRIMARY — NOTES differential (counting rubric, fixed now)

For each of the 8 runs, count NOTES.md items that are **about the allotment
computation** (quota/floor/remainder/tie-break/distribution/leftover/
zero-total/exactness — NOT segmentation, dispatch, I/O, ids, or the
EmptyFeed rule) and classify each such item as exactly one of:

- **HEDGE** — resolves an under-determination by assumption/guess ("I
  assumed", "the spec doesn't say", "ambiguous", "I chose to interpret");
- **MISDESCRIPTION** — describes the algorithm differently from the spec
  (e.g. wrong tie-break level order, "standard largest remainder"), even if
  the implementation is correct;
- **RESTATEMENT** — repeats explicit spec text with confidence (no signal;
  not counted).

A model "hedges" if it has ≥1 HEDGE or MISDESCRIPTION item. Outcomes:

- **Positive clarity signal (supports adoption on ergonomic grounds):**
  ≥2 arm-B models hedge while ≤1 arm-A model does.
- **Reverse signal (evidence AGAINST adoption):** ≥2 arm-A models hedge (or
  express confusion about the `design:` construct, the USES binding, or the
  guarded EmptyFeed binding) while their arm-B counterparts do not. This is
  recorded explicitly as anti-adoption evidence, not explained away.
- **Null:** anything else — the clarity claim gains no experimental support;
  `design-blocks-proposal` adoption remains a pure language-owner preference.

### SECONDARY — marginal-corner misses

Oracle batches are concentrated on the traps (tie levels, floor edge, exact
division, zero-weight, zero-total, leftover>1). A **corroborated (≥2-model)
arm-B miss on trap batches absent in arm A** upgrades the signal from
ergonomic to *correctness-at-the-margin* — the strongest possible pro-adoption
result. A corroborated arm-A trap miss absent in arm B is the strongest
anti-adoption result. Single-model misses are tagged model-error per the
standing rule.

### TERTIARY — duration

Report per-run wall time from `meta.json`, arm A vs arm B, as descriptive
context only (single run per model per arm; noise dominates; never
decision-weight).

### What the combined outcome means for `design-blocks-proposal`

| NOTES differential | trap misses | disposition |
|---|---|---|
| positive | arm-B corroborated miss, arm A clean | adopt-worthy: clarity confirmed AND correctness-at-the-margin — the strongest case DESIGN-BLOCKS can earn |
| positive | none | clarity claim experimentally supported; adoption justified on ergonomic grounds (charter-consistent to adopt or defer — owner's call, now with evidence) |
| null | none | proposal keeps only its unfalsifiable rationale; recommend closing `design-blocks-proposal` as a pure design preference (no experimental mandate) |
| reverse | any | recommend REJECTING the proposal: the construct measurably confuses cold readers |
| any | arm-A corroborated miss, arm B clean | reject: the construct hurts at the margin |

Also cold-covered regardless of outcome: the guarded-USES→design rule (the
`tally_empty` batch fails for any arm-A model that ignores the design block).

## Pinned error/output registry

- usage: `usage: granary {allot|tally}` → stderr, exit 2, no stdout.
- `E40 bad pool header` → stderr, nothing to stdout, no records processed,
  exit 2 (empty input, blank first record, wrong field count, non-POOL
  keyword, malformed units).
- Claim checks in order: `E10 malformed record: empty line` · `E25 unknown
  operation: <field1>` · `E10 malformed record: CLAIM expects 3 fields, got
  <n>` · `E15 bad claim id: <field2>` · `E20 bad number: <field3>`. One line
  per record; rejected → exit 1 (never lines on stdout for that record).
- allot stdout: `<id> GET <units>` per accepted claim in input order, then
  `REMAIN <r>`. Conservation invariant: Σ units + r = pool exactly.
- tally stdout: `LINES <n>` / `CLAIMS <n>` / `UNITS <sum>` / `REMAIN <sum>`,
  exit 0 — or exactly `EMPTY FEED` on zero-byte input.

## Oracle batch design (~50 batches)

Same machinery as round-06 (embedded simulator — sort-key distribution;
`ref.py` uses an explicit repeated-selection scan loop, a genuinely different
shape; hand-pinned `(name, sub, stdin, stdout, stderr, exit)`; byte-exact;
`--self-check` → `SELFTEST OK`; per-batch FAIL diffs; `SCORE: n/m`):

- **DISPATCH (5):** missing / unknown / `ALLOT` / `TALLY` / bare `-`.
- **ALLOT header gate (6):** empty input; lone `"\n"` (blank first record);
  no header (claims only — asserts reject-all + single E40); wrong field
  count; leading-zero units; header-only (`REMAIN <pool>`).
- **ALLOT segmentation (6):** missing trailing LF; blank interior (E10 +
  allotment over the accepted claims); double trailing LF; tab-as-data
  (field-count E10); CR-as-data (`E20 bad number: 1\r`); space-run collapse.
- **ALLOT record format (8):** count-high; unknown keyword; one-field
  record (E25 precedes count); lowercase id; 7-char id; digit-first id;
  leading-zero weight; E15-before-E20.
- **ALLOT algorithm traps (11):** exact division (leftover 0 — zero
  distribution iterations); floor-not-half (`POOL 7`, weights 1,1 → 3,4);
  three-way tie → **later position wins** (10/1,1,1 → 3,3,4 — input-order
  prior gives 4,3,3); remainder-tie → **smaller base wins** against input
  order AND largest-quota (6/3,1 → 4,2 — both rival priors give 5,1);
  remainder ordering (10/3,1,2 → 5,2,3); leftover=2 (11/3,1,2 → 5,2,4);
  zero-weight claim never receives (5/0,1,1 → 0,2,3); zero-total residual
  (8/0,0 → 0,0 REMAIN 8); pool 0; rejected-record non-participation
  (E15 row excluded from total); large exact values (999999/1,2).
- **TALLY (14):** `EMPTY FEED` on zero bytes (**the guarded-design batch**);
  blank-only input (NOT empty → four lines, LINES 1); piped allot output;
  REMAIN summing; trusted out-of-contract id; GET with malformed / missing /
  extra third field (violation residual: CLAIMS counted, UNITS untouched /
  extra field ignored); malformed REMAIN (bad number / wrong count → LINES
  only); unknown token; lowercase token; tab-as-data; missing trailing LF.

Self-check additionally asserts, per designated trap batch, that **rival
models produce different output**: earlier-position tie-break, larger-base
tie-break, and half-up rounding each diverge from the pinned expectation
(so the batches genuinely discriminate); plus conservation (Σ GET + REMAIN =
pool) across every pinned allot expectation, and full simulator/batch
agreement.

## Regression surface (carried, not asserted)

`closed-set-dispatch-residual` (dispatch batches), header-gate write-nothing
idiom (round-02 pattern), `input-segmentation-completeness` +
`segmentation-terminator-mechanics` (full checklist text reused),
`residual-reachability` (exhaustive claim-check assertion, no dead
catch-all), `dataflow-invisible` + `trust-boundary-violation-residual`
(tally trust + violation batches under both carriers),
`cross-cutting-single-home` (`granary.shared`), normativity gradient (SHOULD
in `granary`, SHOULD-NOT in allot SIDE-EFFECT, MAY in tally), five-slot
coverage.

## Procedure

1. Author both arms + oracle + ref + HOWTORUN from this PLAN. Validate:
   `/tmp/check_refs_r07.py` → `RESULT: CLEAN` (arm A: every USES target a
   design block, zero spec→spec USES, no dead designs; arm B: no design
   docs, all refs spec/slot); `grade.py --self-check` → `SELFTEST OK`;
   `grade.py --cmd "python3 …/ref.py"` → full score. Commit before any
   panel run.
2. Panel: 4 models × 2 arms, unique `/tmp/yass-r07-<model>-<arm>`
   workspaces, spec files only, `experiment/cold-prompt.md`, concurrent.
3. Grade all 8; extract NOTES items and classify per the rubric ABOVE
   (pre-hoc); collect durations from meta.json.
4. Write `experiment/round-07/results.md` per the outcome table; update
   FINDINGS (`design-blocks-proposal`, `slot-model-for-non-functional-specs`,
   guarded-USES coverage note).
