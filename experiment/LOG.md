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

_(Round 1 begins next.)_
