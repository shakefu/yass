# Task

Your workspace contains one or more specification files with the extension
`.yass.yaml`. They define the behavior of a command-line program.

Read every `.yass.yaml` file in the workspace, then implement a single
command-line program that satisfies the specification(s) as completely and
precisely as you can.

## Implementation rules

- Choose ONE language: Go, Rust, or Python (all are installed). Record which.
- The program is a command-line tool. Unless a spec says otherwise, it reads
  from standard input and writes to standard output and standard error, and its
  process exit code is part of its contract.
- Implement every obligation you can find in the spec(s), including negative
  obligations (things the program MUST NOT do) and conditional ones.
- Do NOT modify the `.yass.yaml` spec files.

## Required output files

Write these two files into the workspace before you finish:

1. `HOWTORUN.txt` — plain text, machine-followable:
   - any build/compile commands needed (each on its own line), then
   - a line beginning exactly with `RUN: ` followed by the single shell command
     that runs the finished program reading from standard input
     (for example: `RUN: ./double` or `RUN: python3 double.py`).
   - If the spec defines subcommands, the program will be invoked as
     `<RUN command> <subcommand>` (the subcommand appended as the program's
     first argument). Your `RUN: ` line must therefore give the base command
     only, WITHOUT any subcommand of its own.
   - The program must be runnable from the workspace root after the build steps.

2. `NOTES.md` — your implementation notes:
   - which language you chose and why;
   - any part of the specification that was ambiguous, underspecified,
     contradictory, or that you had to guess at — be specific about which
     obligation and what you assumed;
   - anything you could not implement and why.

Be thorough and literal. Where the spec gives an exact string, exit code, or
ordering, match it exactly.
