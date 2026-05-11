#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="${DOCKER_DIR}/backups"
COMPOSE_BIN="${COMPOSE_BIN:-docker compose}"
ENV_FILE="${ENV_FILE:-${DOCKER_DIR}/.env}"

mkdir -p "${BACKUP_DIR}"

COMPOSE_BIN="${COMPOSE_BIN}" ENV_FILE="${ENV_FILE}" "${SCRIPT_DIR}/export-current-db.sh"

LATEST_DUMP="$(ls -1t "${BACKUP_DIR}"/appdb_*.dump | head -n 1)"

if [ -z "${LATEST_DUMP}" ]; then
  echo "No dump file was created." >&2
  exit 1
fi

COMPOSE_BIN="${COMPOSE_BIN}" ENV_FILE="${ENV_FILE}" "${SCRIPT_DIR}/import-dump.sh" "${LATEST_DUMP}"

echo "Database migration completed into the compose postgres volume."
