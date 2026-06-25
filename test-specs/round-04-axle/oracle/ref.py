#!/usr/bin/env python3
"""Private reference implementation of the round-04 axle CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program to full marks. Implemented INDEPENDENTLY of grade.py: it
re-derives the contract straight from the spec rather than importing the
oracle's simulators, so a shared coding bug cannot pass both at once.

Domain: trackside railcar journal-bearing wayside hotbox-detector audit.

Usage:
    python3 ref.py audit   < records      -> verdict lines
    python3 ref.py roster  < verdicts     -> five-line summary
    (missing / unknown subcommand)        -> usage on stderr, exit 2
"""

import sys

# --- bearing defect registry (axle.shared@axle.defects) -----------------------
# class of each code
KLASS = {
    "H01": "MAJOR", "H02": "MINOR", "H03": "CRITICAL", "H04": "MAJOR",
    "H05": "MAJOR", "H06": "CRITICAL", "H07": "MINOR", "H08": "MAJOR",
    "H09": "CRITICAL", "H10": "MINOR", "H11": "MINOR", "H12": "MAJOR",
}

TEXT = {
    "H01": "bearing temp above 80 absolute alarm",
    "H02": "bearing temp above 60 warning",
    "H03": "differential above 70 hotbox",
    "H04": "differential above 50 alarm",
    "H05": "side-to-side differential above 40",
    "H06": "bearing temp above 100 burnoff",
    "H07": "aged bearing side differential above 25",
    "H08": "wheel impact above 140 kN alarm",
    "H09": "wheel impact above 200 kN critical",
    "H10": "wheel impact above 90 kN warning",
    "H11": "load above 130 tonne limit",
    "H12": "reprofile overdue beyond 280",
}

# governing precedence, highest first (deliberately non-monotonic)
ORDER = ["H06", "H09", "H03", "H08", "H01", "H05", "H04",
         "H12", "H02", "H11", "H10", "H07"]

RANK = {"CRITICAL": 3, "MAJOR": 2, "MINOR": 1}

UPPER = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIGIT = set("0123456789")


# --- segmentation (axle.shared@axle.lines) ------------------------------------

def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def split_fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


# --- record format (axle.shared@axle.record) ----------------------------------

def good_axle_id(tok):
    if not (3 <= len(tok) <= 10):
        return False
    if tok[0] not in UPPER:
        return False
    return all((ch in UPPER) or (ch in DIGIT) for ch in tok)


def good_int(tok):
    if tok == "" or any(ch not in DIGIT for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


# --- defect conditions --------------------------------------------------------

def fired_codes(bt, amb, mate, impact, load, mileage):
    hits = []
    if bt > 80:
        hits.append("H01")
    if bt > 60:
        hits.append("H02")
    if bt - amb > 70:
        hits.append("H03")
    if bt - amb > 50:
        hits.append("H04")
    if bt - mate > 40:
        hits.append("H05")
    if bt > 100:
        hits.append("H06")
    if mileage > 200 and bt - mate > 25:   # H07 inner guard: mileage > 200
        hits.append("H07")
    if impact > 140:
        hits.append("H08")
    if impact > 200:
        hits.append("H09")
    if impact > 90:
        hits.append("H10")
    if load > 130:
        hits.append("H11")
    if mileage > 280:
        hits.append("H12")
    return hits


def governing_code(hits):
    present = set(hits)
    for code in ORDER:
        if code in present:
            return code
    return None


# --- audit subcommand ---------------------------------------------------------

def audit(text, out, err):
    worst = 0
    for rec in split_records(text):
        fields = split_fields(rec)
        if len(fields) != 7:
            err.write("E10 malformed record: expected 7 fields, got %d\n" % len(fields))
            worst = max(worst, 1)
            continue
        axle_id = fields[0]
        if not good_axle_id(axle_id):
            err.write("E15 bad axle id: %s\n" % axle_id)
            worst = max(worst, 1)
            continue
        offending = None
        for tok in fields[1:7]:
            if not good_int(tok):
                offending = tok
                break
        if offending is not None:
            err.write("E20 bad number: %s\n" % offending)
            worst = max(worst, 1)
            continue
        bt, amb, mate, impact, load, mileage = (int(x) for x in fields[1:7])
        hits = fired_codes(bt, amb, mate, impact, load, mileage)
        if not hits:
            out.write("%s CLEAR\n" % axle_id)
        else:
            code = governing_code(hits)
            out.write("%s %s %s\n" % (axle_id, code, TEXT[code]))
            worst = max(worst, RANK[KLASS[code]])
    return worst


# --- roster subcommand --------------------------------------------------------

def _bump(tally, code):
    cls = KLASS.get(code)
    if cls == "CRITICAL":
        tally["CRITICAL"] += 1
    elif cls == "MAJOR":
        tally["MAJOR"] += 1
    elif cls == "MINOR":
        tally["MINOR"] += 1
    # codes outside the registry contribute to no class


def roster(text, out, err):
    tally = {"AXLES": 0, "CLEAR": 0, "CRITICAL": 0, "MAJOR": 0, "MINOR": 0}
    for rec in split_records(text):
        fields = split_fields(rec)
        if not fields:
            continue
        tally["AXLES"] += 1
        if fields[0] == "RECHECK":
            # RECHECK <id> <flag> <bt> <amb> <mate> <impact> <load> <mileage>
            flag = fields[2] if len(fields) >= 3 else ""
            if flag == "Y" and len(fields) >= 9 \
                    and all(good_int(t) for t in fields[3:9]):
                bt, amb, mate, impact, load, mileage = (int(t) for t in fields[3:9])
                hits = fired_codes(bt, amb, mate, impact, load, mileage)
                code = governing_code(hits)
                if code is None:
                    tally["CLEAR"] += 1
                else:
                    _bump(tally, code)
            else:
                # flagged N (or not re-derivable) -> tally clear without re-derivation
                tally["CLEAR"] += 1
        else:
            # verdict line: trust the stated second field, do not re-validate
            verdict = fields[1] if len(fields) >= 2 else ""
            if verdict == "CLEAR":
                tally["CLEAR"] += 1
            else:
                _bump(tally, verdict)
    out.write("AXLES %d\n" % tally["AXLES"])
    out.write("CLEAR %d\n" % tally["CLEAR"])
    out.write("CRITICAL %d\n" % tally["CRITICAL"])
    out.write("MAJOR %d\n" % tally["MAJOR"])
    out.write("MINOR %d\n" % tally["MINOR"])
    return 0


# --- dispatch -----------------------------------------------------------------

def main():
    sub = sys.argv[1] if len(sys.argv) >= 2 else None
    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
    out = sys.stdout
    err = sys.stderr
    if sub == "audit":
        code = audit(data, out, err)
    elif sub == "roster":
        code = roster(data, out, err)
    else:
        err.write("usage: axle {audit|roster}\n")
        code = 2
    out.flush()
    err.flush()
    sys.exit(code)


if __name__ == "__main__":
    main()
