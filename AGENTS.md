# yass

YAML specification language and a read-only Go CLI for serving specs to coding agents.

## Entry points

- `yass.yass.yaml`, `context/yass-reference.md`, `context/GUIDANCE.md`: authoritative language definition, reference, and authoring guidance.
- `yass.v1.schema.json`: derived editor-validation schema; keep consistent with the language.
- `root.yass.yaml`: repository spec root; `cli/`: separate spec project and Go module with implementation and tests (see its index).
- `context/`: language guides and provisional research in `context/experiment/`; `experiment/`: active-run workspace, currently a placeholder (see their indexes).
- `GOAL.md`: experiment method only; its cold-start bar does not govern the language.
- `script/agent`: headless model experiment runner; `script/build [version]`: cross-platform release archives in `dist/`.
- `.github/workflows/`: CI, pre-commit, conventional-commit checks, and releases; `.common-repo.yaml`: inherited release configuration; `cog.toml`: release build hook.

## Development

- `./script/test`: gofmt check, vet, tests, and build (requires Go; otherwise skips).
- `./script/sync-docs`: refresh the CLI's embedded corpus after changing authoritative language documents; never edit those copies directly.
- `prek run --all-files`: repository hooks. Use conventional commits.
- CLI build/run instructions: `cli/HOWTORUN.txt`.

## Maintaining this index

- Update the affected `AGENTS.md` files in the same change when paths, responsibilities, commands, dependencies, or conventions change.
- Keep indexes brief: record semantic entry points and non-obvious constraints; link to existing documentation instead of duplicating it.
- Add a directory index only when it provides useful navigation beyond its parent; omit generated, vendored, and fixture trees.
- Every `AGENTS.md` must have a sibling `CLAUDE.md` containing only `@AGENTS.md`.
