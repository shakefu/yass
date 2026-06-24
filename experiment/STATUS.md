# STATUS

- **Round:** 1 (in progress)
- **Phase:** Step 3/4 — panel LAUNCHED (background), awaiting completion + grading.
  Harness validated (smoke: gpt 9/9). PLAN.md written. Probe spec + oracle committed
  (2d7bab9). 4 agents running cold in unique `/tmp` workspaces:
  `/tmp/yass-r01-berth-{gpt,gemini,opus,composer}` via `script/agent` +
  `experiment/round-01/prompt.md`.
- **Next action:** When each agent finishes, grade from inside its workspace:
  `python3 /Users/shakefu/git/shakefu/yass/test-specs/round-01-berth/oracle/grade.py --cmd "<RUN line from that workspace's HOWTORUN.txt>"` (build first if HOWTORUN lists build steps).
  Read each `NOTES.md`. Diagnose every miss spec-defect vs model-error (≥2 models = strong
  signal), write `experiment/round-01/results.md`, then apply source-of-truth fixes.
  Relaunch a model if needed:
  `script/agent -m <alias> --spec test-specs/round-01-berth/spec/berth.yass.yaml --spec test-specs/round-01-berth/spec/berth.reference.yass.yaml --workspace /tmp/yass-r01-berth-<model> --prompt-file experiment/round-01/prompt.md`
- **Open findings:** 26 (see FINDINGS.md). Resolved: 0.
- **Rounds remaining to checkpoint:** 5.
- **Convergence counter (rounds with no new actionable findings):** 0 / 2.

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
