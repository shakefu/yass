# Quick Start

Use yass to describe behavior, then give an agent the relevant obligations. The
CLI reads and checks specs; your agent writes tests and implementation code.

## Install and orient yourself

Install the latest release with the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh | sh
```

The default destination is `/usr/local/bin` when writable, otherwise
`~/.local/bin`; add the destination to your shell's `PATH` if needed.
For Homebrew, platform support, custom destinations, or building from source, see
[Installation](installation.md).

```sh
yass --version
yass
```

`--version` reports the CLI and language versions. Running `yass` gives an overview
of the notation and commands. It also identifies the current project, when one
exists; an overview outside a project still introduces the tool.

Use version 0.4.3 or newer for these examples. Older versions such as 0.4.2
expect the previous schema URL and report a modeline lint warning for the
current Pages URL. See [Updating](installation.md#updating-an-installed-binary)
if your installed version is older.

## Create a small project

Create a new directory and its `root.yass.yaml`:

```sh
mkdir greeting-spec
cd greeting-spec
cat > root.yass.yaml <<'YAML'
# yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json
---
description: A greeting function that returns a message for a supplied name.
version: v1
---
spec: Greeting
INPUT:
- MUST: accept a nonempty UTF-8 name string
RETURN:
- MUST: return Hello followed by one space, the supplied name, and an exclamation mark
SIDE-EFFECT:
- MUST-NOT: write to standard output or modify a file
YAML
```

The first YAML document describes the file. The second names a spec and groups its
obligations into slots. `root.yass.yaml` defines the project root and contains
exactly one spec. This example specifies a function with a stated input contract;
it does not yet define behavior for calls outside that contract.

## Read and check the spec

Run these commands from `greeting-spec`:

```sh
yass root
yass list
yass query root@Greeting
yass query root@Greeting::RETURN
yass find 'supplied name'
yass validate
yass lint
```

`root` prints the absolute path of `greeting-spec`. `list` prints two tab-separated
records: the file `root` and the spec `root@Greeting`. `query` emits YAML, and the
second query emits only the `RETURN` slot. `find` locates the return obligation
with the target `root@Greeting::RETURN`.

Both `validate` and `lint` should produce no output and exit with status `0`.
Validation checks the language's structural rules; lint checks project hygiene.
Neither proves that the prose is complete or that an implementation satisfies it.

Targets omit the `.yass.yaml` suffix. `yass query root` reads the file's preamble;
`yass query root@Greeting` reads the named spec. See the [CLI Guide](cli.md) for
target paths, references, filtering, and exit statuses.

## Give the agent a bounded task

For an existing spec, a useful starting instruction is:

> Use `yass` to read this project's specs. Start with `yass` and `yass list`, then
> read `root@Greeting`. Implement the greeting function with tests for its stated
> input contract and return value. Raise any missing behavior before extending the
> contract.

For a new feature, use `yass docs brainstorming` to elicit behavior before writing
the spec. Use `yass docs reference` and `yass docs guidance` when authoring or
changing it. These documents are embedded in the binary and work outside a
project too.

Continue with the worked example in [Developing with agents](agent-workflows.md)
to grow the spec into a project with references and shared contracts.
