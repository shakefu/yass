# Recommendations: Authoring Guidance & Format Changes

Distilled, actionable recommendations for writing specs and for the spec format
itself. The first half is guidance to fold into `GUIDANCE.md`; the second half is
changes to consider for the language and schema.

## Guiding principle

A specification must be fully intelligible to a reader — human or model — who has
never seen this project, with no prior prompting and no other documents open.
Tooling can make working with specs faster, but a spec must never *require* tooling
or a companion document to be understood or implemented. Every recommendation below
serves that goal: it either moves knowledge into the spec where it belongs, or
removes a reason to look outside it.

---

## Part 1 — Authoring guidance (for `GUIDANCE.md`)

### 1. The specs are the complete contract

Everything an implementer must do to build and integrate the code correctly belongs
in a spec as an obligation. That includes things which are not any single function's
"behavior": how components are wired together, the order in which they run, setup
that must be shared between them, and the data that passes between them.

If you find yourself wanting a separate design or architecture note to explain how
the pieces fit, treat that as a sign a spec is missing an obligation — write the
obligation instead. Anything no spec states is, by definition, left to the
implementer's discretion. Make that choice deliberately, not by omission.

### 2. Describe outcomes, not mechanisms

An obligation should state what is observably true once the work is done, not the
steps used to get there.

- Outcome: "The total MUST equal the sum of all line items."
- Mechanism: "Add each line item to a running total."

Mechanism-shaped obligations read as binding rules but quietly lock in one
implementation, and they are usually the ones that later have to be contradicted or
reinterpreted. When you can only express something as a series of internal steps,
decide which it is: a genuine ordered process (state it as an explicit ordered list)
or implementation detail that does not belong in the spec at all.

### 3. Give every cross-cutting concern a single home

When a rule applies across many specs — how errors are wrapped, how paths are
formatted, how operations are traced, what is written to standard error — write it
once, in one spec that owns it completely, and reference that spec from the others.
Do not restate the rule in fragments across the specs it touches.

A reader should be able to learn the whole of a concern from one place rather than
reconstructing it from scattered mentions. If a concern has several parts — naming
something, deciding where it is recorded, and what data it carries — all of those
parts live together in the owning spec.

### 4. Specify what happens for the errors you did not enumerate

Listing specific failure cases is not enough. State a default policy for every other
failure — for example, "any other error is wrapped with the operation name and
returned." Without it, each implementer invents their own handling and the results
diverge.

Two related points:

- When more than one spec performs the same check, they must handle failure the same
  way — or the difference must be stated as intentional, not left to emerge.
- Where the exact wording or shape of an error is part of the contract that other
  code or tools depend on, pin it precisely rather than leaving it to paraphrase.

### 5. Prove feasibility by running against the real thing

A specification can be perfectly self-consistent and still be impossible to satisfy
against the actual system it targets — a database, an operating system, a protocol.
Reviewing the spec will never reveal this; only running the implementation against
the real dependency will.

Treat passing tests against the genuine external system as the standard for "done."
Do not trust secondhand signals — editor warnings, cached analysis, or review alone
— over a real, observed run.

---

## Part 2 — Format changes to consider

None of these add new keywords. Each gives meaning to a shape the format already
allows, so a reader can still understand a spec without being taught new vocabulary.

### 1. Treat an ordered list as ordered and step-addressable

When an obligation's content is a list whose order matters, that order should be part
of the contract, and each step should be referenceable on its own.

Sequenced steps are among the most directly useful content a spec can carry — they
map almost one-to-one onto both code and tests — but only when their order is
understood as meaningful rather than incidental. This needs no new syntax: a list is
already ordered; we simply commit to reading it that way and to letting each item be
addressed individually.

### 2. Let an unconditional error obligation mean "for everything else"

Most error obligations apply only under a specific stated condition. Allow an error
obligation with no condition attached to mean "this is what happens for any failure
not otherwise covered."

That gives the default-error policy from Part 1 a natural home, without adding a new
slot or keyword — an error rule with no guard simply reads as the catch-all.

### 3. Let inputs and outputs name the data that crosses between specs

Allow the input and output sections of a spec to list the named pieces of data they
accept and produce, each with a plain-language description. The goal is that a reader
can see exactly what one component hands to another without opening the
implementation or a separate document.

This is intentionally just naming with description — not a type system and not full
data modeling. It should carry enough to close the gap that otherwise sends people
looking outside the spec, and no more.

---

## Considered and set aside

A dedicated way to say "use this other behavior but override part of its setup." We
are not recommending it. The cases that motivate it are better handled by the first
authoring rule: state the shared setup as an explicit obligation on the spec that
coordinates the pieces. Adding more relationship vocabulary makes every spec harder
to read cold, which works against the whole point.
