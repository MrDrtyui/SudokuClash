#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="${ENV_FILE:-${DOCKER_DIR}/.env}"
ENV_EXAMPLE="${ENV_EXAMPLE:-${DOCKER_DIR}/.env.example}"
SECRETS_FILE="${SECRETS_FILE:-${DOCKER_DIR}/.secrets.env}"

if [ -f "${SECRETS_FILE}" ]; then
  set -a
  # shellcheck disable=SC1090
  . "${SECRETS_FILE}"
  set +a
fi

if [ ! -f "${ENV_FILE}" ]; then
  cp "${ENV_EXAMPLE}" "${ENV_FILE}"
  echo "Created ${ENV_FILE} from template."
fi

random_hex() {
  openssl rand -hex "${1:-24}"
}

get_value() {
  key="$1"
  awk -F= -v k="$key" '$1 == k { sub(/^[^=]*=/, "", $0); print $0; exit }' "${ENV_FILE}" 2>/dev/null || true
}

set_value() {
  key="$1"
  value="$2"
  tmp_file="$(mktemp)"
  if grep -q "^${key}=" "${ENV_FILE}" 2>/dev/null; then
    awk -v k="$key" -v v="$value" '
      BEGIN { done = 0 }
      $0 ~ "^" k "=" && done == 0 { print k "=" v; done = 1; next }
      { print }
      END { if (done == 0) print k "=" v }
    ' "${ENV_FILE}" > "${tmp_file}"
  else
    cat "${ENV_FILE}" > "${tmp_file}"
    printf "%s=%s\n" "${key}" "${value}" >> "${tmp_file}"
  fi
  mv "${tmp_file}" "${ENV_FILE}"
}

ensure_value() {
  key="$1"
  current="$(get_value "$key")"
  if [ -z "${current}" ]; then
    set_value "$key" "$2"
  fi
}

ensure_nonplaceholder() {
  key="$1"
  placeholder="$2"
  current="$(get_value "$key")"
  if [ -z "${current}" ] || [ "${current}" = "${placeholder}" ]; then
    set_value "$key" "$3"
  fi
}

ensure_nonplaceholder "JWT_SECRET" "change-me-before-production" "$(random_hex 24)"
ensure_value "COMPOSE_PROJECT_NAME" "sudoku"
ensure_value "POSTGRES_DB" "appdb"
ensure_value "POSTGRES_USER" "postgres"
ensure_value "POSTGRES_PASSWORD" "changeme"
ensure_value "POSTGRES_PORT" "5433"
ensure_value "REDIS_PORT" "6380"
ensure_value "NGINX_HTTP_PORT" "8088"
ensure_value "VITE_API_URL" "/api"
ensure_value "ACCESS_TOKEN_TTL" "15m"
ensure_value "REFRESH_TOKEN_TTL" "720h"
ensure_value "SHUTDOWN_TIMEOUT" "10s"
ensure_value "MATCHMAKING_WINDOW" "45s"
ensure_value "AUTO_DUMP_ON_RUN" "true"
ensure_value "AUTO_MIGRATE_ON_RUN" "true"
ensure_value "SOURCE_DATABASE_URL" "postgresql://postgres:changeme@host.docker.internal:5432/appdb?sslmode=disable"

if [ -n "${CLOUDFLARE_TUNNEL_TOKEN:-}" ]; then
  set_value "CLOUDFLARE_TUNNEL_TOKEN" "${CLOUDFLARE_TUNNEL_TOKEN}"
  ensure_value "PUBLIC_HOSTNAME" "sudoku.endfieldhq.com"
  set_value "FRONTEND_URL" "https://$(get_value PUBLIC_HOSTNAME)"
else
  ensure_value "PUBLIC_HOSTNAME" "sudoku.endfieldhq.com"
  current_frontend_url="$(get_value FRONTEND_URL)"
  if [ -z "${current_frontend_url}" ] || [ "${current_frontend_url}" = "https://sudoku.endfieldhq.com" ]; then
    set_value "FRONTEND_URL" "http://localhost:8088"
  fi
fi

if [ -n "${STRIPE_SECRET_KEY:-}" ]; then
  set_value "STRIPE_SECRET_KEY" "${STRIPE_SECRET_KEY}"
fi

if [ -n "${STRIPE_WEBHOOK_SECRET:-}" ]; then
  set_value "STRIPE_WEBHOOK_SECRET" "${STRIPE_WEBHOOK_SECRET}"
fi

echo "Environment bootstrap complete: ${ENV_FILE}"
