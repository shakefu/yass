#!/usr/bin/env python3
"""Private black-box oracle for the round-02 apiary honey-harvest pipeline.

NEVER copied into an agent workspace. The candidate program is graded purely on
observable behaviour: for each subcommand, (stdin) -> (stdout, stderr, exit).

The candidate is a single CLI invoked as `<cmd> <subcommand>` where <subcommand>
is one of: tally, grade, pack. `--cmd` gives the BASE run command (no
subcommand); the oracle appends the subcommand itself.

Grading axes (every comparison is byte-exact on stdout, stderr, exit):
  1. per-stage with CANONICAL input  — feed each subcommand the oracle's
     canonical input for that stage; isolates whether the model's wire format
     and logic match the canonical reference.
  2. chained with the model's OWN intermediates — run tally|grade|pack threaded
     through the candidate's own stdout; a model whose stages are mutually
     self-consistent but diverge from canonical passes (1)'s tally check yet
     fails the chained check (or vice-versa). That gap measures how much the
     spec left the inter-stage dataflow to implementer discretion.

`simulate_*()` are the authoritative reference. SELFTEST guards the simulator
against itself with hand-written expectations.
"""

import argparse
import subprocess
import sys

# ----------------------------------------------------------------------------
# Domain reference (authoritative)
# ----------------------------------------------------------------------------

# grade bands by net weight (grams): A >= 400 ; 200 <= B <= 399 ; 1 <= C <= 199
BAND_A_MIN = 400
BAND_B_MIN = 200
# crate capacity per band (grams)
CAP = {"A": 1000, "B": 800, "C": 600}

TALLY_HEADER = "HARVEST/1"
GRADE_HEADER = "GRADED/1"
PACK_HEADER = "PACK/1"


def _is_digits_no_leading_zero(s):
    """ASCII digits only; no leading zero unless the value is the single '0'."""
    if not s or any(c not in "0123456789" for c in s):
        return False
    if len(s) > 1 and s[0] == "0":
        return False
    return True


def _is_hive_id(s):
    """2-6 chars; first an ASCII uppercase letter; all ASCII uppercase or digit."""
    if not (2 <= len(s) <= 6):
        return False
    if s[0] not in "ABCDEFGHIJKLMNOPQRSTUVWXYZ":
        return False
    return all(c in "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" for c in s)


def _split_records(stdin):
    """Records separated by a single LF. A single trailing LF after the final
    record is not an additional empty record. Field splitting is the caller's
    job; this only segments lines on LF (no CR handling -> off-spec CR is data)."""
    if stdin == "":
        return []
    body = stdin[:-1] if stdin.endswith("\n") else stdin
    return body.split("\n")


def _fields(line):
    """Split a record into fields on runs of one or more ASCII space (0x20),
    ignoring leading/trailing spaces. ASCII space only -- tab/NBSP are data."""
    return [f for f in line.split(" ") if f != ""]


# ----------------------------------------------------------------------------
# Stage simulators -> (stdout, stderr, exit)
# ----------------------------------------------------------------------------


def simulate_tally(stdin):
    out, err = [], []
    worst = 0
    out.append(TALLY_HEADER)
    for line in _split_records(stdin):
        f = _fields(line)
        if len(f) != 3:
            err.append(f"E10 malformed scan: expected 3 fields, got {len(f)}")
            worst = max(worst, 1)
            continue
        hive, gross, tare = f
        if not _is_hive_id(hive):
            err.append(f"E20 bad hive id: {hive}")
            worst = max(worst, 1)
            continue
        if not _is_digits_no_leading_zero(gross):
            err.append(f"E21 bad gross: {gross}")
            worst = max(worst, 1)
            continue
        if not _is_digits_no_leading_zero(tare):
            err.append(f"E22 bad tare: {tare}")
            worst = max(worst, 1)
            continue
        out.append(f"{hive} {gross} {tare}")
    return _join(out), _join(err), worst


def simulate_grade(stdin):
    records = _split_records(stdin)
    # sequencing gate: first line must be exactly the tally header
    if not records or records[0] != TALLY_HEADER:
        return "", _join([f"E40 bad stage header: expected {TALLY_HEADER}"]), 2
    out, err = [GRADE_HEADER], []
    worst = 0
    for line in records[1:]:
        f = _fields(line)
        if len(f) != 3 or not _is_digits_no_leading_zero(f[1]) or not _is_digits_no_leading_zero(f[2]):
            err.append(f"E90 unprocessable record: {line}")
            worst = max(worst, 1)
            continue
        hive, gross, tare = f[0], int(f[1]), int(f[2])
        if tare >= gross:
            err.append(f"E50 nonpositive net: {hive}")
            worst = max(worst, 1)
            continue
        net = gross - tare
        band = "A" if net >= BAND_A_MIN else "B" if net >= BAND_B_MIN else "C"
        out.append(f"{hive} {net} {band}")
    return _join(out), _join(err), worst


def simulate_pack(stdin):
    records = _split_records(stdin)
    if not records or records[0] != GRADE_HEADER:
        return "", _join([f"E41 bad stage header: expected {GRADE_HEADER}"]), 2
    err = []
    worst = 0
    totals = {"A": 0, "B": 0, "C": 0}
    counts = {"A": 0, "B": 0, "C": 0}
    for line in records[1:]:
        f = _fields(line)
        if len(f) != 3 or not _is_digits_no_leading_zero(f[1]) or f[2] not in CAP:
            err.append(f"E90 unprocessable record: {line}")
            worst = max(worst, 1)
            continue
        net, band = int(f[1]), f[2]
        would = totals[band] + net
        if would > CAP[band]:
            err.append(f"E70 crate overflow: band {band} total {would} exceeds {CAP[band]}")
            worst = max(worst, 1)
            continue
        totals[band] = would
        counts[band] += 1
    out = [PACK_HEADER]
    for band in ("A", "B", "C"):
        if counts[band] > 0:
            out.append(f"{band} {counts[band]} {totals[band]}")
    return _join(out), _join(err), worst


def _join(lines):
    """Render emitted lines: each line terminated by LF; empty list -> ''."""
    return "".join(s + "\n" for s in lines)


SIM = {"tally": simulate_tally, "grade": simulate_grade, "pack": simulate_pack}


# ----------------------------------------------------------------------------
# Test batches
# ----------------------------------------------------------------------------

# Each per-stage batch: (name, subcommand, stdin)
# Each pipeline batch: (name, raw_stdin)  -- threaded tally|grade|pack.

# A clean, all-accepted baseline. Per-band totals stay within capacity
# (A 910<=1000, B 719<=800, C 339<=600) so the whole pipeline exits 0. The
# net=200 B boundary is covered separately by grade_band_ties (no pack stage).
VALID_SCANS = (
    "HX1 920 410\n"   # net 510 -> A
    "HX2 700 380\n"   # net 320 -> B
    "BZ9 250 110\n"   # net 140 -> C
    "AA0 600 200\n"   # net 400 -> A (boundary)
    "QQ7 399 0\n"     # net 399 -> B (boundary, just under A)
    "QQ9 199 0\n"     # net 199 -> C (boundary)
)

STAGE_BATCHES = [
    # --- tally ---
    ("tally_clean", "tally", VALID_SCANS),
    ("tally_empty", "tally", ""),
    ("tally_trailing_nl", "tally", "HX1 920 410\n"),
    ("tally_no_trailing_nl", "tally", "HX1 920 410"),
    ("tally_blank_line", "tally", "HX1 920 410\n\nHX2 700 380\n"),
    ("tally_fieldcount", "tally", "HX1 920\nHX1 920 410 7\n"),
    ("tally_bad_hive", "tally", "h1 920 410\nTOOLONGID 920 410\nX 920 410\n"),
    ("tally_bad_gross", "tally", "HX1 09 410\nHX1 abc 410\nHX1 -5 410\n"),
    ("tally_bad_tare", "tally", "HX1 920 0410\nHX1 920 x\n"),
    ("tally_priority", "tally", "h1 09 0410\n"),  # multiple defects -> first wins (E20)
    # off-spec segmentation (must be rejected, not silently accepted)
    ("tally_tab", "tally", "HX1\t920\t410\n"),            # 1 field -> E10
    ("tally_crlf", "tally", "HX1 920 410\r\n"),            # tare '410\r' bad -> E22
    ("tally_nbsp", "tally", "HX1 920 410\n"),         # 2 fields -> E10
    ("tally_leading_space", "tally", "   HX1 920 410\n"),  # leading spaces ignored -> ok
    ("tally_repeated_space", "tally", "HX1   920    410\n"),  # runs collapse -> ok
    # --- grade (fed CANONICAL tally output) ---
    ("grade_clean", "grade", simulate_tally(VALID_SCANS)[0]),
    ("grade_header_only", "grade", TALLY_HEADER + "\n"),
    ("grade_nonpositive", "grade", TALLY_HEADER + "\nEQ1 500 500\nLT1 100 300\n"),
    ("grade_band_ties", "grade", TALLY_HEADER + "\nB1 400 0\nB2 399 0\nB3 200 0\nB4 199 0\n"),
    # sequencing gate
    ("grade_gate_raw", "grade", "HX1 920 410\nHX2 700 380\n"),     # no header -> E40 exit2
    ("grade_gate_wronghdr", "grade", "GRADED/1\nHX1 510 A\n"),     # wrong header -> E40 exit2
    ("grade_gate_empty", "grade", ""),                              # empty -> E40 exit2
    # residual reachable: malformed record lines past a valid header
    ("grade_residual", "grade", TALLY_HEADER + "\nHX1 510\nHX2 abc 10\nHX3 50 5\n"),
    # --- pack (fed CANONICAL grade output) ---
    ("pack_clean", "pack", simulate_grade(simulate_tally(VALID_SCANS)[0])[0]),
    ("pack_header_only", "pack", GRADE_HEADER + "\n"),
    ("pack_overflow_A", "pack", GRADE_HEADER + "\nA1 600 A\nA2 500 A\n"),  # 600 ok, 1100>1000 -> E70
    ("pack_overflow_B", "pack", GRADE_HEADER + "\nB1 300 B\nB2 300 B\nB3 300 B\n"),  # 600 ok, 900>800 E70
    ("pack_overflow_boundary", "pack", GRADE_HEADER + "\nA1 1000 A\nA2 1 A\n"),  # 1000 ok, 1001>1000 E70
    ("pack_multiband", "pack", GRADE_HEADER + "\nA1 500 A\nB1 250 B\nC1 100 C\nA2 400 A\n"),
    # sequencing gate
    ("pack_gate_raw", "pack", "A1 510 A\n"),
    ("pack_gate_wronghdr", "pack", TALLY_HEADER + "\nHX1 920 410\n"),
    ("pack_gate_empty", "pack", ""),
    # residual reachable
    ("pack_residual", "pack", GRADE_HEADER + "\nA1 500\nB1 xx B\nC1 100 Z\nA2 400 A\n"),
]

PIPELINE_BATCHES = [
    ("pipe_clean", VALID_SCANS),
    ("pipe_empty", ""),
    ("pipe_mixed_reject", "HX1 920 410\nbad row\nHX2 700 380\nEQ1 500 500\nBZ9 250 110\n"),
    ("pipe_overflow", "A1 1000 0\nA2 800 0\nA3 700 0\n"),  # nets 1000,800,700 all A; 1000 ok,1800>1000,...
    ("pipe_all_bands", VALID_SCANS + "MM1 1500 100\nNN2 260 60\nPP3 150 5\n"),
]


# ----------------------------------------------------------------------------
# Runner
# ----------------------------------------------------------------------------


def run_sub(cmd, sub, stdin, timeout):
    try:
        p = subprocess.run(
            f"{cmd} {sub}", shell=True, input=stdin.encode(),
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout,
        )
        return p.stdout, p.stderr, p.returncode
    except subprocess.TimeoutExpired:
        return b"", b"<timeout>", 124


def _cmp(label, got, exp_str_out, exp_str_err, exp_exit, fails):
    eo, ee = exp_str_out.encode(), exp_str_err.encode()
    go, ge, gx = got
    ok = go == eo and ge == ee and gx == exp_exit
    if not ok:
        fails.append(
            f"  {label}\n"
            f"    stdout exp={eo!r}\n    stdout got={go!r}\n"
            f"    stderr exp={ee!r}\n    stderr got={ge!r}\n"
            f"    exit   exp={exp_exit} got={gx}"
        )
    return ok


def grade(cmd, timeout):
    passed = total = 0
    fails = []

    # axis 1: per-stage with canonical input
    for name, sub, stdin in STAGE_BATCHES:
        eo, ee, ex = SIM[sub](stdin)
        total += 1
        if _cmp(f"stage/{name}", run_sub(cmd, sub, stdin, timeout), eo, ee, ex, fails):
            passed += 1

    # axis 2: chained through the candidate's OWN intermediate outputs
    for name, raw in PIPELINE_BATCHES:
        # canonical end state
        c1 = simulate_tally(raw)
        c2 = simulate_grade(c1[0])
        c3 = simulate_pack(c2[0])
        # candidate threaded
        m1 = run_sub(cmd, "tally", raw, timeout)
        m2 = run_sub(cmd, "grade", m1[0].decode("utf-8", "surrogateescape"), timeout)
        m3 = run_sub(cmd, "pack", m2[0].decode("utf-8", "surrogateescape"), timeout)
        total += 1
        if _cmp(f"pipe/{name}", m3, c3[0], c3[1], c3[2], fails):
            passed += 1

    for f in fails:
        print(f, file=sys.stderr)
    print(f"SCORE: {passed}/{total}")
    return passed == total


# ----------------------------------------------------------------------------
# SELFTEST -- hand-written expectations guarding the simulator
# ----------------------------------------------------------------------------


def selftest():
    cases = [
        # tally
        (simulate_tally, "HX1 920 410\n", ("HARVEST/1\nHX1 920 410\n", "", 0)),
        (simulate_tally, "", ("HARVEST/1\n", "", 0)),
        (simulate_tally, "HX1 920\n", ("HARVEST/1\n", "E10 malformed scan: expected 3 fields, got 2\n", 1)),
        (simulate_tally, "h1 920 410\n", ("HARVEST/1\n", "E20 bad hive id: h1\n", 1)),
        (simulate_tally, "HX1 09 410\n", ("HARVEST/1\n", "E21 bad gross: 09\n", 1)),
        (simulate_tally, "HX1 920 x\n", ("HARVEST/1\n", "E22 bad tare: x\n", 1)),
        (simulate_tally, "HX1\t920\t410\n", ("HARVEST/1\n", "E10 malformed scan: expected 3 fields, got 1\n", 1)),
        (simulate_tally, "HX1 920 410\r\n", ("HARVEST/1\n", "E22 bad tare: 410\r\n", 1)),
        # grade
        (simulate_grade, "HARVEST/1\nHX1 920 410\n", ("GRADED/1\nHX1 510 A\n", "", 0)),
        (simulate_grade, "HARVEST/1\nAA0 600 200\n", ("GRADED/1\nAA0 400 A\n", "", 0)),  # net 400 -> A boundary
        (simulate_grade, "HARVEST/1\nQQ7 399 0\n", ("GRADED/1\nQQ7 399 B\n", "", 0)),
        (simulate_grade, "HARVEST/1\nQQ8 200 0\n", ("GRADED/1\nQQ8 200 B\n", "", 0)),  # net 200 -> B boundary
        (simulate_grade, "HARVEST/1\nQQ9 199 0\n", ("GRADED/1\nQQ9 199 C\n", "", 0)),
        (simulate_grade, "HARVEST/1\nEQ1 500 500\n", ("GRADED/1\n", "E50 nonpositive net: EQ1\n", 1)),
        (simulate_grade, "HARVEST/1\nXX1 510\n", ("GRADED/1\n", "E90 unprocessable record: XX1 510\n", 1)),
        (simulate_grade, "HX1 920 410\n", ("", "E40 bad stage header: expected HARVEST/1\n", 2)),
        (simulate_grade, "", ("", "E40 bad stage header: expected HARVEST/1\n", 2)),
        # pack
        (simulate_pack, "GRADED/1\nHX1 510 A\n", ("PACK/1\nA 1 510\n", "", 0)),
        (simulate_pack, "GRADED/1\nA1 600 A\nA2 500 A\n",
         ("PACK/1\nA 1 600\n", "E70 crate overflow: band A total 1100 exceeds 1000\n", 1)),
        (simulate_pack, "GRADED/1\nA1 1000 A\nA2 1 A\n",
         ("PACK/1\nA 1 1000\n", "E70 crate overflow: band A total 1001 exceeds 1000\n", 1)),
        (simulate_pack, "GRADED/1\nA1 500 A\nB1 250 B\nC1 100 C\n",
         ("PACK/1\nA 1 500\nB 1 250\nC 1 100\n", "", 0)),
        (simulate_pack, "GRADED/1\nB1 xx B\n", ("PACK/1\n", "E90 unprocessable record: B1 xx B\n", 1)),
        (simulate_pack, "GRADED/1\nC1 100 Z\n", ("PACK/1\n", "E90 unprocessable record: C1 100 Z\n", 1)),
        (simulate_pack, "HARVEST/1\n", ("", "E41 bad stage header: expected GRADED/1\n", 2)),
        (simulate_pack, "", ("", "E41 bad stage header: expected GRADED/1\n", 2)),
    ]
    bad = 0
    for fn, stdin, exp in cases:
        got = fn(stdin)
        if got != exp:
            bad += 1
            print(f"SELFTEST FAIL {fn.__name__}({stdin!r})\n  exp={exp!r}\n  got={got!r}", file=sys.stderr)
    # cross-axis consistency: canonical chain composes
    raw = VALID_SCANS
    c1 = simulate_tally(raw)
    c2 = simulate_grade(c1[0])
    c3 = simulate_pack(c2[0])
    if c1[2] != 0 or c2[2] != 0 or c3[2] != 0:
        bad += 1
        print(f"SELFTEST FAIL canonical chain not clean: {c1[2]},{c2[2]},{c3[2]}", file=sys.stderr)
    if bad == 0:
        print("SELFTEST OK")
        return True
    print(f"SELFTEST: {bad} failure(s)", file=sys.stderr)
    return False


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cmd", help="base run command; oracle appends the subcommand")
    ap.add_argument("--self-check", action="store_true")
    ap.add_argument("--timeout", type=float, default=10.0)
    a = ap.parse_args()
    if a.self_check:
        sys.exit(0 if selftest() else 1)
    if not a.cmd:
        ap.error("--cmd is required unless --self-check")
    sys.exit(0 if grade(a.cmd, a.timeout) else 1)


if __name__ == "__main__":
    main()
