# yass

**Yet Another Spec Syntax** — a YAML-based specification language for
AI-agent-driven development. Specs capture desired behavior and are consumed to
drive TDD and implementation generation.

> Early work in progress.

Yass is for people and coding agents who need an explicit, reviewable contract
before implementing or changing behavior. The language describes inputs,
returns, failures, side effects, and invariants in YAML. The CLI finds and serves
those specifications, resolves references, and checks structure and graph hygiene.
It reads spec sets without changing them; writing specs and implementing code are
tasks for you and your agent.

Start with [installation](installation.md) and the [quick start](quick-start.md),
then use the [CLI guide](cli.md) to navigate a project. The chapter on
[developing with agents](agent-workflows.md) shows how to elicit and review a new spec,
implement an approved one, and adopt Yass in an existing codebase.

The [language reference](reference.md), [authoring guidance](guidance.md), and
[brainstorming flow](brainstorming.md) publish the authoritative source documents
that the CLI embeds and serves with `yass docs`. The [JSON Schema](schema.md)
supports editor validation. Tooling checks do not establish that a contract
captures the intended behavior or that its implementation is correct.

- [Repository](https://github.com/shakefu/yass)
- [Releases](https://github.com/shakefu/yass/releases)
