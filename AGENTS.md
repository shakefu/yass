# yass

YAML specification language and a read-only Go CLI for serving specs to coding agents.

## Entry points

- `yass.yass.yaml`: authoritative language definition; `context/`: authoritative reference, authoring, brainstorming, and implementing documents (see its index). The CLI embeds copies of all of them and the language definition (`cli/10-docs.yass.yaml` lists the corpus and its reading paths): after editing a source, run `./script/sync-docs`; never edit the copies. Each markdown document stays under 200 lines (a test enforces it); split a growing one by topic.
- `yass.v1.schema.json`: derived editor-validation schema; keep consistent with the language.
- `root.yass.yaml`: repository spec root; `cli/`: separate spec project and Go module with implementation and tests (see its index).
- `docs/`: mdBook site published to GitHub Pages (see its index).
- `install.sh`: curl-able release installer (README's Install section documents it); `setup/`: composite action consumed as `shakefu/yass/setup@<ref>` that installs a released yass on CI runners by wrapping install.sh; `.github/workflows/setup-action.yaml`: its platform-matrix test; `Formula/yass.rb`: Homebrew formula, regenerated per release by `script/gen-formula` — never edit it by hand.
- `.pre-commit-hooks.yaml`: published `yass-validate`/`yass-lint` hooks; `script/pre-commit-hook` runs the installed CLI per nearest project root and never installs it.
- `script/build [version]`: cross-platform release archives in `dist/`; `script/gen-formula [version]`: formula from `dist/checksums.txt` (both are cog pre-bump hooks).
- `.github/workflows/`: CI, pre-commit, conventional-commit checks, releases, and the `docs.yaml` mdBook/Pages deploy; `ci.yaml` is common-repo template-owned — no local edits; `.common-repo.yaml`: inherited release configuration; `cog.toml`: release build hook.

## Development

- `prek install` once per clone: installs the pre-commit and commit-msg (conventional commit) hooks.
- PRs are rebase-merged, so every commit lands on `main` and feeds cog's changelog: each must be a conventional commit.
- `./script/test`: installer tests, then gofmt check, vet, Go tests, build, script tests, and the authored-spec gate. Without Go the Go parts skip locally but fail in CI. `./script/test-install`: install.sh and gen-formula tests alone.
- `./script/check-specs [PROJECT...]`: validate, lint, and check citations of `.` and `cli/` with the CLI built from this checkout; every validation error fails, and the lint and citation policy tables disposition each code as failing or advisory (an undispositioned code fails). Add a disposition when a lint or citation code is added. `./script/test-check-specs`: its tests. `./script/test-pre-commit-hook`: tests for the hook wrapper.
- Tests cite the obligation each covers in a `yass cite` line above the test; an edit that moves or rewords a cited obligation fails the citations gate until the citing test's line is taken again from `yass cite`.
- `prek run --all-files`: repository hooks.
- CLI build/run instructions: `cli/HOWTORUN.txt`.

## Maintaining this index

- Update the affected `AGENTS.md` files in the same change when paths, responsibilities, commands, dependencies, or conventions change.
- Keep indexes brief: record semantic entry points and non-obvious constraints; link to existing documentation instead of duplicating it.
- Add a directory index only when it provides useful navigation beyond its parent; omit generated, vendored, and fixture trees.
- Do not add `CLAUDE.md` files; Claude Code reads `AGENTS.md` natively, and a `CLAUDE.md` anywhere in the tree stops every `AGENTS.md` from loading.
