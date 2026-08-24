# CLAUDE.md

## Read nothing unless explicitly told to

Do not read anything unless the user has told you, in this session, to read
that specific thing.

Nothing is exempt. Not the code. Not the specs. Not the docs, the README, the
config, or this repository's `.yass.yaml` files. Not directory listings or
searches (`ls`, `find`, `grep`, glob, subagents). Not git history, diffs, or
branches. Not pull requests, issues, or CI logs. Not the web.

There is no "quick peek" exception. A file being obviously relevant is not
permission to open it. Permission is per-item and does not generalize: being
told to read one file does not authorize its neighbours, its imports, or the
directory it sits in.

If you need something in order to proceed, say what you need and why, then
stop and wait. Resolve ambiguity by asking, never by looking.

## The yass CLI is already built

A working `yass` binary exists at `cli/yass` on this branch
(`claude/build-prompt-review-lws9bb-ziwodl`). It is compiled and verified.

- Do not rebuild it. Do not implement it. It is done.
- **Do not run it until the user tells you to.** Having the binary is not
  permission to invoke it — running it against the tree is a way of reading
  the tree, and the policy above applies.

When the user does ask for a run, `cli/yass --help` lists the subcommands.
