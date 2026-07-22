#!/usr/bin/env python3
"""Private reference implementation of the round-08 `crib` CLI.

NEVER copied into an agent workspace. Used only to confirm the oracle scores a
known-correct program to full marks. Implemented INDEPENDENTLY of grade.py's
offline simulator: it is a genuine CRIB_PGDSN client that round-trips every
balance through real SQL (SELECT / INSERT ... ON CONFLICT / UPDATE), so a
shared coding bug in the dict-model simulator cannot pass both.

Constraint compliance, per the CribStore design / crib.store spec:
  - all state in the PostgreSQL at CRIB_PGDSN (standard postgres:// URL);
  - creates crib_stock(tool_id TEXT PRIMARY KEY, balance BIGINT) on first
    use, idempotently;
  - persists nothing anywhere else; commits before exit;
  - treats the table as shared: every invocation reads current contents.

Usage:
    CRIB_PGDSN=postgres://... python3 ref.py post    < records
    CRIB_PGDSN=postgres://... python3 ref.py report  < anything
    (missing/unknown/wrong-case/bare `-` subcommand) -> usage, exit 2
    (CRIB_PGDSN unset/empty) -> E50, exit 2; (unreachable) -> E51, exit 2
"""

import os
import sys

UPPER = set("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
DIGIT = set("0123456789")


def split_records(text):
    if text == "":
        return []
    if text.endswith("\n"):
        text = text[:-1]
    return text.split("\n")


def split_fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


def good_id(tok):
    if not (1 <= len(tok) <= 6):
        return False
    if tok[0] not in UPPER:
        return False
    return all((ch in UPPER) or (ch in DIGIT) for ch in tok)


def good_int(tok):
    if tok == "" or any(ch not in DIGIT for ch in tok):
        return False
    return tok == "0" or tok[0] != "0"


def post(conn, text, out, err):
    rejected = False
    cur = conn.cursor()
    for rec in split_records(text):
        fields = split_fields(rec)
        n = len(fields)
        if n == 0:
            err.append("E10 malformed record: empty line")
            rejected = True
            continue
        kw = fields[0]
        if kw != "STOCK" and kw != "ISSUE":
            err.append("E25 unknown operation: " + kw)
            rejected = True
            continue
        if n != 3:
            err.append("E10 malformed record: " + kw + " expects 3 fields, got " + str(n))
            rejected = True
            continue
        tid = fields[1]
        if not good_id(tid):
            err.append("E15 bad tool id: " + tid)
            rejected = True
            continue
        if not good_int(fields[2]):
            err.append("E20 bad number: " + fields[2])
            rejected = True
            continue
        qty = int(fields[2])

        # read the CURRENT stored balance (shared-table authority)
        cur.execute("SELECT balance FROM crib_stock WHERE tool_id = %s", (tid,))
        row = cur.fetchone()
        balance = row[0] if row is not None else 0

        if kw == "ISSUE" and qty > balance:
            err.append("E30 insufficient stock: " + tid)
            rejected = True
            continue

        new_balance = balance + qty if kw == "STOCK" else balance - qty
        cur.execute(
            "INSERT INTO crib_stock (tool_id, balance) VALUES (%s, %s) "
            "ON CONFLICT (tool_id) DO UPDATE SET balance = EXCLUDED.balance",
            (tid, new_balance))
        conn.commit()          # committed before exit, per record
        out.append(tid + " OK " + str(new_balance))
    return 1 if rejected else 0


def report(conn, out):
    cur = conn.cursor()
    cur.execute("SELECT tool_id, balance FROM crib_stock")
    rows = cur.fetchall()
    total = 0
    for tid, bal in sorted(rows, key=lambda r: r[0].encode("utf-8")):
        out.append(tid + " " + str(bal))
        total += bal
    out.append("TOTAL " + str(total))
    return 0


def main():
    sub = sys.argv[1] if len(sys.argv) > 1 else None
    if sub != "post" and sub != "report":
        sys.stderr.write("usage: crib {post|report}\n")
        sys.exit(2)

    dsn = os.environ.get("CRIB_PGDSN")
    if dsn is None or dsn == "":
        sys.stderr.write("E50 no database configured\n")
        sys.exit(2)

    data = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")

    import psycopg2
    try:
        conn = psycopg2.connect(dsn, connect_timeout=5)
    except psycopg2.OperationalError:
        sys.stderr.write("E51 database unavailable\n")
        sys.exit(2)

    try:
        cur = conn.cursor()
        cur.execute("CREATE TABLE IF NOT EXISTS crib_stock ("
                    "tool_id TEXT PRIMARY KEY, balance BIGINT NOT NULL)")
        conn.commit()

        out = []
        err = []
        if sub == "post":
            code = post(conn, data, out, err)
        else:
            code = report(conn, out)
    finally:
        conn.close()

    sys.stdout.buffer.write(
        "".join(s + "\n" for s in out).encode("utf-8", "surrogateescape"))
    sys.stderr.buffer.write(
        "".join(s + "\n" for s in err).encode("utf-8", "surrogateescape"))
    sys.exit(code)


if __name__ == "__main__":
    main()
