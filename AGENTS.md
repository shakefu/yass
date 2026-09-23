# yass

YAML specification language and a read-only Go CLI for serving specs to coding agents.

## Entry points

- `yass.yass.yaml`, `context/yass-reference.md`, `context/GUIDANCE.md`, `context/BRAINSTORMING.md`: authoritative language definition, reference, authoring guidance, and the spec-elicitation flow the CLI serves as `yass docs brainstorming`.
- `yass.v1.schema.json`: derived editor-validation schema; keep consistent with the language.
- `root.yass.yaml`: repository spec root; `cli/`: separate spec project and Go module with implementation and tests (see its index).
- `context/`: language guides and provisional research in `context/experiment/`; `experiment/`: active-run workspace, currently a placeholder (see their indexes).
- `GOAL.md`: experiment method only; its cold-start bar does not govern the language.
- `install.sh`: curl-able release installer (README's Install section documents it); `Formula/yass.rb`: Homebrew formula, regenerated per release by `script/gen-formula` — never edit it by hand.
- `script/agent`: headless model experiment runner; `script/build [version]`: cross-platform release archives in `dist/`; `script/gen-formula [version]`: formula from `dist/checksums.txt` (both are cog pre-bump hooks).
- `.github/workflows/`: CI, pre-commit, conventional-commit checks, and releases; `.common-repo.yaml`: inherited release configuration; `cog.toml`: release build hook.

## Development

- `./script/test`: installer tests, then gofmt check, vet, tests, and build (the Go parts skip without Go); `./script/test-install`: install.sh and gen-formula tests alone.
- `./script/sync-docs`: refresh the CLI's embedded corpus after changing authoritative language documents; never edit those copies directly.
- `prek run --all-files`: repository hooks. Use conventional commits.
- CLI build/run instructions: `cli/HOWTORUN.txt`.

## Maintaining this index

- Update the affected `AGENTS.md` files in the same change when paths, responsibilities, commands, dependencies, or conventions change.
- Keep indexes brief: record semantic entry points and non-obvious constraints; link to existing documentation instead of duplicating it.
- Add a directory index only when it provides useful navigation beyond its parent; omit generated, vendored, and fixture trees.
- Every `AGENTS.md` must have a sibling `CLAUDE.md` containing only `@AGENTS.md`.
