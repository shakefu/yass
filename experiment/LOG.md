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

_(Round 2 begins next: scale spec complexity or pivot to the dataflow/sequencing
cluster.)_
