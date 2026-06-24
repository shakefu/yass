#!/usr/bin/env python3
"""Black-box oracle for the round-01 smoke spec ("double").

Usage:
    grade.py --cmd "<shell command that runs the built binary>"

The command is run once per case with the case's stdin piped in. We compare
stdout (exact), stderr (exact), and exit code (exact). Prints a per-case table
and a final "PASS N/M" line; exits 0 iff every case passes.
"""
import argparse
import subprocess
import sys

# (name, stdin, expected_stdout, expected_stderr, expected_exit)
CASES = [
    ("positive",      "21\n",     "42\n",  "",                       0),
    ("negative",      "-5\n",     "-10\n", "",                       0),
    ("zero",          "0\n",      "0\n",   "",                       0),
    ("no_newline",    "7",        "14\n",  "",                       0),
    ("surrounding_ws", "  8  \n", "16\n",  "",                       0),
    ("empty",         "",         "",      "error: no input\n",      2),
    ("whitespace",    "   \n",    "",      "error: no input\n",      2),
    ("not_int",       "abc\n",    "",      "error: invalid integer\n", 2),
    ("float",         "3.5\n",    "",      "error: invalid integer\n", 2),
]


def run_case(cmd, stdin):
    p = subprocess.run(
        cmd, shell=True, input=stdin.encode(),
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30,
    )
    return p.stdout.decode(), p.stderr.decode(), p.returncode


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cmd", required=True)
    args = ap.parse_args()

    passed = 0
    for name, stdin, exp_out, exp_err, exp_exit in CASES:
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
    total = len(CASES)
    print(f"\nPASS {passed}/{total}")
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
