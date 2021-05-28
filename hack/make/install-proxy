#!/usr/bin/env bash

set -e
rm -rf "$DEST"

source "${MAKEDIR}/.install"

(
	DEST="$(dirname $DEST)/binary-proxy"
	install_binary "${DEST}/docker-proxy"
)
