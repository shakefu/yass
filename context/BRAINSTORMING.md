# yass — Brainstorming a Spec

How an agent turns a user's request into a spec set that another agent can implement
cold. Read this when a project or a feature has no spec yet. The keywords below bind the
agent running this flow, in the RFC 2119 sense.

## Why the dialogue exists

The spec set is the only thing the implementing agent will hold: not this conversation,
not this document. Every behavior the spec leaves unstated, that agent decides alone, and
two agents decide differently. Each question below moves one such decision to the user
while it is still cheap. The output is obligations — not notes, not a design essay.

## Gate

- MUST-NOT write product code, scaffold, install a dependency, or create a project before
  the user approves the spec set.
- MUST-NOT write a spec file before the user confirms the written-back understanding
  (stage 2).
- A yes approves what was shown, and nothing that does not exist yet: approving the scope
  does not approve a draft, and approving a draft does not approve implementing it.
- MAY read the repository and run `yass` commands at any stage.

## Stage 1 — Classify, out loud

State the class in one sentence before the first question, so the user can override it.

- **Fresh project**: `yass` reports `project none`. The root spec comes first, then the
  specs it reaches.
- **Feature**: a root governs the directory. MUST read `yass list` and then `yass query`
  the documents the feature touches, and MUST-NOT open spec files by hand for what those
  commands serve. New behavior extends existing specs before it adds new ones.
- **Several independent subsystems** in one request: decompose first — name the pieces,
  their order, and the boundaries between them — then brainstorm one piece. Each piece
  gets its own pass through this flow.

When in doubt, take the larger class. Complexity found mid-flow upgrades the class; nothing
downgrades it.

## Stage 2 — Establish intent

- MUST know, before proposing anything: what the thing is for, who or what uses it, and
  what "done" observably looks like. Ask only what the request has not already answered.
- MUST ask one question per message. SHOULD offer choices, with a recommended default
  marked, so the user corrects rather than composes.
- WHEN a choice shapes the spec — a wire format, a storage model, a dispatch structure —
  MUST present two or three options with trade-offs and one recommendation, and ask.
- MUST then write the understanding back in a few lines, separating what the user said
  from what was assumed, and get a correction or a yes before drafting.

## Stage 3 — Round the edges

Interview by slot, for each spec the set will hold. Every answer becomes an obligation;
an answer nobody gave becomes an invention downstream. Skip a question only when the
answer is already known — never because it seems obvious.

- **INPUT** — the forms accepted. WHEN the input dispatches on a closed set (subcommand,
  mode, enum): MUST ask what an out-of-set value and a missing value do.
- **Segmentation** — WHEN input is split into records, lines, fields, or tokens: MUST pin
  the exact separator and its character class, empty input, a blank interior unit, a
  leading or repeated separator, how many trailing separators are absorbed, and input that
  is only separators.
- **RETURN** — what is yielded, in its exact shape: format, ordering, numbering, and what
  an empty result looks like.
- **ERROR** — each foreseeable failure with its observable outcome (code, message, exit
  status), one per guarded obligation. Then the residual: what every other failure does.
  WHEN the named failures exhaust the input: state that instead — a residual that can
  never fire is a contradiction.
- **SIDE-EFFECT** — what changes outside the return, and what MUST-NOT change.
- **INVARIANT** — only what fits none of the above.
- **Boundaries** — WHEN one spec consumes what another produces: MUST ask which producer
  guarantees the consumer trusts without re-checking, which it re-checks, and what it does
  when a trusted guarantee is violated — even if the answer is "unspecified".
- **Constraints that are not behavior** — a required stack, an algorithm, a startup order:
  these become `design:` blocks bound with `USES`, not prose in a spec.

## Stage 4 — Draft

- MUST read `yass docs guidance` before writing the first file, and `yass docs reference`
  whenever a construct is in doubt.
- **Fresh project**: MUST write `root.yass.yaml` first, so a reader who stops there can
  still say what the project is. Then one spec file per code file, one `spec:` per public
  symbol, files and documents in implementation order.
- MUST write only obligations. Nothing from the conversation survives as prose except
  inside a `design:` block something `USES`; rejected alternatives do not survive at all.
- MUST run `yass validate` and `yass lint` and fix every finding before showing a draft.

## Stage 5 — Review

- MUST show the user each spec document — the draft is the write-back — and ask after
  each whether it is right. A short file is shown whole; a long one, spec by spec.
- Before showing, MUST check what the tools cannot: an obligation with two readings, two
  obligations that contradict, a foreseeable failure folded into a residual, a residual
  the guards have exhausted. Fix, then show.
- The user's corrections go back into the files. Re-run validate and lint after each.

## Stage 6 — Hand off

- WHEN the user approves the spec set: MUST stop. Report where the root is and how to
  begin: an implementing agent starts fresh in the project directory, runs `yass`, and
  builds what the spec set describes.
- Implementing here is a new request. WHEN the user makes it: MUST work from the spec set
  through `yass query`, not from memory of this conversation — a gap that surfaces there
  is a spec defect to fix, not a gap to fill silently.

## Red flags

| Thought | Reality |
|---|---|
| "The request already says what they want" | It says the purpose. It rarely says the edges. Run stage 3; skip only what is answered. |
| "I'll pick a sensible default" | A default the user never saw is a decision the implementer remakes. Ask, with the default marked. |
| "Everything else is an error" | A residual with a named failure hiding in it. Name the failure and its outcome. |
| "The guards cover everything; add a catch-all to be safe" | Dead residual. State exhaustiveness instead. |
| "This context needs a comment" | yass has no prose channel. Obligation, `design:` block, or nothing. |
| "Scope is approved, so write the files and start" | Each stage gets its own yes. |
| "It's a small feature; the gate is overkill" | A small feature gets a short draft and a yes. The gate is the yes, not the size. |
