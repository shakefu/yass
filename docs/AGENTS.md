# Documentation site

- `book.toml`: mdBook configuration; `src/SUMMARY.md`: chapter navigation.
- `src/reference.md`, `src/guidance.md`, `src/brainstorming.md`: `{{#include}}`
  stubs over the authoritative documents in `../context/` — edit the sources,
  never write copies here.
- `src/installation.md`, `src/quick-start.md`, `src/cli.md`: installation and
  command guides; verify examples against the CLI specs and implementation.
- `src/agent-workflows.md`: prompts and worked examples grounded in the
  authoritative guidance and brainstorming flow, including their approval gates.
- `src/introduction.md`, `src/schema.md`, `src/contributing.md`: site overview,
  schema usage, and preview/update instructions.
- `mdbook build docs`: build from the repository root; pull requests build in
  `.github/workflows/docs.yaml`, while only main pushes/manual runs deploy.
- Generated `book/` output is ignored. The deploy workflow copies
  `yass.v1.schema.json` to the site root, publishing it at
  `https://shakefu.github.io/yass/v1.schema.json`.
