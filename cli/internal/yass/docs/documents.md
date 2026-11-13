# yass — Files and Documents

*Write path, 2 of 5: authoring → **documents** → obligations → references → coverage. Run each as `yass docs NAME` and read it whole.*

**yass** — *Yet Another Spec Syntax*. A YAML-based specification language for AI-agent
development: agents (or humans) write specs that capture desired behavior, and tooling
consumes them to drive test (TDD) and implementation generation.

This part covers the file, the project root, and the three document kinds: preamble, spec, and design.

## File

- A yass file uses the **`.yass.yaml`** extension — with at least one character
  before it, so a file named exactly `.yass.yaml` is not a yass file — and is a
  **YAML multi-document stream** (`---`-separated).
- Read as UTF-8 under the YAML 1.2 core schema, so `yes`, `no`, `on`, and `off` are
  plain strings. Invalid UTF-8, a leading BOM, YAML anchors, aliases, or explicit tags,
  and a duplicate key in any mapping are each a validation error.
- SHOULD carry the schema modeline as its first line, so editors validate against the
  JSON Schema with no per-repo config:
  `# yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json`
- **MUST NOT carry an undecided point** anywhere, comments included: no question left
  for a later reader, no `TBD`/`TODO` placeholder, no choice deferred to the
  implementer. A spec is a contract; its implementer builds an undecided point as a
  guess. `yass lint` reports the markers it can see as `yass.lint.open_question`.

## Project root (`root.yass.yaml`)

- A file named exactly **`root.yass.yaml`** is **required**, and its **presence defines
  the project root**: the project root is the directory that contains it. Every
  root-relative ref path — a path token that does not begin with `./` or `../`,
  including one such as `.config/x` whose leading dot opens a hidden name — resolves
  from there.
- **A project root is not a repository root.** It is defined by the presence of the file
  and by nothing else: the anchor is deliberately decoupled from the repository, so
  nothing ties it to a VCS checkout, a package manifest, or a build directory. A project
  root may sit at the top of a checkout, or nowhere near it.
- **Discovery is by name alone.** There is no `.git` search and no
  deepest-`.yass.yaml`-ancestor fallback; a project root is never inferred from layout.
  **Absence is an error.**
- **Layouts are arbitrary.** A repository MAY hold several projects, each with its own
  `root.yass.yaml`, and the one that governs a given file is the **nearest at or above
  it** — a nested root takes over its own subtree, and the outer root does not govern
  within it.
- **Contents:** exactly one `spec:` document, describing the **project as a whole** — what
  it is and what it must do — written with the ordinary five slots. There is **no new
  document type**; the project is a normal spec. A project need not produce an invocable
  program: it may produce a library, a data format, or a language (this repository's
  project is yass itself, which has no entry point), so read the slots structurally when
  the subject is not function-shaped (see *Slots* in `yass docs obligations`). The file MAY also carry `design:`
  documents. More than one spec, or none, is a validation error. (How to write that spec:
  see *The root file* in `yass docs authoring`.)
- **Membership is reachability.** Every spec and design under a project root MUST be
  reachable from the root spec through a **chain of references**, and `CONFORMS`, `USES`,
  and `SEE` all count as edges — reachability is a property of the reference graph, not of
  any one relation. A spec that no chain reaches is either dead or evidence that the root
  file is missing an obligation.
- Encoded in the self-definition as the `Root` construct, which `CONFORMS: Document` — a
  root file is an ordinary yass file (preamble first, `---`-separated stream) that has
  been given a fixed name and a constrained payload.

## Preamble

- **Required**, **exactly one**, and **MUST be the first** document of every file.
  Making it mandatory simplifies parsing and validation.
- Identified structurally by the **absence of a `spec:` or `design:` key**.
- The file name is the identity of the whole unit; the preamble does **not** repeat it.
- Keys, a closed set (any other key is a validation error):
  - `description` — **required**; a string file-level summary (block scalar `>`
    allowed). Surfaced by tooling that summarizes files.
  - `version` — **required**; the yass language version the file targets. **Pinned at
    `v1`** in the current revision.
  - `related` — *optional*; a list of freeform external resource locations (paths, URLs,
    issue links). A distinct channel from the `SEE` relation; **never resolved** by
    tooling.

## Spec document

- A `spec:` key naming the spec (may be namespaced, e.g. `pkg.Symbol`) plus zero or
  more **slots**. A document carries exactly one of `spec:` or `design:` — carrying
  both is a validation error.
- **Name charset:** ASCII letters and digits, `.`, `_`, and `-` (so a slot-keyword
  name like `Slot.SIDE-EFFECT` is legal and addressable). The composition regex
  `^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$` also rejects a leading dot, a trailing dot,
  and consecutive dots.
- Names **MUST NOT** case-insensitively match any yass keyword — a slot name, a
  normativity keyword, `WHEN`, or a relation (`CONFORMS`, `USES`, `SEE`).
- Names **MUST be unique within a file**, so refs resolve unambiguously.
- Granularity: **one public symbol / endpoint → one spec.** Also the unit of retrieval.
- The five slots are themselves defined in the language definition (`yass docs
  language`) as `Slot.INPUT`, `Slot.RETURN`, `Slot.ERROR`, `Slot.SIDE-EFFECT`, and
  `Slot.INVARIANT`.

## Design document

- A `design:` key naming the block, plus **required** `type` and `content` keys, a
  `format` key when and only when `type` is `schema`, and no other key — **no slots**.
  A peer of `spec:` in the YAML stream:

  ```yaml
  ---
  design: StartupSequence
  type: procedure
  content: |
    1. Warm the cache
    2. Connect the database
    3. Mark the service live
  ```

- Names follow the spec name grammar and keyword ban, and share the **file-wide
  uniqueness namespace** with spec names, so a ref target stays unambiguous — a bare
  name or `path@Name` resolves to exactly one document, spec or design.
- `type` — **required**; exactly one of five values. Any other value, including a
  former name (`table`, `code`, `ordered-steps`, `stack`, `pseudocode`), is a
  validation error.
  - `constraint` — prose limits on the solution: stack, library, platform, version floor.
  - `algorithm` — pseudocode for a computation the implementer still writes.
  - `procedure` — ordered steps that span specs and have no owning symbol.
  - `schema` — the exact external shape of data: field names, a JSON Schema, an example
    payload, a wire format. Requires a `format` key naming the notation (`json-schema`,
    `yaml`, `text`, …); no other type carries `format`.
  - `catalog` — a set of uniform records: exit codes, diagnostics, log events, flags,
    an ownership matrix.
- `content` — **required**. For every type but `catalog` it is a string, written as a
  literal block scalar (`|`) by convention. The body is **opaque**: no refs are
  recognized inside it.
- **Data shape, not implementation.** A design states what an implementation must
  match, never the implementation. An `algorithm` is pseudocode, not source in a real
  language; a `schema` is the shape of data, not the steps that build it. Literal source
  is never a design: name the symbol in an obligation instead. A Markdown table is never
  content: rows belong in a `catalog`. `yass lint` reports both as
  `yass.lint.design_code` and `yass.lint.design_table`.
- **Catalogue form.** `catalog` content is a non-empty YAML mapping keyed by each record's
  scalar identifier; each record is a mapping of scalar field names to scalar values,
  and every record carries the same field names. Empty content, a repeated or non-scalar
  identifier, a record that is not a mapping, a non-scalar field name or value, or a
  differing field set is a validation error. A field holds a fact of the implementation
  alone:

  ```yaml
  ---
  design: Diagnostics
  type: catalog
  content:
    yass.io.missing: {severity: error, owner: load, exit: 3, meaning: the named file does not exist}
    yass.cli.usage:  {severity: error, owner: cli,  exit: 2, meaning: the invocation was malformed}
  ```

- **A catalogued fact lives only in its catalogue.** An obligation names the identifier
  and binds the catalogue with `USES` — "MUST report each code the catalogue assigns to
  owner `load`, at the severity and exit status the catalogue gives". It states no field
  value in any wording: not a number, not a status label, not a behaviour such as "fail
  as when the invocation is malformed". A restated value is a second source that drifts.
  `yass lint` reports an identifier named with a value as `yass.lint.restated_fact`,
  agreeing or not, and, experimentally, a catalogued value stated with no identifier as
  `yass.lint.restated_value`. A field whose values are the keys of another catalogue
  (`exit: 2`, keyed into an exit-status catalogue) is worded by that record's fields too.
- **Normative force:** an implementation bound to a design block via reference MUST
  follow it. The block is not commentary; it binds through the reference graph (the
  `USES` relation — see `yass docs references`).
- An **unreferenced design block is dead weight** — declared normative content nothing
  binds to — and is flagged by lint.

## Meta-rules (the spec *system*, distinct from in-spec obligations)

- Every unit of specification MUST be paired with its `.yass.yaml` file. A unit is the
  project's chosen pairing — a command, module, package, or similar cohesive component —
  not necessarily one source file, and a spec file may hold several `spec:` documents.
- Every public symbol or endpoint SHOULD have a definition in the `.yass.yaml` file.
- The `.yass.yaml` file MUST be structured parseably.
- The `.yass.yaml` file MUST-NOT require special syntax to be read by an AI agent.
- The `.yass.yaml` file MUST conform to the format when written, and MUST be validatable by
  tooling.

---

Next in the write path: `yass docs obligations`.
