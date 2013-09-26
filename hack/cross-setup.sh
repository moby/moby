#!/bin/bash
set -e

DEST="$1"

source "$(dirname "$BASH_SOURCE")/cross-platforms"

if [ ! -d /goroot/src ]; then
	echo >&2 "# ERROR! I can't seem to find /goroot/src."
	echo >&2 "# This probably means we're not running this inside the official container."
	exit 1
fi

for PLATFORM in ${CROSS_PLATFORMS[@]}; do
	(
		cd /goroot/src \
			&& GOOS=${PLATFORM%-*} \
				GOARCH=${PLATFORM#*-} \
				./make.bash --no-clean
	)
done
