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

`root`, `list`, `find`, `refs`, `validate`, `lint`, and `update` accept
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

A diagnostic from `validate` or `lint` has the keys `severity`, `code`,
`location`, `line`, and `message`.

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
standard error. Validation findings and lint advisories are their commands'
requested answers, so they go to standard output.

| Status | Meaning |
| --- | --- |
| `0` | Success, including empty search/edge results and clean checks |
| `1` | Findings: validation errors, lint advisories, or partial results due to parse failures |
| `2` | Usage error, such as an unknown option or malformed target |
| `3` | Unresolved path, document, slot, reference, docs name, or lint rule id; a target whose file does not parse; a reference whose relation does not fit its target's kind; or a malformed reference inside a served document |
| `4` | No governing project root |
| `5` | Environment failure, such as unreadable files or a failed update |

For scripts and CI, check the exit status as well as capturing output. `validate`
and `lint` complement each other, so run both when checking a spec change.
