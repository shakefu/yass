# yass — Language Reference

**yass** — *Yet Another Spec Syntax*. A YAML-based specification language for AI-agent
development: agents (or humans) write specs that capture desired behavior, and tooling
consumes them to drive test (TDD) and implementation generation.

## File

- A yass file uses the **`.yass.yaml`** extension and is a **YAML
  multi-document stream** (`---`-separated).
- Read as UTF-8.
- SHOULD carry the schema modeline as its first line, so editors validate against the
  JSON Schema with no per-repo config:
  `# yaml-language-server: $schema=https://textla.dev/yass/v1.schema.json`

## Preamble

- **Required**, **exactly one**, and **MUST be the first** document of every file.
  Making it mandatory simplifies parsing and validation.
- Identified structurally by the **absence of a `spec:` key**.
- The file name is the identity of the whole unit; the preamble does **not** repeat it.
- Keys:
  - `description` — **required**; file-level summary (block scalar `>` allowed).
    Surfaced by a file `query`; **truncated** in `list` output.
  - `version` — **required**; the yass language version the file targets. **Pinned at
    `v1`** in the current revision.
  - `related` — *optional*; a list of freeform external resource locations (paths, URLs,
    issue links). A distinct channel from the `see` relation; **never resolved** by
    tooling.

## Spec document

- A `spec:` key naming the spec (may be namespaced, e.g. `pkg.Symbol`) plus zero or
  more **slots**.
- **Name charset:** letters, digits, `.`, `_`, and `-` (so a slot-keyword name like
  `Slot.SIDE-EFFECT` is legal and addressable).
- Names **MUST be unique within a file**, so refs resolve unambiguously.
- Granularity: **one public symbol / endpoint → one spec.** Also the unit of retrieval.
- The five slots are themselves defined in `yass.yass.yaml` as `Slot.INPUT`, `Slot.RETURN`,
  `Slot.ERROR`, `Slot.SIDE-EFFECT`, and `Slot.INVARIANT`.

## Slots

The recognized set (each is a YAML list of obligations), written **UPPERCASE** (see
*Casing convention* below):

- `INPUT`
- `RETURN`
- `ERROR`
- `SIDE-EFFECT`
- `INVARIANT`

The first four are **function-shaped** and map onto Design by Contract: `INPUT` =
precondition (what it accepts), `RETURN` = postcondition (what it yields), `ERROR` =
failure modes, `SIDE-EFFECT` = observable effects beyond the return. `INVARIANT`
completes the contract: **constraints that always hold**, independent of any call.

`INVARIANT` is **universal** (any spec may use it) but is a **slot of last resort**: an
obligation SHOULD be placed in `INVARIANT` only when none of `INPUT`, `RETURN`, `ERROR`,
or `SIDE-EFFECT` is suitable. It exists so always-true constraints are not forced into
`SIDE-EFFECT` (which means *effects*) — not as a general catch-all.

In the `ERROR` slot specifically, a **guarded** obligation (with `WHEN`) names one
specific failure mode, and a **guard-less** obligation is the **residual** — the policy
for any failure not matched by a guarded obligation in the same slot. State a residual
whenever a spec rejects anything; but a foreseeable, named failure with its own
observable outcome (a distinct error code, message, or exit status) belongs in its own
guarded obligation, never folded into the residual.

The residual discipline generalizes beyond `ERROR`: whenever a slot branches on a
**closed set of values** — most commonly an `INPUT` that dispatches on a subcommand,
mode, or enum — it must state the behavior for a value outside that set, or one that is
missing. The residual is to a dispatch what the guard-less catch-all is to the error
table.

When a spec describes something that is not a function (e.g. the language defining
itself), read the function-shaped slots structurally: `INPUT` = the form a thing takes,
`RETURN` = what a well-formed thing denotes, `ERROR` = malformed forms that must be
rejected, `SIDE-EFFECT` = real effects of processing it.

## Obligation

An obligation is a **YAML mapping** (a list item under a slot):

- **Normativity** — exactly one key, *unless ref-only*. RFC 2119 / RFC 8174 vocabulary,
  **negatives hyphenated**:
  `MUST`, `MUST-NOT`, `SHOULD`, `SHOULD-NOT`, `MAY`. Value = the obligation prose.
- **Guard** (optional) — `WHEN`, value = condition prose. Expresses a **sufficient**
  condition. If `WHEN` is present it **MUST** be accompanied by a normativity keyword.
- **References** (optional) — `CONFORMS` / `USES` / `SEE`, each at most once per
  obligation, value = a **single** ref-target string.
- **Ref-only** — a mapping with one or more relation keys and **no** normativity keyword
  and **no** `WHEN`. Allowed; it adds no obligation of its own and resolves per its
  relation (only `CONFORMS` transcludes).

## References

- Relation is the **YAML key**; target is a **single string**.
- **Target syntax:** `path@SpecName::SLOT`
  - A **bare spec name** (`SpecName`) addresses a spec in the **same file**.
  - `@` separates a path from the spec name (`path@SpecName`) for **other files**.
  - `::` separates the spec name from a slot (`SpecName::SLOT`); omit it to address the
    whole spec.
  - **Path resolution:** a `./` or `../` path is **relative to the referencing file**; a
    path without a leading dot is **from the project root**. The `.yass.yaml` extension
    is omitted.
  - The **slot is the finest addressable unit**. Named anchors, never line numbers.
  - (`#` was rejected as a separator — a leading `#` is a YAML comment, and `@` is a
    reserved YAML indicator that can't start a plain scalar either. Making same-file refs
    bare names means no target ever leads with an indicator, so none need quoting.)
- **Relations:**

  | Relation   | Resolution             | Meaning                                                   |
  |------------|------------------------|-----------------------------------------------------------|
  | `CONFORMS` | always inlined         | hard requirement — must match the referenced spec or slot |
  | `USES`     | pointer, MAY inline    | behavior depends on / draws on the target                 |
  | `SEE`      | pointer, never inlined | related context the behavior does not depend on           |

  `CONFORMS` always inlines, because conformity is a hard requirement — the conformer
  needs that spec or slot in front of it. `USES` is a pointer that tooling MAY inline
  (e.g. surface once per session); `SEE` is a pure pointer, never inlined. The
  discriminator: use `USES` when the obligation's behavior depends on or draws on the
  target, `SEE` when the target is merely related context.

- **Slot-targeted `USES` carries a dataflow reading.** When a `USES` target names a slot
  — characteristically an `INPUT` that points at a producer's `RETURN`,
  `USES <producer>::RETURN` — it means the obligation **consumes or builds on the data
  that slot produces**: the data crossing the boundary is exactly what that slot yields,
  so the producer's `RETURN` guarantees characterize it here. This is the structural
  anchor for a pipeline or producer/consumer relationship. It does **not** by itself
  decide the trust boundary — which of the producer's guarantees the consumer relies on
  versus re-checks is the consuming spec's own obligation to state (see GUIDANCE,
  *Composition*).

- DRY is achieved by **transclusion** (inlining), not bare pointers.

## Casing convention

Fixed-meaning keywords are written **UPPERCASE**; content-bearing field keys are
**lowercase**.

- **UPPERCASE** (the vocabulary — the key *is* the meaning): slot names (`INPUT`,
  `RETURN`, `ERROR`, `SIDE-EFFECT`, `INVARIANT`), normativity (`MUST`, `MAY`, …), the
  guard `WHEN`, and the relations (`CONFORMS`, `USES`, `SEE`). Also the slot portion of a
  ref target (the part after `::`).
- **lowercase** (labels for values): `spec`, `description`, `version`, `related`.
- In **prose**, write a keyword UPPERCASE when you mean the keyword itself (e.g. "the
  `RETURN` slot"), so it cannot be confused with the ordinary word.

Rationale: it mirrors the RFC 2119 uppercase-keyword convention already used for
normativity, unifies the structural vocabulary as one visually distinct layer, and
disambiguates homographs (`RETURN` the slot vs. "return" the word). The disambiguation
is a contextual/attention effect, not a separate token; uppercase common words also tend
to tokenize into distinct sub-tokens, but treat that as a minor bonus, not the
justification. (Token cost is measurable via Anthropic's `count_tokens` endpoint, which
returns counts but not boundaries.)

In the self-definition the rule is encoded once as the `Keyword` construct; slots,
normativity, the `WHEN` guard, and relations declare `CONFORMS: Keyword` rather
than restating it.

## Provenance (emitted fragments)

- Inlined obligations carry a YAML comment naming the source, e.g.
  `# CONFORMS: db/user@User::RETURN`.
- An emitted fragment identifies the **queried** spec; there is **no host `file:`
  header** (redundant and able to contradict — the filesystem is the source of truth).

## Footgun

- Obligation prose containing `: ` (colon-space) mis-parses as a nested YAML mapping and
  **must be quoted**. (Open: lint vs. require quoting / block scalars.)

## Meta-rules (the spec *system*, distinct from in-spec obligations)

- Every code file MUST be paired with its `.yass.yaml` file.
- Every public symbol or endpoint SHOULD have a definition in the `.yass.yaml` file.
- The `.yass.yaml` file MUST be structured parseably.
- The `.yass.yaml` file SHALL NOT require special syntax to be read by an AI agent.
- The `.yass.yaml` file MUST conform to the format when written, and MUST be validatable by
  tooling.

## Notes / open items

- Resolution: `CONFORMS` is inlined into the referencing spec (one level in v1; cycles,
  transitive resolution, batching, and depth are deferred). `SEE` is a pure pointer,
  never inlined. `USES` is a pointer that tooling **MAY** inline.
- Idea (undiscussed until now, not in v1): tooling MAY surface each `USES` target **once
  per calling session** as its own appended doc fragment — present in context but not
  repeated, and not inlined into the referencing spec. `SEE` would stay a pure pointer.
- Drift detection (content hashing) is deferred to a generated index.
- Verification is out of scope — tooling routes/retrieves, never verifies obligation
  content (keeps it language-agnostic). A `COMPATIBLE` relation was deliberately
  excluded.
- Necessary-condition guards (`ONLY WHEN` / `UNLESS`) are deferred; `WHEN` is sufficient
  only.
- Extension: `.yass.yaml`, chosen over bare `.spec` after a collision
  review. Bare `.spec` is owned by RPM spec files / PyInstaller / Vim+Neovim filetype /
  GitHub Linguist / freedesktop MIME, and — decisively — a bare `.spec` never gets the
  `yaml` language id, so it earns neither free YAML highlighting nor JSON-Schema
  association. A trailing `.yaml` fixes that; the `.yass` infix namespaces it.
