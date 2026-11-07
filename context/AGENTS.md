# Context

Authoritative language documents, served by the CLI as `yass docs NAME` and read in
paths (`cli/10-docs.yass.yaml`, *Corpus*). Each stays under 200 lines and opens and
closes with its path line; edit sources here, then run `../script/sync-docs`.

- `AUTHORING.md`, `DOCUMENTS.md`, `OBLIGATIONS.md`, `REFERENCES.md`, `COVERAGE.md`:
  the write path, in that order: authoring rules, file and document format, slots and
  obligations, references and resolution, the case-coverage and implementer passes.
- `BRAINSTORMING.md`: the dialogue an agent runs to elicit a spec set from a user
  before one exists; read before the write path.
- `IMPLEMENTING.md`: the implement path, alone: reading an approved set into code and
  tests.
