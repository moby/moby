#!/usr/bin/env bash
set -euo pipefail

API_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Replace the checked-in specification only after conversion succeeds.
SPEC="$(mktemp "${API_DIR}/swagger.yaml.XXXXXX")"
trap 'rm -f "${SPEC}"' EXIT
python3 -B "${API_DIR}/scripts/swagger_spec.py" "${API_DIR}/openapi.yaml" \
	"${API_DIR}/swagger.yaml" > "${SPEC}"
chmod 644 "${SPEC}"
mv "${SPEC}" "${API_DIR}/swagger.yaml"
