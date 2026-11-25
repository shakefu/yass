# Checking and CLI help

These commands complement the project-reading commands in the [CLI guide](cli.md).

## Check: `validate` and `lint`

```sh
yass validate
yass lint
yass validate root.yass.yaml
yass lint root.yass.yaml
```

`validate [--format FORMAT] [PATH...]` checks the addressed files against the language definition:
YAML structure, preambles, names, slots, obligations, reference syntax, target
existence, and relation/target kinds. It reports errors without repairing files.
The first `PATH` anchors root discovery, so a project can be checked from any
working directory; a `PATH` in another project, a nested one included, exits `4`
before any file is checked.

`lint [--format FORMAT] [PATH...]` checks project hygiene: unreachable documents,
unbound designs, missing schema modelines, a root file with no single named spec to
start reachability from (`no_origin`), a file that does not parse (`unparsed`),
empty slots, specs without slots, open questions or placeholders left in a file
(`TBD`, `TODO`, `FIXME`, `XXX`, `open question`, `??`, or a `?` in obligation or
guard prose or a preamble description), conditions, keywords, or references written
into prose instead of keys, an obligation that is more than one requirement
(`obligation_shape`: prose over 400 characters, code, a sequence of steps, several
clauses, a rationale aside, or an error mapping outside `ERROR`), source code in a
`schema` or `algorithm` design or a Markdown table in any design, and an obligation
that restates a fact a `catalog` design records: an identifier named with one of its
field values, by number, label, or behaviour phrase (`restated_fact`), or, as an
experimental check, such a value stated with no identifier (`restated_value`). It
builds the whole project graph even when paths restrict which files' advisories it
reports.
Lint does not run validation first; it remains usable during authoring.

Both commands are silent and exit `0` when clean. Findings go to standard output
and exit `1`. They check structure, graph properties, and fixed text patterns; they
do not assess prose correctness, completeness, or whether code meets the spec.

## Explain lint rules: `explain`

```sh
yass explain
yass explain yass.lint.obligation_shape
```

`explain [ID]` indexes the lint rule ids when ID is omitted, one record per id in
fixed order. With an ID, it writes what the rule detects, why it exists, how to
resolve a finding, and minimal bad/good YAML examples. It documents lint rules
only, not validation errors or request failures.

IDs match byte-for-byte; prefixes, abbreviations, and other casing do not resolve.
An unknown ID reports `yass.explain.unknown` on standard error, writes nothing to
standard output, and exits `3`. It does not list known ids; use `yass explain` to
find one. Both forms use documentation built into the binary, read no file tree,
and work without a project root. Neither accepts `--format`.

## Check test citations: `citations`

```sh
yass citations
yass citations --missing
yass citations --coverage
```

`citations [--missing | --coverage] [--tests GLOB] [--format FORMAT] [PATH...]` reads
the citations the project's tests carry, each a `yass cite` line as the
[CLI guide](cli.md) describes, and checks every one:

- `yass.citation.syntax`: the citation does not fit the form.
- `yass.citation.unresolved`: its spec, slot, or obligation number does not exist.
- `yass.citation.stale`: the obligation at that number carries another stamp, so
  it was moved or reworded after the citation was written.

A finding names the test file and line. An unresolved or stale one also names the
citation, so it can be passed back to `yass cite`. Findings go to standard output
and exit `1`. This is how a change to a spec reaches the tests written against
it: rather than silently pointing a test at a different obligation, the citation
fails until the test is read again and its line taken again from `yass cite`.

A file is named as a test when its basename holds `test`, `tests`, `spec`, or
`specs` as a whole part: `x_test.go`, `test_x.py`, `x.test.ts`, `XTest.java`.
`--tests GLOB` replaces that rule, matching the whole root-relative path. A
`PATH` narrows the files read, and a file `PATH` is read whatever its name. The
marker is recognized wherever it stands on a line, in any comment syntax, and
only the token after it is read.

`--missing` lists, in the form `yass cite` writes, every `MUST` and `SHOULD`
obligation no citation covers. `--coverage` counts, per spec, the obligations
that `MUST` and that `SHOULD` with how many of each are covered, then the
project's totals on a line whose `REF` is `-`. With two of the quick-start spec's
three obligations cited:

```text
root@Greeting	3	2	0	0
-	3	2	0	0
```

A `MAY` is a permission: it can be cited but is never listed or counted. Both
reports exit `0` and check nothing. A malformed, unresolved, or stale citation
simply covers no obligation there.

## Read authoring documents: `docs`

```sh
yass docs
yass docs brainstorming
yass docs authoring
yass docs language
```

`docs` indexes the embedded language documents with their line counts and the task
that warrants reading each. `docs NAME` writes one document whole. Each fits in 200
lines, so a read through `head -200` loses nothing; the language definition is the
one exception and opens with a line stating its length.

The documents are read in paths, every document in order, and bare `yass` lists
them:

| Path | Documents |
|---|---|
| Write or change a spec | `authoring`, `documents`, `obligations`, `references`, `coverage` |
| Elicit a new spec | `brainstorming`, then the write path |
| Change yass itself | `language`, then the write path |
| Implement and test an approved spec | `implementing` |

Each document's first line names its path and its place in it, and its last line
names the next one or says the path ends. These documents travel with the installed version and work without a
project root. For implementing an existing spec, read `implementing`, then work
from overview and the project's own query results.

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
inside fields and `-` for an absent field. Query output is YAML; named docs and
lint rule explanations are plain text. Diagnostics have five fields: `SEVERITY`,
`CODE`, `LOCATION`, `LINE`, and `MESSAGE`.

### JSON records

`root`, `list`, `find`, `refs`, `cite`, `validate`, `lint`, `citations`, and
`update` accept
`--format FORMAT` after the subcommand: `text` (the default, described above) or
`json`. With `json`, each record becomes one compact JSON object on its own line
(JSON Lines), in the same order, and an empty result still prints nothing:

```sh
yass list --format json
yass find --format=json 'supplied name'
```

```text
{"ref":"root","kind":"file","detail":"1","description":"A greeting function that returns a message for a supplied name."}
{"ref":"root@Greeting","kind":"spec","detail":"INPUT,RETURN,SIDE-EFFECT","description":null}
{"ref":"root@Greeting::RETURN","what":"MUST","text":"return Hello followed by one space, the supplied name, and an exclamation mark"}
```

A diagnostic from `validate`, `lint`, or `citations` has the keys `severity`,
`code`, `location`, `line`, and `message`. A `cite` record has `citation`, `what`,
and `text`; a coverage record has `ref`, `must`, `must_cited`, `should`, and
`should_cited`.

Keys are the lowercase field names, in field order. Each value is the string the
text form would print, with these exceptions. An absent or empty field, which
the text form prints as `-`, is `null`; a field whose real value is `-` stays
the string `"-"`. A diagnostic's `line` is an integer, or `null` when no line is
known. Bytes that are not valid UTF-8 are written as U+FFFD. `<`, `>`, `&`, and
non-ASCII text are written as themselves. Errors on standard error stay in the
text form, and exit statuses do not change. Any other format value is a usage
error, `yass.args.unknown_format`. `overview`, `query`, `docs`, and `explain` do
not accept `--format`.

Requested answers go to standard output. Failures to serve a request go to
standard error. Validation findings, lint advisories, and citation findings are
their commands' requested answers, so they go to standard output.

| Status | Meaning |
| --- | --- |
| `0` | Success, including empty search/edge results and clean checks |
| `1` | Findings: validation errors, lint advisories, malformed, unresolved, or stale citations, or partial results due to parse failures |
| `2` | Usage error, such as an unknown option or malformed target |
| `3` | Unresolved path, document, slot, reference, docs name, or lint rule id; a target whose file does not parse; a reference whose relation does not fit its target's kind; a malformed reference inside a served document; or a `cite` stamp that names no obligation of its slot |
| `4` | No governing project root, or a `PATH` in another project, a nested one included |
| `5` | Environment failure, such as a file or directory that cannot be read, or a failed update |

For scripts and CI, check the exit status as well as capturing output. `validate`
and `lint` complement each other, so run both when checking a spec change.
