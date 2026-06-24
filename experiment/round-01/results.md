# Round 01 — Results

Probe: `test-specs/round-01-berth` — a port-berth assignment CLI in a fresh,
non-famous domain. The probe was built to stress the **structured-obligation
cluster** in one spec: a prose error registry (E10–E90, each packing code +
message template + exit contribution), an 8-step "emit the first matching error
and stop" priority chain stated as one prose sentence, USES-only cross-spec
sequencing, positional input fields named only in prose, a guard-less E90
catch-all, "at most one error line per record" cardinality, and several MUST-NOT
obligations.

Panel: `gpt`, `gemini`, `opus`, `composer`, each run **cold** in a unique
throwaway `/tmp` workspace containing only `spec/*.yass.yaml`. The private oracle
was never copied in. Grading is black-box: stdin → (stdout, stderr, exit),
compared byte-for-byte against the oracle's embedded reference across all batches.

## Grades

| model | pass | duration | exit | result |
|-------|------|----------|------|--------|
| gpt (gpt-5.5-extra-high) | 12/12 | 109s | 0 | clean |
| gemini (gemini-3.1-pro) | 12/12 | 135s | 0 | clean |
| opus (claude-opus-4-8-thinking-high) | 12/12 | 235s | 0 | clean |
| composer (composer-2.5) | 10/12 | 175s | 0 | 2 segmentation misses |

Grades are **honest**: the oracle was hardened mid-round (see below) before final
grading, which is what dropped composer from an initial 10/10 to 10/12.

## Headline meta-finding

The structured-obligation cluster — the highest-leverage language change on the
ledger, and the explicit target of this round — caused **zero functional misses**
across four strong models at this spec scale. The prose error registry, the prose
8-step priority chain, the USES-only sequencing, and the positional-fields-in-prose
parse were all read correctly and implemented byte-exactly by every model.

The cost of these constructs was **inference, not correctness**. Every model spent
visible reasoning reconstructing the priority order, the record/field boundaries,
and (most of all) the routing of the central domain rule through the catch-all. The
constructs are *inference-expensive but survivable* at this scale.

Implication for the cluster findings (`mapping-valued-obligations`,
`error-table-structured`, `priority-chains-prose`, `error-cardinality-implicit`):
they remain real language-design opportunities but are **not** functional defects
at small scale. They are now annotated `probed round-01` and stay `open`. To find
their breaking point, later rounds must scale spec size / interconnection, or
target the specific constructs that demonstrably forced the most inference, rather
than re-probing at this scale.

## Corroborated actionable findings

### `default-error-policy` — strong signal (4/4), RESOLVED

The single most load-bearing domain rule — a request whose window falls *outside a
dock's operating hours* must be rejected — had no dedicated obligation in the spec.
It was reachable only by inferring that such a request "matches none of the errors
listed above" and therefore routes to the guard-less E90 catch-all. All four models
flagged this inference explicitly in their reasoning before getting it right. A
guard-less catch-all that silently absorbs a *foreseeable, named* condition forces
every reader to rediscover a domain rule.

Fix applied to source of truth:

- `yass.yass.yaml` `Slot.ERROR` — added the residual semantics as obligations: a
  guarded ERROR obligation is a specific foreseeable failure; a guard-less ERROR
  obligation is the **residual** for any failure not matched by a guarded
  obligation in the same slot.
- `context/yass-reference.md` — documents the same residual reading in prose in the
  Slots section.
- `context/GUIDANCE.md` — new section "Error obligations: guarded for the
  foreseeable, guard-less for the residual": always state a residual, and never fold
  a foreseeable named failure (with a distinct observable outcome) into it.
- Pruned the now-promoted feedback from `context/RECOMMENDATIONS.md` Part 1 §4
  (default-policy paragraph) and Part 2 §2 (deleted in full).

### `input-segmentation-completeness` — corroborated (3/4), RESOLVED (new finding)

Three of four models flagged ambiguity in how input is segmented — trailing
newline, blank interior line, and the exact field separator. The spec stated "runs
of one or more ASCII space characters" for fields and "a single newline" for
records, but the *completeness* of the boundary contract is what models had to
reason about. New GUIDANCE section "Input segmentation: specify every boundary"
requires, for each level of segmentation, naming: the exact separator and its
character class (ASCII space vs. general whitespace), empty input, a blank interior
unit, and a leading/trailing/repeated separator.

## composer 10/12 — model-error, not spec-defect

composer failed exactly two batches, both segmentation:

- `whitespace_tab` — split fields with Go's `strings.Fields`, which splits on all
  Unicode whitespace, so a tab-separated line parsed as 5 fields instead of being
  rejected as one malformed field (E10 got 1).
- `crlf_record` — stripped a trailing `\r` (via `TrimSpace`-style handling), so a
  CRLF-terminated record's window field parsed as clean instead of producing the
  `\r`-bearing E23.

These are **model-errors**: the obligations ("runs of one or more **ASCII space**
characters" and records "separated by a **single newline**") are unambiguous.
composer's own NOTES.md acknowledged the `strings.Fields` shortcut as assumed
"equivalent for ASCII-only inputs implied by the spec" — i.e. a deliberate
shortcut, not a spec gap. The actionable item is the methodology fix below, not a
yass-language change.

## Methodology: oracle coverage gap (FIXED)

The original oracle used 10 batches, all ASCII-space-separated and LF-terminated.
Under those inputs composer's non-conformant splitting was unobservable and it
scored 10/10. The gap was a property of the *test inputs*, not the candidate.

Fix: added `whitespace_tab` and `crlf_record` batches plus matching SELFTEST
entries (committed in the oracle, `0de09cf`). After hardening, composer correctly
graded 10/12 and the other three held at 12/12. The oracle's self-check still
passes, so the reference itself is trustworthy.

Standing discipline for future rounds: every segmentation obligation must be
exercised with at least one off-spec separator (tab, CRLF, NBSP, leading/trailing/
repeated separator), or a conformance bug like this stays invisible.

## Probed-but-clean findings

- `error-cardinality-implicit` — "at most one error line per record" stated in prose
  was honored by all four models (the `interleave_mustnot` and `priority_ties`
  batches pass). No functional defect at this scale.
- `mustnot-undertested` — the CLI shape made the MUST-NOTs observable (no `OK` line
  for a rejected record; no error line for an accepted one) and the oracle's full
  stdout/stderr diff tested them. All four passed. This is concrete evidence that
  MUST-NOT obligations become black-box-testable once a spec is CLI-shaped; the
  language-level "testable vs. environmental" distinction remains open.

## Findings delta

- Resolved: `default-error-policy`, `input-segmentation-completeness` (new).
- Annotated `probed round-01` (still open): `mapping-valued-obligations`,
  `priority-chains-prose`, `error-table-structured`, `error-cardinality-implicit`,
  `mustnot-undertested`.
- New actionable spec-defect findings this round: 1 (`input-segmentation-completeness`).
