#!/usr/bin/env python3
"""Private reference implementation of the round-06 `kiln` CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program to full marks. Implemented INDEPENDENTLY of grade.py: it
re-derives the contract straight from PLAN.md as straight-line per-stage
statements and if-ladders rather than importing the oracle's data-driven op
table, so a shared coding bug cannot pass both at once.

Domain: kiln-firing controller. Each accepted `LOAD <id> <base>` record runs
the firing procedure -- SOAK, then SEAL, then VENT, then RAMP -- logging one
stage line per stage, then a grade line on the final firing index.

Usage:
    python3 ref.py fire    < manifest  -> stage + grade lines, exit = max contrib
    python3 ref.py report  < lines     -> five-line tally, exit 0
    (missing / unknown / wrong-case / bare `-` subcommand) -> usage, exit 2
"""

import sys

UPPER = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIGIT = set("0123456789")


# --- segmentation (kiln.shared@kiln.lines) --------------------------------------

def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def split_fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


# --- record-format predicates (kiln.shared@kiln.record) --------------------------

def good_id(tok):
    if not (2 <= len(tok) <= 8):
        return False
    if tok[0] not in UPPER:
        return False
    return all((ch in UPPER) or (ch in DIGIT) for ch in tok)


def good_int(tok):
    if tok == "" or any(ch not in DIGIT for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


# --- fire subcommand -------------------------------------------------------------

def fire(text, out, err):
    worst = 0
    for rec in split_records(text):
        fields = split_fields(rec)
        n = len(fields)
        if n == 0:
            err.append("E10 malformed record: empty line")
            if worst < 1:
                worst = 1
            continue
        if fields[0] != "LOAD":
            err.append("E25 unknown operation: " + fields[0])
            if worst < 1:
                worst = 1
            continue
        if n != 3:
            err.append("E10 malformed record: LOAD expects 3 fields, got " + str(n))
            if worst < 1:
                worst = 1
            continue
        lid = fields[1]
        if not good_id(lid):
            err.append("E15 bad load id: " + lid)
            if worst < 1:
                worst = 1
            continue
        if not good_int(fields[2]):
            err.append("E20 bad number: " + fields[2])
            if worst < 1:
                worst = 1
            continue
        base = int(fields[2])

        # the firing procedure, straight-line, in the mandated order:
        soak = base * 3
        out.append(lid + " SOAK " + str(soak))
        seal = soak - 220
        out.append(lid + " SEAL " + str(seal))
        vent = seal + 35
        out.append(lid + " VENT " + str(vent))
        ramp = vent * 2
        out.append(lid + " RAMP " + str(ramp))

        # grade the final firing index (kiln.shared@kiln.grade)
        final = ramp
        if final > 2000:
            out.append(lid + " G90 overfired above 2000")
            if worst < 3:
                worst = 3
        elif final < 500:
            out.append(lid + " G10 underfired below 500")
            if worst < 2:
                worst = 2
        else:
            out.append(lid + " DONE " + str(final))
    return worst


# --- report subcommand ------------------------------------------------------------

def report(text, out):
    lines = 0
    stages = 0
    dones = 0
    unders = 0
    overs = 0
    for rec in split_records(text):
        lines += 1
        fields = split_fields(rec)
        if len(fields) < 2:
            continue
        token = fields[1]
        if token == "RAMP" or token == "SEAL" or token == "SOAK" or token == "VENT":
            stages += 1
        elif token == "DONE":
            dones += 1
        elif token == "G10":
            unders += 1
        elif token == "G90":
            overs += 1
    out.append("LINES " + str(lines))
    out.append("STAGES " + str(stages))
    out.append("DONE " + str(dones))
    out.append("UNDERFIRED " + str(unders))
    out.append("OVERFIRED " + str(overs))
    return 0


# --- dispatch ----------------------------------------------------------------------

def main():
    sub = sys.argv[1] if len(sys.argv) > 1 else None
    if sub != "fire" and sub != "report":
        sys.stderr.write("usage: kiln {fire|report}\n")
        sys.exit(2)

    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
    out = []
    err = []
    if sub == "fire":
        code = fire(data, out, err)
    else:
        code = report(data, out)

    sys.stdout.buffer.write(
        "".join(s + "\n" for s in out).encode("utf-8", "surrogateescape"))
    sys.stderr.buffer.write(
        "".join(s + "\n" for s in err).encode("utf-8", "surrogateescape"))
    sys.exit(code)


if __name__ == "__main__":
    main()
