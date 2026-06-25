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

## Round 2 — composition cluster: dataflow + cross-cutting (2026-06-24)

- **Plan** (`round-02/PLAN.md`): pivot to the dataflow/sequencing cluster. Target
  `dataflow-invisible` + `cross-spec-sequencing` (universal pair) and
  `cross-cutting-single-home`, with `default-error-policy` +
  `input-segmentation-completeness` regression coverage.
- **Spec** (`test-specs/round-02-apiary`): honey-apiary harvest pipeline — one CLI,
  three subcommands `tally`→`grade`→`pack` chained by shell pipes, argv[1] selects the
  stage. A single `honey.shared` spec owns the wire protocol (dispatch, segmentation,
  exit policy, byte-exact error lines); the three stage specs reference it via `USES`,
  and each downstream INPUT carries a `USES …@<prev>::RETURN` pointer to its producer.
  Probe + reference + oracle (38 batches) committed before any agent ran.
- **Panel (cold, isolated `/tmp`, oracle never copied in):** composer 38/38 (Go, ~70s),
  gpt 38/38 (Python, ~125s), gemini 38/38 (Python, ~159s), opus 38/38 (Python, ~242s)
  = **152/152, zero functional misses.** Every targeted obligation read correctly cold.
- **Diagnosis** (`round-02/results.md`): no functional miss to classify; the actionable
  signals were `NOTES.md` ambiguities the panel resolved by guessing alike.
  (1) `dataflow-invisible` STRONG (4/4) — grade/pack consume tally's output but the spec
  never states which upstream guarantees they trust vs re-validate; the `USES …::RETURN`
  pointer carried only structural meaning. (2) `closed-set-dispatch-residual` NEW (3/4) —
  the `{tally,grade,pack}` dispatch had no missing/unknown-argv[1] case; models diverged
  (composer usage+exit1; gpt/opus invented E00+exit2; gemini silent). (3)
  `cross-spec-sequencing` partial positive (4/4) — header-gate idiom expresses the
  dataflow case; re-scoped. (4) `cross-cutting-single-home` (4/4) — single-owner pattern
  worked but was untaught.
- **Source-of-truth fixes:** `yass.yass.yaml` — `Reference` gives a slot-targeted `USES`
  a dataflow reading; `Slot.INPUT` requires naming a producing slot + stating the trust
  boundary, and stating the out-of-set/missing case for a closed-set dispatch.
  `yass-reference.md` — References (slot-targeted USES dataflow reading) + Slots (residual
  generalized to closed-set dispatch). `GUIDANCE.md` — new "Closed-set dispatch" and
  "Composition: dataflow and cross-cutting concerns" sections. No schema change
  (`::SLOT` targets already valid). Pruned `RECOMMENDATIONS.md` Part 1 §3 (cross-cutting
  home) + Part 2 §2 (inputs/outputs name crossing data); renumbered remaining.
- **Findings delta:** resolved `dataflow-invisible`, `cross-cutting-single-home`; added +
  resolved `closed-set-dispatch-residual`; re-scoped `cross-spec-sequencing`; re-verified
  `default-error-policy` + `input-segmentation-completeness`; annotated `conforms-overloaded`
  not-exercised. Open: 23. Resolved total: 5 of 28. Convergence counter: 0/2.

## Round 3 — structured-obligation cluster at adversarial scale (2026-06-24)

- **Plan** (`round-03/PLAN.md`): drive the structured-obligation cluster
  (`priority-chains-prose`, `error-table-structured`, `error-cardinality-implicit`,
  `mapping-valued-obligations`, `error-code-refs`) to a scale where, if a prose-only
  obligation shape actually causes one-shot failures, it would show. Re-verify the four
  Round-1/2 composition fixes under load.
- **Spec** (`test-specs/round-03-vault`): bank-vault time-lock door certification CLI —
  `certify` and `report` subcommands over a shared `vault.shared` spec. Deliberately
  adversarial: an **18-row** prose defect registry (code V01–V18, class, condition,
  byte-exact message); an **18-entry precedence order made non-monotonic** against both
  severity class and code number (so "most severe wins" and "lowest code wins" both
  diverge from the stated order — the deciding measurement); one-verdict-per-door and
  one-error-line-per-record cardinality; bare V-code literals read in two places
  (certify verdict emission + report class tally); a certify→report dataflow trust
  boundary; off-spec segmentation (tab / CRLF / NBSP). Probe + reference + oracle
  (35 batches, SELFTEST OK, ref impl 35/35) committed before any agent ran.
- **Panel (cold, isolated `/tmp`, oracle never copied in):** gpt 35/35 (Python, 152s),
  gemini 35/35 (Python, 199s), opus 35/35 (Python, 200s), composer 35/35 (Python, 63s)
  = **140/140, zero functional misses.** Every precedence batch passed; the 18-row
  registry and byte-exact messages reproduced exactly; bare codes survived being read in
  two places; cardinality honored on multi-defect doors.
- **Diagnosis** (`round-03/results.md`): **the structured-obligation cluster is refuted as
  a correctness defect across two rounds.** Prose obligation values — even at 18 rows, a
  non-monotonic precedence chain, and codes read in two places — caused 0 functional
  misses in 4/4 cold implementations. The black-box method cannot measure the residual
  *ergonomic* hypothesis (authoring/diff/validation cost), so the cluster is closed
  `wontfix` (5 ids) with that residue routed to tooling. Two new corroborated spec-defects
  surfaced from the panel's aligned guessing: (B) `trust-boundary-violation-residual`
  STRONG (3/4) — naming a trust boundary (Round-2 fix) is not enough; the consumer must
  also pin what it does **if a relied-on guarantee is violated**, or models invent the
  out-of-contract handling; (A) `segmentation-terminator-mechanics` (2/4) — "MAY accept a
  trailing newline" without a strip count, and the lone-/all-separator degenerate input,
  are each under-determined. Regression re-verify held 4/4 for `dataflow-invisible`,
  `closed-set-dispatch-residual`, `input-segmentation-completeness`, `cross-cutting-single-home`.
- **Source-of-truth fixes:** `yass.yass.yaml` — `Slot.INPUT` now requires, for each
  relied-on producer guarantee, stating the behavior if that guarantee does not hold
  (residual on violation). `yass-reference.md` — slot-targeted `USES` bullet extended with
  the violation residual. `GUIDANCE.md` — Composition trust-boundary bullet extended with
  the violation residual; input-segmentation checklist gains the optional-terminator strip
  count and the lone-/all-separator degenerate cases. No schema change. **Prune:** removed
  `RECOMMENDATIONS.md` Part 2 §1 ("Treat an ordered list as ordered and step-addressable"),
  the recommendation corresponding to the now-`wontfix` `priority-chains-prose`. The raw
  per-agent feedback corpus (`SPEC/GUIDANCE/OPEN-FEEDBACK.md`) was deliberately **not**
  carved up — it is one-time-ingested provenance that still seeds open findings.
- **Tooling routed:** created `experiment/TOOLING.md`; `lint-anti-slop` now owns the
  cluster's ergonomic residue — `yass extract-errors` registry projection, byte-exact
  message-template lint, error-code reference validation, and priority/cardinality idiom
  lint. The language stays scalar-prose-only.
- **Findings delta:** closed `wontfix` (refuted ×2): `mapping-valued-obligations`,
  `error-table-structured`, `error-code-refs`, `priority-chains-prose`,
  `error-cardinality-implicit`. Added + resolved: `trust-boundary-violation-residual`,
  `segmentation-terminator-mechanics`. Total findings: 30. Resolved (SOT): 7. Open: 18.
  Convergence counter: 0/2.

## Round 4 — the CONFORMS cluster: transclusion · guard-combination · bare-slot (2026-06-24)

- **Plan** (`round-04/PLAN.md`): drive the three open CONFORMS findings —
  `conforms-overloaded`, `conforms-inlining-semantics-misplaced`, `conforms-bare-slot-meaning`
  — never exercised, because Rounds 02–03 used `USES`/`SEE` for shared conventions. Build a
  CLI whose shared-convention references are CONFORMS so each property flips specific oracle
  batches: enforce-vs-ignore (O1–O3), outer×inner guard combination (the four corners), and a
  bare whole-spec CONFORMS (the grammar-vs-`conforms_no_slot` contradiction). Fold in the
  Round-1/2/3 regression fixes for free (CLI-shaped).
- **Spec** (`test-specs/round-04-axle`): trackside wayside hotbox-detector axle audit —
  `audit` and `roster` subcommands over a shared `axle.shared` spec (dispatch, segmentation,
  7-field record format, 12-code defect registry with a non-monotonic precedence chain and an
  inner-guarded H07, error-line format). `audit::INPUT` transcludes the record format via
  CONFORMS; `roster` carries a guarded CONFORMS (outer `WHEN flagged Y` × H07 inner
  `WHEN mileage > 200`), a bare whole-spec `CONFORMS ./axle.audit@axle.audit`, and a
  `USES …::RETURN` trust boundary. A deliberately **dead** guard-less `E90` residual rode
  along (E10/E15/E20 are exhaustive). Probe + reference + oracle (42 batches, SELFTEST OK, ref
  impl 42/42) committed before any agent ran.
- **Panel (cold, isolated `/tmp`, oracle never copied in):** gpt 42/42 (Python, 108s),
  gemini 42/42 (Python, 147s), opus 42/42 (Python, 380s), composer 41/42 (Python, 91s)
  = **167/168.** All four enforced the CONFORMS-transcluded record contract (O1–O3 byte-exact),
  passed all four guard-combination corners (AND-semantics), and read the bare whole-spec
  CONFORMS as whole-spec conformance (no over-validation). composer's sole miss
  (`aud_lone_newline`: a lone `"\n"` treated as zero records) is a model-error — a literal
  reading and opus's NOTES both side with the oracle (one empty record → E10 got 0).
- **Diagnosis** (`round-04/results.md`): the headline is a **new** spec-defect surfaced
  unanimously in NOTES — `residual-reachability` (4/4 STRONG): the `E90` residual is dead
  (the guards partition every input), and all four models independently flagged it as
  unreachable, several noting it contradicts the well-formed/malformed invariant
  (oracle-confirmed: `E90` has zero references). The residual discipline ("always state a
  residual") carried no reachability caveat — the necessary converse of `default-error-policy`.
  CONFORMS findings: `conforms-overloaded` **refuted** behaviorally (4/4 clean, zero NOTES
  confusion); `conforms-inlining-semantics-misplaced` **resolved** (guard-conjunction was
  genuinely absent from the language level); `conforms-bare-slot-meaning` **resolved** in the
  grammar's favor (4/4 read whole-spec CONFORMS correctly; the tooling error was overriding a
  meaning the language already carried). A latent roster consumer divergence (blank-line /
  malformed-`Y` tallying) is covered by existing `closed-set-dispatch` / `trust-boundary`
  guidance and is a round-04 probe-authoring gap, not a new finding.
- **Source-of-truth fixes:** `yass.yass.yaml` — `Slot.ERROR` MUST-NOT carry a guard-less
  residual when the guarded obligations already account for every input; `Reference`
  SIDE-EFFECT now scopes inlining (slot-targeted CONFORMS inlined, whole-spec CONFORMS not
  inlined, USES MAY, SEE never) and states the semantic guard-conjunction (inlined obligation
  with its own guard applies only when carrier ∧ inner hold). `spec/cli.query.yass.yaml` —
  whole-spec CONFORMS gate (leave carrier unchanged, inline nothing); removed the
  `conforms_no_slot` ERROR slot. `spec/cli.errors.yass.yaml` — deleted the
  `yass.query.conforms_no_slot` registry entry (clean retirement per the
  MUST-NOT-reuse-retired-code INVARIANT). `context/GUIDANCE.md` — "Error obligations" gains
  the do-not-state-an-exhausted-residual rule. `context/yass-reference.md` — ERROR-slot
  residual reachability, ref-only note, relations table + dual-aspect prose + guard-conjunction
  bullet, and the Notes/open-items resolution passage updated for the whole-spec exception.
  No schema change. **Prune:** removed the resolved "CONFORMS without ::SLOT" section from
  `context/FIXES.md`. `context/SPEC-FEEDBACK.md` deliberately **not** edited — it is a verbatim
  raw-feedback transcript ("don't take any of it at face value"); editing it would falsify the
  historical record. An adversarial coherence review (delegated) confirmed the edit set is
  internally coherent — no contradiction between `Reference` RETURN (meaning) and SIDE-EFFECT
  (mechanics), no dangling `conforms_no_slot` reference in any `.yass.yaml`, `cli.query` still
  validates structurally and cites only live error codes.
- **Findings delta:** added + resolved `residual-reachability`; resolved
  `conforms-inlining-semantics-misplaced`, `conforms-bare-slot-meaning`, and
  `self-validation-ref-bug` (re-verified repo-wide: 17 specs schema-valid, refs resolve, 0
  dangling); closed `conforms-overloaded` `wontfix` (refuted). Full ledger: 40 total —
  11 resolved (SOT), 6 wontfix, 23 open. Convergence counter: 0/2.

_(Round 5 begins next — final scheduled round: composition/reference sequencing
(`cross-spec-sequencing`, `reftarget-resolution-scattered`, `dispatch-subcommand-override`)
+ re-verify the round-04 CONFORMS and residual-reachability fixes. HALT and report after.)_
