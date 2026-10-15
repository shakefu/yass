# Contributing to the documentation

The site is an [mdBook](https://rust-lang.github.io/mdBook/) built from `docs/`.
Install the version used in `.github/workflows/docs.yaml` (currently 0.4.40),
then run these commands from the repository root:

```sh
mdbook build docs
mdbook serve docs --open
```

The second command starts a local preview and rebuilds when source files change.
Stop it with Ctrl-C. Output in `docs/book/` is generated and ignored by Git.
The workflow also copies `yass.v1.schema.json` to `docs/book/v1.schema.json`;
to preview that download locally, copy it after building:

```sh
cp yass.v1.schema.json docs/book/v1.schema.json
```

## Where to edit

Edit CLI guides and examples in `docs/src/`, and add new chapters to
`docs/src/SUMMARY.md`. Verify command examples against the current CLI contracts
in `cli/` and a locally built binary; `cli/HOWTORUN.txt` explains the build.
When documenting a behavior change, update its governing spec and implementation
as part of that change rather than silently describing a new contract here.

The language reference, authoring guidance, and brainstorming chapters are
`{{#include}}` stubs. Edit their authoritative sources in `context/`, then run:

```sh
./script/sync-docs
./script/test
```

This refreshes and verifies the corpus embedded in the CLI. Do not edit
`cli/internal/yass/docs/` directly.

Before submitting a change, build the book, inspect its rendered pages and
navigation, and run `prek run --all-files`. For installer examples, also run
`./script/test-install`. Keep the affected `AGENTS.md` index current when paths
or development commands change.

## CI and publishing

Pull requests affecting documentation, authoritative language sources, the
schema, or the documentation workflow build the book in CI. Pushes to `main`
with those changes build and deploy to [GitHub Pages](https://shakefu.github.io/yass/).
A manual workflow run on `main` can also deploy. Pull request builds do not deploy.
The site publishes the authoritative language documents through includes,
without a second maintained copy.
