# Round 2 — Plan

## Objective

Probe the two **universal, not-yet-tested** findings on the ledger —
`dataflow-invisible` and `cross-spec-sequencing` — at a scale Round 1 never
reached: a **multi-file, multi-stage pipeline** whose data is serialized across
*process* boundaries (one subcommand's stdout is the next subcommand's stdin).

Round 1's berth probe was a single binary with a single spec; its "parse →
validate → emit" stages lived inside one process and the data never crossed a
spec-file boundary observably. So `dataflow-invisible` / `cross-spec-sequencing`
were listed as targets but never actually stressed (FINDINGS: still `open`, no
`probed round-01` tag). This round makes the inter-spec wire format **black-box
observable** by giving each stage its own subcommand and its own spec file, and
chaining them with shell pipes.

This design also satisfies the Round-1 meta-finding directive to **scale spec
size / interconnection** (4 spec files, cross-file `USES`/`CONFORMS`, a shared
owner spec) to find the breaking point the small berth probe did not.

## Findings under test this round

| id | recurrence | predicted failure mode in cold impls |
|----|-----------|--------------------------------------|
| `dataflow-invisible` | universal | The exact data a stage emits (delimiter, field order, header line) is describable today only as prose split across the producer's RETURN and the consumer's INPUT. Models will reconstruct **self-consistent but mutually incompatible** wire formats → each model's own `A \| B \| C` passes end-to-end, but per-stage output diverges from the canonical and from other models. |
| `cross-spec-sequencing` | universal | `USES` says "B uses A" but not "B's input MUST be A's output / B MUST reject input not produced by A". Models will omit the precondition gate (the stage marker / header check) → a stage accepts arbitrary input it should reject. |
| `conforms-overloaded` | universal | The shared line-grammar spec is referenced cross-file with `CONFORMS`/`USES`; watch whether models inline vs. point, and whether the cross-file reference is read at all. |
| `cross-cutting-single-home` | universal | The wire/error conventions live in one shared spec referenced by the three stages; watch whether models honor the single home or re-derive per stage. |

## Regression re-verification (Round-1 fixes)

The probe is authored so the Round-1 source-of-truth fixes are exercised, not
just asserted:

- **`default-error-policy` (guard-less residual).** Each stage spec carries at
  least one **guarded** ERROR obligation (a named foreseeable failure with its
  own observable outcome) plus exactly one **guard-less** ERROR obligation read
  as the residual. If the new `Slot.ERROR` semantics hold, models route
  unenumerated failures to the residual without having to rediscover a hidden
  domain rule (the Round-1 anti-pattern is absent by construction).
- **`input-segmentation-completeness`.** The shared spec specifies every
  boundary per the new GUIDANCE section (exact separator + character class —
  ASCII space `0x20`; record = single `\n`; empty input; blank interior line;
  leading/trailing/repeated separator). The oracle carries off-spec batches
  (tab, CRLF, NBSP) per the standing oracle discipline. If the fix holds, no
  model should flag segmentation ambiguity and a `strings.Fields`-style shortcut
  is caught.

## Spec design (current-construct encoding, multi-file)

Fresh, non-famous domain (no training prior): an **apiary honey-harvest
pipeline**. Three subcommands of one binary, each reading stdin and writing
stdout, chained by pipes. Integer grams throughout (deterministic arithmetic).

- `spec/honey.shared.yass.yaml` — Preamble + shared specs that **own the
  cross-cutting concerns**: the field/record segmentation rules, the `ErrorLine`
  wire format (stderr), the exit-code policy, and the inter-stage **record-line
  grammar** that every stage's stdout conforms to. The three stage specs
  reference it (`CONFORMS`/`USES`).
- `spec/honey.tally.yass.yaml` — `tally`: raw scan lines → normalized harvest
  records. Emits a stage header line then one normalized record per input line.
  Rejects malformed scans (guarded) + residual.
- `spec/honey.grade.yass.yaml` — `grade`: consumes `tally` output; computes
  `net = gross − tare`, assigns a grade band by net-weight thresholds, rejects
  records that violate a constraint (e.g. `tare ≥ gross`, guarded) + residual.
  **MUST reject** input whose first line is not a valid `tally` header
  (sequencing gate).
- `spec/honey.pack.yass.yaml` — `pack`: consumes `grade` output; accumulates net
  weight per grade band, enforces a per-band crate capacity (overflow → guarded
  error) + residual, emits the final per-band pack manifest. **MUST reject**
  input lacking the `grade` stage marker (sequencing gate).

The inter-stage wire format is pinned in the spec only as completely as current
yass allows: the producing stage's RETURN describes what it emits, the consuming
stage's INPUT describes what it accepts, and both reference the shared
record-line grammar. There is no single construct that *names the data crossing
the boundary* — that absence is exactly `dataflow-invisible`.

## Oracle design (private, black-box, never copied in)

`test-specs/round-02-apiary/oracle/grade.py` — embedded authoritative
`simulate()` for each stage + a SELFTEST self-consistency guard (per Round-1
discipline). Two grading axes:

1. **Per-stage** — feed each subcommand the oracle's *canonical* input for that
   stage on stdin; compare stdout/stderr/exit byte-for-byte. Isolates whether
   the model's wire format matches the canonical intermediate.
2. **End-to-end** — run `prog tally | prog grade | prog pack`; compare final
   stdout/stderr/exit. A model whose stages agree internally but diverge from
   canonical passes end-to-end yet fails per-stage; that gap **is** the
   `dataflow-invisible` measurement.

Batches cover: each guarded error + residual per stage; the two sequencing gates
(grade fed non-tally input; pack fed non-grade input); off-spec segmentation
(tab, CRLF, NBSP, leading/trailing/repeated space, empty input, blank interior
line); grade-band boundary ties; capacity-overflow ties; a clean all-valid run.

## Procedure

1. Author `test-specs/round-02-apiary/spec/*.yass.yaml` (4 files) + private
   oracle. Validate: specs parse; oracle SELFTEST OK. Commit before any agent.
2. Reuse `experiment/round-01/prompt.md` verbatim as `round-02/prompt.md` (the
   cold task is identical: read specs, implement the CLI, write HOWTORUN.txt +
   NOTES.md; do NOT teach yass). The prompt must NOT mention pipelines or stages
   beyond what the spec says.
3. Run the 4-model panel (`gpt`, `gemini`, `opus`, `composer`) cold,
   concurrently, background, unique `/tmp` workspaces; copy in only `spec/*`.
4. Grade both axes + diagnose every miss spec-defect vs. model-error
   (≥2-model corroboration = strong signal). Write `results.md`.
5. Apply confirmed fixes to source of truth; update FINDINGS; prune resolved
   `context/*`; route irreducible tooling to TOOLING.md. Commit `round-02`;
   update STATUS + LOG; update convergence counter.

## Success criteria for the round

- Multi-file pipeline harness validated (per-stage + end-to-end grading both run).
- `dataflow-invisible` / `cross-spec-sequencing` confirmed (≥2-model) or refuted
  with evidence (per-stage-vs-end-to-end divergence is the deciding measurement).
- Round-1 fixes regression-checked (segmentation + residual) and recorded.
- Confirmed spec-defects fixed in source of truth and pruned from context/*.
