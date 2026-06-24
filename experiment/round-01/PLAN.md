# Round 1 — Plan

## Objective

Probe the highest-recurrence spec-definition findings by authoring one CLI-shaped
synthetic spec that deliberately encodes each pain point using **current yass
constructs** (prose-packed error tables, prose priority chains, USES-conflated
sequencing, unnamed dataflow). If cold models fail in the predicted way across
≥2 families, the finding is confirmed as a spec-defect and is actionable.

## Findings under test this round

Lead cluster (structured-obligation), all `universal` unless noted:

| id | predicted failure mode in cold impls |
|----|--------------------------------------|
| `error-table-structured` | error registry packed into prose MUSTs → codes/messages/exit-codes dropped or paraphrased |
| `mapping-valued-obligations` (repeated, ROOT) | per-error metadata only expressible as one prose string → lost structure |
| `priority-chains-prose` | "emit first matching error, then stop" as prose → wrong precedence / multiple errors emitted |
| `dataflow-invisible` | data crossing parse→validate→emit not named in INPUT/RETURN → fields invented/misnamed |
| `cross-spec-sequencing` | USES does not say "runs after" → stages run in wrong order or merged |
| `default-error-policy` (single) | no catch-all for unenumerated failures → divergent handling of malformed input |
| `mustnot-undertested` (guidance) | MUST-NOT (no stdout on error) silently dropped |

## Spec design (current-construct encoding)

CLI-shaped, fresh fictional domain (no training-data prior): a **berth-assignment
validator** for a fictional spaceport. The binary reads request records on stdin
(one per line), validates each, and writes a result line to stdout or an error to
stderr; process exit code reflects the worst per-line outcome.

The spec is authored in **idiomatic current yass** so the pain points are encoded
the way authors are forced to today:

- error registry as a single INVARIANT/ERROR block of prose MUSTs, each string
  packing `code + meaning + message template + exit code`;
- validation precedence as a prose sentence ("emit the first failing check in the
  order listed, then stop");
- stage sequencing expressed only via USES (parse USES nothing about *order*);
- record fields that cross parse→validate→emit described in prose, not named in
  INPUT/RETURN;
- a guard-less catch-all error left implicit;
- a MUST-NOT ("MUST NOT write a result line for a record that failed validation").

Spec files: `test-specs/round-01-berth/spec/*.yass.yaml`.

## Oracle design (private, black-box)

`test-specs/round-01-berth/oracle/` — test vectors of `stdin → (stdout, stderr,
exit)` covering: each error code, precedence ties (record failing 2 checks must
emit the higher-priority one only), the MUST-NOT (no stdout on failed record),
malformed-line default handling, and a clean all-valid batch. Never copied into
any agent workspace.

## Procedure

1. **Harness smoke test first (Round-1 directive).** Author a trivial spec
   (`test-specs/round-01-smoke/`) + oracle + cold prompt; run ONE model
   (`gpt`) cold in a unique `/tmp` dir; confirm: agent emits impl +
   `HOWTORUN.txt` + `NOTES.md`, build per HOWTORUN works, oracle grades. Only
   then expand.
2. Author the real `round-01-berth` spec + oracle. Commit.
3. Write `experiment/round-01/prompt.md` (cold: read spec(s), implement the CLI,
   write `HOWTORUN.txt` + `NOTES.md`; do NOT teach yass).
4. Run the 4-model panel (`gpt`, `gemini`, `opus`, `composer`) cold,
   concurrently, background, unique `/tmp` dirs.
5. Grade + diagnose; write `results.md`.
6. Apply confirmed fixes to source of truth; update FINDINGS; prune context/*.

## Success criteria for the round

- Harness validated end-to-end on one model.
- ≥1 finding confirmed (≥2-model corroboration) or refuted with evidence.
- Confirmed spec-defects fixed in source of truth and pruned from context/*.
