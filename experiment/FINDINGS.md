# FINDINGS ledger

Live working set for the experiment. Status: `open` | `probing` (under test this
round) | `resolved` (fixed in source of truth + pruned from context/*) |
`tooling` (moved to TOOLING.md) | `wontfix` (explicit non-goal).

Ingested from `context/{OPEN,GUIDANCE,SPEC}-FEEDBACK.md`,
`RECOMMENDATIONS.md`, `NOTES.md`, `FIXES.md`, `IDEAS.md`, `FUTURE.md`,
`MAN-ALIGNMENT.md` (26 distinct findings after dedup; +1 round-01
`input-segmentation-completeness`, +1 round-02 `closed-set-dispatch-residual` =
28 total). Recurrence: `universal`
(≈all 7 prior cross-language impls) > `repeated` (several sources) > `single`.

The **structured-obligation cluster** (`mapping-valued-obligations` as root, with
`error-table-structured`, `error-code-refs`, `priority-chains-prose`,
`error-cardinality-implicit`) was the hypothesized highest-leverage language change.
**Refuted as a correctness defect across two rounds of scaled probing** (round-01
small prose registry; round-03 18-code registry + deliberately non-monotonic precedence
+ one-verdict cardinality): zero functional misses both times and, at scale, not even a
NOTES-flagged ambiguity. Closed `wontfix` for the experiment charter (find spec defects
that cause cold one-shot *failures*). The remaining motivation — inference cost,
readability, code-reference validation — is ergonomic, not falsifiable by black-box
one-shot outcome, and is routed to tooling (`lint-anti-slop`), not a language change.

## spec-definition

| id | recurrence | status | title / lever |
|----|-----------|--------|---------------|
| `mapping-valued-obligations` | repeated | wontfix (refuted ×2) | Obligation values are scalar-only; proposed allowing mappings to carry code/priority/metadata (claimed ROOT enabler). Round-01: prose-packed error registry → 0 functional misses (small scale). **Round-03: an 18-row registry packed into scalar prose obligations caused 0 copy errors across 4 models; the code→class→message mapping survived being read in two places (certify + report).** Hypothesized correctness defect refuted; residual value is ergonomic/lint only → `lint-anti-slop`. |
| `priority-chains-prose` | universal | wontfix (refuted ×2) | Ordered "emit first matching" chains live as prose; proposed a PRIORITY/ORDERED construct. Round-01: 8-step prose chain read correctly by 4/4. **Round-03: an 18-entry precedence order, deliberately non-monotonic with both severity class and code number, was read exactly by 4/4 — every precedence batch (the deciding measurement distinguishing the prose order from "most severe wins" / "lowest code wins") passed.** Correctness defect refuted at adversarial scale; residual value ergonomic only (idiom lint → `lint-anti-slop`, TOOLING.md). Round-03 prune: removed the corresponding "Treat an ordered list as ordered and step-addressable" recommendation (Part 2 §1) from `context/RECOMMENDATIONS.md`. |
| `cross-spec-sequencing` | universal | open (probed round-02) | No way to express execution sequencing/preconditions between specs; USES conflates call/depend/after. Need REQUIRES/AFTER. → yass.yass.yaml, schema, reference. Round-02: the header-gate idiom (E40/E41 + write-nothing + reject-all + exit 2, paired with a `USES …::RETURN` pointer to the producing stage) expressed data-pipeline sequencing correctly for 4/4. Re-scoped: the dataflow case is covered; non-dataflow preconditions (REQUIRES/AFTER) remain, and USES now also carries a consumes-output-of meaning (`::RETURN`) — reinforces the USES overload. |
| `dataflow-invisible` | universal | resolved | Specs described components, not the dataflow/trust boundary between them. **Round-02 (4/4 STRONG):** all four models flagged that grade/pack consume tally's output without the spec stating which upstream guarantees they trust vs re-validate; all guessed alike → 38/38, but the spec was silent and the `USES …::RETURN` pointer carried only structural meaning. Fixed: `yass.yass.yaml` (`Reference` gives a slot-targeted USES a dataflow reading; `Slot.INPUT` requires naming a producing slot + stating the trust boundary), `yass-reference` (References + Slots), `GUIDANCE` ("Composition"). No schema change — `::SLOT` targets already valid. Pruned `RECOMMENDATIONS.md` Part 2 §2. |
| `trust-boundary-violation-residual` | repeated | resolved | A consumer that relies on an upstream guarantee "without re-validating" must also state what it does when that guarantee is **violated** (even if only to declare the behavior unspecified). **Round-03 (3/4 STRONG):** `vault.report` stated the trust boundary (the round-02 `dataflow-invisible` fix, which held) but said nothing about an out-of-contract line (blank / unknown code); gemini, opus, composer each flagged it and **guessed** the `<total>`/class behavior — convergent on outcome, divergent on reasoning. Extends the residual principle (ERROR catch-all, closed-set dispatch) to the trust boundary. Fixed: `yass.yass.yaml` (`Slot.INPUT` new violation-residual obligation), `yass-reference` (References, slot-targeted USES), `GUIDANCE` ("Composition"). Distinct from `dataflow-invisible` (which states *what is trusted*; this states *what happens when trust fails*). |
| `error-table-structured` | universal | wontfix (refuted ×2) | Large error registry forced into prose MUSTs (code+class+condition+message packed per string). Round-01: compact prose registry (E10–E90) byte-exact for 4/4. **Round-03: an 18-row registry (code, class, condition, byte-exact message) packed into scalar prose was reproduced byte-exact by 4/4 — no paraphrase, no miscopied threshold or class.** Correctness defect refuted at 2× scale; residual value is lint/readability → `lint-anti-slop`. |
| `error-cardinality-implicit` | repeated | wontfix (refuted ×2) | "at most one per file" vs "one per rule" vs "one per (X,Y) pair" is implicit prose; proposed ONCE-PER/EACH/DEDUP-BY. Round-01: "at most one error line per record" honored by 4/4. **Round-03: one-verdict-per-door AND one-error-line-per-record (both prose) honored by 4/4 — no model emitted one line per triggered defect on multi-defect doors.** Correctness defect refuted; residual value ergonomic only. |
| `error-code-refs` | single | wontfix (refuted ×2) | Error/defect codes cited as bare string literals in prose, unvalidatable by tooling; proposed a structured CODE key. **Round-03: codes V01–V18 cited as bare literals and read in two places (certify verdict emission + report class tally) survived intact for 4/4 cold impls — no broken mapping.** No correctness defect; the unvalidatable-by-tooling concern is real but is a lint capability, not a language defect → `lint-anti-slop`. |
| `conforms-overloaded` | universal | open | CONFORMS does contract+inlining+provenance; most bug-prone feature. Split assertion vs render-time inline; make inline opt-in. → yass.yass.yaml, spec/cli.query. Round-02: not exercised (probe used USES/SEE for shared conventions, not CONFORMS). |
| `conforms-inlining-semantics-misplaced` | repeated | open | Inlining rules live in cli.query not the language; guard-injection edge case silently drops guards. Move semantics into Reference spec. → yass.yass.yaml, spec/cli.query |
| `conforms-bare-slot-meaning` | repeated | open | Grammar marks ::SLOT optional but query rejects bare CONFORMS; define meaning + align. → yass.yass.yaml, schema, spec/cli.query |
| `self-validation-ref-bug` | universal | open | spec/cli.shared uses `../cli@...` resolving to nonexistent root cli.yass.yaml; `yass validate spec/` can't exit 0. Fix to `./cli@...`. → spec/cli.shared |
| `dispatch-subcommand-override` | repeated | open | Dispatch-level vs subcommand-level rule precedence (bare `-`) unexpressed. Need override mechanism or pin rule. → yass.yass.yaml, spec/cli.* |
| `reftarget-resolution-scattered` | repeated | open | Ref-target resolution rules split across 3-4 specs; consolidate in one owner; use ::SLOT-granular refs. → yass.yass.yaml, spec/cli.* |
| `duplicate-normativity-wording` | repeated | open | "same Normativity keyword more than once" misreads as key-repeat; means >1 keyword. Reword. → yass.yass.yaml |
| `unreachable-codes` | repeated | open | Several error codes unreachable by design; mark (SHADOWED-BY/UNREACHABLE-WHEN) or remove. → spec/cli.errors, yass.yass.yaml |
| `default-error-policy` | single | resolved | Guard-less ERROR obligation now defined as the **residual** (`yass.yass.yaml` `Slot.ERROR`; `yass-reference` ERROR-slot paragraph); anti-pattern of folding a foreseeable failure into the catch-all documented (`GUIDANCE`). Round-01: 4/4 models correctly read the guard-less catch-all but **all 4 had to infer** that out-of-hours routed to it (strong signal). Pruned from `RECOMMENDATIONS.md` Part 1 §4 + Part 2 §2. Re-verified round-02 (E90 residual, 4/4). |

## guidance

| id | recurrence | status | title / lever |
|----|-----------|--------|---------------|
| `error-table-first-workflow` | universal | open | Recommend writing/reading the error table first. → GUIDANCE |
| `message-templates-byte-exact` | universal | open | State error message templates are byte-for-byte contracts, never reworded. → GUIDANCE |
| `cross-cutting-single-home` | universal | resolved | Cross-cutting rules need one owning spec + refs. **Round-02 (4/4):** `honey.shared` owned the entire wire protocol (dispatch, segmentation, exit policy, error-line format) and the three stage specs referenced it; all four implemented it consistently with no drift. Fixed: `GUIDANCE` ("Composition"). Pruned `RECOMMENDATIONS.md` Part 1 §3. |
| `closed-set-dispatch-residual` | repeated | resolved | A spec that dispatches on a closed set of input values (subcommand/mode/enum) must state the out-of-set/missing case; the residual principle generalizes beyond ERROR. **Round-02 (3/4):** models flagged the undefined missing/unknown argv[1] and **diverged** (composer usage+exit 1; gpt/opus invented E00+exit 2; gemini unflagged). Distinct from `dispatch-subcommand-override` (rule precedence, still open). Fixed: `GUIDANCE` ("Closed-set dispatch"), `yass.yass.yaml` (`Slot.INPUT`), `yass-reference` (Slots). |
| `mustnot-undertested` | universal | open (probed round-01) | MUST-NOT/negative-space obligations systematically dropped/untested; call out negative-test discipline; distinguish failure-mode prohibition vs feature deferral. → GUIDANCE. Round-01: a CLI-shaped probe made MUST-NOTs observable (no OK line for a rejected record, no error line for an accepted one) and the oracle's full stdout/stderr diff tested them — all 4 models passed. Evidence MUST-NOTs become black-box-testable once a spec is CLI-shaped; the language-level testable-vs-environmental split stays open. |
| `input-segmentation-completeness` | single (round-01) | resolved | Input segmentation must state every boundary: exact separator + character class (ASCII space vs general whitespace), empty input, blank interior unit, leading/trailing/repeated separator. Round-01: 3/4 models flagged trailing-newline/blank-line ambiguity; composer split fields on Unicode whitespace and stripped `\r` (non-conformant) — caught only after hardening the oracle with tab + CRLF batches. → GUIDANCE ("Input segmentation: specify every boundary"). Re-verified round-02: tab/CR/NBSP-as-data, single optional trailing newline, empty input = zero records all handled by 4/4. Re-verified round-03 (off-spec batches 4/4). |
| `segmentation-terminator-mechanics` | repeated (round-01, round-03) | resolved | Refinement of `input-segmentation-completeness`: a bare "MAY accept a trailing newline" leaves the *mechanics* (how many trailing separators are absorbed — characteristically exactly one) and the *degenerate* input (a lone separator with no content; input that is only separators) under-determined. **Round-03 (2/4):** gpt and opus each constructed the precise rule themselves (strip exactly one trailing `0x0A`; remaining blanks are zero-field records; lone `"\n"` = one zero-field record), converging on the oracle but with no spec text saying it. Fixed: `GUIDANCE` ("Input segmentation") checklist now requires the optional-terminator count and the all-separators/lone-separator case. |
| `when-complement-discipline` | repeated | open | Every WHEN guard implies a load-bearing negative branch; treat "or" as exhaustiveness checklist. → GUIDANCE |
| `layer-shadowing-tests` | repeated | open | Obligations shadowed by lower layer need reachable test inputs; document unreachable-by-design codes. → GUIDANCE |
| `outcome-not-mechanism` | repeated | open | Distinguish observable outcomes from prescribed mechanism/ordering. → GUIDANCE |
| `feasibility-against-real-system` | repeated | open | Require proving specs against the real target system (YAML-null, yes/no footguns, signal exit, symlink APIs). → GUIDANCE |

## reference

| id | recurrence | status | title / lever |
|----|-----------|--------|---------------|
| `grammar-and-self-spec-as-reference` | repeated | open | Clarify yass.yass.yaml is a language reference (not impl spec); acknowledge registry-vs-module spec files. → yass-reference, GUIDANCE |
| `redundant-yaml12-restatement` | single | open | "yes/no/on/off as strings" restates YAML 1.2; drop or annotate as emphasis. → yass.yass.yaml (Document), yass-reference |

## tooling-request (candidate TOOLING.md — only if irreducible)

| id | recurrence | status | title |
|----|-----------|--------|-------|
| `lint-anti-slop` | repeated | open | `yass lint` for schema-valid-but-hollow specs + colon-space/Norway footguns + auto-quote fmt. **Now also owns the refuted structured-obligation cluster's ergonomic/machine-checkability residue: `yass extract-errors` registry projection, byte-exact message-template lint, error-code reference validation, priority/cardinality idiom lint. Detailed in TOOLING.md.** |
| `test-gen-and-coverage` | universal | open | spec→test generation + coverage check; promote TEST-TAXONOMY.md. Gated on per-obligation identity language work. |
| `self-validate-ci-gate` | repeated | open | Wire `yass validate spec/` into CI (zero new tooling once self-validation-ref-bug fixed). |

## language-design-decision (open choices; pull candidates from here)

| id | recurrence | status | title |
|----|-----------|--------|-------|
| `man-page-vocabulary` | repeated | open | Realign vocab to man-page sections: CONFORMS→CONFORMS-TO, SEE→SEE-ALSO, ERROR→ERRORS, RETURN→RETURN-VALUE?, add EXIT-STATUS/DIAGNOSTICS/EXAMPLES/OPTIONS. |
| `example-slot` | universal | open | Add EXAMPLE slot (worked input→output pairs) for emitter/serializer specs. |
| `slot-model-for-non-functional-specs` | repeated | open | Five function-shaped slots fit procedural/serializer/config specs awkwardly; PROCEDURE/ALGORITHM slot or split SIDE-EFFECT. |
| `intent-field-and-max-lengths` | repeated | open | Bounded per-spec `intent:` field + enforced max lengths on prose fields (tensions no-free-prose non-goal). |
| `multi-target-refs` | repeated | open | List-valued relation keys (CONFORMS/USES/SEE accept multiple targets). |
| `root-and-rules-files` | single | open | Explicit `root.yass.yaml` project-root marker + `rules.yass.yaml` for tooling meta-rules. |

## Round-01 evidence (2026-06-24) — berth probe, panel gpt / gemini / opus / composer

Probe `test-specs/round-01-berth` stressed the structured-obligation cluster in a
fresh domain (port-berth assignment CLI): a prose error registry (E10–E90), the
8-step priority chain as one prose sentence, USES-only cross-spec sequencing,
positional fields named only in prose (`dataflow-invisible`), a guard-less catch-all
(`default-error-policy`), "at most one error per record" (`error-cardinality-implicit`),
and MUST-NOT obligations (`mustnot-undertested`).

Honest grades (oracle hardened mid-round with `whitespace_tab` + `crlf_record`
batches): **gpt 12/12, gemini 12/12, opus 12/12, composer 10/12.**

- **Headline meta-finding — the cluster caused zero functional misses** across 4
  strong models at this spec scale. Prose error table, prose priority chain, USES
  sequencing, and positional-fields-in-prose were all read correctly. Cost showed up
  as *inference*, not *correctness*: every model wrote paragraphs reasoning about the
  catch-all and record boundaries. Implication: the cluster is "inference-expensive
  but survivable" at small scale — future rounds must scale spec size/interconnection
  to find the breaking point, or target the constructs that demonstrably forced the
  most inference. Cluster findings stay `open`, now annotated `probed round-01`.
- **`default-error-policy` — strong (4/4), RESOLVED.** All four flagged that the most
  load-bearing domain rule (window outside operating hours → reject) had no dedicated
  obligation and was reachable only by inferring it routed to the guard-less E90
  catch-all. Fixed in source of truth (see row). Re-verified round-02: the guard-less
  `E90` residual in tally/grade/pack was read correctly by 4/4.
- **`input-segmentation-completeness` — corroborated (3/4), RESOLVED (new).** 3/4
  models flagged trailing-newline / blank-line ambiguity; composer additionally split
  fields on Unicode whitespace (`strings.Fields`) and stripped trailing `\r` — both
  non-conformant. New GUIDANCE section added.
- **composer 10/12 is a MODEL-ERROR, not a spec-defect.** The obligations ("runs of
  one or more ASCII space characters", "separated by a single newline") are
  unambiguous; composer's own NOTES admit the `strings.Fields` shortcut was assumed
  "equivalent for ASCII-only inputs implied by the spec". The actionable item was the
  oracle gap (now fixed), not a yass-language change.
- **Oracle coverage gap (methodology, FIXED).** The original 10 batches used only
  ASCII-space-separated, LF-terminated input, so composer scored 10/10 despite the
  non-conformance. Added tab + CRLF batches and SELFTEST entries; composer then
  correctly graded 10/12. Standing oracle discipline for future rounds: exercise every
  segmentation obligation with off-spec separators.

## Round-02 evidence (2026-06-24) — apiary probe, panel gpt / gemini / opus / composer

Probe `test-specs/round-02-apiary` stressed the composition cluster in a fresh domain
(honey-apiary harvest pipeline: one CLI, three subcommands `tally`→`grade`→`pack`
chained by shell pipes, argv[1] selects the stage). A single `honey.shared` spec owned
the entire wire protocol (subcommand dispatch, line/field segmentation, exit policy,
byte-exact error-line format); the three stage specs referenced it via `USES`, and
each downstream stage's INPUT carried a `USES …@<prev>::RETURN` pointer to the producer.
Targets: `dataflow-invisible` + `cross-spec-sequencing` (primary), `cross-cutting-single-home`,
with `default-error-policy` + `input-segmentation-completeness` regression coverage.

Oracle: 38 batches (33 per-stage + 5 full-pipeline); SELFTEST OK; reference impl 38/38
before the panel. Grades: **composer 38/38 (Go, ~70 s), gpt 38/38 (Python, ~125 s),
gemini 38/38 (Python, ~159 s), opus 38/38 (Python, ~242 s) = 152/152, zero functional
misses.** Every targeted obligation — inter-stage dataflow, header-gate sequencing,
guard-less `E90` residual, off-spec segmentation, byte-exact tokens / band cutoffs /
capacities / exit codes — was implemented correctly cold by all four.

- **`dataflow-invisible` — strong (4/4), RESOLVED.** All four flagged that `honey.grade`
  / `honey.pack` redefine "well-formed" for their inputs without restating which upstream
  validation the previous stage already performed; the `USES …::RETURN` pointer was
  present and resolved but carried only structural meaning, so the data contract across
  the boundary was invisible. All four guessed the trust boundary identically (trust
  upstream-validated fields, re-check only what the stage adds) → 38/38, but the spec was
  silent. Fixed in source of truth (see row): a slot-targeted `USES` now carries a
  documented dataflow reading, and `Slot.INPUT` requires naming the producing slot and
  stating the trust boundary.
- **`closed-set-dispatch-residual` — corroborated (3/4), RESOLVED (new).** The `honey`
  shared spec dispatches on the closed set `{tally, grade, pack}` but defined no behavior
  for a missing or unknown argv[1]. Three of four flagged it and **diverged**: composer
  printed usage to stderr and exited 1; gpt and opus emitted an invented `E00`-class error
  and exited 2; gemini resolved it silently. Same failure mode as `default-error-policy`
  but in the INPUT/dispatch slot rather than ERROR. Fixed: the residual principle is
  generalized to any closed-set branch (see row). Distinct from `dispatch-subcommand-override`
  (rule precedence, still open).
- **`cross-cutting-single-home` — strong (4/4), RESOLVED.** `honey.shared` owned the whole
  wire protocol and the stage specs referenced it; all four implemented the cross-cutting
  rules consistently with no drift. The pattern worked but the SOT taught it nowhere; fixed
  by the new GUIDANCE *Composition* section.
- **`cross-spec-sequencing` — partial positive (4/4), stays open re-scoped.** The header-gate
  idiom (`E40`/`E41` + write-nothing + reject-all + exit 2 on a wrong stage header, paired
  with the `USES …::RETURN` pointer) expressed data-pipeline sequencing correctly for all
  four. So the dataflow case is expressible today; non-dataflow execution preconditions (a
  REQUIRES/AFTER notion) and the now-reinforced USES overload remain open.
- **Regressions held.** `default-error-policy` (guard-less `E90` residual) and
  `input-segmentation-completeness` (tab/CR/NBSP-as-data, single optional trailing newline,
  empty input = zero records, runs-of-spaces field splitting) were both read correctly by
  4/4. Re-verified.
- **Not exercised.** `conforms-overloaded` — the probe used `USES`/`SEE` for shared
  conventions (they are "draws on", not "must match"), so `CONFORMS` was not tested. Stays
  open, untested.

## Round-03 evidence (2026-06-24) — vault probe, panel gpt / gemini / opus / composer

Probe `test-specs/round-03-vault` was built to break the structured-obligation cluster at a
scale Rounds 1–2 never reached (bank-vault time-lock door annual certification: one binary,
two subcommands `certify`/`report`). A single `vault.shared` owned an **18-code defect
registry** (V01–V18, each row = code + class + condition + byte-exact message packed into one
scalar prose obligation), a **deliberately non-monotonic precedence order** (not severity-
sorted, not code-sorted, so the cheapest shortcut yields a wrong verdict), a class→exit map,
and the wire/segmentation rules; `vault.certify` emitted one verdict per door and
`vault.report` re-read the codes to tally by class. Targets: the full structured-obligation
cluster (primary), `dataflow-invisible` + `cross-cutting-single-home` +
`closed-set-dispatch-residual` + `input-segmentation-completeness` regression coverage.

Oracle: 35 batches; `--self-check` → SELFTEST OK; reference impl 35/35 before the panel.
Grades: **gpt 35/35 (Python, 152 s), gemini 35/35 (Python, 199 s), opus 35/35 (Python,
200 s), composer 35/35 (Python, 63 s) = 140/140, zero functional misses.** All four chose
Python. Every targeted obligation — the 18-row registry, the non-monotonic precedence, one-
verdict cardinality, ordered validation, the `certify | report` trust boundary, and off-spec
segmentation — was implemented correctly cold.

- **Headline meta-finding — the structured-obligation cluster is REFUTED a second time.** The
  18-row registry was reproduced byte-exact by 4/4 (no paraphrase, no miscopied threshold);
  the adversarial 18-entry precedence order was read exactly by 4/4 (every precedence batch,
  the deciding measurement, passed); one-verdict-per-door and one-error-line-per-record were
  honored by 4/4; codes read in two places survived intact. Not even a NOTES-flagged
  ambiguity about the registry or precedence. `mapping-valued-obligations`,
  `priority-chains-prose`, `error-table-structured`, `error-cardinality-implicit`, and
  `error-code-refs` are closed `wontfix (refuted ×2)`; the residual ergonomic/validation
  motivation is routed to `lint-anti-slop` (tooling), not a language change.
- **`trust-boundary-violation-residual` — corroborated (3/4 STRONG), RESOLVED (new).**
  `vault.report` carried the round-02 `dataflow-invisible` fix (named the producer, stated
  the trust boundary) — which held, no model over-validated — but said nothing about an
  out-of-contract line (blank, or unknown code). gemini, opus, and composer each flagged it
  and **guessed** the `<total>`/class contribution: convergent on outcome (count toward
  `<total>`, classify as nothing), divergent on reasoning. Extends the residual principle
  (ERROR catch-all, closed-set dispatch) to the trust boundary. Fixed: `yass.yass.yaml`
  (`Slot.INPUT` violation-residual obligation), `yass-reference` (References / slot-targeted
  USES), `GUIDANCE` ("Composition"). Distinct from `dataflow-invisible`: that states *what is
  trusted*, this states *what happens when the trust is violated*; `dataflow-invisible` stays
  resolved.
- **`segmentation-terminator-mechanics` — corroborated (2/4), RESOLVED (new).** `vault.lines`
  stated the optional trailing terminator as a bare `MAY accept a single trailing newline`,
  leaving the mechanics (how many trailing `0x0A` are absorbed) and the degenerate input
  (a lone `"\n"`; input that is only separators) unstated. gpt and opus each constructed the
  precise rule themselves (strip exactly one trailing newline, remaining blanks are zero-field
  records, lone `"\n"` = one zero-field record), converging on the oracle with no spec text
  saying it. Refines the resolved `input-segmentation-completeness`. Fixed: `GUIDANCE`
  ("Input segmentation") checklist now requires the terminator count and the
  all-separators/lone-separator case.
- **Regressions held (4/4).** `dataflow-invisible` (the `USES …::RETURN` pointer + stated
  trust boundary; no model re-validated trusted fields), `closed-set-dispatch-residual`
  (missing/unknown `argv[1]` → `usage: vault {certify|report}` to stderr, exit 2),
  `input-segmentation-completeness` (tab/CR/NBSP-as-data, runs-of-spaces field splitting,
  empty input = zero records), and `cross-cutting-single-home` (one `vault.shared` owned the
  protocol; both subcommand specs referenced it, no drift) all re-verified.
- **Non-issues (model-reasoning, not spec-defects).** "read all stdin before output"
  ordering (gemini, 1/4) — unambiguous obligation, single-model restatement. Unbounded
  integers (opus, 1/4) — no functional impact, uncorroborated. Encoding (composer decoded
  Latin-1, others bytes) — all byte-exact-correct; the tab/CR/NBSP-as-data MUST-NOT forced
  byte-awareness.
- **Convergence.** Round 3 produced two new actionable spec-defects, so the no-new-findings
  counter stays 0/2. But the primary probe target (structured-obligation cluster) is now
  exhausted/refuted, so Rounds 4–5 pivot to the next-highest-leverage open findings:
  composition/reference (`cross-spec-sequencing` REQUIRES/AFTER) and the CONFORMS cluster
  (`conforms-overloaded`, `conforms-inlining-semantics-misplaced`, `conforms-bare-slot-meaning`,
  `self-validation-ref-bug`).
