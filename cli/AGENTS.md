# CLI index

- `root.yass.yaml`, `00-*.yass.yaml`–`15-*.yass.yaml`: CLI contract and command specs; numeric prefixes give implementation order.
- `HOWTORUN.txt`: build, usage, exit statuses, and examples; Go module starts here.
- `main.go`, `internal/yass/app.go`, `internal/yass/invocation.go`: entry point, dispatch, and argument parsing.
- `internal/yass/`: `cmd_*.go` implements commands; `citation.go` holds the citation grammar, the test-file name rule, obligation numbering, and the stamp; `lint_prose.go`, `lint_design.go`, `lint_restate.go` hold lint's text-pattern advisories (prose, design content, restated catalogue facts); `lint_explain.go` carries documentation for every lint rule; `load.go`, `model.go`, `locate.go`, `target.go`, `fileset.go` handle parsing and discovery; `glob.go`, `fold.go` handle pattern and case-insensitive matching; `emit.go`, `output.go`, `errors.go`, `exit.go` handle output and status.
- `internal/yass/yass_test.go`, `internal/yass/update_test.go`: behavior, embedded-corpus sync, and self-update tests (the latter fake the release host; nothing touches the network); run `../script/test` from here.
- `internal/yass/docs.go`, `internal/yass/docs/`: embedded language corpus; refresh source changes with `../script/sync-docs`, never edit copies or add agent files there.
- Tests cite the obligation each covers above the test, in a line `yass cite` writes, and `../script/check-specs` fails on one that is malformed, unresolved, or stale. A test file, and any script the test-file name rule selects, is scanned as text: a citation spelled whole in a fixture is a citation of this project, so build one from `citationMarker` or assemble the marker.
