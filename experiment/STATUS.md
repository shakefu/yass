# STATUS

- **Round:** 0 (setup complete)
- **Phase:** scaffolding committed; about to start Round 1
- **Next action:** Round 1 — Plan + author spec/oracle. Probe the highest-recurrence
  spec-definition findings, leading with the structured-obligation cluster
  (`mapping-valued-obligations`, `error-table-structured`, `priority-chains-prose`)
  and `dataflow-invisible` / `cross-spec-sequencing`. CLI-shaped spec in a fresh domain.
- **Open findings:** 26 (see FINDINGS.md). Resolved: 0.
- **Rounds remaining to checkpoint:** 5.
- **Convergence counter (rounds with no new actionable findings):** 0 / 2.

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
