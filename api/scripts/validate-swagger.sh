#!/usr/bin/env bash
set -e

# Expected to be in api directory
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "Validating openapi.yaml..."

yamllint -f parsable -c validate/yamllint.yaml openapi.yaml

python3 -B -m unittest discover -s scripts -p '*_test.py'

if out=$(openapi-spec-validator --schema 3.2 openapi.yaml); then
	echo "Validation done! ${out}"
else
	echo "${out}" >&2
	false
fi

echo "Validating swagger.yaml..."
swagger validate swagger.yaml
