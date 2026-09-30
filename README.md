# ClickHouse vs PostgreSQL

One application, run twice. Both frontends share the same React + TypeScript +
shadcn/ui source and image. Both Go backends share the same source, image, and binary.
Only database selection and deployment addresses differ.

```text
ch-front (React/shadcn) → ch-back (Go) → ch-db (ClickHouse)
ps-front (React/shadcn) → ps-back (Go) → ps-db (PostgreSQL)
```

Foundation only: identical placeholder screens, health checks, database connections,
and persistent volumes. No business features or tables yet.
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
reachable, or HTTP 503 with `{"status":"unavailable"}` otherwise.
`GET /livez` checks backend process liveness without contacting the database.

```sh
docker compose ps
docker compose logs -f
python3 scripts/smoke.py
docker compose down
```

The smoke check requires host Python 3 and verifies service count, health, shared
images, frontend HTML/assets, direct/proxied health, and routing parity.
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
