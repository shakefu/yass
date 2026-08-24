# yass

**Yet Another Spec Syntax** — a YAML-based specification language for
AI-agent-driven development. Specs capture desired behavior and are consumed to
drive TDD and implementation generation.

> 🚧 Early work in progress.

## What's here

- [`yass.yass.yaml`](./yass.yass.yaml) — the yass language defined in yass itself.
- [`yass.v1.schema.json`](./yass.v1.schema.json) — JSON Schema for editor
  validation of `.yass.yaml` files.
- [`cli/`](./cli) — `yass`, a read-only command-line program that serves an
  existing spec set to a coding agent. Build and run it per
  [`cli/HOWTORUN.txt`](./cli/HOWTORUN.txt).

## The CLI, for an agent arriving cold

Run `yass` with no arguments. It writes what the program is, what project the
current directory sits in, the notation a spec file uses, and the commands that
read a spec set — enough to start implementing without opening a single file by
hand.

The language documents are carried inside the binary and served by `yass docs`,
so they travel with it wherever it is installed. Read one when you are writing
or changing a spec; implementing against specs that already exist needs none of
them.

## License

[AGPL-3.0](./LICENSE)
