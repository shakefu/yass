# STATUS

- **Round:** 3 (COMPLETE — all 7 steps done).
- **Phase:** Round 3 closed. Vault probe (`vault.shared`/`vault.certify`/`vault.report`,
  7 documents) ran the panel cold; all four models scored **35/35 (140/140, zero
  functional misses)**. The probe drove the structured-obligation cluster to an adversarial
  scale (18-row prose defect registry, deliberately non-monotonic 18-entry precedence chain,
  one-verdict-per-record cardinality, bare V01–V18 code literals read in two places,
  certify→report dataflow trust boundary, off-spec segmentation). Outcome: the cluster's
  hypothesized **correctness** defect is **refuted ×2** and closed `wontfix` (5 ids); its
  irreducible ergonomic/machine-checkability residue is routed to `lint-anti-slop` in
  `experiment/TOOLING.md`. Two new spec-defects resolved in the SOT.
- **Next action (Round 4):** pivot to the highest-leverage OPEN findings — composition /
  reference and the CONFORMS cluster: `cross-spec-sequencing` (REQUIRES/AFTER),
  `conforms-overloaded` / `conforms-inlining-semantics-misplaced` / `conforms-bare-slot-meaning`,
  and `self-validation-ref-bug`. Run the 7-step loop: plan from OPEN findings + re-verify
  prior fixes → `round-04/PLAN.md`; author a fresh-domain CLI probe + private oracle, commit
  before agents run; run the panel cold (unique `/tmp` per model, copy in ONLY
  `spec/*.yass.yaml`); grade; diagnose every miss spec-defect vs model-error (≥2 = strong);
  fix SOT; prune `context/*`; route tooling to TOOLING.md; commit; update this STATUS + LOG.
- **Findings:** 30 total. **Resolved (fixed in SOT): 7** (Round 1: `default-error-policy`,
  `input-segmentation-completeness`; Round 2: `dataflow-invisible`, `cross-cutting-single-home`,
  `closed-set-dispatch-residual`; **Round 3 (+2 new, both resolved): `trust-boundary-violation-residual`,
  `segmentation-terminator-mechanics`**). **Closed `wontfix` (refuted ×2): 5**
  (`mapping-valued-obligations`, `error-table-structured`, `error-code-refs`,
  `priority-chains-prose`, `error-cardinality-implicit`). **Open: 18** (see FINDINGS.md).
- **Rounds remaining to checkpoint:** 2 (halt after Round 5).
- **Convergence counter (rounds with no new actionable findings):** 0 / 2
  (Round 3 produced 2 new actionable findings, both resolved → counter stays 0. Note: the
  primary probe target — the structured-obligation cluster — is now exhausted/refuted, so
  Rounds 4–5 pivot to composition/reference and the CONFORMS cluster.)

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
