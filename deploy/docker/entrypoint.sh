#!/bin/bash
set -euo pipefail

if [[ -n "${OPENAI_API_KEY_FILE:-}" && -f "${OPENAI_API_KEY_FILE}" ]]; then
  export OPENAI_API_KEY="$(<"${OPENAI_API_KEY_FILE}")"
fi

if [[ -n "${MYSQL_PASSWORD_FILE:-}" && -f "${MYSQL_PASSWORD_FILE}" ]]; then
  export MYSQL_PASSWORD="$(<"${MYSQL_PASSWORD_FILE}")"
fi

if [[ -n "${OPENMCP_DATA_DIR:-}" ]]; then
  mkdir -p "${OPENMCP_DATA_DIR}"
  export OPENMCP_DATA_DIR
fi

if [[ -n "${OPENMCP_CONFIG_TEMPLATE:-}" && -f "${OPENMCP_CONFIG_TEMPLATE}" ]]; then
  target_path="${OPENMCP_CONFIG:-/etc/openmcp/config/openmcp.json}"
  mkdir -p "$(dirname "${target_path}")"
  envsubst < "${OPENMCP_CONFIG_TEMPLATE}" > "${target_path}"
fi

exec "$@"
