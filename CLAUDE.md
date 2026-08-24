# CLAUDE.md

## Reading policy — read nothing unless told

Do not read, open, list, search, or fetch anything unless the user has
explicitly told you, in this session, to read that specific thing.

This covers:

- source files, spec files, docs, READMEs, config
- directory listings and file searches (`ls`, `find`, `grep`, glob, agents)
- git history, diffs, branches, tags, stashes
- pull requests, issues, CI logs
- the web

There is no "quick peek" exception. A file being obviously relevant is not
permission to open it. Permission is per-item and does not generalize: being
told to read one file does not authorize its neighbours, its imports, or the
directory it sits in.

If you need something in order to proceed, name what you need and why, then
stop and wait. Resolve ambiguity by asking, never by looking.

Running a build, a test, or a binary is not reading — those are fine when the
user asks for them, even though they touch files.

## Building the CLI

`yass` is a Go program rooted at `cli/`:

```
cd cli && go build -o yass .
./yass --help
```

- `cli/*.yass.yaml` — the specs the CLI is built from
- `cli/internal/` — implementation code

## What "a clean CLI build" means

Implement the CLI from the spec files the user names, and from nothing else.

Do not consult any prior or parallel implementation: another branch, a
worktree, a leftover tree in the working directory, `cli/internal/` as it
already stands, the commit history, an open PR, or a web search. If a spec
reads ambiguously, pick a reading, implement it, and write down the ambiguity
and the choice you made — do not go looking for how it was resolved before.

An implementation that borrows from an earlier attempt tells us about the
borrowing, not about the specs.
