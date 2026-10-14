# Developing with agents

Yass lets an authoring conversation produce a contract that a fresh agent can
implement without that conversation. The CLI reads and checks the contract;
it does not call a model, write specs, or generate product code. Use the prompts
below with an agent that can run commands in your project directory.

Replace bracketed text before copying a prompt. Give each agent the repository
location and access to the CLI. Start a fresh session for implementation and
independent review: the spec set must carry the decisions those agents need.

The [brainstorming flow](brainstorming.md) owns the authoring process and its
approval gates. The [authoring guidance](guidance.md) owns coverage and structure;
the [reference](reference.md) owns language syntax. The prompts below ask agents
to read those sources rather than substitute a second authoring procedure.

## 1. Explore an idea

**Provide:** the project directory, the problem, intended users, and any fixed
constraints. A new project may have no `root.yass.yaml` yet.

```text
Work in [project directory]. I want to explore [problem or idea] for [users].
The fixed constraints are [constraints, or none yet].

Run yass to orient yourself and read yass docs brainstorming. Classify this
request out loud using that flow. Read existing code and use yass list, find,
refs, and query when an existing spec project governs the work.

Establish purpose, users, and observable success. Ask one unresolved decision
per message, proposing marked defaults and alternatives with trade-offs where
the flow requires them. Write back what you understood, separating my stated
requirements from assumptions. Stop for my confirmation before writing specs.
Do not write product code, scaffold a project, or install dependencies.
```

**Expected artifact:** a short written-back understanding and any open
decisions, in the conversation. **Human decision:** confirm or correct that
understanding. This confirmation permits drafting specs, not implementation.

## 2. Specify the behavior

**Provide:** the confirmed understanding and fixed constraints. If this is a
new session, paste the understanding and record that you have confirmed it.
For an existing feature, name the behavior to change and any compatibility
requirements.

```text
Work in [project directory]. I confirmed this understanding:
[confirmed purpose, users, observable success, scope, and constraints].

Run yass and read yass docs brainstorming, guidance, and reference. Follow
the brainstorming flow from this confirmed understanding. Check existing
specs through the CLI before extending them. Resolve remaining decisions
with me; do not invent missing requirements.

Draft a spec set with a root that explains the project and follow the
guidance's Granularity rules for code-file pairing and public-symbol coverage. Keep shared concerns
in one home and bind nonbehavioral constraints through USES design blocks.
Apply Case coverage and the Implementer view rehearsal, following references
that query does not inline. Run yass validate and yass lint and fix findings.
Show every spec document for my review. Stop for approval of the complete
spec set; do not implement here.
```

**Expected artifact:** reviewable `.yass.yaml` files, successful validation and
lint results, and the root location. **Human decision:** approve the set or
request corrections. Corrections are checked again before approval. The flow
requires its own yes at each gate, even for a small request.

## 3. Review a spec critically

**Provide:** the proposed spec set, its root directory, the requested scope,
and any requirements it is meant to satisfy. Use an independent session so
the reviewer has to locate behavior in the contract.

```text
Review the proposed spec set in [project directory] against [requirements].
This is a review request; do not edit files or implement code.

Run yass, read yass docs guidance and reference, and discover the set with
list, find, refs, and query. Run validate and lint. Apply all five Case
coverage checks, then rehearse implementation from each queried fragment
and the whole-spec CONFORMS and SEE references it leaves visible.

Look for conflicting obligations, two reasonable readings, unstated output
shapes, missing defaults, unspecified boundaries, and failures that cannot
occur within their owning component's input contract. Distinguish dead
residual errors from failures shadowed by another component's guarantee.

Return concise findings with the affected spec or slot, file location,
the concrete case exposing the gap, and the decision needed to resolve it.
Separate tool findings from semantic findings. If there are no findings,
state what you checked and any limits of the review.
```

**Expected artifact:** actionable findings or a bounded clean review.
Validation and lint check language and graph hygiene; they do not establish
that the contract expresses the intended behavior. **Human decision:** choose
the behavior for each gap and approve the revised set after it is rechecked.

## 4. Implement an approved set

**Provide:** the directory containing the approved root, approval status, the
implementation scope, and repository development instructions. Do not supply
the authoring transcript as an extra source of behavior.

```text
Implement the approved spec set in [project directory], scoped to [scope].
The set has been approved; this is the request to implement it.

Start by running yass. Discover the project with list, find, and refs, then
read behavior through query. Follow whole-spec CONFORMS and SEE references
and the USES constraints. Treat the set as the source of requirements, not
earlier conversation or plausible defaults.

Follow repository development instructions. Plan implementation in the
specs' dependency order, implement the scoped behavior, and verify it with
tests tied to observable obligations, including applicable boundary and
failure cases. If implementation requires guessing, report the affected
spec and stop that work for a spec decision rather than filling the gap.
Report changed files, verification results, and any remaining limitations.
```

**Expected artifact:** implementation and verification evidence traceable to
the approved obligations. **Human decision:** resolve any spec defect before
dependent work proceeds, then review the implementation. Implementing code
against an existing set needs the notation summary from `yass`; the authoring
documents are needed when the contract itself must change.

## 5. Maintain behavior and specs together

**Provide:** the bug or change request, compatibility constraints, and known
reproduction steps. Distinguish a bug against the current contract from a
request to change the contract.

```text
Investigate [bug or requested change] in [project directory]. Reproduction
steps and compatibility constraints are [details].

Run yass. Locate the governing specs with list, find, refs, and query, and
compare the observed code behavior with their obligations. Report whether
this is an implementation defect, a requested contract change, or a spec
gap. Check incoming references for consumers affected by the change.

If the approved contract already determines the fix, implement and test
the correction under the repository instructions. If requirements change
or are missing, read yass docs brainstorming and guidance and use the
feature authoring flow: present the understanding, draft after confirmation,
check and review the set, and stop for spec approval and handoff. Do not
silently rewrite the contract to match a bug or implement an unapproved
contract change.
```

**Expected artifact:** either a tested fix against the existing contract or a
reviewable spec change with its affected consumers. **Human decision:** settle
new behavior and approve revised specs; request implementation as the next
step after the authoring handoff.

## 6. Adopt Yass in an existing codebase

Start with a bounded component whose behavior matters: one formatter, parser,
public function, or endpoint. A project root can govern that component's
directory; it need not coincide with the repository root. A nested root takes
over its subtree, so choose that boundary deliberately.

**Provide:** the component boundary, existing code and tests, and known
compatibility expectations. Existing code supplies answers for a retrofit;
the agent asks you where it is ambiguous, contradictory, silent, or apparently
accidental.

```text
Adopt Yass for [bounded component] in [repository directory]. Its code and
tests are [paths]. Preserve [compatibility expectations]. The scope is this
component; do not claim to specify the rest of the repository.

Run yass and read yass docs brainstorming, guidance, and reference. Treat
this as a retrofit. Determine the appropriate project root and inspect
existing specs through the CLI. Use code and tests to establish intent and
edge behavior. Ask me only where the evidence is ambiguous, contradictory,
silent, or apparently accidental, including whether to pin or correct it.

Draft the scoped contract from that evidence; for a retrofit, the draft is
the written-back understanding. Follow the remaining brainstorming stages,
including validate, lint, Case coverage, Implementer view, and review of
every document. Identify behavior the draft preserves and decisions still
needed. Stop for spec approval and handoff without changing product code.
```

**Expected artifact:** a small spec project describing the chosen component,
with explicit unresolved decisions where evidence was insufficient.
**Human decision:** approve what the contract preserves and any intentional
corrections. A subsequent implementation request can then align code with the
approved contract.

## A small end-to-end workflow

Suppose you want a library function `formatLabel` that formats an item count.
Use the idea prompt with: “Return `1 item` for one and `<count> items` otherwise;
accept nonnegative integers; do not perform I/O.” The author still needs to
settle the accepted integer range, the exact failure outcome for invalid
inputs, and any technology constraints that the request does not answer.

After you confirm the understanding, the author drafts `root.yass.yaml`
describing the library and referring to `format-label.yass.yaml@formatLabel`.
That file describes the public function in its own spec. Its contract pins
the exact strings, the zero case, invalid inputs, and absence of side effects.
The author validates, lints, and rehearses both queried specs before presenting
them. An independent reviewer uses the critical-review prompt to check that
the cases can be implemented without guessing.

For example, suppose you explicitly approve these remaining choices: the
function accepts integers, accepts the range zero through 2,147,483,647 as
valid counts, and returns a two-field result rather than throwing an error.
Inputs outside the integer contract have explicitly unspecified behavior.
These are hypothetical user decisions, not defaults for an agent to invent.
The reviewable artifact could be:

`root.yass.yaml`:

```yaml
# yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json
description: A library for formatting integer item counts.
version: v1
---
spec: LabelLibrary
INPUT:
  - MUST: accept calls to the public formatLabel function
RETURN:
  - MUST: provide formatLabel with the behavior defined by its function spec
    SEE: format-label@formatLabel
SIDE-EFFECT:
  - MUST-NOT: perform I/O or change external state
```

`format-label.yass.yaml`, paired with the code file for `formatLabel`:

```yaml
# yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json
description: The public item-count formatting function.
version: v1
---
spec: formatLabel
INPUT:
  - MUST: accept one integer count
  - WHEN: the supplied value is not an integer or the argument is missing
    MAY: behave in an unspecified manner outside the input contract
RETURN:
  - MUST: return a result with exactly the fields label and error
  - WHEN: count is an integer from 0 through 2147483647 inclusive
    MUST: return a string label and a null error
  - WHEN: count equals 1
    MUST: set label to the exact string "1 item"
  - WHEN: count is 0 or an integer from 2 through 2147483647 inclusive
    MUST: set label to count's base-10 digits without leading zeros followed by " items"
ERROR:
  - WHEN: count is an integer less than 0 or greater than 2147483647
    MUST: return a null label and the exact error string "invalid count"
SIDE-EFFECT:
  - MUST-NOT: perform I/O or change external state
INVARIANT:
  - MUST: treat the valid-count and invalid-count cases as exhaustive for integer inputs
```

The reviewer can now check these concrete cases against the files:

| Input | Exact result |
|---|---|
| `0` | `{label: "0 items", error: null}` |
| `1` | `{label: "1 item", error: null}` |
| `2` | `{label: "2 items", error: null}` |
| `2147483647` | `{label: "2147483647 items", error: null}` |
| `-1` or `2147483648` | `{label: null, error: "invalid count"}` |
| Missing argument or noninteger | Explicitly unspecified; no expected result |

The integer cases are exhaustive, so the spec carries no guard-less residual
`ERROR`. The unspecified cases are outside this function's integer input
contract; another component calling it would need to state its own handling
of that boundary.

Approve the final set, then start a fresh implementation session with the
implementation prompt. The implementation agent queries the set and tests
zero, one, multiple items, and the specified invalid-input cases. Acceptance
rests on those obligations and verification results, not on remembering the
example conversation.

## An incremental adoption example

A service already has a `formatLabel` utility and tests for one and several
items, but no test for zero. Scope the adoption prompt to that utility's
directory. If no root governs it, the author drafts a root describing that
small library and a paired function spec, using the implementation to answer
what zero currently does. If the code accepts negative counts accidentally,
the author presents that behavior for your decision: preserve it for
compatibility, or specify a correction.

After approval, ask an implementation agent to add contract-based coverage
and make any approved corrections. Expand adoption to the next component
only when its boundary is clear. If an existing root already governs the
utility, extend that project's reachable spec graph instead of introducing
a nested project without considering which files it would take over.
