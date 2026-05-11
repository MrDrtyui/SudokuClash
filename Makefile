SHELL := /bin/sh

ROOT_DIR := $(abspath .)
DOCKER_DIR := $(ROOT_DIR)/docker
ENV_FILE := $(DOCKER_DIR)/.env
ENV_EXAMPLE := $(DOCKER_DIR)/.env.example
COMPOSE_FILE := $(DOCKER_DIR)/docker-compose.yml

ifeq ($(shell docker compose version >/dev/null 2>&1; echo $$?),0)
COMPOSE := docker compose
else
COMPOSE := docker-compose
endif

.PHONY: help env check run build up down restart logs ps tunnel stop clean migrate-db export-db import-db validate ensure-dump

help:
	@echo "Sudoku stack commands"
	@echo ""
	@echo "  make env          - create and auto-fill docker/.env"
	@echo "                     also auto-load local docker/.secrets.env if present"
	@echo "  make validate     - validate compose config"
	@echo "  make build        - build the Docker images"
	@echo "  make run          - one-command local start for the full stack"
	@echo "  make tunnel       - start Cloudflare tunnel profile too"
	@echo "  make stop         - stop all services"
	@echo "  make down         - stop and remove services"
	@echo "  make restart      - restart the stack"
	@echo "  make logs         - tail compose logs"
	@echo "  make ps           - show running services"
	@echo "  make migrate-db   - import the committed/latest dump into compose postgres"
	@echo "  make export-db    - manually create a fresh dump from the current source DB"
	@echo "  make import-db DUMP=/abs/path/file.dump - import a specific dump into compose postgres"
	@echo "  make clean        - remove compose stack and named volumes"

env:
	@chmod +x "$(DOCKER_DIR)/scripts/bootstrap-env.sh"
	@ENV_FILE='$(ENV_FILE)' ENV_EXAMPLE='$(ENV_EXAMPLE)' "$(DOCKER_DIR)/scripts/bootstrap-env.sh"

check: env
	@command -v docker >/dev/null 2>&1 || { echo "docker is required"; exit 1; }
	@$(COMPOSE) version >/dev/null 2>&1 || { echo "docker compose or docker-compose is required"; exit 1; }

validate: check
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" config >/dev/null
	@echo "Compose config is valid."

build: validate
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" build

ensure-dump:
	@if [ ! -f "$(DOCKER_DIR)/backups/appdb_latest.dump" ]; then \
		echo "Missing committed dump: $(DOCKER_DIR)/backups/appdb_latest.dump"; \
		echo "Run 'make export-db' once to generate it, then commit that file."; \
		exit 1; \
	fi

up: validate
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" up -d
	@echo "Stack is up. Open http://localhost:$$(grep '^NGINX_HTTP_PORT=' "$(ENV_FILE)" | cut -d= -f2 || echo 8088)"

run: ensure-dump migrate-db up

tunnel: validate
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" --profile tunnel up -d
	@echo "Stack and tunnel profile are up."

stop:
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" stop

down:
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" down

restart: down up

logs:
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" logs -f --tail=150

ps:
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" ps

export-db: env
	@chmod +x "$(DOCKER_DIR)/scripts/export-current-db.sh"
	@set -a; . "$(ENV_FILE)"; set +a; COMPOSE_BIN='$(COMPOSE)' ENV_FILE='$(ENV_FILE)' "$(DOCKER_DIR)/scripts/export-current-db.sh"

migrate-db: env ensure-dump
	@chmod +x "$(DOCKER_DIR)/scripts/import-dump.sh"
	@set -a; . "$(ENV_FILE)"; set +a; COMPOSE_BIN='$(COMPOSE)' ENV_FILE='$(ENV_FILE)' "$(DOCKER_DIR)/scripts/import-dump.sh" "$(DOCKER_DIR)/backups/appdb_latest.dump"

import-db: env
	@if [ -z "$(DUMP)" ]; then echo "Usage: make import-db DUMP=/absolute/path/to/dump.dump"; exit 1; fi
	@chmod +x "$(DOCKER_DIR)/scripts/import-dump.sh"
	@set -a; . "$(ENV_FILE)"; set +a; COMPOSE_BIN='$(COMPOSE)' ENV_FILE='$(ENV_FILE)' "$(DOCKER_DIR)/scripts/import-dump.sh" "$(DUMP)"

clean:
	@$(COMPOSE) --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" down -v
