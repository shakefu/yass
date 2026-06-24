#!/usr/bin/env python3
"""Black-box oracle for the round-01 berth spec.

Usage:
    grade.py --cmd "<shell command that runs the built program>"

The candidate program reads assignment-request records from stdin (one per
line) and writes OK lines to stdout / error lines to stderr, exiting with the
worst per-record outcome. This oracle embeds an authoritative reference
implementation (`simulate`) that encodes the one true reading of the spec. For
each test batch it computes the expected (stdout, stderr, exit) from the
reference, runs the candidate on the same stdin, and compares all three exactly.

A self-check (`SELFTEST`) pins hand-verified expectations for the trickiest
batches so a bug in the reference is caught before any candidate is graded.

This file is NEVER copied into an agent workspace.
"""
import argparse
import re
import subprocess
import sys

# --- dock table (per dock letter) -------------------------------------------
# letter -> (capacity, open_slot_inclusive, close_slot_exclusive) or None if closed
DOCKS = {
    "A": (500, 12, 36),
    "B": (800, 0, 48),
    "C": (1200, 16, 40),
    "D": (300, 12, 36),
    "E": (1000, 8, 44),
    "F": None,  # closed
}

SHIP_RE = re.compile(r"^[A-Za-z][A-Za-z0-9]{2,7}$")   # 3..8 chars, first letter
DOCK_RE = re.compile(r"^[A-F][0-9]{2}$")
MASS_RE = re.compile(r"^[1-9][0-9]*$")                 # digits, no leading zero, >=1
INT_RE = re.compile(r"^(0|[1-9][0-9]*)$")              # 0, or no-leading-zero


def split_records(stdin):
    """Split stdin into records. A single trailing newline does not create an
    extra empty record; a blank line in the middle is a (malformed) record."""
    if stdin == "":
        return []
    parts = stdin.split("\n")
    if parts and parts[-1] == "":
        parts = parts[:-1]
    return parts


def split_fields(line):
    """Split on runs of one or more ASCII spaces, ignoring leading/trailing."""
    return [t for t in line.split(" ") if t != ""]


def window_ok(ws, we):
    if not (INT_RE.match(ws) and INT_RE.match(we)):
        return False
    a, b = int(ws), int(we)
    return 0 <= a <= 47 and 0 <= b <= 47 and a < b


def simulate(stdin):
    """Authoritative reference. Returns (stdout, stderr, exit)."""
    out_lines = []
    err_lines = []
    exit_code = 0
    accepted = []  # list of (letter, start, end) for accepted records

    for line in split_records(stdin):
        fields = split_fields(line)

        # (1) field count
        if len(fields) != 5:
            err_lines.append(
                f"E10 malformed record: expected 5 fields, got {len(fields)}"
            )
            exit_code = max(exit_code, 2)
            continue

        ship, dock, mass, ws, we = fields

        # (2) ship-id
        if not SHIP_RE.match(ship):
            err_lines.append(f"E20 bad ship id: {ship}")
            exit_code = max(exit_code, 1)
            continue
        # (3) dock-code
        if not DOCK_RE.match(dock):
            err_lines.append(f"E21 bad dock code: {dock}")
            exit_code = max(exit_code, 1)
            continue
        # (4) mass
        if not MASS_RE.match(mass):
            err_lines.append(f"E22 bad mass: {mass}")
            exit_code = max(exit_code, 1)
            continue
        # (5) window
        if not window_ok(ws, we):
            err_lines.append(f"E23 bad window: {ws}-{we}")
            exit_code = max(exit_code, 1)
            continue

        letter = dock[0]
        info = DOCKS[letter]

        # (6) closed
        if info is None:
            err_lines.append(f"E30 dock closed: {dock}")
            exit_code = max(exit_code, 1)
            continue
        cap, open_s, close_s = info

        # (7) capacity
        if int(mass) > cap:
            err_lines.append(f"E31 over capacity: {dock} mass {mass} exceeds {cap}")
            exit_code = max(exit_code, 1)
            continue

        # (8) conflict vs earlier accepted on same letter (half-open overlap)
        a, b = int(ws), int(we)
        conflict = any(
            l == letter and a < e and s < b for (l, s, e) in accepted
        )
        if conflict:
            err_lines.append(f"E32 window conflict: {dock} {ws}-{we}")
            exit_code = max(exit_code, 1)
            continue

        # assignment: must be within operating hours, else catch-all E90
        if a >= open_s and b <= close_s:
            out_lines.append(f"OK {ship} {dock} {ws}-{we}")
            accepted.append((letter, a, b))
        else:
            err_lines.append(f"E90 unprocessable: {dock} {ws}-{we}")
            exit_code = max(exit_code, 1)

    stdout = "".join(s + "\n" for s in out_lines)
    stderr = "".join(s + "\n" for s in err_lines)
    return stdout, stderr, exit_code


# --- test batches (name -> stdin) -------------------------------------------
BATCHES = {
    "all_accept": (
        "SHIP1 A20 400 12 20\n"
        "VESSEL2 B07 750 0 47\n"
        "Probe3 C11 1100 16 40\n"
    ),
    "format_errors": (
        "A20 400 12 20\n"          # 4 fields -> E10 got 4 (exit 2)
        "99 B07 400 0 10\n"        # ship bad -> E20
        "SHIP1 G07 400 0 10\n"     # dock bad -> E21
        "SHIP1 A20 0 0 10\n"       # mass 0 -> E22
        "SHIP1 A20 400 20 12\n"    # start>=end -> E23
    ),
    "dock_rules_state": (
        "SHIP1 F01 100 0 10\n"     # closed -> E30
        "SHIP2 D05 500 12 20\n"    # over cap (cap 300) -> E31
        "SHIP3 A10 400 12 24\n"    # accept
        "SHIP4 A11 400 20 30\n"    # conflict with A10 (same letter A) -> E32
        "SHIP5 A12 400 24 30\n"    # adjacent, no conflict -> accept
    ),
    "catch_all_hours": (
        "SHIP1 A20 400 0 6\n"      # within fields, outside A hours [12,36) -> E90
        "SHIP2 C09 400 16 41\n"    # we=41 > close 40 -> E90
        "SHIP3 B07 400 0 47\n"     # within B hours -> accept
    ),
    "priority_ties": (
        "9 G07 0 50 10\n"          # ship+dock+mass+window all bad -> E20 wins
        "SHIP1 F01 9999 0 10\n"    # closed beats over-capacity -> E30
        "SHIP2 A20 400 0 50\n"     # window malformed (50>47) beats hours -> E23
        "SHIP3 D05 500 0 6\n"      # over cap beats outside-hours -> E31
    ),
    "interleave_mustnot": (
        "SHIP1 A20 400 12 20\n"    # accept
        "BAD G07 400 0 10\n"       # E21, no stdout line
        "SHIP2 B07 400 0 10\n"     # accept
    ),
    "empty_input": "",
    "trailing_newline": "SHIP1 A20 400 12 20\n",
    "blank_line_mid": (
        "SHIP1 A20 400 12 20\n"
        "\n"                       # blank middle line -> E10 got 0 (exit 2)
        "SHIP2 B07 400 0 10\n"
    ),
    "whitespace_split": "  SHIP1   A20  400 12   20  \n",
}

# --- self-check: hand-verified expectations for tricky batches --------------
# (batch_name, expected_stdout, expected_stderr, expected_exit)
SELFTEST = [
    (
        "priority_ties",
        "",
        "E20 bad ship id: 9\n"
        "E30 dock closed: F01\n"
        "E23 bad window: 0-50\n"
        "E31 over capacity: D05 mass 500 exceeds 300\n",
        1,
    ),
    (
        "dock_rules_state",
        "OK SHIP3 A10 12-24\nOK SHIP5 A12 24-30\n",
        "E30 dock closed: F01\n"
        "E31 over capacity: D05 mass 500 exceeds 300\n"
        "E32 window conflict: A11 20-30\n",
        1,
    ),
    (
        "catch_all_hours",
        "OK SHIP3 B07 0-47\n",
        "E90 unprocessable: A20 0-6\nE90 unprocessable: C09 16-41\n",
        1,
    ),
    (
        "format_errors",
        "",
        "E10 malformed record: expected 5 fields, got 4\n"
        "E20 bad ship id: 99\n"
        "E21 bad dock code: G07\n"
        "E22 bad mass: 0\n"
        "E23 bad window: 20-12\n",
        2,
    ),
    (
        "interleave_mustnot",
        "OK SHIP1 A20 12-20\nOK SHIP2 B07 0-10\n",
        "E21 bad dock code: G07\n",
        1,
    ),
    (
        "blank_line_mid",
        "OK SHIP1 A20 12-20\nOK SHIP2 B07 0-10\n",
        "E10 malformed record: expected 5 fields, got 0\n",
        2,
    ),
    (
        "whitespace_split",
        "OK SHIP1 A20 12-20\n",
        "",
        0,
    ),
    ("empty_input", "", "", 0),
    ("trailing_newline", "OK SHIP1 A20 12-20\n", "", 0),
]


def run_self_check():
    ok = True
    for name, exp_out, exp_err, exp_exit in SELFTEST:
        out, err, code = simulate(BATCHES[name])
        if (out, err, code) != (exp_out, exp_err, exp_exit):
            ok = False
            print(f"SELFTEST FAIL {name}", file=sys.stderr)
            if out != exp_out:
                print(f"  stdout: want {exp_out!r} got {out!r}", file=sys.stderr)
            if err != exp_err:
                print(f"  stderr: want {exp_err!r} got {err!r}", file=sys.stderr)
            if code != exp_exit:
                print(f"  exit:   want {exp_exit} got {code}", file=sys.stderr)
    return ok


def run_case(cmd, stdin):
    p = subprocess.run(
        cmd, shell=True, input=stdin.encode(),
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30,
    )
    return p.stdout.decode(), p.stderr.decode(), p.returncode


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cmd", required=True)
    ap.add_argument("--self-check", action="store_true",
                    help="only run the reference self-check, do not grade")
    args = ap.parse_args()

    if not run_self_check():
        print("\nERROR: reference self-check failed; oracle is not trustworthy",
              file=sys.stderr)
        sys.exit(3)

    if args.self_check:
        print("SELFTEST OK")
        sys.exit(0)

    passed = 0
    names = list(BATCHES)
    for name in names:
        stdin = BATCHES[name]
        exp_out, exp_err, exp_exit = simulate(stdin)
        try:
            out, err, code = run_case(args.cmd, stdin)
        except Exception as e:  # noqa: BLE001
            print(f"FAIL {name}: execution error: {e}")
            continue
        ok = out == exp_out and err == exp_err and code == exp_exit
        if ok:
            passed += 1
            print(f"PASS {name}")
        else:
            print(f"FAIL {name}")
            if out != exp_out:
                print(f"  stdout: want {exp_out!r} got {out!r}")
            if err != exp_err:
                print(f"  stderr: want {exp_err!r} got {err!r}")
            if code != exp_exit:
                print(f"  exit:   want {exp_exit} got {code}")
    total = len(names)
    print(f"\nPASS {passed}/{total}")
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
