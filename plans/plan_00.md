# Project plan: Discovery notes

Date: 2026-10-01
Goal: Pressure-test the exercise analytics map before writing an implementation plan.

## Summary / key decisions

- Store plans in `plans/`, numbered `plan_00.md`, `plan_01.md`, and so on.
- Use the installed grill-me skill to capture each answer before the next question.
- Follow the repository's `AGENTS.md` invariants throughout planning.
- The user envisions two main application components: one workload intended to favor ClickHouse, another intended to favor PostgreSQL. Discovery starts with the first.
- Proposed analytics source: an `Exercises` table with `id`, `muscle_area`, `reps`, `kg`, `country_code`, and `created_at`; additional fields remain open.
- Proposed interaction: hovering over or clicking countries on a frontend map shows exercise metrics, including volume.
- User requirement: calculations happen "real time without cache" so the database performs the aggregation work. Exact freshness and cache conditions remain to be defined.
- This file began as discovery notes. The user requested implementation on 2026-10-01; the proposed defaults below were adopted for that work. See the implementation record at the end.
- One `Exercises` row represents one completed exercise set, such as chest incline bench press at 60 kg for 8 reps.
- Exercise identity must be distinguishable from muscle area; the exact field/catalog design remains open.
- `kg` means total external load moved per rep, including the bar for barbells and both dumbbells for paired exercises.
- Training volume is `reps × kg` per set, summed across matching sets. The 60 kg × 8 reps example contributes 480 kg of training volume.
- `country_code` represents the athlete's nationality, not the location where the set was performed. The user considers this distinction low priority.
- Use generated user exercise data. This is an example app to demonstrate ClickHouse, with a frontend for viewing data, a simple backend, and aggregation work primarily expressed in database SQL.
- Keep discovery focused on the analytical demonstration; no user-submission workflow is currently requested. Repository parity still requires equivalent PostgreSQL support.
- Hovering over a country shows basic statistics: total training volume and number of athletes. Additional summary metrics are undecided.
- Clicking a country opens a modal with more detailed statistics, including volume by muscle area: back, shoulder, legs, chest, tricep, bicep, forearm.
- Counting athletes requires stable `athlete_id` across generated sets. Count distinct athletes exactly within the selected country and date range; multiple matching sets from one athlete count once.
- A two-handle date-range slider in the top-right selects the time frame. Its default spans the full available dataset ("max"). The selected range applies to hover and modal statistics.
- Commit time-range changes when a slider handle is released, not during dragging. Analytics execute only on country hover/click; changing the range does not automatically query.
- The map shows statistics only on country hover or click. No volume-based country coloring or map-wide aggregate query is requested.
- Releasing the slider commits the date range for the next country interaction. Previously displayed statistics must not be presented as belonging to the new range; clear or mark them stale without querying.
- Desired dataset scale is millions of exercise rows, with the intent to expose a useful performance difference. Exact initial count remains undecided.
- Desired demonstration outcome: both engines return the same results, with PostgreSQL taking seconds longer and ClickHouse feeling almost instant. This is a target to validate through measurements, not an established result.
- No precomputed volume or aggregates. Aggregate calculations must execute at request time over raw exercise sets on both databases.
- ClickHouse engine requirement: background merges must preserve raw-set semantics rather than sum or aggregate rows. Plain `MergeTree` is the proposed engine; exclude pre-aggregating engines and aggregate materialized views/projections.
- Create the generated dataset through migrations before exploration. No continuous ingestion scenario is requested.
- Opening the page must not trigger exercise analytics. Calculations start only on a country hover or click.
- The country modal includes a daily training-volume chart over the selected date range, calculated from raw sets on click, alongside volume by muscle area.
- Hover summary and country modal show a small "Calculated in … ms" label based on backend-measured database query duration. This is distinct from browser/network/render latency.
- Entering a country with the pointer triggers a fresh summary request immediately, with no hover delay/debounce. Clicking triggers fresh detail calculations immediately.
- Generated data covers three years. The user considers the exact span low priority and accepts this default.

## Q&A log

### Initial explanation — exercise analytics map

- Captured: "the app should have 2 main componets, one thatf avores clickhouse, other tnhat favors postgres"; begin by pressure-testing the component intended to favor ClickHouse.
- Captured: `Exercises` fields proposed above; country hover/click displays "volume etc etc" with real-time calculations and no cache.
- Repository constraint: both workloads must run with equivalent behavior and data on both stacks. Intended performance advantages are hypotheses to measure.
- Flags: define row meaning, metrics, map behavior, dataset, ingestion, query scope, cache policy, and comparison conditions.

### Q1 — Meaning of one row

- Asked: Does one row represent a completed set, an entire workout, or something else?
- Captured: "a set for exmaple i did chest inclined bench press with 60kg for 8reps". One row is one completed set; the example includes both a muscle area (chest) and a specific exercise (incline bench press).
- Flags: weight convention and exercise identity field remain unresolved. The proposed volume formula has not yet been explicitly confirmed.

### Q2 — Weight convention and training volume

- Asked: Should `kg` mean total weight moved per rep, including the bar or both dumbbells, with volume calculated as `reps × kg`?
- Captured: User answered "yes". This confirms the total external load convention and the proposed volume calculation.
- Flags: handling bodyweight and assisted exercises remains open.

### Q3 — Country attribution

- Asked: Does `country_code` identify where the set was performed or the athlete's nationality?
- Captured: "it does not matter that much, but is athelets nationalaty". Attribute each set to athlete nationality.
- Flags: nationality changes and multiple nationalities remain unspecified; revisit only if relevant to dataset scope.

### Q4 — Data source and demonstration scope

- Asked: Will the component use generated exercise data, real user submissions, or both?
- Captured: "generated user data"; "a frontend to see the data a simple backend and mainly clickhouse sql"; "this is an exmeple app to show clickhouse".
- Decision: focus on generated records and a read-oriented analytics demonstration. SQL aggregation belongs inside the database adapters, with shared API behavior on both stacks.
- Flags: generated dataset size/distribution and analytics interactions remain open.

### Q5 — Hover summary and country detail

- Asked: What should hovering over a country show? Suggested volume, sets, reps, and muscle-area breakdown.
- Captured: User is unsure of all metrics; hover should show "total volume number of athletes and some more random info". Clicking opens a modal with "more way delaied info", including volume per category: "back shoulder legs chest tricep bicep forearm".
- Decision: separate compact hover summary from detailed click modal; muscle-area breakdown belongs in the modal.
- Proposal: use total sets and total reps as the remaining hover statistics; add stable `athlete_id` so athlete counts have a defined basis. These details are not yet user-confirmed.
- Flags: time scope, exact athlete count semantics, further modal metrics, and final hover metrics remain open.

### Q6 — Time-range selection

- Asked: Should statistics cover all data or a selected date range? Recommended all-time default with a shared range filter.
- Captured: "a bar on the top righr that has 2 points i can scroll and thats our time frame data range (default is max)".
- Decision: use a two-handle date-range slider at the top-right, defaulting to the full dataset range, to control the analytics time frame.
- Flags: slider granularity, time-zone/boundary semantics, and when changes trigger database queries remain open.

### Q7 — Slider query trigger

- Asked: Should queries run while dragging or on handle release?
- Captured: "when we release".
- Decision: dates can update visually during dragging; commit the range on release. Q12 later clarifies that release alone must not execute analytics.
- Flags: hover/click request lifecycle and map-wide aggregation behavior remain open; this answer only settles slider query timing.

### Q8 — Map-wide statistics

- Asked: Should the map color countries by volume for the selected range, or only show statistics on hover/click?
- Captured: "only shows statistics when ove ror clicked".
- Decision: show country statistics on hover/click only; omit data-driven country coloring and global aggregation requests.
- Consequence: no global aggregation is needed. Q12 later clarifies that slider release only changes the selected range, without automatically recalculating visible statistics.
- Flags: hover request timing and detailed metrics remain open.

### Q9 — Dataset scale and intended contrast

- Asked: How large should the generated dataset be? Suggested configurable size, initially 10 million sets, followed by measurements.
- Captured: "large enough for clickhouse to support but clickhouse not. so maybe some milions of rows".
- Interpretation pending confirmation: the second "clickhouse" may mean PostgreSQL; clarify whether the desired contrast is responsiveness or inability to complete.
- Decision: millions of generated rows are desired; neither a precise count nor a guaranteed performance outcome is established.
- Flags: clarify intended comparison, then define dataset scale and response-time targets under equal resources.

### Q10 — Intended performance contrast

- Asked: Should PostgreSQL return the same results noticeably slower, or should requests time out?
- Captured: "same results but seconds difference while clickhoue almost instant".
- Decision: prioritize successful equivalent results and a visible latency difference, not intentional timeouts. Validate the desired difference under fair comparison conditions.
- Flags: numerical latency targets, dataset scale/distribution, country selectivity, and resource settings remain unresolved.

### Q11 — No precomputation and merge semantics

- Asked: Does "no cache" also exclude precomputed totals and aggregate tables? Recommended request-time raw-row aggregation, no result caches, with ordinary memory/filesystem caches documented.
- Captured: "no precomputed"; "use an engine that the result of the merge is simple as postgres"; "i dont want to \"cheat\" and pre compule volume ... it should do that realtime".
- Decision: retain raw exercise sets and compute volume and other aggregates during each request. No merge-time aggregation or persisted aggregate results.
- Technical proposal: plain `MergeTree`, whose ordinary part merges do not perform aggregate transformations. References: https://clickhouse.com/docs/engines/table-engines/mergetree-family/mergetree and https://clickhouse.com/docs/managing-data/core-concepts/merges.
- Flags: ordinary memory/filesystem cache measurement conditions still need explicit definition; no precomputation does not imply a cold disk read on every request.

### Q12 — Dataset creation and lazy calculations

- Asked: Should data be loaded once before exploration or continuously ingested?
- Captured: "the dataset should be created with migrations"; "when i open the page the calculation only start when i hover or click on some counrty".
- Decision: migrations create/seed the dataset before exploration; no analytics on page load. Execute analytics only for country hover/click.
- Correction to Q7/Q8: slider release commits a range but does not itself query. Clear or mark old statistics stale when the range changes; exact presentation remains open.
- Flags: reproducible migration seeding, completion/retry behavior, configured dataset time bounds without page-load aggregation, and hover triggering remain to be designed.

### Q13 — Country volume over time

- Asked: Should the modal show a daily volume chart within the selected range alongside muscle-category volume?
- Captured: "yes".
- Decision: include daily training volume for the selected country and time frame; calculate chart points from raw exercise sets when the country is clicked.
- Flags: missing-day representation and UTC day boundaries remain to be specified.

### Q14 — Visible calculation timing

- Asked: Should both the hover summary and modal show a calculation-time label measuring backend database-query duration?
- Captured: "yes".
- Decision: display "Calculated in … ms" for each country analytics response on both stacks.
- Flags: if a response executes multiple queries, define the label as the elapsed database-work interval; precise timing instrumentation remains an implementation detail.

### Q15 — Immediate hover triggering

- Asked: Trigger on passing across a country immediately, or require a brief pause? Recommended a 200 ms pause.
- Captured: "trigegr immidialty".
- Decision: immediate query on country pointer entry; no dwell delay. Pointer movement within the same country does not constitute a new entry.
- Implementation requirement: prevent outdated country/range responses from appearing under the current selection; cancel superseded requests and propagate cancellation to database work where supported.
- Flags: verify cancellation behavior on both adapters; final hover metrics and demo operating conditions remain open.

### Q16 — Generated time span

- Asked: What time span should generated data cover? Recommended three years with fixed dates and reproducible generation.
- Captured: Initially "not important..", then "three years for the generator is good enought".
- Decision: use three years; exact dates can be chosen as a documented implementation default.
- Flags: initial row count, country distribution, and stable athlete identity remain to be finalized.

### Q17 — Exact athlete counts and faster discovery

- Asked: Should athlete counts be exact distinct counts within the selected country/date range?
- Captured: "yes. lets speed up this".
- Decision: exact athlete counts on both engines. Accelerate discovery by proposing routine defaults together instead of asking separate questions for every implementation detail.

## Proposed defaults for closing discovery

These are recommendations, not additional user-confirmed requirements.

- Start with 10 million sets; make dataset size configurable. Use a fixed seed and identical deterministic records for both engines, generated/loaded in batches through migrations with resumable progress and readiness withheld until complete. A restart must not duplicate completed seed data.
- Use stable athletes with one nationality each, a documented uneven country distribution, and repeat sets across the seven muscle categories. Include sparse/no-data countries as contract cases. Use weighted exercises only initially, without bodyweight/assistance semantics.
- Add `athlete_id` and `exercise_id` to the proposed fields. Use exact decimal load/volume semantics and identical validation; one set has one primary muscle category to prevent category-volume double counting.
- Hover metrics: volume, exact athlete count, set count, rep count. Modal: the same totals, seven-category breakdown, daily-volume chart. Return zero totals/empty-day values consistently for no-data periods.
- Use UTC calendar days and a daily-step slider; include both selected dates via an exclusive next-day end boundary. Publish configured seed time bounds without scanning exercise rows on page load.
- Clear old statistics when the range changes, awaiting the next hover/click. Show loading/errors explicitly, discard stale responses, and cancel superseded requests on both stacks.
- Disable browser/API/database result caching; no precomputed aggregates. Document ordinary database/OS memory caching, warm/cold runs, ingestion completion, native indexes/sort keys, and equal resource settings. Tune both engines reasonably and report results without guaranteeing a latency gap.
- Validate schema/seed parity and exact query results against both real databases, including empty ranges, boundaries, distinct counts, decimals, category ordering, cancellation, and errors. Preserve the six-service topology and report the shared-code ratio when adding persistence.

## Open flags (pending input)

- Remaining defaults above → user corrections or acceptance during plan handoff.
- Measured latency gap and required dataset scale → implementation measurements; do not claim a guaranteed outcome.
- Database cancellation behavior and migration runtime/recovery → implementation verification.
- PostgreSQL-oriented component → later discovery.

## Implementation record — 2026-10-01

The user requested: “start developing plan_00.md”. Implemented the analytics
component using the proposed defaults, with both engines supported together.

- Shared country map, search, immediate hover summaries, click details, daily
  chart, exact decimal readouts, and a two-handle UTC date slider. No analytics on
  page load or slider release. Superseded requests are canceled and late responses
  discarded; loading, failure and empty states are explicit.
- Shared `/api/dataset`, `/api/country/summary` and `/api/country/detail` contracts.
  Exact athlete counts and raw-set volume aggregation; all seven categories and
  every UTC day, including leap days and zeros. Database-work timing is returned
  separately from network/rendering latency.
- Default 10 million deterministic raw sets, 100,000 stable athletes and 21
  exercise identities covering 2023–2025. Configurable row count, batch
  checkpoints, interrupted-tail replay and readiness withheld until completion.
  Existing volumes are preserved; configuration changes on seeded volumes fail.
- Plain ClickHouse MergeTree and PostgreSQL native indexes; no precomputed
  aggregates, query-result/condition caches or app/API/browser result caches.
  Ordinary database/OS caching remains enabled. Equal container limits, native
  sort/index choices, durability and consistency conditions are documented in
  README.md.
- Real-database contracts verify independent generated results, decimal and
  midnight boundaries, exact distinct counts, empty data, stable ordering,
  rerunnable/recoverable seeding and canceled queries stopping on both servers.
  PostgreSQL required an explicit pgx server cancellation handler; socket closure
  alone did not stop a running query promptly.
- Frontend tests cover lazy query triggers, slider commits, fresh hover requests,
  superseded requests and stale responses, detail rendering and errors.
- Local full-range US summary samples: ClickHouse 79–98 ms; PostgreSQL
  3,536–4,039 ms, with identical results. Full-range US details in one contract run:
  ClickHouse 228 ms; PostgreSQL 9,719 ms. Narrow ranges and empty countries
  sometimes favor PostgreSQL. These are local samples after ingestion with
  ordinary caches populated, not guaranteed gaps or cold-cache measurements.

Implementation and validation commands are documented in README.md. Source
sharing is checked by scripts/shared-code.py; the checkout remains above 90%.
The separately authorized plan_01 reservation work appeared concurrently in the
same checkout and has been preserved. Its different race outcomes are outside
plan_00; analytics parity has no exception.

Final validation for this implementation: frontend tests/build/lint, Go vet and
race tests, the shared contracts against both real databases, Compose validation,
six-service smoke checks, raw seed/API parity checks and diff whitespace checks
passed. Current first-party source count: 1,227 shared lines and 112
engine-specific lines (91.64% shared), excluding generated/shadcn scaffolding,
tests and documentation; this includes the concurrent plan_01 additions.
Both local frontends remain available at ports 3001 and 3002.


## Seed expansion — 2026-10-01

The user requested more countries in the seed itself. `exercise-v2` expands the
weighted nationality distribution from 11 to 50 countries. The original eleven
keep their relative weights; 39 countries from Europe, Asia, Africa, the Americas
and Oceania have weight 1 each, giving a total weight of 139. Default row count,
athlete identities and all exercise fields except nationality remain unchanged.

Existing v1 datasets automatically regenerate only the exercise seed at the same
configured row count. Durable version-specific checkpoints allow recovery from
interruption before or after an upgrade batch, without old completed v1
checkpoints masking v2 progress. Database volumes and reservation fixtures remain
in place. Unknown versions and changed row counts still fail explicitly.

The shared real-database contracts cover initial seeding, v1 upgrades, interrupted
upgrades at zero and nonzero checkpoints, repeat migration, exact totals for all
50 countries, unknown-version rejection and preserved reservations. Empty-result
checks now use the unseeded `ZZ` sentinel because New Zealand has generated data.
At checkpoint zero, only the exercise table is truncated to avoid deleted-row
bloat. PostgreSQL rebuilds its country/date index after incomplete seeding,
before readiness, retaining the same serving-time index and primary key.


Validation passed: frontend tests/build/lint, Go race tests/vet, shared contracts
against both real databases, Compose validation, six-service smoke and full raw
seed/API parity. Both local databases now contain 10,000,000 exercises across
50 countries. Before/after checks confirmed unchanged non-nationality exercise
checksums and existing class/reservation counts, before smoke created its usual
additional race fixtures. Source sharing: 1,313 shared lines and 113
engine-specific lines, or 92.08%.
