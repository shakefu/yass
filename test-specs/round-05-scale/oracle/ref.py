#!/usr/bin/env python3
"""Private reference implementation of the round-05 `scale` CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program to full marks. Implemented INDEPENDENTLY of grade.py: it
re-derives the contract straight from PLAN.md as a plain if-ladder rather than
importing the oracle's data-driven simulators, so a shared coding bug cannot
pass both at once.

Domain: weighbridge (truck-scale) ticket session. A vehicle must be tared
(empty-weight zeroed) before it can be weighed; the weigh derives a net weight
and classifies overload.

Usage:
    python3 ref.py run    < operations   -> ticket / error lines, exit = max sev
    python3 ref.py audit  < tickets      -> five-line class tally, exit 0
    (missing / unknown / wrong-case / bare `-` subcommand) -> usage, exit 2
"""

import sys

UPPER = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIGIT = set("0123456789")


# --- segmentation (scale.shared@scale.lines) ----------------------------------

def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def split_fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


# --- record-format predicates (scale.shared@scale.record) ---------------------

def good_id(tok):
    if not (3 <= len(tok) <= 10):
        return False
    if tok[0] not in UPPER:
        return False
    return all((ch in UPPER) or (ch in DIGIT) for ch in tok)


def good_int(tok):
    if tok == "" or any(ch not in DIGIT for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


# --- defect registry (scale.shared@scale.defects), first-match precedence ------

def verdict(net, gross):
    """Return (code, message, severity_rank) or None for CLEAR.
    Precedence W01 > W02 > W03 > W04; W04 inner guard gross > 50000."""
    if net > 44000:
        return ("W01", "critical overload above 44000", 3)
    if net > 40000:
        return ("W02", "major overload above 40000", 2)
    if net > 36000:
        return ("W03", "minor overload above 36000", 1)
    if gross > 50000 and net > 30000:
        return ("W04", "heavy vehicle net above 30000", 1)
    return None


# --- run subcommand -----------------------------------------------------------

def run(text, out, err):
    tare = {}
    worst = 0
    for rec in split_records(text):
        fields = split_fields(rec)
        n = len(fields)
        if n == 0:
            err.write("E10 malformed record: empty line\n")
            worst = max(worst, 1)
            continue
        keyword = fields[0]
        if keyword != "TARE" and keyword != "WEIGH":
            err.write("E25 unknown operation: %s\n" % keyword)
            worst = max(worst, 1)
            continue
        if n != 3:
            err.write("E10 malformed record: %s expects 3 fields, got %d\n"
                      % (keyword, n))
            worst = max(worst, 1)
            continue
        vid = fields[1]
        number = fields[2]
        if not good_id(vid):
            err.write("E15 bad vehicle id: %s\n" % vid)
            worst = max(worst, 1)
            continue
        if not good_int(number):
            err.write("E20 bad number: %s\n" % number)
            worst = max(worst, 1)
            continue
        value = int(number)
        if keyword == "TARE":
            tare[vid] = value
            out.write("%s TARE %d\n" % (vid, value))
            continue
        # WEIGH
        if vid not in tare:
            err.write("E40 weigh before tare: %s\n" % vid)
            worst = max(worst, 1)
            continue
        if value < tare[vid]:
            err.write("E50 negative net: %s\n" % vid)
            worst = max(worst, 1)
            continue
        net = value - tare[vid]
        v = verdict(net, value)
        if v is None:
            out.write("%s NET %d\n" % (vid, net))
        else:
            code, message, rank = v
            out.write("%s %s %s\n" % (vid, code, message))
            worst = max(worst, rank)
    return worst


# --- audit subcommand (consumer) ----------------------------------------------

CLASS_OF_TOKEN = {
    "TARE": "CLEAR", "NET": "CLEAR",
    "W01": "CRITICAL", "W02": "MAJOR", "W03": "MINOR", "W04": "MINOR",
}


def audit(text, out, err):
    tickets = 0
    clear = critical = major = minor = 0

    def bump(klass):
        nonlocal clear, critical, major, minor
        if klass == "CLEAR":
            clear += 1
        elif klass == "CRITICAL":
            critical += 1
        elif klass == "MAJOR":
            major += 1
        elif klass == "MINOR":
            minor += 1

    for rec in split_records(text):
        fields = split_fields(rec)
        if not fields:
            continue                       # OVERRIDE: blank line skipped, not counted
        tickets += 1
        if fields[0] == "REWEIGH":
            flag = fields[2] if len(fields) >= 3 else ""
            if len(fields) == 5 and flag == "Y" \
                    and good_int(fields[3]) and good_int(fields[4]):
                gross = int(fields[3])
                t = int(fields[4])
                v = verdict(gross - t, gross)
                if v is None:
                    bump("CLEAR")
                else:
                    bump(CLASS_OF_TOKEN[v[0]])
            else:
                bump("CLEAR")              # not-flagged / out-of-contract residual
        else:
            token = fields[1] if len(fields) >= 2 else ""
            klass = CLASS_OF_TOKEN.get(token)
            if klass is not None:
                bump(klass)
            # unrecognized verdict token: counted in TICKETS, no class bump
    out.write("TICKETS %d\n" % tickets)
    out.write("CLEAR %d\n" % clear)
    out.write("CRITICAL %d\n" % critical)
    out.write("MAJOR %d\n" % major)
    out.write("MINOR %d\n" % minor)
    return 0


# --- dispatch -----------------------------------------------------------------

def main():
    sub = sys.argv[1] if len(sys.argv) >= 2 else None
    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
    out = sys.stdout
    err = sys.stderr
    if sub == "run":
        code = run(data, out, err)
    elif sub == "audit":
        code = audit(data, out, err)
    else:
        err.write("usage: scale {run|audit}\n")
        code = 2
    out.flush()
    err.flush()
    sys.exit(code)


if __name__ == "__main__":
    main()
