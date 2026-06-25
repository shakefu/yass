#!/usr/bin/env python3
"""Private oracle for round-04 (axle wayside hotbox-detector audit).

NEVER copied into an agent workspace. Black-box grades a candidate CLI by
byte-exact comparison of stdout / stderr / exit code on four axes:

  1. audit with raw axle reading records (clean, single-defect-per-class message
     exactness, multi-defect precedence -- including the deciding batch where the
     governing code is neither the lowest code number nor the max-class pick --
     the H07 inner mileage guard ON and OFF, processing errors E10/E15/E20 in
     priority order, off-spec segmentation, the exit ladder).
  2. roster with canonical / hand-built input (empty, clean source, mixed source,
     the over-validation probe, and the four guard-combination corners
     C-TT / C-TF / C-FT / C-FF that isolate H07's outer x inner guard product).
  3. dispatch: missing / unknown / wrong-case argv[1] -> usage, exit 2.
  4. pipeline: the candidate's own audit stdout fed to its own roster, so that
     roster never sees audit's stderr.

The embedded simulate_audit / simulate_roster ARE the authoritative reference.
They are written independently of oracle/ref.py so a shared coding bug cannot
pass both. `--self-check` runs SELFTEST (registry/precedence integrity, the four
corners, H07 on/off, byte-exact errors, ordered checks, chain consistency) and
prints "SELFTEST OK". `--cmd '<run command>'` grades that command and prints
"SCORE: passed/total".
"""

import argparse
import subprocess
import sys

# --- bearing defect registry (the structured-obligation cluster under test) ---
#
# Each row: code -> (class, message, predicate). The predicate is evaluated over
# a readings dict so the condition wiring is data-driven here -- a deliberately
# different shape from ref.py's straight-line if-ladder.

REGISTRY = {
    "H01": ("MAJOR",    "bearing temp above 80 absolute alarm",
            lambda r: r["bt"] > 80),
    "H02": ("MINOR",    "bearing temp above 60 warning",
            lambda r: r["bt"] > 60),
    "H03": ("CRITICAL", "differential above 70 hotbox",
            lambda r: r["bt"] - r["amb"] > 70),
    "H04": ("MAJOR",    "differential above 50 alarm",
            lambda r: r["bt"] - r["amb"] > 50),
    "H05": ("MAJOR",    "side-to-side differential above 40",
            lambda r: r["bt"] - r["mate"] > 40),
    "H06": ("CRITICAL", "bearing temp above 100 burnoff",
            lambda r: r["bt"] > 100),
    # H07 carries an inner WHEN guard: mileage > 200
    "H07": ("MINOR",    "aged bearing side differential above 25",
            lambda r: r["mileage"] > 200 and r["bt"] - r["mate"] > 25),
    "H08": ("MAJOR",    "wheel impact above 140 kN alarm",
            lambda r: r["impact"] > 140),
    "H09": ("CRITICAL", "wheel impact above 200 kN critical",
            lambda r: r["impact"] > 200),
    "H10": ("MINOR",    "wheel impact above 90 kN warning",
            lambda r: r["impact"] > 90),
    "H11": ("MINOR",    "load above 130 tonne limit",
            lambda r: r["load"] > 130),
    "H12": ("MAJOR",    "reprofile overdue beyond 280",
            lambda r: r["mileage"] > 280),
}

# governing precedence, highest first (deliberately non-monotonic vs class & code)
PRECEDENCE = ["H06", "H09", "H03", "H08", "H01", "H05", "H04",
              "H12", "H02", "H11", "H10", "H07"]

CONTRIB = {"CRITICAL": 3, "MAJOR": 2, "MINOR": 1}

CLASS_OF = {code: row[0] for code, row in REGISTRY.items()}
MSG_OF = {code: row[1] for code, row in REGISTRY.items()}

USAGE = "usage: axle {audit|roster}\n"

_UP = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DG = frozenset("0123456789")


# --- shared helpers -----------------------------------------------------------

def _records(stdin):
    if stdin == "":
        return []
    body = stdin[:-1] if stdin.endswith("\n") else stdin
    return body.split("\n")


def _fields(rec):
    return list(filter(None, rec.split(" ")))


def _emit(lines):
    return "".join(s + "\n" for s in lines)


def _is_number(tok):
    if not tok or not set(tok) <= _DG:
        return False
    return tok == "0" or not tok.startswith("0")


def _is_axle_id(tok):
    if len(tok) < 3 or len(tok) > 10:
        return False
    if tok[0] not in _UP:
        return False
    return set(tok) <= (_UP | _DG)


def _readings(nums):
    keys = ("bt", "amb", "mate", "impact", "load", "mileage")
    return dict(zip(keys, (int(x) for x in nums)))


def _verdict_code(readings):
    """Highest-precedence triggered code, or None for CLEAR."""
    active = {code for code, (_c, _m, pred) in REGISTRY.items() if pred(readings)}
    for code in PRECEDENCE:
        if code in active:
            return code
    return None


# --- authoritative reference simulators (independent of ref.py) ---------------

def simulate_audit(stdin):
    stdout_lines = []
    stderr_lines = []
    exit_code = 0
    for rec in _records(stdin):
        f = _fields(rec)
        n = len(f)
        if n != 7:
            stderr_lines.append("E10 malformed record: expected 7 fields, got %d" % n)
            exit_code = max(exit_code, 1)
            continue
        if not _is_axle_id(f[0]):
            stderr_lines.append("E15 bad axle id: %s" % f[0])
            exit_code = max(exit_code, 1)
            continue
        bad = next((tok for tok in f[1:7] if not _is_number(tok)), None)
        if bad is not None:
            stderr_lines.append("E20 bad number: %s" % bad)
            exit_code = max(exit_code, 1)
            continue
        readings = _readings(f[1:7])
        code = _verdict_code(readings)
        if code is None:
            stdout_lines.append("%s CLEAR" % f[0])
        else:
            stdout_lines.append("%s %s %s" % (f[0], code, MSG_OF[code]))
            exit_code = max(exit_code, CONTRIB[CLASS_OF[code]])
    return (_emit(stdout_lines), _emit(stderr_lines), exit_code)


def simulate_roster(stdin):
    counts = {"AXLES": 0, "CLEAR": 0, "CRITICAL": 0, "MAJOR": 0, "MINOR": 0}

    def credit(code_or_clear):
        if code_or_clear == "CLEAR":
            counts["CLEAR"] += 1
            return
        klass = CLASS_OF.get(code_or_clear)
        if klass in CONTRIB:
            counts[klass] += 1
        # a second field that is neither CLEAR nor a registry code -> no class

    for rec in _records(stdin):
        f = _fields(rec)
        if not f:
            continue
        counts["AXLES"] += 1
        if f[0] == "RECHECK":
            flag = f[2] if len(f) > 2 else ""
            nums = f[3:9]
            if flag == "Y" and len(nums) == 6 and all(_is_number(t) for t in nums):
                code = _verdict_code(_readings(nums))
                credit("CLEAR" if code is None else code)
            else:
                # flagged N (or no re-derivable readings) -> clear, no re-derivation
                counts["CLEAR"] += 1
        else:
            credit(f[1] if len(f) > 1 else "")
    order = ["AXLES", "CLEAR", "CRITICAL", "MAJOR", "MINOR"]
    return (_emit("%s %d" % (k, counts[k]) for k in order), "", 0)


SIM = {"audit": simulate_audit, "roster": simulate_roster}


# --- batches ------------------------------------------------------------------

# audit: (name, stdin)
AUDIT_BATCHES = [
    ("aud_empty", ""),
    ("aud_clean1", "AX1 40 20 35 50 80 100\n"),
    ("aud_clean2", "WHEEL42 0 0 0 0 0 0\n"),
    ("aud_clean3",
     "AX1 40 20 35 50 80 100\n"
     "AX2 55 30 30 80 120 150\n"
     "GR8X 0 0 0 0 0 0\n"),
    ("aud_single_minor", "MN1 65 20 50 50 80 100\n"),     # H02 only (bt>60)
    ("aud_single_major", "MJ1 85 40 60 50 80 100\n"),     # H01 only (bt>80, diffs low)
    ("aud_single_critical", "CR1 50 20 30 205 80 100\n"),  # H09 only (impact>200)
    ("aud_prec_crit_over_major", "CV1 95 10 70 50 80 100\n"),  # H01,H02,H03,H04 -> H03
    ("aud_prec_minor_over_major", "PB1 0 0 0 95 135 100\n"),    # H10,H11 -> H11
    ("aud_prec_deciding", "AX1 85 20 20 145 80 100\n"),         # H08 wins; low-code=H01
    ("aud_h03_diff", "DF1 95 10 90 50 80 100\n"),              # H03 differential
    ("aud_h05_side", "SD1 80 60 30 50 80 100\n"),              # H05 side differential
    ("aud_h07_on", "AX1 50 20 20 50 80 250\n"),               # H07 fires (mileage 250)
    ("aud_h07_off", "AX1 50 20 20 50 80 100\n"),              # CLEAR (mileage 100)
    ("aud_e10_count", "AX1 40 20 35 50 80\nAX2 40 20 35 50 80 100 200\n"),
    ("aud_e15_id",
     "a1 40 20 35 50 80 100\n"
     "TOOLONGAXLE1 40 20 35 50 80 100\n"
     "1AX 40 20 35 50 80 100\n"),
    ("aud_e20_number",
     "AX1 040 20 35 50 80 100\n"
     "AX2 40 -5 35 50 80 100\n"
     "AX3 40 20 35 50 80 1x\n"),
    ("aud_ordered_id_over_num", "a1 040 20 35 50 80 100\n"),   # E15 before E20
    ("aud_tab", "AX1\t40\t20\t35\t50\t80\t100\n"),             # E10 got 1
    ("aud_crlf", "AX1 40 20 35 50 80 100\r\n"),                # E20 (\r rides last tok)
    ("aud_nbsp", "AX1 40 20 35 50 80 100\xa0\n"),             # NBSP rides last tok -> E20
    ("aud_lead_trail_runs", "   AX1   40    20 35  50 80 100   \n"),
    ("aud_no_trailing_nl", "AX1 40 20 35 50 80 100"),
    ("aud_blank_interior",
     "AX1 40 20 35 50 80 100\n\nAX2 40 20 35 50 80 100\n"),    # blank -> E10 got 0
    ("aud_lone_newline", "\n"),                                # one zero-field rec, E10 got 0
    ("aud_ladder",
     "AX1 40 20 35 50 80 100\n"      # CLEAR
     "MN1 65 20 50 50 80 100\n"      # MINOR
     "MJ1 85 40 60 50 80 100\n"      # MAJOR
     "CR1 50 20 30 205 80 100\n"     # CRITICAL
     "xx 1 2\n"),                    # malformed -> exit max = 3
]

# roster: (name, roster_stdin) -- fed straight to candidate's roster
ROSTER_BATCHES = [
    ("rep_empty", ""),
    ("rep_clean",
     "AX1 CLEAR\nAX2 CLEAR\nAX3 CLEAR\n"),
    ("rep_mixed",
     "AX1 CLEAR\n"
     "AX2 H06 bearing temp above 100 burnoff\n"
     "AX3 H01 bearing temp above 80 absolute alarm\n"
     "AX4 H02 bearing temp above 60 warning\n"
     "AX5 H09 wheel impact above 200 kN critical\n"),
    ("rep_overvalidate",
     "AX1 CLEAR\nAX2 H06 bearing temp above 100 burnoff\n"),
    ("corner_tt", "RECHECK AX1 Y 50 20 20 50 80 250\n"),
    ("corner_tf", "RECHECK AX1 Y 50 20 20 50 80 100\n"),
    ("corner_ft", "RECHECK AX1 N 50 20 20 50 80 250\n"),
    ("corner_ff", "RECHECK AX1 N 50 20 20 50 80 100\n"),
    ("corner_mix",
     "AX0 CLEAR\n"
     "AX9 H03 differential above 70 hotbox\n"
     "RECHECK AX1 Y 50 20 20 50 80 250\n"   # H07 -> +1 MINOR
     "RECHECK AX2 Y 50 20 20 50 80 100\n"   # inner false -> clear
     "RECHECK AX3 N 50 20 20 50 80 250\n"   # outer false -> clear
     "RECHECK AX4 N 50 20 20 50 80 100\n"), # clear
]

# dispatch: (name, subarg, stdin) -> expect usage / exit 2
DISPATCH_BATCHES = [
    ("disp_missing", "", ""),
    ("disp_unknown", "verify", ""),
    ("disp_wrong_case", "AUDIT", ""),
]

# pipeline: (name, raw audit input) -> chain through candidate's own audit stdout
PIPELINE_BATCHES = [
    ("pipe_empty", ""),
    ("pipe_clean",
     "AX1 40 20 35 50 80 100\n"
     "AX2 55 30 30 80 120 150\n"
     "GR8X 0 0 0 0 0 0\n"),
    ("pipe_mixed",
     "AX1 40 20 35 50 80 100\n"
     "MN1 65 20 50 50 80 100\n"
     "MJ1 85 40 60 50 80 100\n"
     "CR1 50 20 30 205 80 100\n"),
    ("pipe_with_errors",
     "AX1 40 20 35 50 80 100\n"
     "MN1 65 20 50 50 80 100\n"
     "CR1 50 20 30 205 80 100\n"
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
    total = passed = 0
    fails = []

    for name, stdin in AUDIT_BATCHES:
        eo, ee, ex = simulate_audit(stdin)
        got = run_sub(cmd, "audit", stdin, timeout)
        total += 1
        if _cmp("audit:" + name, got, eo, ee, ex, fails):
            passed += 1

    for name, rin in ROSTER_BATCHES:
        eo, ee, ex = simulate_roster(rin)
        got = run_sub(cmd, "roster", rin, timeout)
        total += 1
        if _cmp("roster:" + name, got, eo, ee, ex, fails):
            passed += 1

    for name, subarg, stdin in DISPATCH_BATCHES:
        got = run_sub(cmd, subarg, stdin, timeout)
        total += 1
        if _cmp("dispatch:" + name, got, "", USAGE, 2, fails):
            passed += 1

    for name, raw in PIPELINE_BATCHES:
        a_out, _a_err, _a_exit = run_sub(cmd, "audit", raw, timeout)
        a_str = a_out.decode("utf-8", "surrogateescape")
        got = run_sub(cmd, "roster", a_str, timeout)
        eo, ee, ex = simulate_roster(simulate_audit(raw)[0])
        total += 1
        if _cmp("pipe:" + name, got, eo, ee, ex, fails):
            passed += 1

    for line in fails:
        print(line)
    print("SCORE: %d/%d" % (passed, total))
    return passed, total


# --- self-check ---------------------------------------------------------------

SELFTEST = [
    # control + clean
    ("audit", "AX1 40 20 35 50 80 100\n", "AX1 CLEAR\n", "", 0),
    ("audit", "", "", "", 0),
    # finding-1 transclusion deciding measurements (O1-O4)
    ("audit", "a1 40 20 35 50 80 100\n", "", "E15 bad axle id: a1\n", 1),
    ("audit", "AX1 040 20 35 50 80 100\n", "", "E20 bad number: 040\n", 1),
    ("audit", "AX1 40 20 35 50 80\n", "",
     "E10 malformed record: expected 7 fields, got 6\n", 1),
    # single-class message exactness
    ("audit", "MN1 65 20 50 50 80 100\n",
     "MN1 H02 bearing temp above 60 warning\n", "", 1),
    ("audit", "MJ1 85 40 60 50 80 100\n",
     "MJ1 H01 bearing temp above 80 absolute alarm\n", "", 2),
    ("audit", "CR1 50 20 30 205 80 100\n",
     "CR1 H09 wheel impact above 200 kN critical\n", "", 3),
    # the deciding precedence batch: H08 governs, not lowest code (H01), not class pick
    ("audit", "AX1 85 20 20 145 80 100\n",
     "AX1 H08 wheel impact above 140 kN alarm\n", "", 2),
    # within-class precedence divergence: H11 governs over lower-code H10
    ("audit", "PB1 0 0 0 95 135 100\n",
     "PB1 H11 load above 130 tonne limit\n", "", 1),
    # cross-field differentials
    ("audit", "DF1 95 10 90 50 80 100\n",
     "DF1 H03 differential above 70 hotbox\n", "", 3),
    ("audit", "SD1 80 60 30 50 80 100\n",
     "SD1 H05 side-to-side differential above 40\n", "", 2),
    # H07 inner guard ON / OFF
    ("audit", "AX1 50 20 20 50 80 250\n",
     "AX1 H07 aged bearing side differential above 25\n", "", 1),
    ("audit", "AX1 50 20 20 50 80 100\n", "AX1 CLEAR\n", "", 0),
    # ordered checks: E10 before E15 before E20
    ("audit", "xx 1 2\n", "", "E10 malformed record: expected 7 fields, got 3\n", 1),
    ("audit", "a1 040 20 35 50 80 100\n", "", "E15 bad axle id: a1\n", 1),
    # segmentation degenerate
    ("audit", "AX1\t40\t20\t35\t50\t80\t100\n", "",
     "E10 malformed record: expected 7 fields, got 1\n", 1),
    ("audit", "AX1 40 20 35 50 80 100\r\n", "", "E20 bad number: 100\r\n", 1),
    ("audit", "\n", "", "E10 malformed record: expected 7 fields, got 0\n", 1),
    ("audit", "   AX1   40    20 35  50 80 100   \n", "AX1 CLEAR\n", "", 0),
    # roster: empty, the four corners, over-validation probe
    ("roster", "", "AXLES 0\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("roster", "RECHECK AX1 Y 50 20 20 50 80 250\n",
     "AXLES 1\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 1\n", "", 0),
    ("roster", "RECHECK AX1 Y 50 20 20 50 80 100\n",
     "AXLES 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("roster", "RECHECK AX1 N 50 20 20 50 80 250\n",
     "AXLES 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("roster", "RECHECK AX1 N 50 20 20 50 80 100\n",
     "AXLES 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("roster", "AX1 CLEAR\nAX2 H06 bearing temp above 100 burnoff\n",
     "AXLES 2\nCLEAR 1\nCRITICAL 1\nMAJOR 0\nMINOR 0\n", "", 0),
]


def _h07_isolation_ok():
    """At bt50/amb20/mate20/impact50/load80, only H07 may fire, and only at
    mileage > 200. Verify per-code so the four corners truly isolate H07."""
    base = {"bt": 50, "amb": 20, "mate": 20, "impact": 50, "load": 80}
    for mileage, expect_h07 in ((100, False), (250, True)):
        r = dict(base, mileage=mileage)
        for code, (_c, _m, pred) in REGISTRY.items():
            fires = pred(r)
            if code == "H07":
                if fires != expect_h07:
                    return False
            elif fires:
                return False
    return True


def selftest():
    bad = 0

    # 1. registry / precedence integrity
    if sorted(PRECEDENCE) != sorted(REGISTRY.keys()):
        print("SELFTEST: precedence does not list every code exactly once")
        bad += 1
    if len(set(PRECEDENCE)) != len(PRECEDENCE):
        print("SELFTEST: precedence has a duplicate")
        bad += 1
    if set(CLASS_OF) != set(MSG_OF) or set(CLASS_OF) != set(REGISTRY):
        print("SELFTEST: class / message / registry code sets differ")
        bad += 1
    if any(CLASS_OF[c] not in CONTRIB for c in REGISTRY):
        print("SELFTEST: a code has an unranked severity class")
        bad += 1

    # 2. H07 isolation for the four-corner readings
    if not _h07_isolation_ok():
        print("SELFTEST: H07 isolation broken -- another code fires for the corner readings")
        bad += 1

    # 3. hand-written expectation tuples
    for sub, stdin, eo, ee, ex in SELFTEST:
        go, ge, gx = SIM[sub](stdin)
        if (go, ge, gx) != (eo, ee, ex):
            print("SELFTEST mismatch sub=%s stdin=%r" % (sub, stdin))
            print("  exp %r %r %r" % (eo, ee, ex))
            print("  got %r %r %r" % (go, ge, gx))
            bad += 1

    # 4. canonical-chain consistency: audit stdout is consumable by roster
    for name, raw in PIPELINE_BATCHES:
        a_out, _, _ = simulate_audit(raw)
        r_out, r_err, r_exit = simulate_roster(a_out)
        nverdicts = 0 if a_out == "" else a_out.count("\n")
        first = r_out.split("\n")[0]
        if r_exit != 0 or r_err != "" or first != "AXLES %d" % nverdicts \
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
