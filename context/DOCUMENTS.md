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
- Read as UTF-8.
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
  root-relative ref path — a path token with no leading dot — resolves from there.
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
- Keys:
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
- The five slots are themselves defined in `yass.yass.yaml` as `Slot.INPUT`, `Slot.RETURN`,
  `Slot.ERROR`, `Slot.SIDE-EFFECT`, and `Slot.INVARIANT`.

## Design document

- A `design:` key naming the block, plus **required** `type` and `content` keys — and
  **no slots**. A peer of `spec:` in the YAML stream:

  ```yaml
  ---
  design: StartupSequence
  type: ordered-steps
  content: |
    1. Warm the cache
    2. Connect the database
    3. Mark the service live
  ```

- Names share the **file-wide uniqueness namespace** with spec names (same name
  grammar), so a ref target stays unambiguous — a bare name or `path@Name` resolves to
  exactly one document, spec or design.
- `type` — **required**; a brief freeform interpretation hint, one or two words
  (`pseudocode`, `ordered-steps`, `constraint`, `stack`, …), **opaque to tooling**.
  Deliberately not a closed set: tooling never branches on the value, so there is no
  out-of-set case to pin.
- `content` — **required**; a string holding the freeform body, written as a literal
  block scalar (`|`) by convention — the rule fixes the value's type, not its scalar
  style, since a YAML parser cannot distinguish styles after loading. The body is
  **opaque**: no refs are recognized inside it.
- **Normative force:** an implementation bound to a design block via reference MUST
  follow it. The block is not commentary; it binds through the reference graph (the
  `USES` relation — see `yass docs references`).
- The home for normative content that is not an obligation on observable behavior:
  algorithms / pseudocode, technology or stack constraints from outside forces, and
  multi-spec lifecycle procedures that have no owning symbol.
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
