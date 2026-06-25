# Round 5 — Plan (FINAL scheduled round)

## Targets (highest-leverage OPEN composition/reference findings + re-verify round-04)

This round pivots from the now-exhausted CONFORMS cluster to **composition / reference
sequencing**, and re-verifies the round-04 fixes under load. Findings in scope:

1. **`cross-spec-sequencing`** (universal recurrence, OPEN). yass has no relation key for
   execution ordering / preconditions between specs; `USES` conflates *calls* / *depends-on* /
   *after*. Round-02 showed the **dataflow** ordering case is expressible (`USES …::RETURN`
   pipeline). The remaining piece is a **non-dataflow precondition**: spec B may run only after
   spec A established something, where B does **not** consume A's output as data. Deciding
   question: can a cold model implement a cross-spec precondition expressed only in prose + a
   `USES`/`SEE` pointer (no `REQUIRES`/`AFTER` key exists), or do ≥2 models diverge — proving the
   language needs an ordering relation?

2. **`dispatch-subcommand-override`** (repeated, OPEN). When a shared/dispatch spec states a
   global rule and one subcommand spec **overrides** it, the precedence is unexpressed (yass has
   no override mechanism). Deciding question: do cold models read "subcommand-specific overrides
   the shared rule" correctly from prose, applying the override **only** in the overriding
   subcommand and the shared rule **everywhere else** — or do ≥2 over-apply / under-apply it?
   Includes the bare `-` `argv[1]` lexical case (does `-` route to usage like any non-subcommand
   token, or does a "stdin convention" override the dispatch rule?).

3. **`reftarget-resolution-scattered`** (repeated, OPEN). Ref-target resolution rules are split
   across multiple `spec/cli.*.yass.yaml` specs + `yass.yass.yaml`. **Addressed directly as an
   SOT-consolidation review** (delegated agent audit of where resolution rules live, then
   consolidate into one owner). Not probe-dependent — it is about the yass spec files'
   organization, not generated-CLI behavior. Per directive, fixed this round, not deferred.

### Re-verification surface (round-04 fixes, exercised by the probe — not asserted)

- **`residual-reachability`** — author a genuinely **exhaustive** error set with **no** dead
  guard-less catch-all; confirm 4/4 implement cleanly and none flag a missing residual.
- **CONFORMS slot-targeted transclusion** — shared record format transcluded into an op spec;
  confirm the must-match contract is enforced cold.
- **CONFORMS whole-spec (non-transcluded conformance reference)** — `audit` carries a bare
  whole-spec `CONFORMS` to `run`; confirm read as conformance, not over-validation.
- **guard-conjunction** — guarded carrier CONFORMS × inner-guarded registry obligation; the four
  corners (outer × inner) must AND.

### Regression surface (free, CLI-shaped)

- `closed-set-dispatch-residual` (missing/unknown/wrong-case `argv[1]` → usage, exit 2).
- `input-segmentation-completeness` + `segmentation-terminator-mechanics` (ASCII-space split with
  run-collapse, LF separator, strip exactly one trailing LF, empty input = 0 records, blank
  interior line = zero-field record, lone `"\n"` = one zero-field record, tab/CR as data).
- `dataflow-invisible` + `trust-boundary-violation-residual` (`audit` consumes `run` output; trust
  the verdict text, do not re-validate; stated residual on out-of-contract input).
- `cross-cutting-single-home` (one `scale.shared` owns protocol/record/registry/errors).

### Probe-authoring gap closure (the round-04 lapse)

The round-04 oracle lacked a blank-line / malformed-`RECHECK`-`Y` consumer batch, leaving a latent
divergence untested. Round-05's oracle **MUST** exercise out-of-contract `audit` input directly:
a blank interior line (the override case) and a malformed `REWEIGH` flag (the residual case).

---

## Domain

**Weighbridge (truck-scale) ticket session.** Fresh, non-famous, deterministic, integer-only
arithmetic; no clock / network / randomness. (Prior domains: berth, apiary, vault, axle.)

A vehicle weighbridge processes a session of operations. A vehicle must be **tared** (empty-weight
zeroed) before it can be **weighed**; the weigh derives a net weight and classifies overload.

## CLI shape

One binary `scale`. `argv[1]` selects a subcommand from the **closed set `{run, audit}`**.

- **`scale run`** — reads an operation stream from stdin (`TARE` / `WEIGH` lines), emits one
  ticket line per successful operation to stdout, errors to stderr, exit = max severity.
- **`scale audit`** — the **consumer**: reads `run`'s ticket output (+ hand-fed `REWEIGH` lines),
  tallies by class, exit 0.

Dispatch residual (regression): a missing `argv[1]`, an unknown token, a wrong-case token
(`RUN`/`AUDIT`), or a **bare `-`** → `usage: scale {run|audit}\n` to **stderr**, exit **2**, no
stdout. (The dispatch closed-set rule governs `-`; there is no "stdin convention" override.)

---

## Specs and file layout (`test-specs/round-05-scale/spec/`)

Nine specs across five files. `scale.shared` is the single home for cross-cutting protocol.

### `scale.shared.yass.yaml`
- **Preamble.**
- **`scale`** (dispatch) — INPUT routes `argv[1]` over `{run, audit}`; ERROR residual = usage /
  exit 2 for missing / unknown / wrong-case / bare-`-`.
- **`scale.lines`** (segmentation) — byte→records: read all stdin; strip **exactly one** trailing
  LF if present; split on LF into records; **a blank line is a zero-field record** (shared rule —
  this is what `audit` overrides); fields = ASCII-space split with runs collapsed and
  leading/trailing stripped; tab and CR are ordinary data bytes. Empty input (zero bytes) = zero
  records. Byte-exact via surrogateescape.
- **`scale.record`** (operation record format) — INPUT: a record is `<KEYWORD> <id> <number>`;
  `KEYWORD` ∈ `{TARE, WEIGH}`; `id` = 3–10 chars, first char `A`–`Z`, each char `A`–`Z` or `0`–`9`;
  `number` = a well-formed non-negative integer (a non-empty run of ASCII digits; no leading zero
  unless the literal `0`). ERROR codes E10/E25/E15/E20 (below). **Exhaustive** — no residual.
- **`scale.defects`** (overload registry) — INVARIANT: the verdict registry W01–W04 with
  precedence and class→exit; W04 carries an **inner `WHEN`**. (Detail below.)
- **`scale.errors`** (error-line format) — INVARIANT: every error line is `<CODE> <message>\n` to
  stderr; one line per rejected record; codes and message templates pinned (below).

### `scale.tare.yass.yaml`
- **Preamble.**
- **`scale.tare`** — the TARE operation.
  - INPUT — `CONFORMS scale.shared@scale.record::INPUT` (**slot-targeted transclusion**: the
    record contract is inlined and enforced here), with `KEYWORD = TARE`.
  - RETURN — emits `<id> TARE <tare>\n` to stdout; CLEAR class (exit contribution 0).
  - SIDE-EFFECT — records `tare[id] = <number>` for the session; a later TARE for the same id
    **overwrites** (last tare wins); no error on re-tare.

### `scale.weigh.yass.yaml`
- **Preamble.**
- **`scale.weigh`** — the WEIGH operation. **This is the `cross-spec-sequencing` probe.**
  - INPUT — `CONFORMS scale.shared@scale.record::INPUT` (slot-targeted transclusion), with
    `KEYWORD = WEIGH`.
  - **Precondition (expressed in prose + `USES scale.tare`, NO ordering key):** a WEIGH for `<id>`
    requires a **prior `scale.tare` for the same `<id>`** earlier in the same session. If none has
    been processed → **E40** (`weigh before tare`), no stdout line, error contribution. The
    precondition is **per-id**: a prior TARE for a *different* id does not satisfy it.
  - RETURN — `net = gross − tare[id]`; classify by `CONFORMS scale.shared@scale.defects::INVARIANT`
    (registry transclusion); emit exactly one verdict line.
  - ERROR — **E50** (`negative net`) WHEN `gross < tare[id]` (net would be negative); no stdout.
    The error set {E10,E25,E15,E20,E40,E50} is **exhaustive** over malformed / precondition-failing
    / impossible inputs — **assert exhaustiveness, state no residual** (residual-reachability).

### `scale.run.yass.yaml`
- **Preamble.**
- **`scale.run`** — the `run` subcommand.
  - INPUT — `USES scale.shared@scale.lines` (segmentation); for each record dispatch on `KEYWORD`
    to `scale.tare` (`USES scale.tare`) or `scale.weigh` (`USES scale.weigh`); a record whose
    first field is neither → **E25** unknown operation.
  - RETURN — per-record ticket / error lines in input order; exit = **max contribution** across all
    records (CLEAR/TARE-ack 0; any error 1; MINOR 1; MAJOR 2; CRITICAL 3). Empty input → no output,
    exit 0.
  - ERROR — `USES scale.shared@scale.errors`; one error line per rejected record to stderr.

### `scale.audit.yass.yaml`
- **Preamble.**
- **`scale.audit`** — the `audit` subcommand (consumer). **This is the
  `dispatch-subcommand-override` + trust-boundary + whole-spec-CONFORMS + guard-conjunction
  probe.**
  - INPUT — **whole-spec** `CONFORMS ./scale.run@scale.run` (conformance reference, **not**
    transcluded): consume `run`'s ticket output as its contract; do **not** re-run `run`'s field
    validation on ticket lines. `USES scale.run::RETURN` (dataflow/pipeline anchor).
    - **OVERRIDE (subcommand overrides shared rule):** *overriding `scale.shared@scale.lines`, a
      blank line in `audit` input is a **section break** — skipped, not counted, not an error.*
      (In `run` the shared rule still holds: a blank line is a zero-field record → E10.)
  - RETURN — classify each ticket line by its **verdict token** (field 2), trusting the text
    (`trust-boundary`): `TARE`/`NET` → CLEAR; `W01` → CRITICAL; `W02` → MAJOR; `W03`/`W04` → MINOR
    (`CONFORMS scale.shared@scale.defects::INVARIANT` for the class map). Emit exactly five lines,
    exit 0:
    ```
    TICKETS <total>
    CLEAR <n>
    CRITICAL <n>
    MAJOR <n>
    MINOR <n>
    ```
    `<total>` counts every consumed line (ticket lines + `REWEIGH` lines; blank lines skipped).
  - `REWEIGH` re-derivation (**guard-conjunction four corners**): a `REWEIGH <id> <Y|N> <gross>
    <tare>` line (5 fields). **Guarded carrier** `CONFORMS scale.shared@scale.defects::INVARIANT`
    **WHEN the re-weigh is flagged `Y`**: only when flagged `Y`, re-derive `net = gross − tare` and
    classify by the registry; the registry's **W04 carries its own inner `WHEN gross > 50000`**, so
    W04 fires only under **(flag = Y) AND (gross > 50000)**. Flag `N` → no re-derivation → CLEAR.
    - **Residual on out-of-contract `REWEIGH` (trust-boundary residual, gap closure):** a `REWEIGH`
      whose flag ∉ `{Y, N}`, or whose field count ≠ 5, is **treated as not-flagged** → counts
      toward `TICKETS` as CLEAR; never errors (audit exits 0 by contract).
    - **Unrecognized ticket verdict token** (field 2 ∉ {TARE, NET, W01..W04}) → counts toward
      `TICKETS`, bumps **no** class.

---

## Defect registry `scale.defects` (pinned)

Verdict codes, precedence **highest → lowest = W01, W02, W03, W04**, evaluated on `net = gross −
tare` for a well-formed weigh (`net ≥ 0`):

| code | class    | exit | guard                              | verdict line (stdout)                         |
|------|----------|-----:|------------------------------------|-----------------------------------------------|
| W01  | CRITICAL |    3 | `net > 44000`                      | `<id> W01 critical overload above 44000`      |
| W02  | MAJOR    |    2 | `net > 40000`                      | `<id> W02 major overload above 40000`         |
| W03  | MINOR    |    1 | `net > 36000`                      | `<id> W03 minor overload above 36000`         |
| W04  | MINOR    |    1 | **`WHEN gross > 50000`**: `net > 30000` | `<id> W04 heavy vehicle net above 30000` |
| —    | CLEAR    |    0 | (none of the above)                | `<id> NET <net>`                              |

- Evaluation is **first-match by precedence order** (W01 then W02 then W03 then W04); the
  complement → CLEAR (`NET`). The four guarded codes + CLEAR are **exhaustive over `net ≥ 0`**;
  `net < 0` is E50, never a verdict. **No residual verdict.** (residual-reachability)
- W04 vs W03 precedence interaction (regression): a `net` in (36000, …] with `gross > 50000`
  matches both W03 and W04 → **W03 wins** (listed first); the message differs, so a model that
  classifies by *class* alone (both MINOR) cannot distinguish — only *registry order* gives the
  right line.
- W04's inner guard (`gross > 50000`) is the inner half of the guard-conjunction test in `audit`.

---

## `scale run` ordered per-record checks (oracle, byte-exact)

For each record (after segmentation), in order; stop at first failure, one error line per record:

1. **0 fields** (blank/whitespace-only line) → `E10 malformed record: empty line`
2. **field 1 ∉ {TARE, WEIGH}** → `E25 unknown operation: <field1>`
3. **TARE and field count ≠ 3** → `E10 malformed record: TARE expects 3 fields, got <n>`
4. **WEIGH and field count ≠ 3** → `E10 malformed record: WEIGH expects 3 fields, got <n>`
5. **field 2 not a well-formed id** → `E15 bad vehicle id: <field2>`
6. **field 3 not a well-formed integer** → `E20 bad number: <field3>`
7. **WEIGH and no prior TARE for `<id>`** → `E40 weigh before tare: <id>`
8. **WEIGH and `gross < tare[id]`** → `E50 negative net: <id>`

Success:
- **TARE** → set `tare[id]=number` (overwrite ok); stdout `<id> TARE <number>`; contribution 0.
- **WEIGH** → `net = gross − tare[id]`; emit registry verdict line; contribution per class.

Exit = max contribution (error = 1; CLEAR/TARE = 0; MINOR = 1; MAJOR = 2; CRITICAL = 3). Empty
input → exit 0, no output.

---

## Oracle batch design (`test-specs/round-05-scale/oracle/grade.py`)

Single-invocation, pure stdin → stdout/stderr/exit. Byte-exact via `surrogateescape`. Same shape as
round-04 (`grade.py --self-check` → `SELFTEST OK`; `--cmd "<RUN>"` → `SCORE: passed/total`; a
`ref.py` reference implementation scores full before the panel). Batches (≈40), each pins
`(argv, stdin) → (stdout, stderr, exit)`:

**DISPATCH (regression):** missing argv; `verify` (unknown); `RUN`/`AUDIT` (wrong case); bare `-`
→ all `usage: scale {run|audit}\n` / exit 2.

**RUN — segmentation/terminator (regression):** empty input → exit 0; trailing-LF strip;
blank-interior line → E10 empty line; lone `"\n"` → one E10 empty-line record; tab/CR as data;
run-collapse of repeated/leading/trailing spaces.

**RUN — record format (CONFORMS transclusion re-verify):** wrong field count (E10); unknown
keyword (E25); bad id `a1` / `040`-style (E15); bad number (E20). Confirms the transcluded record
contract is enforced.

**RUN — cross-spec-sequencing (`cross-spec-sequencing` PROBE):**
- `WEIGH AX1 70000` with no prior TARE → **E40** (gate fires).
- `TARE AX1 30000` then `WEIGH AX1 70000` → verdict (net 40000 → W03 MINOR; pick value to land a
  clear class).
- `TARE AX1 30000` then `WEIGH AX2 70000` → **E40** (per-id: AX2 not tared).
- `TARE AX1 30000`, `WEIGH AX1 ...`, `WEIGH AX1 ...` → second weigh reuses the tare (both verdicts).
- re-tare: `TARE AX1 30000`, `TARE AX1 25000`, `WEIGH AX1 60000` → net uses **last** tare (35000).

**RUN — verdict registry + residual-reachability (re-verify):** values landing each of
W01/W02/W03/W04/CLEAR; the W03-vs-W04 precedence case (net 37000, gross 55000 → **W03**); E50
(`gross < tare`). Confirms no dead residual and first-match precedence.

**AUDIT — pipeline / trust-boundary (regression):** feed a realistic `run` ticket block →
correct five-line tally; a ticket line with an out-of-range-looking id is **trusted** (not
re-validated); unrecognized verdict token → counted in TICKETS, no class bump.

**AUDIT — dispatch-subcommand-override (`dispatch-subcommand-override` PROBE):**
- a blank interior line in `audit` input → **skipped** (TICKETS unchanged, exit 0). Contrast with
  the RUN blank-line batch (→ E10). Tests that the override applies in `audit` and the shared rule
  still applies in `run`.

**AUDIT — guard-conjunction four corners (re-verify):** tare fixed per line via the 5-field
`REWEIGH`:
- C-TT `REWEIGH AX1 Y 55000 20000` → net 35000, gross > 50000 → **W04 → MINOR +1**.
- C-TF `REWEIGH AX1 Y 50000 15000` → net 35000, gross = 50000 **not** > 50000 → inner false →
  **CLEAR** (drop-inner-guard model wrongly bumps MINOR).
- C-FT `REWEIGH AX1 N 55000 20000` → flag N → outer false → **CLEAR** (drop-outer-guard model
  wrongly bumps MINOR).
- C-FF `REWEIGH AX1 N 50000 15000` → **CLEAR**.
- (All four chosen so only W04 is reachable: net ≤ 36000 avoids W03/W02/W01.)

**AUDIT — out-of-contract residual (GAP CLOSURE):** `REWEIGH AX1 X 55000 20000` (flag ∉ {Y,N})
→ treated as not-flagged → CLEAR, counted in TICKETS; a `REWEIGH` with wrong field count →
CLEAR, counted; never errors. Confirms the stated trust-boundary residual.

---

## Procedure

1. Author specs + oracle + `ref.py` + `HOWTORUN.txt` from this PLAN. **Validate:** repo-wide
   `/tmp/check_all_refs.py` → `RESULT: CLEAN`; `grade.py --self-check` → `SELFTEST OK`;
   `grade.py --cmd "python3 .../ref.py"` → full score. Commit before any agent runs.
2. Run the panel cold: unique `/tmp/yass-r05-<model>-<rand>` per model, copy in **only**
   `spec/*.yass.yaml`, background + concurrent via `script/agent` with `experiment/round-05/prompt.md`.
3. Grade each candidate (`grade.py --cmd "<RUN>"`); capture stdout/stderr/exit + `NOTES.md`.
4. Diagnose every miss: spec-defect vs model-error; cross-model ≥2 = strong signal; NOTES
   ambiguities flagged by ≥2 models = latent spec defects; 4/4-clean = evidence **against** a
   defect. Write `experiment/round-05/results.md`.
5. Fix source of truth (`yass.yass.yaml`, schema, GUIDANCE, yass-reference); keep `spec/*` consistent;
   prune resolved content from `context/*`; route tooling to TOOLING.md; update FINDINGS.
6. Commit `round-05`; update STATUS.md + LOG.md + convergence counter. **HALT and report.**

## Success criteria / what each outcome means

- **`cross-spec-sequencing`:** if ≥2 models miss the per-id precondition gate (emit a verdict for an
  un-tared weigh, or use a global rather than per-id tare) → **confirmed**, the language needs an
  ordering relation (`REQUIRES`/`AFTER`); add it to the SOT. If 4/4 implement it from prose +
  `USES` → **refuted** (prose suffices, like round-02 dataflow), `wontfix`.
- **`dispatch-subcommand-override`:** if ≥2 models mis-scope the blank-line override (apply it in
  `run`, or fail it in `audit`) → **confirmed**, add an override/precedence mechanism. If 4/4 scope
  it correctly → **refuted**, `wontfix`.
- **Re-verifications (residual-reachability, CONFORMS slot/whole-spec, guard-conjunction):** 4/4
  clean confirms the round-04 fixes hold under load. Any ≥2 divergence reopens the finding.
- **`reftarget-resolution-scattered`:** resolved by SOT consolidation (agent audit → single owner),
  re-validated by `check_all_refs.py` CLEAN.
