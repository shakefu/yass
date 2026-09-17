# CLI index

- `root.yass.yaml`, `00-*.yass.yaml`–`11-*.yass.yaml`: CLI contract and command specs; numeric prefixes give implementation order.
- `HOWTORUN.txt`: build, usage, exit statuses, and examples; Go module starts here.
- `main.go`, `internal/yass/app.go`, `internal/yass/invocation.go`: entry point, dispatch, and argument parsing.
- `internal/yass/`: `cmd_*.go` implements commands; `load.go`, `model.go`, `locate.go`, `target.go`, `fileset.go` handle parsing and discovery; `emit.go`, `output.go`, `errors.go`, `exit.go` handle output and status.
- `internal/yass/yass_test.go`: behavior and embedded-corpus sync tests; run `../script/test` from here.
- `internal/yass/docs.go`, `internal/yass/docs/`: embedded language corpus; refresh source changes with `../script/sync-docs`, never edit copies or add agent files there.
- `build-prompt.md`, `NOTES.md`: implementation exercise constraints and recorded ambiguities; specs remain normative.
