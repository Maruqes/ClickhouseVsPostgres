# Plan 01 — Minimal gym reservation race

Date: 2026-10-01
Status: Implemented on 2026-10-01. Shared page/API, additive migrations, real database contracts and six-service smoke verification complete.

## User-confirmed scope

Another page in the same shared app, available on both deployments:

- Exactly one gym class, such as Spinning.
- Show 20 seats: 19 booked, one free.
- One button: **Try 20 reservations**.
- Each click launches 20 independent synthetic booking attempts competing for the remaining seat.
- Show a compact table: attempt number, accepted/rejected/error, time taken, and rejection/error reason.
- Show totals for accepted, rejected, and overbooked seats. Report infrastructure errors separately so all 20 attempts are accounted for.
- A subsequent click starts fresh with 19 booked seats and replaces displayed results.
- No users, authentication, profiles, member tables, class browsing, history screen, or reset button.

## Correctness demonstration and scoped parity exception

PostgreSQL uses a protected booking operation: in a healthy completed run, exactly one attempt is accepted, 19 are rejected as class full, and there are 20 final reservations.

ClickHouse deliberately uses an unprotected check-then-insert operation. Competing attempts can see the same remaining seat and insert multiple reservations. Report the actual result; overbooking is possible, not guaranteed on every run.

The user explicitly authorized this correctness difference. Relax AGENTS.md's parity requirement only for the booking race outcome. All other routes, input validation, fixtures, UI, concurrency settings, response shapes, and analytics semantics remain shared. During implementation, document this narrow exception in AGENTS.md and README.md.

Use one shared frontend/image and one shared Go backend/image. Engine-specific SQL and reservation strategies belong under `backend/internal/database/`; migrations belong under `backend/migrations/`. Preserve exactly six Docker services. Both apps show both pages, at localhost:3001 and localhost:3002.

## Minimal page

Proposed route: `/classes`, with simple navigation to the exercise analytics page. Integrate with the map implementation already present; do not overwrite unrelated work.

Show a single class card with seat indicators, its capacity/booking counts, and the button. On click, clear old results, disable the button, and show a simple running state. After completion, show the result table ordered by attempt number and update the stored booking/overbooking counts. Extra accepted seats must remain visible through an overflow count rather than disappearing from a fixed 20-seat display.

Use shared text explaining that PostgreSQL protects the final seat with a transaction and lock, while the ClickHouse demo performs an unprotected check followed by an insert. Do not claim that ACID alone prevents races or that every ClickHouse application overbooks.

No live progress dashboard, per-user details, history browsing, or automatic retries. A compact run-duration label may supplement the confirmed per-attempt timings.

## Migrations and isolated fixtures

Create equivalent logical `demo_classes` and `demo_reservations` schemas on both databases. Store class ID, capacity/name and reservation ID, class ID, synthetic attempt ID, and timestamp. Synthetic attempt IDs are request identifiers, not users.

Migrations create the schema and a small deterministic template with capacity 20 and 19 reservations. Every race creates a fresh isolated copy; commit/acknowledge all 19 initial rows before releasing the contenders. Requests from different browsers cannot compete against or reset each other's fixtures.

Use PostgreSQL primary/foreign keys and uniqueness of `(class_id, attempt_id)`; use plain ClickHouse MergeTree preserving raw reservation rows. Add native indexes/sort keys appropriate to each engine and document them. Do not rely on ClickHouse merges to enforce uniqueness or capacity.

Keep existing analytics migrations, seed checkpoints, data, and volumes intact. Migration reruns must be safe. Leave small experiment records stored without a history UI; no automatic destructive cleanup in this scope.

## Database booking strategies

**PostgreSQL:** begin a READ COMMITTED transaction; lock the class row with `SELECT ... FOR UPDATE`; count reservations in a fresh statement after acquiring the lock; reject if full, otherwise insert; return confirmed only after commit. Every demo writer follows that class-lock protocol. Roll back failed operations. Unique attempt IDs protect against duplicate records separately from capacity enforcement.

**ClickHouse:** read capacity and current booking count; reject if already full; otherwise synchronously insert one reservation and return confirmed after acknowledgement. No lock makes the availability check and insert one protected operation. Keep this deliberately unsafe demonstration isolated from production-style booking claims.

No one-sided artificial sleeps, forced errors, process-local mutexes, external coordinators, aggregate tables, or changed resource limits to manufacture results. Use synchronous acknowledged writes and document durability conditions for both engines.

## Shared backend and API

Use domain-oriented storage methods for loading the template, creating a fresh fixture, attempting a reservation, and reading stored results. Handlers and the shared runner contain no SQL or engine branching.

Minimal proposed API:

- `GET /api/classes/demo`: return the template class and 19 initial reservations for the initial page.
- `POST /api/classes/demo/race`: create the fresh fixture, execute 20 attempts, reconcile stored results, and return one completed report. No polling endpoint or persistent run-management subsystem is required.

Use 20 shared Go workers released together by one start gate before database calls. Do not put a barrier after acquiring PostgreSQL locks. Both stacks use the same connection-pool configuration sufficient for 20 concurrent booking operations; document any increase from the current 10-connection limit and its effect on analytics comparisons.

The report contains fixture ID, initial/final booking counts, capacity, elapsed time, ordered attempt outcomes/durations/reasons, and summary totals. Overbooked seats equal `max(final stored bookings - 20, 0)`. Reconcile from the actual database rather than assuming success counts equal committed writes.

Use the same bounded request/run deadlines and JSON error contract on both engines. Return a normal report for a completed race even when some attempts have infrastructure errors; label them separately from class-full rejections. On unrecoverable fixture/reconciliation failure, return a shared error and do not fabricate a completed report. Do not retry ambiguous writes or mutating POST requests automatically. Uncertain writes must be reconciled by reservation ID or marked uncertain.

If the client disconnects or the backend restarts, preserve committed data and do not silently relaunch the experiment. The page may show an error and the user may explicitly start a new isolated experiment. Durable resumption and exactly-once delivery across crashes are outside this minimal scope.

## Verification and delivery

1. Document the scoped parity exception and verify booking behavior against pinned ClickHouse 26.3 and PostgreSQL 18.3 images.
2. Add additive migrations, shared models/storage interfaces, and the two booking adapters.
3. Add the common 20-attempt runner and minimal API, including authoritative result reconciliation.
4. Build the shared page/navigation, result table, loading/errors, and fresh-repeat behavior.
5. Test both real databases: initial 19 bookings, PostgreSQL's one winner, isolated repeat/concurrent runs, exact stored totals, error handling, migration reruns, and preserved analytics parity.
6. Use controlled interleaving in adapter integration tests to prove the unsafe ClickHouse race. Keep scheduling hooks test-only. Natural UI races may occasionally have one winner; never fabricate overbooking or promise a fixed accepted count.
7. Run frontend lint/build, Go vet/tests, Compose validation, and smoke verification against six healthy containers; verify direct `/classes` navigation/reload and identical images/pages/assets.
8. Update README.md with endpoints, behavior, local URLs, durability/concurrency conditions, and measured outcomes. Report the shared-code ratio using AGENTS.md's rules; refactor below 90% without padding.

## References

- PostgreSQL row locks: https://www.postgresql.org/docs/current/explicit-locking.html
- PostgreSQL constraints: https://www.postgresql.org/docs/current/ddl-constraints.html
- ClickHouse insert and experimental transaction guarantees: https://clickhouse.com/docs/concepts/features/operations/insert/transactions

## Grill-me review log

### Initial scope correction — minimal reservation demonstration

- Captured: "use grill me on that plan remember i want minimalistic no user just some reservations showing and 50 tries on it".
- Decision: remove user/account/member features and represent contenders as synthetic attempt IDs. Reopen the plan for simplification before implementation.
- Resolved by Q1 below: exactly one class card with 20 visible seats.
- Next decisions: essential result display and repeat-run behavior; prefer minimal defaults and avoid expanding into a reservation product.

### Q1 — Single class, attempt count, and visible outcomes

- Asked: Should the page show one class or several? Recommended one class with 20 seats, 19 booked, and a race button.
- Captured: "1 class with 20 seats, 1 left"; "button try 20 reservations"; "show witch one of them was accepted and some info".
- Decision: exactly one class, 20 capacity, 19 initial reservations; reduce the race from the earlier 50 attempts to 20. Show an individual result for every attempt.
- Correction: healthy PostgreSQL completion now means one accepted attempt and 19 class-full rejections; all current plan requirements above use 20 attempts.
- Open question: what information beyond accepted/rejected should each attempt display?

### Q2 — Minimal result information

- Asked: Is attempt number, accepted/rejected/error, time taken, and rejection reason enough, with a compact table and accepted/rejected/overbooked totals?
- Captured: "yes".
- Decision: use those per-attempt columns and summary totals; no user details or additional result dashboard.
- Next question: repeat-run behavior.

### Q3 — Repeat the experiment

- Asked: Should each new click start fresh at 19 booked seats and one free, replacing the displayed results without history or a reset button?
- Captured: "yes".
- Decision: every click starts an isolated fresh experiment; replace visible results. No history screen or separate reset control.


## Implementation verification — 2026-10-01

- Shared `/classes` page and navigation preserve the existing analytics map.
- Shared 20-worker runner, fixture isolation, ordered timings, stored-ID
  reconciliation, distinct infrastructure errors and visible overflow implemented.
- PostgreSQL class-lock protocol and intentionally unprotected ClickHouse adapter
  verified on PostgreSQL 18.3 and ClickHouse 26.3; controlled interleaving is test-only.
- Frontend lint/build and eight tests passed; Go vet and race-enabled tests passed.
- Real database tests passed for both engines, including rerunnable migrations,
  repeat/concurrent fixtures, cancellation and preserved analytics semantics.
- Compose validated; all six services healthy; images, HTML/assets and `/classes`
  reload matched. Desktop/mobile browser checks verified the button and results.
- Natural smoke run: PostgreSQL accepted 1 / rejected 19 / final 20; ClickHouse
  accepted 20 / rejected 0 / final 39 / overflow 19; zero infrastructure errors.
- Common pool is 40 open / 40 idle, documented alongside durability conditions.
- ClickHouse adapter explicitly sets async_insert=0 and disables result/condition
  caches; real database contracts verify those settings rather than assuming defaults.
- Final smoke timings: ClickHouse 471.03 ms; PostgreSQL 155.11 ms. Analytics parity
  verification also ran during this observation; timings are not a benchmark.
- Shared-code ratio is audited by `scripts/shared-code.py`; see README for counts.
