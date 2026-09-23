# yass

**Yet Another Spec Syntax** — a YAML-based specification language for
AI-agent-driven development. Specs capture desired behavior and are consumed to
drive TDD and implementation generation.

> 🚧 Early work in progress.

## What's here

- [`yass.yass.yaml`](./yass.yass.yaml) — the yass language defined in yass itself.
- [`yass.v1.schema.json`](./yass.v1.schema.json) — JSON Schema for editor
  validation of `.yass.yaml` files.
- [`cli/`](./cli) — `yass`, a command-line program that serves an existing spec
  set to a coding agent, read-only toward every spec set. Install it per the
  section below, or build and run it per [`cli/HOWTORUN.txt`](./cli/HOWTORUN.txt).

## Install

Releases publish prebuilt binaries for macOS (arm64, amd64), Linux (arm64,
amd64), and Windows (amd64), with sha256 checksums, on the
[releases page](https://github.com/shakefu/yass/releases). No Go toolchain is
needed for any path below.

### Installer script

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh | sh
```

The installer detects the host OS and architecture, downloads the matching
archive for the latest stable release, verifies it against the release's
published checksum, and installs it into `/usr/local/bin` when that is
writable, else `~/.local/bin` — telling you about any PATH setup it needs. Pin
a version or choose the directory:

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh |
  sh -s -- --version v0.2.0 --bin-dir "$HOME/bin"
```

On Windows, download the `windows_amd64` zip from the releases page instead.

### Homebrew

```sh
brew tap shakefu/yass https://github.com/shakefu/yass
brew install yass
```

The formula ([`Formula/yass.rb`](./Formula/yass.rb)) is regenerated with each
release, so `brew update && brew upgrade yass` moves to the newest one.

### Updating

An installer-managed (or manually placed) binary updates itself:

```sh
yass update
```

It checks the newest stable release, verifies the download against the
published checksum, and replaces the binary in place — preserving the working
installation if anything fails. A Homebrew-managed binary is left alone and
reported as such; upgrade it with `brew upgrade yass`.

Verify any installation with `yass --version`.

### From source

Build with Go per [`cli/HOWTORUN.txt`](./cli/HOWTORUN.txt).

## Quick start

yass is for working with a coding agent. You say what you want, the agent
writes it down as a spec the two of you agree on, and then an agent builds
exactly that. The `yass` command is how the agent finds its way around, and it
works with any agent that can run a shell command.

1. **Install `yass`** (above) and tell your agent to use it. One line in your
   project's agent instructions (`AGENTS.md`, `CLAUDE.md`, or the like) is
   enough:

   > Run `yass` with no arguments before writing, changing, or implementing a
   > spec.

2. **Ask for what you want.** The output of that one command tells the agent
   which of three paths to take:

   - **Starting something new** — a project or feature with no spec yet. The
     agent runs the brainstorming flow: it asks what the thing is for, works
     through each behavior with you (proposing answers for you to correct rather
     than making you compose them), and drafts the spec. Nothing gets built until
     you have read the spec and said yes.
   - **Implementing a spec that exists** — the agent reads the spec set through
     `yass`, one document at a time rather than whole files, and builds what it
     says.
   - **Changing a spec** — the agent reads the documents carried inside the
     binary (`yass docs`), edits the spec files, and checks them with
     `yass validate` and `yass lint`.

3. **Review the spec first.** It is the contract the code is built against, so
   when something comes out wrong, the fix is usually a line in the spec — and
   the code follows.

## The CLI, for an agent arriving cold

Run `yass` with no arguments. It writes what the program is, what project the
current directory sits in, the notation a spec file uses, the commands that
read a spec set, and the flow that elicits a new one — enough to start without
opening a single file by hand.

The documents that define the language and how to write specs in it are
carried inside the binary and served by `yass docs`, so they travel with it
wherever it is installed. Read one when you are eliciting, writing, or changing
a spec; implementing against specs that already exist needs none of them.

## License

[AGPL-3.0](./LICENSE)
