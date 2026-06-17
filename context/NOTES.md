# Captured Notes — Triage

Loose notes left for triage, contextualized against the repo as of 2026-06-16.
Each note is recorded verbatim, interpreted, and checked against the existing
corpus (`yass.yass.yaml`, `spec/*`, `yass.v1.schema.json`, and the `context/`
docs). Nothing here is decided. The intended destinations on resolution are
`FUTURE.md` (language design), `IDEAS.md` (tooling/features), and `FIXES.md`
(concrete open issues).

Status legend:

- **NEW** — not recorded anywhere; no neighboring work.
- **PARTIAL** — an adjacent idea or the underlying problem is recorded, but this
  specific proposal is not.
- **EXISTS** — already true in the repo.
- **CONFLICTS** — contradicts a current rule or a stated non-goal.

| # | Note | Status |
|---|------|--------|
| 1 | `root.yass.yaml` — required project-root file | NEW |
| 2 | `yass.yass.yaml` — language format spec | EXISTS |
| 3 | `rules.yass.yaml` — tooling-enforced non-language rules | NEW (concept exists in prose) |
| 4 | Obligations must be ordered — implied control flow | CONFLICTS (problem captured) |
| 5 | `USES:` becomes "call-like" rather than soft-include | PARTIAL |
| 6 | Spec gains `intent:` — bounded spec-intent field | PARTIAL / CONFLICTS |
| 7 | `description:`, `intent:`, obligations need max lengths | NEW |

---

## Group A — File layout & project structure

The first three notes sketch a three-file split at the project root, separating
the language definition, the tooling-enforced project rules, and a required root
marker.

### 1. `root.yass.yaml` — project root file required — NEW

**Note (verbatim):** `root.yass.yaml - project root file required`

**Reading:** introduce a dedicated, named file that marks the project root and
whose presence is required.

**Status:** NEW. No occurrence of `root.yass.yaml` anywhere in the repo. The only
existing root concept is `cli.FindProjectRoot` (`spec/cli.shared.yass.yaml:7-27`),
which finds the root by walking upward for a `.git` entry, falling back to the
deepest ancestor "containing any `.yass.yaml` file" — never a specific marker
file. A required `root.yass.yaml` would change root discovery from inference to an
explicit, unambiguous marker.

**Tension / open question:**

- Root discovery today is honor-system inference; a required marker makes it
  deterministic but adds a mandatory file and a new failure mode ("no root
  marker") distinct from the existing `yass.findroot.no_marker`.
- Interaction with `cli.FindProjectRoot`'s `.git`-first precedence: does
  `root.yass.yaml` outrank `.git`, replace the `.yass.yaml` fallback, or become a
  third tier?
- What does the file *contain* — is it a preamble-only `.yass.yaml`, or a new
  shape? This connects to note 3.

### 2. `yass.yass.yaml` — language format spec — EXISTS

**Note (verbatim):** `yass.yass.yaml - language format spec`

**Reading:** `yass.yass.yaml` is the file that defines the language itself.

**Status:** EXISTS. `yass.yass.yaml` is the self-definition — `Document`,
`Preamble`, `Spec`, `Slot` (+ five slot subdocs), `Obligation`, `Normativity`,
`Guard`, `Reference`, `RefTarget`, `Keyword`. Its prose companion is
`context/yass-reference.md`. The note records the role accurately; the only open
angle is whether naming/role is stated explicitly relative to notes 1 and 3 (the
three-file split makes the division of responsibility explicit, where today it is
implicit).

### 3. `rules.yass.yaml` — non-language rules enforced by tooling — NEW (concept in prose)

**Note (verbatim):** `rules.yass.yaml - non-language rules enforced by tooling
(like root file existence)`

**Reading:** a dedicated file holding project-level rules the tooling enforces
but which are *not* part of the language's well-formedness — e.g. "a root file
must exist," 1:1 spec/code pairing, naming conventions.

**Status:** NEW as a file; the *distinction* is already recognized in prose:

- `context/yass-reference.md:151` — "Meta-rules (the spec *system*, distinct from
  in-spec obligations)" lists exactly this class (every code file paired with a
  `.yass.yaml`, every public symbol defined, etc.).
- `context/GUIDANCE.md` carries "candidate future meta-rules" that are
  **deliberately kept out** of `yass.yass.yaml` ("the generalized language
  definition is not the place for code-authoring steerage").

No document proposes giving these rules a *home file* or making them
tooling-enforced data. This note is the missing piece: today the meta-rules are
scattered prose; `rules.yass.yaml` would make them a first-class, enforced
artifact.

**Tension / open question:**

- Clean separation of concerns: language well-formedness (`yass.yass.yaml`) vs.
  project policy (`rules.yass.yaml`) vs. root marker (`root.yass.yaml`).
- What expresses a "rule" — reuse the obligation/slot grammar, or a new shape?
  If it reuses the grammar, several listed rules ("root file must exist," 1:1
  pairing) require filesystem/LSP awareness the language has so far avoided (cf.
  `IDEAS.md` "Pairing gate," gated behind the language-agnosticism constraint).
- Where decided policy lives: tooling-enforced rules and settled decisions may
  belong together once a home for them exists.

---

## Group B — Language-design proposals

### 4. Obligations must be ordered — implied control flow — CONFLICTS (problem captured)

**Note (verbatim):** `Obligations must be ordered - implied control flow`

**Reading:** obligation order within a slot should be significant, so a slot can
express ordered/priority control flow (try (1), then (2), …, first match wins)
as structure rather than prose.

**Status:** CONFLICTS with the current language; the underlying *problem* is the
most-repeated theme in the feedback.

- Direct contradiction: `yass.yass.yaml:94` — `Slot` `INVARIANT`:
  `MUST-NOT: carry meaning in the order of its obligations`.
- The pain is documented exhaustively. Priority-ordered "emit at most one error"
  chains (`cli.validate.CheckYAML`, `cli.validate.CheckPreamble`) are control
  flow buried in `INVARIANT` prose, and five implementers independently asked for
  a first-class ordering construct:
  - `context/SPEC-FEEDBACK.md` — `PRIORITY`/`PRECEDENCE` (C, Go, Rust ~lines
    100, 109, 164–183), `PRIORITY` slot vs. `ORDER: N` key vs. accept-as-prose
    (Bun, 414–418), `ORDERED` modifier / `PRIORITY` construct (Haskell, 498–512).
  - `context/OPEN-FEEDBACK.md:51-56, 99-101` — "priority chains … encoded as an
    INVARIANT prose sentence"; "extract priority chains into data (ordered arrays),
    not code."
  - `context/GUIDANCE-FEEDBACK.md:195-201, 326-332` — "priority-ordered error
    lists are implicitly control flow specs. Call that out."

**Tension / open question:**

- The note (make obligation order meaningful) is the reverse of the current
  `Slot` `INVARIANT`. Adopting it is a breaking language change, not an addition.
- The feedback's framing is narrower: a *dedicated ordered construct* (a
  `PRIORITY` slot or an `ORDER:` key) so that *most* slots stay unordered while
  the few that need ordering say so. Decide between (a) all obligations ordered,
  (b) a new ordered slot/keyword, (c) status quo + better guidance.
- If adopted, the `Slot` `MUST-NOT: carry meaning in the order` rule, the
  `OutputProfile` key-ordering guarantees, and any order-insensitive tooling
  (e.g. the merge-driver idea in `IDEAS.md`) all need revisiting.

### 5. `USES:` becomes "call-like" rather than soft-include — PARTIAL

**Note (verbatim):** `USES: becomes "call like" implying branching/jumping/
subroutine instead of soft-include`

**Reading:** redefine `USES` so it denotes invocation/transfer-of-control ("calls
into / jumps to a subroutine"), replacing today's "behavior draws on the target,
pointer that MAY inline."

**Status:** PARTIAL. The deficiency this targets is named precisely, but the
recorded fix differs.

- Recognized gap: `context/OPEN-FEEDBACK.md:411` — "the only signal is USES, which
  means the same thing for both 'I call this' and 'I run after this'."
- Recorded fix is to **add a new relation**, not redefine `USES`:
  `context/SPEC-FEEDBACK.md:281-287` (`DEPENDS`/`REQUIRES`), `429-436`
  (`AFTER`/`REQUIRES` "distinct from USES"), `50-53` ("a pipeline or sequencing
  relation"); also a `SEQUENCE` slot (170-171).
- Current definition stays soft: `context/yass-reference.md` — `USES` = "behavior
  depends on / draws on the target," a pointer tooling MAY inline; the
  discriminator vs. `SEE` is dependence, not invocation.

**Tension / open question:**

- Redefining `USES` (this note) vs. adding `REQUIRES`/`AFTER`/`CALLS` alongside it
  (the feedback). Redefinition is breaking and collapses the current
  depends-on/draws-on meaning; addition preserves it but grows the closed
  relation set (`CONFORMS`/`USES`/`SEE`).
- "Call-like / branching / jumping" implies execution-order and control-transfer
  semantics — close to the ordering question in note 4 and to the deferred
  retrieval-depth-per-relation idea in `FUTURE.md` ("`USES` → default depth 0").
- Inlining behavior: `CONFORMS` always inlines, `USES` MAY inline today. Does
  "call-like" change whether/when `query` resolves a `USES` target?

### 6. Spec gains `intent:` key — bounded field for spec intent — PARTIAL / CONFLICTS

**Note (verbatim):** `Spec gains "intent:" key - bounded field for spec intent`

**Reading:** add a per-spec, length-bounded free-text field stating the spec's
intent (distinct from the file-level preamble `description`).

**Status:** PARTIAL, and in tension with a stated non-goal.

- No `intent:` key is proposed anywhere ("intent" appears only as the English
  word). The nearest recorded idea is per-spec prose: `context/FIXES.md:40` —
  add a `description`/`purpose` key to the `spec` mapping — but scoped to fixing
  redundant `list` output, not "spec intent."
- Conflicts with `context/GUIDANCE.md` "Deliberate non-goal: no free-prose
  channel": the *only* free-text field is the preamble `description`; withholding
  a prose channel is an intentional forcing function so authors express intent as
  obligations. `context/IDEAS.md` (`UNSURE` entry, LLM-native + open-questions
  sections) flags that any authored prose channel "reopens the deliberately-closed
  no-free-prose relief valve."
- The schema (`yass.v1.schema.json`) sets `additionalProperties: false` on the
  spec object, so `intent:` would require a schema change plus a new
  `yass.spec.unknown_key`-style allowance.

**Tension / open question:**

- The whole proposal turns on the no-free-prose non-goal. `intent:` is exactly the
  channel that non-goal closes. The note's "bounded field" framing (see note 7)
  is the mitigation — a hard length cap to stop narrative bloat.
- If accepted, decide its relationship to the per-spec `purpose` idea in
  `FIXES.md` (one field or two?) and whether `list`/`query` surface it.
- Does `intent:` weaken the forcing function (authors write intent prose instead
  of obligations), the same risk `IDEAS.md` raises for `UNSURE`?

### 7. `description:`, `intent:`, and obligations need max lengths — NEW

**Note (verbatim):** `description: and intent: and obligations need max lengths
defined`

**Reading:** define hard maximum lengths for the preamble `description`, the
proposed `intent:` field (note 6), and obligation prose values — enforced by
schema/validate.

**Status:** NEW. No `max length` / `length limit` / `char limit` anywhere.

- The only length-related rules are display-time: `list` *truncates* the
  description to terminal width (`spec/cli.list.yass.yaml`), a rendering concern,
  not a stored bound.
- The opposite direction exists as an idea: `IDEAS.md` "lint command" mentions
  "stub-length obligations" — a *minimum*-quality signal (catch hollow specs),
  advisory only, not a maximum.

**Tension / open question:**

- This is the enforcement mechanism behind notes 6 and 1/3: bounded fields are
  what keep `intent:` (and any future prose channel) from becoming the narrative
  bloat the no-free-prose non-goal exists to prevent.
- Where enforced — `yass.v1.schema.json` (`maxLength`) for editor-time, plus a
  `validate` error code? Picking limits needs the token-cost lens already noted
  in `yass-reference.md` (count_tokens) rather than raw character counts.
- Granularity: one global limit, or per-field (`description` vs. `intent` vs.
  obligation prose)? Obligations are the bulk of spec content, so a cap there has
  the largest effect and the largest risk of rejecting legitimate prose.

---

## Cross-cutting

- **Notes 1, 3, and 2 form one proposal**: an explicit three-file root split
  (`root.yass.yaml` marker, `yass.yass.yaml` language, `rules.yass.yaml` policy).
  Triage them together; they share the question of what a "rule" and a "root"
  file contain.
- **Notes 4 and 5 both touch control flow / execution order** (ordered
  obligations; call-like `USES`). The feedback corpus already pushes hard on this
  axis via `PRIORITY` constructs and a `REQUIRES`/`AFTER` relation; reconcile the
  notes with that body of feedback before deciding.
- **Notes 6 and 7 are a pair**: a bounded `intent:` field is only viable with the
  max-length enforcement of note 7, and both must be weighed against the
  no-free-prose non-goal.
- **No home for settled decisions yet.** Several of these notes (especially the
  non-goal tensions in 4 and 6) will need a decision recorded somewhere once
  resolved; there is no decisions log in the repo today.
