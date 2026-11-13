# yass — References

*Write path, 4 of 5: authoring → documents → obligations → **references** → coverage. Run each as `yass docs NAME` and read it whole.*

How a spec points at another spec, slot, or design, what each relation binds, and how a query resolves them.

## References

- Relation is the **YAML key**; target is a **single string**.
- **Target syntax:** `path@SpecName::SLOT`
  - A **bare name** (`SpecName`) addresses a spec or design in the **same file**.
  - `@` separates a path from the spec name (`path@SpecName`) for **other files**.
  - `::` separates the spec name from a slot (`SpecName::SLOT`); omit it to address the
    whole spec. The `SLOT` token must name one of the five recognized slots (see
    *Slots*), and the referenced spec must declare that slot in its own document. Any
    other slot token, or a slot the target spec does not declare, is a validation error.
  - **Path resolution:** a `./` or `../` path is **relative to the referencing file**; a
    path with any other lead resolves **from the project root** (see *Project root*
    in `yass docs documents`) — including one whose leading dot opens a hidden name, such as `.config`,
    which is a root-relative token, not a file-relative one. A path beginning with `/`
    is **rejected**: yass paths are file-relative or root-relative, never absolute. A
    path may climb with `../` past the project root; the reached file stays governed by
    its own nearest root. The `.yass.yaml` extension is omitted: resolution appends
    that literal suffix to the path token and tries no other basename or extension.
  - **Token charsets:** the path token uses `/` separators and only `[A-Za-z0-9._/-]`, so
    a backslash is rejected; the name token uses only `[A-Za-z0-9._-]`; the slot token
    uses only `[A-Z-]`. Paths and names compare case-sensitively, byte for byte.
  - The **slot is the finest addressable unit**. Named anchors, never line numbers.
    Slots belong to specs only: **`::SLOT` on a target that resolves to a design is a
    validation error** — a design carries no slots.
  - (`#` was rejected as a separator — a leading `#` is a YAML comment, and `@` is a
    reserved YAML indicator that can't start a plain scalar either. Making same-file refs
    bare names means no target ever leads with an indicator, so none need quoting.)
- **Relations:**

  - `CONFORMS` targets a spec or a slot. A hard requirement: the carrier must match the
    spec or slot. A slot target is inlined; a whole-spec target is not.
  - `USES` targets a design block. Binding design the implementation must follow: its
    content is appended with provenance.
  - `SEE` targets a spec, a slot, or a design. An open-ended pointer, never inlined, that
    imposes nothing on its own.

  **`CONFORMS` and `USES` each pin a target kind; `SEE` takes any.** A
  relation/target-kind mismatch — `USES` → spec, `CONFORMS` → design — is a **validation
  error**, so the CONFORMS-vs-USES distinction is machine-checkable rather than a
  judgment call.

  `CONFORMS` is a hard requirement with **two aspects of one meaning**: the carrier must
  **match** the referenced spec or slot, and inlining is *how* that match is made
  checkable in place. A **slot-targeted** `CONFORMS` (`…::SLOT`) inlines the referenced
  slot's obligations in front of the carrier. A **whole-spec** `CONFORMS` (no `::SLOT`) is
  a conformance reference to the entire spec — it is **not** transcluded (there is no
  single slot to splice); the conformer must satisfy the referenced spec as a whole. This
  matches the language meaning that `CONFORMS` must "match the referenced spec **or**
  slot." `USES` binds the carrier to a **design block**: resolution appends the block's
  typed content to the emitted fragment, **once**, with a provenance comment
  (`# USES: StartupSequence`), deduplicated when several obligations reference the same
  block — a design block has no obligations to splice into a slot. When a guarded
  obligation carries a `USES`, the guard scopes **when the design binds**. `SEE` names a
  spec or design **for the reader**: it is never inlined and imposes nothing on the
  carrier beyond what the obligation's own prose already states.

- **Choosing a relation — three questions, in order.**
  1. *Must this obligation match what the target says?* → **`CONFORMS`**. Slot-targeted
     (`…::SLOT`) when one specific slot is the contract; whole-spec when another spec
     owns cross-cutting rules this one must obey (see *Composition* in `yass docs authoring`).
  2. *Is the target a design block whose freeform body binds the implementation?* →
     **`USES`**. Design blocks only — `USES` at a spec is a validation error.
  3. *Everything else* → **`SEE`**. It is the open-ended catch-all, and the right key
     even when a genuine dependency exists: a dispatcher spec naming the handler specs
     it **dispatches** to, a stage that **must run after** another spec, or plain "go
     read this next." The dependency is stated in the obligation's own prose; `SEE` only
     says where the named thing is defined. What `SEE` must never carry is a
     *constraint* — if the target's rules bind this spec, that is whole-spec `CONFORMS`,
     and reaching for `SEE` silently drops the requirement.

- **A carrier's guard scopes what it inlines.** When a slot-targeted `CONFORMS` carrier
  has a `WHEN` guard, every inlined obligation applies only when that guard holds. An
  inlined obligation that carries its own `WHEN` applies only when **both** guards hold
  — the carrier's guard conjoined with the inner guard. (This is the language-level
  meaning; how a tool renders the combined guard text is the tool's concern.)

- **Slot-targeted `CONFORMS` carries the dataflow reading.** When one spec's `INPUT`
  consumes the data another spec's slot produces — characteristically an `INPUT` that
  points at a producer's `RETURN`, `CONFORMS <producer>::RETURN` — it means the
  obligation **must match the data that slot produces**: the data crossing the boundary
  is exactly what that slot yields, so the producer's `RETURN` guarantees characterize
  it here, and inlining puts those guarantees in front of the consumer's `INPUT`. This
  is the structural anchor for a pipeline or producer/consumer relationship. It does
  **not** by itself decide the trust boundary — which of the producer's guarantees the
  consumer relies on versus re-checks is the consuming spec's own obligation to state
  (see *Composition* in `yass docs authoring` and `yass docs coverage`). That obligation includes the **residual on violation**: for each
  guarantee the consumer relies on without re-validating, the consuming spec must also
  state what it does if that guarantee does not hold — even if only to declare the
  behavior unspecified. Stating the trust without pinning its violation leaves every
  implementer to invent the out-of-contract behavior.

- DRY is achieved by **transclusion** (inlining), not bare pointers.

## Provenance (emitted fragments)

- Inlined obligations carry a YAML comment naming the source, e.g.
  `# CONFORMS: db/user@User::RETURN`. Appended design blocks carry the same, e.g.
  `# USES: StartupSequence`.
- An emitted fragment identifies the **queried** spec; there is **no host `file:`
  header** (redundant and able to contradict — the filesystem is the source of truth).

## Resolution and scope

- Resolution: a **slot-targeted** `CONFORMS` is inlined into the referencing spec (one level
  in v1); a **whole-spec**
  `CONFORMS` (no `::SLOT`) is not inlined — it is a conformance reference to the entire spec.
  `SEE` is a pure pointer, never inlined. `USES` appends its design block's typed content
  to the emitted fragment, once per block, with provenance.
- Verification is out of scope — tooling routes/retrieves, never verifies obligation
  content (keeps it language-agnostic). A `COMPATIBLE` relation was deliberately
  excluded.
- `WHEN` states a sufficient condition only.
- Extension: `.yass.yaml`, chosen over bare `.spec` after a collision
  review. Bare `.spec` is owned by RPM spec files / PyInstaller / Vim+Neovim filetype /
  GitHub Linguist / freedesktop MIME, and — decisively — a bare `.spec` never gets the
  `yaml` language id, so it earns neither free YAML highlighting nor JSON-Schema
  association. A trailing `.yaml` fixes that; the `.yass` infix namespaces it.

---

Next in the write path: `yass docs coverage`.
