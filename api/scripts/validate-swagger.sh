#!/usr/bin/env bash
set -e

# Expected to be in api directory
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "Validating swagger.yaml..."

yamllint -f parsable -c validate/yamllint.yaml swagger.yaml

if out=$(swagger validate swagger.yaml); then
	echo "Validation done! ${out}"
else
	echo "${out}" >&2
	false
fi
