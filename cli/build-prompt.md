# Build prompt — implementing the yass CLI

Steerage for an agent implementing the specs in this directory.

> **This document is not part of the spec.** The `.yass.yaml` files are the contract and
> the only normative statement of behaviour; nothing here adds, relaxes, or reinterprets an
> obligation. This file covers what the specs deliberately do *not* say — which language and
> libraries to reach for, and which library defaults will silently violate a spec that reads
> as though it were satisfied. If this document and a spec ever disagree, the spec wins.
>
> Do not read this to understand *what* to build. Read the specs for that: they are
> self-sufficient by design, and `root.yass.yaml` is the entry point.

## The task

Implement `yass`, a read-only CLI, from the specs in this directory. Start at
`root.yass.yaml`; the numeric file prefixes are implementation order.

## Language

**Use Go**, unless you have a reason not to.

The whole risk of this build sits in the YAML parser. `02-load` needs four things at once:
1-based line numbers on every document and obligation, resolved tags (to keep
`yes`/`no`/`on`/`off` as strings), anchor/alias/tag detection, and preserved duplicate keys.
Most YAML APIs destroy all four the moment you load into a native map.
`gopkg.in/yaml.v3`'s `yaml.Node` exposes every one as a struct field — `.Line`, `.Tag`,
`.Anchor`, `.Alias`, and `Content` pairs kept verbatim so duplicates are a short walk.

Go also lines up with three specific obligations almost for free:

- `strings.EqualFold` / `unicode.SimpleFold` is exactly the *simple* Unicode case folding
  `06-find` asks for.
- `filepath.WalkDir` does not follow symlinks by default, and `DirEntry.Type()` gives the
  file-versus-directory symlink distinction `01-locate@FileSet` needs.
- A single static binary is the most reliable artifact to hand a black-box grader: no
  dependency install, no runtime version skew.

Reasonable alternates, with their trade-offs:

- **Rust** — the one real argument against Go. This spec is closed sets everywhere (55
  diagnostic codes, 6 exit statuses, 5 slots, 3 relations), and `match` exhaustiveness
  catches a missed arm at compile time. Use `yaml-rust2`; `serde_yaml` is archived and has
  no source positions.
- **Python** — the most compact option. Use `ruamel.yaml`, not `PyYAML`: ruamel gives line
  and column via `.lc`, duplicate-key detection via `allow_duplicate_keys=False`, and YAML
  1.2 semantics directly. With PyYAML you must subclass `SafeLoader`, rebuild
  `yaml_implicit_resolvers`, and add a custom mapping constructor to get the same three
  things.

Do not reach for C++ here. In a prior multi-language run of an earlier version of these
specs it produced nearly five times the code of the next largest implementation, for no
capability this spec needs.

## The two instructions that matter most

**1. Do not use a YAML serializer for `query` output. Hand-roll the emitter.**

`04-query` requires byte-identical output, `# CONFORMS:` and `# USES:` provenance comments
placed *between list items*, authored order preserved, and per-scalar quoting for prose
holding a colon followed by a space. Every YAML library will fight you on at least one of
those, and round-tripping through one makes byte-identical output a hope rather than a
property. The emitted grammar is small and fully specified — `spec:`/`design:` keys, slot
keys, `- KEYWORD: value` items, comment lines, `---` separators. Write it as direct string
formatting; it is on the order of sixty lines and it makes determinism trivially true.

**2. Parse at the node or event level. Never load straight into a native map.**

One `Unmarshal` into a struct, `map[string]any`, or `dict` throws away line numbers,
duplicate keys, and tag information in a single step — and `08-validate` needs all three to
emit `yass.document.duplicate_key`, `yass.document.yaml_feature`, and every diagnostic's
`LINE` field. Build your own model from the node tree, as `02-load@Load::RETURN` describes.

## Traps that read as satisfied but are not

Each of these is a place where a library's default quietly violates a spec obligation.

- **The Norway problem.** `yes`, `no`, `on`, `off` must stay plain strings
  (`02-load@Load::INPUT`). Go's `yaml.v3` tags them `!!bool`; walk the node tree and retag
  to `!!str` before building your model. PyYAML resolves them as booleans too.
- **Glob semantics.** `05-list`'s `--filter` says `*` matches *any* run of characters. Go's
  `path.Match` refuses to let `*` cross `/`, so it is the wrong function here. Use
  `doublestar`, or hand-roll the matcher — it is two dozen lines.
- **Case folding is *simple*, not full.** Python's `str.casefold()` performs full folding
  (ß → ss); that is a deviation from `06-find::INPUT`. Go's `strings.EqualFold` is correct.
- **Truncation counts Unicode scalar values.** `05-list` truncates a description to 100
  scalar values — code points, not bytes and not grapheme clusters. `[]rune` in Go, `len()`
  on a `str` in Python.
- **Symlinks are asymmetric.** Do not descend through a symlink naming a directory; do read
  through one naming a regular file (`01-locate@FileSet::INVARIANT`).
- **Resolution is one level only.** A reference carried by an obligation you inlined, or by
  a design you appended, stays unresolved (`04-query@Resolution::INVARIANT`). Do not
  recurse, and do not add cycle detection — one level cannot enter a cycle.
- **Inlined obligations are never deduplicated; appended designs always are.** Two carriers
  inlining the same slot each get their own copy in place, but a design referenced by
  several obligations is appended exactly once per fragment.
- **An empty result exits 0, not 1.** Status 1 means a finding *about the spec set*. A
  search that matched nothing, a list that selected nothing, and a document with no edges
  all succeed silently.
- **Exactly one exit status per invocation, and the higher number wins.** Select it once;
  do not let a later condition raise or lower it.
- **Loading is permissive on purpose.** A file that parses but violates a yass rule still
  yields its documents. Only `08-validate` names a rule violation; every other command
  reports at most that a file yielded no documents.

## Determinism checklist

`root@YassCli::INVARIANT` requires byte-identical output for the same tree, starting
directory, and argument vector. Before you call it done:

- Sort every traversal explicitly. Never emit in filesystem-report order.
- Write bytes, not locale-aware formatted text. No locale, no time, no terminal-width
  probing, no `isatty` branching.
- Collapse whitespace in a field before writing it, then trim, then check for empty and
  substitute `-`.
- Terminate every line with exactly one LF, the last one included. Never emit a blank line.

## Deliverables

- The built program, invocable as `yass`.
- A `HOWTORUN.txt` giving the exact build and run commands from a clean checkout.
- A `NOTES.md` recording anything the specs left you guessing about, any obligation you
  could not satisfy and why, and the language and libraries you chose.

Report honestly in `NOTES.md`. An obligation you skipped and named is far more useful than
one you silently approximated.
