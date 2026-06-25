# Round 3 — Results

Probe `test-specs/round-03-vault` (bank-vault time-lock door annual certification:
one binary, two subcommands `certify`/`report`). Panel run cold, isolated `/tmp`
workspace per model, only `spec/*.yass.yaml` copied in. Prompt
`experiment/round-03/prompt.md`.

## Grades (byte-exact stdout/stderr/exit across 35 oracle cases)

Oracle `--self-check` → SELFTEST OK; `grade.py --cmd <ref.py>` → 35/35 before the panel.

| model | language | duration | score |
|-------|----------|---------:|------:|
| gpt-5.5-extra-high | Python 3 | 152 s | **35/35** |
| gemini-3.1-pro | Python 3 | 199 s | **35/35** |
| claude-opus-4-8-thinking-high | Python 3 | 200 s | **35/35** |
| composer-2.5 | Python 3 | 63 s | **35/35** |

**140/140, zero functional misses.** All four chose Python. Every targeted obligation
— the 18-code prose defect registry, the deliberately non-monotonic precedence order,
one-verdict-per-door cardinality, the E10/E15/E20 ordered validation, the
`certify | report` dataflow trust boundary, and off-spec segmentation (tab/CRLF/NBSP) —
was implemented correctly cold.

The signal this round is in `NOTES.md`, not in the scores: at 100% the friction is
latent spec defects flagged by multiple models (≥2-model corroboration = strong signal).

## Headline meta-finding — the structured-obligation cluster is refuted a second time

This probe was authored specifically to break the cluster at a scale Rounds 1–2 never
reached, with an adversarial twist so the cheapest shortcut yields a *wrong* answer:

- **`priority-chains-prose`** — an **18-entry precedence order**, deliberately NOT
  monotonic with severity class and NOT in code-number order, stated as one prose list.
  The oracle's precedence batches (multi-defect doors) distinguish the prose order from
  both "most severe wins" and "lowest code wins". A shortcut yields a wrong verdict.
  **Result: 4/4 read the exact prose order; every precedence batch passed.** No model
  shortcut.
- **`error-table-structured`** — an **18-row registry** (code, class, condition, message)
  packed into prose `MUST` obligations. **Result: 4/4 reproduced every byte-exact message
  and every threshold/class.** No paraphrase, no miscopy.
- **`error-cardinality-implicit`** — "at most one verdict per door" / "at most one
  processing-error line per record" stated only in prose. **Result: 4/4 emitted exactly
  one line per record.** No model emitted one line per triggered defect.
- **`mapping-valued-obligations`** (root) / **`error-code-refs`** — each registry row is a
  logical 4-field record forced into one scalar string, and codes (V01–V18) are cited as
  bare literals read in two places (certify verdict + report tally). **Result: the packing
  caused no copy errors and the code→class→message mapping survived being read in both
  places for 4/4.**

Round 1 found the cluster "inference-expensive but survivable" at small scale (E10–E90,
8-step chain). Round 3 scales it 2× in registry size, makes the precedence chain
adversarial, and adds a second consumer that re-reads the codes — and still finds **zero
functional misses and, this round, not even a NOTES-flagged ambiguity about the registry
or precedence.** opus listed the 18-code registry, the fixed precedence order, and the
severity ranks matter-of-factly under "what was implemented"; none of the four flagged
any of them as hard or ambiguous.

**Verdict: the cluster's hypothesized *correctness* defect is refuted across two rounds
of scaled probing.** The remaining motivation for a structured/ordered/table construct is
ergonomic (inference cost, readability, lint-ability, code-reference validation), which is
**not falsifiable by black-box one-shot outcome** and was not corroborated by NOTES
ambiguity at scale. Per the experiment charter (find spec defects that cause cold one-shot
*failures*), the cluster is closed as not-a-correctness-defect; the ergonomic/validation
angle is routed to tooling (`lint-anti-slop`), not a language change. See FINDINGS.

## New finding B — trust-boundary violation residual (corroborated 3/4, STRONG)

`vault.report` INPUT carries the round-02 `dataflow-invisible` fix in action — it names the
producer and states the trust boundary:

> `MUST: read standard input as the verdict lines vault certify writes to standard output …
> rely on that guarantee and do not re-validate the identifier, the field layout, or the
> message text` (`USES ./vault.certify@vault.certify::RETURN`)

That fix held — no model over-validated, all used the pointer. **But the spec states what
`report` *trusts* without stating what it *does when the guarantee is violated*** (a line
that is not `<id> PASS` or `<id> <code> <message>` — a blank line, or an unknown code). The
RETURN slot ties `<total>` to "the number of verdict lines read" and the class counts to
"verdict lines whose governing defect has severity class …", but a classless / malformed
line has no governing defect, so its contribution to `<total>` and to the class tallies is
undefined.

Three of four flagged this and **guessed** — convergently on outcome, divergently on
reasoning:

- **gemini:** "even if an empty line is encountered, it should be treated as a record (and
  thus a verdict line), incrementing the total count, even though it wouldn't have been
  produced by `vault certify`."
- **opus (#2):** "Malformed/blank line in `report` input. Not expected per the consumer
  guarantee. I chose to still count such a record toward `<total>` but classify it as
  neither PASS nor any severity class."
- **composer (#2):** "Unknown verdict codes in `report`: counted in `DOORS` but not in
  PASS/…/MINOR. The spec assumes input comes from `certify`, so this should not occur."

(gpt restated the trust rule without flagging the residual.) All three landed on the same
behavior — count toward `<total>`, classify as nothing — but that convergence is luck of
shared defaults, not the spec. **Spec-defect, not model-error.**

This is the same residual principle already fixed for `ERROR` (guard-less catch-all) and
`closed-set-dispatch` (out-of-set value): a rule that *delegates validation to an upstream
guarantee* must also state the behavior when that guarantee does not hold — even if only to
declare it unspecified. The round-02 `dataflow-invisible` fix made the consumer state which
guarantees it trusts; it did not require stating the **violation residual**. Distinct
sub-problem; `dataflow-invisible` stays resolved, this is new.

## New finding A — optional-terminator mechanics + lone-separator (corroborated 2/4)

`vault.lines` pins most segmentation boundaries (separator + byte class, empty input = zero
records, blank interior unit = zero-field record, runs-of-spaces field splitting, off-spec
bytes are data — the round-01 `input-segmentation-completeness` checklist). But the
optional trailing terminator is stated as a bare permission:

> `MAY: accept a single trailing newline after the final record without counting it as an
> additional empty record`

This leaves the **mechanics** unstated: is exactly *one* trailing `0x0A` absorbed (and any
further blank lines kept as zero-field records), or all of them? And the **degenerate
input** — a lone `"\n"` (one byte, not empty input), or input that is only separators — is
not addressed. Two models constructed the precise rule themselves:

- **opus (#4):** "after byte-splitting on `0x0A`, drop exactly one trailing empty element if
  present. Thus `"…\n"` => N records, `"…\n\n"` => the extra blank line is a genuine
  zero-field record, and a lone `"\n"` (one byte, not empty input) => one zero-field record."
- **gpt:** "a single final newline is accepted without creating an extra empty record …
  A blank line that remains after this single optional trailing newline removal is treated
  as a zero-field record."

Both converged on the oracle's rule (`_split_records`: strip exactly one trailing `\n`, then
split; `"\n"` → one zero-field record), but the spec text does not say it — each had to
derive it. **Spec-defect, not model-error.** It refines the resolved
`input-segmentation-completeness` checklist, which lists "leading/trailing/repeated
separator" without forcing the *count* absorbed by an optional terminator or the
all-separators / lone-separator degenerate case.

## Regression re-verification (prior fixes held)

All exercised by the probe, not asserted — all passed 4/4:

- **`dataflow-invisible` (round-02).** `report` consumed `certify`'s output via the
  `USES …::RETURN` pointer and the stated trust boundary; no model re-validated the trusted
  fields, none flagged the boundary as invisible. Held. (The *violation* residual above is a
  new, adjacent gap, not a regression.)
- **`closed-set-dispatch-residual` (round-02).** The shared `vault` dispatch states the
  missing/unknown-`argv[1]` case (`usage: vault {certify|report}` to stderr, exit 2). All
  four converged on it exactly; the DISPATCH oracle batches passed 4/4. Held.
- **`input-segmentation-completeness` (round-01).** Tab/CR/NBSP-as-data, runs-of-spaces
  field splitting, empty input = zero records all handled by 4/4; the off-spec segmentation
  batches passed. Held. (Finding A above is a newly-probed *edge* of the same area, not a
  regression of what was fixed.)
- **`cross-cutting-single-home` (round-02).** One `vault.shared` owned the wire protocol,
  record layout, defect registry, precedence order, class→exit map, and error-line format;
  both subcommand specs referenced it. All four implemented the shared rules consistently
  with no drift. Held.

## Non-issues (model-reasoning, not spec-defects)

- **"read all of stdin before output" ordering** (gemini, 1/4) — interpreted as
  read-to-EOF-then-emit. The obligation is unambiguous; single-model restatement, not a
  defect.
- **Unbounded integers** (opus, 1/4) — fields have no stated upper bound; opus used
  arbitrary-precision int. Single-model, no functional impact; not actionable as a strong
  signal. (A general "state bounded ranges" guidance point is plausible but uncorroborated.)
- **Character encoding** — composer decoded Latin-1, others used bytes directly; all
  byte-exact-correct. The MUST-NOT on tab/CR/NBSP-as-separator forced byte-awareness and all
  four complied. No miss.

## Convergence

Round 3 produced **two new actionable spec-defect findings** (`trust-boundary-violation-residual`,
`segmentation-terminator-mechanics`), both fixed in source of truth this round. The
convergence counter therefore **stays at 0 / 2**. The primary probe target (the
structured-obligation cluster) is closed as refuted, so Rounds 4–5 pivot to the remaining
open findings (composition/reference and the CONFORMS cluster are the next-highest leverage).
