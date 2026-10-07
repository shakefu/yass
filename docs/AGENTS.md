# Documentation site

- `book.toml`: mdBook configuration; `src/SUMMARY.md`: chapter navigation.
- `src/reference.md`, `src/guidance.md`, `src/brainstorming.md`: `{{#include}}`
  stubs over the authoritative documents in `../context/` — edit the sources,
  never write copies here.
- `src/introduction.md`, `src/schema.md`: hand-written site pages.
- Generated `book/` output is ignored. The deploy workflow copies
  `yass.v1.schema.json` to the site root, publishing it at
  `https://shakefu.github.io/yass/v1.schema.json`.
