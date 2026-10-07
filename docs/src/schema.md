# JSON Schema

`yass.v1.schema.json` is the per-document editor-validation schema for yass v1
files. Each YAML document in a `.yass.yaml` stream validates as a preamble, a
spec, or a design. Stream-level rules (exactly one leading preamble, name
uniqueness, ref resolution, relation target kinds) are out of scope and remain
in `yass validate`.

The schema is published at:

<https://shakefu.github.io/yass/v1.schema.json>

Point yaml-language-server at it with the modeline every `.yass.yaml` file
carries as its first line:

```yaml
# yaml-language-server: $schema=https://shakefu.github.io/yass/v1.schema.json
```

Source: [`yass.v1.schema.json`](https://github.com/shakefu/yass/blob/main/yass.v1.schema.json)
in the repository root.
