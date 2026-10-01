#!/usr/bin/env bash
# Optional, ephemeral test runners. The six-service Compose topology is unchanged.
set -euo pipefail
cd "$(dirname "$0")/.."
for driver in clickhouse postgres; do
  if [ "$driver" = clickhouse ]; then
    service=ch-db
    contract_url='clickhouse://app:app@ch-db:9000/app'
  else
    service=ps-db
    contract_url='postgres://app:app@ps-db:5432/app?sslmode=disable'
  fi
  container=$(docker compose ps -q "$service")
  network=$(docker inspect --format '{{range $name, $value := .NetworkSettings.Networks}}{{$name}}{{end}}' "$container")
  docker run --rm --network "$network" -v "$PWD/backend:/app" -w /app \
    -v exercise-contract-go-mod:/go/pkg/mod -v exercise-contract-go-build:/root/.cache/go-build \
    -e CONTRACT_DRIVER="$driver" -e CONTRACT_URL="$contract_url" \
    golang:1.26.2-alpine go test -count=1 -timeout=2m -v ./internal/database
 done
