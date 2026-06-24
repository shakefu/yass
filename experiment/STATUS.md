# STATUS

- **Round:** 3 (IN PROGRESS — Steps 1–2 done, PAUSED before panel run).
- **Phase:** Step 2 done — probe spec + private oracle authored, validated, committed.
  Specs (`vault.shared`/`vault.certify`/`vault.report`, 7 documents) all parse against
  `yass.v1.schema.json`; all 11 cross-file refs resolve; oracle `--self-check` prints
  SELFTEST OK; `grade.py --cmd <ref.py>` scores 35/35. **PAUSED on user instruction
  before running the panel; /goal cleared.** Probing the structured-obligation cluster
  at scale: 18-code prose defect registry, deliberately non-monotonic precedence chain,
  one-verdict-per-record cardinality, E10/E15/E20 ordered error checks, certify|report
  dataflow trust boundary, off-spec segmentation (tab/CRLF/NBSP). Findings under test:
  `priority-chains-prose`, `error-table-structured`, `error-cardinality-implicit`,
  `mapping-valued-obligations`, `error-code-refs`; regression re-verify of
  `dataflow-invisible`, `closed-set-dispatch-residual`, `input-segmentation-completeness`,
  `cross-cutting-single-home`.
- **Next action (Round 3, on resume):** Step 3 — run the panel cold (gpt/gemini/opus/
  composer), concurrent + background, unique `/tmp` workspace per model, copy in ONLY
  `test-specs/round-03-vault/spec/*.yass.yaml`, via
  `script/agent ... --prompt-file experiment/round-03/prompt.md`. Then Steps 4–7: grade
  all three axes, diagnose every miss spec-defect vs model-error (≥2-model = strong
  signal), write `experiment/round-03/results.md`, fix SOT, update FINDINGS, prune
  resolved `context/*`, route irreducible tooling to TOOLING.md, commit `round-03`,
  update this STATUS + LOG, update convergence counter. (Note: `/goal` was cleared on
  user instruction — resuming the experiment requires a fresh `/goal` invocation.)
- **Open findings:** 23 open (see FINDINGS.md). Resolved this round: 3
  (`dataflow-invisible`, `cross-cutting-single-home`, `closed-set-dispatch-residual`).
  Resolved total: 5 of 28.
- **Rounds remaining to checkpoint:** 3 (halt after Round 5).
- **Convergence counter (rounds with no new actionable findings):** 0 / 2
  (Round 2 produced new actionable findings — one strong corroboration resolved, one
  cross-cutting resolved, one new finding → counter stays 0).

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
