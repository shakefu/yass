# Fixes & Follow-ups

Concrete issues discovered during spec maintenance that need resolution.

## CONFORMS without ::SLOT

The `yass.yass.yaml` specs (`Slot`, `Normativity`, `Guard`, `Reference`) originally
had bare `CONFORMS: Keyword` refs. The query subcommand rejects these at resolution
time (`yass.query.conforms_no_slot`) because the inliner needs a slot target to know
which obligations to pull in.

We patched them to `CONFORMS: Keyword::INVARIANT` to unblock query, but the deeper
question is open: **what should bare-spec CONFORMS mean?**

Options:
- **Inline all slots** — a bare `CONFORMS: Foo` inlines every slot from `Foo`. Simple
  but potentially noisy.
- **Require ::SLOT always** — current v1 behavior. Explicit but verbose when a spec
  has only one slot.
- **Inline all slots only when the target has a single slot** — convenience shorthand,
  but adds a rule that depends on the target's shape.

The language spec (`RefTarget`) says `::SLOT` is optional (`MAY`), so the grammar
allows it — the restriction is purely in the query subcommand's `InlineConforms` logic.
The spec and tooling should agree on what bare CONFORMS means.

## List output format

`yass list` repeats the file-level preamble description for every spec in a file. When
a file has many specs (e.g. `cli.query.yass.yaml` with 5, `yass.yass.yaml` with 15),
the output is redundant and wastes tokens for LLM consumers.

Problems:
- The same description is repeated per-spec, not per-file.
- The preamble description describes the *file*, not the individual spec — so it gives
  no useful signal for distinguishing specs within the same file.
- The tab-separated format makes it hard to scan visually.

Possible improvements:
- **Per-spec description field** — add a `description` or `purpose` key to the `spec`
  mapping so each spec can carry its own one-liner.
- **Drop the description column** — just emit `file\tspec_name`, one per line. Simpler,
  no redundancy.
- **Group by file** — emit the file path once, then indent spec names below it. Cuts
  repetition without losing the file-level description.
- **Hybrid** — group by file, show preamble description once per group, show per-spec
  descriptions (if present) per spec.

Any change here touches `spec/cli.list.yass.yaml` and the list subcommand implementation.
