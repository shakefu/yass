# CLI Guide

The yass CLI helps you locate, retrieve, and check a project's specs. It reads
spec files without changing them. `update` is the exception: it can replace the
installed CLI binary.

```text
yass [-C DIR] [--version] [--help] [SUBCOMMAND [ARGS...]]
```

Global options go before the subcommand; command options go after it. `-C DIR`
sets the starting directory for paths and root discovery. `--help` (`-h`) prints
usage; `--version` (`-V`) prints the CLI and language versions. Use `--` before an
operand that starts with `-`.

```sh
yass -C greeting-spec list
yass find -- '-'
```

The following examples use the project from [Quick Start](quick-start.md), unless
a target is described as an example from a larger project.

## Projects, paths, and targets

The nearest `root.yass.yaml` at or above the starting directory defines the
project. A nested root defines a separate project; ordinary project traversal
does not include that subtree. Project traversal collects `.yass.yaml` files
regardless of version-control ignore rules. It does not descend through directory
symlinks.

Commands accepting `PATH` take filesystem paths, including the suffix when naming
a file. Commands accepting `TARGET` use the notation below and omit that suffix:

| Target | What it addresses |
| --- | --- |
| `root` | Preamble of `root.yass.yaml` |
| `root@Greeting` | The spec named `Greeting` in that file |
| `root@Greeting::RETURN` | The spec's declared `RETURN` slot |
| `format@GreetingTemplate` | A design named `GreetingTemplate` in `format.yass.yaml` |
| `./format@GreetingTemplate` | The same form, relative to the starting directory |

A target path without a leading dot resolves from the project root. `./` and
`../` paths on the command line resolve from the starting directory. Inside spec
files, those paths resolve from the referencing file's directory, and a bare
document name such as `GreetingTemplate` refers to a document in the same file.
On the command line, a target with no `@` addresses a **file**, not a same-file
spec. Use `root@Greeting` to select that spec explicitly.

Document names and slots are case-sensitive. Slots are `INPUT`, `RETURN`,
`ERROR`, `SIDE-EFFECT`, and `INVARIANT`; a target must name a slot the spec
actually declares. Designs have no slots.

## Orient: `overview` and `root`

```sh
yass
yass overview
yass root
yass root root.yass.yaml
```

With no subcommand, yass runs `overview`. It introduces the CLI, reports the
project when one is available, summarizes the notation, and lists reading
commands. It also works outside a project.

`root [PATH]` prints only the absolute directory path of the project governing
`PATH`, or the starting directory when omitted. Use it to check which project
a command will inspect. Missing a project root is an error for `root`.

## Map: `list`

```sh
yass list
yass list root.yass.yaml
yass list --filter 'root*'
yass list --filter root
yass list --filter '*@Greeting'
yass list --filter '*@*Handler*' --filter '*@*Policy*'
```

`list [--filter GLOB] [PATH...]` indexes the project, or the files and directories
you select. Each tab-separated record has `REF`, `KIND`, `DETAIL`, and
`DESCRIPTION` fields. File detail is the number of documents; spec detail is its
declared slots; design detail is its type. File descriptions are shortened to
100 Unicode characters, and document descriptions are absent (`-`).

Filters match the **whole REF string**: `*` matches any run of characters,
including `/` and `@`, and `?` matches one character. All other characters are
literal; there are no regexes or bracket classes. Quote patterns so your shell
does not expand them. Multiple filters are combined with OR. If a document
matches, its parent file record is retained even when that record does not match.

`root*` matches the file REF `root` and document REFs such as `root@Greeting`;
the wildcard crosses the `@` separator. The exact filter `root` matches only the
file record. `*@Greeting` selects that document name across files, together with
each matching document's parent file record.

An empty selection prints nothing and succeeds. An unparseable file remains in
the index with unavailable fields and causes status `1`; use `validate` to
diagnose it.

## Read: `query`

```sh
yass query root
yass query root@Greeting
yass query root@Greeting::INPUT root@Greeting::RETURN
yass query --raw root@Greeting
```

`query [--raw] TARGET...` emits a YAML document stream in target order. A file
target emits its preamble, a spec target emits its declared slots, a slot target
emits that slot under the spec name, and a design target emits its design block.

By default, query resolves one level of references:

- Slot-targeted `CONFORMS` inserts the referenced obligations before the carrier,
  with provenance comments. Guarded carriers and guarded inserted obligations
  have their guards conjoined.
- `USES` appends the referenced design once per fragment, with provenance.
- Whole-spec `CONFORMS` and `SEE` remain as authored references.

References inside inserted obligations or appended designs are not recursively
expanded. `--raw` emits the addressed content as authored, with no resolution or
provenance comments. It can retrieve a parsed document whose references are
broken. All requested targets must resolve before query emits anything; an
unresolved target or a failed default resolution leaves standard output empty.

## Locate prose: `find`

```sh
yass find 'supplied name'
yass find --slot RETURN 'hello'
```

`find [--slot SLOT] PATTERN` performs a case-insensitive **literal substring**
search, not a regex, glob, or word-boundary search. It searches obligation prose,
`WHEN` guards, and individual lines of design content. It does not search names,
preamble descriptions, or reference targets.

Records contain `REF`, `WHAT`, and `TEXT`. `WHAT` is the normativity keyword,
`WHEN`, or `content`; a matching field appears once with its full text. The REF
can be passed directly to `query`. `--slot` restricts obligations, while design
content is still searched because designs have no slots. No matches is a
successful empty result. Parse failures allow matches from the remaining files
and produce status `1` with diagnostics on standard error.

## Follow dependencies: `refs`

```sh
yass refs root@Greeting
yass refs --out root@Greeting
yass refs --in root@Greeting::RETURN
```

`refs [--in] [--out] TARGET` reports direct reference edges for one document or
slot. Without flags, or with both flags, it reports both directions. `--out`
shows what the target references; `--in` shows what references it. Records contain
`DIR`, `RELATION`, and the `REF` at the other end. Outgoing records precede
incoming records. The quick-start spec has no references, so these examples
produce no output and succeed.

A whole-spec target includes incoming references to any of its slots; a slot
target selects outgoing references carried in that slot and incoming references
that name that slot. File-only targets are not accepted. Incoming edges are
searched within the project, and unresolved references are omitted; use
`validate` to diagnose them. References are not followed transitively.

## Check: `validate` and `lint`

```sh
yass validate
yass lint
yass validate root.yass.yaml
yass lint root.yass.yaml
```

`validate [PATH...]` checks the addressed files against the language definition:
YAML structure, preambles, names, slots, obligations, reference syntax, target
existence, and relation/target kinds. It reports errors without repairing files.

`lint [PATH...]` checks project hygiene: unreachable documents, unbound designs,
missing schema modelines, empty slots, and specs without slots. It builds the
whole project graph even when paths restrict which files' advisories it reports.
Lint does not run validation first; it remains usable during authoring.

Both commands are silent and exit `0` when clean. Findings go to standard output
and exit `1`. They check structure and graph properties; they do not assess prose
correctness, completeness, or whether code meets the spec.

## Read authoring documents: `docs`

```sh
yass docs
yass docs brainstorming
yass docs reference
yass docs guidance
yass docs language
```

`docs` indexes the embedded language documents with the task that warrants reading
each. `docs NAME` writes one document whole. Use `brainstorming` to elicit a new
spec, `reference` and `guidance` to author specs, and `language` to change yass
itself. These documents travel with the installed version and work without a
project root. For implementing an existing spec, start with overview and the
project's own query results.

## Keep current: `update`

```sh
yass --version
yass update
```

`update` checks the newest stable GitHub release. If already current it reports
`current`; otherwise it downloads the platform archive, verifies its checksum,
and replaces the installed binary. A Homebrew-managed binary is left in place
with a `homebrew` result; upgrade it with `brew update && brew upgrade yass`.
It needs network access and permission to replace the binary, but no project
root. See [Installation](installation.md) for installation and upgrade options.

## Output and exit statuses

Indexes and search results use tab-separated records, with whitespace collapsed
inside fields and `-` for an absent field. Query output is YAML; named docs are
plain text. Diagnostics have five fields: `SEVERITY`, `CODE`, `LOCATION`, `LINE`,
and `MESSAGE`.

Requested answers go to standard output. Failures to serve a request go to
standard error. Validation findings and lint advisories are their commands'
requested answers, so they go to standard output.

| Status | Meaning |
| --- | --- |
| `0` | Success, including empty search/edge results and clean checks |
| `1` | Findings: validation errors, lint advisories, or partial results due to parse failures |
| `2` | Usage error, such as an unknown option or malformed target |
| `3` | Unresolved path, document, slot, reference, or docs name |
| `4` | No governing project root |
| `5` | Environment failure, such as unreadable files or a failed update |

For scripts and CI, check the exit status as well as capturing output. `validate`
and `lint` complement each other, so run both when checking a spec change.
