# yass — Authoring Guidance (holding doc)

Steerage for AI agents (and humans) writing yass specs. This is a **holding doc**: the
rules here emerge from trial-and-error using the specs, and their final home is TBD —
some may become language-level meta-rules, others may stay as guidance. Nothing here is
settled; it exists so the guidance is not lost while we figure out where it belongs.

## Granularity: keep specs small and located

The core failure mode: an LLM, unguided, will slop a single giant spec file for a whole
codebase. To prevent that:

- **One spec file per code file.** A `.yass.yaml` is paired 1:1 with the code file it
  describes.
- **One spec definition per public symbol or endpoint.** Each public function, method,
  class, or endpoint gets its own `spec:` document.

Candidate phrasing as a *future* meta-rule (deliberately NOT in the self-defining
`yass.yass.yaml` — the generalized language definition is not the place for code-authoring
steerage):

    WHEN: the spec defines code behavior
    MUST: maintain one spec file per code file, and one spec definition per public symbol

## Deliberate non-goal: no free-prose channel

yass intentionally provides **no formal way to attach non-spec prose or commentary** to
a spec. Authors get structured obligations only; the sole free-text field is the
preamble `description`.

Rationale: when a language allows free prose (as Allium does), the prose goes out of
control — specs bloat with narrative that isn't behavior. Withholding a prose/comment
channel forces authors (especially LLMs) to express intent as obligations, or not at
all.

Note: the only comments yass emits are tooling-generated provenance comments
(`# CONFORMS: ...`) on resolved fragments. Those are not an author-facing channel, and
should not become one.

## Ordering: implementation sequence

Spec document fragments (`spec:` documents) within a file SHOULD be ordered in the
sequence they should be implemented. This gives an implementing agent (or human) a
natural top-to-bottom work order and makes dependency ordering explicit without extra
metadata.

Two levels of ordering are at play:

- **Inter-file ordering** — across spec files. Enforceable today with numeric-prefix
  naming conventions (e.g. `00-init.yass.yaml`, `01-core.yass.yaml`,
  `02-api.yass.yaml`).
- **Intra-file ordering** — within a single spec file. The YAML document stream is
  ordered; spec documents earlier in the file should be implemented before later ones.

Candidate phrasing (not yet a meta-rule):

    WHEN: the spec defines code behavior
    SHOULD: order spec documents in implementation sequence, both across files
            (via naming convention) and within files (via document position)

Note: this is emergent guidance from early usage — not yet formalized. A second pass on
spec revisions may promote it to a meta-rule or refine the conventions. Captured here so
it isn't lost.

## Open: how a skill uses the test taxonomy

Moved here from TEST-TAXONOMY.md — depends on tooling (CLI commands, obligation-JSON
projection) that is not yet spec'd. Revisit once the CLI is scoped.

1. Retrieve the spec (with `CONFORMS` inlined and provenance attached) via whatever
   tooling exists.
2. For each slot, for each obligation, look up its row in the test taxonomy (by slot +
   normativity + presence of `WHEN`/relation) to get the fixed test obligation(s).
3. Emit the obligation list (this is the deterministic artifact) before writing any
   code, so it can be reviewed against the spec.
4. Locate the implementing symbol in the paired code file (the meta-rule: every
   code file is paired with a `.yass.yaml`); reuse existing test infrastructure.
5. Write tests in the target framework, one mapped to each obligation, labeled with
   the originating `spec`/`slot`/obligation so failures point back to the spec.
6. Report coverage per the model above.

## Open: emergent guidance

As the specs get used, more steerage will surface (placeholder conventions, naming,
when to split a spec, etc.). Collect it here until it earns a permanent home — either
as a language meta-rule or as stable authoring documentation.
