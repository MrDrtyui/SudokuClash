#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
SECRETS_FILE="${SECRETS_FILE:-${DOCKER_DIR}/.secrets.env}"
SECRETS_ENC_FILE="${SECRETS_ENC_FILE:-${DOCKER_DIR}/.secrets.env.enc}"

if [ ! -f "${SECRETS_ENC_FILE}" ]; then
  echo "No encrypted secrets file found at ${SECRETS_ENC_FILE}; skipping decrypt."
  exit 0
fi

if [ -z "${SUDOKU_SECRETS_KEY:-}" ]; then
  echo "SUDOKU_SECRETS_KEY is not set; skipping decrypt of ${SECRETS_ENC_FILE}."
  exit 0
fi

tmp_file="$(mktemp)"
cleanup() {
  rm -f "${tmp_file}"
}
trap cleanup EXIT INT TERM

if ! openssl enc -d -aes-256-cbc -pbkdf2 -a \
  -pass "pass:${SUDOKU_SECRETS_KEY}" \
  -in "${SECRETS_ENC_FILE}" \
  -out "${tmp_file}" 2>/dev/null; then
  echo "Failed to decrypt ${SECRETS_ENC_FILE}; check SUDOKU_SECRETS_KEY." >&2
  exit 1
fi

mv "${tmp_file}" "${SECRETS_FILE}"
trap - EXIT INT TERM
echo "Decrypted secrets into ${SECRETS_FILE}."
