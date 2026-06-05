# Spec Language — Future Improvements & Open Questions

Everything deliberately deferred or left undecided. Nothing here is required for v1
(see DECISIONS.md). Grouped by area.

## Drift detection (content hashing)

- Each referenceable slot gets a **content hash**; an inline ref records the hash it
  was authored against; tooling flags every dependent when the target's hash diverges.
  The main payoff of typed refs — catch silent drift.
- **Hashes belong in a generated index, never inline in source.** Inline `@hash`
  (`./exampleA@ExampleA::RETURN@<hash>`) was rejected: it puts hashes on the
  hand-edited surface and creates churn / diff noise.
- **Decision:** implement when/if specs are indexed. Until then, no drift tracking.

## Generated project-level index

- A regenerated (never hand-maintained) index for fast cross-file lookup.
- Also the natural home for content hashes.
- Per-file `specs:`/`refs:` footers were rejected — derivable, and drift instantly if
  hand-maintained.

## Multi-target refs

- Today a relation key (`CONFORMS`/`USES`/`SEE`) takes a **single** ref string; to
  reference two targets you split the obligation. A future option is **list-valued**
  relation keys:
  ```yaml
  USES:
  - ./other@SpecA::RETURN
  - ./other@SpecB::RETURN
  ```
  Deferred — single-string keeps the obligation model simplest, and splitting reads
  fine for now.

## Retrieval depth & batching

- **Batch retrieval:** request multiple refs in one call. Considered higher-value
  than depth because the agent already sees all refs in a slot and can manage its own
  context budget. Deferred — agents may not even need it.
- **Min/max retrieval depth:** caller-tunable depth to cut round trips.
- **Per-relation default depth** (instead of a global knob):
  - `CONFORMS` → default depth 1 (eager one hop)
  - `USES` → default depth 0 (lazy)
  - `SEE` → depth 0 always
- **Open fork:** when batch/depth resolves more than the explicit list, should
  relation type drive a default extra hop, or resolve exactly what was asked?

## Transitive resolution, cycles, dedup

- Currently `CONFORMS` resolves exactly one level. Going deeper requires:
  - **Cycle detection** (`A::RETURN USES B`, `B USES A` would loop).
  - **Dedup** across a multi-hop / batch result set.

## Resolution of `USES` and `SEE`

- v1 only inlines `CONFORMS`. Decide later whether/when `USES` also inlines on
  `query`, and whether `SEE` ever surfaces more than a pointer.
- Transclusion provenance for `USES` once it resolves (same comment mechanism).

## MCP mode (same binary, stdio)

- **Not a separate build target and not a network server** — an `mcp` subcommand on
  the same binary, speaking MCP's stdio JSON-RPC transport, exposing
  `query`/`validate`/`list` as MCP tools. It shares the resolver
  (`ParseSpecFile`/`ResolveRef`/`InlineConforms`) with the other subcommands, so the
  earlier "MCP wraps the resolver later" framing is really "add an `mcp` subcommand."
- **Stateless by design:** every invocation reads the filesystem fresh — no session,
  cache, or long-lived process to justify a server.
- **Open:** the exact tool input/output contract — what an agent passes, and the
  precise return shape (including whether the return carries onward refs as
  unresolved pointers so the agent can decide next hops with full information).

## Necessary / sufficient

- Guards (`WHEN`) express sufficient conditions only.
- **Necessary-condition forms** (`ONLY WHEN` / `UNLESS`) deferred until a real spec
  forces the distinction.
- General note: RFC 2119 covers the *normativity* axis (how binding). Necessary vs
  sufficient is the orthogonal *logical-force* axis. We intentionally do not conflate
  them into one keyword set.

## Verification (out of scope, possibly forever)

- The format is a structured envelope around prose: tooling **routes, retrieves, and
  drift-tracks** obligations; it does **not verify** their content.
- A `COMPATIBLE` relation was considered and **excluded**: verifying "compatible with
  the return value of X" requires understanding the implementation language's type
  system, which would break the "generic, lightweight, language-agnostic" constraint.
  Mechanical checking would be a separate verifier / model-checker / SMT project.
- If verification is ever wanted, the `CONFORMS` ("must match") vs `USES` ("composed
  from") distinction already gives a downstream checker the intent it needs.

## Addressing granularity

- The **slot** is the finest addressable unit. Referencing an individual obligation
  within a slot (`...::RETURN#0` or named obligation anchors) was considered and **not**
  adopted; revisit only if reuse needs it.

## Literal values in obligations

- Literals (e.g. "concatenate inputs joined by `,`") are handled by quoting for now.
  Machine-extractable literal syntax would push toward a formal language — deferred.

## Name matching

- v1 `query` name lookup is a single grep: `^spec: [^ ]*NAME$`. The `[^ ]*` prefix
  lets a bare name (`MyMethod`) match a namespaced spec name (`myobject.MyMethod`),
  while the `^spec: ` anchor and `$` keep prose and partial names from matching.
- **Deferred to indexing:** richer matching — qualified-name resolution, fuzzy /
  substring matching, disambiguation by namespace — once specs are indexed and a
  grep-per-query is no longer the lookup mechanism.

## Misc

- **LSP integration:** an agent finds a symbol via LSP in code, then cross-references
  to its spec via `query`.
- **YAML footguns to lint for:**
  - The Norway problem (`no`/`yes`/`on`/`off` → boolean) — keep keys/values quoted
    where ambiguous.
- **Host-file header:** dropped from emitted fragments; listed here only as a
  decided-against item in case retrieval needs ever revive the question.
