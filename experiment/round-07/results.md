# Round 07 results — granary probe (DESIGN-BLOCKS clarity, two-arm)

**Date:** 2026-07-22 · **Panel:** gpt-5.5-extra-high · gemini-3.1-pro ·
claude-opus-4-8-thinking-high · composer-2.5 · **4 models × 2 arms = 8 cold runs.**

## Probe design

Round-06 settled correctness (the `design:` construct is cold-legible but
answers no one-shot-failure need). Round-07 measured the proposal's strongest
surviving rationale — the **algorithm-home clarity claim** — directly, with a
**pre-registered rubric fixed in PLAN.md before any panel run**. Two spec sets
pinning byte-identical behavior (communal granary allotment: `allot`/`tally`,
largest-remainder allotment with five explicit traps — per-claim floor, exact
integer remainders, exactly-`leftover` distribution, the non-conventional
three-level tie-break remainder-desc → base-asc → later-position-first, and
the zero-total residual):

- **Arm A:** the six-step computation lives only in `design:
  AllotmentProcedure` (`type: pseudocode`), bound by a bare `USES:`; plus
  `design: EmptyFeed` bound via a **guarded** `WHEN … MUST … USES:` (the
  guarded-USES→design rule, flagged untested in round-06, made
  observable-if-ignored: zero-byte tally input → the single line `EMPTY
  FEED`).
- **Arm B (good-faith current v1):** the identical rules as declarative
  obligations scattered across INPUT / four guarded RETURN obligations /
  INVARIANT; dataflow as `USES …::RETURN`.

Correctness was expected clean in both arms; the signal was the
**differential**: (1) PRIMARY — NOTES hedging about the algorithm, per the
pre-registered HEDGE/MISDESCRIPTION/RESTATEMENT rubric; (2) SECONDARY —
corroborated trap-batch misses in one arm only; (3) TERTIARY — duration,
descriptive only.

Oracle: 50 batches; `--self-check` → `SELFTEST OK` (including proof that every
trap batch discriminates its rival models — input-order, larger-base,
earlier-position, half-up); reference impl 50/50 and ref-check `RESULT: CLEAN`
before the panel.

## Grades and per-run table — 400/400, zero misses anywhere

| model | arm | language | score | duration (s) | algorithm NOTES items | hedger? |
|-------|-----|----------|-------|--------------|----------------------|---------|
| gpt | A | Python | 50/50 | 102 | 0 | no |
| gpt | B | Python | 50/50 | 155 | 0 | no |
| gemini | A | Python | 50/50 | 175 | 0 | no |
| gemini | B | Python | 50/50 | 238 | 1 HEDGE (borderline — see below) | **yes** |
| opus | A | Python | 50/50 | 223 | 0 | no |
| opus | B | Python | 50/50 | 217 | 1 RESTATEMENT | no |
| composer | A | Python | 50/50 | 92 | 2 RESTATEMENT (1 borderline — see below) | no |
| composer | B | Go | 50/50 | 155 | 0 | no |

All HOWTORUN lines worked (composer-B: `go build granary.go`, then full
score). **The model-error ledger is empty this round: no misses at all.**

## PRIMARY — NOTES differential: arm A 0 hedgers, arm B 1 → **NULL**

Rubric applied independently by author and lead with agreement; the two
borderline calls, both resolved per the rubric's letter:

- **gemini-B (counted as HEDGE, the caveat noted):** on the tie-break, "later
  input position first … could mean the relative index among accepted claims
  or the absolute index among all input records. I assumed absolute" — a
  literal rubric match ("I assumed" resolving an interpretation of the
  tie-break). The caveat: the model itself then proved the two readings order
  identically (accepted claims are a subsequence of input records, so either
  index sorts the same) — the flagged ambiguity is **vacuous**, and the same
  vacuous distinction exists verbatim in arm A's design block, where no model
  raised it. Counted per the pre-registered rubric; it changes nothing (1 < 2).
- **composer-A (classed RESTATEMENT, the closest call against arm A):** its
  tie-break item opens "Interpreted as higher input index receiving higher
  priority" under an assumptions heading — hedge-flavored phrasing, but the
  content is exactly the explicit spec text with no alternative reading
  offered, stated with confidence and implemented correctly. Its second item
  (weight-0 claims participate in the total and compete for leftover) is a
  correct derivation from the stated steps, not an under-determination.
- **opus-B (RESTATEMENT):** the magnitude/no-clamping note restates the
  explicit arbitrary-precision requirement.
- Everything else in all eight NOTES files falls under the rubric's exclusion
  list (segmentation, dispatch/extra-argv, I/O buffering/stream-interleaving,
  identifier byte-length, tally classification, usage-path stdin).

Hedger counts: **arm A = 0, arm B = 1.** The pre-registered positive signal
(≥2 arm-B hedgers with ≤1 arm-A) is NOT met; the reverse signal (≥2 arm-A) is
NOT met. **Outcome: NULL.** Both carriers were read essentially cleanly; the
single arm-B hedge was self-refuted by the model that raised it.

## SECONDARY — trap-batch misses: none

All 22 algorithm-relevant batches (11 trap batches + their tally-side
consumers) passed in all 8 runs — every floor edge, all three tie-break
levels, exact-division zero-iteration, zero-weight exclusion, zero-total
residual, rejected-record non-participation, and the 999999 exactness case.
No correctness-at-the-margin differential in either direction.

## TERTIARY — duration (descriptive only, per PLAN: never decision-weight)

Arm A was faster for 3 of 4 models (gpt 102 vs 155, gemini 175 vs 238,
composer 92 vs 155; opus ~flat 223 vs 217); means 148 s (A) vs 191 s (B).
Single run per cell, no repeats — suggestive at most, and explicitly not
part of the disposition.

## Secondary target — guarded-USES→design: cold-covered, clean

All four arm-A models produced `EMPTY FEED` on the zero-byte `tally_empty`
batch (a model ignoring the design block would have written the four
zero-count summary lines and failed), and none over-applied it (the
`tally_blank_only` boundary — `"\n"` is NOT empty — passed 8/8). Zero NOTES
confusion about the `design:` construct, the bare `USES` binding, or the
guarded EmptyFeed binding in any arm-A run. The rule flagged untested in
round-06 is now cold-covered.

## Disposition — per the pre-registered outcome table

PLAN row: **null NOTES differential + no trap misses** →

> "proposal keeps only its unfalsifiable rationale; recommend closing
> `design-blocks-proposal` as a pure design preference (no experimental
> mandate)."

Applied:

- **`design-blocks-proposal` → recommend `wontfix` (no experimental mandate,
  rounds 06–07).** The experiment's full verdict across both rounds: the
  construct is **cold-legible and harmless** (round-06: 4/4 functional on the
  ordering probe; round-07: 4/4 clean incl. the guarded binding; zero
  construct confusion in any of 8 arm-A runs) — but it fixes no correctness
  defect (round-06: A pass + B pass) and its clarity advantage did not
  materialize under a pre-registered measurement (round-07: null, 0 vs 1
  borderline hedger). Its remaining rationale (a non-distorting home for tech
  constraints; the USES relation disambiguation, already shown
  machine-checkable; dead-block lint) is not falsifiable by black-box one-shot
  outcome. Adoption is a legitimate language-owner *preference*; it is not an
  experimentally mandated change, and the experiment charter therefore closes
  it.
- **`slot-model-for-non-functional-specs` → recommend `wontfix` (probed
  rounds 06–07).** Both of its strongest facets are now probed: the
  multi-spec lifecycle (round-06, refuted as correctness) and the step-wise
  algorithm home (round-07, clarity claim not experimentally supported —
  scattered declarative obligations were read as cleanly as the pseudocode
  block). The residual serializer/config-spec awkwardness claim is ergonomic
  and rests on the same unfalsifiable ground.
- **Convergence.** Round-07 produced **no new actionable spec-defect** and no
  miss of any kind. This is the third consecutive no-new-findings round
  (05, 06, 07); the K=2 convergence signal, first reached at round-06, is now
  exceeded — the language under test is stable against this probe family.
