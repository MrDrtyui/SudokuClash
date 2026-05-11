#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="${DOCKER_DIR}/docker-compose.yml"
COMPOSE_BIN="${COMPOSE_BIN:-docker compose}"
ENV_FILE="${ENV_FILE:-${DOCKER_DIR}/.env}"
DUMP_FILE="${1:-}"

if [ -z "${DUMP_FILE}" ]; then
  echo "Usage: $0 /absolute/path/to/dump.dump" >&2
  exit 1
fi

if [ ! -f "${DUMP_FILE}" ]; then
  echo "Dump file not found: ${DUMP_FILE}" >&2
  exit 1
fi

cd "${DOCKER_DIR}"

${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d postgres

echo "Waiting for postgres container to become ready..."
until ${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T postgres pg_isready -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-appdb}" >/dev/null 2>&1; do
  sleep 2
done

echo "Importing ${DUMP_FILE} into compose postgres volume..."
cat "${DUMP_FILE}" | ${COMPOSE_BIN} --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T postgres \
  pg_restore --clean --if-exists --no-owner -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-appdb}"

echo "Import complete."
