#!/usr/bin/env bash
set -e
rm -rf "$DEST"

# This script exists as backwards compatibility for CI
(
	DEST="${DEST}-daemon"
	ABS_DEST="${ABS_DEST}-daemon"
	. hack/make/dynbinary-daemon
	. hack/make/dynbinary-proxy
)
