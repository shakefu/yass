/goal Run the autonomous yass spec-improvement experiment on the goal-experiment branch.

GOAL.md is the authority — read it top to bottom first. It defines the mission, the operating parameters, the per-round loop, the model panel, and the halt/convergence checkpoint. Do not contradict it.

Resume or start fresh, depending on branch state:
- If a run is already in progress (experiment/STATUS.md exists), follow the resume protocol: read experiment/STATUS.md, then experiment/FINDINGS.md, then the latest "## Round NN" section of experiment/LOG.md, and continue the loop from the exact phase STATUS.md records — do not restart work already done.
- If no run is in progress (no experiment/STATUS.md), start a fresh run: create experiment/STATUS.md and experiment/LOG.md, seed the round plan from the OPEN findings in experiment/FINDINGS.md (plus fresh exploratory probes), and begin at Round 1.

Per-round loop (full detail in GOAL.md): plan the round from the open findings (lead with the highest-recurrence ones; always re-verify prior-round fixes for regression); author a fresh-domain, CLI-shaped synthetic spec plus a private acceptance oracle under test-specs/round-NN-<name>/ and commit them before running anything; run the full model panel COLD, each model in its own unique /tmp workspace via script/agent using the generic cold prompt at experiment/cold-prompt.md — copy in ONLY the spec/*.yass.yaml files, the private oracle is NEVER copied into any agent workspace; build each impl per its HOWTORUN.txt and grade it black-box against the oracle (stdin → stdout/stderr/exit-code, byte-exact); diagnose every miss and every NOTES-level ambiguity as spec-defect vs model-error with cross-model corroboration (a miss/ambiguity shared by ≥2 models is a strong spec-defect signal); apply confirmed spec-defect fixes to the source-of-truth files (yass.yass.yaml, yass.v1.schema.json, context/GUIDANCE.md, context/yass-reference.md, keeping spec/*.yass.yaml consistent); update the experiment/FINDINGS.md ledger (mark resolved, add newly discovered), prune resolved content out of the context/*.md feedback docs by deletion — but NEVER edit the raw context/*-FEEDBACK.md transcripts — and route irreducible tooling-only asks to experiment/TOOLING.md.

On the first round of a fresh run, smoke-test the harness end-to-end with one tiny spec and one model before expanding to the full panel.

Commit to goal-experiment after every phase (small, frequent — compaction can happen at any time), and keep experiment/STATUS.md + experiment/LOG.md current so the next iteration can resume cleanly.

Stop and report when Round 5 completes (the checkpoint). Also stop early and report if 2 consecutive rounds produce no new actionable spec-defect findings (convergence). When you halt, summarize: findings resolved vs still open, source-of-truth changes made, and whether to continue past the checkpoint.
