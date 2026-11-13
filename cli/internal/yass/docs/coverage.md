# yass — Case Coverage and Review

*Write path, 5 of 5: authoring → documents → obligations → references → **coverage**. Run each as `yass docs NAME` and read it whole.*

The passes that finish a draft: account for every case it divides, then read the set as its cold implementer would.

## Case coverage: enumerate, then account for the remainder

Before considering a draft complete, identify every place it selects behavior,
divides input into cases, names failures, or relies on another component's
guarantees. Then work through each applicable check below, over the whole spec
and its binding references — a single obligation need not carry the entire
decision. `Slot.INPUT` and `Slot.ERROR` make several of these binding; the
checks are the discipline those rules generalize.

1. **Values that select behavior** (usually `INPUT`). For each command, mode,
   enum, or other closed set the spec branches on, account for every recognized
   value, an unrecognized value, and absence: if the enumerated set is
   `{tally, grade, pack}`, say what the program does with `harvest`, and with
   no command at all. Otherwise each implementer invents the out-of-set
   handling. A reader must never have to deduce the
   residual case from the enumerated ones.

2. **Conditional behavior** (any slot). For each `WHEN` guard, determine what
   constrains behavior when the condition is false. A guard states a
   *sufficient* condition only — "when exactly one item matches, emit a
   fragment" does not itself forbid emitting a fragment otherwise — so an
   existing baseline or another obligation may already cover the false case,
   and that is fine. But if the intent is "only when," say so with obligations:
   a complement-guarded `MUST-NOT`, or an unguarded baseline the guarded
   obligation overrides. Do not encode necessity in the guard's prose — `WHEN`
   cannot carry it. Where guards overlap, make the applicable obligations
   compatible, or state the precedence inside one obligation's prose: a slot's
   obligation order carries no meaning, so list position settles no conflict.

3. **Segmented input** (`INPUT`). When a spec defines how input is broken into
   units — records, lines, fields, tokens — state the boundary behavior
   completely, not just the happy path. For each level of segmentation, name:

   - the exact separator and its character class (e.g. *one ASCII space `0x20`*,
     not the vaguer "whitespace", which invites splitting on tabs and other
     Unicode spaces);
   - empty input (zero units);
   - an empty or blank unit in the interior;
   - a leading or repeated separator;
   - the mechanics of an *optional trailing terminator* — say how many trailing
     separators are absorbed (characteristically exactly one), and therefore
     what a second trailing separator, or a blank final unit, denotes. "MAY
     accept a trailing newline" without the count leaves an implementer to
     guess whether one or all are stripped;
   - the degenerate input that is *only* separators — a lone separator with no
     content, or a run of them — which the cases above otherwise leave
     under-determined (is a lone separator empty input, or one empty unit?).

   Each of these is a case an implementer will hit and otherwise resolve by
   guessing. State the intended outcome as an obligation, or declare it out of
   scope — do not leave it to emerge.

4. **Failures** (`ERROR`). Name each foreseeable failure as a guarded obligation
   carrying its observable outcome (a specific error code, message, or exit
   status); a reader must not have to deduce a domain rule from "everything
   else." Then check whether any failures remain uncovered. If they do, state
   the residual policy in a guard-less obligation: in `ERROR`, an obligation
   with no `WHEN` is the policy for any failure no guarded obligation matched
   (the reading is fixed by the `Slot.ERROR` spec of the language definition,
   `yass docs language`). If the guarded cases are instead exhaustive (every
   input rejected by a named guard or accepted as well-formed), state that
   exhaustiveness in the spec's `INVARIANT` and omit the residual from `ERROR`.
   A residual over an empty remainder is dead and can never fire, asserting one
   alongside an exhaustive partition is a contradiction, and a guard-less
   `ERROR` obligation is itself read as the residual so exhaustiveness cannot
   live there.

   Reachability is judged within the component's declared input contract,
   not the composed program. A guarded failure another component prevents
   in this composition is **shadowed**, not dead: it stays, it names the
   spec owning the preventing guarantee or check under a relation key
   (`CONFORMS` if that contract binds this component, `SEE` to point)
   rather than restating it in prose, and *Trusted inputs* still applies
   to the violation side. A **dead**
   obligation contributes no behavior within its owning contract, and is
   removed or revised. It takes two forms: the guard-less residual over an
   empty remainder above, and a guarded obligation whose guard no input the
   declared contract admits can satisfy. The second form is a contradiction
   between the guard and `INPUT`: fix whichever is wrong. Apparent
   shadowing may instead reveal redundancy, a contradiction, or a
   mistaken reading: verify each claim. Obligation order settles nothing
   — state the shadowing explicitly, never by list position. For review
   and testing, exercise a component policy at the component boundary; an
   end-to-end input bypassing upstream validation is not required.

5. **Trusted inputs** (`INPUT`). For each guarantee from a producing spec that
   the consumer relies on without re-validating — *Composition* in `yass docs authoring` covers naming
   the producer and the trust boundary — state what the consumer does if that
   guarantee is violated, even if only to declare the behavior unspecified.
   Pointing at the producer's contract does not discharge this: the contract
   says what should arrive, not what the consumer does when it doesn't.

The finishing question: **can a reader locate the contractual disposition of
every case this pass identified?** Defined behavior, an explicitly permitted
choice, and a stated "unspecified" all count; silence does not.

## Implementer view: read the set as its implementer

The implementing agent starts cold and holds the set only through `yass query`. Before
showing a draft and again before handing the set off, the authoring agent MUST rehearse
that view: query every spec and implement from the fragment — the queried spec with
slot-targeted `CONFORMS` obligations spliced in front of their carriers
(provenance-commented, guards conjoined) and `USES` design blocks appended. Whole-spec
`CONFORMS` and `SEE` are not transcluded: they remain references out of the fragment,
and following them the way a cold reader would is part of the pass. Wherever the only
honest answer is a guess, the set is defective — fix the spec, not the implementer's
imagination.

*Case coverage* works from the author's map of decisions; the rehearsal works from the
artifact a cold implementer holds, and that vantage has gap classes of its own:

- **A policy named but never stated** — "library defaults when the policy is absent,"
  with no default anywhere in the set. Naming a policy is a promise; state the value.
- **A value used but never defined** — a named directory, a content type, a path, a
  token appearing in several fragments and defined in none.
- **A named seam with no shape** — an extension point three specs accept and none
  constructs: nothing tells the implementer what building it requires.
- **A rule gap at a boundary** — a case each side of an interface assumes the other
  owns, so no spec disposes of it: what validation rejects up front versus what only
  the call can know, and what happens then.

*Case coverage*'s own classes — a classification with no vocabulary, an uncovered
failure, an unstated trust boundary — still apply as written; the fragment view is
where their misses surface.

Read the transclusions as the consumer will hold them: a slot-targeted `CONFORMS`
inherits the producer's whole slot, so a `RETURN` mixing data guarantees with
assertions hands the consumer misleading obligations under the producer's provenance.
Confirm each guard-less residual the fragment view exposes can fire — *Case coverage*'s
**Failures** check applies to every one.

The dress rehearsal: what cannot be implemented from the fragment and the contracts it
points to cannot be implemented from the set.

---

End of the write path.
