#!/usr/bin/env bash

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPTDIR}/.validate"

IFS=$'\n'
files=($(validate_diff --diff-filter=ACMR --name-only -- 'api/types/' 'api/swagger.yaml' || true))
unset IFS

if [ -n "${TEST_FORCE_VALIDATE:-}" ] || [ ${#files[@]} -gt 0 ]; then
	"${SCRIPTDIR}"/../generate-swagger-api.sh 2> /dev/null
	# Let see if the working directory is clean
	diffs="$(git diff -- api/types/)"
	if [ "$diffs" ]; then
		{
			echo 'The result of hack/generate-swagger-api.sh differs'
			echo
			echo "$diffs"
			echo
			echo 'Please update api/swagger.yaml with any API changes, then '
			echo 'run hack/generate-swagger-api.sh.'
		} >&2
		false
	else
		echo 'Congratulations!  All API changes are done the right way.'
	fi
else
	echo 'No api/types/ or api/swagger.yaml changes in diff.'
fi
