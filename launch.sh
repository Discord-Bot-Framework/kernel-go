#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
readonly CONFIGS_DIR="${SCRIPT_DIR}/../Discord"
readonly DOTENV_PATH="${CONFIGS_DIR}/.env"
readonly PROFILE_FILE="${SCRIPT_DIR}/firejail.profile"
readonly BIN_DIR="${SCRIPT_DIR}/bin"
readonly KERNEL_BIN="${BIN_DIR}/kernel"

cd "${SCRIPT_DIR}"

if [[ -f .env ]]; then
  mkdir -p "${CONFIGS_DIR}"
  cp .env "${DOTENV_PATH}"
fi

if [[ -f "${DOTENV_PATH}" ]]; then
  cp "${DOTENV_PATH}" .env
fi

if ! command -v go >/dev/null 2>&1; then
  printf 'Not found: Go\n' >&2
  exit 1
fi

mkdir -p "${BIN_DIR}"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${KERNEL_BIN}" ./cmd/kernel

if [[ ! -f "${PROFILE_FILE}" ]]; then
  printf 'Not found: firejail.profile\n' >&2
  exit 1
fi

exec firejail \
  --profile="${PROFILE_FILE}" \
  --read-write="${SCRIPT_DIR}" \
  --read-only="${PROFILE_FILE}" \
  "${KERNEL_BIN}"
