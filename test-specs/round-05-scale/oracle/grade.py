#!/usr/bin/env python3
"""Private oracle for round-05 (scale -- weighbridge truck-scale ticket session).

NEVER copied into an agent workspace. Black-box grades a candidate `scale` CLI by
byte-exact comparison of stdout / stderr / exit code over the full contract:

  - DISPATCH: missing / unknown / wrong-case argv[1] and a bare `-` -> usage,
    exit 2, no stdout (the closed-set dispatch rule governs `-`; no "stdin
    convention" override).
  - run: stdin operation stream (TARE / WEIGH) -> one ticket line per success,
    one error line per rejected record, exit = max severity. Exercises
    segmentation/terminator mechanics, the ordered per-record check sequence
    (E10/E25/E15/E20/E40/E50), cross-spec sequencing (per-id weigh-before-tare
    gate, re-tare last-wins), and the W01-W04 registry with first-match
    precedence (including the W03-vs-W04 deciding case).
  - audit: the consumer. Reads run's ticket output (+ hand-fed REWEIGH lines),
    tallies by class, exit 0. Exercises the blank-line OVERRIDE (a blank line is
    a section break -- skipped, contrast run where it is E10), the trust
    boundary (verdict text trusted, ids never re-validated), the guard
    conjunction four corners (REWEIGH carrier guard flag=Y AND W04 inner guard
    gross>50000), and the out-of-contract REWEIGH residual.

The embedded `simulate` (simulate_run / simulate_audit + dispatch) is the
authoritative reference semantics. It is written INDEPENDENTLY of oracle/ref.py
(different code shape -- data-driven registry vs ref's if-ladder) so a shared
coding bug cannot pass both. Every batch additionally carries a HAND-PINNED
expected (stdout, stderr, exit); `--self-check` asserts the simulator reproduces
every hand-pinned expectation (and a set of structural registry/boundary
invariants) and prints "SELFTEST OK". `--cmd '<run command>'` grades that command
against the hand-pinned expectations and prints "SCORE: passed/total".
"""

import argparse
import subprocess
import sys

# --- defect registry (scale.shared@scale.defects) -----------------------------
#
# code -> (class, verdict-message-tail). Guards live in classify() below.
# Precedence is highest -> lowest = W01, W02, W03, W04, then CLEAR. W04 carries an
# inner WHEN gross > 50000. Data-driven here; ref.py uses a straight if-ladder.

REGISTRY = {
    "W01": ("CRITICAL", "critical overload above 44000"),
    "W02": ("MAJOR",    "major overload above 40000"),
    "W03": ("MINOR",    "minor overload above 36000"),
    "W04": ("MINOR",    "heavy vehicle net above 30000"),
}
PRECEDENCE = ["W01", "W02", "W03", "W04"]

CLASS_OF = {code: row[0] for code, row in REGISTRY.items()}
MSG_OF = {code: row[1] for code, row in REGISTRY.items()}

CONTRIB = {"CRITICAL": 3, "MAJOR": 2, "MINOR": 1, "CLEAR": 0}

# audit verdict-token -> class (trust-boundary class map). TARE/NET ack -> CLEAR.
TOKEN_CLASS = {
    "TARE": "CLEAR", "NET": "CLEAR",
    "W01": "CRITICAL", "W02": "MAJOR", "W03": "MINOR", "W04": "MINOR",
}

USAGE = "usage: scale {run|audit}\n"

_UP = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DG = frozenset("0123456789")


# --- shared helpers -----------------------------------------------------------

def _records(stdin):
    """scale.shared@scale.lines segmentation: strip exactly one trailing LF if
    present, split on LF. Empty input (zero bytes) -> zero records. A blank line
    is preserved as a zero-field record (run's E10; audit overrides to skip)."""
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
    """Well-formed non-negative integer: non-empty digit run, no leading zero
    unless the literal 0."""
    if not tok or not set(tok) <= _DG:
        return False
    return tok == "0" or not tok.startswith("0")


def _is_id(tok):
    """3-10 chars, first char A-Z, every char A-Z or 0-9."""
    if len(tok) < 3 or len(tok) > 10:
        return False
    if tok[0] not in _UP:
        return False
    return set(tok) <= (_UP | _DG)


def classify(net, gross):
    """First-match by precedence over a well-formed weigh (net >= 0). Returns a
    registry code, or None for CLEAR. W04 conjoins its inner WHEN gross > 50000."""
    if net > 44000:
        return "W01"
    if net > 40000:
        return "W02"
    if net > 36000:
        return "W03"
    if gross > 50000 and net > 30000:
        return "W04"
    return None


# --- authoritative reference simulators (independent of ref.py) ---------------

def simulate_run(stdin):
    tare = {}
    out_lines = []
    err_lines = []
    exit_code = 0
    for rec in _records(stdin):
        f = _fields(rec)
        n = len(f)
        # 1. zero fields (blank / whitespace-only) -> E10 empty line
        if n == 0:
            err_lines.append("E10 malformed record: empty line")
            exit_code = max(exit_code, 1)
            continue
        kw = f[0]
        # 2. field 1 not a known keyword -> E25 (before the field-count check)
        if kw not in ("TARE", "WEIGH"):
            err_lines.append("E25 unknown operation: %s" % kw)
            exit_code = max(exit_code, 1)
            continue
        # 3/4. keyword present but field count != 3 -> E10 with keyword + count
        if n != 3:
            err_lines.append("E10 malformed record: %s expects 3 fields, got %d" % (kw, n))
            exit_code = max(exit_code, 1)
            continue
        vid = f[1]
        num = f[2]
        # 5. field 2 not a well-formed id -> E15
        if not _is_id(vid):
            err_lines.append("E15 bad vehicle id: %s" % vid)
            exit_code = max(exit_code, 1)
            continue
        # 6. field 3 not a well-formed integer -> E20
        if not _is_number(num):
            err_lines.append("E20 bad number: %s" % num)
            exit_code = max(exit_code, 1)
            continue
        value = int(num)
        if kw == "TARE":
            tare[vid] = value          # last tare wins; no error on re-tare
            out_lines.append("%s TARE %d" % (vid, value))
            continue                   # contribution 0
        # WEIGH:
        # 7. no prior TARE for this id (per-id precondition) -> E40
        if vid not in tare:
            err_lines.append("E40 weigh before tare: %s" % vid)
            exit_code = max(exit_code, 1)
            continue
        # 8. gross < tare -> negative net -> E50
        if value < tare[vid]:
            err_lines.append("E50 negative net: %s" % vid)
            exit_code = max(exit_code, 1)
            continue
        net = value - tare[vid]
        code = classify(net, value)
        if code is None:
            out_lines.append("%s NET %d" % (vid, net))   # CLEAR, contribution 0
        else:
            out_lines.append("%s %s %s" % (vid, code, MSG_OF[code]))
            exit_code = max(exit_code, CONTRIB[CLASS_OF[code]])
    return (_emit(out_lines), _emit(err_lines), exit_code)


def simulate_audit(stdin):
    counts = {"CLEAR": 0, "CRITICAL": 0, "MAJOR": 0, "MINOR": 0}
    tickets = 0
    for rec in _records(stdin):
        f = _fields(rec)
        if len(f) == 0:
            continue                   # OVERRIDE: blank line = section break, skipped
        tickets += 1
        if f[0] == "REWEIGH":
            # guarded carrier: re-derive only when 5-field AND flagged Y AND
            # gross/tare are well-formed; everything else is treated as
            # not-flagged -> CLEAR (out-of-contract residual; never errors).
            if len(f) == 5 and f[1:] and f[2] == "Y" \
                    and _is_number(f[3]) and _is_number(f[4]):
                gross = int(f[3])
                t = int(f[4])
                net = gross - t
                code = classify(net, gross)
                counts["CLEAR" if code is None else CLASS_OF[code]] += 1
            else:
                counts["CLEAR"] += 1
        else:
            token = f[1] if len(f) >= 2 else ""
            klass = TOKEN_CLASS.get(token)
            if klass is not None:
                counts[klass] += 1
            # unrecognized verdict token -> counted in TICKETS, no class bump
    out = ("TICKETS %d\nCLEAR %d\nCRITICAL %d\nMAJOR %d\nMINOR %d\n"
           % (tickets, counts["CLEAR"], counts["CRITICAL"],
              counts["MAJOR"], counts["MINOR"]))
    return (out, "", 0)


def simulate(sub, stdin):
    """Full contract incl. dispatch. `sub` is argv[1] (or '' for missing)."""
    if sub == "run":
        return simulate_run(stdin)
    if sub == "audit":
        return simulate_audit(stdin)
    return ("", USAGE, 2)              # missing / unknown / wrong-case / bare `-`


# --- batches: (name, sub, stdin, exp_stdout, exp_stderr, exp_exit) -------------
#
# Every expected triple is HAND-PINNED to PLAN.md, byte for byte. --self-check
# asserts the independent simulator reproduces each one.

BATCHES = [
    # ---- DISPATCH (closed-set residual) -------------------------------------
    # missing argv[1] -> usage even with valid-looking stdin (dispatch precedes work)
    ("disp_missing", "", "TARE AX1 30000\n", "", USAGE, 2),
    ("disp_unknown", "verify", "", "", USAGE, 2),
    ("disp_wrong_case_run", "RUN", "", "", USAGE, 2),
    ("disp_wrong_case_audit", "AUDIT", "", "", USAGE, 2),
    ("disp_bare_dash", "-", "", "", USAGE, 2),

    # ---- RUN: segmentation / terminator mechanics ---------------------------
    ("run_empty", "run", "", "", "", 0),
    ("run_trailing_lf_strip", "run", "TARE AX1 30000\n", "AX1 TARE 30000\n", "", 0),
    ("run_no_trailing_lf", "run", "TARE AX1 30000", "AX1 TARE 30000\n", "", 0),
    ("run_blank_interior", "run",
     "TARE AX1 30000\n\nTARE BX2 40000\n",
     "AX1 TARE 30000\nBX2 TARE 40000\n",
     "E10 malformed record: empty line\n", 1),
    ("run_lone_newline", "run", "\n", "",
     "E10 malformed record: empty line\n", 1),
    ("run_tab_is_data", "run", "TARE AX1\t30000\n", "",
     "E10 malformed record: TARE expects 3 fields, got 2\n", 1),
    ("run_cr_is_data", "run", "TARE AX1 30000\r\n", "",
     "E20 bad number: 30000\r\n", 1),
    ("run_collapse_runs", "run", "   TARE   AX1    30000   \n",
     "AX1 TARE 30000\n", "", 0),

    # ---- RUN: record format (CONFORMS transclusion) -------------------------
    ("run_e10_tare_count_hi", "run", "TARE AX1 30000 EXTRA\n", "",
     "E10 malformed record: TARE expects 3 fields, got 4\n", 1),
    ("run_e10_weigh_count_lo", "run", "WEIGH AX1\n", "",
     "E10 malformed record: WEIGH expects 3 fields, got 2\n", 1),
    ("run_e25_unknown", "run", "FOO AX1 30000\n", "",
     "E25 unknown operation: FOO\n", 1),
    ("run_e25_before_count", "run", "bogus\n", "",
     "E25 unknown operation: bogus\n", 1),
    ("run_e15_bad_id", "run", "TARE a1 30000\n", "",
     "E15 bad vehicle id: a1\n", 1),
    ("run_e20_bad_number", "run", "TARE AX1 040\n", "",
     "E20 bad number: 040\n", 1),
    ("run_e15_before_e20", "run", "TARE a1 040\n", "",
     "E15 bad vehicle id: a1\n", 1),

    # ---- RUN: cross-spec-sequencing (per-id weigh-before-tare gate) ---------
    ("run_weigh_no_tare", "run", "WEIGH AX1 70000\n", "",
     "E40 weigh before tare: AX1\n", 1),
    ("run_tare_then_weigh", "run",
     "TARE AX1 30000\nWEIGH AX1 70000\n",
     "AX1 TARE 30000\nAX1 W03 minor overload above 36000\n", "", 1),
    ("run_per_id_gate", "run",
     "TARE AX1 30000\nWEIGH AX2 70000\n",
     "AX1 TARE 30000\n", "E40 weigh before tare: AX2\n", 1),
    ("run_reuse_tare", "run",
     "TARE AX1 20000\nWEIGH AX1 60000\nWEIGH AX1 65000\n",
     "AX1 TARE 20000\n"
     "AX1 W03 minor overload above 36000\n"
     "AX1 W01 critical overload above 44000\n", "", 3),
    ("run_retare_last_wins", "run",
     "TARE AX1 30000\nTARE AX1 25000\nWEIGH AX1 60000\n",
     "AX1 TARE 30000\nAX1 TARE 25000\n"
     "AX1 W04 heavy vehicle net above 30000\n", "", 1),

    # ---- RUN: verdict registry + residual reachability ----------------------
    ("run_w01", "run", "TARE AX1 0\nWEIGH AX1 45000\n",
     "AX1 TARE 0\nAX1 W01 critical overload above 44000\n", "", 3),
    ("run_w02", "run", "TARE AX1 0\nWEIGH AX1 42000\n",
     "AX1 TARE 0\nAX1 W02 major overload above 40000\n", "", 2),
    ("run_w03", "run", "TARE AX1 0\nWEIGH AX1 38000\n",
     "AX1 TARE 0\nAX1 W03 minor overload above 36000\n", "", 1),
    ("run_w04", "run", "TARE AX1 20000\nWEIGH AX1 55000\n",
     "AX1 TARE 20000\nAX1 W04 heavy vehicle net above 30000\n", "", 1),
    ("run_clear", "run", "TARE AX1 10000\nWEIGH AX1 40000\n",
     "AX1 TARE 10000\nAX1 NET 30000\n", "", 0),
    ("run_w04_boundary_clear", "run", "TARE AX1 25000\nWEIGH AX1 55000\n",
     "AX1 TARE 25000\nAX1 NET 30000\n", "", 0),
    ("run_w03_beats_w04", "run", "TARE AX1 18000\nWEIGH AX1 55000\n",
     "AX1 TARE 18000\nAX1 W03 minor overload above 36000\n", "", 1),
    ("run_e50_negative_net", "run", "TARE AX1 30000\nWEIGH AX1 20000\n",
     "AX1 TARE 30000\n", "E50 negative net: AX1\n", 1),

    # ---- AUDIT: pipeline / trust-boundary -----------------------------------
    ("aud_empty", "audit", "",
     "TICKETS 0\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_ticket_block", "audit",
     "AX1 TARE 30000\n"
     "AX1 NET 30000\n"
     "AX2 W01 critical overload above 44000\n"
     "AX3 W02 major overload above 40000\n"
     "AX4 W03 minor overload above 36000\n"
     "AX5 W04 heavy vehicle net above 30000\n",
     "TICKETS 6\nCLEAR 2\nCRITICAL 1\nMAJOR 1\nMINOR 2\n", "", 0),
    ("aud_trusted_id", "audit", "z9 NET 5000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_unrecognized_token", "audit", "AX1 BOGUS extra stuff\n",
     "TICKETS 1\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),

    # ---- AUDIT: dispatch-subcommand-override (blank = section break) --------
    ("aud_blank_skip", "audit",
     "AX1 NET 30000\n\nAX2 W01 critical overload above 44000\n",
     "TICKETS 2\nCLEAR 1\nCRITICAL 1\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_lone_newline", "audit", "\n",
     "TICKETS 0\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_multi_blank", "audit", "\n\n\n",
     "TICKETS 0\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),

    # ---- AUDIT: guard-conjunction four corners ------------------------------
    ("aud_corner_tt", "audit", "REWEIGH AX1 Y 55000 20000\n",
     "TICKETS 1\nCLEAR 0\nCRITICAL 0\nMAJOR 0\nMINOR 1\n", "", 0),
    ("aud_corner_tf", "audit", "REWEIGH AX1 Y 50000 15000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_corner_ft", "audit", "REWEIGH AX1 N 55000 20000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_corner_ff", "audit", "REWEIGH AX1 N 50000 15000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    # flag Y enables the full registry, not only W04 (re-derive lands W01)
    ("aud_reweigh_critical", "audit", "REWEIGH AX1 Y 55000 5000\n",
     "TICKETS 1\nCLEAR 0\nCRITICAL 1\nMAJOR 0\nMINOR 0\n", "", 0),

    # ---- AUDIT: out-of-contract REWEIGH residual ----------------------------
    ("aud_resid_flag_x", "audit", "REWEIGH AX1 X 55000 20000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_resid_wrong_count_lo", "audit", "REWEIGH AX1 Y 55000\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),
    ("aud_resid_wrong_count_hi", "audit", "REWEIGH AX1 Y 55000 20000 EXTRA\n",
     "TICKETS 1\nCLEAR 1\nCRITICAL 0\nMAJOR 0\nMINOR 0\n", "", 0),

    # ---- AUDIT: combined mix (corners + tickets in one tally) ---------------
    ("aud_corner_mix", "audit",
     "AX0 NET 30000\n"
     "AX9 W01 critical overload above 44000\n"
     "REWEIGH AX1 Y 55000 20000\n"   # W04 -> MINOR
     "REWEIGH AX2 Y 50000 15000\n"   # inner false -> CLEAR
     "REWEIGH AX3 N 55000 20000\n"   # outer false -> CLEAR
     "REWEIGH AX4 N 50000 15000\n",  # CLEAR
     "TICKETS 6\nCLEAR 4\nCRITICAL 1\nMAJOR 0\nMINOR 1\n", "", 0),
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
    """Registry/boundary invariants, independent of the batch table."""
    problems = []

    # precedence lists every registry code exactly once
    if sorted(PRECEDENCE) != sorted(REGISTRY.keys()):
        problems.append("precedence does not list every code exactly once")
    if len(set(PRECEDENCE)) != len(PRECEDENCE):
        problems.append("precedence has a duplicate")
    if any(CLASS_OF[c] not in CONTRIB for c in REGISTRY):
        problems.append("a code maps to an unranked class")

    # classify first-match + boundary truth table: (net, gross) -> code/None
    cases = [
        (44001, 0, "W01"), (44000, 0, "W02"),       # W01 strict boundary
        (40001, 0, "W02"), (40000, 0, "W03"),       # W02 strict boundary
        (36001, 0, "W03"), (36000, 0, None),        # W03 strict boundary (low gross)
        (35000, 55000, "W04"),                      # W04 fires (inner+outer true)
        (35000, 50000, None),                       # gross == 50000 not > 50000 -> CLEAR
        (30001, 55000, "W04"), (30000, 55000, None),  # W04 inner strict boundary
        (37000, 55000, "W03"),                      # W03 beats W04 (deciding case)
        (45000, 0, "W01"), (42000, 0, "W02"), (38000, 0, "W03"),
        (30000, 40000, None),                       # low net low gross -> CLEAR
    ]
    for net, gross, expect in cases:
        if classify(net, gross) != expect:
            problems.append("classify(%d, %d) != %r (got %r)"
                            % (net, gross, expect, classify(net, gross)))

    # four-corner isolation: net 35000, only C-TT (Y, gross 55000) re-derives W04
    corners = [
        ("Y", 55000, 20000, "W04"),   # C-TT
        ("Y", 50000, 15000, None),    # C-TF: gross not > 50000
        ("N", 55000, 20000, None),    # C-FT: not flagged
        ("N", 50000, 15000, None),    # C-FF
    ]
    for flag, gross, t, expect in corners:
        net = gross - t
        if net != 35000:
            problems.append("corner net != 35000 for (%s,%d,%d)" % (flag, gross, t))
        got = classify(net, gross) if flag == "Y" else None
        if got != expect:
            problems.append("corner (%s,%d,%d) -> %r (expected %r)"
                            % (flag, gross, t, got, expect))
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
