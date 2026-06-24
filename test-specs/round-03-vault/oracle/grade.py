#!/usr/bin/env python3
"""Private oracle for round-03 (vault time-lock door certification).

NEVER copied into an agent workspace. Black-box grades a candidate CLI by
byte-exact comparison of stdout / stderr / exit code on three axes:

  1. certify with canonical door-inspection input (clean, single-defect-per-class
     message exactness, multi-defect precedence, cross-field defects, processing
     errors E10/E15/E20 in priority order, off-spec segmentation, exit ladder).
  2. report with canonical certify output (clean, mixed-class, all-pass, empty).
  3. chained certify | report threaded through the candidate's OWN certify
     stdout, compared to the canonical chain.

The embedded simulate_certify / simulate_report ARE the authoritative reference.
`--self-check` runs SELFTEST (hand-written expectations + chain consistency) and
prints "SELFTEST OK". `--cmd '<run command>'` grades that command and prints
"SCORE: passed/total".
"""

import argparse
import subprocess
import sys

# --- defect registry (the structured-obligation cluster under test) ----------

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

# highest precedence first; deliberately non-monotonic vs severity and code order
PRECEDENCE = ["V05", "V13", "V08", "V01", "V17", "V03", "V11", "V10", "V14",
              "V07", "V02", "V16", "V09", "V06", "V12", "V18", "V04", "V15"]

CLASS_RANK = {"CRITICAL": 3, "MAJOR": 2, "MINOR": 1}

USAGE = "usage: vault {certify|report}\n"

_UP = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DIG = set("0123456789")


# --- shared helpers -----------------------------------------------------------

def _split_records(stdin):
    if stdin == "":
        return []
    if stdin.endswith("\n"):
        stdin = stdin[:-1]
    return stdin.split("\n")


def _fields(line):
    return [f for f in line.split(" ") if f != ""]


def _join(lines):
    return "".join(s + "\n" for s in lines)


def _is_int(tok):
    if tok == "":
        return False
    for ch in tok:
        if ch not in _DIG:
            return False
    if tok == "0":
        return True
    return tok[0] != "0"


def _is_door_id(tok):
    if not (2 <= len(tok) <= 8):
        return False
    if tok[0] not in _UP:
        return False
    for ch in tok:
        if ch not in _UP and ch not in _DIG:
            return False
    return True


def _triggered(b, w, r, d, t, l, c, a):
    out = []
    if b < 20:
        out.append("V01")
    if b > 80:
        out.append("V02")
    if w < 150:
        out.append("V03")
    if w > 600:
        out.append("V04")
    if r < 1:
        out.append("V05")
    if r > 4:
        out.append("V06")
    if d < 30:
        out.append("V07")
    if t < 15:
        out.append("V08")
    if l < 48:
        out.append("V09")
    if l > 168:
        out.append("V10")
    if c > 50000:
        out.append("V11")
    if a > 365:
        out.append("V12")
    if d < t:
        out.append("V13")
    if b > w:
        out.append("V14")
    if a > 730:
        out.append("V15")
    if c > 100000:
        out.append("V16")
    if t < 8:
        out.append("V17")
    if d > 240:
        out.append("V18")
    return out


def _governing(trig):
    s = set(trig)
    for code in PRECEDENCE:
        if code in s:
            return code
    return None


# --- authoritative reference simulators ---------------------------------------

def simulate_certify(stdin):
    out = []
    err = []
    ranks = [0]
    for rec in _split_records(stdin):
        f = _fields(rec)
        if len(f) != 9:
            err.append("E10 malformed record: expected 9 fields, got %d" % len(f))
            ranks.append(1)
            continue
        door = f[0]
        if not _is_door_id(door):
            err.append("E15 bad door id: %s" % door)
            ranks.append(1)
            continue
        nums = f[1:9]
        bad = None
        for tok in nums:
            if not _is_int(tok):
                bad = tok
                break
        if bad is not None:
            err.append("E20 bad number: %s" % bad)
            ranks.append(1)
            continue
        b, w, r, d, t, l, c, a = (int(x) for x in nums)
        trig = _triggered(b, w, r, d, t, l, c, a)
        if not trig:
            out.append("%s PASS" % door)
            ranks.append(0)
        else:
            code = _governing(trig)
            out.append("%s %s %s" % (door, code, MSG[code]))
            ranks.append(CLASS_RANK[CLASS[code]])
    return (_join(out), _join(err), max(ranks))


def simulate_report(stdin):
    doors = passes = crit = maj = minr = 0
    for rec in _split_records(stdin):
        f = _fields(rec)
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
    out = ["DOORS %d" % doors, "PASS %d" % passes, "CRITICAL %d" % crit,
           "MAJOR %d" % maj, "MINOR %d" % minr]
    return (_join(out), "", 0)


SIM = {"certify": simulate_certify, "report": simulate_report}


# --- batches ------------------------------------------------------------------

# certify: (name, stdin)
CERT_BATCHES = [
    ("cert_empty", ""),
    ("cert_clean",
     "DOOR1 40 300 2 60 30 72 1000 30\n"
     "DOOR2 50 400 3 80 40 100 2000 100\n"
     "GR8 30 200 1 35 20 48 0 0\n"),
    ("cert_single_minor", "M1 40 300 2 60 10 72 1000 30\n"),
    ("cert_single_major", "J1 10 300 2 60 30 72 1000 30\n"),
    ("cert_single_critical", "C1 40 300 0 60 30 72 1000 30\n"),
    ("cert_prec_minor_over_crit", "P1 30 100 2 50 10 72 1000 30\n"),
    ("cert_prec_minor_over_crit2", "P2 40 300 2 50 30 72 120000 30\n"),
    ("cert_prec_minor_over_major", "P3 40 300 2 50 5 72 1000 30\n"),
    ("cert_prec_many", "P4 10 100 0 5 20 200 1000 30\n"),
    ("cert_prec_major_over_minor", "P5 40 300 2 60 30 72 1000 800\n"),
    ("cert_cross_v13", "X1 40 300 2 30 40 72 1000 30\n"),
    ("cert_cross_v14", "X2 500 300 2 60 30 72 1000 30\n"),
    ("cert_e10", "X1 1 2\nY2 1 2 3 4 5 6 7 8 9 10\n"),
    ("cert_e15",
     "d1 40 300 2 60 30 72 1000 30\n"
     "TOOLONGID 40 300 2 60 30 72 1000 30\n"
     "1A 40 300 2 60 30 72 1000 30\n"),
    ("cert_e20",
     "Z1 40 300 2 60 30 72 1000 0x\n"
     "Z2 09 300 2 60 30 72 1000 30\n"
     "Z3 40 -5 2 60 30 72 1000 30\n"),
    ("cert_priority_id_over_num", "z9 09 300 2 60 30 72 1000 30\n"),
    ("cert_tab", "D1\t40\t300\t2\t60\t30\t72\t1000\t30\n"),
    ("cert_crlf", "D1 40 300 2 60 30 72 1000 30\r\n"),
    ("cert_nbsp", "D1 40 300 2 60 30 72 1000 30\n"),
    ("cert_leading_trailing_space", "   D1 40 300 2 60 30 72 1000 30   \n"),
    ("cert_repeated_space", "D1   40    300 2 60 30 72 1000 30\n"),
    ("cert_no_trailing_nl", "D1 40 300 2 60 30 72 1000 30"),
    ("cert_blank_line",
     "D1 40 300 2 60 30 72 1000 30\n\nD2 50 400 3 80 40 100 2000 100\n"),
    ("cert_mixed_ladder",
     "D1 40 300 2 60 30 72 1000 30\n"
     "P1 30 100 2 50 10 72 1000 30\n"
     "J1 10 300 2 60 30 72 1000 30\n"
     "C1 40 300 0 60 30 72 1000 30\n"
     "xx 1 2\n"),
]

# report: (name, certify_source) -- report's stdin is certify's stdout for source
REPORT_BATCHES = [
    ("rep_empty", ""),
    ("rep_clean",
     "DOOR1 40 300 2 60 30 72 1000 30\n"
     "DOOR2 50 400 3 80 40 100 2000 100\n"
     "GR8 30 200 1 35 20 48 0 0\n"),
    ("rep_allpass",
     "A1 40 300 2 60 30 72 1000 30\n"
     "B2 50 400 3 80 40 100 2000 100\n"),
    ("rep_mixed",
     "A1 40 300 2 60 30 72 1000 30\n"
     "B1 40 300 0 60 30 72 1000 30\n"
     "C1 30 100 2 50 10 72 1000 30\n"
     "D1 10 300 2 60 30 72 1000 30\n"
     "E1 40 300 0 60 30 72 1000 30\n"),
]

# dispatch: (name, subarg, stdin) -> expect usage / exit 2
DISPATCH_BATCHES = [
    ("disp_missing", "", ""),
    ("disp_unknown", "verify", ""),
    ("disp_unknown_case", "REPORT", ""),
]

# pipeline: (name, raw certify input) -> chain through candidate own stdout
PIPELINE_BATCHES = [
    ("pipe_empty", ""),
    ("pipe_clean",
     "DOOR1 40 300 2 60 30 72 1000 30\n"
     "DOOR2 50 400 3 80 40 100 2000 100\n"
     "GR8 30 200 1 35 20 48 0 0\n"),
    ("pipe_mixed",
     "A1 40 300 2 60 30 72 1000 30\n"
     "B1 40 300 0 60 30 72 1000 30\n"
     "C1 30 100 2 50 10 72 1000 30\n"
     "D1 10 300 2 60 30 72 1000 30\n"
     "E1 40 300 0 60 30 72 1000 30\n"),
    ("pipe_with_errors",
     "D1 40 300 2 60 30 72 1000 30\n"
     "P1 30 100 2 50 10 72 1000 30\n"
     "J1 10 300 2 60 30 72 1000 30\n"
     "C1 40 300 0 60 30 72 1000 30\n"
     "xx 1 2\n"),
]


# --- candidate execution + comparison -----------------------------------------

def run_sub(cmd, sub, stdin, timeout):
    full = (cmd + " " + sub) if sub else cmd
    try:
        p = subprocess.run(full, shell=True,
                           input=stdin.encode("utf-8", "surrogateescape"),
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           timeout=timeout)
        return (p.stdout, p.stderr, p.returncode)
    except subprocess.TimeoutExpired:
        return (b"", b"<timeout>", 124)


def _cmp(label, got, exp_out, exp_err, exp_exit, fails):
    g_out, g_err, g_exit = got
    e_out = exp_out.encode("utf-8", "surrogateescape")
    e_err = exp_err.encode("utf-8", "surrogateescape")
    ok = (g_out == e_out) and (g_err == e_err) and (g_exit == exp_exit)
    if not ok:
        fails.append("FAIL %s" % label)
        if g_out != e_out:
            fails.append("  stdout exp %r got %r" % (e_out, g_out))
        if g_err != e_err:
            fails.append("  stderr exp %r got %r" % (e_err, g_err))
        if g_exit != exp_exit:
            fails.append("  exit   exp %r got %r" % (exp_exit, g_exit))
    return ok


def grade(cmd, timeout):
    total = 0
    passed = 0
    fails = []

    for name, stdin in CERT_BATCHES:
        exp_out, exp_err, exp_exit = simulate_certify(stdin)
        got = run_sub(cmd, "certify", stdin, timeout)
        total += 1
        if _cmp("certify:" + name, got, exp_out, exp_err, exp_exit, fails):
            passed += 1

    for name, src in REPORT_BATCHES:
        rin = simulate_certify(src)[0]
        exp_out, exp_err, exp_exit = simulate_report(rin)
        got = run_sub(cmd, "report", rin, timeout)
        total += 1
        if _cmp("report:" + name, got, exp_out, exp_err, exp_exit, fails):
            passed += 1

    for name, subarg, stdin in DISPATCH_BATCHES:
        got = run_sub(cmd, subarg, stdin, timeout)
        total += 1
        if _cmp("dispatch:" + name, got, "", USAGE, 2, fails):
            passed += 1

    for name, raw in PIPELINE_BATCHES:
        c_out, _c_err, _c_exit = run_sub(cmd, "certify", raw, timeout)
        c_str = c_out.decode("utf-8", "surrogateescape")
        got = run_sub(cmd, "report", c_str, timeout)
        exp_out, exp_err, exp_exit = simulate_report(simulate_certify(raw)[0])
        total += 1
        if _cmp("pipe:" + name, got, exp_out, exp_err, exp_exit, fails):
            passed += 1

    for line in fails:
        print(line)
    print("SCORE: %d/%d" % (passed, total))
    return passed, total


# --- self-check ---------------------------------------------------------------

SELFTEST = [
    ("certify", "DOOR1 40 300 2 60 30 72 1000 30\n", "DOOR1 PASS\n", "", 0),
    ("certify", "", "", "", 0),
    ("certify", "C1 40 300 0 60 30 72 1000 30\n", "C1 V05 relocker absent\n", "", 3),
    ("certify", "M1 40 300 2 60 10 72 1000 30\n",
     "M1 V08 torch rating below 15 minute minimum\n", "", 1),
    ("certify", "J1 10 300 2 60 30 72 1000 30\n",
     "J1 V01 bolt throw below 20 mm minimum\n", "", 2),
    ("certify", "P1 30 100 2 50 10 72 1000 30\n",
     "P1 V08 torch rating below 15 minute minimum\n", "", 1),
    ("certify", "P2 40 300 2 50 30 72 120000 30\n",
     "P2 V11 service interval exceeded above 50000 cycles\n", "", 1),
    ("certify", "P3 40 300 2 50 5 72 1000 30\n",
     "P3 V08 torch rating below 15 minute minimum\n", "", 1),
    ("certify", "P4 10 100 0 5 20 200 1000 30\n", "P4 V05 relocker absent\n", "", 3),
    ("certify", "P5 40 300 2 60 30 72 1000 800\n",
     "P5 V12 audit overdue beyond 365 days\n", "", 2),
    ("certify", "X1 40 300 2 30 40 72 1000 30\n",
     "X1 V13 drill rating below torch rating\n", "", 3),
    ("certify", "X2 500 300 2 60 30 72 1000 30\n",
     "X2 V14 bolt throw exceeds wall thickness\n", "", 2),
    ("certify", "xx 1 2\n", "",
     "E10 malformed record: expected 9 fields, got 3\n", 1),
    ("certify", "d1 40 300 2 60 30 72 1000 30\n", "", "E15 bad door id: d1\n", 1),
    ("certify", "Z2 09 300 2 60 30 72 1000 30\n", "", "E20 bad number: 09\n", 1),
    ("certify", "z9 09 300 2 60 30 72 1000 30\n", "", "E15 bad door id: z9\n", 1),
    ("certify", "D1\t40\t300\t2\t60\t30\t72\t1000\t30\n", "",
     "E10 malformed record: expected 9 fields, got 1\n", 1),
    ("certify", "D1 40 300 2 60 30 72 1000 30\r\n", "",
     "E20 bad number: 30\r\n", 1),
    ("certify", "D1 40 300 2 60 30 72 1000 30\n", "",
     "E10 malformed record: expected 9 fields, got 8\n", 1),
    ("report", "", "DOORS 0\nPASS 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("report",
     "A1 PASS\nB1 V05 relocker absent\n"
     "C1 V08 torch rating below 15 minute minimum\n"
     "D1 V01 bolt throw below 20 mm minimum\nE1 V05 relocker absent\n",
     "DOORS 5\nPASS 1\nCRITICAL 2\nMAJOR 1\nMINOR 1\n", "", 0),
]


def selftest():
    bad = 0
    # 1. registry / precedence integrity
    if sorted(PRECEDENCE) != sorted(CLASS.keys()):
        print("SELFTEST: precedence does not list every code exactly once")
        bad += 1
    if len(set(PRECEDENCE)) != len(PRECEDENCE):
        print("SELFTEST: precedence has a duplicate")
        bad += 1
    if set(MSG.keys()) != set(CLASS.keys()):
        print("SELFTEST: MSG / CLASS code sets differ")
        bad += 1

    # 2. hand-written expectation tuples
    for sub, stdin, eo, ee, ex in SELFTEST:
        go, ge, gx = SIM[sub](stdin)
        if (go, ge, gx) != (eo, ee, ex):
            print("SELFTEST mismatch sub=%s stdin=%r" % (sub, stdin))
            print("  exp %r %r %r" % (eo, ee, ex))
            print("  got %r %r %r" % (go, ge, gx))
            bad += 1

    # 3. canonical-chain consistency: certify stdout is consumable by report
    for name, raw in PIPELINE_BATCHES:
        c_out, _, _ = simulate_certify(raw)
        r_out, r_err, r_exit = simulate_report(c_out)
        nverdicts = 0 if c_out == "" else c_out.count("\n")
        first = r_out.split("\n")[0]
        if r_exit != 0 or r_err != "" or first != "DOORS %d" % nverdicts \
                or r_out.count("\n") != 5:
            print("SELFTEST chain inconsistency: %s" % name)
            bad += 1

    if bad:
        print("SELFTEST FAILED (%d)" % bad)
        return False
    print("SELFTEST OK")
    return True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cmd", help="base run command for the candidate program")
    ap.add_argument("--self-check", action="store_true",
                    help="run the oracle's internal self-test and exit")
    ap.add_argument("--timeout", type=float, default=10.0)
    args = ap.parse_args()

    if args.self_check:
        sys.exit(0 if selftest() else 1)

    if not args.cmd:
        ap.error("--cmd is required unless --self-check is given")

    passed, total = grade(args.cmd, args.timeout)
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
