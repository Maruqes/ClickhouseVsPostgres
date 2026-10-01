# ClickHouse vs PostgreSQL

One application, run twice. Both frontends share the same React + TypeScript +
shadcn/ui source and image. Both Go backends share the same source, image, and binary.
Only database selection and deployment addresses differ.

```text
ch-front (React/shadcn) → ch-back (Go) → ch-db (ClickHouse)
ps-front (React/shadcn) → ps-back (Go) → ps-db (PostgreSQL)
```

Both deployments provide the exercise analytics map at `/` and the minimal
reservation experiment at `/classes`. Analytics remains identical. Only the
reservation race outcome has the explicitly authorized parity exception in
`plan_01`: PostgreSQL protects capacity; ClickHouse deliberately uses an
unprotected check followed by an insert.
Permanent parity rules live in [AGENTS.md](AGENTS.md).

## Run

Requires Docker Engine and Compose v2+; host Node and Go are not required.

```sh
docker compose up --build -d --wait
```

| Stack | Frontend | Backend health |
| --- | --- | --- |
| ClickHouse | http://localhost:3001 | http://localhost:8081/api/health |
| PostgreSQL | http://localhost:3002 | http://localhost:8082/api/health |

Each frontend proxies `/api/` to its own backend. The direct and proxied
`GET /api/health` returns HTTP 200 with `{"status":"ok"}` when its database is
reachable and seed migrations are complete, or HTTP 503 with `{"status":"unavailable"}` otherwise.
`GET /livez` checks backend process liveness without contacting the database.

```sh
docker compose ps
docker compose logs -f
python3 scripts/smoke.py
docker compose down
```

The smoke check requires host Python 3 and verifies service count, health, shared
images, frontend HTML/assets, direct/proxied health, metadata, empty-country analytics, and routing parity.
Stopping preserves data. `docker compose down -v` deletes both database volumes;
use only for an intentional data reset.

Copy `.env.example` to `.env` to customize exposed ports. Database ports are not
published. The `app` username/password are local development defaults. Networks
and database volumes are separate for each stack. PostgreSQL 18 uses the
`/var/lib/postgresql` volume mount for its versioned data directory.

## Development

Changes affect both stacks after `docker compose up --build -d --wait`.
For frontend hot reload, use Node 22.22.2+:

```sh
cd frontend
npm ci
npm run dev
```

Vite proxies `/api/` to `http://localhost:8081` by default. Target PostgreSQL with
`API_UPSTREAM=http://localhost:8082 npm run dev`. Add shadcn components from
`frontend/` using `npx shadcn@latest add <name>`. The `@/` alias, `components.json`,
and Tailwind v4 are configured.

Backend development requires Go 1.26.2+. Configuration: `DATABASE_DRIVER`
(`clickhouse` or `postgres`), `DATABASE_URL`, and optional `HTTP_ADDR` (default
`:8080`). Compose database hostnames resolve only inside Docker; rebuild the
backend containers to work against the private database services.

```sh
cd frontend
npm run lint
npm run build
cd ../backend
go vet ./...
go test ./...
cd ..
docker compose config -q
python3 scripts/smoke.py
```

Dependencies are locked in `frontend/package-lock.json` and `backend/go.sum`.
References: [official shadcn Vite setup](https://ui.shadcn.com/docs/installation/vite),
[ClickHouse Go driver](https://github.com/ClickHouse/clickhouse-go).


## Reservation experiment

Open [ClickHouse reservations](http://localhost:3001/classes) or
[PostgreSQL reservations](http://localhost:3002/classes). Both pages show Spinning
with 20 seats and 19 initial reservations. **Try 20 reservations** starts 20
independent synthetic attempts together. The table shows every attempt in number
order, its outcome, duration, and reason. Each subsequent click creates a new
isolated fixture and replaces the displayed results. Concurrent browsers never
share or reset fixtures. Overflow remains visible above the capacity of 20.

- `GET /api/classes/demo`: deterministic template class, capacity, and 19 reservations.
- `POST /api/classes/demo/race`: no body or query parameters; returns a completed
  report with fixture ID, counts, elapsed milliseconds, ordered attempts, and totals.

Both endpoints return uncached JSON. Invalid input returns 400 `invalid_request`;
startup/fixture/reconciliation failure returns 503 `demo_unavailable`; the shared
30-second deadline returns 504 `demo_timeout`. Fixture creation and booking workers share a 25-second
budget inside that deadline, leaving time for reconciliation. Infrastructure
failures are separate from `class_full` rejections. A write whose acknowledgement
was lost is confirmed if its reservation ID appears in storage; otherwise it
remains an error (`write_uncertain`). `infrastructure_errors` includes errors whose
writes were subsequently confirmed, while `errors` counts unresolved attempt
outcomes. Accepted + rejected + errors always accounts for all 20 attempts in a
completed report. Overbooking is computed from actual stored rows.

There are no automatic mutation retries, users, history UI, or cleanup jobs.
Committed fixtures remain stored on disconnect/restart. A failed response is not
silently resumed; an explicit new click creates another isolated experiment.
Durable run resumption and exactly-once delivery across crashes are outside scope.

### Booking strategies and comparison conditions

PostgreSQL 18.3 uses a READ COMMITTED transaction, `SELECT ... FOR UPDATE` on the
class, a fresh reservation count after that lock, and an insert followed by commit.
All demo writers use this class-lock protocol. A healthy completed run accepts one
attempt and rejects 19. Primary/foreign keys and `(class_id, attempt_id)` uniqueness
protect identity separately from capacity. The unique index also serves class
reservation lookups. See [PostgreSQL row locking](https://www.postgresql.org/docs/18/explicit-locking.html).

ClickHouse 26.3 checks capacity/count then synchronously inserts a raw row in a
plain MergeTree. It has no lock joining those operations. Its sort keys are class
ID for classes and `(class_id, attempt_id, id)` for reservations. Merges preserve
raw rows and enforce neither uniqueness nor capacity. Natural overbooking is
possible, not guaranteed. This is a deliberately unsafe demo strategy, not a
claim that every ClickHouse application overbooks or that ACID alone prevents
races. See [ClickHouse insert guarantees](https://clickhouse.com/docs/concepts/features/operations/insert/transactions).

Both engines acknowledge all 19 initial rows before releasing the 20 workers.
No scheduling delays or barriers after availability reads exist in the app.
PostgreSQL keeps default WAL/fsync/synchronous_commit durability; ClickHouse keeps
synchronous inserts (`async_insert=0` explicitly configured in the adapter) and MergeTree
`fsync_after_insert=1`, `fsync_part_directory=1`. Neither stack uses replicas or
claims replication-level durability. Results are read after all workers finish;
uncertain writes are never retried. The common pool increased from 10 open / 5 idle
to 40 open / 40 idle connections to allow 20 contenders and concurrent experiments.
This also affects analytics concurrency comparisons; compare both stacks with the
same pool, workload, warmed/cold connection state, and cache conditions. No app
result cache is used; ClickHouse result and condition caches are explicitly disabled. Container CPU/memory settings remain equal across stacks.

Additive, rerunnable migrations retain analytics tables, checkpoints, and volumes.
The deterministic template uses negative synthetic attempt IDs; contenders use
1–20. Partial template seeding resumes by reservation ID without deleting data.

### Verification

```sh
cd frontend && npm test
cd ../backend && go test -race ./...
cd .. && bash scripts/test-databases.sh
python3 scripts/smoke.py
```

Database contracts use disposable test databases on the pinned running engines;
they preserve application volumes and verify template data, migration reruns,
stored totals, concurrent/repeat isolation, PostgreSQL's single winner, cancellation,
and unchanged analytics. A test-only insert gate proves ClickHouse's unsafe
interleaving; the production app contains no gate after availability reads.


Measured local smoke run on 2026-10-01 (10,000,000 analytics rows present, existing
Compose CPU/memory limits, explicitly synchronous durable writes, naturally
scheduled race; analytics contract checks also ran during this verification):

| Engine | Accepted | Rejected | Infrastructure errors | Final bookings | Overbooked | Whole run |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ClickHouse | 20 | 0 | 0 | 39 | 19 | 471.03 ms |
| PostgreSQL | 1 | 19 | 0 | 20 | 0 | 155.11 ms |

These are observations from one run, including fixture setup and reconciliation;
ClickHouse's winner count and all timings vary. This is not a throughput benchmark.

The first-party source audit (`python3 scripts/shared-code.py`) currently reports
91.72% shared code (1,241 shared / 112 engine-specific nonblank source lines), above
the 90% floor. Shared SQL and fixture lifecycle count once; native migrations,
dialect/connection setup and the protected/unprotected strategy count as specific.
Generated map data, shadcn components/theme, dependencies, lockfiles, tests and
documentation are excluded. This is a line-count convention, not code padding.

## Exercise analytics (plan_00)

Open either frontend and hover over a country for training volume, exact distinct
athletes, completed sets and repetitions. Click for the same totals, volume by
seven muscle areas and a daily volume chart. Country attribution is nationality.
The map uses bundled Natural Earth geometry; search provides access to small
countries and a keyboard-friendly country list.

The two-handle slider uses UTC calendar days, initially 2023-01-01 through
2025-12-31. Both dates are inclusive. Dragging changes the displayed dates;
release commits them and clears previous statistics without querying. The next
country entry/click calculates fresh results. Page load fetches configuration
metadata only. Hover has no debounce; leaving, changing countries/ranges or
closing details cancels superseded work. Keyboard focus provides hover summaries;
Enter/Space opens details. Touch users tap countries or use search.

| Endpoint | Behavior |
| --- | --- |
| `GET /api/dataset` | Seed version, row count, configured date bounds, category order; no exercise aggregation |
| `GET /api/country/summary?country=US&from=2023-01-01&to=2025-12-31` | Country/date totals and `query_ms` |
| `GET /api/country/detail?country=US&from=2023-01-01&to=2025-12-31` | Summary plus ordered `muscles` and ascending UTC `days` |

Country codes must be two uppercase letters. Syntactically valid countries with
no generated sets return zeros. Dates must be ordered, valid `YYYY-MM-DD` values
inside configured bounds; duplicate/unknown query parameters return 400
`invalid_request`. Before seeding completes, endpoints return 503
`dataset_unavailable`. Query failures return 503 `query_unavailable`; the common
30-second query deadline returns 504 `query_timeout`. Errors do not expose drivers.
Every analytics response includes `Cache-Control: no-store`.

`kg` is total external load per repetition, including a bar or both dumbbells.
Volume is computed from raw rows as `SUM(reps × kg)` with exact centikilogram
arithmetic, returned as a decimal string such as `"480.25"` rather than a binary
float. Athletes are counted exactly within the filter. Empty days and all seven
categories are returned with zero volume. Categories follow back, shoulder, legs,
chest, tricep, bicep, forearm. The chart's plotted coordinates use numbers;
readouts and its accessible daily-value table preserve exact decimal strings.

### Seed migrations and recovery

On startup the backend applies embedded SQL under `backend/migrations/`, then
seeds raw sets in 250,000-row batches using shared deterministic SQL formulas.
`SEED_ROWS` defaults to **10,000,000** on both stacks and accepts 1–100,000,000.
The `exercise-v2` generator defines 100,000 stable athletes, 21 exercise
identities, seven primary muscle areas, 5–20 repetitions and loads in 0.25 kg
increments. The seed now covers **50 countries**; nationality remains fixed for
each athlete. Relative weights for the original countries are US 30, BR 15,
DE 12, IN 10, GB 8, FR 7, JP 6, AU 5, CA 4, PT 2 and ZA 1. Each of the 39
additions has weight 1 (total weight 139):

ES, IT, NL, BE, CH, AT, SE, NO, DK, FI, PL, CZ, RO, GR, TR, UA, RU, CN, KR,
ID, TH, VN, MY, PH, PK, BD, SA, AE, IL, EG, MA, NG, KE, MX, AR, CL, CO, PE, NZ.

An athlete's bucket is `((athlete_id - 1) * 37) % 139`, matched against cumulative
weights in that order. All countries have records at the default row count;
small custom seeds may omit countries. Other countries provide empty cases.
Timestamps cycle deterministically through the same three-year interval. This
synthetic distribution is an illustration, not a representative training dataset.

Readiness stays unavailable until seeding and native index setup finish. Initial
startup or an upgrade may take several minutes; use
`docker compose up --build -d --wait --wait-timeout 900` and follow backend logs.
Completed batches have durable, version-specific checkpoints. Restart removes
only the uncheckpointed exercise tail and replays it, so uncertain inserts do not
duplicate sets. This assumes one seeding backend per database, as in the
six-service topology.

Existing `exercise-v1` volumes automatically upgrade to `exercise-v2` at the same
configured row count. A durable v2 zero checkpoint precedes regeneration of the
exercise seed, because country is part of ClickHouse's immutable sort key.
Exercise IDs, athletes, exercise identities, muscles, repetitions, loads and
creation times retain their previous formulas; only nationality changes.
A zero checkpoint truncates only `exercises`, avoiding deleted-row bloat. The
PostgreSQL country/date index is removed during incomplete seeding and rebuilt
before readiness; its primary key remains. Serving-time indexes are unchanged.
Volumes, classes and reservations are retained. An interrupted upgrade resumes
using v2 checkpoints, even when v1 has a completed checkpoint. Other version
changes or a changed row count fail explicitly; use another Compose project and
unused ports for another dataset size.

### Comparison conditions

Both databases have 2 CPUs and 2 GiB memory; matching backend and frontend
containers each have 1 CPU and 512 MiB. The current shared pool has 40 open/idle
connections. Compare the same country, range, endpoint and concurrency.
ClickHouse uses plain MergeTree ordered by `(country_code, created_at, id)` and
low-cardinality country/category columns. PostgreSQL uses a primary key on `id`,
a B-tree on `(country_code, created_at)`, ANALYZE after seed completion, and at
most one parallel worker plus its leader. ClickHouse analytics uses at most two
query threads. Native default compression/storage settings otherwise remain.

There are no precomputed volumes, aggregate tables, projections, materialized
views or application/browser/API result caches. ClickHouse query-result and
query-condition caches are explicitly disabled on analytics queries; exact
COUNT DISTINCT is forced. Ordinary database buffers, decompressed-block caches
and OS page caching remain enabled. Repeated requests still execute SQL. The
label measures the elapsed database-work interval, including connection-pool
wait and result reads; details execute three queries sequentially. JSON shaping,
zero filling, network and rendering are outside that timer.

All seed inserts complete before exploration; the analytics dataset is immutable
while serving. Detail queries therefore read the same logical dataset even though
they are separate statements. Background merges preserve raw-set semantics.
PostgreSQL retains WAL/fsync/synchronous_commit; ClickHouse uses synchronous
inserts and `fsync_after_insert=1`, `fsync_part_directory=1` for data and checkpoints.
Both run locally without replicas. Query cancellation reaches each real server;
PostgreSQL uses pgx's server CancelRequest handler with a one-second socket-deadline
fallback ([driver documentation](https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#CancelRequestContextWatcherHandler)).

Historical `exercise-v1` local contract runs showed full-range US summaries
around 0.08–0.10 seconds on ClickHouse and 3.5–4.0 seconds on PostgreSQL, with
identical results. Narrow
ranges/empty countries can favor PostgreSQL. These are single-user local samples
with the previous 11-country distribution, ingestion finished and ordinary caches
populated, not cold-cache benchmarks or guaranteed performance. No caches or volumes were cleared for measurement.

### Analytics verification and map provenance

```sh
cd frontend && npm test && npm run build && npm run lint
cd ../backend && go test -race ./... && go vet ./...
cd ..
docker compose config -q
bash scripts/test-databases.sh
python3 scripts/smoke.py
python3 scripts/analytics-contract.py
python3 scripts/shared-code.py
```

Frontend tests cover lazy requests, slider commits, fresh hover requests,
cancellation, stale responses, detail UI and failures. The shared real-database
suite uses disposable databases to test recovery, reruns, independent reference
results, decimal loads, midnight/leap days, ordering, exact athlete counts, empty
results, and cancellation observed on the server. The running-stack contract
checks full-dataset checksums, raw samples and direct/proxied API parity. The
source-count script excludes generated geometry, shadcn scaffolding/components,
dependencies, tests and documentation and gates the 90% shared-code invariant.

Map geometry is derived from public-domain
[Natural Earth 5.1.2](https://www.naturalearthdata.com/about/), pinned to its
[versioned source](https://github.com/nvkelso/natural-earth-vector/blob/v5.1.2/geojson/ne_110m_admin_0_countries.geojson).
Run `cd frontend && npm run generate-map` to regenerate. Geometry with a common
country code is combined into one path; Somaliland is grouped under SO, northern
Cyprus under CY, and Kosovo uses XK. The simplified 110m asset omits tiny countries
and Antarctica. Geometry is visualization data, not an assertion about borders.
