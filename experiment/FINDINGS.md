# FINDINGS ledger

Live working set for the experiment. Status: `open` | `probing` (under test this
round) | `resolved` (fixed in source of truth + pruned from context/*) |
`tooling` (moved to TOOLING.md) | `wontfix` (explicit non-goal).

Ingested from `context/{OPEN,GUIDANCE,SPEC}-FEEDBACK.md`,
`RECOMMENDATIONS.md`, `NOTES.md`, `FIXES.md`, `IDEAS.md`, `FUTURE.md`,
`MAN-ALIGNMENT.md` (26 distinct findings after dedup). Recurrence: `universal`
(≈all 7 prior cross-language impls) > `repeated` (several sources) > `single`.

The **structured-obligation cluster** (`mapping-valued-obligations` as root, with
`error-table-structured`, `error-code-refs`, `priority-chains-prose`,
`error-cardinality-implicit`) should be triaged together — it is the
highest-leverage language change.

## spec-definition

| id | recurrence | status | title / lever |
|----|-----------|--------|---------------|
| `mapping-valued-obligations` | repeated | open (probed round-01) | Obligation values are scalar-only; allow mappings to carry code/priority/metadata. ROOT enabler. → yass.yass.yaml, schema. Round-01: prose-packed error registry caused 0 functional misses across 4 models at small scale (inference-expensive, not incorrect). |
| `priority-chains-prose` | universal | open (probed round-01) | Ordered "emit first matching error, stop" chains live as prose in INVARIANT; need PRIORITY/ORDERED construct or ordered-list semantics. → yass.yass.yaml, schema, GUIDANCE, reference. Round-01: an 8-step prose priority chain was read correctly by all 4 models (0 misses); cost was paragraphs of reasoning per model. |
| `cross-spec-sequencing` | universal | open | No way to express execution sequencing/preconditions between specs; USES conflates call/depend/after. Need REQUIRES/AFTER. → yass.yass.yaml, schema, reference |
| `dataflow-invisible` | universal | open | Specs describe components (trees) not dataflow between them (graphs); INPUT/RETURN don't name data crossing boundaries. → yass.yass.yaml, schema, GUIDANCE, reference |
| `error-table-structured` | universal | open (probed round-01) | ~85-row error registry forced into prose MUSTs (code+message+exit packed per string). Best AND worst artifact. Need structured error-table. → yass.yass.yaml, schema, spec/cli.errors. Round-01: a compact prose error registry (E10–E90, message+exit each) produced byte-exact output from all 4 models. |
| `error-cardinality-implicit` | repeated | open (probed round-01) | "at most one per file" vs "one per rule" vs "one per (X,Y) pair" is implicit prose. Need ONCE-PER/EACH/DEDUP-BY. → yass.yass.yaml, schema. Round-01: "at most one error line per record" stated in prose was honored by all 4 models (interleave + priority-tie batches pass); no functional defect at this scale. |
| `error-code-refs` | single | open | Error codes cited as bare string literals in prose, unvalidatable. Need structured CODE key. → yass.yass.yaml, schema |
| `conforms-overloaded` | universal | open | CONFORMS does contract+inlining+provenance; most bug-prone feature. Split assertion vs render-time inline; make inline opt-in. → yass.yass.yaml, spec/cli.query |
| `conforms-inlining-semantics-misplaced` | repeated | open | Inlining rules live in cli.query not the language; guard-injection edge case silently drops guards. Move semantics into Reference spec. → yass.yass.yaml, spec/cli.query |
| `conforms-bare-slot-meaning` | repeated | open | Grammar marks ::SLOT optional but query rejects bare CONFORMS; define meaning + align. → yass.yass.yaml, schema, spec/cli.query |
| `self-validation-ref-bug` | universal | open | spec/cli.shared uses `../cli@...` resolving to nonexistent root cli.yass.yaml; `yass validate spec/` can't exit 0. Fix to `./cli@...`. → spec/cli.shared |
| `dispatch-subcommand-override` | repeated | open | Dispatch-level vs subcommand-level rule precedence (bare `-`) unexpressed. Need override mechanism or pin rule. → yass.yass.yaml, spec/cli.* |
| `reftarget-resolution-scattered` | repeated | open | Ref-target resolution rules split across 3-4 specs; consolidate in one owner; use ::SLOT-granular refs. → yass.yass.yaml, spec/cli.* |
| `duplicate-normativity-wording` | repeated | open | "same Normativity keyword more than once" misreads as key-repeat; means >1 keyword. Reword. → yass.yass.yaml |
| `unreachable-codes` | repeated | open | Several error codes unreachable by design; mark (SHADOWED-BY/UNREACHABLE-WHEN) or remove. → spec/cli.errors, yass.yass.yaml |
| `default-error-policy` | single | resolved | Guard-less ERROR obligation now defined as the **residual** (`yass.yass.yaml` `Slot.ERROR`; `yass-reference` ERROR-slot paragraph); anti-pattern of folding a foreseeable failure into the catch-all documented (`GUIDANCE`). Round-01: 4/4 models correctly read the guard-less catch-all but **all 4 had to infer** that out-of-hours routed to it (strong signal). Pruned from `RECOMMENDATIONS.md` Part 1 §4 + Part 2 §2. |

## guidance

| id | recurrence | status | title / lever |
|----|-----------|--------|---------------|
| `error-table-first-workflow` | universal | open | Recommend writing/reading the error table first. → GUIDANCE |
| `message-templates-byte-exact` | universal | open | State error message templates are byte-for-byte contracts, never reworded. → GUIDANCE |
| `cross-cutting-single-home` | universal | open | Cross-cutting rules (path formatting, ErrorLine wire protocol) need one owning spec + refs. → GUIDANCE |
| `mustnot-undertested` | universal | open (probed round-01) | MUST-NOT/negative-space obligations systematically dropped/untested; call out negative-test discipline; distinguish failure-mode prohibition vs feature deferral. → GUIDANCE. Round-01: a CLI-shaped probe made MUST-NOTs observable (no OK line for a rejected record, no error line for an accepted one) and the oracle's full stdout/stderr diff tested them — all 4 models passed. Evidence MUST-NOTs become black-box-testable once a spec is CLI-shaped; the language-level testable-vs-environmental split stays open. |
| `input-segmentation-completeness` | single (round-01) | resolved | Input segmentation must state every boundary: exact separator + character class (ASCII space vs general whitespace), empty input, blank interior unit, leading/trailing/repeated separator. Round-01: 3/4 models flagged trailing-newline/blank-line ambiguity; composer split fields on Unicode whitespace and stripped `\r` (non-conformant) — caught only after hardening the oracle with tab + CRLF batches. → GUIDANCE ("Input segmentation: specify every boundary"). |
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
| `lint-anti-slop` | repeated | open | `yass lint` for schema-valid-but-hollow specs + colon-space/Norway footguns + auto-quote fmt. |
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
  catch-all. Fixed in source of truth (see row).
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
