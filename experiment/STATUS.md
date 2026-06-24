# STATUS

- **Round:** 1 (in progress)
- **Phase:** Step 3 — panel run. Harness validated (smoke: gpt 9/9). PLAN.md written
  (`experiment/round-01/PLAN.md`). Probe spec + oracle authored and committed (2d7bab9):
  `test-specs/round-01-berth/spec/{berth,berth.reference}.yass.yaml` and
  `oracle/grade.py` (reference SELFTEST OK). Prompt ready: `experiment/round-01/prompt.md`.
- **Next action:** Run the 4-model panel COLD on the berth spec — unique `/tmp` workspace
  per model, copy in ONLY `spec/*.yass.yaml` (oracle NEVER copied), background + concurrent,
  via `script/agent` with `experiment/round-01/prompt.md`. Then grade each (build per
  HOWTORUN.txt → run oracle), diagnose spec-defect vs model-error, write
  `experiment/round-01/results.md`. Run command per model:
  `script/agent -m <alias> --spec test-specs/round-01-berth/spec/berth.yass.yaml --spec test-specs/round-01-berth/spec/berth.reference.yass.yaml --workspace /tmp/yass-r01-<model>-<rand> --prompt-file experiment/round-01/prompt.md`
  Grade: `python3 test-specs/round-01-berth/oracle/grade.py --cmd "<RUN line from workspace HOWTORUN.txt>"` (run from inside each workspace).
- **Open findings:** 26 (see FINDINGS.md). Resolved: 0.
- **Rounds remaining to checkpoint:** 5.
- **Convergence counter (rounds with no new actionable findings):** 0 / 2.

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
