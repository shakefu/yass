#!/usr/bin/env python3
"""Private oracle for round-07 (granary -- largest-remainder allotment, two-arm
clarity probe).

NEVER copied into an agent workspace. Black-box grades a candidate `granary`
CLI by byte-exact comparison of stdout / stderr / exit code. Both spec arms
(spec-arm-a: `design: AllotmentProcedure` pseudocode carrier; spec-arm-b:
current-v1 declarative obligations) pin the SAME behavior, so this one oracle
grades both. Correctness is expected clean in both arms; the batches exist to
make every algorithm trap observable so any marginal-corner miss is caught:

  - DISPATCH: missing / unknown / wrong-case argv[1] and a bare `-` -> usage,
    exit 2, no stdout.
  - allot: `POOL <units>` header gate (E40, write nothing, exit 2), claim
    records (E10/E25/E15/E20, exit 1 on any rejection), then largest-remainder
    allotment. THE TRAPS: per-claim integer floor (never half-up, never
    at-end); remainders compared as exact integer numerators; distribute
    exactly `leftover` units, <=1 per claim, zero iterations when leftover=0;
    tie-break = larger remainder, then SMALLER base, then LATER input
    position (defeats the input-order / largest-quota conventions); zero
    participation total -> all 0 + REMAIN pool. Output: GET lines in INPUT
    order + one REMAIN line; conservation sum(GET)+REMAIN == pool.
  - tally: the consumer. Zero-byte input -> the single line `EMPTY FEED`
    (arm A binds this via a guarded USES -> EmptyFeed design block: a model
    that ignores the block fails the batch). Otherwise LINES/CLAIMS/UNITS/
    REMAIN with the stated trust boundary and violation residual; exit 0.

The embedded simulators are the authoritative reference semantics, written
INDEPENDENTLY of oracle/ref.py (sort-key distribution here vs ref's explicit
repeated-selection scan) so a shared coding bug cannot pass both. Every batch
carries a HAND-PINNED expected (stdout, stderr, exit); `--self-check` asserts
the simulator reproduces every expectation, that the designated trap batches
DISCRIMINATE (rival models -- earlier-position tie-break, larger-base
tie-break, half-up rounding -- each produce different output), and the
conservation invariant across all pinned allot expectations; then prints
"SELFTEST OK". `--cmd '<run command>'` grades and prints "SCORE: passed/total".
"""

import argparse
import subprocess
import sys

USAGE = "usage: granary {allot|tally}\n"

_UP = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DG = frozenset("0123456789")


# --- shared helpers (granary.lines segmentation) --------------------------------

def _records(stdin):
    if stdin == "":
        return []
    body = stdin[:-1] if stdin.endswith("\n") else stdin
    return body.split("\n")


def _fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


def _emit(lines):
    return "".join(s + "\n" for s in lines)


def _is_number(tok):
    if not tok or not set(tok) <= _DG:
        return False
    return tok == "0" or not tok.startswith("0")


def _is_id(tok):
    """1-6 chars, first char A-Z, every char A-Z or 0-9."""
    if len(tok) < 1 or len(tok) > 6:
        return False
    if tok[0] not in _UP:
        return False
    return set(tok) <= (_UP | _DG)


# --- the allotment (AllotmentProcedure), sort-key formulation --------------------

def allot_units(pool, weights, tie="pinned"):
    """Return the per-claim unit list. `tie` selects the distribution order:
    'pinned' is the mandated precedence (remainder desc, base asc, later
    position first); the rivals exist only for --self-check discrimination."""
    n = len(weights)
    total = sum(weights)
    if total == 0:
        return [0] * n
    bases = [(pool * w) // total for w in weights]
    rems = [pool * w - b * total for w, b in zip(weights, bases)]
    leftover = pool - sum(bases)
    keys = {
        "pinned":        lambda i: (-rems[i], bases[i], -i),
        "earlier_pos":   lambda i: (-rems[i], bases[i], i),
        "larger_base":   lambda i: (-rems[i], -bases[i], -i),
        # the standard largest-remainder convention: remainder desc, then
        # input order -- no base level at all
        "input_order_no_base": lambda i: (-rems[i], i),
    }
    if tie == "half_up":
        # rival rounding model: per-claim round-half-up, no distribution fix
        return [(2 * pool * w + total) // (2 * total) for w in weights]
    order = sorted(range(n), key=keys[tie])
    units = bases[:]
    for i in order[:leftover]:
        units[i] += 1
    return units


# --- authoritative reference simulators (independent of ref.py) -----------------

def simulate_allot(stdin, tie="pinned"):
    recs = _records(stdin)
    if not recs:
        return ("", "E40 bad pool header\n", 2)
    hf = _fields(recs[0])
    if len(hf) != 2 or hf[0] != "POOL" or not _is_number(hf[1]):
        return ("", "E40 bad pool header\n", 2)
    pool = int(hf[1])
    err = []
    claims = []                       # (id, weight) accepted, in input order
    rejected = False
    for rec in recs[1:]:
        f = _fields(rec)
        n = len(f)
        if n == 0:
            err.append("E10 malformed record: empty line")
            rejected = True
            continue
        if f[0] != "CLAIM":
            err.append("E25 unknown operation: %s" % f[0])
            rejected = True
            continue
        if n != 3:
            err.append("E10 malformed record: CLAIM expects 3 fields, got %d" % n)
            rejected = True
            continue
        if not _is_id(f[1]):
            err.append("E15 bad claim id: %s" % f[1])
            rejected = True
            continue
        if not _is_number(f[2]):
            err.append("E20 bad number: %s" % f[2])
            rejected = True
            continue
        claims.append((f[1], int(f[2])))
    units = allot_units(pool, [w for _, w in claims], tie)
    out = ["%s GET %d" % (cid, u) for (cid, _), u in zip(claims, units)]
    out.append("REMAIN %d" % (pool - sum(units)))
    return (_emit(out), _emit(err), 1 if rejected else 0)


def simulate_tally(stdin):
    if stdin == "":
        return ("EMPTY FEED\n", "", 0)     # EmptyFeed procedure
    total = claims = units = remain = 0
    for rec in _records(stdin):
        total += 1
        f = _fields(rec)
        if len(f) >= 2 and f[1] == "GET":
            claims += 1
            if len(f) >= 3 and _is_number(f[2]):
                units += int(f[2])
        elif len(f) == 2 and f[0] == "REMAIN" and _is_number(f[1]):
            remain += int(f[1])
        # every other record: LINES only (violation residual)
    return ("LINES %d\nCLAIMS %d\nUNITS %d\nREMAIN %d\n"
            % (total, claims, units, remain), "", 0)


def simulate(sub, stdin):
    if sub == "allot":
        return simulate_allot(stdin)
    if sub == "tally":
        return simulate_tally(stdin)
    return ("", USAGE, 2)


# --- batches: (name, sub, stdin, exp_stdout, exp_stderr, exp_exit) ---------------
#
# Every expected triple is HAND-PINNED to PLAN.md, byte for byte. --self-check
# asserts the independent simulator reproduces each one.

E40 = "E40 bad pool header\n"

BATCHES = [
    # ---- DISPATCH (closed-set residual) --------------------------------------
    ("disp_missing", "", "POOL 5\n", "", USAGE, 2),
    ("disp_unknown", "audit", "", "", USAGE, 2),
    ("disp_wrong_case_allot", "ALLOT", "", "", USAGE, 2),
    ("disp_wrong_case_tally", "TALLY", "", "", USAGE, 2),
    ("disp_bare_dash", "-", "", "", USAGE, 2),

    # ---- ALLOT: pool-header gate ----------------------------------------------
    ("gate_empty_input", "allot", "", "", E40, 2),
    ("gate_lone_newline", "allot", "\n", "", E40, 2),
    ("gate_no_header", "allot", "CLAIM AB 3\nCLAIM CD 4\n", "", E40, 2),
    ("gate_wrong_count", "allot", "POOL 5 X\n", "", E40, 2),
    ("gate_bad_number", "allot", "POOL 05\n", "", E40, 2),
    ("gate_header_only", "allot", "POOL 5\n", "REMAIN 5\n", "", 0),

    # ---- ALLOT: segmentation / terminator mechanics ----------------------------
    ("allot_no_trailing_lf", "allot", "POOL 10\nCLAIM AB 1",
     "AB GET 10\nREMAIN 0\n", "", 0),
    ("allot_blank_interior", "allot", "POOL 10\nCLAIM AB 1\n\nCLAIM CD 1\n",
     "AB GET 5\nCD GET 5\nREMAIN 0\n",
     "E10 malformed record: empty line\n", 1),
    ("allot_double_trailing_lf", "allot", "POOL 10\nCLAIM AB 1\n\n",
     "AB GET 10\nREMAIN 0\n", "E10 malformed record: empty line\n", 1),
    ("allot_tab_is_data", "allot", "POOL 10\nCLAIM AB\t1\n",
     "REMAIN 10\n", "E10 malformed record: CLAIM expects 3 fields, got 2\n", 1),
    ("allot_cr_is_data", "allot", "POOL 10\nCLAIM AB 1\r\n",
     "REMAIN 10\n", "E20 bad number: 1\r\n", 1),
    ("allot_collapse_runs", "allot", "POOL  10 \n  CLAIM   AB  1 \n",
     "AB GET 10\nREMAIN 0\n", "", 0),

    # ---- ALLOT: claim record format (ordered checks, exhaustive) ---------------
    ("fmt_count_hi", "allot", "POOL 9\nCLAIM AB 1 X\n",
     "REMAIN 9\n", "E10 malformed record: CLAIM expects 3 fields, got 4\n", 1),
    ("fmt_unknown", "allot", "POOL 9\nGRANT AB 1\n",
     "REMAIN 9\n", "E25 unknown operation: GRANT\n", 1),
    ("fmt_unknown_single", "allot", "POOL 9\nbogus\n",
     "REMAIN 9\n", "E25 unknown operation: bogus\n", 1),
    ("fmt_bad_id_case", "allot", "POOL 9\nCLAIM ab 1\n",
     "REMAIN 9\n", "E15 bad claim id: ab\n", 1),
    ("fmt_bad_id_long", "allot", "POOL 9\nCLAIM ABCDEFG 1\n",
     "REMAIN 9\n", "E15 bad claim id: ABCDEFG\n", 1),
    ("fmt_bad_id_digit_first", "allot", "POOL 9\nCLAIM 1A 1\n",
     "REMAIN 9\n", "E15 bad claim id: 1A\n", 1),
    ("fmt_bad_weight", "allot", "POOL 9\nCLAIM AB 01\n",
     "REMAIN 9\n", "E20 bad number: 01\n", 1),
    ("fmt_e15_before_e20", "allot", "POOL 9\nCLAIM ab 01\n",
     "REMAIN 9\n", "E15 bad claim id: ab\n", 1),

    # ---- ALLOT: algorithm traps (THE PROBE CORNERS) -----------------------------
    # exact division: leftover 0, distribution runs zero times
    ("alg_exact_division", "allot", "POOL 9\nCLAIM A1 1\nCLAIM B2 2\n",
     "A1 GET 3\nB2 GET 6\nREMAIN 0\n", "", 0),
    # floor, never half-up (7/2 = 3.5 -> 3,3 + one extra; half-up gives 4,4)
    ("alg_floor_not_half", "allot", "POOL 7\nCLAIM A1 1\nCLAIM B2 1\n",
     "A1 GET 3\nB2 GET 4\nREMAIN 0\n", "", 0),
    # three-way remainder+base tie -> LATER position wins (input-order: 4,3,3)
    ("alg_tie_later_position", "allot",
     "POOL 10\nCLAIM A1 1\nCLAIM B2 1\nCLAIM C3 1\n",
     "A1 GET 3\nB2 GET 3\nC3 GET 4\nREMAIN 0\n", "", 0),
    # remainder tie -> SMALLER base wins (input-order AND largest-quota: 5,1)
    ("alg_tie_smaller_base", "allot", "POOL 6\nCLAIM A1 3\nCLAIM B2 1\n",
     "A1 GET 4\nB2 GET 2\nREMAIN 0\n", "", 0),
    # plain remainder ordering, leftover 1
    ("alg_remainder_order", "allot",
     "POOL 10\nCLAIM A1 3\nCLAIM B2 1\nCLAIM C3 2\n",
     "A1 GET 5\nB2 GET 2\nC3 GET 3\nREMAIN 0\n", "", 0),
    # leftover 2: exactly two extras, one each, remainder order B2(5) C3(4)
    ("alg_leftover_two", "allot",
     "POOL 11\nCLAIM A1 3\nCLAIM B2 1\nCLAIM C3 2\n",
     "A1 GET 5\nB2 GET 2\nC3 GET 4\nREMAIN 0\n", "", 0),
    # zero-weight claim has remainder 0 and never receives an extra
    ("alg_zero_weight_skipped", "allot",
     "POOL 5\nCLAIM A1 0\nCLAIM B2 1\nCLAIM C3 1\n",
     "A1 GET 0\nB2 GET 2\nC3 GET 3\nREMAIN 0\n", "", 0),
    # zero participation total: all 0, whole pool remains, no distribution
    ("alg_zero_total", "allot", "POOL 8\nCLAIM A1 0\nCLAIM B2 0\n",
     "A1 GET 0\nB2 GET 0\nREMAIN 8\n", "", 0),
    # pool 0: bases 0, remainders 0, leftover 0
    ("alg_pool_zero", "allot", "POOL 0\nCLAIM A1 1\nCLAIM B2 2\n",
     "A1 GET 0\nB2 GET 0\nREMAIN 0\n", "", 0),
    # a rejected record participates in nothing (total is 2, not 3)
    ("alg_reject_participation", "allot",
     "POOL 10\nCLAIM A1 1\nCLAIM xx 1\nCLAIM B2 1\n",
     "A1 GET 5\nB2 GET 5\nREMAIN 0\n", "E15 bad claim id: xx\n", 1),
    # exactness at scale
    ("alg_large", "allot", "POOL 999999\nCLAIM A1 1\nCLAIM B2 2\n",
     "A1 GET 333333\nB2 GET 666666\nREMAIN 0\n", "", 0),

    # ---- TALLY: EmptyFeed (guarded USES -> design, arm A) ------------------------
    ("tally_empty", "tally", "", "EMPTY FEED\n", "", 0),
    ("tally_blank_only", "tally", "\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),

    # ---- TALLY: pipeline / trust boundary ----------------------------------------
    ("tally_pipeline", "tally", "A1 GET 5\nB2 GET 2\nC3 GET 3\nREMAIN 0\n",
     "LINES 4\nCLAIMS 3\nUNITS 10\nREMAIN 0\n", "", 0),
    ("tally_remain_sum", "tally", "REMAIN 5\nREMAIN 3\n",
     "LINES 2\nCLAIMS 0\nUNITS 0\nREMAIN 8\n", "", 0),
    ("tally_trusted_id", "tally", "zz GET 4\n",
     "LINES 1\nCLAIMS 1\nUNITS 4\nREMAIN 0\n", "", 0),
    ("tally_get_bad_units", "tally", "AB GET x7\n",
     "LINES 1\nCLAIMS 1\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_get_missing_units", "tally", "AB GET\n",
     "LINES 1\nCLAIMS 1\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_get_extra_fields", "tally", "AB GET 4 X\n",
     "LINES 1\nCLAIMS 1\nUNITS 4\nREMAIN 0\n", "", 0),
    ("tally_remain_bad_number", "tally", "REMAIN x\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_remain_wrong_count", "tally", "REMAIN 4 X\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_unknown_token", "tally", "AB TOOK 4\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_lowercase_token", "tally", "AB get 4\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_tab_is_data", "tally", "AB\tGET 4\n",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 0\n", "", 0),
    ("tally_no_trailing_lf", "tally", "REMAIN 7",
     "LINES 1\nCLAIMS 0\nUNITS 0\nREMAIN 7\n", "", 0),
]

# trap batches whose pinned output the rival models must NOT reproduce, and
# which rivals they are designed to defeat
DISCRIMINATORS = {
    "alg_floor_not_half":     ["half_up"],
    "alg_tie_later_position": ["earlier_pos", "input_order_no_base"],
    "alg_tie_smaller_base":   ["input_order_no_base", "larger_base"],
    "alg_zero_weight_skipped": ["earlier_pos", "input_order_no_base"],
}


# --- candidate execution + comparison --------------------------------------------

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
    for name, sub, stdin, eo, ee, ex in BATCHES:
        got = run_sub(cmd, sub, stdin, timeout)
        total += 1
        if _cmp(name, got, eo, ee, ex, fails):
            passed += 1
    for line in fails:
        print(line)
    print("SCORE: %d/%d" % (passed, total))
    return passed, total


# --- self-check --------------------------------------------------------------------

def _structural_ok():
    problems = []
    batch_by_name = {b[0]: b for b in BATCHES}

    # trap batches must DISCRIMINATE: each designated rival diverges from pinned
    for name, rivals in DISCRIMINATORS.items():
        _, sub, stdin, eo, _, _ = batch_by_name[name]
        for rival in rivals:
            got, _, _ = simulate_allot(stdin, tie=rival)
            if got == eo:
                problems.append("%s does not discriminate rival %r"
                                % (name, rival))

    # conservation: sum(GET) + REMAIN == pool for every pinned allot success
    for name, sub, stdin, eo, ee, ex in BATCHES:
        if sub != "allot" or ex == 2:
            continue
        pool = int(_fields(_records(stdin)[0])[1])
        gets = 0
        remain = None
        for line in eo.splitlines():
            f = line.split(" ")
            if f[1:2] == ["GET"]:
                gets += int(f[2])
            elif f[0] == "REMAIN":
                remain = int(f[1])
        if remain is None:
            problems.append("%s pinned output has no REMAIN line" % name)
        elif gets + remain != pool:
            problems.append("%s violates conservation: %d + %d != %d"
                            % (name, gets, remain, pool))

    # tie-break sanity on the deciding datasets
    if allot_units(10, [1, 1, 1]) != [3, 3, 4]:
        problems.append("three-way tie does not give the extra to the last claim")
    if allot_units(6, [3, 1]) != [4, 2]:
        problems.append("remainder tie does not prefer the smaller base")
    if allot_units(7, [1, 1]) != [3, 4]:
        problems.append("floor+tie case wrong")
    if allot_units(9, [1, 2]) != [3, 6]:
        problems.append("exact division must not distribute")
    if allot_units(8, [0, 0]) != [0, 0]:
        problems.append("zero total must allot nothing")
    return problems


def selftest():
    bad = 0
    for msg in _structural_ok():
        print("SELFTEST: %s" % msg)
        bad += 1
    for name, sub, stdin, eo, ee, ex in BATCHES:
        go, ge, gx = simulate(sub, stdin)
        if (go, ge, gx) != (eo, ee, ex):
            print("SELFTEST mismatch: %s (sub=%r stdin=%r)" % (name, sub, stdin))
            print("  exp %r %r %r" % (eo, ee, ex))
            print("  got %r %r %r" % (go, ge, gx))
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
