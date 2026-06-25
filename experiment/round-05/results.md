# Round 05 results — scale probe (truck weighbridge CLI)

**Date:** 2026-06-24 · **Panel:** gpt-5.5-extra-high · gemini-3.1-pro ·
claude-opus-4-8-thinking-high · composer-2.5 · **This is the final scheduled round.**

## Probe design

A fresh-domain CLI (truck weighbridge) chosen to be the next-highest-leverage test of the
remaining open **composition/reference** findings. One binary, two subcommands `run`/`audit`,
five specs:

- `scale.shared` — owns dispatch (`scale {run|audit}`, else `usage:`/exit 2), line/field
  segmentation, the 3-field operation-record format with ordered errors (E10/E25/E15/E20),
  the overload defect registry (W01 CRITICAL > W02 MAJOR > W03 MINOR > W04 MINOR-when-gross>50000,
  else CLEAR; net<0 "never a verdict but the E50 error"), and the error set.
- `scale.tare` / `scale.weigh` — the run operations (TARE records a per-id tare weight; WEIGH
  derives net = gross − tare and classifies it).
- `scale.run` — the producer subcommand (validate/dispatch each record, max-severity exit).
- `scale.audit` — the consumer subcommand (reads the ticket lines `run` writes + hand-fed
  REWEIGH lines, tallies by overload class, writes a 5-line summary, exit 0).

Three probe targets, each chosen to test whether an *open construct request* is genuinely
needed or is already expressible with today's language:

1. **`cross-spec-sequencing`** — `scale.weigh` requires a prior TARE for the **same vehicle id**
   earlier in the input: a non-dataflow, per-identifier stateful precondition, expressed with a
   normative prose obligation + `USES ./scale.tare@scale.tare` and deliberately **no** REQUIRES/AFTER
   key. (Round-02 already showed the *dataflow* sequencing form is expressible via the header-gate
   idiom; this round tests the *non-dataflow stateful-precondition* form.)
2. **`dispatch-subcommand-override`** — `scale.audit` overrides the shared blank-line segmentation
   rule with a scoped prose obligation, while also carrying a whole-spec
   `CONFORMS ./scale.run@scale.run`. This simultaneously re-tests the round-04 whole-spec-CONFORMS
   (non-transcluded conformance ref) and guard-conjunction resolutions under a fresh probe.
3. **`reftarget-resolution-scattered`** — a documentation-consolidation finding, verified by
   inspection of the source-of-truth specs rather than by the panel.

## Method

Cold + isolated: each model ran in its own throwaway `/tmp/yass-r05/<model>` workspace containing
**only** the five `spec/*.yass.yaml` files. The private oracle was never copied into any workspace.
Black-box grading: stdin → stdout/stderr/exit-code, byte-exact (surrogateescape).

Oracle: 49 batches; `grade.py --self-check` → `SELFTEST OK`; reference impl 49/49 before the panel.
Batches include the four-corner audit guard-conjunction matrix, the `disp_bare_dash` dispatch case
(`-` → `usage:` / exit 2), the residual/exhaustiveness cases, and the full `run | audit` pipeline.

## Grades — 196/196, zero functional misses

| model | language | score |
|-------|----------|-------|
| gpt | Python | 49/49 |
| gemini | Python | 49/49 |
| opus | Python | 49/49 |
| composer | Go | 49/49 |

## Analysis — every probe target refuted; no new spec defect

### `cross-spec-sequencing` — REFUTED (4/4 clean) → `wontfix`

The WEIGH-after-TARE per-identifier precondition (satisfied only by a prior TARE for the *identical*
id, never a different id; an unmet precondition yields the `E40` rejection rather than a verdict) was
implemented correctly cold by all four models, with **zero NOTES confusion**, expressed entirely in
normative prose + a `USES` pointer. No model needed (or wished for) a sequencing relation.

Combined with round-02's dataflow header-gate case (also 4/4), **both** forms of cross-spec
sequencing — dataflow ("consumes the output of") and non-dataflow ("a stateful precondition must
already hold") — are expressible today without a dedicated REQUIRES/AFTER construct. The construct is
unwarranted as a *correctness* need. The residual observation that `USES` reads as both "calls" and
"runs after" is an ergonomic/lint concern, not a one-shot-failure defect, and does not justify a
language change under the experiment charter.

### `dispatch-subcommand-override` — REFUTED (4/4 clean) → `wontfix`

`scale.audit`'s INPUT obligation overrode the shared segmentation rule (shared: a blank line is a
zero-field record → `E10` under `run`; audit: a blank line is a *skipped section break*, counted
toward nothing) via a scoped prose obligation that explicitly names the shared rule it overrides and
scopes the override to audit alone. All four read the scoped override correctly; the bare-`-`
dispatch precedence was exercised by the `disp_bare_dash` batch (→ `usage:` / exit 2, 4/4).

A scoped prose obligation that names what it overrides is sufficient to express subcommand-level
override of a shared/CONFORMS'd rule. No OVERRIDES construct or pin-rule mechanism is warranted.

**Round-04 resolutions re-verified under load (4/4).** The same probe carried a whole-spec
`CONFORMS ./scale.run@scale.run` (read as a non-transcluded conformance reference, not inlined) and
guard-conjoined inlined obligations (`CONFORMS …@scale.defects::INVARIANT` under an outer WHEN). Both
were handled correctly by all four — `conforms-bare-slot-meaning`,
`conforms-inlining-semantics-misplaced`, and the `residual-reachability` exhaustiveness discipline
(both `scale.weigh` and `scale.audit` assert an *empty* residual instead of stating a dead catch-all)
all held. The round-04 fixes are durable.

### `reftarget-resolution-scattered` — RESOLVED (documentation consolidation)

Confirmed `yass.yass.yaml` `RefTarget` is the sole canonical owner of both the resolution rule
(RETURN slot: `./`/`../` → relative to referencing file, no leading dot → project root, append
`.yass.yaml`, case-sensitive byte compare) and the charset grammar (ERROR slot: path
`[A-Za-z0-9._/-]`, spec-name `[A-Za-z0-9._-]`, slot `[A-Z-]`, `path@SpecName::SLOT` shape).

Two genuine restatements consolidated to cite the owner without changing behavior:

- `spec/cli.validate.yass.yaml` ERROR — had re-encoded the charset as a literal regex
  `^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$` (the highest drift risk). Now defers to the
  RefTarget grammar by name + a reference-only `SEE: yass@RefTarget::ERROR`.
- `spec/cli.query.yass.yaml` INVARIANT — had restated the `./`/`../`-vs-project-root resolution.
  Now defers to the RefTarget resolution rule + a reference-only `SEE: yass@RefTarget::RETURN`.

Repo ref-check CLEAN (150 refs, 0 dangling, 0 schema errors). Deliberately **not** consolidated
(avoid over-citation that adds noise without reducing real drift risk): the spec-*name* grammar
`^[A-Za-z0-9_-]+(\.[…])*$` (a different owner), and the `.yass.yaml`-suffix / `FindProjectRoot` rules
(separate concerns — file discovery and root detection, not ref-target resolution).

### Negative-net audit REWEIGH (4/4 NOTES) — probe-authoring artifact, NOT a yass finding

All four models flagged the same tension in `scale.audit`: a flagged-Y re-weigh line with
`gross < tare` yields a negative re-derived net, and the shared registry says "net below zero is
never a verdict but is the `E50` error" — yet audit raises no error (it exits 0 by contract). All
four resolved it **identically to the oracle** (tally as CLEAR: a negative net governs no verdict,
and `E50` is a run-pipeline-only path that audit has no access to). There was therefore **zero
behavioral divergence**, and no oracle batch exercises the case.

Per the round-04 methodology lesson: a spot the spec *should* have pinned unambiguously but did not,
which nonetheless produced no behavioral divergence, is a **probe-authoring artifact**, not a
language defect. (A 4/4 NOTES flag is a strong signal only when it tracks latent *ambiguity* the
models resolve *differently* or hedge on — here they resolved it the same way with confidence.) The
probe spec was tightened post-run (the flag-Y obligation and the exhaustiveness INVARIANT now state
the negative-net → CLEAR disposition explicitly) for self-consistency. This does not affect any grade:
the oracle's expected output was unchanged and no batch depends on the case.

## Disposition

- `cross-spec-sequencing` → **wontfix (refuted round-05)**
- `dispatch-subcommand-override` → **wontfix (refuted round-05)**
- `reftarget-resolution-scattered` → **resolved** (cite RefTarget at `cli.validate` ERROR and
  `cli.query` INVARIANT; ref-check CLEAN at 150 refs)
- negative-net audit REWEIGH → **probe-authoring artifact** (not a finding)

**Round 5 produced no new actionable spec defect.** Both construct-request probes were refuted; the
one resolved finding was a pre-existing open documentation item; the negative-net ambiguity was an
authoring artifact. Convergence counter advances **0/2 → 1/2**. Round 5 is the final scheduled round,
so the experiment **HALTS** here.
