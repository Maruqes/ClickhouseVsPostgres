# Project purpose

Demonstrate how the same application behaves when only its database changes:

- `ch-front` (React + shadcn/ui) → `ch-back` (Go) → `ch-db` (ClickHouse).
- `ps-front` (React + shadcn/ui) → `ps-back` (Go) → `ps-db` (PostgreSQL).

The initial scope is infrastructure and a placeholder screen only. Do not add
business features, datasets, benchmarks, or dashboards until requested.

# Invariants — never change without an explicit user request

1. **Identical behavior.** Both stacks expose the same features, screens, workflows,
   routes, schemas, validation, status codes, errors, ordering, pagination, and domain
   semantics for equivalent inputs and data. Performance and resource usage may differ;
   these differences are what this project is designed to demonstrate.
2. **One frontend source.** Both frontend services use `frontend/` and the same image.
   Never fork it, branch UI behavior by database, or create different themes or features.
   The upstream backend address is runtime deployment configuration only.
3. **One backend source.** Both backend services use `backend/`, the same image and
   binary. Routes, domain logic, validation, serialization, middleware, and caching
   policies are shared. Select storage through configuration at the composition boundary.
4. **At least 90% shared application code.** Target 100% shared frontend and all
   non-persistence backend code. At most 10% may be database-specific adapters, SQL,
   migrations, or connection setup. This is a ceiling, not a goal or permission to
   duplicate business logic. Assess first-party frontend/backend source and SQL/migrations;
   exclude generated/vendor code, shadcn components, dependencies, lockfiles, tests, and
   documentation. Count shared lines once and engine-specific lines together. Report the
   ratio when introducing significant persistence code; refactor before shipping if it
   would fall below 90%. Never pad shared files to satisfy the ratio.
5. **Isolate database differences.** All drivers, SQL, connection setup, and engine-specific
   behavior belong under `backend/internal/database/` (future migrations under
   `backend/migrations/`). Handlers and domain services must not import drivers, issue SQL,
   branch on the engine, or expose engine-specific errors. Extend shared domain-oriented
   storage interfaces rather than exposing generic query APIs to business logic.
6. **Exactly six default Docker services.** Preserve two frontends, two backends, and two
   databases. Each stack connects only to its own database and uses its own volume.
   Optional tooling must not add default services without an explicit user request.
7. **Keep the technologies.** React + TypeScript + shadcn/ui, Go, ClickHouse, PostgreSQL,
   and Docker Compose. Do not replace these without an explicit user request.
8. **Fair comparisons.** Future workloads use equivalent logical schemas and identical
   datasets, operations, concurrency, payloads, and application/container resource settings.
   Database-native schemas, indexes, and optimizations are allowed inside storage code
   and must be documented. State cache, durability, consistency, ingestion-completion,
   and tuning conditions. Never silently weaken one stack's guarantees or skip work to
   compensate for an engine limitation.
9. **Ship every feature to both stacks together.** Implement shared behavior once and
   equivalent adapter support with contract coverage for both engines. If equivalent
   semantics cannot be achieved, surface the conflict before implementation rather than
   shipping different behavior.

# Authorized reservation-demo exception (plan_01)

Only the `/api/classes/demo/race` booking outcome may differ: PostgreSQL uses a
READ COMMITTED transaction and class-row lock; ClickHouse deliberately uses an
unprotected availability check followed by an insert, so it may overbook.
Both pages, inputs, fixtures, 20 contenders, deadlines, response shapes and
analytics semantics remain shared. This isolated demonstration is not a claim
that ACID alone prevents races or that all ClickHouse applications overbook.

# Repository map

- `compose.yaml`: six services, shared images, separate networks and volumes.
- `frontend/`: shared Vite/React/Tailwind/shadcn app; Nginx forwards `/api/` to runtime
  `API_UPSTREAM`.
- `backend/cmd/server/`: shared HTTP server and graceful shutdown.
- `backend/internal/database/`: shared storage interface and adapter boundary.
- `scripts/smoke.py`: checks six healthy services, identical images/pages/assets,
  and API contract parity.

# Working rules

- Keep changes limited to the requested work. The base is deliberately minimal.
- Make parity structural through shared source/images, not copies.
- Frontend calls use relative `/api/` URLs. Credentials never reach the browser.
- Database ports stay internal; exposed application ports bind locally.
- Checked-in `app` credentials are local defaults only. Never commit real secrets or
  `.env` files. Do not delete database volumes unless requested.
- Commit dependency lockfiles and `go.sum`. Avoid floating `latest` images.
- Document endpoint and setup changes in `README.md`.
- Run relevant checks: frontend build/lint, Go vet/tests, Compose validation, and
  smoke verification against six healthy containers.
- Future persistence features need shared contract tests against both real databases,
  including ordering, empty results, errors, and data type semantics.
- Identical performance is not an invariant. Report measured differences honestly.
