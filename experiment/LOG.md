# Experiment LOG

Chronological lab notebook. One `## Round NN` section per round: plan, spec,
panel results, diagnosis, fixes applied, findings delta.

## Round 0 — setup (2026-06-24)

- Wrote `GOAL.md` (durable north star) and `script/agent` (cold cursor-agent
  wrapper). Created `goal-experiment` branch.
- Ingested all `context/*.md` feedback into `experiment/FINDINGS.md`: 26 distinct
  findings after dedup. Highest-recurrence themes: error table being both best
  and worst artifact, priority chains as prose, dataflow/sequencing gaps, CONFORMS
  over-complexity, byte-exact messages, cross-cutting single-home, MUST-NOT
  undertesting.
- Confirmed toolchains: go1.26.3, rustc1.94.1/cargo, python3.14.5, cursor-agent
  2026.06.24, gtimeout, jq.
- Key design lock-ins: cold + isolated `/tmp` per agent; CLI-shaped synthetic
  specs for black-box oracle grading; private oracle never copied to agent;
  spec-defect vs model-error diagnosis with cross-model corroboration.

## Round 1 — structured-obligation cluster (2026-06-24)

- **Plan** (`round-01/PLAN.md`): target the structured-obligation cluster + the
  guard-less `default-error-policy` in one CLI-shaped probe.
- **Spec** (`test-specs/round-01-berth`): port-berth assignment CLI — prose error
  registry (E10–E90), 8-step prose priority chain, USES sequencing, positional
  fields in prose, guard-less E90 catch-all, per-record error cardinality, MUST-NOTs.
  Probe + reference + oracle committed (2d7bab9) before any agent ran.
- **Harness validation:** smoke (gpt, tiny spec) 9/9 before the full panel.
- **Panel (cold, isolated `/tmp`, oracle never copied in):** gpt 12/12 (109s),
  gemini 12/12 (135s), opus 12/12 (235s), composer 10/12 (175s); all exit 0.
- **Oracle hardening mid-round:** original 10 batches were all ASCII-space + LF, so
  composer's non-conformant `strings.Fields` (Unicode whitespace) + `\r`-stripping
  scored 10/10. Added `whitespace_tab` + `crlf_record` batches + SELFTEST entries
  (0de09cf); composer correctly dropped to 10/12, others held.
- **Diagnosis** (`round-01/results.md`): the cluster caused **zero functional
  misses** at this scale — inference-expensive, not incorrect. Two corroborated
  actionable findings: `default-error-policy` (4/4 had to infer the central rule
  routed to the guard-less catch-all) and `input-segmentation-completeness` (3/4
  flagged boundary ambiguity). composer's 2 misses are a model-error, not a
  spec-defect; the actionable item was the oracle gap (fixed).
- **Source-of-truth fixes:** `yass.yass.yaml` `Slot.ERROR` now defines guarded =
  specific failure, guard-less = residual; `yass-reference.md` documents it;
  `GUIDANCE.md` gains an error-residual section and an input-segmentation section.
  Pruned `RECOMMENDATIONS.md` Part 1 §4 (default-policy paragraph) + Part 2 §2.
- **Findings delta:** resolved `default-error-policy`; added + resolved
  `input-segmentation-completeness`; annotated 5 cluster/MUST-NOT findings
  `probed round-01`. Open: 25. Resolved total: 2.

## Round 2 — composition cluster: dataflow + cross-cutting (2026-06-24)

- **Plan** (`round-02/PLAN.md`): pivot to the dataflow/sequencing cluster. Target
  `dataflow-invisible` + `cross-spec-sequencing` (universal pair) and
  `cross-cutting-single-home`, with `default-error-policy` +
  `input-segmentation-completeness` regression coverage.
- **Spec** (`test-specs/round-02-apiary`): honey-apiary harvest pipeline — one CLI,
  three subcommands `tally`→`grade`→`pack` chained by shell pipes, argv[1] selects the
  stage. A single `honey.shared` spec owns the wire protocol (dispatch, segmentation,
  exit policy, byte-exact error lines); the three stage specs reference it via `USES`,
  and each downstream INPUT carries a `USES …@<prev>::RETURN` pointer to its producer.
  Probe + reference + oracle (38 batches) committed before any agent ran.
- **Panel (cold, isolated `/tmp`, oracle never copied in):** composer 38/38 (Go, ~70s),
  gpt 38/38 (Python, ~125s), gemini 38/38 (Python, ~159s), opus 38/38 (Python, ~242s)
  = **152/152, zero functional misses.** Every targeted obligation read correctly cold.
- **Diagnosis** (`round-02/results.md`): no functional miss to classify; the actionable
  signals were `NOTES.md` ambiguities the panel resolved by guessing alike.
  (1) `dataflow-invisible` STRONG (4/4) — grade/pack consume tally's output but the spec
  never states which upstream guarantees they trust vs re-validate; the `USES …::RETURN`
  pointer carried only structural meaning. (2) `closed-set-dispatch-residual` NEW (3/4) —
  the `{tally,grade,pack}` dispatch had no missing/unknown-argv[1] case; models diverged
  (composer usage+exit1; gpt/opus invented E00+exit2; gemini silent). (3)
  `cross-spec-sequencing` partial positive (4/4) — header-gate idiom expresses the
  dataflow case; re-scoped. (4) `cross-cutting-single-home` (4/4) — single-owner pattern
  worked but was untaught.
- **Source-of-truth fixes:** `yass.yass.yaml` — `Reference` gives a slot-targeted `USES`
  a dataflow reading; `Slot.INPUT` requires naming a producing slot + stating the trust
  boundary, and stating the out-of-set/missing case for a closed-set dispatch.
  `yass-reference.md` — References (slot-targeted USES dataflow reading) + Slots (residual
  generalized to closed-set dispatch). `GUIDANCE.md` — new "Closed-set dispatch" and
  "Composition: dataflow and cross-cutting concerns" sections. No schema change
  (`::SLOT` targets already valid). Pruned `RECOMMENDATIONS.md` Part 1 §3 (cross-cutting
  home) + Part 2 §2 (inputs/outputs name crossing data); renumbered remaining.
- **Findings delta:** resolved `dataflow-invisible`, `cross-cutting-single-home`; added +
  resolved `closed-set-dispatch-residual`; re-scoped `cross-spec-sequencing`; re-verified
  `default-error-policy` + `input-segmentation-completeness`; annotated `conforms-overloaded`
  not-exercised. Open: 23. Resolved total: 5 of 28. Convergence counter: 0/2.

_(Round 3 begins next: structured-obligation cluster at larger scale, `conforms-overloaded`,
or non-dataflow sequencing.)_
