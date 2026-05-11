#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="${DOCKER_DIR}/backups"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
DUMP_FILE="${BACKUP_DIR}/appdb_${TIMESTAMP}.dump"
LATEST_DUMP_FILE="${BACKUP_DIR}/appdb_latest.dump"

: "${SOURCE_DATABASE_URL:=postgresql://postgres:changeme@host.docker.internal:5432/appdb?sslmode=disable}"

mkdir -p "${BACKUP_DIR}"

echo "Exporting current database to ${DUMP_FILE}"
docker run --rm \
  -e SOURCE_DATABASE_URL="${SOURCE_DATABASE_URL}" \
  -v "${BACKUP_DIR}:/backup" \
  postgres:16-alpine \
  sh -lc 'pg_dump --dbname="$SOURCE_DATABASE_URL" --format=custom --no-owner --no-privileges --file="/backup/$(basename "'"${DUMP_FILE}"'")"'

cp "${DUMP_FILE}" "${LATEST_DUMP_FILE}"

echo "Done: ${DUMP_FILE}"
echo "Updated latest dump: ${LATEST_DUMP_FILE}"
