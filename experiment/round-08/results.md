# Round 08 results — crib probe (DESIGN-BLOCKS tech-constraint, live Postgres)

**Date:** 2026-07-22 · **Panel:** gpt-5.5-extra-high · gemini-3.1-pro ·
claude-opus-4-8-thinking-high · composer-2.5 · **4 models × 2 arms = 8 cold
runs.** Spec+oracle committed pre-panel (`caf118e`).

## Probe design

The third and last DESIGN-BLOCKS motivation — **technology/stack
constraints** — probed with the unfalsifiability excuse removed: the grader
provisions a live PostgreSQL (`postgres:16`, docker) per graded run, hands
each implementation a `CRIB_PGDSN`, and then **queries the database
directly**, so "the state actually lands in Postgres, and only there" is an
observable, gradeable outcome. Unlike rounds 06–07 (carrier swaps of
well-homed content), this round measured a **placement differential**: arm
B's home for a stack constraint is the contested one — SIDE-EFFECT/INVARIANT
prose on a `crib.store` spec, exactly the placement the proposal calls
"distorting the slot's meaning" — so this was the first probe where the
proposal predicted a *behavioral* difference.

Domain: tool-crib ledger (`crib post` / `crib report`), cross-process
persistence inherent, algorithmic load deliberately low. The constraint
(identical semantics, both arms): state in the Postgres at `CRIB_PGDSN`
(pinned URL form); idempotent creation of `crib_stock(tool_id, balance)`;
**MUST-NOT persist anywhere else**; **shared-table authority** (rows changed
by other clients between invocations are authoritative — this makes shadow
state behaviorally fatal, not merely inspection-detectable). Arm A:
`design: CribStore`, `type: stack`, bound by `USES`. Arm B: the same four
points as SIDE-EFFECT/INVARIANT obligations on `crib.store`. Everything else
byte-identical (diff-verified pre-panel).

Oracle: 31 batches / ~55 steps as sequences of RUN (separate process per
invocation, byte-exact), SQL (direct `SELECT` against `crib_stock` — the
compliance check), MUTATE (grader-driven UPDATE/INSERT/DELETE between
invocations), FILES (fresh scratch CWD must stay empty). `--self-check` →
`SELFTEST OK` (offline); ref.py 31/31 live; ref-check `RESULT: CLEAN` both
arms; plus a discrimination proof — a deliberately non-compliant shadow-state
implementation fails every SQL/MUTATE batch and the E51 residual while
passing behavior-only batches.

## Grades — 248/248, zero batch failures of any kind

| model | arm | language / driver | score | duration (s) | self-tested vs live DB? |
|-------|-----|-------------------|-------|--------------|--------------------------|
| gpt | A | Python / psycopg2 | 31/31 | 284 | no |
| gpt | B | Python / psycopg2 | 31/31 | 151 | no |
| gemini | A | Python / psycopg2 | 31/31 | 225 | no |
| gemini | B | Python / psycopg2 | 31/31 | 135 | no |
| opus | A | Go / pgx v5 | 31/31 | 369 | **yes** (left `crib-pg-test` postgres:16-alpine on :5433 — swept post-panel per the teardown note) |
| opus | B | Python / psycopg2 | 31/31 | 341 | **yes** (NOTES: "verified manually against a local postgres:16 instance") |
| composer | A | Go / pgx v5 | 31/31 | 101 | no |
| composer | B | Python / psycopg2 | 31/31 | 129 | no |

Go builds needed their build step and then scored full (grading procedure,
not a candidate failure). Self-provisioning was permitted per the PLAN and is
recorded as descriptive context only; the evidence is NOTES + the leftover
container (agent.log greps were uninformative). The model-error ledger is
empty: no misses at all.

## PRIMARY — compliance differential: 4/4 vs 4/4 → **NULL**

Every model in BOTH arms passed every direct-SQL batch (exact
`(tool_id, balance)` rows in the live database after every driven sequence),
every external-mutation batch (grader-changed rows reflected by the next
invocation — including the shadow-killer `mut_update_report_and_issue`,
where remembered state would answer `E30` instead of `AB1 OK 3`), and every
FILES batch (no state files in the scratch CWD). **Both arms fully
compliant.** The slot-distorted SIDE-EFFECT/INVARIANT home bound the
constraint exactly as well as the `design:` block. Pre-registered disposition
row 3 applies: **confirm both wontfix rows with the unfalsifiability excuse
removed.**

## SECONDARY — NOTES rubric: NULL under either scope (scope decision documented)

The pre-registered scope listed "the env-var contract" among constraint
topics, but the E50/E51 residual text lives in the shared `crib` spec —
**outside the arm deltas and byte-identical in both arms** — so items about
it cannot measure the carrier differential. Resolution: counts are reported
under BOTH scopes; the outcome is scope-invariant, so nothing turns on the
choice. (Author re-read all 8 NOTES.md and re-applied the rubric
independently; full agreement with the lead's provisional classification.)

**Narrow scope (constraint-carrier content only — the four points):**

- gemini-A: 1 HEDGE — shared-table authority vs intra-invocation races: "I
  assumed the strongest interpretation" (`SELECT … FOR UPDATE` against a
  race the spec never mentions).
- gemini-B: 1 HEDGE — commit scope: "I assumed committing after each record
  is the safest way" (point-3's per-record vs before-exit
  under-determination).
- All others 0: composer-A's transaction-boundaries item is a confident
  correct derivation (RESTATEMENT); gpt-A/gpt-B's `numeric`-vs-`bigint`
  column choice is permitted by "at least 64-bit range"
  (RESTATEMENT/derivation).

Hedgers: **arm A 1, arm B 1** — same model, same theme, both arms → NULL.

**Broad scope (adding the env-var / store-failure mapping):** one shared
theme dominates — *"the spec defines no code for a store failure AFTER a
successful connect; I mapped it to E51"* — raised by composer-A, opus-A
(also 64-bit-overflow→E51), gpt-B, opus-B, composer-B, plus composer-B's
whitespace-only-DSN edge. Hedgers: **arm A 3 (gemini, opus, composer), arm B
4 (all)** — neither the positive (≥2 with ≤1) nor the reverse threshold is
met → NULL.

### The shared E51-mapping theme — a round-spec authoring observation, NOT a carrier differential

Five models across BOTH arms flagged the same unpinned corner: a store
failure after a successful connect (failed `CREATE TABLE`, dropped
connection mid-`post`) has no defined code. All five resolved it identically
(map to `E51`, exit 2), there was **zero behavioral divergence**, and no
oracle batch induces a mid-run store failure. Because the gap sits in the
shared `crib` spec — outside the arm deltas, identical in both arms — it says
nothing about either carrier. Per the round-05 precedent (corroborated NOTES
flag + identical resolution + no batch dependence = probe-authoring
artifact): **the language already provides the needed idiom** (the residual
discipline — "every delegated check needs its failure case pinned",
GUIDANCE *Composition*/*Error obligations*); the round-08 spec author
under-applied it to the store-failure error set. Recorded here so it does
not silently vanish. Not a language defect; no FINDINGS row.

**Tightening applied post-run (lead-approved), per the round-05 precedent:**
the shared `crib` spec now carries the residual in BOTH arms, byte-identical
("WHEN the subcommand is valid and the connection succeeded but the store
fails afterward — a failed table creation, a lost connection, or any other
database error during processing → `E51 database unavailable` to stderr,
exit 2; output already written stands, no further records processed"). The
arm deltas are untouched (the new obligation appears in zero diff lines
between the arms); re-verified post-edit: ref-check `RESULT: CLEAN` both
arms, `--self-check` → `SELFTEST OK`. The oracle is unchanged and no batch
depends on the case, so no grade is affected.

## TERTIARY — duration (descriptive only, never decision-weight)

Arm A mean 245 s (284/225/369/101), arm B mean 189 s (151/135/341/129) —
**arm B faster 4/4 this round, the opposite direction from round-07** (arm A
faster 3/4 there), which itself supports the standing caveat that single-run
durations are noise-dominated. The two Go runs (both arm A) carried module
fetch + build time.

## Also re-exercised (incidental coverage)

- **Guarded USES→design** (round-07's secondary target): arm A's E50
  obligation is a guarded `WHEN … MUST … USES: CribStore` — read correctly
  by 4/4 (all E50/E51 batches clean), a second cold data point for the
  guard-scopes-binding rule.
- Standing regression surface held 8/8: closed-set dispatch residual,
  segmentation edges (tab/CR/blank/lone-LF/empty), exhaustive error set with
  no dead residual, byte-exact echoes, cross-cutting single home.

## Disposition — per pre-registered outcome table, row 3 (both clean + NULL)

- **`design-blocks-proposal` — wontfix CONFIRMED, now with its strongest
  close.** All three of the proposal's motivations are now probed: lifecycle
  procedures (round-06: correctness refuted), algorithms (round-07: clarity
  null under a pre-registered rubric), tech constraints (round-08: **the
  facet previously excused as unfalsifiable WAS falsifiable — live
  container, direct SQL — and no differential appeared**, 4/4 vs 4/4
  compliant). The construct remains validated as cold-legible and harmless
  across 12 arm-A runs; nothing in three rounds of evidence mandates it.
  What remains genuinely unfalsifiable by this method shrinks to the
  relation-disambiguation ergonomics and the dead-block lint.
- **`slot-model-for-non-functional-specs` — wontfix CONFIRMED.** The
  "slots distort non-functional content" claim was tested *behaviorally* for
  the first time: a binding stack constraint homed in SIDE-EFFECT/INVARIANT
  bound compliance exactly as well as the purpose-built document type.
- **FINDINGS hygiene applied:** both rows' "unfalsifiable by this method"
  wording is corrected — the tech-constraint facet was falsifiable and was
  probed.
- **Convergence:** round-08 produced **no new actionable spec-defect and no
  miss of any kind** — the **fourth consecutive no-new-findings round
  (05, 06, 07, 08)**. The K=2 convergence signal remains exceeded.
