# Round 4 — Results

Probe `test-specs/round-04-axle` (trackside wayside hotbox-detector axle audit: one binary,
two subcommands `audit`/`roster`). Panel run cold, isolated `/tmp` workspace per model, only
`spec/*.yass.yaml` copied in. Prompt `experiment/round-04/prompt.md`. This round targeted the
**CONFORMS cluster** — the language's most-overloaded relation, never exercised in Rounds 2–3
(which used `USES`/`SEE` for shared conventions).

## Grades (byte-exact stdout/stderr/exit across 42 oracle batches)

Oracle `--self-check` → SELFTEST OK; `grade.py --cmd <ref.py>` → 42/42 before the panel.

| model | language | duration | score |
|-------|----------|---------:|------:|
| gpt-5.5-extra-high | Python 3 | 108 s | **42/42** |
| gemini-3.1-pro | Python 3 | 147 s | **42/42** |
| claude-opus-4-8-thinking-high | Python 3 | 380 s | **42/42** |
| composer-2.5 | Python 3 | 91 s | **41/42** |

**167/168, one functional miss (composer, model-error — see below).** All four chose Python;
every `HOWTORUN.txt` was `python3 axle.py` with no build step. Every CONFORMS-targeted obligation
was implemented correctly cold by 4/4: the R3 record-format transclusion (E10/E15/E20 byte-exact),
the R4/R10 12-code registry transclusion read in two consumers, the R7 guard-combination corners,
and the R8 bare whole-spec conformance.

The signal this round is split between one new latent defect surfaced unanimously in `NOTES.md`
and three CONFORMS findings the probe resolved or refuted.

## Headline finding — `residual-reachability` (corroborated 4/4, STRONG, new)

The `audit::ERROR` slot carried a deliberately guard-less `E90` residual
(`E90 unprocessable record: <record>`) as the `default-error-policy` regression form — the
catch-all safety net. But E10 (wrong field count), E15 (bad axle id), and E20 (bad integer
field) are **exhaustive** over field-count / id / integer failures: every record either splits
into ≠7 fields (E10), or has 7 fields with a bad id (E15) or a bad integer (E20), or is
well-formed and yields a verdict. The guards partition the input completely, so the E90 residual
set is empty — **E90 is dead, it can never fire.** The oracle confirms it: `E90` has zero
references in the reference implementation.

All four models independently flagged this, each deriving the same unreachability argument:

- **gpt:** "treated `E90` as unreachable given E10/E15/E20 cover all malformed shapes; followed
  the explicit well-formed-record invariant."
- **gemini:** "any record passing all three checks is by definition well-formed and must yield a
  verdict line instead of an error line — `E90` cannot be reached."
- **opus:** "the `E90` residual is unreachable in practice; E10/E15/E20 are exhaustive over the
  malformed cases, so it cannot fire."
- **composer:** "No third rejection condition is defined beyond field count, id, and integers, so
  `E90` appears unreachable; emitting it would violate the well-formed-record invariant."

Four-of-four convergence on "this obligation is dead" is a STRONG signal that the residual
discipline itself was incomplete. The prior rule (`default-error-policy`: *always state a
residual*) had **no reachability caveat** — it actively encouraged stating a residual even where
the guards leave no remainder. This is the necessary converse of `default-error-policy`: a
residual is meaningful only when the guarded obligations leave some input unmatched; when they
are exhaustive, a guard-less catch-all is a contradiction every careful reader flags.
**Spec-defect, not model-error.**

Fixed in source of truth this round:
- `yass.yass.yaml` `Slot.ERROR` — a guard-less residual `MUST-NOT` be carried when the guarded
  obligations already account for every possible input.
- `context/GUIDANCE.md` (*Error obligations* section) — third rule: do not state a residual the
  guards have already exhausted; confirm a genuine remainder before adding one, else assert the
  exhaustiveness.
- `context/yass-reference.md` (`ERROR`-slot paragraph) — a residual is meaningful only when the
  guards leave inputs unmatched; an exhausted residual is dead.

## CONFORMS finding 3 — `conforms-bare-slot-meaning` resolved (grammar wins)

R8 placed a **bare whole-spec** `CONFORMS: ./axle.audit@axle.audit` (no `::SLOT`) on
`axle.roster::INPUT` — the exact shape the `cli.query` tooling rejected as
`yass.query.conforms_no_slot` (exit 1) while the `RefTarget` grammar **allows** it (`::SLOT` is
optional). That static contradiction was the finding.

All four models read R8 correctly as "conform to `audit`'s whole contract" — none over-validated
the short verdict lines against audit's 7-field record format, none dropped them from the tally;
the `rep_overvalidate` batch passed 4/4. The grammar's reading (whole-spec CONFORMS = a
conformance reference to the entire spec, not a slot transclusion) is the intuitive one, and the
language already said CONFORMS must "match the referenced spec **or** slot" (`yass.yass.yaml`
`Reference` RETURN). The contradiction was the tooling overriding a meaning the language already
carried.

**Resolved in the grammar's favor** — no schema change needed:
- `yass.yass.yaml` `Reference` SIDE-EFFECT — a slot-targeted CONFORMS is inlined; a whole-spec
  CONFORMS `MUST-NOT` be inlined (it is a conformance reference, not a slot transclusion).
- `spec/cli.query.yass.yaml` — added a whole-spec gate (leave the carrier unchanged, inline
  nothing) as the first RETURN obligation; scoped the reference-only replace rule to slot-targets;
  **removed** the `conforms_no_slot` ERROR slot.
- `spec/cli.errors.yass.yaml` — deleted the `yass.query.conforms_no_slot` registry entry; the
  INVARIANT *MUST-NOT reuse a retired code* makes deletion a clean retirement.
- `context/yass-reference.md` — ref-only note + relations table + prose now describe the two
  aspects of one meaning (match vs inline) and the whole-spec non-transcluded conformance reading.

## CONFORMS finding 2 — `conforms-inlining-semantics-misplaced` resolved

R7 was a **guarded carrier** CONFORMS (`WHEN flagged Y`) pointing at `axle.defects::INVARIANT`,
whose H07 obligation carries its **own** inner guard (`WHEN mileage > 200`). The four guard
corners (C-TT/C-TF/C-FT/C-FF) isolate H07 so it fires only under **(flag = Y) AND
(mileage > 200)** — only C-TT moves a count. A model dropping the outer guard fires H07 on C-FT;
one dropping the inner guard fires it on C-TF. **All four passed all four corners** — the AND
semantics held cold.

The defect was that the guard-combination rule lived **only** in the application spec
(`cli.query.InlineConforms`, as a literal-render rule a cold reader never sees), not at the
language level. Promoted the **semantic** conjunction to the language:
- `yass.yass.yaml` `Reference` SIDE-EFFECT — when an inlined obligation carries its own guard and
  the carrier also carried one, the obligation applies only when **both** guards hold (carrier
  conjoined with inner).
- `context/yass-reference.md` — new bullet: guards conjoin when an inlined obligation is itself
  guarded; how a tool renders the combined guard text is the tool's concern.
- The literal `" and "` rendering rule stays in `cli.query` as a tooling concern (the meaning is
  now at the language level; the rendering is not).

## CONFORMS finding 1 — `conforms-overloaded` refuted behaviorally (4/4 clean)

The deciding measurement was whether a cold model **enforces** a CONFORMS-referenced slot's
obligations at the referencing site (must-match contract) or reads CONFORMS as an ignorable
`SEE`-like pointer. O1–O3 flip on this: if CONFORMS is ignored, the model emits a verdict where
the oracle rejects (`a1` → E15, `040` → E20, 6 fields → E10). **4/4 byte-exact on O1–O3** — every
model enforced the transcluded record contract. No NOTES entry showed any match-vs-inline
confusion. The dual aspect (a hard must-match contract whose inlining is *how* the match is made
checkable in place) is coherent and was read coherently by every model.

No relation split, no schema change — the hypothesized overload defect is **refuted**. Only a
documentation consolidation in `yass-reference.md` (describing the two aspects of one meaning).
Marked `wontfix (refuted round-04)` in FINDINGS.

## Regression re-verification (prior fixes held)

All exercised by the probe, not asserted — all passed 4/4:

- **`closed-set-dispatch-residual` (round-02).** Missing / unknown (`verify`) / wrong-case
  (`AUDIT`) `argv[1]` → `usage: axle {audit|roster}` to stderr, exit 2. The DISPATCH batches
  passed 4/4. Held.
- **`input-segmentation-completeness` + `segmentation-terminator-mechanics` (round-01/03).**
  ASCII-space field split (runs of spaces stripped), LF record separator, strip exactly one
  trailing LF, empty input = zero records, blank-interior line = E10 got 0, lone `"\n"` = one
  zero-field record (E10 got 0), tab/CRLF as data. All passed 4/4 except composer's lone-`"\n"`
  miss (below). Held.
- **`dataflow-invisible` + `trust-boundary-violation-residual` (round-02/03).** R9 had `roster`
  consume `audit`'s output via `USES …::RETURN` + a stated trust boundary (trust the verdict text,
  do not re-validate id/message). No model over-validated; the `rep_overvalidate` and PIPELINE
  batches passed 4/4. Held.
- **`cross-cutting-single-home` (round-02).** One `axle.shared` owned the wire protocol, record
  format, defect registry, precedence order, class→exit map, and error-line format; both
  subcommand specs referenced it. No drift across 4/4. Held.
- **structured-obligation cluster (`wontfix`).** The 12-code prose defect registry with a
  deliberately non-monotonic precedence chain (`H06, H09, H03, H08, H01, H05, H04, H12, H02, H11,
  H10, H07`) and byte-exact messages was reproduced byte-exact by 4/4; every precedence batch
  passed. Closed-as-refuted a third time.

## The one miss — composer `aud_lone_newline` is a MODEL-ERROR

composer's sole miss: a lone `"\n"` (one byte) on `audit` stdin. The oracle treats it as one
empty record → `E10 malformed record: expected 7 fields, got 0` / exit 1; composer treated it as
zero records → exit 0, empty stderr. A literal reading of `axle.lines` supports the oracle — only
*zero bytes* is empty input; one byte is not zero bytes, so a lone `"\n"` is one (empty) record.
opus's NOTES item independently agreed with the oracle's reading. Lone 1/4 deviation, contradicted
by a literal reading and by another model → **model-error, not a yass defect.**

## Latent, untested — roster consumer divergence (NOT a new finding)

NOTES surfaced two out-of-contract `roster` input divergences the oracle never exercised: whether
a blank line counts toward `AXLES` (gpt counts every line; gemini/opus do not), and how a
malformed `RECHECK … Y` line tallies (gemini → CLEAR; others → no class). Both are out-of-contract
`roster` input lines already covered by the resolved `closed-set-dispatch-residual` +
`trust-boundary-violation-residual` guidance (GUIDANCE *Composition* even cites a prior
3/4-model instance of exactly this). This is a probe-authoring lapse — the oracle lacked a
blank-line / malformed-`Y` roster batch — not a yass-language gap. Noted for round-05 probe
design; not actionable as a language change.

## Convergence

Round 4 produced **one new actionable spec-defect** (`residual-reachability`), fixed in source of
truth this round. The convergence counter therefore **stays at 0 / 2**. The CONFORMS cluster is
now exhausted (two resolved, one refuted). Round 5 is the final scheduled round; it pivots to the
remaining open findings (composition/reference sequencing) and re-verifies the round-04 fixes.
