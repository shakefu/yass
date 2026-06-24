# Round 3 — Plan

## Objective

Probe the **structured-obligation cluster** at a scale Rounds 1 and 2 never
reached. Round 1 stressed it with a small prose error registry (E10–E90) and an
8-step priority chain and found **zero functional misses** (inference-expensive,
not incorrect). This round scales the same constructs up and adds a deliberate
adversarial twist so the cheapest shortcut yields a *wrong* answer:

- a **large prose defect registry** — 18 codes across 3 severity classes
  (CRITICAL / MAJOR / MINOR), each code packing its trigger condition, its
  severity class, and its exact byte-exact verdict message into one prose
  obligation (`mapping-valued-obligations` as root, `error-table-structured`,
  `error-code-refs`);
- a **precedence chain stated as one prose enumerated order** that is
  deliberately **NOT monotonic with severity class and NOT in code-number
  order** (`priority-chains-prose`). A model that shortcuts "report the most
  severe defect" or "report the lowest-numbered code" gets a wrong verdict on
  the many doors that trigger several defects at once;
- **one verdict per record** — emit only the single highest-precedence triggered
  defect per door, else PASS (`error-cardinality-implicit`). A model that emits
  one line per triggered defect fails cardinality.

Hypothesis: at 18 overlapping rules with prose precedence and one-emit-per-record,
cold models will mis-rank precedence, emit multiple verdict lines, or miss a byte.
A functional miss traces to the prose form of these structured obligations and is
direct evidence for an ordered/priority/table construct. If 4/4 pass instead, that
is strong convergence evidence that the cluster is ergonomic, not correctness-
critical at this scale.

## Findings under test this round

| id | recurrence | predicted failure mode in cold impls |
|----|-----------|--------------------------------------|
| `priority-chains-prose` | universal | An 18-entry precedence order, non-monotonic vs both severity and code number, is expressible today only as one prose list. Models will shortcut to "most severe wins" or "lowest code wins" and mis-rank the verdict on multi-defect doors. |
| `error-table-structured` | universal | An 18-row registry (code, class, condition, message) packed into prose obligations. Models will drop a class, paraphrase a message off byte-exact, or miscopy a threshold. |
| `error-cardinality-implicit` | repeated | "At most one verdict per door" is prose, not a structural cardinality. Models will emit one line per triggered defect. |
| `mapping-valued-obligations` | repeated (root) | Each registry row is logically a 4-field record forced into a single scalar string; watch how models reconstruct the table and whether the packing causes copy errors. |
| `error-code-refs` | single | Verdict lines and the report class-tally both reference codes by symbol (V05, …); watch whether the code→class→message mapping survives being read in two places. |

## Regression re-verification (Round-1 and Round-2 fixes)

The probe is authored so prior source-of-truth fixes are exercised, not asserted:

- **`dataflow-invisible` (Round 2).** A second subcommand `report` consumes
  `certify`'s stdout. Per the new `Slot.INPUT` obligation, `report`'s INPUT names
  the producing slot with `USES …@vault.certify::RETURN` and states which of
  `certify`'s guarantees it relies on (verdict-line shape, code validity) without
  re-validating the message text. If the fix holds, no model flags the boundary
  and none over-validates.
- **`closed-set-dispatch-residual` (Round 2).** `argv[1]` selects a closed set
  `{certify, report}`; the shared dispatch spec states the missing/unknown case
  (usage line, exit 2). If the fix holds, all models converge on it.
- **`input-segmentation-completeness` (Round 1).** The shared segmentation spec
  pins every boundary (single `\n` record; single trailing newline; empty input;
  ASCII space `0x20` field runs; tab/CR/NBSP are data). The oracle carries
  off-spec batches (tab, CRLF, NBSP, leading/trailing/repeated space).
- **`cross-cutting-single-home` (Round 2).** One `vault.shared` spec owns the
  wire protocol, the record layout, the defect registry, the precedence order,
  the class→exit map, and the error-line format; both subcommands reference it.

## Spec design (current-construct encoding, multi-file)

Fresh, non-famous domain (no training prior): **bank-vault time-lock door annual
certification**. One binary, two subcommands, integer arithmetic throughout
(deterministic).

- `spec/vault.shared.yass.yaml` — Preamble + the cross-cutting specs:
  - `vault` — dispatch (`argv[1]` ∈ {certify, report}), read-all-stdin,
    the severity-ladder exit policy, the missing/unknown-subcommand residual.
  - `vault.lines` — record/field segmentation.
  - `vault.record` — the 9-field door inspection record layout + the
    well-formed door-id and well-formed integer rules.
  - `vault.defects` — the 18-defect registry (code, class, condition, message),
    the precedence order, and the class→exit-rank map.
  - `vault.errors` — the processing-error line format + one-error-per-record.
- `spec/vault.certify.yass.yaml` — `certify`: raw door inspection records →
  one verdict per door (the single highest-precedence triggered defect, else
  PASS); guarded processing errors E10/E15/E20 in a fixed validation order;
  exit per the severity ladder.
- `spec/vault.report.yass.yaml` — `report`: consumes `certify`'s verdict lines,
  tallies the door total, the pass count, and the count per severity class, and
  writes a five-line summary; exits 0.

The registry rows, the precedence order, and the one-verdict-per-door cardinality
are pinned only as completely as current yass allows — prose obligations. There is
no construct that carries a table, a class tag, or an explicit order; those
absences are exactly the cluster under test.

## Oracle design (private, black-box, never copied in)

`test-specs/round-03-vault/oracle/grade.py` — embedded authoritative
`simulate_certify` / `simulate_report` + a SELFTEST self-consistency guard. A
private `oracle/ref.py` reference implementation is graded to confirm a fully
correct program scores 100/100 before the panel runs. Three grading axes
(byte-exact stdout/stderr/exit throughout):

1. **certify with canonical input** — clean PASS doors; one single-defect door
   per class (message byte-exactness); multi-defect precedence cases that
   distinguish the prose order from "most severe wins" and "lowest code wins";
   cross-field defects (drill<torch, bolt>wall); processing errors E10/E15/E20 in
   priority order; off-spec segmentation (tab, CRLF, NBSP, leading/trailing/
   repeated space, empty, trailing-newline, blank interior line); a mixed run
   exercising the full exit ladder.
2. **report with canonical certify output** — clean, mixed-class, all-pass,
   empty.
3. **chained** — `certify | report` threaded through the candidate's OWN
   certify stdout; compares the final summary to the canonical chain. Measures
   whether the candidate's certify output is consumable by its own report and
   matches canonical.

## Procedure

1. Author `test-specs/round-03-vault/spec/*.yass.yaml` (3 files) + private
   oracle (`grade.py` + `ref.py`). Validate: every document parses against
   `yass.v1.schema.json`; every ref resolves; oracle `--self-check` prints
   SELFTEST OK; `grade.py --cmd <ref.py>` scores 100/100. Commit before any agent.
2. Reuse `experiment/round-02/prompt.md` verbatim as `round-03/prompt.md`.
3. Run the 4-model panel (`gpt`, `gemini`, `opus`, `composer`) cold,
   concurrently, background, unique `/tmp` workspaces; copy in only `spec/*`.
4. Grade all axes + diagnose every miss spec-defect vs. model-error
   (≥2-model corroboration = strong signal). Write `results.md`.
5. Apply confirmed fixes to source of truth; update FINDINGS; prune resolved
   `context/*`; route irreducible tooling to TOOLING.md. Commit `round-03`;
   update STATUS + LOG; update convergence counter.

## Success criteria for the round

- Multi-axis harness validated (certify + report + chained all run; reference
  scores 100/100; SELFTEST OK).
- `priority-chains-prose` / `error-table-structured` / `error-cardinality-implicit`
  confirmed (≥2-model functional miss) or refuted with evidence (the precedence-
  shortcut batches are the deciding measurement).
- Round-1/Round-2 fixes regression-checked (segmentation, dispatch residual,
  dataflow boundary, single-home) and recorded.
- Confirmed spec-defects fixed in source of truth and pruned from `context/*`.
