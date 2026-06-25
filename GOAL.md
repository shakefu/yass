# GOAL — yass spec-improvement experiment

> This file is my durable north star and the reusable method. The main session
> autocompacts at 100k, so during a run **this file plus the run's `experiment/`
> working state is the only memory that survives** — after a compaction, re-read
> this file top to bottom, then the in-progress run's `experiment/STATUS.md`,
> `experiment/FINDINGS.md`, and the latest round in `experiment/LOG.md`, and
> continue. Between runs, only this file and the `experiment/FINDINGS.md` ledger
> persist; `experiment/STATUS.md`, `experiment/LOG.md`, `experiment/round-NN/`,
> and `test-specs/round-NN-*/` are authored fresh each run. Branch:
> `goal-experiment` (scratch — commit freely, force/rewrite OK). To launch or
> resume a run, paste `experiment/run-prompt.md` into a `/goal` invocation.

## Mission

**yass** is a YAML spec language whose goal is to let an LLM **one-shot a
software project by reading only the `.yass.yaml` spec files** — no prior
context, no yass manual, no optional tooling. The spec files must be
self-sufficient.

This experiment improves the yass language *by experimental verification*:
generate synthetic specs, have a panel of LLMs implement them cold, measure
where they fail, and trace each failure to a concrete defect in the yass
language / guidance / reference — then fix the source of truth and re-verify.

## End state (definition of done)

1. The **source-of-truth** files are updated to fix every confirmed defect:
   - `yass.yass.yaml` — the self-hosting language definition
   - `yass.v1.schema.json` — the validation schema
   - `context/GUIDANCE.md` — spec-authoring guidance
   - `context/yass-reference.md` — language reference
   - (and `spec/*.yass.yaml` example specs kept consistent with language changes)
2. All **findings/feedback files are removed** (by deletion, not annotation)
   once their content is resolved into the source of truth:
   `context/{OPEN-FEEDBACK,GUIDANCE-FEEDBACK,SPEC-FEEDBACK,RECOMMENDATIONS,NOTES,FIXES,IDEAS,FUTURE,MAN-ALIGNMENT}.md`
3. Recurring requests that are *genuinely tooling* (cannot be resolved by making
   the spec/guidance clearer) are gathered into a single new `TOOLING.md`.
4. `experiment/FINDINGS.md` ledger is fully resolved.

## Operating parameters (decided with the user)

- **Cold + isolated.** Agents see ONLY the synthetic `.yass.yaml` spec files, no
  yass docs. For each agent run, the spec is copied to a **unique `/tmp` dir** so
  every agent works independently and completely cold. The synthetic spec itself
  is committed to this branch first (under `test-specs/`).
- **CLI-shaped specs.** Synthetic specs describe **command-line programs** so the
  oracle can grade them **black-box** (run binary with inputs → check
  stdout/stderr/exit code) regardless of implementation language.
- **Oracle.** For every synthetic spec I author a *private* acceptance oracle
  (test vectors + expected outputs) kept in `test-specs/<round>/oracle/`, which
  is **never** copied into an agent workspace. I may evolve the oracle to sharpen
  signal.
- **Signal = correct / complete / efficient.** Each miss is diagnosed as either a
  **spec-defect** (ambiguous/underspecified/misleading/contradictory language —
  actionable) or a **model-error** (spec was clear, model blew it — not
  actionable). A miss shared by ≥2 models is a strong spec-defect signal.
- **Model panel (family-diverse), run concurrently in background per round:**
  - `gpt` → `gpt-5.5-extra-high`
  - `gemini` → `gemini-3.1-pro`
  - `opus` → `claude-opus-4-8-thinking-high`
  - `composer` → `composer-2.5`
  - (`grok` → `grok-4.3` available to add for breadth)
- **Languages:** agents may choose Go, Rust, or Python (all installed:
  go1.26, rust1.94, python3.14). Language choice is recorded, not controlled —
  the black-box oracle is language-agnostic.
- **Blast radius open.** Backward-incompatible language changes (slot renames,
  new/dropped slots, structured obligations) are fair game. Pre-release, scratch.
- **Tooling framing.** A request that *sounds* like tooling but is really the
  spec under-specifying something gets fixed in the spec/guidance, NOT by
  building a tool. Only irreducible tooling needs go to `TOOLING.md`.
- **Checkpoint: 5 full rounds, then HALT and report.** Unlimited budget, but 5
  rounds is the sanity checkpoint. Also track "K=2 consecutive rounds with no new
  actionable spec-defect findings" as the convergence signal to report.

## The loop (one round)

Each round is recorded in `experiment/LOG.md` and its artifacts in
`experiment/round-NN/` + `test-specs/round-NN-<name>/`.

1. **Plan.** Pick the highest-recurrence OPEN findings from
   `experiment/FINDINGS.md` to probe this round, plus re-verify fixes applied in
   prior rounds (regression). Write `experiment/round-NN/PLAN.md`.
2. **Author spec + oracle.** Write a small CLI-shaped synthetic spec in a *fresh,
   non-famous domain* (avoid training-data priors) that deliberately exercises
   the targeted findings and spans the constructs (all five slots, full
   normativity gradient, cross-file CONFORMS/USES/SEE where relevant). Author the
   private oracle (black-box cases). Files:
   `test-specs/round-NN-<name>/spec/*.yass.yaml` and `.../oracle/`.
   **Commit** before running agents.
3. **Run the panel.** For each model: make a unique `/tmp` workspace, copy in
   only `spec/*.yass.yaml`, run via `script/agent` with the generic cold prompt
   (`experiment/cold-prompt.md`), in the background, concurrently.
4. **Grade.** Build each impl (per its `HOWTORUN.txt`), run the oracle, score
   pass rate + obligation coverage + an efficiency/quality read. Read each
   agent's `NOTES.md` for self-reported friction.
5. **Analyze.** For every miss, decide spec-defect vs model-error. Cross-model
   patterns = strong signal. Record in `experiment/round-NN/results.md`.
6. **Fix the source of truth.** Apply confirmed spec-defect fixes to
   `yass.yass.yaml` / schema / GUIDANCE.md / yass-reference.md (+ keep `spec/`
   examples consistent). Update `experiment/FINDINGS.md`: mark resolved findings,
   add newly discovered ones. **Prune** the corresponding content out of the
   original `context/*.md` feedback files (remove, don't annotate). Route
   irreducible tooling asks to `TOOLING.md`.
7. **Commit** everything with a `round-NN` message and update
   `experiment/STATUS.md` + `experiment/LOG.md`.

## Grading rubric (per model, per round)

- **builds**: yes/no (HOWTORUN.txt works)
- **oracle**: N passed / M total black-box cases
- **coverage**: which obligations (MUST/MUST-NOT/SHOULD/MAY, per slot) are
  satisfied; MUST-NOT and WHEN-complement cases get explicit negative tests
- **efficiency/quality**: qualitative read of the produced code
- **misses**: each tagged `spec-defect:<finding-id>` or `model-error`

## Resume protocol (after compaction)

1. Read this `GOAL.md`.
2. Read `experiment/STATUS.md` (current round + phase + next action).
3. Read `experiment/FINDINGS.md` (open vs resolved).
4. Read the latest `## Round NN` section of `experiment/LOG.md`.
5. Continue the loop from the recorded phase. Commit after each phase so progress
   is never lost to the next compaction.

## Commit discipline

Commit after: scaffolding, each spec+oracle authoring, each round's
results+fixes, and each findings-file pruning. Conventional commits (repo uses
cog.toml). Small, frequent commits — compaction can happen at any time.
