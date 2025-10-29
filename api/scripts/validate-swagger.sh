#!/usr/bin/env bash
set -e
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPTDIR}/.validate"

IFS=$'\n'
files=($(validate_diff --diff-filter=ACMR --name-only -- 'api/swagger.yaml' || true))
unset IFS

if [ -n "${TEST_FORCE_VALIDATE:-}" ] || [ ${#files[@]} -gt 0 ]; then
	yamllint -f parsable -c "${SCRIPTDIR}"/yamllint.yaml api/swagger.yaml
	if out=$(swagger validate api/swagger.yaml); then
		echo "Congratulations!  ${out}"
	else
		echo "${out}" >&2
		false
	fi
fi
