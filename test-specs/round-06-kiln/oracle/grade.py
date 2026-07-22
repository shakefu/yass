#!/usr/bin/env python3
"""Private oracle for round-06 (kiln -- kiln-firing controller, two-arm probe).

NEVER copied into an agent workspace. Black-box grades a candidate `kiln` CLI by
byte-exact comparison of stdout / stderr / exit code over the full contract.
Both spec arms (spec-arm-a: design-block ordering carrier; spec-arm-b: current
v1 prose carrier) pin the SAME behavior, so this one oracle grades both.

  - DISPATCH: missing / unknown / wrong-case argv[1] and a bare `-` -> usage,
    exit 2, no stdout (closed-set dispatch residual).
  - fire: stdin load manifest (`LOAD <id> <base>` records) -> per accepted
    record four stage lines plus one grade line to stdout, per rejected record
    one error line (E10/E25/E15/E20) to stderr, exit = max contribution.
    Exercises segmentation/terminator mechanics, the ordered record checks,
    and THE PROBE: the firing procedure's mandated stage order
    SOAK -> SEAL -> VENT -> RAMP, which is deliberately not alphabetical, not
    the stage specs' documentation order, and not domain intuition. The stage
    arithmetic (x3, -220, +35, x2) is non-commutative, so a wrong order breaks
    both the label sequence and every logged value.
  - report: the consumer. Classifies each line by its second field alone
    (trusting fire's text -- the dataflow/trust-boundary probe), writes a
    five-line tally, exit 0. Violation inputs (out-of-contract id, wrong field
    count, unknown token, blank line) are counted toward LINES per the stated
    residual, never re-validated or rejected.

The embedded simulate_fire / simulate_report + dispatch are the authoritative
reference semantics, written INDEPENDENTLY of oracle/ref.py (data-driven op
table here vs ref's straight-line per-stage statements) so a shared coding bug
cannot pass both. Every batch carries a HAND-PINNED expected (stdout, stderr,
exit); `--self-check` asserts the simulator reproduces every expectation plus
a set of structural invariants (mandated order differs from every rival prior,
rival-order traces diverge for every batch base, grade boundary truth table)
and prints "SELFTEST OK". `--cmd '<run command>'` grades that command and
prints "SCORE: passed/total".
"""

import argparse
import subprocess
import sys

# --- firing procedure (the probe) ---------------------------------------------
#
# Mandated execution order and per-stage ops, data-driven (ref.py instead uses
# explicit sequential statements). Order is SOAK, SEAL, VENT, RAMP -- compare
# the rival priors asserted in --self-check.

ORDER = ["SOAK", "SEAL", "VENT", "RAMP"]
OPS = {
    "SOAK": ("mul", 3),
    "SEAL": ("add", -220),
    "VENT": ("add", 35),
    "RAMP": ("mul", 2),
}

RIVAL_ORDERS = {
    "alphabetical/doc-order": ["RAMP", "SEAL", "SOAK", "VENT"],
    "reverse-doc-order":      ["VENT", "SOAK", "SEAL", "RAMP"],
    "domain-intuition":       ["VENT", "RAMP", "SOAK", "SEAL"],
}

# --- grade registry (kiln.grade) -----------------------------------------------

def grade_of(final):
    """Return (token, line-tail-or-None, contribution). DONE tail is dynamic."""
    if final > 2000:
        return ("G90", "overfired above 2000", 3)
    if final < 500:
        return ("G10", "underfired below 500", 2)
    return ("DONE", None, 0)


# report: second-field token -> tally category
STAGE_TOKENS = frozenset(ORDER)
GRADE_TOKENS = {"DONE": "DONE", "G10": "UNDERFIRED", "G90": "OVERFIRED"}

USAGE = "usage: kiln {fire|report}\n"

_UP = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DG = frozenset("0123456789")


# --- shared helpers (kiln.lines segmentation) ----------------------------------

def _records(stdin):
    """Strip exactly one trailing LF if present, split on LF. Empty input
    (zero bytes) -> zero records. A blank line is a zero-field record."""
    if stdin == "":
        return []
    body = stdin[:-1] if stdin.endswith("\n") else stdin
    return body.split("\n")


def _fields(rec):
    """ASCII-space split, runs collapsed, leading/trailing stripped. Tab and CR
    are ordinary data bytes (not separators)."""
    return [tok for tok in rec.split(" ") if tok != ""]


def _emit(lines):
    return "".join(s + "\n" for s in lines)


def _is_number(tok):
    """Non-empty ASCII-digit run, no leading zero unless the literal 0."""
    if not tok or not set(tok) <= _DG:
        return False
    return tok == "0" or not tok.startswith("0")


def _is_id(tok):
    """2-8 chars, first char A-Z, every char A-Z or 0-9."""
    if len(tok) < 2 or len(tok) > 8:
        return False
    if tok[0] not in _UP:
        return False
    return set(tok) <= (_UP | _DG)


def run_procedure(base, order):
    """Thread the running index through the stages in `order`; return the list
    of (stage, produced-index) pairs."""
    idx = base
    trace = []
    for stage in order:
        kind, operand = OPS[stage]
        idx = idx * operand if kind == "mul" else idx + operand
        trace.append((stage, idx))
    return trace


# --- authoritative reference simulators (independent of ref.py) ---------------

def simulate_fire(stdin):
    out_lines = []
    err_lines = []
    exit_code = 0
    for rec in _records(stdin):
        f = _fields(rec)
        n = len(f)
        # ordered record checks, stop at first failure
        if n == 0:
            err_lines.append("E10 malformed record: empty line")
            exit_code = max(exit_code, 1)
            continue
        if f[0] != "LOAD":
            err_lines.append("E25 unknown operation: %s" % f[0])
            exit_code = max(exit_code, 1)
            continue
        if n != 3:
            err_lines.append("E10 malformed record: LOAD expects 3 fields, got %d" % n)
            exit_code = max(exit_code, 1)
            continue
        lid = f[1]
        num = f[2]
        if not _is_id(lid):
            err_lines.append("E15 bad load id: %s" % lid)
            exit_code = max(exit_code, 1)
            continue
        if not _is_number(num):
            err_lines.append("E20 bad number: %s" % num)
            exit_code = max(exit_code, 1)
            continue
        # accepted: run the firing procedure in the mandated order
        trace = run_procedure(int(num), ORDER)
        for stage, idx in trace:
            out_lines.append("%s %s %d" % (lid, stage, idx))
        final = trace[-1][1]
        token, tail, contrib = grade_of(final)
        if token == "DONE":
            out_lines.append("%s DONE %d" % (lid, final))
        else:
            out_lines.append("%s %s %s" % (lid, token, tail))
        exit_code = max(exit_code, contrib)
    return (_emit(out_lines), _emit(err_lines), exit_code)


def simulate_report(stdin):
    counts = {"STAGES": 0, "DONE": 0, "UNDERFIRED": 0, "OVERFIRED": 0}
    total = 0
    for rec in _records(stdin):
        total += 1                      # every record counts toward LINES
        f = _fields(rec)
        if len(f) < 2:
            continue                    # violation residual: no category
        token = f[1]
        if token in STAGE_TOKENS:
            counts["STAGES"] += 1
        elif token in GRADE_TOKENS:
            counts[GRADE_TOKENS[token]] += 1
        # anything else: counted toward LINES, no category
    out = ("LINES %d\nSTAGES %d\nDONE %d\nUNDERFIRED %d\nOVERFIRED %d\n"
           % (total, counts["STAGES"], counts["DONE"],
              counts["UNDERFIRED"], counts["OVERFIRED"]))
    return (out, "", 0)


def simulate(sub, stdin):
    """Full contract incl. dispatch. `sub` is argv[1] (or '' for missing)."""
    if sub == "fire":
        return simulate_fire(stdin)
    if sub == "report":
        return simulate_report(stdin)
    return ("", USAGE, 2)               # missing / unknown / wrong-case / bare `-`


# --- hand-pinned traces used by several batches --------------------------------
# base 200: SOAK 600, SEAL 380, VENT 415, RAMP 830 -> DONE 830 (contribution 0)

T200_K1 = ("K1 SOAK 600\nK1 SEAL 380\nK1 VENT 415\nK1 RAMP 830\nK1 DONE 830\n")
T200_B2 = ("B2 SOAK 600\nB2 SEAL 380\nB2 VENT 415\nB2 RAMP 830\nB2 DONE 830\n")
# base 0: SOAK 0, SEAL -220, VENT -185, RAMP -370 -> G10 (contribution 2)
T0_K1 = ("K1 SOAK 0\nK1 SEAL -220\nK1 VENT -185\nK1 RAMP -370\n"
         "K1 G10 underfired below 500\n")
T0_B2 = ("B2 SOAK 0\nB2 SEAL -220\nB2 VENT -185\nB2 RAMP -370\n"
         "B2 G10 underfired below 500\n")
# base 396: SOAK 1188, SEAL 968, VENT 1003, RAMP 2006 -> G90 (contribution 3)
T396_K1 = ("K1 SOAK 1188\nK1 SEAL 968\nK1 VENT 1003\nK1 RAMP 2006\n"
           "K1 G90 overfired above 2000\n")

REP5 = "LINES %d\nSTAGES %d\nDONE %d\nUNDERFIRED %d\nOVERFIRED %d\n"


# --- batches: (name, sub, stdin, exp_stdout, exp_stderr, exp_exit) -------------
#
# Every expected triple is HAND-PINNED to PLAN.md, byte for byte. --self-check
# asserts the independent simulator reproduces each one.

BATCHES = [
    # ---- DISPATCH (closed-set residual) -------------------------------------
    # missing argv[1] -> usage even with valid-looking stdin (dispatch precedes work)
    ("disp_missing", "", "LOAD K1 200\n", "", USAGE, 2),
    ("disp_unknown", "bake", "", "", USAGE, 2),
    ("disp_wrong_case_fire", "FIRE", "", "", USAGE, 2),
    ("disp_wrong_case_report", "REPORT", "", "", USAGE, 2),
    ("disp_bare_dash", "-", "", "", USAGE, 2),

    # ---- FIRE: segmentation / terminator mechanics --------------------------
    ("fire_empty", "fire", "", "", "", 0),
    ("fire_trailing_lf", "fire", "LOAD K1 200\n", T200_K1, "", 0),
    ("fire_no_trailing_lf", "fire", "LOAD K1 200", T200_K1, "", 0),
    ("fire_double_trailing_lf", "fire", "LOAD K1 200\n\n", T200_K1,
     "E10 malformed record: empty line\n", 1),
    ("fire_blank_interior", "fire", "LOAD K1 200\n\nLOAD B2 200\n",
     T200_K1 + T200_B2, "E10 malformed record: empty line\n", 1),
    ("fire_lone_newline", "fire", "\n", "",
     "E10 malformed record: empty line\n", 1),
    ("fire_tab_is_data", "fire", "LOAD K1\t200\n", "",
     "E10 malformed record: LOAD expects 3 fields, got 2\n", 1),
    ("fire_cr_is_data", "fire", "LOAD K1 200\r\n", "",
     "E20 bad number: 200\r\n", 1),
    ("fire_collapse_runs", "fire", "  LOAD   K1  200 \n", T200_K1, "", 0),

    # ---- FIRE: record format (ordered checks, exhaustive set) ---------------
    ("fire_e10_count_hi", "fire", "LOAD K1 200 X\n", "",
     "E10 malformed record: LOAD expects 3 fields, got 4\n", 1),
    ("fire_e10_count_lo", "fire", "LOAD K1\n", "",
     "E10 malformed record: LOAD expects 3 fields, got 2\n", 1),
    ("fire_e25_unknown", "fire", "BAKE K1 200\n", "",
     "E25 unknown operation: BAKE\n", 1),
    ("fire_e25_before_count", "fire", "bogus\n", "",
     "E25 unknown operation: bogus\n", 1),
    ("fire_e15_bad_id", "fire", "LOAD k1 200\n", "",
     "E15 bad load id: k1\n", 1),
    ("fire_e15_short_id", "fire", "LOAD A 200\n", "",
     "E15 bad load id: A\n", 1),
    ("fire_e15_long_id", "fire", "LOAD ABCDEFGHI 200\n", "",
     "E15 bad load id: ABCDEFGHI\n", 1),
    ("fire_e20_leading_zero", "fire", "LOAD K1 0200\n", "",
     "E20 bad number: 0200\n", 1),
    ("fire_e20_not_number", "fire", "LOAD K1 12x\n", "",
     "E20 bad number: 12x\n", 1),
    ("fire_e15_before_e20", "fire", "LOAD k1 0200\n", "",
     "E15 bad load id: k1\n", 1),

    # ---- FIRE: lifecycle ordering (THE PROBE) --------------------------------
    # a wrong stage order fails every one of these: labels AND values diverge
    ("fire_trace_negative", "fire", "LOAD K1 0\n", T0_K1, "", 2),
    ("fire_boundary_done_low", "fire", "LOAD K1 145\n",
     "K1 SOAK 435\nK1 SEAL 215\nK1 VENT 250\nK1 RAMP 500\nK1 DONE 500\n",
     "", 0),
    ("fire_boundary_g10", "fire", "LOAD K1 144\n",
     "K1 SOAK 432\nK1 SEAL 212\nK1 VENT 247\nK1 RAMP 494\n"
     "K1 G10 underfired below 500\n", "", 2),
    ("fire_boundary_done_high", "fire", "LOAD K1 395\n",
     "K1 SOAK 1185\nK1 SEAL 965\nK1 VENT 1000\nK1 RAMP 2000\nK1 DONE 2000\n",
     "", 0),
    ("fire_boundary_g90", "fire", "LOAD K1 396\n", T396_K1, "", 3),
    ("fire_g90_large", "fire", "LOAD KILNA 1000\n",
     "KILNA SOAK 3000\nKILNA SEAL 2780\nKILNA VENT 2815\nKILNA RAMP 5630\n"
     "KILNA G90 overfired above 2000\n", "", 3),
    ("fire_two_records_grouping", "fire", "LOAD K1 200\nLOAD B2 0\n",
     T200_K1 + T0_B2, "", 2),
    ("fire_mixed_error_between", "fire", "LOAD K1 200\nBAKE X1 5\nLOAD B2 200\n",
     T200_K1 + T200_B2, "E25 unknown operation: BAKE\n", 1),
    ("fire_exit_error_vs_g90", "fire", "bogus\nLOAD K1 396\n", T396_K1,
     "E25 unknown operation: bogus\n", 3),
    ("fire_exit_g10_vs_error", "fire", "LOAD K1 0\nbogus\n", T0_K1,
     "E25 unknown operation: bogus\n", 2),

    # ---- REPORT: pipeline / dataflow trust boundary --------------------------
    ("rep_empty", "report", "", REP5 % (0, 0, 0, 0, 0), "", 0),
    ("rep_pipeline_done", "report", T200_K1, REP5 % (5, 4, 1, 0, 0), "", 0),
    ("rep_pipeline_mixed", "report", T200_K1 + T0_B2,
     REP5 % (10, 8, 1, 1, 0), "", 0),
    ("rep_g90_line", "report", "K1 G90 overfired above 2000\n",
     REP5 % (1, 0, 0, 0, 1), "", 0),
    ("rep_trusted_id", "report", "zz DONE 700\n",
     REP5 % (1, 0, 1, 0, 0), "", 0),
    ("rep_trusted_shape", "report", "K1 DONE 700 EXTRA\n",
     REP5 % (1, 0, 1, 0, 0), "", 0),
    ("rep_unknown_token", "report", "K1 BAKED 500\n",
     REP5 % (1, 0, 0, 0, 0), "", 0),
    ("rep_one_field", "report", "JUNK\n", REP5 % (1, 0, 0, 0, 0), "", 0),
    ("rep_blank_counts", "report", "K1 DONE 700\n\nK1 VENT 5\n",
     REP5 % (3, 1, 1, 0, 0), "", 0),
    ("rep_lone_newline", "report", "\n", REP5 % (1, 0, 0, 0, 0), "", 0),
    ("rep_no_trailing_lf", "report", "K1 DONE 700",
     REP5 % (1, 0, 1, 0, 0), "", 0),
    ("rep_all_stage_tokens", "report",
     "A1 RAMP 1\nA1 SEAL 2\nA1 SOAK 3\nA1 VENT 4\n",
     REP5 % (4, 4, 0, 0, 0), "", 0),
    ("rep_case_sensitive_token", "report", "K1 done 700\n",
     REP5 % (1, 0, 0, 0, 0), "", 0),
    ("rep_tab_is_data", "report", "K1\tDONE 700\n",
     REP5 % (1, 0, 0, 0, 0), "", 0),
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
    for name, sub, stdin, eo, ee, ex in BATCHES:
        got = run_sub(cmd, sub, stdin, timeout)
        total += 1
        if _cmp(name, got, eo, ee, ex, fails):
            passed += 1
    for line in fails:
        print(line)
    print("SCORE: %d/%d" % (passed, total))
    return passed, total


# --- self-check ---------------------------------------------------------------

def _structural_ok():
    """Procedure/registry/boundary invariants, independent of the batch table."""
    problems = []

    # the mandated order is a permutation of the op table's stages
    if sorted(ORDER) != sorted(OPS.keys()):
        problems.append("ORDER is not a permutation of the op table")

    # the mandated order differs from every rival prior a lazy reader might use
    for label, rival in RIVAL_ORDERS.items():
        if sorted(rival) != sorted(ORDER):
            problems.append("rival %s is not a permutation" % label)
        if rival == ORDER:
            problems.append("mandated order equals rival prior %s" % label)
    if ORDER == sorted(ORDER):
        problems.append("mandated order is alphabetical")
    if ORDER == sorted(ORDER, reverse=True):
        problems.append("mandated order is reverse-alphabetical")

    # every batch base: each rival order must diverge from the mandated trace
    # (labels always differ; assert the finals differ too, so even a
    # values-only comparison distinguishes them)
    bases = [0, 144, 145, 200, 395, 396, 1000]
    for base in bases:
        mandated = run_procedure(base, ORDER)
        for label, rival in RIVAL_ORDERS.items():
            got = run_procedure(base, rival)
            if got == mandated:
                problems.append("rival %s trace equals mandated for base %d"
                                % (label, base))
            if got[-1][1] == mandated[-1][1]:
                problems.append("rival %s FINAL equals mandated for base %d"
                                % (label, base))

    # grade boundary truth table
    cases = [(2001, "G90"), (2000, "DONE"), (500, "DONE"), (499, "G10"),
             (-370, "G10"), (830, "DONE")]
    for final, expect in cases:
        got = grade_of(final)[0]
        if got != expect:
            problems.append("grade_of(%d) = %s (expected %s)"
                            % (final, got, expect))

    # report token map covers exactly the stage + grade vocabulary
    if STAGE_TOKENS != frozenset(OPS.keys()):
        problems.append("report stage tokens != procedure stages")
    if set(GRADE_TOKENS) != {"DONE", "G10", "G90"}:
        problems.append("report grade tokens wrong")
    return problems


def selftest():
    bad = 0

    for msg in _structural_ok():
        print("SELFTEST: %s" % msg)
        bad += 1

    # the simulator must reproduce every hand-pinned batch expectation
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
