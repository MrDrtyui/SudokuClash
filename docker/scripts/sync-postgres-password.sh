#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="${DOCKER_DIR}/docker-compose.yml"
COMPOSE_BIN="${COMPOSE_BIN:-docker compose}"
ENV_FILE="${ENV_FILE:-${DOCKER_DIR}/.env}"

if [ -z "${POSTGRES_PASSWORD:-}" ]; then
  echo "POSTGRES_PASSWORD is empty; skipping postgres password sync."
  exit 0
fi

cd "${DOCKER_DIR}"

${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d postgres

echo "Waiting for postgres container to become ready..."
until ${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T postgres pg_isready -U "${POSTGRES_USER:-postgres}" -d postgres >/dev/null 2>&1; do
  sleep 2
done

SAFE_PASSWORD=$(printf "%s" "${POSTGRES_PASSWORD}" | sed "s/'/''/g")

${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T postgres \
  psql -U "${POSTGRES_USER:-postgres}" -d postgres -c "ALTER USER \"${POSTGRES_USER:-postgres}\" WITH PASSWORD '${SAFE_PASSWORD}';" >/dev/null

echo "Postgres password synced for role ${POSTGRES_USER:-postgres}."
