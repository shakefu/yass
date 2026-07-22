# Round 6 — Plan (DESIGN-BLOCKS two-arm probe)

## What this round tests

`context/DESIGN-BLOCKS.md` proposes a second document type (`design:` — named,
typed, freeform normative content) plus a repurposing of `USES` to target design
blocks exclusively (dataflow pointers migrate to slot-targeted `CONFORMS`;
structural links migrate to `SEE`). The proposal's own validation plan demands an
experimental probe **before** any source-of-truth change. This round is that
probe: **two spec sets pinning byte-identical observable behavior**, differing
only in which construct carries a multi-spec lifecycle-ordering procedure and
which relation carries the dataflow pointer.

## Targets (finding ids + the deciding question each)

1. **`slot-model-for-non-functional-specs`** (repeated, OPEN — the
   PROCEDURE/ALGORITHM slot request, which DESIGN-BLOCKS resolves as a new
   *document type*). Deciding question: can a **multi-spec lifecycle procedure**
   — stage ordering that spans four stage specs' subjects and is deliberately
   NOT guessable from convention (not alphabetical, not documentation order, not
   domain intuition) — be one-shot implemented correctly cold, (a) when the
   ordering lives in a `design:` block bound by `USES`, and (b) when it lives as
   normative prose obligations in the coordinating spec (`kiln.fire`), the
   strongest expression current v1 allows? Rounds 02/05 refuted per-call and
   fine-grained sequencing *inside an owning spec*; the multi-spec lifecycle
   with no natural owning symbol is the residue those rounds never tested.

2. **`dataflow-invisible` + `trust-boundary-violation-residual`** (both
   RESOLVED — re-validation, not reopening). The resolved fixes were validated
   with `USES <producer>::RETURN` carrying the dataflow pointer (rounds 02–03,
   4/4). DESIGN-BLOCKS migrates that pointer to slot-targeted
   `CONFORMS: <producer>::RETURN`. Deciding question: does the
   producer-pointer + stated-trust-boundary + violation-residual pattern
   reproduce 4/4 with `CONFORMS` carrying the pointer (arm A), while the
   current `USES` form re-verifies as regression (arm B)? A ≥2-model divergence
   in arm A only → the CONFORMS migration (DESIGN-BLOCKS migration item 1) is
   rejected even if the design block itself works.

3. **USES-repurposing machine-checkability** (the relation/target-kind
   partition: `USES`→design only, `CONFORMS`→spec/slot only, `SEE`→either).
   Not panel-graded — verified **locally** by the ref-checker: arm A must
   contain zero spec→spec `USES`, every arm-A `USES` target must resolve to a
   design block, and every `CONFORMS` target to a spec/slot. This demonstrates
   the partition is expressible and checkable; the panel measures only whether
   it is *legible*.

## Two-arm structure and what each outcome combination MEANS

Both arms pin the **same observable behavior byte-for-byte** and are graded by
the **same oracle**. Panel: 4 models × 2 arms = 8 cold, isolated runs (unique
`/tmp` workspace each, spec files only, generic `experiment/cold-prompt.md`).
Per the standing rule, a miss shared by **≥2 models** in the same arm is a
spec-defect signal; a single-model miss is a model-error. The deciding
measurement is the **lifecycle-ordering batches** (a wrong stage order cannot
pass them) plus NOTES-reported confusion about the ordering source.

| Arm A (design block) | Arm B (current v1) | Meaning / action |
|---|---|---|
| pass (4/4 order correct) | **fail** (≥2 models mis-order) | **Correctness need confirmed**: current language cannot carry a multi-spec lifecycle even at its best; the `design:` construct fixes it and is cold-legible. Adopt the construct (execute DESIGN-BLOCKS blast radius) — contingent on the arm-A dataflow re-validation also holding. |
| pass | pass | **Ergonomic only**: the strongest current expression already one-shots. Per the discipline that closed five prior findings, the lifecycle case is `wontfix` as a correctness motivation; DESIGN-BLOCKS loses its experimental justification and must stand or fall on non-correctness grounds outside this experiment's charter. `slot-model-for-non-functional-specs` closes refuted for the lifecycle case. |
| **fail** (≥2 mis-order) | pass | The proposed construct is **not cold-legible** while plain prose is — the design block *hurts*. Rework or drop the proposal; finding stays open with new evidence. |
| fail | fail | The lifecycle gap is real but the proposed construct does not fix it. New actionable finding; rework the construct (the ordering content, the binding relation, or both). |
| A dataflow batches fail (≥2) regardless of order result | — | The `CONFORMS: <producer>::RETURN` migration breaks the resolved dataflow reading → reject migration item 1 (keep `USES` for dataflow) even if the design block itself is adopted. |

## Domain

**Kiln-firing controller** (`kiln`). Fresh, non-famous; prior rounds used
berth, apiary, vault, axle, weighbridge. Deterministic integer arithmetic; no
clock/network/randomness. A pottery kiln controller processes a load manifest;
each accepted load runs a four-stage firing procedure; each stage adjusts a
running **firing index** and logs one line, so both the stage-line sequence and
every logged value observably depend on stage order.

**The mandated order is deliberately non-obvious: SOAK → SEAL → VENT → RAMP.**
A model that does not read the ordering source will fall back on a prior, and
every plausible prior yields a different, failing trace:

- alphabetical / stage-spec documentation order: RAMP, SEAL, SOAK, VENT
- reverse documentation order: VENT, SOAK, SEAL, RAMP
- domain intuition (heat up, hold, vent, finish): VENT, RAMP, SOAK, SEAL

Stage arithmetic is non-commutative (two multiplies, an add, a subtract), so a
wrong order breaks the logged values as well as the label sequence — a model
that computes in one order but prints labels in another still fails:

| stage | operation on the running index |
|-------|-------------------------------|
| SOAK  | × 3   |
| SEAL  | − 220 |
| VENT  | + 35  |
| RAMP  | × 2   |

Correct final index = 6·base − 370. Rival-order finals for base 200: mandated
830; alphabetical 575; reverse-doc 970; intuition 1190 — all distinct (asserted
by the oracle self-check for every batch base).

The ordering source appears in **exactly one place per arm**. Every other
mention of the stages (stage-spec documentation order, report's token
enumeration) uses alphabetical order, and both arms' stage-file preambles state
explicitly that documentation order is not execution order.

## CLI shape

One binary `kiln`; `argv[1]` selects from the closed set `{fire, report}`.

- **`kiln fire`** — reads a load manifest (`LOAD <id> <base>` records) from
  stdin; per accepted record emits four stage lines + one grade line to stdout;
  per rejected record one error line to stderr; exit = max contribution.
- **`kiln report`** — the consumer of `kiln fire | kiln report`: classifies
  each line by its second field alone (trusting fire's text), writes a
  five-line tally, exit 0.
- Dispatch residual: missing / unknown / wrong-case (`FIRE`/`REPORT`) /
  bare `-` argv[1] → `usage: kiln {fire|report}\n` to stderr, exit 2, no stdout.

## Specs and file layout

Ten specs across four files, identical in both arms except the deltas listed
below. Layout (`test-specs/round-06-kiln/spec-arm-{a,b}/`):

### `kiln.shared.yass.yaml`
- **Preamble.**
- **`kiln`** — dispatch over the closed set + usage residual; global
  line-termination / byte-echo invariants; a `SHOULD` (output-as-you-go) for
  the normativity gradient.
- **`kiln.lines`** — segmentation, the full GUIDANCE checklist: raw bytes,
  empty input = zero records, strip exactly one trailing LF, LF record split,
  lone `"\n"` = one zero-field record, blank interior = zero-field record,
  ASCII-space field runs, tab/CR are data.
- **`kiln.record`** — `LOAD <id> <base>` format; ordered error checks
  E10/E25/E15/E20 (registry below); error-line shape; at-most-one-error-line;
  exhaustiveness invariant (**no dead residual** — `residual-reachability`
  regression).
- **`kiln.grade`** — grade registry on the final firing index: G90/G10/DONE,
  exit contributions, exhaustive over all integers (all three branches guarded
  + exhaustiveness asserted).
- **(arm A only)** **`design: FiringSequence`** — `type: ordered-steps`,
  `content` block scalar carrying the numbered stage order and the
  index-threading rule. The sole statement of the order in arm A.

### `kiln.stages.yass.yaml`
- **`kiln.ramp`, `kiln.seal`, `kiln.soak`, `kiln.vent`** — documented in
  **alphabetical** order (≠ execution order, by design). Each: INPUT (receives
  id + running index at *its position in the procedure* — position stated
  nowhere in the stage spec), RETURN (its arithmetic + its
  `<id> <STAGE> <index>` line), SIDE-EFFECT (never errs). Each stage's INPUT
  carries the arm's ordering-source ref.

### `kiln.fire.yass.yaml`
- **`kiln.fire`** — the producer. INPUT: segmentation + record format via
  slot-targeted `CONFORMS` (both arms). RETURN: run the procedure per the
  arm's ordering source; thread the index; grade via
  `CONFORMS …@kiln.grade::INVARIANT`; per-record grouping (no interleaving);
  exit = max contribution. ERROR: reject per shared registry
  (`CONFORMS …@kiln.record::ERROR`), exhaustive set, no residual.
  SIDE-EFFECT + INVARIANT (exact possibly-negative integers; exactly five
  stdout lines per accepted record).

### `kiln.report.yass.yaml`
- **`kiln.report`** — the consumer. INPUT: the arm's dataflow pointer at
  `kiln.fire::RETURN` + stated trust boundary (relies on id-first/token-second
  line shape; never re-validates ids, field counts, or indexes, never
  re-derives arithmetic or grades) + **violation residual** (out-of-contract
  line → counted in LINES, no category, no error) + closed-set token
  classification (alphabetical enumeration). RETURN: exactly five lines
  `LINES/STAGES/DONE/UNDERFIRED/OVERFIRED`, exit 0.

### Arm deltas (everything else is byte-identical)

| site | Arm A (proposed language) | Arm B (current v1, strongest form) |
|---|---|---|
| ordering carrier | `design: FiringSequence` document in `kiln.shared.yass.yaml`; `kiln.fire` RETURN says "in exactly the order the FiringSequence design states — the order is stated there and nowhere else" | no design doc; `kiln.fire` RETURN states the order inline as normative prose ("first SOAK, then SEAL, then VENT, then RAMP …") |
| binding refs | `USES: ./kiln.shared@FiringSequence` from `kiln.fire` + each stage spec; bare same-file `USES: FiringSequence` from `kiln.grade`; `SEE: ./kiln.shared@FiringSequence` from `kiln.report` (SEE→design) | `USES: ./kiln.fire@kiln.fire` from each stage spec + `kiln.grade` (cross-cutting-concern backlink per GUIDANCE) |
| dataflow pointer (report INPUT) | `CONFORMS: ./kiln.fire@kiln.fire::RETURN` | `USES: ./kiln.fire@kiln.fire::RETURN` |
| fire → stage links | ref-only `SEE:` ×4 (structural nav per the proposal) | ref-only `USES:` ×4 (current "draws on") |
| spec→spec USES | **none anywhere** (validated) | per current rules |

Both arms keep the same slot-targeted `CONFORMS` for segmentation
(`kiln.lines::INPUT`), record format (`kiln.record::INPUT`/`::ERROR`), and the
grade registry (`kiln.grade::INVARIANT`) — legal today and unchanged under the
proposal, minimizing confounds: the arms differ **only** in the ordering
carrier and the relation kinds under test.

## Pinned error registry

Dispatch residual: `usage: kiln {fire|report}` → stderr, exit 2, no stdout.

`kiln fire` per-record checks, in order, stop at first failure, exactly one
error line per rejected record, no stdout line for it (contribution 1):

1. 0 fields (blank/whitespace-only) → `E10 malformed record: empty line`
2. field 1 ≠ `LOAD` → `E25 unknown operation: <field1>`
3. `LOAD` and field count ≠ 3 → `E10 malformed record: LOAD expects 3 fields, got <n>`
4. field 2 not a well-formed id (2–8 chars, first A–Z, all A–Z0–9) → `E15 bad load id: <field2>`
5. field 3 not a well-formed non-negative integer (digit run, no leading zero unless `0`) → `E20 bad number: <field3>`

Grade registry (final index T = value RAMP produces):

| grade | guard | line | exit contribution |
|-------|-------|------|------------------:|
| G90 | T > 2000 | `<id> G90 overfired above 2000` | 3 |
| G10 | T < 500  | `<id> G10 underfired below 500`  | 2 |
| DONE | 500 ≤ T ≤ 2000 | `<id> DONE <T>` | 0 |

Rejected record contributes 1. Exit = max contribution; empty input → exit 0.

`kiln report` classification by second field (case-sensitive, byte-exact):
`RAMP|SEAL|SOAK|VENT` → STAGES; `DONE` → DONE; `G10` → UNDERFIRED; `G90` →
OVERFIRED; anything else (or <2 fields, incl. blank line) → LINES only. Output
exactly `LINES/STAGES/DONE/UNDERFIRED/OVERFIRED` with counts; always exit 0.

## Oracle batch design (`test-specs/round-06-kiln/oracle/grade.py`)

One oracle grades both arms (byte-identical contract). Same shape as rounds
04/05: embedded independent simulator (data-driven op table; `ref.py` uses
explicit sequential statements), hand-pinned `(name, sub, stdin, stdout,
stderr, exit)` batches, byte-exact via surrogateescape, per-batch FAIL diffs,
`--self-check` → `SELFTEST OK`, `--cmd` → `SCORE: n/m`. ~48 batches:

- **DISPATCH (5):** missing / unknown / `FIRE` / `REPORT` / bare `-` → usage, 2.
- **FIRE segmentation (9):** empty input; trailing-LF strip; missing trailing
  LF; double trailing LF (second → E10 empty record); blank interior; lone
  `"\n"`; tab-as-data (field count drops → E10); CR-as-data (`E20 bad number:
  200\r`); space-run collapse.
- **FIRE record format (10):** field count high/low; E25 unknown keyword; E25
  on a 1-field record (E25 precedes the count check); bad ids (lowercase,
  1-char, 9-char); bad numbers (leading zero, non-digit); E15-before-E20.
- **FIRE lifecycle ordering (THE PROBE, 10):** full five-line traces
  byte-pinned for bases 200 (DONE 830), 0 (negative intermediates −220/−185,
  final −370 → G10), 145/144 (DONE 500 / G10 494 boundary), 395/396 (DONE
  2000 / G90 2006 boundary), 1000 (G90); two-record grouping (no
  interleaving); error-between-records; exit-policy maxima (error vs G90 → 3;
  G10 vs error → 2). Any wrong stage order fails every trace batch — labels
  AND values.
- **REPORT / dataflow trust boundary (14):** empty; exact `fire` output piped
  in (single + mixed); G90 line; trusted out-of-contract id (`zz DONE 700` —
  violation input, counted DONE, not re-validated); trusted wrong field count
  (`K1 DONE 700 EXTRA`); unknown token; one-field line; blank line counts
  toward LINES only; lone `"\n"`; missing trailing LF; all four stage tokens
  (fed alphabetically — classification is order-insensitive); lowercase token
  (case-sensitivity); tab-as-data.

Self-check additionally asserts: mandated order ≠ each rival prior; rival-order
traces differ from mandated for every batch base; grade boundary truth table;
stage-token/registry consistency; simulator reproduces every hand-pinned batch.

`oracle/ref.py`: independent reference implementation (straight-line per-stage
statements + if-ladders, no shared code or data tables with grade.py); must
score full before any panel run. `oracle/HOWTORUN.txt` documents both.

## Regression surface (carried, not asserted)

- `closed-set-dispatch-residual` — dispatch batches.
- `input-segmentation-completeness` + `segmentation-terminator-mechanics` —
  segmentation batches per the GUIDANCE checklist.
- `residual-reachability` — `kiln.record`/`kiln.fire`/`kiln.grade` all assert
  exhaustiveness instead of stating a dead catch-all.
- `dataflow-invisible` + `trust-boundary-violation-residual` — report's trust
  boundary + violation batches (arm B re-verifies as-resolved; arm A tests the
  migrated carrier).
- `cross-cutting-single-home` — `kiln.shared` owns protocol/format/registry.
- slot-targeted CONFORMS transclusion + whole-registry conformance — fire and
  report both carry `CONFORMS …::INVARIANT`/`::INPUT`/`::ERROR` refs.
- normativity gradient — MUST/MUST-NOT throughout; SHOULD (`kiln`),
  SHOULD-NOT (`kiln.fire` SIDE-EFFECT), MAY (`kiln.report` hand-fed lines).
- Five-slot coverage — all five slots appear across the set (INPUT/RETURN/
  ERROR/SIDE-EFFECT/INVARIANT).

## Procedure

1. Author both arms + oracle + ref + HOWTORUN from this PLAN. **Validate:**
   `/tmp/check_refs_r06.py` (every ref target in both arms resolves; arm A
   `USES`→design-only and zero spec→spec USES; arm B all-spec/slot targets, no
   design docs) → `RESULT: CLEAN`; `grade.py --self-check` → `SELFTEST OK`;
   `grade.py --cmd "python3 …/ref.py"` → full score. Commit before any panel
   run.
2. Run the panel cold: 4 models × 2 arms, unique `/tmp/yass-r06-<arm>-<model>-<rand>`
   workspaces, spec files only, `experiment/cold-prompt.md`, concurrent
   background runs.
3. Grade all 8 candidates with the one oracle; capture NOTES.md.
4. Diagnose per the outcome matrix above (≥2-model corroboration per arm;
   NOTES ambiguity flagged by ≥2 = latent defect; 4/4-clean = evidence against).
5. Fix source of truth per the matrix (adopt / wontfix / rework DESIGN-BLOCKS);
   update FINDINGS.md; commit round-06.

## Success criteria

- **Lifecycle probe:** graded solely by the trace batches + NOTES. Arm A
  4/4-clean AND arm B ≥2-model order misses = adopt; both clean = wontfix
  (ergonomic); arm A ≥2 misses = proposal not cold-legible, rework.
- **Dataflow re-validation:** arm A report batches (trust + violation) must be
  4/4-clean for the CONFORMS migration to stand; arm B re-verifies the
  resolved findings.
- **Machine-checkability:** ref-checker CLEAN on both arms, with the arm-A
  target-kind partition enforced, before any panel run.
