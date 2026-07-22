#!/usr/bin/env python3
"""Private oracle for round-08 (crib -- tool-crib ledger, live-Postgres
tech-constraint probe).

NEVER copied into an agent workspace. Grades a candidate `crib` CLI against a
LIVE PostgreSQL it provisions itself (docker, image pinned `postgres:16`).
Both spec arms (spec-arm-a: `design: CribStore`, type stack; spec-arm-b: a
`crib.store` spec carrying the same constraint in SIDE-EFFECT/INVARIANT) pin
the SAME behavior, so this one oracle grades both.

A batch is a SEQUENCE of steps -- persistence is multi-invocation by nature:

  RUN(sub, stdin, exp_out, exp_err, exp_exit, envmode)
      one CLI invocation as a separate process, byte-exact comparison.
      envmode: None (CRIB_PGDSN -> the live database), "unset", "empty",
      "badport" (DSN at a closed port) for the E50/E51 residuals.
  SQL(expected_rows)
      THE COMPLIANCE CHECK: `SELECT tool_id, balance FROM crib_stock ORDER BY
      tool_id COLLATE "C"` via psql inside the container must return exactly
      the expected rows. A shadow-state implementation (SQLite/file/memory)
      passes the RUN steps and fails here.
  MUTATE(op)
      the grader itself changes crib_stock between invocations (UPDATE /
      INSERT / DELETE); the next RUN must reflect it (shared-table
      authority) -- this makes shadow state BEHAVIORALLY fatal.
  FILES()
      every candidate runs with CWD set to a fresh scratch dir per batch;
      this step asserts the dir is still empty (no state files).

Container lifecycle (grading only; --self-check is fully OFFLINE): one
container per grader invocation, uniquely named (pid+timestamp) with a
dynamically chosen free host port, so an aborted grade never collides with
the next; per batch the database is dropped and recreated, so every batch
starts fresh. The grader's SQL goes through `docker exec ... psql` -- no host
psql or Python driver is needed by the grader.

The embedded offline simulator (an in-memory table model that replays every
step) is the authoritative reference semantics, written INDEPENDENTLY of
oracle/ref.py (dict-model here vs ref's real SQL round-trips). Every batch
carries HAND-PINNED expectations; `--self-check` asserts the simulator
reproduces each one plus structural invariants, and prints "SELFTEST OK"
without touching docker. `--cmd '<run command>'` grades and prints
"SCORE: passed/total".
"""

import argparse
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time

USAGE = "usage: crib {post|report}\n"
E50 = "E50 no database configured\n"
E51 = "E51 database unavailable\n"

IMAGE = "postgres:16"          # pinned for reproducibility
PGUSER = "postgres"
PGPASS = "oracle"
DBNAME = "crib_t"

_UP = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
_DG = frozenset("0123456789")


# --- shared helpers (crib.lines segmentation) ------------------------------------

def _records(stdin):
    if stdin == "":
        return []
    body = stdin[:-1] if stdin.endswith("\n") else stdin
    return body.split("\n")


def _fields(rec):
    return [tok for tok in rec.split(" ") if tok != ""]


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


# --- offline simulator (authoritative semantics; independent of ref.py) ----------

def sim_post(stdin, table):
    out, err = [], []
    rejected = False
    for rec in _records(stdin):
        f = _fields(rec)
        n = len(f)
        if n == 0:
            err.append("E10 malformed record: empty line")
            rejected = True
            continue
        kw = f[0]
        if kw not in ("STOCK", "ISSUE"):
            err.append("E25 unknown operation: %s" % kw)
            rejected = True
            continue
        if n != 3:
            err.append("E10 malformed record: %s expects 3 fields, got %d" % (kw, n))
            rejected = True
            continue
        tid, qty = f[1], f[2]
        if not _is_id(tid):
            err.append("E15 bad tool id: %s" % tid)
            rejected = True
            continue
        if not _is_number(qty):
            err.append("E20 bad number: %s" % qty)
            rejected = True
            continue
        q = int(qty)
        cur = table.get(tid, 0)
        if kw == "ISSUE" and q > cur:
            err.append("E30 insufficient stock: %s" % tid)
            rejected = True
            continue
        table[tid] = cur + q if kw == "STOCK" else cur - q
        out.append("%s OK %d" % (tid, table[tid]))
    o = "".join(s + "\n" for s in out)
    e = "".join(s + "\n" for s in err)
    return (o, e, 1 if rejected else 0)


def sim_report(table):
    lines = ["%s %d" % (tid, bal) for tid, bal in sorted(table.items())]
    lines.append("TOTAL %d" % sum(table.values()))
    return ("".join(s + "\n" for s in lines), "", 0)


def sim_run(sub, stdin, envmode, table):
    """Full contract: dispatch, then store residuals, then the subcommand."""
    if sub not in ("post", "report"):
        return ("", USAGE, 2)
    if envmode in ("unset", "empty"):
        return ("", E50, 2)
    if envmode == "badport":
        return ("", E51, 2)
    if sub == "post":
        return sim_post(stdin, table)
    return sim_report(table)


def sim_mutate(op, table):
    kind, tid, val = op
    if kind == "set":
        table[tid] = val
    elif kind == "insert":
        table[tid] = val
    elif kind == "delete":
        table.pop(tid, None)


# --- batches ----------------------------------------------------------------------
#
# step forms:
#   ("RUN", sub, stdin, exp_out, exp_err, exp_exit, envmode)
#   ("SQL", [(tool_id, balance), ...])   # rows sorted by tool_id, byte order
#   ("MUTATE", (op, tool_id, value_or_None))
#   ("FILES",)
# Every expectation is HAND-PINNED; --self-check replays all steps offline.

BATCHES = [
    # ---- DISPATCH (closed-set residual; decided before store contact) ----------
    ("disp_missing", [("RUN", "", "STOCK AB1 5\n", "", USAGE, 2, None)]),
    ("disp_unknown", [("RUN", "audit", "", "", USAGE, 2, None)]),
    ("disp_wrong_case_post", [("RUN", "POST", "", "", USAGE, 2, None)]),
    ("disp_wrong_case_report", [("RUN", "REPORT", "", "", USAGE, 2, None)]),
    ("disp_bare_dash", [("RUN", "-", "", "", USAGE, 2, None)]),

    # ---- ENV / store residuals --------------------------------------------------
    ("env_unset_post",
     [("RUN", "post", "STOCK AB1 5\n", "", E50, 2, "unset")]),
    ("env_empty_report",
     [("RUN", "report", "", "", E50, 2, "empty")]),
    # empty stdin: connect happens before processing, so E51 still fires
    ("env_badport_post_empty_stdin",
     [("RUN", "post", "", "", E51, 2, "badport")]),

    # ---- POST behavior + direct-SQL compliance ----------------------------------
    ("post_stock_create",
     [("RUN", "post", "STOCK AB1 5\n", "AB1 OK 5\n", "", 0, None),
      ("SQL", [("AB1", 5)])]),
    ("post_stock_accumulate",
     [("RUN", "post", "STOCK AB1 5\nSTOCK AB1 3\n",
       "AB1 OK 5\nAB1 OK 8\n", "", 0, None),
      ("SQL", [("AB1", 8)])]),
    ("post_issue_math",
     [("RUN", "post", "STOCK AB1 5\nISSUE AB1 2\n",
       "AB1 OK 5\nAB1 OK 3\n", "", 0, None),
      ("SQL", [("AB1", 3)])]),
    ("post_issue_to_zero",
     [("RUN", "post", "STOCK AB1 5\nISSUE AB1 5\n",
       "AB1 OK 5\nAB1 OK 0\n", "", 0, None),
      ("SQL", [("AB1", 0)])]),
    ("post_e30_insufficient",
     [("RUN", "post", "STOCK AB1 2\nISSUE AB1 3\n",
       "AB1 OK 2\n", "E30 insufficient stock: AB1\n", 1, None),
      ("SQL", [("AB1", 2)])]),
    ("post_e30_unknown_tool",
     [("RUN", "post", "ISSUE ZZ9 1\n", "",
       "E30 insufficient stock: ZZ9\n", 1, None),
      ("SQL", [])]),                    # rejected record creates no row
    ("post_issue_zero_unknown",
     [("RUN", "post", "ISSUE ZZ9 0\n", "ZZ9 OK 0\n", "", 0, None),
      ("SQL", [("ZZ9", 0)])]),          # accepted record always leaves a row
    ("post_fmt_errors",
     [("RUN", "post", "\nBORROW AB1 1\nSTOCK AB1 1 X\nSTOCK ab1 1\nSTOCK AB1 01\n",
       "",
       "E10 malformed record: empty line\n"
       "E25 unknown operation: BORROW\n"
       "E10 malformed record: STOCK expects 3 fields, got 4\n"
       "E15 bad tool id: ab1\n"
       "E20 bad number: 01\n", 1, None),
      ("SQL", [])]),
    ("post_id_grammar",
     [("RUN", "post", "STOCK ABCDEFG 1\nSTOCK 1AB 1\nSTOCK A 1\n",
       "A OK 1\n",
       "E15 bad tool id: ABCDEFG\nE15 bad tool id: 1AB\n", 1, None),
      ("SQL", [("A", 1)])]),            # 1-char id is valid; 7-char is not
    ("post_seg_edges",
     [("RUN", "post", "  STOCK   AB1  5 \nSTOCK\tCD2 5\nSTOCK CD2 5\r\n",
       "AB1 OK 5\n",
       "E25 unknown operation: STOCK\tCD2\nE20 bad number: 5\r\n", 1, None),
      ("SQL", [("AB1", 5)])]),
    ("post_lone_newline",
     [("RUN", "post", "\n", "", "E10 malformed record: empty line\n", 1, None),
      ("SQL", [])]),                    # table exists, no rows
    ("post_empty_input",
     [("RUN", "post", "", "", "", 0, None),
      ("SQL", [])]),                    # empty post still creates the table

    # ---- PERSISTENCE across separate processes ----------------------------------
    ("persist_two_posts",
     [("RUN", "post", "STOCK AB1 5\n", "AB1 OK 5\n", "", 0, None),
      ("RUN", "post", "STOCK AB1 3\nSTOCK CD2 7\n",
       "AB1 OK 8\nCD2 OK 7\n", "", 0, None),
      ("RUN", "report", "", "AB1 8\nCD2 7\nTOTAL 15\n", "", 0, None),
      ("SQL", [("AB1", 8), ("CD2", 7)]),
      ("FILES",)]),
    ("report_sort_order",
     [("RUN", "post", "STOCK ZZ9 1\nSTOCK AB1 2\nSTOCK M5 3\n",
       "ZZ9 OK 1\nAB1 OK 2\nM5 OK 3\n", "", 0, None),
      ("RUN", "report", "", "AB1 2\nM5 3\nZZ9 1\nTOTAL 6\n", "", 0, None)]),
    ("report_fresh_db",
     [("RUN", "report", "", "TOTAL 0\n", "", 0, None),
      ("SQL", [])]),
    ("report_zero_listed",
     [("RUN", "post", "STOCK AB1 4\nISSUE AB1 4\n",
       "AB1 OK 4\nAB1 OK 0\n", "", 0, None),
      ("RUN", "report", "", "AB1 0\nTOTAL 0\n", "", 0, None)]),
    ("persist_interleave",
     [("RUN", "post", "STOCK AB1 1\n", "AB1 OK 1\n", "", 0, None),
      ("RUN", "report", "", "AB1 1\nTOTAL 1\n", "", 0, None),
      ("RUN", "post", "ISSUE AB1 1\n", "AB1 OK 0\n", "", 0, None),
      ("RUN", "report", "", "AB1 0\nTOTAL 0\n", "", 0, None)]),

    # ---- EXTERNAL MUTATION (shared-table authority; shadow state fails HERE) ----
    ("mut_update_report_and_issue",
     [("RUN", "post", "STOCK AB1 5\n", "AB1 OK 5\n", "", 0, None),
      ("MUTATE", ("set", "AB1", 9)),
      ("RUN", "report", "", "AB1 9\nTOTAL 9\n", "", 0, None),
      # shadow state remembers 5 and wrongly answers E30 here:
      ("RUN", "post", "ISSUE AB1 6\n", "AB1 OK 3\n", "", 0, None),
      ("SQL", [("AB1", 3)])]),
    ("mut_insert",
     [("RUN", "report", "", "TOTAL 0\n", "", 0, None),
      ("MUTATE", ("insert", "QQ7", 4)),
      ("RUN", "report", "", "QQ7 4\nTOTAL 4\n", "", 0, None)]),
    ("mut_delete",
     [("RUN", "post", "STOCK AB1 5\nSTOCK CD2 2\n",
       "AB1 OK 5\nCD2 OK 2\n", "", 0, None),
      ("MUTATE", ("delete", "AB1", None)),
      ("RUN", "report", "", "CD2 2\nTOTAL 2\n", "", 0, None),
      ("SQL", [("CD2", 2)])]),
    ("mut_negative",
     [("RUN", "post", "STOCK AB1 5\n", "AB1 OK 5\n", "", 0, None),
      ("MUTATE", ("set", "AB1", -5)),
      ("RUN", "report", "", "AB1 -5\nTOTAL -5\n", "", 0, None)]),

    # ---- NEGATIVE / idempotence ---------------------------------------------------
    ("files_clean",
     [("RUN", "post", "STOCK AB1 5\n", "AB1 OK 5\n", "", 0, None),
      ("RUN", "report", "", "AB1 5\nTOTAL 5\n", "", 0, None),
      ("FILES",)]),
    ("ddl_idempotent",
     [("RUN", "report", "", "TOTAL 0\n", "", 0, None),
      ("RUN", "report", "", "TOTAL 0\n", "", 0, None),
      ("RUN", "post", "STOCK AB1 1\n", "AB1 OK 1\n", "", 0, None),
      ("SQL", [("AB1", 1)])]),
]

# batch-name prefixes that constitute the PRIMARY compliance measurement
COMPLIANCE_PREFIXES = ("mut_",)
COMPLIANCE_SQL = True  # any batch containing an SQL step also counts


# --- docker plumbing ----------------------------------------------------------------

def _docker(*args, check=True, timeout=60):
    p = subprocess.run(["docker"] + list(args), stdout=subprocess.PIPE,
                       stderr=subprocess.PIPE, timeout=timeout)
    if check and p.returncode != 0:
        raise RuntimeError("docker %s failed: %s" % (args[:2], p.stderr.decode()))
    return p


def _free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


class Container:
    def __init__(self):
        self.name = "crib-oracle-%d-%d" % (os.getpid(), int(time.time()))
        self.port = _free_port()
        self.dsn = "postgres://%s:%s@127.0.0.1:%d/%s" % (PGUSER, PGPASS,
                                                          self.port, DBNAME)

    def start(self):
        _docker("run", "-d", "--rm", "--name", self.name,
                "-e", "POSTGRES_PASSWORD=" + PGPASS,
                "-p", "127.0.0.1:%d:5432" % self.port, IMAGE)
        deadline = time.time() + 90
        while time.time() < deadline:
            p = _docker("exec", self.name, "psql", "-U", PGUSER,
                        "-d", "postgres", "-tA", "-c", "SELECT 1",
                        check=False, timeout=15)
            if p.returncode == 0 and p.stdout.strip() == b"1":
                return
            time.sleep(1)
        raise RuntimeError("postgres container did not become ready")

    def stop(self):
        _docker("rm", "-f", self.name, check=False)

    def admin_sql(self, stmt):
        return _docker("exec", self.name, "psql", "-U", PGUSER,
                       "-d", "postgres", "-tA", "-c", stmt)

    def sql(self, stmt, check=True):
        return _docker("exec", self.name, "psql", "-U", PGUSER,
                       "-d", DBNAME, "-tA", "-F", "|", "-c", stmt, check=check)

    def fresh_db(self):
        self.admin_sql("DROP DATABASE IF EXISTS %s WITH (FORCE)" % DBNAME)
        self.admin_sql("CREATE DATABASE %s" % DBNAME)

    def select_rows(self):
        """-> list of (tool_id, balance) or None if the table is unreadable."""
        p = self.sql('SELECT tool_id, balance FROM crib_stock '
                     'ORDER BY tool_id COLLATE "C"', check=False)
        if p.returncode != 0:
            return None
        rows = []
        for line in p.stdout.decode().splitlines():
            if line == "":
                continue
            tid, bal = line.split("|", 1)
            rows.append((tid, int(bal)))
        return rows

    def mutate(self, op):
        """-> True on success. Non-fatal: a candidate that never created the
        table makes the MUTATE fail; that is recorded as a step FAIL, not a
        grader crash."""
        kind, tid, val = op
        if kind == "set":
            stmt = "UPDATE crib_stock SET balance = %d WHERE tool_id = '%s'" % (val, tid)
        elif kind == "insert":
            stmt = ("INSERT INTO crib_stock (tool_id, balance) "
                    "VALUES ('%s', %d)" % (tid, val))
        else:
            stmt = "DELETE FROM crib_stock WHERE tool_id = '%s'" % tid
        return self.sql(stmt, check=False).returncode == 0


# --- candidate execution --------------------------------------------------------------

def run_candidate(cmd, sub, stdin, envmode, dsn, cwd, timeout):
    env = dict(os.environ)
    if envmode == "unset":
        env.pop("CRIB_PGDSN", None)
    elif envmode == "empty":
        env["CRIB_PGDSN"] = ""
    elif envmode == "badport":
        env["CRIB_PGDSN"] = "postgres://%s:%s@127.0.0.1:1/%s" % (PGUSER, PGPASS, DBNAME)
    else:
        env["CRIB_PGDSN"] = dsn
    full = (cmd + " " + sub) if sub else cmd
    try:
        p = subprocess.run(full, shell=True, cwd=cwd, env=env,
                           input=stdin.encode("utf-8", "surrogateescape"),
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           timeout=timeout)
        return (p.stdout, p.stderr, p.returncode)
    except subprocess.TimeoutExpired:
        return (b"", b"<timeout>", 124)


def _cmp_run(label, got, eo, ee, ex, fails):
    g_out, g_err, g_exit = got
    b_eo = eo.encode("utf-8", "surrogateescape")
    b_ee = ee.encode("utf-8", "surrogateescape")
    ok = (g_out == b_eo) and (g_err == b_ee) and (g_exit == ex)
    if not ok:
        fails.append("FAIL %s" % label)
        if g_out != b_eo:
            fails.append("  stdout exp %r got %r" % (b_eo, g_out))
        if g_err != b_ee:
            fails.append("  stderr exp %r got %r" % (b_ee, g_err))
        if g_exit != ex:
            fails.append("  exit   exp %r got %r" % (ex, g_exit))
    return ok


def grade(cmd, timeout):
    ctr = Container()
    print("starting %s as %s on port %d ..." % (IMAGE, ctr.name, ctr.port))
    ctr.start()
    total = passed = 0
    fails = []
    try:
        for name, steps in BATCHES:
            total += 1
            ctr.fresh_db()
            scratch = tempfile.mkdtemp(prefix="crib-scratch-")
            ok = True
            try:
                for i, step in enumerate(steps):
                    label = "%s[%d:%s]" % (name, i, step[0])
                    if step[0] == "RUN":
                        _, sub, stdin, eo, ee, ex, envmode = step
                        got = run_candidate(cmd, sub, stdin, envmode,
                                            ctr.dsn, scratch, timeout)
                        if not _cmp_run(label, got, eo, ee, ex, fails):
                            ok = False
                    elif step[0] == "SQL":
                        rows = ctr.select_rows()
                        if rows != step[1]:
                            fails.append("FAIL %s" % label)
                            fails.append("  rows exp %r got %r" % (step[1], rows))
                            ok = False
                    elif step[0] == "MUTATE":
                        if not ctr.mutate(step[1]):
                            fails.append("FAIL %s" % label)
                            fails.append("  mutate could not run (crib_stock missing?)")
                            ok = False
                    elif step[0] == "FILES":
                        left = sorted(os.listdir(scratch))
                        if left:
                            fails.append("FAIL %s" % label)
                            fails.append("  scratch dir not empty: %r" % left)
                            ok = False
            finally:
                shutil.rmtree(scratch, ignore_errors=True)
            if ok:
                passed += 1
    finally:
        ctr.stop()
    for line in fails:
        print(line)
    print("SCORE: %d/%d" % (passed, total))
    return passed, total


# --- self-check (fully offline) ---------------------------------------------------------

def _structural_ok():
    problems = []
    # id grammar truth table
    cases = [("A", True), ("AB1", True), ("Z9Z9Z9", True), ("ABCDEFG", False),
             ("1AB", False), ("ab1", False), ("", False), ("A_B", False)]
    for tok, expect in cases:
        if _is_id(tok) != expect:
            problems.append("_is_id(%r) != %r" % (tok, expect))
    # number grammar
    for tok, expect in [("0", True), ("10", True), ("01", False), ("", False),
                        ("1x", False), ("5\r", False)]:
        if _is_number(tok) != expect:
            problems.append("_is_number(%r) != %r" % (tok, expect))
    # E30 boundary: issue == balance is allowed, issue > balance rejected
    t = {"AB1": 5}
    o, e, x = sim_post("ISSUE AB1 5\n", dict(t))
    if (o, e, x) != ("AB1 OK 0\n", "", 0):
        problems.append("E30 boundary: issue==balance must be accepted")
    o, e, x = sim_post("ISSUE AB1 6\n", dict(t))
    if (o, e, x) != ("", "E30 insufficient stock: AB1\n", 1):
        problems.append("E30 boundary: issue>balance must be rejected")
    # report sort is byte order
    o, _, _ = sim_report({"ZZ9": 1, "AB1": 2, "M5": 3})
    if o != "AB1 2\nM5 3\nZZ9 1\nTOTAL 6\n":
        problems.append("report sort order wrong: %r" % o)
    # every batch with a MUTATE or SQL step is fresh-DB self-consistent (replay
    # happens in selftest); here just confirm at least one of each step kind
    kinds = {s[0] for _, steps in BATCHES for s in steps}
    for k in ("RUN", "SQL", "MUTATE", "FILES"):
        if k not in kinds:
            problems.append("no %s step in any batch" % k)
    return problems


def selftest():
    bad = 0
    for msg in _structural_ok():
        print("SELFTEST: %s" % msg)
        bad += 1
    # replay every batch offline against the simulator table model
    for name, steps in BATCHES:
        table = {}
        for i, step in enumerate(steps):
            label = "%s[%d:%s]" % (name, i, step[0])
            if step[0] == "RUN":
                _, sub, stdin, eo, ee, ex, envmode = step
                go, ge, gx = sim_run(sub, stdin, envmode, table)
                if (go, ge, gx) != (eo, ee, ex):
                    print("SELFTEST mismatch: %s" % label)
                    print("  exp %r %r %r" % (eo, ee, ex))
                    print("  got %r %r %r" % (go, ge, gx))
                    bad += 1
            elif step[0] == "SQL":
                rows = sorted(table.items())
                if rows != step[1]:
                    print("SELFTEST mismatch: %s rows exp %r got %r"
                          % (label, step[1], rows))
                    bad += 1
            elif step[0] == "MUTATE":
                sim_mutate(step[1], table)
            # FILES: no-op offline
    if bad:
        print("SELFTEST FAILED (%d)" % bad)
        return False
    print("SELFTEST OK")
    return True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cmd", help="base run command for the candidate program")
    ap.add_argument("--self-check", action="store_true",
                    help="run the oracle's OFFLINE self-test and exit (no docker)")
    ap.add_argument("--timeout", type=float, default=20.0)
    args = ap.parse_args()

    if args.self_check:
        sys.exit(0 if selftest() else 1)

    if not args.cmd:
        ap.error("--cmd is required unless --self-check is given")

    passed, total = grade(args.cmd, args.timeout)
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
