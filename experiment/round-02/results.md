# Round 02 — results (apiary harvest pipeline)

Probe: `test-specs/round-02-apiary` — one CLI program, three subcommands
(`tally` → `grade` → `pack`) chained by shell pipes. argv[1] selects the stage;
each stage reads stdin → writes stdout/stderr; exit code is contract. Shared wire
protocol (subcommand dispatch, line/field segmentation, exit policy, error-line
format) owned by a single `honey.shared` spec (specs `honey`, `honey.lines`,
`honey.errors`) and referenced from the three stage specs via `USES`.

Targets (from PLAN): `dataflow-invisible` + `cross-spec-sequencing` (primary,
universal pair), `cross-cutting-single-home` (cross-cutting wire protocol in one
owning spec), with `default-error-policy` + `input-segmentation-completeness`
regression coverage.

Panel: gpt-5.5-extra-high · gemini-3.1-pro · claude-opus-4-8-thinking-high ·
composer-2.5. Cold + isolated `/tmp` workspace per model; only `spec/*.yass.yaml`
copied in; private oracle never present in any workspace.

## Scores

Oracle: 38 batches (33 per-stage + 5 full-pipeline). SELFTEST OK; reference impl
38/38 before the panel ran.

| model    | lang   | score | wall time |
|----------|--------|-------|-----------|
| composer | Go     | 38/38 | ~70 s     |
| gpt      | Python | 38/38 | ~125 s    |
| gemini   | Python | 38/38 | ~159 s    |
| opus     | Python | 38/38 | ~242 s    |

**152/152 batches. Zero functional misses.** Every targeted obligation —
inter-stage dataflow, header-gate sequencing, guard-less residual (`E90`),
off-spec segmentation (tab / CR / NBSP as data, single trailing newline, empty
input) — was implemented correctly cold by all four models. Byte-exact tokens
(`HARVEST/1`/`GRADED/1`/`PACK/1`, `E10`/`E20`/`E21`/`E22`/`E40`/`E41`/`E50`/`E70`/`E90`,
band cutoffs 400/200/199, capacities 1000/800/600, exit codes 0/1/2) all matched.

## Analysis — every signal is spec-quality, not a miss

No functional miss to classify. The actionable signals are the ambiguities the
panel flagged in `NOTES.md` despite scoring perfectly — latent spec defects that
all surviving implementations resolved by *guessing alike*.

### Signal 1 — inter-stage trust boundary (4/4 models) → `dataflow-invisible`, STRONG

All four models flagged the same gap: `honey.grade` and `honey.pack` redefine
"well-formed" for their inputs **without restating the upstream validation the
previous stage already performed**. `honey.grade` INPUT consumes the records
`honey.tally` RETURN produces (it carries `USES ./honey.tally@honey.tally::RETURN`),
but the spec never states whether grade re-validates the hive-id format or trusts
it as already-checked and opaque. Every model had to decide the trust boundary by
inference; all four decided identically (downstream trusts upstream-validated
fields, re-checks only what its own stage adds — net positivity, band membership)
— which is why they all matched the oracle. opus additionally flagged that
`honey.pack` does not re-check net-vs-band consistency.

The `USES …::RETURN` pointer was present and resolved, but it carries **only
structural meaning** in the current SOT (it names a slot that exists); it conveys
no dataflow reading — nothing that tells a reader "the data this INPUT consumes is
exactly what that RETURN produces, so that slot's guarantees characterize it
here." The data contract between stages was therefore invisible, exactly the
`dataflow-invisible` finding (universal, open), now corroborated 4/4 in a fresh
domain.

### Signal 2 — closed-set dispatch has no residual (3/4 models) → NEW `closed-set-dispatch-residual`

The `honey` shared spec dispatches on the closed set `{tally, grade, pack}` but
defines **no behavior for a missing or unknown argv[1]**. Three of four models
flagged this and **diverged** on the invented behavior:

- composer → print usage to stderr, exit 1
- gpt, opus → emit an invented `E00`-class error, exit 2
- gemini → did not flag (also resolved it by guessing)

This is the same failure mode as `default-error-policy` (resolved Round 1) — a
closed enumeration with no stated residual — but in the **INPUT/dispatch slot**
rather than the ERROR slot. The Round-1 fix scoped the residual principle to
ERROR only; the principle generalizes to any closed-set branch. Distinct from the
existing open finding `dispatch-subcommand-override`, which concerns *precedence
between* dispatch-level and subcommand-level rules (the bare `-` case), not the
handling of an *out-of-set value*.

### Signal 3 — `cross-spec-sequencing` mechanism proved adequate (4/4) → partial positive

The header-gate pattern (`E40`/`E41` + write-nothing + reject-all + exit 2 on a
wrong stage header, paired with the `USES …::RETURN` pointer to the producing
stage) expressed the data-pipeline sequencing **correctly for all four models**.
The pipeline batches (tally→grade→pack with right and wrong headers) all passed.
So *data-pipeline* sequencing is expressible today with header gates + a RETURN
pointer. What remains open is (a) the **data contract** across the boundary
(Signal 1, `dataflow-invisible`), and (b) the **USES overload**: `USES` now also
carries a "consumes-the-output-of" meaning (`::RETURN`), piling another role onto
a relation that already conflates call / depend / after. `cross-spec-sequencing`
stays open, re-scoped: the header-gate idiom covers the dataflow case; non-dataflow
execution preconditions (a REQUIRES/AFTER notion) and the USES overload are
unaddressed.

### `cross-cutting-single-home` (4/4) → resolved

`honey.shared` owned the entire wire protocol (dispatch, segmentation, exit
policy, error-line format) and the three stage specs referenced it. All four
models implemented the cross-cutting rules consistently from the single owner — no
drift. The pattern works; the SOT had no guidance teaching it. Resolved by the new
composition guidance.

### Regressions hold

- `default-error-policy` — the guard-less `E90 unprocessable record: <record>`
  residual in `honey.tally`/`grade`/`pack` was read correctly by 4/4. Re-verified.
- `input-segmentation-completeness` — tab/CR/NBSP-as-data, single optional
  trailing newline, empty input = zero records, runs-of-spaces field splitting:
  all handled correctly by 4/4. Re-verified.

### Not exercised

- `conforms-overloaded` — the probe used `USES`/`SEE` for shared conventions, not
  `CONFORMS` (shared conventions are "draws on", not "must match"), so this finding
  was not tested this round. Stays open, untested.

## Conclusions / SOT actions

1. **`dataflow-invisible` → resolve.** Give a slot-targeted `USES` (e.g.
   `…@Spec::RETURN`) a documented **dataflow reading** in the language: it means
   the obligation consumes/builds on the data that slot produces, so that slot's
   guarantees characterize the data at the boundary. Add the matching authoring
   rule (state which upstream guarantees you trust vs re-check). SOT:
   `yass.yass.yaml` (`Reference` + `Slot.INPUT`), `yass-reference.md`,
   `GUIDANCE.md`. No schema change — `::SLOT` targets are already valid.
2. **`cross-cutting-single-home` → resolve.** Document the "one owning spec for a
   cross-cutting wire format, referenced from the rest" pattern in `GUIDANCE.md`.
3. **`closed-set-dispatch-residual` (new) → resolve.** Generalize the residual
   principle beyond ERROR: any closed-set branch (input dispatch on a
   subcommand/mode/enum) MUST state the out-of-set/missing case. SOT:
   `GUIDANCE.md`, `yass.yass.yaml` (`Slot.INPUT`), `yass-reference.md`.
4. **`cross-spec-sequencing` → stays open, re-scoped** (header-gate idiom covers
   dataflow sequencing; non-dataflow preconditions + USES overload remain).
5. Prune the now-resolved distilled recommendations from `RECOMMENDATIONS.md`
   (Part 1 §3 cross-cutting home; Part 2 §2 inputs/outputs name crossing data).

Convergence counter unchanged at 0/2 — Round 2 produced new actionable findings
(one strong corroboration resolved, one cross-cutting resolved, one new finding).
