# Round 8 — Plan (DESIGN-BLOCKS tech-constraint probe: live Postgres)

**Status: PLAN ONLY — pre-registered before any spec/oracle authoring, per the
standing round discipline. No spec text or oracle code exists yet.**

## What this round tests, and why it can differentiate where 06–07 could not

Rounds 06–07 probed two of DESIGN-BLOCKS' three motivations (multi-spec
lifecycle procedures; algorithms) and closed both without experimental
mandate. The third motivation — **technology/stack constraints** ("the
service is already on Postgres") — was closed alongside them under the
recorded reasoning that it is *unfalsifiable by black-box one-shot outcome*.
**That reasoning is now corrected:** Docker is available on this machine
(server 29.6.1; `postgres:16` pulled), so the grader can hand every
implementation a live PostgreSQL DSN and then **query the database directly**
to verify the state actually landed in Postgres rather than SQLite, a flat
file, or process memory. Compliance is an observable, gradeable outcome. The
round-07 `wontfix` rows (`design-blocks-proposal`,
`slot-model-for-non-functional-specs`) keep their status **pending this
round's outcome**: a null here CONFIRMS them with the unfalsifiability excuse
removed; a corroborated differential REOPENS them (FINDINGS hygiene note at
the end).

**The structural difference from 06–07 — a placement differential, not a
carrier swap.** In both prior rounds the probed content had a natural
current-v1 home in both arms: the lifecycle order sat comfortably in a
coordinating spec's RETURN prose (round-06), the algorithm in guarded RETURN
obligations (round-07). Both times the arms differed only in *which container
carried the same well-homed text*, and both times the panel read either
container cleanly. A stack constraint's arm-B home is **contested**: it is
not an INPUT precondition, not a RETURN postcondition, not an observable
per-call effect — it must ride as SIDE-EFFECT/INVARIANT prose on a spec whose
slots mean something else, which is precisely the proposal's "forcing them
into a slot distorts the slot's meaning" claim. Round-08 therefore measures
whether the *distorted placement itself* degrades cold compliance — the first
probe where the proposal predicts a behavioral difference rather than a
comfort difference.

## Targets

1. **`design-blocks-proposal` motivation 2 (tech/stack constraints)** —
   deciding question: does a binding "state lives in THIS Postgres, and
   nowhere else" constraint survive cold one-shot implementation equally well
   when carried (a) in a `design:` block bound by `USES`, vs (b) as
   SIDE-EFFECT/INVARIANT obligations on a store spec, the strongest
   good-faith current-v1 home? Graded by **direct-SQL compliance**, not just
   observable stdout.
2. **`slot-model-for-non-functional-specs`** — the same result read from the
   slot side: if the slot-distorted placement (arm B) loses compliance that
   the design block keeps, the "slots fit non-functional content awkwardly"
   claim graduates from ergonomic to behavioral.

Non-targets (settled, carried as regression only): lifecycle ordering,
algorithm clarity, dataflow carriers, closed-set dispatch, segmentation.

## Domain

**Tool-crib ledger** (`crib`) — a workshop tool-crib stock ledger. Fresh vs
prior rounds (berth, apiary, vault, axle, weighbridge, kiln, granary).
**Genuine cross-process persistence is inherent to the domain:** each CLI
invocation is a separate process, and invocation N's writes MUST be visible
to invocation N+1 — impossible to fake with process memory, and the grader
runs invocations as separate processes by construction. **Algorithmic load is
deliberately LOW** (per-tool integer balances, no registries at scale, no
precedence chains): the probe target is the constraint, not another
algorithm.

## CLI shape (identical both arms, byte-exact)

- Dispatch: `argv[1]` ∈ closed set `{post, report}`; missing / unknown /
  wrong-case / bare `-` → `usage: crib {post|report}\n` to stderr, exit 2, no
  stdout. The usage path is decided before any database contact.
- **`crib post`** — reads ledger records from stdin (standard segmentation
  rules, the standing regression text): `STOCK <id> <qty>` adds qty to the
  tool's balance (creating it at 0 first); `ISSUE <id> <qty>` subtracts,
  **rejecting** an issue that would take the balance below zero (`E30
  insufficient stock: <id>`, balance unchanged, no row created; issuing a
  never-stocked tool is the balance-0 case of the same rule; an accepted
  record always leaves the tool's row present, so `ISSUE <new-id> 0` creates
  a balance-0 row). Per accepted record: `<id> OK <new-balance>` to stdout.
  **Tool-id grammar (pinned, the standing 06–07 rule):** 1–6 characters,
  first character A–Z, every character A–Z or 0–9; `<qty>` is the standing
  well-formed non-negative integer (digit run, no leading zero unless `0`).
  Record-format registry as in prior rounds: `E10 malformed record: empty
  line` / `E25 unknown operation: <field1>` / `E10 malformed record: <KW>
  expects 3 fields, got <n>` / `E15 bad tool id: <field2>` / `E20 bad
  number: <field3>` — ordered, one line per record, exhaustive (no dead
  residual). Exit 0, or 1 if any record was rejected (E30 counts as
  rejected). Every valid-subcommand invocation connects to the store and
  ensures the table exists before processing records (so E51 fires even on
  empty stdin, and an empty post still leaves the table created).
- **`crib report`** — ignores stdin content; writes one line `<id>
  <balance>` per tool **present in the store** (including balance 0 rows),
  sorted ascending by byte order of `<id>`, then `TOTAL <sum>` (the exact
  integer sum of the listed balances); exit 0. Balances are reported exactly
  as stored — including a balance another client set outside the E30 rule,
  such as a negative one (the shared-table-authority constraint made
  observable; TOTAL may be negative).
- **Store residuals (both arms, ERROR slots):** WHEN the `CRIB_PGDSN`
  environment variable is unset or empty → `E50 no database configured` to
  stderr, exit 2, nothing to stdout. WHEN it is set but the database cannot
  be reached or the connection is refused → `E51 database unavailable` to
  stderr, exit 2, nothing to stdout. (Both observable without/with a broken
  container — gradeable residuals per the closed-set discipline.)

## The constraint (identical semantics both arms — the probe content)

1. All ledger state MUST be persisted in the PostgreSQL database reachable
   via the connection URL in the **`CRIB_PGDSN`** environment variable (a
   standard `postgres://user:password@host:port/dbname` URL, provided by the
   operator at run time — pinned; any standard client library/driver accepts
   this form).
2. On first use the implementation MUST create its own table: **`crib_stock`**
   with columns `tool_id` (text, unique/primary) and `balance` (integer,
   64-bit range) — creation idempotent across invocations.
3. State MUST-NOT be persisted anywhere else: no files, no other databases,
   no caches that survive the process; each invocation is a fresh process and
   the database is the ONLY carrier of state between invocations.
4. The table is **shared**: rows inserted, updated, or deleted by other
   database clients between invocations are authoritative, and every
   invocation MUST reflect the table's current contents rather than any
   remembered or shadow copy.

(Point 4 is what makes shadow-state behaviorally fatal, not just
inspection-detectable — see the external-mutation batches.)

### Arm A (proposed language)

`design: CribStore`, **`type: stack`** (pinned — the proposal's own example
vocabulary for this motivation), carrying constraint points 1–4 verbatim as
its `content`; placed in `crib.shared.yass.yaml` (it spans both subcommands).
`crib.post` and `crib.report` each bind via `USES: ./crib.shared@CribStore`
(ref-only or attached); the E50/E51 ERROR obligations name the variable "the
CRIB_PGDSN variable the CribStore design requires". No spec→spec USES
anywhere; no `crib.store` spec exists.

### Arm B (good-faith current v1 — the contested home)

A **`crib.store` spec** in `crib.shared.yass.yaml` (same file, same position
— the cross-cutting-single-home guidance applied as strongly as current v1
allows), carrying points 1–4 as SIDE-EFFECT obligations (connects via
`CRIB_PGDSN`; creates `crib_stock` on first use; writes state only to that
database) and INVARIANT obligations (sole store; MUST-NOT persist elsewhere;
shared-table authority). `crib.post`/`crib.report` bind via
`USES: ./crib.shared@crib.store`. This is deliberately the placement the
proposal calls distorted — that distortion is the thing under test, so arm B
must be complete and unambiguous (all four points present verbatim) but
placed where current v1 puts them.

Everything else — dispatch, segmentation, record formats, error registry,
E50/E51, output formats — **byte-identical between arms, diff-verified**, per
the 06/07 method. Files per arm: `crib.shared.yass.yaml` (dispatch spec,
lines spec, errors spec, + CribStore design [A] / crib.store spec [B]),
`crib.post.yass.yaml`, `crib.report.yass.yaml`.

## Oracle and grading mechanics

One oracle grades both arms. **Container lifecycle owned by grade.py:** per
graded run, `docker run -d --rm -e POSTGRES_PASSWORD=… -p 127.0.0.1:<free
port>:5432 postgres:16` (image tag pinned for reproducibility), readiness by
polling `docker exec <ctr> pg_isready`; per batch, `DROP DATABASE IF EXISTS
crib_t; CREATE DATABASE crib_t;` via `docker exec <ctr> psql` so every batch
starts from a fresh database without container churn; container torn down at
the end of the run. The grader's direct SQL goes through `docker exec <ctr>
psql -tA` — **no host psql or Python driver needed by the grader.**

Each batch is a **sequence of steps** (this round's structural novelty —
persistence is multi-invocation by nature):

- `RUN(sub, stdin, expected stdout, expected stderr, expected exit,
  env-override?)` — one CLI invocation as a separate process, byte-exact
  comparison as always; `env-override` supports the E50 (var unset/empty) and
  E51 (DSN → closed port `postgres://…@127.0.0.1:1/x`) residual batches.
- `SQL(query, expected rows)` — **the compliance check**: after driving the
  CLI, assert via psql-in-container that `crib_stock` holds exactly the
  expected `(tool_id, balance)` rows. An implementation that shadow-writes to
  SQLite/files passes every RUN step and fails these.
- `MUTATE(statement)` — the grader itself UPDATEs/INSERTs/DELETEs rows
  between invocations; the following `RUN(report)` must reflect the change
  (constraint point 4). This makes non-compliance **behaviorally** fatal:
  shadow state reports stale values.
- `FILES()` — negative check: every candidate is executed with its CWD set to
  a fresh empty scratch directory; after the batch the directory must still
  be empty (state files in CWD are the common shadow pattern; combined with
  SQL/MUTATE this leaves no useful place to hide state).

Batch classes (~30–38 batches / ~70 steps):

1. **DISPATCH (5):** usual closed-set residual (usage, exit 2) — decided
   before DB contact, so these run with a valid DSN but require none.
2. **ENV/STORE residuals (3):** unset `CRIB_PGDSN` → E50; empty → E50;
   unreachable DSN → E51. Exit 2, no stdout, byte-exact.
3. **POST behavior (9):** STOCK create/accumulate; ISSUE math; E30
   insufficient (existing balance and never-stocked id); the five
   record-format errors; segmentation edges (tab/CR-as-data, blank interior,
   lone `"\n"`, empty input, missing trailing LF) — the standing regression
   surface.
4. **PERSISTENCE (5):** post → separate-process post (cumulative) → report;
   report sort order (ids fed unsorted); report on fresh DB (`TOTAL 0` only);
   zero-balance row still listed; a post/report/post/report interleave.
5. **DIRECT-SQL COMPLIANCE (4):** after each driven sequence above, SQL
   steps assert the exact rows — **the PRIMARY measurement**.
6. **EXTERNAL MUTATION (3):** grader UPDATE → report reflects; grader INSERT
   → new row appears in report; grader DELETE → row gone.
7. **NEGATIVE (2):** FILES() after a full post/report sequence; repeated
   first-run table creation (two fresh-DB sequences back-to-back —
   idempotent DDL).

Standing gates unchanged: `--self-check` → `SELFTEST OK` **offline** (the
embedded simulator is an in-memory table model that replays every batch's RUN
and SQL steps against hand-pinned expectations — no docker needed for
self-check); independent `ref.py` (a genuine `CRIB_PGDSN` client, coded
independently of the simulator) at full score against the live container
before any panel run; transient ref-check → `RESULT: CLEAN` (arm A: every
USES target a design block, zero spec→spec USES, no dead designs; arm B: no
design docs, all refs spec/slot).

### Operational preconditions (verify before authoring completes)

- Docker server 29.6.1 confirmed; `postgres:16` image present (pinned).
- Host has **no psql and no psycopg** (verified): the grader avoids both via
  docker-exec; **ref.py needs a real driver** → pre-flight installs
  `psycopg2-binary` (or `psycopg[binary]`) for python3 and smoke-tests a
  connect before the panel.
- Panel implementations must be able to reach Postgres from their chosen
  language: pre-flight verifies at minimum the Python driver (installed
  above, the panel's dominant choice 7/8 runs in rounds 06–07) and records
  whether Go/Rust drivers would need network module fetches (if a model's
  HOWTORUN build step fails on a missing driver, that is graded as builds=no
  and diagnosed separately — an environment artifact, not a spec-defect;
  noted now so it cannot be confused with non-compliance later).
- The spec (both arms) carries the full env-var contract — the cold prompt
  stays generic per standing method; the oracle provides `CRIB_PGDSN` at run
  time. Nothing about Postgres reaches the models outside the spec files.
- **Schema decision (lead-approved): PINNED.** The free-schema variant would
  weaken PRIMARY into inexact discovery heuristics with a
  grader-cannot-find-the-table failure mode indistinguishable from
  non-compliance; real stack constraints pin schemas; and both arms carry
  the identical pinned schema, so it cannot confound the placement
  differential.
- **Panel self-testing (lead directive):** panel agents receive NO
  `CRIB_PGDSN` and no container from us, but docker exists on the machine
  and agents run unrestricted — a model MAY self-provision a Postgres to
  test against. This is NOT prohibited (it leaks no spec knowledge and is
  equal-opportunity across arms). Per run, `agent.log` is inspected and
  whether the model self-tested against a live DB is recorded — descriptive
  context only, useful when diagnosing any miss. **Teardown:** after the
  panel, sweep for containers (and volumes) the agents left running.
- **Grading serialization:** the 8 runs are graded sequentially; grade.py
  uses one container per grader invocation with a dynamically chosen free
  host port AND a unique container name (pid + timestamp) so an aborted
  grade never collides with the next.

## Pre-registered measurement (fixed now, before authoring)

### PRIMARY — compliance differential

Per arm, count models whose state **actually lands in Postgres**: a model is
*compliant* iff it passes ALL direct-SQL and external-mutation batches
(classes 5–6); *non-compliant* iff it fails any of them while passing enough
of class 3–4 to show the behavior itself works (i.e., the state went
somewhere else). Thresholds per the standing ≥2-model rule:

- **Constraint-home claim VALIDATED (strongest pro-adoption result):** ≥2
  arm-B models non-compliant AND ≤0 arm-A models non-compliant (arm A clean).
- **Anti-adoption:** ≥2 arm-A models non-compliant AND arm B clean — the
  design block is *less* binding than slot prose.
- **Null on compliance:** both arms fully compliant (or any single-model
  non-compliance, tagged model-error per the standing rule and excluded from
  the differential).
- **Both arms ≥2 non-compliant:** not a carrier question — a language-level
  gap in expressing binding environmental constraints in ANY home → NEW
  actionable finding, distinct from the carrier comparison.

### SECONDARY — NOTES rubric (round-07 machinery, constraint-scoped)

Count NOTES items **about the store constraint** (where state must live,
whether Postgres is mandatory vs an example, the env-var contract, the
sole-store prohibition, shared-table authority — NOT segmentation, dispatch,
record math, or driver/library choice per se), classified
HEDGE / MISDESCRIPTION / RESTATEMENT exactly as in round-07. Thresholds
(fixed now): positive = ≥2 arm-B hedgers with ≤1 arm-A; reverse = ≥2 arm-A
hedgers with ≤1 arm-B; else null. Per the round-07 precedent, a NOTES
differential **alone** (compliance null) does NOT reopen the wontfix rows —
it is recorded as ergonomic-only evidence.

### TERTIARY — duration

Per-run wall time from meta.json, descriptive only, never decision-weight.

### Outcome → disposition table (including the round-07 wontfix rows)

| PRIMARY (compliance) | SECONDARY (NOTES) | Disposition |
|---|---|---|
| arm B ≥2 non-compliant, arm A clean | any | **REOPEN** `design-blocks-proposal` AND `slot-model-for-non-functional-specs`: the constraint-home claim is validated behaviorally — the slot-distorted placement loses real compliance that the design block keeps. Adopt the construct (execute the DESIGN-BLOCKS blast radius, tech-constraint motivation confirmed). K-counter resets (new actionable finding). |
| arm A ≥2 non-compliant, arm B clean | any | **CONFIRM wontfix + record anti-adoption evidence**: the design block is measurably worse at binding a constraint than current slots. `design-blocks-proposal` stays wontfix with strengthened rationale; recommend the proposal be dropped, not just deferred. K-counter: no new spec-defect (the current language WON) — counter advances. |
| both arms clean | null | **CONFIRM both wontfix rows with the unfalsifiability excuse removed** — the strongest possible close: the tech-constraint facet was falsifiable and no differential appeared. FINDINGS rows re-worded (see hygiene). Counter advances (fourth consecutive no-new-findings round). |
| both arms clean | positive (≥2 arm-B constraint-hedgers, ≤1 arm-A) | wontfix rows stay closed (r07 precedent: comfort alone does not reopen), but the ergonomic evidence is recorded in both rows — the first measured clarity differential across the three probes, noted for the owner's preference call. Counter advances. |
| both arms ≥2 non-compliant | any | **NEW finding** (`environmental-constraint-binding` or similar): neither home binds a stack constraint cold — a genuine language gap orthogonal to the carrier question. `design-blocks-proposal` stays wontfix (its construct didn't fix it either); the new finding opens. K-counter resets. |
| single-model non-compliance only | any | model-error per the ≥2 rule; treat as the both-clean row for disposition; the miss is diagnosed and ledgered as usual. |

### FINDINGS hygiene (correction this round makes regardless of outcome)

The round-07 close of `design-blocks-proposal` and
`slot-model-for-non-functional-specs` recorded "remaining rationale …
unfalsifiable by black-box one-shot outcome." **That was too broad:** the
tech-constraint facet IS falsifiable with a live container and direct-SQL
verification, as this round demonstrates. Whatever the outcome, both rows'
text is corrected in round-08's FINDINGS update — either re-worded to
"probed round-08, no differential" (confirm) or reopened per the table. The
rows keep `wontfix` status until this round's outcome is graded.

## Procedure

1. Lead reviews this PLAN. **No spec or oracle authoring until approved.**
2. Author both arms + oracle + ref + HOWTORUN per this PLAN; run the three
   standing local gates plus the operational pre-flight (driver install +
   live-container smoke of ref.py). **Commit spec+oracle before any panel
   run** per the round discipline.
3. Panel: 4 models × 2 arms, unique `/tmp/yass-r08-<model>-<arm>` workspaces,
   spec files only, generic cold prompt, concurrent; oracle provides
   `CRIB_PGDSN` at grading time only.
4. Grade all 8 with the container-backed oracle; apply the PRIMARY /
   SECONDARY / TERTIARY measurements exactly as pre-registered above.
5. Write `experiment/round-08/results.md` + FINDINGS updates per the
   disposition table; apply the K-counter rule stated in each cell.
