#!/usr/bin/env bash

# This file is just wrapper around 'go mod vendor' tool.
# For updating dependencies you should change `vendor.mod` file in root of the
# project.

set -e
set -x

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPTDIR}"/go-mod-prepare.sh

GO111MODULE=auto go mod tidy -modfile 'vendor.mod' -compat 1.18
GO111MODULE=auto go mod vendor -modfile vendor.mod
