# yass — Brainstorming a Spec

How an agent turns a user's request into a spec set that another agent can implement
cold. The keywords below bind the agent running this flow, in the RFC 2119 sense.

## Why the dialogue exists

The spec set is the only thing the implementing agent will hold: not this conversation,
not this document. Every behavior the spec leaves unstated, that agent decides alone, and
two agents decide differently. Each question below moves one such decision to the user
while it is still cheap.

## Gate

- MUST-NOT write product code, scaffold, install a dependency, or create a project before
  the user approves the spec set.
- MUST-NOT write a spec file before the user confirms the written-back understanding
  (stage 2).
- Each gate takes its own yes, whatever the size of the request.
- Skip a question only when the request or the repository already answers it — never
  because the answer seems obvious.
- MAY read the repository and run `yass` commands at any stage.

## Stage 1 — Classify, out loud

State the class in one sentence before the first question, so the user can override it.

- **Fresh project**: `yass` reports `project none`. The root spec comes first, then the
  specs it reaches.
- **Feature**: a root governs the directory. MUST read `yass list`, locate what the
  feature touches with `yass find` and `yass refs`, and `yass query` only those documents;
  MUST-NOT open spec files by hand for what those commands serve. New behavior extends
  existing specs before it adds new ones.
- **Several independent subsystems** in one request: decompose first — name the pieces,
  their order, and the boundaries between them — then run this flow once per piece.

When in doubt, take the larger class. Complexity found mid-flow upgrades the class;
nothing downgrades it.

## Stage 2 — Establish intent

- MUST know, before proposing anything: what the thing is for, who or what uses it, and
  what "done" observably looks like.
- MUST ask one question per message. SHOULD offer choices with a recommended default
  marked, so the user corrects rather than composes; WHEN the choice shapes the spec — a
  wire format, a storage model, a dispatch structure — MUST, with two or three options
  and their trade-offs.
- MUST then write the understanding back in a few lines, separating what the user said
  from what was assumed.

## Stage 3 — Round the edges

- MUST read `yass docs guidance` first: it owns the edge cases this stage asks about, and
  this document does not restate them. Section names below are its.
- Interview by slot, for each spec the set will hold, one slot per message: propose every
  answer with a marked default and let the user correct in one reply. Every answer becomes
  an obligation.
  - **INPUT** — the forms accepted; for a closed set, the out-of-set and missing cases
    (*Closed-set dispatch*); for split input, every boundary (*Input segmentation*).
  - **RETURN** — what is yielded, in its exact shape: format, ordering, numbering, and
    what an empty result looks like.
  - **ERROR** — each foreseeable failure with its observable outcome, then the residual
    or the exhaustiveness that replaces it (*Error obligations*).
  - **SIDE-EFFECT** — what changes outside the return, and what MUST-NOT change.
  - **INVARIANT** — only what fits none of the above.
  - **Boundaries** — WHEN one spec consumes what another produces: what is trusted, what
    is re-checked, and what happens on violation (*Composition*).
  - **Constraints that are not behavior** — stack, algorithm, startup order: a `design:`
    block bound with `USES` (*The root file*).

## Stage 4 — Draft

- MUST lay files out as guidance's *Granularity*, *The root file*, and *Ordering* say;
  `yass docs reference` settles a construct in doubt.
- MUST write only obligations: nothing from the conversation survives except inside a
  `design:` block something `USES`.
- MUST run `yass validate` and `yass lint` and fix every finding before showing a draft.

## Stage 5 — Review

- MUST show the user each spec document and ask after each whether it is right. A short
  file is shown whole; a long one, spec by spec.
- Before showing, MUST check what the tools cannot: an obligation with two readings, two
  that contradict, a residual *Error obligations* forbids. Fix, then show.
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
| "The request already says what they want" | It says the purpose. It rarely says the edges. |
| "I'll pick a sensible default" | A default the user never saw is a decision the implementer remakes. Propose it, marked. |
| "It's a small feature; the gate is overkill" | The gate is the yes, not the size. |
