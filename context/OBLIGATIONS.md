# yass — Slots and Obligations

*Write path, 3 of 5: authoring → documents → **obligations** → references → coverage. Run each as `yass docs NAME` and read it whole.*

What a spec document holds: its five slots, the obligations inside them, and the casing that keeps keywords distinct from prose.

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
for any failure not matched by a guarded obligation in the same slot. A residual is
meaningful only when the guards leave inputs unmatched: if the guarded obligations
already account for every input, the residual set is empty and a guard-less catch-all
is **dead** — it can never fire, and MUST-NOT be carried. State that exhaustiveness in
the spec's `INVARIANT` instead: a guard-less `ERROR` obligation is itself read as the
residual, so exhaustiveness cannot live there.

`ERROR` is also where rejections live, and only there: a rule phrased as *be rejected*
sits in an `ERROR` slot, and validation enforces exactly those rules — a MUST or
MUST-NOT in any other slot binds authoring without being a validation rejection
(reachability and the modeline SHOULD are lint's advisory domain).

The residual discipline generalizes beyond `ERROR`: whenever a slot branches on a
**closed set of values** — most commonly an `INPUT` that dispatches on a subcommand,
mode, or enum — it must state the behavior for a value outside that set, or one that is
missing. The residual is to a dispatch what the guard-less catch-all is to the error
table. (The draft-time pass that walks every such case — closed sets, guards,
segmentation boundaries, failures, trusted inputs: see `yass docs coverage`.)

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
- **Structure stays in keys** — a condition on when the obligation applies is its
  `WHEN` guard, not a clause in its prose; a second requirement is a second
  obligation; a reference is a relation key. A keyword mentioned rather than used is
  written in backticks. `yass lint` reports the prose forms it can see as
  `yass.lint.prose_keyword`.
- **Closed key set** — a normativity keyword, `WHEN`, and the relation keys are the
  only keys an obligation mapping may carry; any other key is a validation error.
- **Prose values** — a normativity keyword's or `WHEN`'s value must be a **string**:
  mappings, sequences, core-schema nulls, and non-string scalars (numbers, booleans)
  are rejected. An absent value and unquoted `null`, `Null`, `NULL`, or `~` resolve
  to null; quoted empty strings and quoted `"null"` or `"~"` remain strings. Resolve
  YAML tags before checking this rule rather than testing only the scalar text.
- **Ref-only** — a mapping with one or more relation keys and **no** normativity keyword
  and **no** `WHEN`. Allowed; it adds no obligation of its own and resolves per its
  relation (a **slot-targeted** `CONFORMS` transcludes; a whole-spec `CONFORMS` stays in
  place as a declaration that the enclosing spec conforms to the referenced spec as a
  whole; a ref-only `USES` — a bare `- USES: Name` list item —
  transcludes too, appending its design block's content).

## Obligation shape

An obligation states one requirement a reader checks against the code; it is never the
code. Naming the symbol it describes, with its signature, flags, defaults, and exact
names, is right: those are the behavior. Beyond that, these rules are absolute:

- **One requirement.** A second requirement, step, or effect is a second obligation.
- **No procedure.** No call sequence and no step order ("and then", "after that"). A
  mechanism the implementer must follow is a `design:` block bound by `USES`.
- **No restatement of another spec.** What another spec guarantees is its `CONFORMS`
  key (it binds) or `SEE` key (it points), never a paraphrase in prose.
- **No reference by prose token.** A `path@Name` in prose is an edge no command follows.
- **No literal code** beyond the symbol's own name and signature: no assignments, struct
  literals, interface bodies, or arguments for the tests to pass.
- **No error mapping outside `ERROR`.** Each failure and its outcome is its own `ERROR`
  obligation under a `WHEN` guard.
- **No reading of another obligation** ("the guard above"): each one stands alone.
- **At most one reason, carrying no requirement.** One trailing clause may say why
  (", because …"). If deleting it would change what an implementer builds or tests, it
  hid a requirement: state that as its own obligation. A parenthetical aside never stays.

`yass lint` reports the forms it can see as `yass.lint.obligation_shape`: prose over 400
characters, several clauses or sentences, procedural connectives, a parenthetical reason,
code shapes, and an error mapping outside `ERROR`. Validation rejects none of them,
because each depends on what the prose says and validation rejects only structure.

## Casing convention

Fixed-meaning keywords are written **UPPERCASE**; content-bearing field keys are
**lowercase**.

- **UPPERCASE** (the vocabulary — the key *is* the meaning): slot names (`INPUT`,
  `RETURN`, `ERROR`, `SIDE-EFFECT`, `INVARIANT`), normativity (`MUST`, `MAY`, …), the
  guard `WHEN`, and the relations (`CONFORMS`, `USES`, `SEE`). Also the slot portion of a
  ref target (the part after `::`).
- **lowercase** (labels for values): `spec`, `design`, `type`, `content`, `description`,
  `version`, `related`.
- In **prose**, write a keyword UPPERCASE when you mean the keyword itself (e.g. "the
  `RETURN` slot"), so it cannot be confused with the ordinary word.

Rationale: it mirrors the RFC 2119 uppercase-keyword convention already used for
normativity, unifies the structural vocabulary as one visually distinct layer, and
disambiguates homographs (`RETURN` the slot vs. "return" the word).

In the self-definition the rule is encoded once as the `Keyword` construct; slots,
normativity, the `WHEN` guard, and relations declare `CONFORMS: Keyword::INVARIANT`
rather than restating it — transcluding the invariant instead of conforming to it,
because the keyword constructs are the rule's home, not its conformers.

## Footgun

- Obligation prose containing `: ` (colon-space) mis-parses as a nested YAML mapping and
  **must be quoted**.

---

Next in the write path: `yass docs references`.
