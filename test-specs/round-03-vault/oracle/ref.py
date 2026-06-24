#!/usr/bin/env python3
"""Private reference implementation of the round-03 vault CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program 100/100. Independent of grade.py: it re-derives the same
contract directly from the spec rather than importing the simulators.

Usage:
    python3 ref.py certify   < records      -> verdict lines
    python3 ref.py report    < verdicts     -> five-line summary
    (missing / unknown subcommand)          -> usage on stderr, exit 2
"""

import sys

CLASS = {
    "V01": "MAJOR", "V02": "MINOR", "V03": "CRITICAL", "V04": "MINOR",
    "V05": "CRITICAL", "V06": "MINOR", "V07": "MAJOR", "V08": "MINOR",
    "V09": "MAJOR", "V10": "CRITICAL", "V11": "MINOR", "V12": "MAJOR",
    "V13": "CRITICAL", "V14": "MAJOR", "V15": "MINOR", "V16": "CRITICAL",
    "V17": "MAJOR", "V18": "MINOR",
}

MSG = {
    "V01": "bolt throw below 20 mm minimum",
    "V02": "bolt throw above 80 mm maximum",
    "V03": "wall thickness below 150 mm minimum",
    "V04": "wall thickness above 600 mm maximum",
    "V05": "relocker absent",
    "V06": "relocker count above 4 maximum",
    "V07": "drill rating below 30 minute minimum",
    "V08": "torch rating below 15 minute minimum",
    "V09": "time-lock below 48 hour minimum",
    "V10": "time-lock above 168 hour maximum",
    "V11": "service interval exceeded above 50000 cycles",
    "V12": "audit overdue beyond 365 days",
    "V13": "drill rating below torch rating",
    "V14": "bolt throw exceeds wall thickness",
    "V15": "audit overdue beyond 730 days",
    "V16": "service interval exceeded above 100000 cycles",
    "V17": "torch rating below 8 minute floor",
    "V18": "drill rating above 240 minute maximum",
}

PRECEDENCE = ["V05", "V13", "V08", "V01", "V17", "V03", "V11", "V10", "V14",
              "V07", "V02", "V16", "V09", "V06", "V12", "V18", "V04", "V15"]

CLASS_RANK = {"CRITICAL": 3, "MAJOR": 2, "MINOR": 1}

UP = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIG = set("0123456789")


def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def fields(rec):
    return [f for f in rec.split(" ") if f != ""]


def is_door_id(tok):
    if not (2 <= len(tok) <= 8):
        return False
    if tok[0] not in UP:
        return False
    return all((ch in UP) or (ch in DIG) for ch in tok)


def is_int(tok):
    if tok == "" or any(ch not in DIG for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


def defects(b, w, r, d, t, l, c, a):
    found = []
    if b < 20:
        found.append("V01")
    if b > 80:
        found.append("V02")
    if w < 150:
        found.append("V03")
    if w > 600:
        found.append("V04")
    if r < 1:
        found.append("V05")
    if r > 4:
        found.append("V06")
    if d < 30:
        found.append("V07")
    if t < 15:
        found.append("V08")
    if l < 48:
        found.append("V09")
    if l > 168:
        found.append("V10")
    if c > 50000:
        found.append("V11")
    if a > 365:
        found.append("V12")
    if d < t:
        found.append("V13")
    if b > w:
        found.append("V14")
    if a > 730:
        found.append("V15")
    if c > 100000:
        found.append("V16")
    if t < 8:
        found.append("V17")
    if d > 240:
        found.append("V18")
    return found


def governing(found):
    s = set(found)
    for code in PRECEDENCE:
        if code in s:
            return code
    return None


def certify(text, out, err):
    exit_code = 0
    for rec in split_records(text):
        f = fields(rec)
        if len(f) != 9:
            err.write("E10 malformed record: expected 9 fields, got %d\n" % len(f))
            exit_code = max(exit_code, 1)
            continue
        door = f[0]
        if not is_door_id(door):
            err.write("E15 bad door id: %s\n" % door)
            exit_code = max(exit_code, 1)
            continue
        bad = None
        for tok in f[1:9]:
            if not is_int(tok):
                bad = tok
                break
        if bad is not None:
            err.write("E20 bad number: %s\n" % bad)
            exit_code = max(exit_code, 1)
            continue
        b, w, r, d, t, l, c, a = (int(x) for x in f[1:9])
        found = defects(b, w, r, d, t, l, c, a)
        if not found:
            out.write("%s PASS\n" % door)
        else:
            code = governing(found)
            out.write("%s %s %s\n" % (door, code, MSG[code]))
            exit_code = max(exit_code, CLASS_RANK[CLASS[code]])
    return exit_code


def report(text, out, err):
    doors = passes = crit = maj = minr = 0
    for rec in split_records(text):
        f = fields(rec)
        if not f:
            continue
        doors += 1
        verdict = f[1] if len(f) >= 2 else ""
        if verdict == "PASS":
            passes += 1
        elif verdict in CLASS:
            cls = CLASS[verdict]
            if cls == "CRITICAL":
                crit += 1
            elif cls == "MAJOR":
                maj += 1
            else:
                minr += 1
    out.write("DOORS %d\n" % doors)
    out.write("PASS %d\n" % passes)
    out.write("CRITICAL %d\n" % crit)
    out.write("MAJOR %d\n" % maj)
    out.write("MINOR %d\n" % minr)
    return 0


def main():
    sub = sys.argv[1] if len(sys.argv) >= 2 else None
    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
    out = sys.stdout
    err = sys.stderr
    if sub == "certify":
        code = certify(data, out, err)
    elif sub == "report":
        code = report(data, out, err)
    else:
        err.write("usage: vault {certify|report}\n")
        code = 2
    out.flush()
    err.flush()
    sys.exit(code)


if __name__ == "__main__":
    main()
