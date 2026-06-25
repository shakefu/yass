# STATUS

- **Round:** 5 (COMPLETE — all 7 steps done). **EXPERIMENT HALTED — final scheduled round reached.**
- **Phase:** Round 5 closed. Scale probe (truck weighbridge: `scale.shared`/`scale.tare`/
  `scale.weigh`/`scale.run`/`scale.audit`, five specs, two subcommands `run`/`audit`) ran the
  panel cold; **all four models scored 49/49 = 196/196, zero functional misses** (gpt/gemini/opus
  Python, composer Go). The probe targeted the remaining open composition/reference findings:
  - `cross-spec-sequencing` — **refuted** (`wontfix`). The WEIGH-after-TARE per-identifier
    stateful precondition (non-dataflow), expressed with normative prose + `USES ./scale.tare`
    and deliberately no REQUIRES/AFTER key, was implemented correctly 4/4 with zero NOTES
    confusion. Combined with round-02's dataflow case, **both** sequencing forms are expressible
    today; no REQUIRES/AFTER construct is warranted.
  - `dispatch-subcommand-override` — **refuted** (`wontfix`). `scale.audit`'s scoped prose
    override of the shared blank-line segmentation rule (plus a whole-spec
    `CONFORMS ./scale.run@scale.run`) was read correctly 4/4; bare-`-` dispatch handled via the
    `disp_bare_dash` batch. A scoped prose obligation that names what it overrides suffices; no
    OVERRIDES construct needed. **Re-verifies round-04** whole-spec-CONFORMS + guard-conjunction
    + residual-reachability under load (all held 4/4).
  - `reftarget-resolution-scattered` — **resolved**. `yass.yass.yaml` `RefTarget` confirmed sole
    canonical owner; the two genuine restatements consolidated to cite it without behavior change
    — `cli.validate` ERROR (literal charset regex → defer to RefTarget grammar + `SEE
    yass@RefTarget::ERROR`) and `cli.query` INVARIANT (resolution restatement → defer +
    `SEE yass@RefTarget::RETURN`). Ref-check CLEAN at 150 refs. Spec-name grammar and
    `.yass.yaml`-suffix/`FindProjectRoot` deliberately left (different owners / separate concerns).
  - Negative-net audit REWEIGH (4/4 NOTES) — **probe-authoring artifact, not a finding**: all four
    resolved it identically to the oracle (tally CLEAR), zero behavioral divergence, no batch
    exercises it. Probe tightened post-run for self-consistency; grades unaffected.
- **Next action:** none. **HALT and report.** Round 5 was the final scheduled round and produced
  no new actionable spec defect.
- **Findings (full ledger, see FINDINGS.md):** **40 total** — **resolved (fixed in SOT): 12**
  (Round 1: `default-error-policy`, `input-segmentation-completeness`; Round 2:
  `dataflow-invisible`, `cross-cutting-single-home`, `closed-set-dispatch-residual`; Round 3:
  `trust-boundary-violation-residual`, `segmentation-terminator-mechanics`; Round 4:
  `residual-reachability` (new), `conforms-inlining-semantics-misplaced`,
  `conforms-bare-slot-meaning`, `self-validation-ref-bug` (re-verified repo-wide); **Round 5 (+1):
  `reftarget-resolution-scattered`**). **Closed `wontfix` (refuted): 8**
  (`mapping-valued-obligations`, `error-table-structured`, `error-code-refs`,
  `priority-chains-prose`, `error-cardinality-implicit`; Round 4: `conforms-overloaded`;
  **Round 5: `cross-spec-sequencing`, `dispatch-subcommand-override`**). **Open: 20** (incl.
  tooling-request + backlog sections; no remaining open finding is an actionable spec defect with
  cross-model corroboration).
- **Rounds remaining to checkpoint:** 0 (halted after Round 5).
- **Convergence counter (rounds with no new actionable findings):** **1 / 2** (Round 5 produced no
  new actionable finding — both construct-request probes refuted, the one resolved finding was a
  pre-existing documentation item, the negative-net ambiguity was an authoring artifact. The
  experiment halts on the scheduled-round limit before the counter reaches 2.)

## Outcome summary (5 rounds)

- 40 findings triaged: **12 resolved** in source-of-truth, **8 refuted** (`wontfix`), **20 open**
  (tooling requests + backlog; none an actionable, cross-model-corroborated spec defect).
- Every round's panel scored at or near 100% against a private black-box oracle; the experiment's
  value was in the **NOTES-level ambiguities** and **construct requests**, each traced to
  spec-defect vs model-error and resolved at the source.
- Net language changes: `default-error-policy`/`residual-reachability` (residual discipline + its
  reachability converse), dataflow/cross-cutting/segmentation/trust-boundary obligations, the
  CONFORMS semantics cluster (slot-targeted = transcluded, whole-spec = non-transcluded
  conformance ref, guard-conjunction), and ref-target single-ownership consolidation. No
  REQUIRES/AFTER, no OVERRIDES, no `intent:`/ordered-obligations construct was found necessary for
  cold one-shot correctness.

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
