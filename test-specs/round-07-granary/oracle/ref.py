#!/usr/bin/env python3
"""Private reference implementation of the round-07 `granary` CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program to full marks. Implemented INDEPENDENTLY of grade.py: it
re-derives the contract straight from PLAN.md, and its leftover distribution is
an explicit repeated-selection scan (pick the winning claim `leftover` times
with hand-written comparisons) rather than the oracle's sort-key formulation,
so a shared coding bug cannot pass both at once.

Domain: communal granary allotment. `allot` distributes a pool over claims by
largest remainder: per-claim integer floor, remainders as exact integer
numerators, exactly `leftover` extra units distributed by remainder desc /
base asc / later-input-position-first. `tally` consumes allot's output.

Usage:
    python3 ref.py allot  < manifest  -> GET lines + REMAIN, exit 0/1 (gate: 2)
    python3 ref.py tally  < lines     -> LINES/CLAIMS/UNITS/REMAIN, exit 0
                                         (zero-byte input -> `EMPTY FEED`)
    (missing / unknown / wrong-case / bare `-` subcommand) -> usage, exit 2
"""

import sys

UPPER = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIGIT = set("0123456789")


# --- segmentation (granary.shared@granary.lines) ---------------------------------

def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def split_fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


# --- grammar predicates ------------------------------------------------------------

def good_id(tok):
    if not (1 <= len(tok) <= 6):
        return False
    if tok[0] not in UPPER:
        return False
    return all((ch in UPPER) or (ch in DIGIT) for ch in tok)


def good_int(tok):
    if tok == "" or any(ch not in DIGIT for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


# --- allot subcommand ----------------------------------------------------------------

def allot(text, out, err):
    records = split_records(text)

    # pool-header gate: the FIRST record must be `POOL <units>`
    header_ok = False
    pool = 0
    if records:
        hf = split_fields(records[0])
        if len(hf) == 2 and hf[0] == "POOL" and good_int(hf[1]):
            header_ok = True
            pool = int(hf[1])
    if not header_ok:
        err.append("E40 bad pool header")
        return 2

    ids = []
    weights = []
    rejected = False
    for rec in records[1:]:
        fields = split_fields(rec)
        n = len(fields)
        if n == 0:
            err.append("E10 malformed record: empty line")
            rejected = True
            continue
        if fields[0] != "CLAIM":
            err.append("E25 unknown operation: " + fields[0])
            rejected = True
            continue
        if n != 3:
            err.append("E10 malformed record: CLAIM expects 3 fields, got " + str(n))
            rejected = True
            continue
        if not good_id(fields[1]):
            err.append("E15 bad claim id: " + fields[1])
            rejected = True
            continue
        if not good_int(fields[2]):
            err.append("E20 bad number: " + fields[2])
            rejected = True
            continue
        ids.append(fields[1])
        weights.append(int(fields[2]))

    # AllotmentProcedure, step by step
    count = len(ids)
    total = 0
    for w in weights:                       # step 1: accumulate from zero
        total += w

    units = [0] * count
    remain = pool
    if total > 0:
        base = [0] * count
        rem = [0] * count
        for i in range(count):              # step 3: floor + integer remainder
            base[i] = (pool * weights[i]) // total
            rem[i] = (pool * weights[i]) - (base[i] * total)
        leftover = pool                     # step 4
        for b in base:
            leftover -= b
        extra = [0] * count
        for _ in range(leftover):           # step 5: repeated selection
            best = -1
            for i in range(count):
                if extra[i]:
                    continue
                if best < 0:
                    best = i
                    continue
                if rem[i] > rem[best]:
                    best = i
                elif rem[i] == rem[best]:
                    if base[i] < base[best]:
                        best = i
                    elif base[i] == base[best] and i > best:
                        best = i
            extra[best] = 1
        allotted = 0
        for i in range(count):              # step 6
            units[i] = base[i] + extra[i]
            allotted += units[i]
        remain = pool - allotted

    for i in range(count):
        out.append(ids[i] + " GET " + str(units[i]))
    out.append("REMAIN " + str(remain))
    return 1 if rejected else 0


# --- tally subcommand ------------------------------------------------------------------

def tally(text, out):
    if text == "":                          # EmptyFeed
        out.append("EMPTY FEED")
        return 0
    lines = 0
    claims = 0
    units = 0
    remain = 0
    for rec in split_records(text):
        lines += 1
        fields = split_fields(rec)
        if len(fields) >= 2 and fields[1] == "GET":
            claims += 1
            if len(fields) >= 3 and good_int(fields[2]):
                units += int(fields[2])
        elif len(fields) == 2 and fields[0] == "REMAIN" and good_int(fields[1]):
            remain += int(fields[1])
    out.append("LINES " + str(lines))
    out.append("CLAIMS " + str(claims))
    out.append("UNITS " + str(units))
    out.append("REMAIN " + str(remain))
    return 0


# --- dispatch ------------------------------------------------------------------------------

def main():
    sub = sys.argv[1] if len(sys.argv) > 1 else None
    if sub != "allot" and sub != "tally":
        sys.stderr.write("usage: granary {allot|tally}\n")
        sys.exit(2)

    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
    out = []
    err = []
    if sub == "allot":
        code = allot(data, out, err)
    else:
        code = tally(data, out)

    sys.stdout.buffer.write(
        "".join(s + "\n" for s in out).encode("utf-8", "surrogateescape"))
    sys.stderr.buffer.write(
        "".join(s + "\n" for s in err).encode("utf-8", "surrogateescape"))
    sys.exit(code)


if __name__ == "__main__":
    main()
