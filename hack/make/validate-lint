#!/bin/bash

source "${MAKEDIR}/.validate"

# We will eventually get to the point when packages should be the complete list
# of subpackages, vendoring excluded, as given by:
#
# packages=( $(go list ./... 2> /dev/null | grep -vE "^github.com/docker/docker/vendor" || true ) )

packages=(
	builder
	builder/command
	builder/parser
	builder/parser/dumper
	daemon/events
	daemon/execdriver/native/template
	daemon/network
	docker
	dockerinit
	integration-cli
	pkg/chrootarchive
	pkg/directory
	pkg/fileutils
	pkg/homedir
	pkg/listenbuffer
	pkg/mflag/example
	pkg/mount
	pkg/namesgenerator
	pkg/nat
	pkg/promise
	pkg/pubsub
	pkg/random
	pkg/reexec
	pkg/symlink
	pkg/timeutils
	pkg/tlsconfig
	pkg/urlutil
	pkg/version
	registry
	utils
)

errors=()
for p in "${packages[@]}"; do
	# Run golint on package/*.go file explicitly to validate all go files
	# and not just the ones for the current platform.
	failedLint=$(golint "$p"/*.go)
	if [ "$failedLint" ]; then
		errors+=( "$failedLint" )
	fi
done

if [ ${#errors[@]} -eq 0 ]; then
	echo 'Congratulations!  All Go source files have been linted.'
else
	{
		echo "Errors from golint:"
		for err in "${errors[@]}"; do
			echo "$err"
		done
		echo
		echo 'Please fix the above errors. You can test via "golint" and commit the result.'
		echo
	} >&2
	false
fi
