# Round 06 results — kiln probe (two-arm DESIGN-BLOCKS validation)

**Date:** 2026-07-22 · **Panel:** gpt-5.5-extra-high · gemini-3.1-pro ·
claude-opus-4-8-thinking-high · composer-2.5 · **4 models × 2 arms = 8 cold runs.**

## Probe design

A fresh-domain CLI (kiln-firing controller) built as the experimental probe that
`context/DESIGN-BLOCKS.md` demands before it can land. **Two spec sets pinning
byte-identical observable behavior**, graded by one oracle, differing only in
the constructs under test:

- **Arm A (proposed language):** a `design: FiringSequence` document
  (`type: ordered-steps`, `content` block scalar) is the sole carrier of the
  multi-spec firing order; obligations bind to it via ref-only / attached
  `USES` (`./kiln.shared@FiringSequence`, plus one bare same-file
  `USES: FiringSequence`); one `SEE`→design; the pipeline dataflow pointer is
  slot-targeted `CONFORMS: ./kiln.fire@kiln.fire::RETURN`; **zero spec→spec
  USES anywhere** (structural fire→stage links are `SEE`).
- **Arm B (current v1, strongest good-faith form):** no design doc; the
  coordinating spec `kiln.fire` owns the order as a normative prose obligation;
  stage specs / `kiln.grade` back-reference it with `USES ./kiln.fire@kiln.fire`
  per the cross-cutting-concern guidance; the dataflow pointer is
  `USES: ./kiln.fire@kiln.fire::RETURN` per current `Slot.INPUT` rules.

Ten specs across four files per arm: `kiln.shared` (dispatch over
`{fire, report}` + usage residual, `kiln.lines` segmentation, `kiln.record`
E10/E25/E15/E20 registry, `kiln.grade` G90/G10/DONE registry + exit
contributions), four stage specs (`kiln.ramp`/`kiln.seal`/`kiln.soak`/
`kiln.vent`, documented **alphabetically**), producer `kiln.fire`, consumer
`kiln.report`. Both arms are byte-identical outside the five deltas above
(diff-verified pre-panel).

**The probe target:** the mandated stage order **SOAK → SEAL → VENT → RAMP**
is deliberately non-obvious — not alphabetical/documentation order
(RAMP, SEAL, SOAK, VENT), not reverse-doc order, not domain intuition
(vent → ramp → soak → seal). The stage arithmetic (×3, −220, +35, ×2) is
non-commutative and every stage logs `<id> <STAGE> <index>`, so a wrong order
fails on labels AND values (the oracle self-check asserts every rival order's
trace and final index diverge for every batch base). The order appears in
exactly ONE place per arm; every other stage enumeration is alphabetical.

## Method

Cold + isolated: each of the 8 runs got its own throwaway
`/tmp/yass-r06-<model>-<arm>` workspace containing **only** the four
`.yass.yaml` files for its arm, with the generic `experiment/cold-prompt.md`.
The private oracle was never copied anywhere. Black-box grading: stdin →
stdout/stderr/exit, byte-exact (surrogateescape).

Oracle: 48 batches; `grade.py --self-check` → `SELFTEST OK`; independent
reference impl 48/48 before the panel. Local pre-panel gates also included
`/tmp/check_refs_r06.py` → `RESULT: CLEAN` (arm A: 20 refs, every `USES`
target a design block, zero spec→spec USES, no dead design blocks; arm B: 18
refs, all spec/slot, no design docs).

## Grades — 381/384, both arms functionally clean on every probe target

| model | arm A | arm B |
|-------|-------|-------|
| gpt (Python / Python) | 48/48 | 48/48 |
| gemini (Python / Python) | 48/48 | 48/48 |
| opus (Python / Python) | 48/48 | 48/48 |
| composer (Go / Go) | 45/48 | 48/48 |

## Analysis

### Lifecycle ordering — arm A pass + arm B pass → **NOT a correctness need**

All eight runs implemented the non-obvious SOAK→SEAL→VENT→RAMP order exactly
(every lifecycle batch — full traces, negative-index trace, grade boundaries
499/500 and 2000/2001, per-record grouping, exit maxima — passed in all 8;
composer's arm-A misses were segmentation, below, and its NOTES explicitly
list "Firing sequence SOAK → SEAL → VENT → RAMP with correct arithmetic").
No model in either arm fell back on alphabetical order, documentation order,
or domain intuition; zero NOTES confusion about where the order lived.

Per the PLAN's outcome table, **pass + pass = ergonomic only**: the strongest
current-language expression (a coordinating spec owning the order as prose,
back-referenced per the cross-cutting guidance) one-shots just as well as the
proposed construct. The multi-spec lifecycle-ordering case joins the five
prior construct requests closed under the same discipline: **`wontfix` as a
correctness motivation.** This was the last untested residue of the
sequencing/ordering cluster (rounds 02/05 covered per-call dataflow and
stateful preconditions inside an owning spec; round-06 covered ordering with
no natural owning symbol).

### The `design:` construct itself — **cold-legible, 4/4 functional**

The other half of the result: the proposed language did not hurt. All four
arm-A models parsed the unfamiliar `design:` document cold, treated its
`content` as normative, followed ref-only `- USES: FiringSequence` /
`USES: ./kiln.shared@FiringSequence` bindings across files, and read the
`SEE`→design pointer without comment. opus's NOTES walk the design block, the
CONFORMS dataflow boundary, and the empty-residual reasoning explicitly; no
model flagged the new document type, the repurposed USES, or the missing
spec→spec USES as ambiguous or contradictory. So if DESIGN-BLOCKS is adopted,
it will not break cold one-shot implementation — it just cannot claim
*ordering correctness* as its justification.

### Dataflow reading as slot-targeted `CONFORMS` — **re-validated 4/4**

Arm A's consumer (`kiln.report` INPUT → `CONFORMS: ./kiln.fire@kiln.fire::RETURN`
+ stated trust boundary + violation residual) reproduced the resolved
`dataflow-invisible` / `trust-boundary-violation-residual` behavior exactly:
all four arm-A models trusted fire's line shape, never re-validated ids /
field counts / indexes, never re-derived arithmetic or grades, and routed
every violation input (out-of-contract id `zz`, wrong field count, unknown
token, blank line, lowercase token) to the stated LINES-only residual. Arm B
re-verified the same behavior under the current `USES …::RETURN` carrier
(4/4). **DESIGN-BLOCKS migration item 1 (dataflow pointer → CONFORMS) is
experimentally cleared**; the trust-boundary obligations are carrier-independent,
as the proposal claimed.

### composer arm A 45/48 — **MODEL-ERROR, not a spec defect**

Misses: `fire_tab_is_data`, `fire_cr_is_data`, `rep_tab_is_data` — all three
are the same root cause, confirmed in the source: composer's arm-A Go
implementation splits fields with `strings.Fields` (any Unicode whitespace),
so a tab or CR acts as a separator despite the spec's explicit MUST-NOT
("any byte other than the ASCII space (0x20) -- including the tab (0x09) and
the carriage return (0x0D) -- … is ordinary field data"). This is the
**identical shortcut composer took in round-01** (also `strings.Fields`, also
caught only by the tab/CRLF batches). The decisive control: the segmentation
text is byte-identical in both arms, and composer's own arm-B implementation
hand-rolled a byte splitter and scored 48/48 — same model, same words, right
one time out of two, while the other three models were right in both arms
(7/8 runs clean on these batches). Uncorroborated single-model miss on
unambiguous text → model-error per the ≥2-model rule. No relation to the
probe targets.

### NOTES signal — benign assumptions only

Across all 8 NOTES.md files: extra argv beyond the first ignored (spec silent
— true, and harmless: the oracle never passes extras); identifier length
checked in bytes (equivalent for the ASCII-only grammar); `str(int)` /`%d`
negative formatting matches the minus-sign INVARIANT; the rejected-record
exit contribution read across `kiln.fire` + `kiln.grade` (opus and composer
both note it resolves unambiguously once both specs are read — the
cross-cutting-single-home pattern working as designed). No hedged guesses, no
divergent resolutions, no contradiction reports in either arm.

### Regression surface — held (8/8 where exercised)

`closed-set-dispatch-residual` (5 dispatch batches incl. bare `-`),
`input-segmentation-completeness` + `segmentation-terminator-mechanics`
(7/8 runs fully clean; the eighth is the composer model-error above),
`residual-reachability` (all three exhaustiveness assertions — record set,
fire error set, grade set — implemented with no invented catch-all),
`cross-cutting-single-home` (`kiln.shared` owning protocol/format/registry,
no drift), slot-targeted CONFORMS transclusion (`kiln.lines::INPUT`,
`kiln.record::INPUT`/`::ERROR`, `kiln.grade::INVARIANT`) all held in both arms.

## What this means for context/DESIGN-BLOCKS.md

- **The proposal's own validation plan is now complete, with a split result.**
  Probe 1 (dataflow as CONFORMS) passed 4/4. Probe 2 (lifecycle both ways)
  found the construct cold-legible **and** unnecessary for correctness — the
  arm that "matters" (B, per the proposal's own words) did *not* fail.
- **Adoption can no longer be justified as fixing a one-shot-failure defect.**
  Under this experiment's charter, lifecycle-ordering-as-correctness goes to
  `wontfix`, consistent with the five prior refuted construct requests.
- **What survives is the proposal's other grounds**, none of which this
  experiment's black-box method can falsify: a non-distorting home for
  algorithms and tech/stack constraints (the rest of
  `slot-model-for-non-functional-specs`), the USES relation disambiguation
  (each relation one target kind — demonstrated machine-checkable by the
  arm-A ref-check partition), and the unreferenced-design-block lint. Those
  are ergonomic/authoring-quality arguments. If the language owner wants them,
  round-06 shows the construct costs nothing in cold legibility; but the
  experiment's evidence rule says the language does not *need* it.

## Disposition

- Multi-spec lifecycle ordering (the last open facet of the sequencing
  cluster) → **refuted as a correctness need (4/4 + 4/4 clean)**.
- `design:` construct cold-legibility → **validated (4/4 functional, zero
  NOTES confusion)** — adoption is now a design decision on ergonomic grounds,
  recorded against `slot-model-for-non-functional-specs` /
  `context/DESIGN-BLOCKS.md`, not an experimental necessity.
- `dataflow-invisible` + `trust-boundary-violation-residual` → **re-verified**
  under both the current (`USES`) and proposed (`CONFORMS`) carriers.
- composer arm-A 45/48 → **model-error** (repeat of the round-01
  `strings.Fields` shortcut; uncorroborated; arm B clean).

**Round 6 produced no new actionable spec defect.** Convergence counter
advances **1/2 → 2/2** — the K=2 no-new-findings convergence signal is
reached.
