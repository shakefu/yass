# Man-Page Alignment — shaping v1

Status: proposal, not decided. Captures what it would take to realign yass's
structural vocabulary to POSIX/man-page section conventions, and why that may beat
leaning on RFC 2119 alone. yass is pre-release (v0.0.x); **v1 is still being
designed and nothing is published**, so this is in-development shaping of the v1
vocabulary, not a breaking change or a migration. The `version: v1` key is
unchanged — we are deciding what v1 *says*.

## Why man pages

yass already relies on the model having *learned* its keywords cold — that's what
makes a spec deterministic to an agent. RFC 2119 supplies the **normativity**
axis (`MUST`/`SHOULD`/`MAY`). But RFC 2119 says nothing about *structure*, so
yass invented or borrowed the rest. Man pages are the other vocabulary in the
same recognition class: POSIX-standardized, consistent across decades of tooling,
written as UPPERCASE sections, and saturated in pretraining. The realization that
prompted this doc is that **yass is already man-shaped** — most of its slots and
relations map onto man sections one-for-one. Aligning the words exactly buys two
things:

1. **Better training alignment.** `SEE ALSO`, `RETURN VALUE`, `EXIT STATUS`,
   `CONFORMING TO`, `ERRORS`, `DIAGNOSTICS` are stronger, less ambiguous learned
   tokens than the trimmed forms. Bare `SEE` could read as an imperative; `SEE-ALSO`
   is unmistakably the cross-reference idiom. Bare `ERROR` is a word; `ERRORS` is a
   man section.
2. **More flexibility.** The man section set is broader than five function-shaped
   slots and covers both *command*-shaped specs (`EXIT STATUS`, `OPTIONS`,
   `SYNOPSIS`) and *function*-shaped specs (`RETURN VALUE`, `ERRORS`). That
   directly answers the feedback that non-function specs (`OutputProfile`, the
   self-definition) read awkwardly when forced through the five DbC slots.

### The standing keyword rule this implies

> Keywords are drawn from **RFC 2119** (normativity), **POSIX / man-pages**
> (structure and relations), or **Design-by-Contract** (the contract gaps man
> pages don't cover), in that priority — never invented. Borrow the word for its
> learned recognition; do **not** import the man page's loose prose semantics.

The three vocabularies are orthogonal — normativity vs. section vs. contract —
which preserves the "keywords don't collide in meaning" property that lets `CONFORMS`
inline safely.

## The casing decision (the enabler)

Man sections are multi-word (`SEE ALSO`, `RETURN VALUE`, `EXIT STATUS`). yass keys
are single tokens. yass already resolves this exact problem for RFC's `MUST NOT`
by **hyphenating**: `MUST-NOT`, `SHOULD-NOT`, and the two-word `SIDE-EFFECT`. Apply
the same rule to man's multi-word sections:

| Man idiom | yass key |
|-----------|----------|
| `SEE ALSO` | `SEE-ALSO` |
| `RETURN VALUE` | `RETURN-VALUE` |
| `EXIT STATUS` | `EXIT-STATUS` |
| `CONFORMING TO` | `CONFORMS-TO` |

Hyphenation keeps keys greppable, valid as YAML/JSON-Schema property names, and —
crucially — **needs no ref-grammar change**: the slot token grammar is already
`::[A-Z][A-Z-]*` (it accepts `SIDE-EFFECT` today), so `::EXIT-STATUS` and
`::RETURN-VALUE` parse with no edit. The model still recognizes the hyphenated
form as the man idiom, just as it reads `MUST-NOT` as RFC's `MUST NOT`.

## The realignment

Counts are current occurrences in `yass.yass.yaml` + `spec/*.yass.yaml`.

| # | current | proposed | Man origin | Change | Edit sites |
|---|---------|----------|------------|--------|------------|
| 1 | `CONFORMS` | `CONFORMS-TO` | `CONFORMING TO` / `STANDARDS` | rename | 20 keys + schema + self-def |
| 2 | `SEE` | `SEE-ALSO` | `SEE ALSO` | rename | **0 keys (unused)** + schema + self-def |
| 3 | `ERROR` | `ERRORS` | `ERRORS` | rename | 24 slot decls + schema + self-def; **no refs** |
| 4 | `RETURN` | `RETURN-VALUE` | `RETURN VALUE` | rename (optional) | many slot decls + 16 `::RETURN` refs + schema + self-def |
| 5 | exit-code prose (`cli.ExitCode`) | `EXIT-STATUS` slot | `EXIT STATUS` | new construct | recast `cli.ExitCode` |
| 6 | error table (`cli.errors`) | `DIAGNOSTICS` | `DIAGNOSTICS` | new construct | recast `cli.errors` |

### 1. `CONFORMS` → `CONFORMS-TO`

The strongest case in the set: man's `CONFORMING TO` is *exactly* this relation
(a spec adhering to a referenced contract), giving `CONFORMS` a second learned
anchor on top of its RFC adjacency. This also kills the earlier `SATISFIES` idea —
man never says "satisfies," it says "conforming to." Adding the `-TO` particle
makes the relational reading unmistakable. Pure rename: 20 obligation keys, the
two schema occurrences, and the `Reference` spec in `yass.yass.yaml`. Targets
(`::RETURN`, `::INVARIANT`) are unaffected — only the relation key changes.

- Open: `CONFORMS-TO` (verb-tense-consistent with `USES`) vs `CONFORMING-TO`
  (man-exact). Recommend `CONFORMS-TO`.

### 2. `SEE` → `SEE-ALSO`

Near-free. `SEE` is defined in the language but **used in zero specs today**, so
the rename touches only the schema and the `Reference` spec — no obligation edits.
`SEE ALSO` is the single most recognizable cross-reference idiom in technical
documentation. No reason not to.

### 3. `ERROR` → `ERRORS`

Man's section is plural (`ERRORS`). 24 slot declarations change, plus the schema
property, the `Slot` enum, and the `Slot.ERROR` sub-spec in the self-definition.
**No `::ERROR` refs exist**, so nothing in the ref graph moves. Mechanical.

### 4. `RETURN` → `RETURN-VALUE` (flagged optional — not in the original ask)

The natural completion of the set, but the **heaviest** change: every `RETURN`
slot plus all 16 `::RETURN` refs. Two ways to take it:

- **Adopt `RETURN-VALUE`** for full man fidelity (function postcondition), and use
  `EXIT-STATUS` (below) for command exit — man itself separates `RETURN VALUE`
  (§3 functions) from `EXIT STATUS` (§1 commands), a distinction yass currently
  blurs.
- **Keep `RETURN`** as the generic postcondition and only add `EXIT-STATUS` where a
  command's exit is the subject. Less churn; slightly less man-faithful.

Recommend deciding this together with #5, since they're the same function-vs-command
split.

### 5. Exit-code prose → `EXIT-STATUS` slot

Today `cli.ExitCode` is a spec whose `RETURN` slot carries exit behavior as prose
(`MUST: exit 0 on success`, `WHEN … MUST: exit 1`). Man models this as its own
section. Promoting `EXIT-STATUS` to a **slot** (a sixth slot, command-shaped) lets
command specs state exit behavior in the section the model expects, instead of
overloading `RETURN` or maintaining a separate `ExitCode` spec that every other
spec must `CONFORMS-TO`. This is a design change, not a rename: it grows the slot
set and shifts where exit obligations live.

### 6. Error table → `DIAGNOSTICS`

`cli.errors` is the error-code registry — the artifact every implementer called
the single most valuable spec, and also the one most awkwardly encoded (≈85 prose
`MUST`s each packing a code + message + exit). Man's `DIAGNOSTICS` section is
exactly "the messages this program emits." Adopting the name is clean; **two
larger questions ride along and are out of scope for this doc**:

- **Centralized vs. per-spec.** Man pages put a `DIAGNOSTICS` section in each
  page; yass centralizes all codes in `cli.errors`. The feedback wanted
  centralization (one source of truth). Decide whether `DIAGNOSTICS` is a per-spec
  slot, a standalone registry spec, or both.
- **Prose vs. structured rows.** The "error table should be data, not prose"
  overhaul (mapping-valued obligations) is a separate decision. If it happens,
  `DIAGNOSTICS` is its natural home and man supplies the name — but renaming and
  restructuring should not be conflated.

## What stays

- **Normativity** — `MUST` / `MUST-NOT` / `SHOULD` / `SHOULD-NOT` / `MAY`. Pure
  RFC 2119; man is descriptive and offers nothing here.
- **`WHEN`** guard — plain-English conditional, no man equivalent, no change.
- **`INPUT` / `INVARIANT` / `SIDE-EFFECT`** — DbC fills what man doesn't name
  cleanly. (`INPUT` ≈ `SYNOPSIS`/`OPTIONS` but is clearer as one word;
  `SIDE-EFFECT` is scattered across man's `FILES`/`ENVIRONMENT`/`DIAGNOSTICS`;
  `INVARIANT` has no man section.)
- **`USES`** — the lone relation man does **not** supply. Man models only two
  inter-document relations, `SEE ALSO` and `CONFORMING TO` (= our `SEE-ALSO` and
  `CONFORMS-TO`); the dependency middle is ours. It stays in-house and rides on
  plain-English recognition, so its meaning must be pinned tightly — it has no
  external vocabulary to lean on. (57 uses; the most-used relation, so any change
  here is expensive and unmotivated.)

## Sections man unlocks later (not now)

Adopting the man vocabulary leaves room to grow without inventing: `EXAMPLES`
(the Python agent's requested example slot), `OPTIONS` / `SYNOPSIS` (argv-shaped
specs), `ENVIRONMENT`, `FILES`, `STANDARDS`. Each is a learned token available
when a real spec needs it.

## Scope of the change

Because nothing is published, this is a single coordinated edit of the current
in-development artifacts, not a migration:

- the self-definition (`yass.yass.yaml`) — slot enum, relation keys, affected
  sub-specs;
- the JSON Schema (`yass.v1.schema.json`) — slot and relation property names;
- the CLI spec set (`spec/*.yass.yaml`) — the rename sites counted above;
- the context docs that name the old keywords (`yass-reference.md`,
  `TEST-TAXONOMY.md`).

Renames #1–#4 are mechanical find-and-replace; #5 and #6 need authoring judgment.
No migrator is involved — that tooling (`IDEAS.md`'s `yass migrate`) only matters
once a version is published and real specs must move between versions, which is not
where we are.

Self-validation stays the gate: `yass validate` must pass on the revised
self-definition and spec set before this lands.

## Open questions

1. `CONFORMS-TO` vs `CONFORMING-TO`.
2. Adopt `RETURN-VALUE`, or keep `RETURN` and add only `EXIT-STATUS`? (The
   function-vs-command split — decide #4 and #5 together.)
3. Is `EXIT-STATUS` a sixth slot, and does the five-slot identity of yass survive
   becoming six/seven?
4. `DIAGNOSTICS`: centralized registry, per-spec slot, or both — and is it coupled
   to the structured-obligation overhaul or kept independent?
5. Hyphen as the multi-word separator is assumed throughout (consistent with
   `MUST-NOT`/`SIDE-EFFECT`); confirm we never want spaces-in-quoted-keys instead.
6. Does `USES` stay as the single in-house relation, or does its lack of a man
   anchor justify re-examining the dependency middle entirely?
