# STATUS

- **Round:** 4 (COMPLETE — all 7 steps done).
- **Phase:** Round 4 closed. Axle probe (`axle.shared`/`axle.audit`/`axle.roster`,
  7+ documents) ran the panel cold; gpt/gemini/opus scored **42/42**, composer **41/42**
  (**167/168**, one miss = model-error). The probe drove the **CONFORMS cluster** — never
  exercised before (Rounds 2–3 used `USES`/`SEE`) — across all three open CONFORMS findings:
  plain slot transclusion (E10/E15/E20 enforced 4/4), the guard-combination corners (outer
  carrier WHEN × inner H07 WHEN, AND-semantics held 4/4), and a bare whole-spec
  `CONFORMS: ./axle.audit@axle.audit` (read as whole-spec conformance 4/4). Outcomes:
  `conforms-overloaded` **refuted** (`wontfix`), `conforms-inlining-semantics-misplaced` and
  `conforms-bare-slot-meaning` **resolved** in the grammar's favor (whole-spec CONFORMS = a
  non-transcluded conformance reference; `conforms_no_slot` retired from `cli.errors`; no
  schema change). One **new** spec-defect surfaced unanimously (4/4): `residual-reachability`
  — the residual discipline encouraged a *dead* guard-less residual (axle `E90`, unreachable
  by construction); resolved in `yass.yass.yaml` `Slot.ERROR` + GUIDANCE + yass-reference.
  Edit set passed an adversarial coherence review (no contradiction, no dangling ref).
- **Next action (Round 5 — final scheduled round):** pivot to the remaining highest-leverage
  OPEN composition/reference findings (`cross-spec-sequencing` REQUIRES/AFTER;
  `reftarget-resolution-scattered`; `dispatch-subcommand-override`) and re-verify the round-04
  CONFORMS + residual-reachability fixes under load. Run the 7-step loop: plan from OPEN
  findings + re-verify prior fixes → `round-05/PLAN.md`; author a fresh-domain CLI probe +
  private oracle (add a blank-line / malformed-`RECHECK`-`Y` roster batch — the round-04
  probe-authoring gap), commit before agents run; run the panel cold (unique `/tmp` per model,
  copy in ONLY `spec/*.yass.yaml`); grade; diagnose every miss spec-defect vs model-error
  (≥2 = strong); fix SOT; prune `context/*`; route tooling to TOOLING.md; commit; update this
  STATUS + LOG. **HALT and report after Round 5** (or earlier on K=2 convergence).
- **Findings (full ledger, see FINDINGS.md):** **40 total** — **resolved (fixed in SOT): 11**
  (Round 1: `default-error-policy`, `input-segmentation-completeness`; Round 2:
  `dataflow-invisible`, `cross-cutting-single-home`, `closed-set-dispatch-residual`; Round 3:
  `trust-boundary-violation-residual`, `segmentation-terminator-mechanics`; **Round 4 (+4):
  `residual-reachability` (new), `conforms-inlining-semantics-misplaced`,
  `conforms-bare-slot-meaning`, `self-validation-ref-bug` (re-verified repo-wide)**).
  **Closed `wontfix` (refuted): 6** (`mapping-valued-obligations`, `error-table-structured`,
  `error-code-refs`, `priority-chains-prose`, `error-cardinality-implicit`; **Round 4:
  `conforms-overloaded`**). **Open: 23** (incl. tooling-request + backlog sections).
- **Rounds remaining to checkpoint:** 1 (halt after Round 5).
- **Convergence counter (rounds with no new actionable findings):** 0 / 2
  (Round 4 produced 1 new actionable finding — `residual-reachability` — so the counter stays
  0. The CONFORMS cluster is now exhausted: 2 resolved, 1 refuted. Round 5 pivots to
  composition/reference sequencing and re-verifies the round-04 fixes.)

## Model panel
gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high · composer-2.5

## Quick commands
- Run one model cold:
  `script/agent -m <alias> --spec test-specs/round-NN-<name>/spec/*.yass.yaml --workspace /tmp/yass-rNN-<model>-<rand> --prompt-file experiment/round-NN/prompt.md`
- Toolchains: go1.26, rust1.94 (cargo/rustc via rustup), python3.14; gtimeout, jq present.
