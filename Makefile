.DEFAULT_GOAL := help

COMPOSE ?= docker compose
WAIT_TIMEOUT ?= 900

.PHONY: help up down migrate reset-db reset

help:
	@echo "make up        Build and start all six services; apply migrations and seed"
	@echo "make down      Stop all services, preserving database volumes"
	@echo "make migrate   Apply Goose migrations to both databases and restart the app"
	@echo "make reset-db  Delete BOTH database volumes, then rebuild and seed the app"
	@echo "make reset     Delete all project volumes, rebuild without cache and recreate everything"

up:
	$(COMPOSE) up --build -d --wait --wait-timeout $(WAIT_TIMEOUT)

down:
	$(COMPOSE) down

# Stop application writers before running the same binary as a one-off migrator.
# If a migration fails, keep the app stopped so it cannot use a partial schema.
migrate:
	$(COMPOSE) build ch-back ps-back
	$(COMPOSE) stop ch-front ps-front ch-back ps-back
	$(COMPOSE) up -d --wait --wait-timeout $(WAIT_TIMEOUT) ch-db ps-db
	$(COMPOSE) run --rm --no-deps ps-back server migrate
	$(COMPOSE) run --rm --no-deps ch-back server migrate
	$(MAKE) up

reset-db:
	$(COMPOSE) down --volumes
	$(MAKE) up

reset:
	$(COMPOSE) down --volumes --remove-orphans
	$(COMPOSE) build --no-cache
	$(COMPOSE) up -d --force-recreate --wait --wait-timeout $(WAIT_TIMEOUT)
