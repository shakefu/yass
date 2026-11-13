# yass — Implementing and Testing a Spec

*Implement path: **implementing**, alone. Run it as `yass docs implementing` and read it whole; reach for `yass docs obligations` or `yass docs references` only to settle a question of notation.*

How to turn an approved spec set into code and tests. Authoring has its own review
(*Case coverage* and *Implementer view* in `yass docs coverage`): that pass asks whether
the set is complete. This document starts after it, and asks what the set obliges the
code to do and how a test shows the code does it.

## Read the set through the CLI

- `yass` with no arguments orients you: the project, the notation, the commands.
- `yass list` names every document. `yass query path@Name` (the REF `yass list` prints,
  such as `root@Name`) gives one spec as its implementer holds it, and
  `yass query path@Name::SLOT` gives one slot. A target with no `@` names a whole file.
  Query the spec you are implementing rather than reading its whole file.
  `yass list --filter GLOB` narrows the index; GLOB matches the whole REF, so `*`
  crosses `@`. `--filter 'root*'` matches
  `root` and `root@Name`; `--filter root` matches only the file record.
- `yass refs path@Name` shows what a spec points at and what points at it: the specs your
  code must agree with, and the specs whose code depends on yours.
- Work from the queried fragments, never from memory of the conversation that produced
  the set. A gap you meet there is a spec defect; report it rather than fill it.

## Read the references as resolution does

A queried fragment is already resolved one level deep. Each relation asks something
different of the code:

- **Slot-targeted `CONFORMS`** (`path@Name::SLOT`). The target slot's obligations are
  spliced in front of the carrier, each under a `# CONFORMS:` comment, with the
  carrier's guard conjoined into its own. They bind your code exactly as the spec's own
  obligations do.
- **Whole-spec `CONFORMS`** (`path@Name`, no slot). Nothing is inlined. Your code must
  meet the whole target spec: query it and read it as part of this contract.
- **`USES`**. It targets a design block only, and resolution appends that block after
  the spec. The design binds the mechanism: follow it. A guard on the carrier scopes
  when the design binds.
- **`SEE`**. It binds nothing beyond the obligation's own prose. Follow it for context.
- An inserted obligation's own references are not resolved again. Query their targets
  yourself when you need them.

## Read each obligation for what it obliges

- **`MUST` and `MUST-NOT`** are the contract. Each one gets code and a test.
- **`SHOULD` and `SHOULD-NOT`** bind unless you have a reason to depart. Record the
  reason where the project keeps such decisions.
- **`MAY`** is a permission. Either choice conforms. Test that the choice you made does
  not break another obligation, not that you made it.
- **Anything no spec states** is your discretion. Choose deliberately, and do not test
  your choice as though it were contract, so the next implementer may choose otherwise.
- **Exact text is contract only where the spec quotes it.** A quoted message, flag, code,
  or format is byte-for-byte: never reword it. Prose that describes an outcome without
  quoting it leaves the wording to you.

## A guard is sufficient, never necessary

`WHEN` states a sufficient condition. When it holds, the obligation applies. When it
does not, the guard says nothing: another obligation may cover that case, or it is your
discretion.

- Do not infer a prohibition from a false guard. "WHEN exactly one item matches, MUST
  emit a fragment" does not forbid emitting a fragment when two match.
- An "only when" reading needs its own obligation: a `MUST-NOT` under the complement
  guard, or an unguarded baseline. If the set means "only when" and states no such
  obligation, report the gap.
- Test each guard where it holds. Test its false side only where another obligation
  governs that side, and against that obligation.

## Turn obligations into tests

Write roughly one test per obligation, named so a reader can find the obligation it
covers (the spec, slot, and a few words of the prose). One test may cover several
obligations when one input exercises them all; one obligation may need several tests
when its guard has several cases.

- **Start with `ERROR`.** Write the foreseeable failures first: each guarded `ERROR`
  obligation names an input and the outcome it must produce (a code, a message, an exit
  status). These are the cases an implementation most often skips.
- **Test the residual with an input no guard matches.** A guard-less `ERROR` obligation
  is the policy for every failure no guarded obligation matched. If you cannot construct
  such an input within the declared contract, the residual is dead: report it as a spec
  defect rather than write a test that cannot fail.
- **Derive negative tests from `MUST-NOT`.** Assert the absence the obligation names: no
  output line for a rejected input, no file written, no second record, no retry. Make
  the absence observable at the component's boundary: standard output and standard
  error, the returned value, the files or calls it makes.
- **Test outcomes, not mechanism.** Assert what is observably true once the work is
  done. Assert a sequence or an internal step only when a `USES` design binds it.
- **Test a policy at the component's own boundary.** A failure another component
  prevents in the composed program is shadowed, not dead: the obligation stays, and you
  test it by calling the component directly with the input the other component would
  have stopped. An end-to-end input that bypasses upstream validation is not required.
- **Test trusted inputs on the violation side as the spec states it.** When an `INPUT`
  obligation relies on a guarantee from a producing spec, the spec states what happens
  when that guarantee is violated, or declares it unspecified. Test the stated behavior
  with a violating input at the boundary. Write no test for behavior the spec declares
  unspecified.
- **Pin exact text where the spec pins it.** Compare a quoted message or format
  byte-for-byte, including spacing and case. Do not pin text the spec leaves to you.
- **Cover every case a closed set implies.** An `INPUT` that dispatches on a subcommand,
  mode, or enum states the behavior for an unknown or missing value; test that value as
  well as each member.

## Prove it against the real system

A set can be consistent on paper and still impossible to meet against the system it
targets: a database, an operating system, a protocol, a YAML parser. Reading the spec
will not show this; running the code against the real dependency will.

- Run the tests against the genuine dependency wherever the obligation is about it.
  A mock tests your assumptions about the dependency, not the dependency.
- Trust an observed run over secondhand signals such as editor warnings, cached
  analysis, or review alone.
- When the real system cannot meet an obligation, that is a spec defect. Report the
  obligation and what the system does instead.

## When the spec and the code disagree

- **The code is wrong** when the approved contract already decides the case. Fix the
  code and add the test that would have caught it.
- **The contract should change** when the behavior the user wants differs from what the
  spec states. That is a request to revise and re-approve the spec before the code
  changes; never rewrite a spec to match a bug.
- **The spec is defective** when implementing it requires a guess: an undefined value, a
  policy named but never stated, a guard with two readings, two obligations that
  contradict. Report the affected obligations and stop that part of the work.

A spec change that rewords without changing behavior needs no new test. A change to what
an obligation requires needs the test that shows the code now meets it.

---

End of the implement path.
