#!/bin/sh
set -eu

: "${OTASIGN_BACKEND_URL:?OTASIGN_BACKEND_URL is required}"

curl --fail --silent --show-error "${OTASIGN_BACKEND_URL%/}/readyz" >/dev/null
curl --fail --silent --show-error "${OTASIGN_BACKEND_URL%/}/healthz/full"
