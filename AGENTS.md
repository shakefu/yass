# yass

YAML specification language and a read-only Go CLI for serving specs to coding agents.

## Entry points

- `yass.yass.yaml`: authoritative language definition; `context/`: authoritative reference, authoring guidance, and brainstorming flow (see its index). The CLI embeds copies of these four documents: after editing a source, run `./script/sync-docs`; never edit the copies.
- `yass.v1.schema.json`: derived editor-validation schema; keep consistent with the language.
- `root.yass.yaml`: repository spec root; `cli/`: separate spec project and Go module with implementation and tests (see its index).
- `docs/`: mdBook site published to GitHub Pages (see its index).
- `install.sh`: curl-able release installer (README's Install section documents it); `Formula/yass.rb`: Homebrew formula, regenerated per release by `script/gen-formula` — never edit it by hand.
- `.pre-commit-hooks.yaml`: published `yass-validate`/`yass-lint` hooks; `script/pre-commit-hook` runs the installed CLI per nearest project root and never installs it.
- `script/build [version]`: cross-platform release archives in `dist/`; `script/gen-formula [version]`: formula from `dist/checksums.txt` (both are cog pre-bump hooks).
- `.github/workflows/`: CI, pre-commit, conventional-commit checks, releases, and the `docs.yaml` mdBook/Pages deploy; `.common-repo.yaml`: inherited release configuration; `cog.toml`: release build hook.

## Development

- `prek install` once per clone: installs the pre-commit and commit-msg (conventional commit) hooks.
- PRs are rebase-merged, so every commit lands on `main` and feeds cog's changelog: each must be a conventional commit.
- `./script/test`: installer tests, then gofmt check, vet, Go tests, build, script tests, and the authored-spec gate. Without Go the Go parts skip locally but fail in CI. `./script/test-install`: install.sh and gen-formula tests alone.
- `./script/check-specs [PROJECT...]`: validate and lint `.` and `cli/` with the CLI built from this checkout; every validation error fails, and its lint policy table dispositions each lint code as failing or advisory (an undispositioned code fails). Add a disposition when a lint code is added. `./script/test-check-specs`: its tests. `./script/test-pre-commit-hook`: tests for the hook wrapper.
- `prek run --all-files`: repository hooks.
- CLI build/run instructions: `cli/HOWTORUN.txt`.

## Maintaining this index

- Update the affected `AGENTS.md` files in the same change when paths, responsibilities, commands, dependencies, or conventions change.
- Keep indexes brief: record semantic entry points and non-obvious constraints; link to existing documentation instead of duplicating it.
- Add a directory index only when it provides useful navigation beyond its parent; omit generated, vendored, and fixture trees.
- Do not add `CLAUDE.md` files; Claude Code reads `AGENTS.md` natively, and a `CLAUDE.md` anywhere in the tree stops every `AGENTS.md` from loading.
