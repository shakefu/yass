# Test taxonomy

This document describes the categories of tests a person, agent, or skill should
**derive** from a `.yass.yaml` specification. It is not a tool to invoke. The taxonomy
maps spec constructs to test obligations; how those obligations are expressed as
actual tests depends on the target language and test framework.

This is the `.yass.yaml` analog of Allium's `test-generation.md`. It is deliberately
smaller. Allium's taxonomy has teeth because the language declares typed entities,
variants, transitions, and surfaces — a formal domain model the checklist can walk.
The `.yass.yaml` format is a "structured envelope around prose": one spec per public
symbol, five fixed slots, RFC-2119 obligations, `WHEN` guards, and
`CONFORMS`/`USES`/`SEE` refs. Nothing verifies obligation *content*. So the
taxonomy maps the only structure we have — slot, normativity keyword, guard,
relation — to test obligations, and the agent writes the assertions.

## What "deterministic" means here

The mapping from construct to test obligation is fixed. Two agents applying this
taxonomy to the same spec should produce the **same list of things to test**. That
determinism removes "did I think to test that?" guesswork. The step from a test
obligation to working test code is **not** deterministic: it depends on the
language, framework, and how the symbol is invoked, and it is the agent's job.

## Normativity → test strength

The normativity keyword decides whether a failing test is a defect:

- **MUST** → required. Generate an assertion. A failure is a real defect.
- **MUST-NOT** → required negative. Generate an assertion that the prohibited
  behavior never occurs (absence of an effect, a value, an error).
- **SHOULD** → recommended default. Generate an assertion that the behavior
  holds in the default/common case, but document that deviation is acceptable
  under stated conditions. A failure in the default case is a defect; a failure
  under a legitimate exception is not.
- **SHOULD-NOT** → discouraged. Generate an assertion that the behavior is
  absent in the default/common case, but document that deviation is acceptable
  under stated conditions. Same test-strength semantics as `SHOULD`, inverted.
- **MAY** → optional. Do **not** assert the behavior always happens. Generate
  tests that **both** the present and the absent forms are *acceptable* (two
  cases), or record a single "permitted, not required" note. Never let a `MAY`
  produce a failing test for the absent case.

## `WHEN` guards → arrange + branch

`WHEN` states a sufficient condition under which the paired normativity obligation
applies. Treat it as the test's **arrange** step.

- Each distinct `WHEN` guard in a slot → at least one scenario where that condition
  holds and the paired obligation is asserted.
- Where the guard is effectively boolean (a thing is/is not present, matches/does
  not match), also generate the **complementary** branch to confirm the obligation
  does *not* fire when the guard is false. Specs often pair the two explicitly
  (e.g. `IsRefFormat`: one `WHEN match → true`, one `WHEN not-match → false`); when
  only one branch is written, the complement is still a test obligation.
- An obligation with no `WHEN` applies unconditionally — test it for representative
  valid inputs.

## Per slot

A spec is one public symbol. Walk its slots. Each slot's obligations map
to test obligations as follows.

### `INPUT`

Obligations describe the inputs the symbol must accept and the constraints on them.

- **MUST** → verify the symbol accepts and correctly handles input meeting this
  obligation (the positive path). For "take a ref string of the form X", supply a
  well-formed X and assert it is accepted.
- **MUST-NOT** → verify the symbol rejects or refuses input that violates the
  obligation (the guard path).
- **MAY** → verify the symbol works both when the optional input is supplied and
  when it is omitted.
- A `USES` ref on an input obligation (e.g. `take a parsed spec document` /
  `USES ./other@ParseSpecFile::RETURN`) names the **shape** of the input. Construct
  the input via that referenced producer where practical, so the input under test
  is a real product of the upstream symbol, not a hand-built fake.

### `RETURN`

Obligations are postconditions on the result for valid inputs.

- **MUST** → assert the returned value has this property after a valid call.
  "return an ordered list of spec documents" → call with a valid file, assert the
  result is a list, ordered as the input documents were.
- **MUST-NOT** → assert the returned value never has the prohibited property.
- **MAY** → assert that a result with, and a result without, the property are both
  valid; do not require it.
- **WHEN ... MUST** → conditional postcondition. Arrange the guard, assert the
  obligation. Add the complementary branch where the guard is boolean.

### `ERROR`

Each obligation is a failure-path test. This is the highest-value slot — error
obligations enumerate exactly the failure modes worth testing.

- **MUST (ERROR)** → arrange the triggering condition, assert the symbol produces
  the specified error/failure (one test per error obligation). "error if the file
  does not exist" → call with a missing path, assert it errors.
- **WHEN ... MUST** → the guard names the triggering condition; arrange it, assert
  the error.
- **MUST-NOT (ERROR)** → assert the symbol does *not* fail in this way under the
  named condition (negative error test).
- Generate one failure test per independent error obligation, so a regression in
  any single failure mode is caught in isolation.

### `SIDE-EFFECT`

Obligations describe observable effects beyond the return value.

- **MUST (SIDE-EFFECT)** → assert the effect is observable after the call (a write
  happened, a message was emitted, a downstream call was made, state mutated).
- **MUST-NOT (SIDE-EFFECT)** → assert the effect is **absent**. "MUST-NOT touch the
  filesystem" → run under a sandbox/spy and assert zero filesystem I/O.
  "MUST-NOT modify the file" → assert the file's content/mtime is unchanged.
  These are the most commonly skipped and most valuable assertions; always
  generate them.
- **WHEN / MAY** → as above (conditional / optional effect).

### `INVARIANT`

Obligations are constraints that always hold, independent of any single call.
Because they are not tied to one invocation, derive property-style tests that
assert the constraint holds across the relevant operations.

- **MUST (INVARIANT)** → generate a property test (or a representative set of
  cases) asserting the constraint holds before and after every operation the spec
  describes. "MUST-NOT share its name with another Spec in the same file" →
  assert that after any sequence of valid operations the name remains unique.
- **MUST-NOT (INVARIANT)** → assert that the prohibited state is never reachable
  via any operation covered by the spec.
- **CONFORMS ref on INVARIANT** → the inlined obligations from the referenced slot
  are invariants too; apply the same property-test derivation to each.
- Use the `INVARIANT` slot only when no other slot is suitable; if the constraint
  can be expressed as a `RETURN` postcondition or a `SIDE-EFFECT` absence, prefer
  those slots.

## Relations

- **`CONFORMS ./other@Spec::SLOT`** → contract obligation. The symbol must also
  satisfy the referenced obligations. Because `query` inlines `CONFORMS` one level,
  the referenced slot's obligations expand into this slot; apply the per-slot rules
  above to each **inlined** obligation as if it were local. The provenance comment
  (`# CONFORMS: ./other@Spec::SLOT`) marks which tests came from the contract. A spec that
  conforms to another must pass the other's tests too.
- **`USES ./other@Spec::SLOT`** → composition obligation. The symbol's behavior is
  composed from the referenced spec. Lower priority than `CONFORMS`: generate an
  integration test that exercises the composed path, and ensure the input/output at
  the seam is consistent with the referenced contract. Do not duplicate the
  referenced symbol's own tests here.
- **`SEE`** → no test obligation. It is a link for human/agent context only.
- **Ref-only obligation** (a relation with no normativity keyword and no `WHEN`):
  - `CONFORMS` or `USES` → expand the target and apply the taxonomy to the
    expanded obligations. (`USES` inlining is optional in general tooling, but
    for test derivation, expanding it ensures integration coverage at the seam.)
  - `SEE` → no test obligation.

## Coverage model

There is no typed-construct coverage as in Allium. Coverage is defined over
obligations:

- Every **MUST** and **MUST-NOT** obligation in every slot has at least one
  corresponding test.
- Every **SHOULD** and **SHOULD-NOT** obligation has at least one corresponding
  test covering the default/common case.
- Every distinct **WHEN** guard is exercised (and its complement, where boolean).
- Every **`CONFORMS`** ref is resolved and its obligations covered as inlined
  obligations.
- **MAY** obligations are tracked as optional; "uncovered MAY" is not a coverage
  gap.

Report coverage as: obligations covered / required obligations (MUST + MUST-NOT +
SHOULD + SHOULD-NOT + guarded variants), with MAY and `SEE` listed separately as
non-required.

## Constructs without test obligations

- Preamble `description` and `related` — context only.
- `SEE` relations — links only.
- `MAY` obligations with no observable difference between present and absent — note
  as permitted, generate nothing.
- A spec whose only content is `USES`/`SEE` pointers with no local normativity —
  its tests live with the referenced symbols.
