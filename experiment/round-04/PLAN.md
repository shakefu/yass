# Round 04 — PLAN: the CONFORMS cluster (transclusion · guard-combination · bare-slot)

## Objective

Drive the three open **CONFORMS** findings — never yet exercised, because Rounds 02–03
used `USES`/`SEE` for shared conventions. CONFORMS is the language's most-overloaded,
"most bug-prone" relation: it bundles a hard *must-match contract*, *always-inline*, and
*provenance* into one key, and its inlining *mechanics* (guard preservation and guard
combination) live in an application spec (`cli.query.InlineConforms`) a cold reader never
sees. This round makes those three properties **black-box measurable** by building a CLI
whose shared-convention references are CONFORMS, so "does a cold model treat
`CONFORMS: shared@X::SLOT` as an enforced contract or an ignorable pointer?" flips specific
oracle batches between accept and reject.

## Findings under test

| id | recurrence | the deciding question |
|----|-----------|-----------------------|
| `conforms-overloaded` | universal | Does a cold model ENFORCE all obligations of a CONFORMS-referenced slot at the referencing site (must-match contract), or read CONFORMS as a `SEE`-like pointer it can ignore? |
| `conforms-inlining-semantics-misplaced` | repeated | When a CONFORMS carrier has its OWN `WHEN` guard and points at a slot whose obligations have their OWN `WHEN` guards, does the referenced rule fire only under **(outer AND inner)**? (The combination rule lives only in `cli.query.InlineConforms` 124–125; a reader following the language `Reference` rule alone drops a guard.) |
| `conforms-bare-slot-meaning` | repeated | A bare whole-spec `CONFORMS` (no `::SLOT`) is allowed by `RefTarget` grammar (yass.yass.yaml 220, 222) but rejected by `cli.query.InlineConforms::ERROR` (135–136, `yass.query.conforms_no_slot`, exit 1). What does the static contradiction resolve to, and what does a cold model do with whole-spec conformance? |

## Regression re-verification (fold in, free, because the probe is CLI-shaped)

- `self-validation-ref-bug` — **already resolved** (commit `868112e`); Round-04 repo-wide
  re-check: 17 specs schema-valid, 114 refs resolve, 0 dangling. Closed with evidence in
  FINDINGS; not a probe target.
- `closed-set-dispatch-residual` — missing/unknown `argv[1]` → usage to stderr, exit 2.
- `default-error-policy` — guard-less `E90` residual reachable only as the safety net.
- `input-segmentation-completeness` + `segmentation-terminator-mechanics` — ASCII-space
  field split (runs of spaces), LF record sep, strip exactly one trailing LF, empty input =
  zero records, blank-interior / lone-separator degenerate, tab/CRLF/NBSP as data.
- `dataflow-invisible` + `trust-boundary-violation-residual` — `roster` consumes `audit`'s
  output via a `USES …::RETURN` pointer + a stated trust boundary (trust the verdict text,
  do not re-validate the id/message) and a stated behavior for an out-of-contract line.
- `cross-cutting-single-home` — one `axle.shared` owns the wire protocol; both subcommand
  specs reference it.
- structured-obligation cluster (`wontfix`) rides along as regression-grade: a 12-code prose
  defect registry with a deliberately non-monotonic precedence chain and byte-exact messages.

## Domain + CLI

**Domain:** trackside railcar journal-bearing **wayside hotbox-detector audit** — a
detector emits one record per axle (bearing temperatures + physical readings) as a railcar
passes; the tool audits each axle against a fixed defect-threshold registry, then
summarizes. Fresh, non-famous, no training prior; pure integer arithmetic; fully
deterministic (no clock/network/randomness). Distinct from prior domains (berth, apiary,
vault).

**Binary `axle`**, `argv[1]` selects a closed set `{audit, roster}`:

- `axle audit` — reads raw axle reading records from stdin; validates each against the
  shared record format (transcluded by **CONFORMS**) and the defect registry; emits exactly
  one verdict line per well-formed axle to stdout (`<axle-id> CLEAR` or
  `<axle-id> <code> <message>`); processing-error lines to stderr; exit = max severity
  contribution over all records.
- `axle roster` — the consumer; reads `audit`'s verdict lines plus hand-fed re-inspection
  lines; tallies axles/cleared/per-class; emits a fixed five-line summary; exit 0.

**Exit policy:** missing/unknown `argv[1]` → `usage: axle {audit|roster}\n` to stderr,
nothing to stdout, exit 2. `audit`: CLEAR/clean = 0, malformed record = 1, MINOR = 1,
MAJOR = 2, CRITICAL = 3; exit = max contribution; empty input = 0. `roster`: always exit 0.
Every emitted line terminated by exactly one `0x0A`; output is UTF-8.

## File list (`test-specs/round-04-axle/spec/`)

| File | Specs declared | Outbound references |
|---|---|---|
| `axle.shared.yass.yaml` | Preamble; `axle` (dispatch), `axle.lines` (segmentation), `axle.record` (7-field record format + integer rules), `axle.defects` (12-code registry + precedence + class→exit, **H07 carries an inner WHEN guard**), `axle.errors` (error-line format) | none (owner) |
| `axle.audit.yass.yaml` | Preamble; `axle.audit` | R3 `CONFORMS ./axle.shared@axle.record::INPUT`; R4 `CONFORMS ./axle.shared@axle.defects::INVARIANT`; R5 `USES ./axle.shared@axle.lines`; R6 `USES ./axle.shared@axle.errors` |
| `axle.roster.yass.yaml` | Preamble; `axle.roster` | R7 **guarded** `WHEN flagged Y, CONFORMS ./axle.shared@axle.defects::INVARIANT`; R8 **bare whole-spec** `CONFORMS ./axle.audit@axle.audit`; R9 `USES ./axle.audit@axle.audit::RETURN` (+ trust-boundary prose); R10 `CONFORMS ./axle.shared@axle.defects::INVARIANT` |

All `./` targets resolve within the probe dir; every referenced slot is declared in the
target document. Preambles list siblings in `related:`. First line of every file is the
modeline `# yaml-language-server: $schema=https://textla.dev/yass/v1.schema.json`.

## Record format (`axle.record::INPUT`) — the finding-1 transclusion payload

A well-formed record is **exactly seven space-delimited fields**, in order:

1. `axle-id` — 3–10 chars; first char `A-Z`; every char in `A-Z` or `0-9`.
2. `bearing-temp` — non-negative integer (deg C rise).
3. `ambient-temp` — non-negative integer.
4. `mate-temp` — non-negative integer (same-side opposite axle rise).
5. `wheel-impact` — non-negative integer (kN).
6. `load` — non-negative integer (tonnes).
7. `mileage` — non-negative integer (thousands of miles since reprofile).

Integer well-formedness (round-03 proven): a non-empty run of ASCII digits `0-9`; no
leading zero unless the field is the single digit `0`.

## Defect registry (`axle.defects::INVARIANT`) — regression-grade prose, 12 codes

Row prose form: `<code> is a <CLASS> defect: its condition is <cond>; its message is
\`<message>\`` (double-quoted because of `: `). **H07 carries an inner WHEN guard.**

| code | class | condition | byte-exact message |
|---|---|---|---|
| H01 | MAJOR | bearing-temp > 80 | `bearing temp above 80 absolute alarm` |
| H02 | MINOR | bearing-temp > 60 | `bearing temp above 60 warning` |
| H03 | CRITICAL | bearing-temp − ambient-temp > 70 | `differential above 70 hotbox` |
| H04 | MAJOR | bearing-temp − ambient-temp > 50 | `differential above 50 alarm` |
| H05 | MAJOR | bearing-temp − mate-temp > 40 | `side-to-side differential above 40` |
| H06 | CRITICAL | bearing-temp > 100 | `bearing temp above 100 burnoff` |
| H07 | MINOR | **WHEN mileage > 200:** bearing-temp − mate-temp > 25 | `aged bearing side differential above 25` |
| H08 | MAJOR | wheel-impact > 140 | `wheel impact above 140 kN alarm` |
| H09 | CRITICAL | wheel-impact > 200 | `wheel impact above 200 kN critical` |
| H10 | MINOR | wheel-impact > 90 | `wheel impact above 90 kN warning` |
| H11 | MINOR | load > 130 | `load above 130 tonne limit` |
| H12 | MAJOR | mileage > 280 | `reprofile overdue beyond 280` |

**Precedence (governing defect, highest first; deliberately non-monotonic vs both class and
code number):** `H06, H09, H03, H08, H01, H05, H04, H12, H02, H11, H10, H07`. A well-formed
axle emits exactly **one** verdict line: `CLEAR` if no condition holds, else the single
highest-precedence triggered code.

**Class → exit-rank:** CRITICAL = 3, MAJOR = 2, MINOR = 1, CLEAR = 0, malformed record = 1.
Exit = max contribution over all records.

## Errors (`axle.audit::ERROR`, format owned by `axle.errors`)

Ordered checks, stop at and emit only the first failure; at most one error line per record;
all to stderr; byte-exact `<code> <detail>`:

| code | guard | byte-exact stderr line | exit |
|---|---|---|---|
| E10 | record does not split into exactly 7 fields | `E10 malformed record: expected 7 fields, got <n>` | 1 |
| E15 | 7 fields, field 1 not a well-formed axle-id | `E15 bad axle id: <axle-id>` | 1 |
| E20 | 7 fields, good id, some integer field not well-formed | `E20 bad number: <token>` (first offending, left→right) | 1 |
| E90 | **guard-less residual** (catch-all safety net) | `E90 unprocessable record: <record>` (`<record>` = the input line exactly as read) | 1 |

E10/E15/E20 are exhaustive over field-count/id/integer failures, so E90 is reachable only by
design as the residual (the `default-error-policy` regression form). Mark E90 explicitly
guard-less.

## The CONFORMS / USES / SEE references (every relation, with finding mapping)

| # | site | obligation shape | target | outer WHEN? | finding |
|---|---|---|---|---|---|
| R1 | `axle` (dispatch) INPUT | `SEE` ref-only | `./axle.audit@axle.audit` | – | doc cross-link (SEE never inlines) |
| R2 | `axle` (dispatch) INPUT | `SEE` ref-only | `./axle.roster@axle.roster` | – | doc cross-link |
| **R3** | `axle.audit::INPUT` | `MUST: validate each record against the shared axle reading record format` + CONFORMS | `./axle.shared@axle.record::INPUT` | no | **finding 1** — plain transclusion, used by both subcommands |
| **R4** | `axle.audit::RETURN` | `MUST: select the single governing defect by the registry precedence order` + CONFORMS | `./axle.shared@axle.defects::INVARIANT` | no | finding 1 — registry transclusion |
| R5 | `axle.audit::INPUT` | `USES` ref-only | `./axle.shared@axle.lines` | – | segmentation regression (USES may inline) |
| R6 | `axle.audit::ERROR` | `USES` ref-only | `./axle.shared@axle.errors` | – | error-line format regression |
| **R7** | `axle.roster::INPUT` | **guarded carrier** ref-only: `WHEN: a re-inspection line is flagged Y` + CONFORMS | `./axle.shared@axle.defects::INVARIANT` | **yes — flagged Y** | **finding 2** — outer guard × H07 inner guard |
| **R8** | `axle.roster::INPUT` | **bare whole-spec** CONFORMS ref-only | `./axle.audit@axle.audit` | no | **finding 3** — whole-spec conformance |
| R9 | `axle.roster::INPUT` | `MUST: …trust the verdict text audit writes; do not re-validate id or message` + USES | `./axle.audit@axle.audit::RETURN` | no | trust-boundary regression |
| R10 | `axle.roster::RETURN` | `MUST: classify each verdict's code by the registry` + CONFORMS | `./axle.shared@axle.defects::INVARIANT` | no | finding 1 — registry transcluded in consumer |

## `roster` input grammar (makes finding 2 observable)

`roster` reads two line types, distinguished by field 1:

- **Verdict line** (what `audit` emits): `<axle-id> CLEAR` or `<axle-id> <code> <message>`.
  Counts toward AXLES; tallied by stated verdict (CLEAR, or code→class via R10). Trust
  boundary R9: do not re-derive or re-validate — trust the text.
- **Re-inspection line:** field 1 is the literal `RECHECK`, then
  `RECHECK <axle-id> <Y|N> <bt> <amb> <mate> <impact> <load> <mileage>` (8 tokens after
  `RECHECK`'s id — i.e. id, flag, six readings). Counts toward AXLES. The **outer guard**
  "flagged for re-inspection" is true iff the flag field is `Y`. **R7 combined semantics:**
  re-derive the governing defect from the six readings via the transcluded registry **only
  WHEN flagged Y**; H07 within that registry fires only WHEN mileage > 200 — so H07
  contributes iff **(flag = Y) AND (mileage > 200)**. When flag = N, no re-derivation → the
  line tallies as CLEAR.

**roster output:** five lines, exit 0:
```
AXLES <total>
CLEAR <n>
CRITICAL <n>
MAJOR <n>
MINOR <n>
```

## Deciding measurements

### Finding 1 — is a CONFORMS-referenced obligation enforced at all? (`audit`)
| batch | stdin record | CORRECT stdout / stderr / exit | divergence if CONFORMS ignored |
|---|---|---|---|
| O1 | `a1 40 20 35 50 80 100` | – / `E15 bad axle id: a1\n` / 1 | emits `a1 CLEAR\n`, exit 0 |
| O2 | `AX1 040 20 35 50 80 100` | – / `E20 bad number: 040\n` / 1 | treats `040`=40, emits a verdict, exit 0 |
| O3 | `AX1 40 20 35 50 80` (6) | – / `E10 malformed record: expected 7 fields, got 6\n` / 1 | emits a verdict / different message |
| O4 | `AX1 40 20 35 50 80 100` | `AX1 CLEAR\n` / – / 0 | control |

4/4 byte-exact on O1–O3 ⇒ CONFORMS transcludes the contract reliably (weakens
`conforms-overloaded`). ≥2 emitting a verdict where the oracle rejects ⇒ CONFORMS read as an
ignorable pointer (confirmed).

### Finding 2 — the four guard-combination corners (`roster`, isolating H07)
Readings chosen so the ONLY candidate is H07: `bt 50, amb 20, mate 20, impact 50, load 80`
(bt−mate = 30 > 25; nothing else triggers), mileage per corner. H07 inner = mileage > 200.

| corner | flag | mileage | RECHECK line | H07 fires? | MINOR delta |
|---|---|---|---|---|---|
| C-TT | Y | 250 | `RECHECK AX1 Y 50 20 20 50 80 250` | **yes** | +1 |
| C-TF | Y | 100 | `RECHECK AX1 Y 50 20 20 50 80 100` | no (inner false) | 0 |
| **C-FT** | **N** | 250 | `RECHECK AX1 N 50 20 20 50 80 250` | **no (outer false)** | 0 |
| C-FF | N | 100 | `RECHECK AX1 N 50 20 20 50 80 100` | no | 0 |

Only **C-TT** moves a count. A model that drops the **outer** guard fires H07 on **C-FT**
(MINOR 1 vs 0). A model that drops the **inner** guard fires H07 on **C-TF**. Only a correct
AND passes all four. ≥2 diverging on C-FT = the strong silent-guard-drop signal that would
move guard combination into the language `Reference` spec.

### Finding 3 — bare whole-spec CONFORMS observation (`roster`)
| batch | roster stdin | CORRECT (pointer / trust holds) | divergence (enforce all audit slots) |
|---|---|---|---|
| rep_overvalidate | `AX1 CLEAR\nAX2 H06 bearing temp above 100 burnoff\n` | `AXLES 2\nCLEAR 1\nCRITICAL 1\nMAJOR 0\nMINOR 0\n` / – / 0 | re-runs audit's 7-field validation on the short verdict lines → E10/E15 to stderr or drops them from the tally |

Plus the static contradiction (grammar allows bare whole-spec; `InlineConforms` rejects
`conforms_no_slot`) and ≥2 cold NOTES.md flags of "bare CONFORMS meaning unclear" =
corroboration even if `rep_overvalidate` passes 4/4.

## Oracle design (`oracle/grade.py`, clone round-03 harness shape)

Embedded authoritative `simulate_audit` / `simulate_roster`; byte-exact stdout/stderr/exit
via `surrogateescape`; `--self-check` prints `SELFTEST OK`; `--cmd "<argv>"` runs a candidate
binary over every batch and prints `SCORE: <passed>/<total>`. ~34 batches:

- **AUDIT (~20):** empty; clean×3; single MINOR/MAJOR/CRITICAL; precedence CRITICAL-over-MAJOR;
  precedence MINOR-over-MAJOR (distinguishes prose order from class/code shortcuts);
  multi-code governing-by-precedence; H03 differential; H05 side differential; **H07 inner
  guard ON (mileage 250 → H07) and OFF (mileage 100 → CLEAR)**; E10 wrong count; E15 bad id;
  E20 leading-zero/non-digit; ordered-check id-over-number; tab-as-data (1 field → E10 got 1);
  CRLF (`\r` in last token → E20); runs/lead/trail spaces stripped → CLEAR; no-trailing-NL,
  blank-interior line (E10 got 0), lone `"\n"` (one zero-field record → E10 got 0), empty
  input (no output); full ladder → exit 3.
- **ROSTER (~9):** empty → all-zero summary; clean canonical source; mixed-class canonical
  source; **rep_overvalidate**; the four corners **C-TT / C-TF / C-FT / C-FF**; a mixed
  batch interleaving all four corners with ordinary verdicts (exactly one MINOR delta).
- **DISPATCH (3):** `argv[1]` empty / unknown (`verify`) / wrong-case (`AUDIT`) → usage,
  exit 2.
- **PIPELINE (~4):** candidate's `audit` stdout fed to candidate's `roster` (empty / clean /
  mixed / with-errors) — verifies the audit stdout is self-consumable (roster never sees
  audit stderr).

**SELFTEST:** registry/precedence integrity (PRECEDENCE lists each code exactly once;
condition/class/message key-sets agree); each corner hand-expectation; H07 on/off;
E10/E15/E20 byte-exact; ordered id-over-number; CRLF-as-data; chain consistency. A reference
implementation (`oracle/ref.py`) must score **full** before the panel runs.

## Procedure

1. Author the three spec files + `oracle/grade.py` + `oracle/ref.py` + `HOWTORUN.txt` from
   this plan (templates: `test-specs/round-03-vault/spec/*` and `…/oracle/grade.py`).
2. Validate: schema + ref resolution (`/tmp/check_all_refs.py` repo-wide stays CLEAN);
   `grade.py --self-check` → `SELFTEST OK`; `grade.py --cmd "python3 oracle/ref.py"` → full.
3. **Commit `round-04` spec + oracle BEFORE any agent runs.**
4. Author `experiment/round-04/prompt.md` (cold, generic build-from-spec; no oracle hint).
5. Run the panel cold — unique `/tmp/yass-r04-<model>-<rand>` per model, copy in ONLY
   `spec/*.yass.yaml`, background + concurrent, via `script/agent`.
6. Grade each candidate with `grade.py --cmd`, capture stdout/stderr/exit + the model's
   NOTES.md. Score.
7. Diagnose every miss spec-defect vs model-error; ≥2-model corroboration = strong. Write
   `round-04/results.md`.
8. Fix the source of truth; prune resolved content from `context/*`; route any irreducible
   tooling to TOOLING.md; update FINDINGS; commit `round-04`; update STATUS + LOG +
   convergence counter.

## Success criteria

- The probe isolates each CONFORMS finding to ≥1 batch whose pass/fail flips on the
  finding's behavior (O1–O3 for transclusion; C-TT/C-TF/C-FT for guard combination;
  rep_overvalidate for bare-slot over-enforcement).
- Reference impl full before the panel; oracle `SELFTEST OK`; repo refs CLEAN.
- Every miss is classified; ≥2-model signals drive SOT fixes; the round either produces
  actionable spec-defects (reset convergence) or, if 4/4 pass clean with no ≥2 NOTES
  ambiguity, increments the convergence counter.
